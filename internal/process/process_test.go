package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/logger"
)

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("tests require sh/sleep on Unix-like systems")
	}
}

func TestTryStartWritesPIDAndStatus(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p1.pid")
	spec := Spec{Name: "p1", Command: "sleep 0.2", PIDFile: pidfile}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	st := r.Snapshot()
	if !st.Running || st.PID <= 0 || st.Name != "p1" {
		t.Fatalf("status not set after start: %+v", st)
	}
	b, err := os.ReadFile(pidfile)
	if err != nil || len(strings.TrimSpace(string(b))) == 0 {
		t.Fatalf("pidfile not written: %v, content=%q", err, string(b))
	}
}

func TestConfigureCmdAppliesEnvWorkdirLogging(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	_ = os.MkdirAll(work, 0o755)
	logs := filepath.Join(dir, "logs")

	spec := Spec{
		Name:    "cfg",
		Command: "sh -c 'echo out; echo err 1>&2; sleep 0.05'",
		WorkDir: work,
		Log:     logger.Config{File: logger.FileConfig{Dir: logs}},
	}
	r := New(spec)
	mergedEnv := []string{"FOO=bar"}
	cmd := r.ConfigureCmd(mergedEnv)

	if cmd.Dir != work {
		t.Fatalf("workdir not applied: got %q want %q", cmd.Dir, work)
	}
	if len(cmd.Env) != len(mergedEnv) || cmd.Env[0] != "FOO=bar" {
		t.Fatalf("env not applied: got %#v", cmd.Env)
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("SysProcAttr Setpgid not set")
	}

	// Start and let it produce logs
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Wait for process to exit and simulate monitor behavior to close waitDone
	c := r.CopyCmd()
	done := make(chan struct{})
	go func() {
		_ = c.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("process did not exit in time")
	}
	// Allow file buffers to flush
	time.Sleep(50 * time.Millisecond)

	outPath := filepath.Join(logs, "cfg.stdout.log")
	errPath := filepath.Join(logs, "cfg.stderr.log")
	ob, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	eb, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if !strings.Contains(string(ob), "out") {
		t.Fatalf("stdout missing content: %q", string(ob))
	}
	if !strings.Contains(string(eb), "err") {
		t.Fatalf("stderr missing content: %q", string(eb))
	}
}

func TestEnforceStartDurationEarlyExit(t *testing.T) {
	requireUnix(t)
	spec := Spec{Name: "early", Command: "sh -c 'sleep 0.05'"}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Wait for exit and mark exited
	c := r.CopyCmd()
	go func() {
		err := c.Wait()
		r.MarkExited(err)
	}()

	d := 200 * time.Millisecond
	start := time.Now()
	err := r.EnforceStartDuration(d)
	if err == nil || !IsBeforeStartErr(err) {
		t.Fatalf("expected before-start error, got: %v", err)
	}
	if time.Since(start) >= d {
		t.Fatalf("expected prompt failure before start duration, took %v", time.Since(start))
	}
}

