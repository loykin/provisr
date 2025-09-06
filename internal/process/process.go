package process

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/loykin/provisr/internal/detector"
)

type Process struct {
	spec       Spec
	cmd        *exec.Cmd
	status     Status
	mu         sync.Mutex
	stopping   bool // true when Stop has been requested; suppress autorestart
	restarts   int
	outCloser  io.WriteCloser
	errCloser  io.WriteCloser
	waitDone   chan struct{} // closed by monitor when cmd.Wait returns
	monitoring bool          // true when monitor goroutine is running
}

func New(spec Spec) *Process { return &Process{spec: spec} }

// UpdateSpec replaces the internal spec under lock.
func (r *Process) UpdateSpec(s Spec) {
	r.mu.Lock()
	r.spec = s
	r.mu.Unlock()
}

// ConfigureCmd builds and configures *exec.Cmd for this process using mergedEnv.
// It sets workdir, environment, stdio/logging, and process group attributes.
// Logging writers are prepared and stored via EnsureLogClosers.
func (r *Process) ConfigureCmd(mergedEnv []string) *exec.Cmd {
	cmd := r.spec.BuildCommand()
	if r.spec.WorkDir != "" {
		cmd.Dir = r.spec.WorkDir
	}
	if len(mergedEnv) > 0 {
		cmd.Env = mergedEnv
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Setup logging if configured
	if r.spec.Log.Dir != "" || r.spec.Log.StdoutPath != "" || r.spec.Log.StderrPath != "" {
		if r.spec.Log.Dir != "" {
			_ = os.MkdirAll(r.spec.Log.Dir, 0o750)
		}
		outW, errW, _ := r.spec.Log.Writers(r.spec.Name)
		r.EnsureLogClosers(outW, errW)
		ow, ew := r.OutErrClosers()
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

// Accessors with internal locking kept within methods to avoid external lock usage.

func (r *Process) CopyCmd() *exec.Cmd {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd
}

func (r *Process) SetStarted(cmd *exec.Cmd) {
	r.mu.Lock()
	r.cmd = cmd
	r.waitDone = make(chan struct{})
	r.status.Name = r.spec.Name
	r.status.Running = true
	r.status.PID = cmd.Process.Pid
	r.status.StartedAt = time.Now()
	r.status.Restarts = r.restarts
	r.stopping = false
	r.mu.Unlock()
}

// TryStart atomically starts the command and updates internal state and PID file.
// It encapsulates cmd.Start + SetStarted + WritePIDFile to reduce races.
func (r *Process) TryStart(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	// After successful start, record state and write PID file under lock-ordered ops.
	r.SetStarted(cmd)
	// Write PID file synchronously to ensure availability immediately after Start returns.
	r.WritePIDFile()
	return nil
}
func (r *Process) CloseWaitDone() {
	r.mu.Lock()
	if r.waitDone != nil {
		close(r.waitDone)
		r.waitDone = nil
	}
	r.mu.Unlock()
}

func (r *Process) WaitDoneChan() chan struct{} {
	r.mu.Lock()
	wd := r.waitDone
	r.mu.Unlock()
	return wd
}

func (r *Process) MarkExited(err error) {
	r.mu.Lock()
	r.status.Running = false
	r.status.StoppedAt = time.Now()
	r.status.ExitErr = err
	r.mu.Unlock()
}

func (r *Process) SetStopRequested(v bool) {
	r.mu.Lock()
	r.stopping = v
	r.mu.Unlock()
}

func (r *Process) StopRequested() bool {
	r.mu.Lock()
	v := r.stopping
	r.mu.Unlock()
	return v
}

func (r *Process) IncRestarts() int {
	r.mu.Lock()
	r.restarts++
	v := r.restarts
	r.mu.Unlock()
	return v
}

func (r *Process) MonitoringStartIfNeeded() bool {
	r.mu.Lock()
	if r.monitoring {
		r.mu.Unlock()
		return false
	}
	r.monitoring = true
	r.mu.Unlock()
	return true
}

func (r *Process) MonitoringStop() {
	r.mu.Lock()
	r.monitoring = false
	r.mu.Unlock()
}

func (r *Process) OutErrClosers() (io.WriteCloser, io.WriteCloser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outCloser, r.errCloser
}

func (r *Process) EnsureLogClosers(stdout, stderr io.WriteCloser) {
	r.mu.Lock()
	if r.outCloser == nil && stdout != nil {
		r.outCloser = stdout
	}
	if r.errCloser == nil && stderr != nil {
		r.errCloser = stderr
	}
	r.mu.Unlock()
}

func (r *Process) CloseWriters() {
	r.mu.Lock()
	if r.outCloser != nil {
		_ = r.outCloser.Close()
		r.outCloser = nil
	}
	if r.errCloser != nil {
		_ = r.errCloser.Close()
		r.errCloser = nil
	}
	r.mu.Unlock()
}

func (r *Process) WritePIDFile() {
	if r.spec.PIDFile == "" {
		return
	}
	r.mu.Lock()
	pid := 0
	if r.cmd != nil && r.cmd.Process != nil {
		pid = r.cmd.Process.Pid
	}
	r.mu.Unlock()
	if pid == 0 {
		return
	}
	_ = os.MkdirAll(filepath.Dir(r.spec.PIDFile), 0o750)
	_ = os.WriteFile(r.spec.PIDFile, []byte(strconv.Itoa(pid)), 0o600)
}

// RemovePIDFile best-effort
func (r *Process) RemovePIDFile() {
	if r.spec.PIDFile == "" {
		return
	}
	_ = os.Remove(r.spec.PIDFile)
}

// Snapshot returns a copy of the current status.
func (r *Process) Snapshot() Status {
	r.mu.Lock()
	s := r.status
	r.mu.Unlock()
	return s
}

// DetectAlive probes liveness avoiding races with os/exec internals.
func (r *Process) DetectAlive() (bool, string) {
	r.mu.Lock()
	running := r.status.Running
	cmd := r.cmd
	r.mu.Unlock()
	if !running {
		return false, ""
	}
	if cmd != nil && cmd.Process != nil {
		if syscall.Kill(-cmd.Process.Pid, 0) == nil {
			return true, "exec:pid"
		}
		return false, ""
	}
	for _, d := range r.detectors() {
		ok, _ := d.Alive()
		if ok {
			return true, d.Describe()
		}
	}
	return false, ""
}

func (r *Process) detectors() []detector.Detector {
	dets := make([]detector.Detector, 0, len(r.spec.Detectors)+1)
	if r.spec.PIDFile != "" {
		dets = append(dets, detector.PIDFileDetector{PIDFile: r.spec.PIDFile})
	}
	dets = append(dets, r.spec.Detectors...)
	return dets
}

// EnforceStartDuration waits until d ensuring process stays up; returns error if it exits early.
func (r *Process) EnforceStartDuration(d time.Duration) error {
	if d <= 0 {
		return nil
	}
	deadline := time.NewTimer(d)
	defer deadline.Stop()
	// Also observe waitDone to detect early exit promptly (monitor closes it)
	wd := r.WaitDoneChan()
	// Quick check: if process already gone
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return errBeforeStart(d)
	}
	select {
	case <-wd:
		return errBeforeStart(d)
	case <-deadline.C:
		return nil
	}
}
