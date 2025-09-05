package manager

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/loykin/provisr/internal/env"
	"github.com/loykin/provisr/internal/metrics"
	run "github.com/loykin/provisr/internal/process"
)

// Manager starts, stops and monitors processes.
type Manager struct {
	mu    sync.Mutex
	procs map[string]*entry
	envM  *env.Env
}

type entry struct {
	r    *run.Process
	spec run.Spec
}

// monitor waits for process exit and auto-restarts if configured.
func (m *Manager) monitor(e *entry) {
	for {
		cmd := e.r.CopyCmd()
		if cmd == nil {
			e.r.MonitoringStop()
			return
		}
		// Wait for process to exit
		err := cmd.Wait()
		// notify waiters and mark status
		e.r.CloseWaitDone()
		e.r.MarkExited(err)
		stopping := e.r.StopRequested()
		autoRestart := e.spec.AutoRestart
		interval2 := e.spec.RestartInterval
		if interval2 <= 0 {
			interval2 = 1 * time.Second
		}
		if stopping || !autoRestart {
			e.r.MonitoringStop()
			return
		}
		// attempt restart
		time.Sleep(interval2)
		_ = e.r.IncRestarts()
		metrics.IncRestart(e.spec.Name)
		_ = m.Start(e.spec)
		// loop continues to wait on new cmd
	}
}

func NewManager() *Manager { return &Manager{procs: make(map[string]*entry), envM: env.New()} }

// SetGlobalEnv sets global environment variables affecting all processes managed by this Manager.
// kvs must be in the form "KEY=VALUE".
func (m *Manager) SetGlobalEnv(kvs []string) {
	if m.envM == nil {
		m.envM = env.New()
	}
	for _, kv := range kvs {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k := kv[:i]
			v := kv[i+1:]
			m.envM.Set(k, v)
		}
	}
}

func (m *Manager) get(name string) *entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.procs[name]
}

// configureCmd builds the exec.Cmd and sets env, workdir, stdio/logging.
func (m *Manager) configureCmd(spec run.Spec, e *entry) *exec.Cmd {
	cmd := spec.BuildCommand()
	if spec.WorkDir != "" {
		cmd.Dir = spec.WorkDir
	}
	// compute environment using env manager (global + per-process)
	if m.envM != nil {
		cmd.Env = m.envM.Merge(spec.Env)
	} else if len(spec.Env) > 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Setup logging if configured
	if spec.Log.Dir != "" || spec.Log.StdoutPath != "" || spec.Log.StderrPath != "" {
		if spec.Log.Dir != "" {
			_ = os.MkdirAll(spec.Log.Dir, 0o750)
		}
		outW, errW, _ := spec.Log.Writers(spec.Name)
		e.r.EnsureLogClosers(outW, errW)
		ow, ew := e.r.OutErrClosers()
		if ow != nil {
			cmd.Stdout = ow
		} else {
			cmd.Stdout, _ = os.OpenFile(os.DevNull, os.O_RDWR, 0)
		}
		if ew != nil {
			cmd.Stderr = ew
		} else {
			cmd.Stderr, _ = os.OpenFile(os.DevNull, os.O_RDWR, 0)
		}
	} else {
		null, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		cmd.Stdout = null
		cmd.Stderr = null
	}
	return cmd
}

