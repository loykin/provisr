package process

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	// Setup logging if configured
	if spec.Log.Dir != "" || spec.Log.StdoutPath != "" || spec.Log.StderrPath != "" {
		if spec.Log.Dir != "" {
			_ = os.MkdirAll(spec.Log.Dir, 0o750)
		}
		outW, errW, _ := spec.Log.Writers(spec.Name)
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

// IsMonitoring reports whether a monitor goroutine (e.g., Supervisor) is actively
// waiting on the underlying process. When true, Stop/Kill must not call cmd.Wait
// to avoid a race with the monitor; they should instead wait on waitDone.
func (r *Process) IsMonitoring() bool {
	r.mu.Lock()
	v := r.monitoring
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

// DetectAlive probes liveness avoiding races with os/exec internals.
func (r *Process) DetectAlive() (bool, string) {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()

	// First, try exec:pid detection if we have a command process
	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		// On Linux, a quickly-exiting child can be a zombie; treat that as not alive.
		if runtime.GOOS == "linux" {
			if isZombieLinux(pid) {
				return false, ""
			}
			if syscall.Kill(pid, 0) == nil {
				return true, "exec:pid"
			}
		} else {
			// Non-Linux: use process group signal check as before to handle quick-exit detection.
			if syscall.Kill(-pid, 0) == nil {
				return true, "exec:pid"
			}
		}
	}

	// If exec:pid detection fails or no process, try configured detectors
	for _, d := range r.detectors() {
		ok, _ := d.Alive()
		if ok {
			return true, d.Describe()
		}
	}
	return false, ""
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

// isZombieLinux returns true if /proc/<pid>/status reports a zombie state (Z) on Linux.
func isZombieLinux(pid int) bool {
	path := "/proc/" + strconv.Itoa(pid) + "/status"
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(b, []byte("State:\tZ"))
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
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		alive, _ := r.DetectAlive()
		if !alive {
			return errBeforeStart(d)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (r *Process) Stop(wait time.Duration) error {
	alive, _ := r.DetectAlive()
	if !alive {
		return nil
	}
	r.SetStopRequested(true)
	cmd := r.CopyCmd()
	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		if r.IsMonitoring() {
			// A supervisor/monitor goroutine is responsible for waiting and state transitions.
			// Wait on waitDone (closed by monitor) with timeout and escalate if needed.
			wd := r.WaitDoneChan()
			if wd != nil {
				select {
				case <-wd:
					// exited and reaped by monitor
				case <-time.After(wait):
					_ = syscall.Kill(-pid, syscall.SIGKILL)
					select {
					case <-wd:
						// reaped by monitor after kill
					case <-time.After(200 * time.Millisecond):
						// best-effort
					}
				}
			} else {
				// No wait channel available; fall back to brief sleep and continue.
				time.Sleep(wait)
			}
		} else {
			// No monitor observed. Try to claim monitoring to ensure single waiter.
			if r.MonitoringStartIfNeeded() {
				// We own the wait; perform it and finalize state.
				ch := make(chan error, 1)
				go func() {
					err := cmd.Wait()
					r.CloseWaitDone()
					r.MarkExited(err)
					ch <- err
				}()
				select {
				case <-ch:
					// done
				case <-time.After(wait):
					_ = syscall.Kill(-pid, syscall.SIGKILL)
					select {
					case <-ch:
						// reaped
					case <-time.After(200 * time.Millisecond):
						// best-effort
					}
				}
				// When we owned the wait, close writers and release monitoring flag.
				r.CloseWriters()
				r.MonitoringStop()
			} else {
				// Someone else claimed monitoring concurrently; wait on waitDone instead.
				wd := r.WaitDoneChan()
				if wd != nil {
					select {
					case <-wd:
						// reaped by monitor
					case <-time.After(wait):
						_ = syscall.Kill(-pid, syscall.SIGKILL)
						select {
						case <-wd:
							// reaped after kill
						case <-time.After(200 * time.Millisecond):
							// best-effort
						}
					}
				} else {
					// No wait channel; brief sleep as last resort.
					time.Sleep(wait)
				}
			}
		}
	}
	// If monitor owned the wait, writers will be closed in the supervisor.
	rs := r.Snapshot()
	return rs.ExitErr
}

// Kill sends SIGKILL to the process group and attempts to reap promptly.
func (r *Process) Kill() error {
	cmd := r.CopyCmd()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	if r.IsMonitoring() {
		// Let monitor reap; wait on waitDone briefly.
		wd := r.WaitDoneChan()
		if wd != nil {
			select {
			case <-wd:
				// reaped by monitor
			case <-time.After(200 * time.Millisecond):
				// best-effort
			}
		} else {
			time.Sleep(200 * time.Millisecond)
		}
		// writers will be closed by supervisor
	} else if r.MonitoringStartIfNeeded() {
		// We successfully claimed monitoring; we own the wait here.
		ch := make(chan error, 1)
		go func() {
			err := cmd.Wait()
			r.CloseWaitDone()
			r.MarkExited(err)
			ch <- err
		}()
		select {
		case <-ch:
			// ok
		case <-time.After(200 * time.Millisecond):
			// best-effort
		}
		// Close writers and release monitoring flag when we own the wait.
		r.CloseWriters()
		r.MonitoringStop()
	} else {
		// Someone else claimed monitoring concurrently.
		wd := r.WaitDoneChan()
		if wd != nil {
			select {
			case <-wd:
				// reaped by monitor
			case <-time.After(200 * time.Millisecond):
				// best-effort
			}
		} else {
			// last resort
			time.Sleep(200 * time.Millisecond)
		}
	}
	rs := r.Snapshot()
	return rs.ExitErr
}