func TestEnforceStartDurationSuccess(t *testing.T) {
	requireUnix(t)
	d := 150 * time.Millisecond
	spec := Spec{Name: "ok", Command: "sleep 0.3"}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Simulate monitor that will signal after the process exits (after 300ms),
	// so EnforceStartDuration should return on its deadline before that.
	c := r.CopyCmd()
	go func() {
		err := c.Wait()
		r.MarkExited(err)
	}()

	start := time.Now()
	if err := r.EnforceStartDuration(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < d-20*time.Millisecond {
		t.Fatalf("EnforceStartDuration returned too early: %v < %v", elapsed, d)
	}
}

func TestStopRequestedToggleAndIncRestarts(t *testing.T) {
	r := New(Spec{Name: "x", Command: "sleep 0.2"})
	if r.StopRequested() {
		t.Fatalf("default StopRequested should be false")
	}
	r.SetStopRequested(true)
	if !r.StopRequested() {
		t.Fatalf("StopRequested should be true after SetStopRequested(true)")
	}
	r.SetStopRequested(false)
	if r.StopRequested() {
		t.Fatalf("StopRequested should be false after SetStopRequested(false)")
	}
}

func TestCloseWritersAndRemovePIDFileAndDetectAlive(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p.pid")
	r := New(Spec{Name: "alive", Command: "sleep 0.3", PIDFile: pidfile})
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	// After start, PID file should exist
	if _, err := os.Stat(pidfile); err != nil {
		t.Fatalf("pidfile missing after start: %v", err)
	}
	// DetectAlive should report true via exec:pid
	if ok, src := r.DetectAlive(); !ok || !strings.Contains(src, "exec:pid") {
		t.Fatalf("DetectAlive expected true,exec:pid got %v,%q", ok, src)
	}
	// Close writers should be safe even if defaults (devnull) were used
	r.CloseWriters()
	// Stop process by sending SIGKILL to its pgid via syscall.Kill in manager is not available here;
	// instead wait for natural exit and then remove pid file.
	c := r.CopyCmd()
	_ = c.Process.Kill()
	_, _ = c.Process.Wait()
	r.MarkExited(nil)

	// RemovePIDFile should remove the file and be idempotent
	r.RemovePIDFile()
	if _, err := os.Stat(pidfile); !os.IsNotExist(err) {
		t.Fatalf("pidfile should be removed, stat err=%v", err)
	}
	r.RemovePIDFile() // second time should be no-op

	// Now DetectAlive should return false
	if ok, _ := r.DetectAlive(); ok {
		t.Fatalf("DetectAlive expected false after exit")
	}
}

func TestDetectorsAndUpdateSpec(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p.pid")
	r := New(Spec{Name: "d", Command: "sleep 0.2", PIDFile: pidfile})
	// with PIDFile set, detectors should include pidfile detector
	dets := r.detectors()
	if len(dets) == 0 {
		t.Fatalf("expected at least one detector")
	}
	found := false
	for _, d := range dets {
		if strings.HasPrefix(d.Describe(), "pidfile:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pidfile detector present")
	}
	// UpdateSpec should change fields used by ConfigureCmd
	work := filepath.Join(dir, "work")
	_ = os.MkdirAll(work, 0o755)
	r.UpdateSpec(Spec{Name: "d", Command: "sh -c 'exit 0'", WorkDir: work})
	cmd := r.ConfigureCmd([]string{"X=1"})
	if cmd.Dir != work {
		t.Fatalf("ConfigureCmd did not apply updated WorkDir: %q", cmd.Dir)
	}
	if len(cmd.Env) == 0 || cmd.Env[0] != "X=1" {
		t.Fatalf("ConfigureCmd did not apply merged env")
	}
	// Start to ensure nothing crashes with updated spec
	_ = r.TryStart(cmd)
	// Wait quickly
	c := r.CopyCmd()
	_ = c.Wait()
	r.MarkExited(nil)
	// ensure EnforceStartDuration with 0 is no-op
	if err := r.EnforceStartDuration(0); err != nil {
		t.Fatalf("EnforceStartDuration(0) unexpected err: %v", err)
	}
}

// waitUntilProc polls fn until it returns true or timeout expires.
func waitUntilProc(timeout, step time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(step)
	}
	return false
}

func TestProcessKillWithoutMonitor(t *testing.T) {
	requireUnix(t)
	r := New(Spec{Name: "kill-nomon", Command: "sleep 10"})
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	_ = r.Kill()
	if !waitUntilProc(1*time.Second, 20*time.Millisecond, func() bool { alive, _ := r.DetectAlive(); return !alive }) {
		t.Fatalf("expected process to be dead after Kill")
	}
}

func TestProcessDetectAliveParallel(t *testing.T) {
	requireUnix(t)
	r := New(Spec{Name: "alive-par", Command: "sleep 0.3"})
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for {
				alive, _ := r.DetectAlive()
				if !alive {
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}
	c := r.CopyCmd()
	_ = c.Wait()
	r.MarkExited(nil)
	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatalf("goroutine %d did not finish", i)
		}
	}
}

func TestEnforceStartDurationFail(t *testing.T) {
	spec := Spec{
		Name:    "test-fail",
		Command: "false", // 'false' command exits immediately with status 1
	}

	p := New(spec)

	// Start process (should succeed)
	var env []string
	cmd := p.ConfigureCmd(env)
	err := p.TryStart(cmd)
	if err != nil {
		t.Fatalf("TryStart should succeed: %v", err)
	}

	t.Logf("Process started with PID: %d", cmd.Process.Pid)

	// Wait a bit to see if process exits quickly
	time.Sleep(100 * time.Millisecond)

	alive, source := p.DetectAlive()
	t.Logf("After 100ms: DetectAlive=%v, source=%s", alive, source)

	// Enforce start duration (should fail because process exits immediately)
	err = p.EnforceStartDuration(50 * time.Millisecond)
	if err == nil {
		t.Fatalf("EnforceStartDuration should fail for quickly exiting process")
	}

	t.Logf("✅ Got expected error: %v", err)
}

