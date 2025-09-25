package process

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	// Configure platform-specific process attributes (detached, process group, etc.)
	configureSysProcAttr(cmd, spec)

	// Setup slog-based logging if configured
	if spec.Detached && spec.Log.File.Dir != "" {
		slog.Warn("Detached processes do not support logging")
	}

	if !spec.Detached && (spec.Log.File.Dir != "" || spec.Log.File.StdoutPath != "" || spec.Log.File.StderrPath != "") {
		if spec.Log.File.Dir != "" {
			if err := os.MkdirAll(spec.Log.File.Dir, 0o750); err != nil {
				slog.Warn("Failed to create log directory", "dir", spec.Log.File.Dir, "error", err)
			}
		}
		// Use unified config for both structured logging and file writers
		outW, errW, _ := spec.Log.ProcessWriters(spec.Name)
		r.EnsureLogClosers(outW, errW)
		ow, ew := r.OutErrClosers()
		if ow != nil {
			cmd.Stdout = ow
		} else {
			// Use io.Discard to avoid opening file handles for /dev/null
			cmd.Stdout = io.Discard
		}
		if ew != nil {
			cmd.Stderr = ew
		} else {
			// Use io.Discard to avoid opening file handles for /dev/null
			cmd.Stderr = io.Discard
		}
	}
	return cmd
}

// Accessors with internal locking kept within methods to avoid external lock usage.

func (r *Process) CopyCmd() *exec.Cmd {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd
}

// SeedPID seeds the internal PID (e.g., after manager restart) without changing running state.
// It also updates the Snapshot() PID for observability.
func (r *Process) SeedPID(pid int) {
	r.mu.Lock()
	if pid > 0 {
		r.pid = pid
		r.status.PID = pid
	}
	r.mu.Unlock()
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
	// SysProcAttr must already be configured by ConfigureCmd; do not override here.
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
		if err := r.outCloser.Close(); err != nil {
			slog.Warn("Failed to close stdout writer", "error", err)
		}
		r.outCloser = nil
	}
	if r.errCloser != nil {
		if err := r.errCloser.Close(); err != nil {
			slog.Warn("Failed to close stderr writer", "error", err)
		}
		r.errCloser = nil
	}
	r.mu.Unlock()
}

func (r *Process) WritePIDFile() {
	r.mu.Lock()
	pidFile := r.spec.PIDFile
	pid := 0
	var specCopy *Spec
	if r.cmd != nil && r.cmd.Process != nil {
		pid = r.cmd.Process.Pid
	}
	if r.spec.Name != "" {
		specCopy = r.spec.DeepCopy()
	}
	r.mu.Unlock()

	if pidFile == "" || pid == 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o750); err != nil {
		slog.Warn("Failed to create PID file directory", "dir", filepath.Dir(pidFile), "error", err)
		return
	}

	// Write PID in first line for backward compatibility, then JSON-encoded Spec,
	// and optionally a third line with PIDMeta JSON containing process start time.
	var body []byte
	if specCopy != nil {
		if jb, err := json.Marshal(specCopy); err == nil {
			body = append([]byte(strconv.Itoa(pid)), '\n')
			body = append(body, jb...)
			// Append meta
			startUnix := getProcStartUnix(pid)
			if startUnix > 0 {
				meta := PIDMeta{StartUnix: startUnix}
				if mb, err := json.Marshal(&meta); err == nil {
					body = append(body, '\n')
					body = append(body, mb...)
				}
			}
		} else {
			body = []byte(strconv.Itoa(pid))
		}
	} else {
		body = []byte(strconv.Itoa(pid))
	}
	if err := os.WriteFile(pidFile, body, 0o600); err != nil {
		slog.Warn("Failed to write PID file", "file", pidFile, "error", err)
	}
}

// RemovePIDFile best-effort
func (r *Process) RemovePIDFile() {
	r.mu.Lock()
	pidFile := r.spec.PIDFile
	r.mu.Unlock()

	if pidFile == "" {
		return
	}
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to remove PID file", "file", pidFile, "error", err)
	}
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
	spec := r.spec
	r.mu.Unlock()

	// If we already detected exit, process is dead
	if exited {
		return false, "exit-detected"
	}

	// If we have a PID, prefer checking it directly first
	if pid > 0 {
		if syscall.Kill(pid, 0) == nil {
			return true, "exec:pid"
		}
	}

	// If PID-based detection fails or PID is unknown, try configured detectors (e.g., PID file)
	for _, d := range r.detectors() {
		ok, _ := d.Alive()
		if ok {
			// Best-effort: if a PID file is configured, read it and seed internal PID for later signaling
			if spec.PIDFile != "" {
				if b, err := os.ReadFile(spec.PIDFile); err == nil {
					if n, err2 := strconv.Atoi(strings.TrimSpace(string(b))); err2 == nil && n > 0 {
						r.SeedPID(n)
					}
				}
			}
			return true, d.Describe()
		}
	}

	if pid == 0 {
		return false, "no-pid"
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
	hasProcess := cmd != nil && cmd.Process != nil
	r.mu.Unlock()
	if !hasProcess {
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
			slog.Warn("Failed to send signal to process group, falling back to SIGKILL",
				"pid", pid, "signal", sig, "error", err)
			// Fall back to SIGKILL best-effort; upper layers manage further retries
			return r.Kill()
		}
		return nil
	}
	// Fallback: if cmd is unavailable (e.g., after manager restart), use stored PID
	r.mu.Lock()
	pid := r.pid
	r.mu.Unlock()
	if pid > 0 {
		if err := syscall.Kill(-pid, sig); err != nil {
			slog.Warn("Failed to send signal to stored PID, falling back to SIGKILL",
				"pid", pid, "signal", sig, "error", err)
			// Fall back to SIGKILL on the same PID
			if killErr := syscall.Kill(-pid, syscall.SIGKILL); killErr != nil {
				slog.Warn("Failed to kill process with SIGKILL fallback", "pid", pid, "error", killErr)
			}
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
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		slog.Warn("Failed to kill process", "pid", pid, "error", err)
	}
	return nil
}
