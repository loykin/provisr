package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/loykin/provisr/internal/env"
	"github.com/loykin/provisr/internal/metrics"
)

// buildCommand constructs an *exec.Cmd for the given spec.Command.
// It avoids invoking a shell when not necessary to reduce command injection surface (G204 mitigation).
// If the command contains obvious shell metacharacters, it falls back to /bin/sh -c.
func buildCommand(spec Spec) *exec.Cmd {
	cmdStr := strings.TrimSpace(spec.Command)
	if cmdStr == "" {
		// still create a command that will fail when started
		return exec.Command("/bin/true")
	}
	// #nosec G204 Detect shell metacharacters
	if strings.ContainsAny(cmdStr, "|&;<>*?`$\"'(){}[]~") {
		return exec.Command("/bin/sh", "-c", cmdStr)
	}
	parts := strings.Fields(cmdStr)
	name := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	// #nosec G204
	return exec.Command(name, args...)
}

// Manager starts, stops and monitors processes.
type Manager struct {
	mu    sync.Mutex
	procs map[string]*proc
	envM  *env.Env
}

// monitor waits for process exit and auto-restarts if configured.
func (m *Manager) monitor(p *proc) {
	for {
		p.mu.Lock()
		cmd := p.cmd
		auto := p.spec.AutoRestart
		interval := p.spec.RestartInterval
		if interval <= 0 {
			interval = 1 * time.Second
		}
		p.mu.Unlock()

		if cmd == nil || !auto {
			return
		}

		// Wait for process to exit
		err := cmd.Wait()

		p.mu.Lock()
		p.status.Running = false
		p.status.StoppedAt = time.Now()
		p.status.ExitErr = err
		stopping := p.stopping
		p.mu.Unlock()

		if stopping {
			return
		}

		// attempt restart
		time.Sleep(interval)
		p.mu.Lock()
		p.restarts++
		name := p.spec.Name
		spec := p.spec
		p.mu.Unlock()
		metrics.IncRestart(name)
		_ = m.Start(spec)
		// After Start, loop continues and will set up a new Wait in the next iteration
	}
}

func NewManager() *Manager { return &Manager{procs: make(map[string]*proc), envM: env.New()} }

// SetGlobalEnv sets global environment variables affecting all processes managed by this Manager.
// kvs must be in the form "KEY=VALUE". Values may reference ${OTHER} variables and will be expanded at Merge time.
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

func (m *Manager) get(name string) *proc {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.procs[name]
}

// Start launches a process according to spec. It is idempotent if already running.
// configureCmd builds the exec.Cmd and sets env, workdir, stdio/logging.
func (m *Manager) configureCmd(spec Spec, p *proc) *exec.Cmd {
	cmd := buildCommand(spec)
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
		// Ensure directory exists if using Dir
		if spec.Log.Dir != "" {
			_ = os.MkdirAll(spec.Log.Dir, 0o750)
		}
		// Create writers only once per proc; reuse across restarts
		if p.outCloser == nil && p.errCloser == nil {
			outW, errW, _ := spec.Log.Writers(spec.Name)
			p.outCloser = outW
			p.errCloser = errW
		}
		if p.outCloser != nil {
			cmd.Stdout = p.outCloser
		} else {
			cmd.Stdout, _ = os.OpenFile(os.DevNull, os.O_RDWR, 0)
		}
		if p.errCloser != nil {
			cmd.Stderr = p.errCloser
		} else {
			cmd.Stderr, _ = os.OpenFile(os.DevNull, os.O_RDWR, 0)
		}
	} else {
		// Default to devnull
		null, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		cmd.Stdout = null
		cmd.Stderr = null
	}
	return cmd
}

// writePIDFile writes the PID file if requested.
func writePIDFile(pidfile string, pid int) {
	if pidfile == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(pidfile), 0o750)
	_ = os.WriteFile(pidfile, []byte(strconv.Itoa(pid)), 0o600)
}