// TestDetectAlive_ProcessLifecycle tests the complete lifecycle of process detection
func TestDetectAlive_ProcessLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name    string
		command string
		timeout time.Duration
	}{
		{
			name:    "simple_echo_process",
			command: "sh -c 'echo test process; sleep 5'",
			timeout: 10 * time.Second,
		},
		{
			name:    "long_running_process",
			command: "sh -c 'while true; do echo running; sleep 1; done'",
			timeout: 15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal process spec
			spec := Spec{
				Name:    tt.name,
				Command: tt.command,
			}

			// Create process instance
			proc := New(spec)

			// Phase 1: Start process and verify it's alive
			t.Log("Phase 1: Starting process and verifying alive detection")

			// Build and start command
			cmd := spec.BuildCommand()
			err := proc.TryStart(cmd)
			if err != nil {
				t.Fatalf("Failed to start process: %v", err)
			}
			proc.SetStarted(cmd)

			defer func() {
				if proc.cmd != nil && proc.cmd.Process != nil {
					_ = proc.cmd.Process.Kill()
				}
			}()

			// Wait a moment for process to fully start
			time.Sleep(100 * time.Millisecond)

			// Verify process is detected as alive
			alive, source := proc.DetectAlive()
			if !alive {
				t.Fatalf("Process should be alive after start, got alive=%v, source=%s", alive, source)
			}
			t.Logf("✓ Process correctly detected as alive: source=%s", source)

			// Get the PID for verification
			if proc.cmd == nil || proc.cmd.Process == nil {
				t.Fatal("Process command or process is nil")
			}
			pid := proc.cmd.Process.Pid
			t.Logf("Process PID: %d", pid)

			// Phase 2: Kill the process and verify it's detected as dead
			t.Log("Phase 2: Killing process and verifying dead detection")

			// Kill the process forcefully
			err = proc.cmd.Process.Kill()
			if err != nil {
				t.Fatalf("Failed to kill process: %v", err)
			}

			// Wait for process to die
			err = proc.cmd.Wait()
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					t.Logf("Process exited as expected: %v", exitErr)
				}
			}
			// Give some time for the system to clean up
			time.Sleep(200 * time.Millisecond)

			// Phase 3: Verify DetectAlive correctly identifies dead process
			t.Log("Phase 3: Verifying dead process detection")

			maxAttempts := 5
			var lastResult bool
			var lastSource string

			for i := 0; i < maxAttempts; i++ {
				alive, source = proc.DetectAlive()
				lastResult = alive
				lastSource = source

				t.Logf("Attempt %d: DetectAlive returned alive=%v, source=%s", i+1, alive, source)

				if !alive {
					t.Logf("✓ Process correctly detected as dead after %d attempts", i+1)
					break
				}

				// Wait a bit before retrying
				time.Sleep(100 * time.Millisecond)
			}

			if lastResult {
				// This is the critical failure - false positive detection
				t.Errorf("CRITICAL: DetectAlive shows false positive! Process PID %d is dead but DetectAlive returned alive=true, source=%s", pid, lastSource)

				// Additional verification using system tools
				t.Logf("Additional verification for PID %d:", pid)

				// Test with direct kill signal
				err := syscall.Kill(pid, 0)
				t.Logf("syscall.Kill(pid, 0) error: %v", err)

				// Test with ps command
				cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid))
				output, err := cmd.CombinedOutput()
				t.Logf("ps command output: %s, error: %v", string(output), err)
			}
		})
	}
}

// TestDetectAlive_FalsePositiveScenarios tests specific scenarios known to cause false positives
func TestDetectAlive_FalsePositiveScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Test the specific api-server command that's causing issues
	spec := Spec{
		Name:    "test-api-server",
		Command: "sh -c 'while true; do echo api-server running; sleep 2; done'",
	}

	proc := New(spec)

	// Start the process
	cmd := spec.BuildCommand()
	err := proc.TryStart(cmd)
	if err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	proc.SetStarted(cmd)

	defer func() {
		if proc.cmd != nil && proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
	}()

	// Wait for process to start
	time.Sleep(200 * time.Millisecond)

	// Verify alive
	alive, source := proc.DetectAlive()
	if !alive {
		t.Fatalf("Process should be alive, got alive=%v, source=%s", alive, source)
	}

	pid := proc.cmd.Process.Pid
	t.Logf("Started test process with PID: %d", pid)

	// Kill with SIGKILL
	t.Logf("Killing process PID %d with SIGKILL", pid)
	err = syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		t.Fatalf("Failed to send SIGKILL: %v", err)
	}

	// Wait for process to die
	err = proc.cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Logf("Process exited as expected: %v", exitErr)
		}
	}

	time.Sleep(2 * time.Second)

	// This is the critical test - should detect as dead
	alive, source = proc.DetectAlive()
	if alive {
		t.Errorf("FALSE POSITIVE DETECTED: PID %d is dead but DetectAlive returned alive=true, source=%s", pid, source)

		// Debug information
		t.Logf("OS: %s", runtime.GOOS)

		// Test raw syscall
		killErr := syscall.Kill(pid, 0)
		t.Logf("Raw syscall.Kill(%d, 0) error: %v", pid, killErr)

		// Test ps command
		psCmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid))
		psOutput, psErr := psCmd.CombinedOutput()
		t.Logf("ps -p %d output: %s, error: %v", pid, string(psOutput), psErr)
	} else {
		t.Logf("✓ Correctly detected dead process: alive=%v, source=%s", alive, source)
	}
}

// BenchmarkDetectAlive benchmarks the performance of DetectAlive
func BenchmarkDetectAlive(b *testing.B) {
	spec := Spec{
		Name:    "benchmark-process",
		Command: "sleep 10",
	}

	proc := New(spec)

	cmd := spec.BuildCommand()
	err := proc.TryStart(cmd)
	if err != nil {
		b.Fatalf("Failed to start process: %v", err)
	}
	proc.SetStarted(cmd)

	defer func() {
		if proc.cmd != nil && proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proc.DetectAlive()
	}
}
