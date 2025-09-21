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
	spec      Spec
	cmd       *exec.Cmd
	status    Status
	mu        sync.Mutex
	stopping  bool // true when Stop has been requested; suppress autorestart
	outCloser io.WriteCloser
	errCloser io.WriteCloser
	pid       int   // Process ID for safe detection
	exited    bool  // Track if process has exited
	exitErr   error // Exit error if any
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
	r.mu.Lock()
	spec := r.spec // Create a copy to avoid holding lock during I/O operations
	r.mu.Unlock()

	cmd := spec.BuildCommand()
	if spec.WorkDir != "" {
		cmd.Dir = spec.WorkDir
	}
	if len(mergedEnv) > 0 {
		cmd.Env = mergedEnv
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Setup slog-based logging if configured
	if spec.Log.File.Dir != "" || spec.Log.File.StdoutPath != "" || spec.Log.File.StderrPath != "" {
		if spec.Log.File.Dir != "" {
			_ = os.MkdirAll(spec.Log.File.Dir, 0o750)
		}
		// Use unified config for both structured logging and file writers
		outW, errW, _ := spec.Log.ProcessWriters(spec.Name)
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
	r.status.Name = r.spec.Name
	r.status.Running = true
	r.status.PID = cmd.Process.Pid
	r.status.StartedAt = time.Now()
	r.stopping = false

	// Store PID for race-free detection
	r.pid = cmd.Process.Pid
	r.exited = false
	r.exitErr = nil
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

	go func() {
		err := cmd.Wait()
		r.MarkExited(err)
	}()

	return nil
}

func (r *Process) MarkExited(err error) {
	r.mu.Lock()
	r.status.Running = false
	r.status.StoppedAt = time.Now()
	r.status.ExitErr = err

	// Mark as exited for race-free detection
	r.exited = true
	r.exitErr = err
	r.mu.Unlock()
}

func (r *Process) GetName() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.spec.Name
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
	r.mu.Lock()
	pidFile := r.spec.PIDFile
	pid := 0
	if r.cmd != nil && r.cmd.Process != nil {
		pid = r.cmd.Process.Pid
	}
	r.mu.Unlock()

	if pidFile == "" || pid == 0 {
		return
	}
	_ = os.MkdirAll(filepath.Dir(pidFile), 0o750)
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o600)
}

// RemovePIDFile best-effort
func (r *Process) RemovePIDFile() {
	r.mu.Lock()
	pidFile := r.spec.PIDFile
	r.mu.Unlock()

	if pidFile == "" {
		return
	}
	_ = os.Remove(pidFile)
}

// Snapshot returns a copy of the current status.
func (r *Process) Snapshot() Status {
	r.mu.Lock()
	s := r.status
	r.mu.Unlock()
	return s
}

func (r *Process) GetSpec() *Spec {
	r.mu.Lock()
	s := r.spec.DeepCopy()
	r.mu.Unlock()
	return s
}

func (r *Process) GetAutoStart() bool {
	r.mu.Lock()
	v := r.spec.AutoRestart
	r.mu.Unlock()
	return v
}

// DetectAlive probes liveness without accessing cmd to avoid races.
func (r *Process) DetectAlive() (bool, string) {
	r.mu.Lock()
	pid := r.pid
	exited := r.exited
	r.mu.Unlock()

	// If we don't have a PID yet, process never started
	if pid == 0 {
		return false, "no-pid"
	}

	// If we already detected exit, process is dead
	if exited {
		return false, "exit-detected"
	}

	// Prefer individual PID signal check first (works reliably across platforms)
	if syscall.Kill(pid, 0) == nil {
		return true, "exec:pid"
	}

	// If PID-based detection fails, try configured detectors
	for _, d := range r.detectors() {
		ok, _ := d.Alive()
		if ok {
			return true, d.Describe()
		}
	}
	return false, "not-found"
}

func (r *Process) detectors() []detector.Detector {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	// Quick check: if process already gone
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return errBeforeStart(d)
	}

	// Poll-based approach to avoid race conditions with cmd.Wait()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		// Check if the process is still alive
		alive, _ := r.DetectAlive()
		if !alive {
			return errBeforeStart(d)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// StopWithSignal sends the provided signal to the process group. It does not wait.
// If sending the signal fails, it falls back to Kill().
func (r *Process) StopWithSignal(sig syscall.Signal) error {
	alive, _ := r.DetectAlive()
	if !alive {
		return nil
	}
	cmd := r.CopyCmd()
	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		if err := syscall.Kill(-pid, sig); err != nil {
			// Fall back to SIGKILL best-effort; upper layers manage further retries
			return r.Kill()
		}
	}
	return nil
}

// Stop keeps backward compatibility with previous API. It sends SIGTERM and returns immediately.
// The wait parameter is ignored to preserve SRP; upper layers handle waiting and state.
func (r *Process) Stop(_ time.Duration) error {
	return r.StopWithSignal(syscall.SIGTERM)
}

// Kill sends SIGKILL to the process group and attempts to reap promptly.
func (r *Process) Kill() error {
	cmd := r.CopyCmd()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return nil
}