// enforceStartDuration ensures the process stays up for StartDuration, otherwise returns an error.
func enforceStartDuration(p *proc, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		// Check for early exit
		if p.cmd == nil {
			return fmt.Errorf("process exited before start duration %s", d)
		}
		if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			return fmt.Errorf("process exited before start duration %s", d)
		}
		if tryReap(p) {
			return fmt.Errorf("process exited before start duration %s", d)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (m *Manager) Start(spec Spec) error {
	m.mu.Lock()
	p, exists := m.procs[spec.Name]
	if !exists {
		p = &proc{spec: spec}
		m.procs[spec.Name] = p
	}
	m.mu.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	alive, _ := p.detectAlive()
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
		cmd := m.configureCmd(spec, p)

		if err := cmd.Start(); err != nil {
			lastErr = err
			if i < attempts {
				time.Sleep(interval)
				continue
			}
			return lastErr
		}

		p.cmd = cmd
		p.status = Status{Name: spec.Name, Running: true, PID: cmd.Process.Pid, StartedAt: time.Now(), Restarts: p.restarts}
		p.stopping = false
		metrics.IncStart(spec.Name)

		// write pidfile if requested
		writePIDFile(spec.PIDFile, cmd.Process.Pid)

		// If start duration is specified, ensure the process stays up that long before considering success.
		startWait := spec.StartDuration
		if err := enforceStartDuration(p, startWait); err != nil {
			// Treat as failed start, cleanup and retry if allowed
			if spec.PIDFile != "" {
				_ = os.Remove(spec.PIDFile)
			}
			p.status.Running = false
			p.status.StoppedAt = time.Now()
			lastErr = err
			if i < attempts {
				time.Sleep(interval)
				continue
			}
			return lastErr
		}
		if startWait > 0 {
			metrics.ObserveStartDuration(spec.Name, startWait.Seconds())
		}

		// launch monitor if autorestart enabled
		if p.spec.AutoRestart {
			go m.monitor(p)
		}

		return nil
	}
	return lastErr
}

// Stop stops a running process. If already stopped, it's a no-op.
func (m *Manager) Stop(name string, wait time.Duration) error {
	p := m.get(name)
	if p == nil {
		return fmt.Errorf("unknown process: %s", name)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || !p.status.Running {
		return nil
	}

	// prevent monitor from auto-restarting due to this stop
	p.stopping = true
	// Send SIGTERM to process group
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
	ch := make(chan error, 1)
	go func() { ch <- p.cmd.Wait() }()

	select {
	case err := <-ch:
		p.status.Running = false
		p.status.StoppedAt = time.Now()
		p.status.ExitErr = err
		if p.outCloser != nil {
			_ = p.outCloser.Close()
			p.outCloser = nil
		}
		if p.errCloser != nil {
			_ = p.errCloser.Close()
			p.errCloser = nil
		}
		metrics.IncStop(name)
		return err
	case <-time.After(wait):
		// escalate to SIGKILL
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		p.status.Running = false
		p.status.StoppedAt = time.Now()
		p.status.ExitErr = fmt.Errorf("killed after timeout")
		if p.outCloser != nil {
			_ = p.outCloser.Close()
			p.outCloser = nil
		}
		if p.errCloser != nil {
			_ = p.errCloser.Close()
			p.errCloser = nil
		}
		metrics.IncStop(name)
		return p.status.ExitErr
	}
}

// Status returns current status including detector check.
func (m *Manager) Status(name string) (Status, error) {
	p := m.get(name)
	if p == nil {
		return Status{}, fmt.Errorf("unknown process: %s", name)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	alive, by := p.detectAlive()
	p.status.Running = alive
	p.status.DetectedBy = by
	p.status.Restarts = p.restarts
	return p.status, nil
}

// StartN starts Spec.Instances instances by suffixing names with -1..-N.
// If Instances <= 1, it behaves like Start.
func (m *Manager) StartN(spec Spec) error {
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

// StopAll stops all instances that were started with base name (prefix base-).
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
	// update gauge after stopping
	if c, err := m.Count(base); err == nil {
		metrics.SetRunningInstances(base, c)
	}
	return firstErr
}

// StatusAll returns statuses for all instances matching the base name.
func (m *Manager) StatusAll(base string) ([]Status, error) {
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for name := range m.procs {
		if name == base || strings.HasPrefix(name, base+"-") {
			names = append(names, name)
		}
	}
	m.mu.Unlock()
	res := make([]Status, 0, len(names))
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