func (m *Manager) Start(spec run.Spec) error {
	m.mu.Lock()
	e, exists := m.procs[spec.Name]
	if !exists {
		r := run.New(run.Spec{Name: spec.Name, PIDFile: spec.PIDFile, Detectors: spec.Detectors})
		e = &entry{r: r, spec: spec}
		m.procs[spec.Name] = e
	} else {
		e.spec = spec
	}
	m.mu.Unlock()

	// Fast path: if already alive, no-op
	alive, _ := e.r.DetectAlive()
	if alive {
		return nil
	}

	var lastErr error
	attempts := spec.RetryCount
	if attempts < 0 {
		attempts = 0
	}
	interval := spec.RetryInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	for i := 0; i <= attempts; i++ {
		cmd := m.configureCmd(spec, e)
		if err := cmd.Start(); err != nil {
			lastErr = err
			if i < attempts {
				time.Sleep(interval)
				continue
			}
			return lastErr
		}

		e.r.SetStarted(cmd)
		metrics.IncStart(spec.Name)
		e.r.WritePIDFile()

		// Start monitor BEFORE enforcing start duration to catch early exits promptly
		if e.r.MonitoringStartIfNeeded() {
			go m.monitor(e)
		}

		if err := e.r.EnforceStartDuration(spec.StartDuration); err != nil {
			e.r.RemovePIDFile()
			e.r.MarkExited(err)
			lastErr = err
			if i < attempts {
				time.Sleep(interval)
				continue
			}
			return lastErr
		}
		if spec.StartDuration > 0 {
			metrics.ObserveStartDuration(spec.Name, spec.StartDuration.Seconds())
		}

		return nil
	}
	return lastErr
}

// Stop stops a running process. If already stopped, it's a no-op.
func (m *Manager) Stop(name string, wait time.Duration) error {
	e := m.get(name)
	if e == nil {
		return fmt.Errorf("unknown process: %s", name)
	}
	alive, _ := e.r.DetectAlive()
	if !alive {
		return nil
	}
	e.r.SetStopRequested(true)
	cmd := e.r.CopyCmd()
	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		waitCh := e.r.WaitDoneChan()
		select {
		case <-waitCh:
			// monitor observed exit
		case <-time.After(wait):
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			time.Sleep(10 * time.Millisecond)
		}
	}
	e.r.CloseWriters()
	metrics.IncStop(name)
	rs := e.r.Snapshot()
	return rs.ExitErr
}

// Status returns current status including detector check.
func (m *Manager) Status(name string) (run.Status, error) {
	e := m.get(name)
	if e == nil {
		return run.Status{}, fmt.Errorf("unknown process: %s", name)
	}
	alive, by := e.r.DetectAlive()
	rs := e.r.Snapshot()
	st := run.Status{
		Name:       rs.Name,
		Running:    alive,
		PID:        rs.PID,
		StartedAt:  rs.StartedAt,
		StoppedAt:  rs.StoppedAt,
		ExitErr:    rs.ExitErr,
		DetectedBy: by,
		Restarts:   rs.Restarts,
	}
	return st, nil
}

// StartN starts Spec.Instances instances by suffixing names with -1..-N.
func (m *Manager) StartN(spec run.Spec) error {
	n := spec.Instances
	if n <= 1 {
		return m.Start(spec)
	}
	for i := 1; i <= n; i++ {
		inst := spec
		inst.Name = fmt.Sprintf("%s-%d", spec.Name, i)
		if err := m.Start(inst); err != nil {
			return err
		}
	}
	// update gauge for base name
	if c, err := m.Count(spec.Name); err == nil {
		metrics.SetRunningInstances(spec.Name, c)
	}
	return nil
}

// StopAll stops all instances with the base name.
func (m *Manager) StopAll(base string, wait time.Duration) error {
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for name := range m.procs {
		if name == base || strings.HasPrefix(name, base+"-") {
			names = append(names, name)
		}
	}
	m.mu.Unlock()
	var firstErr error
	for _, name := range names {
		if err := m.Stop(name, wait); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c, err := m.Count(base); err == nil {
		metrics.SetRunningInstances(base, c)
	}
	return firstErr
}

// StatusAll returns statuses for all instances matching the base name.
func (m *Manager) StatusAll(base string) ([]run.Status, error) {
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for name := range m.procs {
		if name == base || strings.HasPrefix(name, base+"-") {
			names = append(names, name)
		}
	}
	m.mu.Unlock()
	res := make([]run.Status, 0, len(names))
	for _, n := range names {
		st, err := m.Status(n)
		if err != nil {
			return nil, err
		}
		res = append(res, st)
	}
	return res, nil
}

// Count returns number of running instances for the base name.
func (m *Manager) Count(base string) (int, error) {
	sts, err := m.StatusAll(base)
	if err != nil {
		return 0, err
	}
	c := 0
	for _, st := range sts {
		if st.Running {
			c++
		}
	}
	return c, nil
}
