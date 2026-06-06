package manager

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/core/internal/process"
)

// Mock functions for testing ManagedProcess
func mockEnvMerger(spec process.Spec) []string {
	return append([]string{"TEST_ENV=test"}, spec.Env...)
}

func TestNewManagedProcess(t *testing.T) {
	spec := process.Spec{
		Name:    "test-process",
		Command: "echo hello",
	}

	mp := NewManagedProcess(
		spec,
		mockEnvMerger,
	)

	if mp == nil {
		t.Fatal("NewManagedProcess returned nil")
	}

	status := mp.Status()
	if status.Name != "test-process" {
		t.Errorf("Expected name 'test-process', got '%s'", status.Name)
	}

	if status.State != "stopped" {
		t.Errorf("Expected initial state 'stopped', got '%s'", status.State)
	}
}

func TestManagedProcessStatus(t *testing.T) {
	spec := process.Spec{
		Name:    "status-test",
		Command: "echo hello",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)

	// Test initial status
	status := mp.Status()
	if status.Name != "status-test" {
		t.Errorf("Expected name 'status-test', got '%s'", status.Name)
	}

	if status.Running {
		t.Error("Expected Running=false for new process")
	}

	if status.Restarts != 0 {
		t.Errorf("Expected Restarts=0, got %d", status.Restarts)
	}
}

func TestManagedProcessStartStop(t *testing.T) {
	spec := process.Spec{
		Name:    "start-stop-test",
		Command: "sleep 0.1",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()
	// Test start
	err := mp.Start(spec)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify state changed
	status := mp.Status()
	if !status.Running {
		t.Error("Expected Running=true after start")
	}

	if status.PID == 0 {
		t.Error("Expected non-zero PID after start")
	}

	// Test stop
	err = mp.Stop(3 * time.Second)
	if err != nil {
		t.Logf("Stop result: %v", err) // May fail if process already exited
	}
}

func TestManagedProcessUpdateSpec(t *testing.T) {
	spec := process.Spec{
		Name:    "update-test",
		Command: "echo original",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)

	// Test updating spec
	newSpec := process.Spec{
		Name:    "update-test",
		Command: "echo updated",
		Env:     []string{"UPDATED=true"},
	}

	_ = mp.UpdateSpec(newSpec)

	// We can't directly test the internal spec update,
	// but we can verify the method doesn't panic
}

func TestManagedProcessStateMachine(t *testing.T) {
	spec := process.Spec{
		Name:    "state-test",
		Command: "sleep 0.05",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	// Initial state should be Stopped
	status := mp.Status()
	if status.State != "stopped" {
		t.Errorf("Expected initial state 'stopped', got '%s'", status.State)
	}

	// Start process
	err := mp.Start(spec)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// State should transition through Starting to Running
	// Note: This is timing-dependent, so we'll just check it's not Stopped
	currentStatus := mp.Status()
	if currentStatus.State == "stopped" {
		t.Error("Expected state to change from Stopped after start")
	}

	// Wait for process to potentially complete
	time.Sleep(100 * time.Millisecond)
}

func TestManagedProcessConcurrentOperations(t *testing.T) {
	spec := process.Spec{
		Name:    "concurrent-test",
		Command: "sleep 0.1",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	// Start multiple goroutines to test concurrent access
	done := make(chan bool, 3)

	// Goroutine 1: Start
	go func() {
		err := mp.Start(spec)
		if err != nil {
			t.Logf("Concurrent start: %v", err)
		}
		done <- true
	}()

	// Goroutine 2: Check Status
	go func() {
		for i := 0; i < 5; i++ {
			status := mp.Status()
			t.Logf("Concurrent status: %s, Running: %v", status.Name, status.Running)
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 3: Update Spec
	go func() {
		time.Sleep(20 * time.Millisecond)
		newSpec := process.Spec{
			Name:    "concurrent-test",
			Command: "sleep 0.05",
		}
		_ = mp.UpdateSpec(newSpec)
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestManagedProcessShutdown(t *testing.T) {
	spec := process.Spec{
		Name:    "shutdown-test",
		Command: "sleep 10",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)

	// Start process
	err := mp.Start(spec)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Test shutdown
	err = mp.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify process is stopped
	status := mp.Status()
	if status.Running {
		t.Error("Expected process to be stopped after shutdown")
	}
}

func TestManagedProcessQuickCommands(t *testing.T) {
	spec := process.Spec{
		Name:    "quick-test",
		Command: "true", // Very quick command
	}

	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	// Test starting a quick command
	err := mp.Start(spec)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for command to complete
	time.Sleep(100 * time.Millisecond)

	// Check final status
	status := mp.Status()
	t.Logf("Quick command final status: Running=%v, PID=%d", status.Running, status.PID)
}

func TestManagedProcessMultipleStarts(t *testing.T) {
	spec := process.Spec{
		Name:    "multi-start-test",
		Command: "sleep 0.05",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	// First start
	err := mp.Start(spec)
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	// Second start (should handle gracefully)
	err = mp.Start(spec)
	if err == nil {
		t.Log("Second start succeeded (process may have already stopped)")
	} else {
		t.Logf("Second start failed as expected: %v", err)
	}
}

func TestManagedProcessStopNonRunning(t *testing.T) {
	spec := process.Spec{
		Name:    "stop-non-running-test",
		Command: "echo hello",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)

	// Try to stop a process that was never started - should be no-op, not error
	err := mp.Stop(1 * time.Second)
	if err != nil {
		t.Errorf("Unexpected error when stopping non-running process: %v", err)
	}
}

func TestManagedProcessStateConstants(t *testing.T) {
	// Test that state constants are defined
	states := []processState{
		StateStopped,
		StateStarting,
		StateRunning,
		StateStopping,
	}

	// Verify they have different values
	stateSet := make(map[processState]bool)
	for _, state := range states {
		if stateSet[state] {
			t.Errorf("Duplicate state value: %v", state)
		}
		stateSet[state] = true
	}

	if len(stateSet) != 4 {
		t.Errorf("Expected 4 unique states, got %d", len(stateSet))
	}

	// Test String() method
	expectedStrings := map[processState]string{
		StateStopped:  "stopped",
		StateStarting: "starting",
		StateRunning:  "running",
		StateStopping: "stopping",
	}

	for state, expected := range expectedStrings {
		if state.String() != expected {
			t.Errorf("State %v: expected string '%s', got '%s'", state, expected, state.String())
		}
	}
}

// TestDetectAliveFalsePositiveInManager tests for false positives in the manager context
func TestDetectAliveFalsePositiveInManager(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows - see TestDetectAliveFalsePositiveInManager_Windows")
	}

	spec := process.Spec{
		Name:        "test-false-positive",
		Command:     "sh -c 'echo starting; sleep 10'",
		AutoRestart: false, // Don't auto-restart for this test
	}

	envMerger := func(spec process.Spec) []string { return spec.Env }

	mp := NewManagedProcess(spec, envMerger)
	defer func() { _ = mp.Stop(5 * time.Second) }()

	// Start the process
	startCmd := command{action: ActionStart, spec: spec, reply: make(chan error, 1)}
	mp.cmdChan <- startCmd
	err := <-startCmd.reply
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify alive
	status := mp.Status()
	if !status.Running {
		t.Fatalf("Process should be running, got status: %+v", status)
	}

	pid := status.PID
	t.Logf("Started process with PID: %d", pid)

	err = killProcessByPID(pid)
	if err != nil {
		t.Fatalf("Failed to kill process: %v", err)
	}

	// Wait for process to die
	time.Sleep(5 * time.Second)

	// Test DetectAlive multiple times to ensure consistency
	mp.mu.RLock()
	proc := mp.proc
	mp.mu.RUnlock()

	if proc != nil {
		for i := 0; i < 5; i++ {
			alive, source := proc.DetectAlive()
			t.Logf("DetectAlive attempt %d: alive=%v, source=%s", i+1, alive, source)

			if alive {
				t.Errorf("FALSE POSITIVE DETECTED on attempt %d: PID %d is dead but DetectAlive returned alive=true, source=%s", i+1, pid, source)
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}

// TestManagedProcessNoAutoRestart tests that processes without auto-restart don't restart
func TestManagedProcessNoAutoRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping no auto-restart test")
	}

	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows - see TestManagedProcessNoAutoRestart_Windows")
	}

	spec := process.Spec{
		Name:        "test-no-restart",
		Command:     "sh -c 'echo no-restart-test; sleep 5'",
		AutoRestart: false, // Explicitly disable auto-restart
	}

	envMerger := func(spec process.Spec) []string { return spec.Env }

	mp := NewManagedProcess(spec, envMerger)
	defer func() { _ = mp.Stop(2 * time.Second) }()

	// Start the process
	startCmd := command{action: ActionStart, spec: spec, reply: make(chan error, 1)}
	mp.cmdChan <- startCmd
	err := <-startCmd.reply
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Get initial state
	status := mp.Status()
	if !status.Running {
		t.Fatalf("Process should be running initially")
	}

	initialPID := status.PID
	initialRestarts := status.Restarts
	t.Logf("Initial state: PID=%d, Restarts=%d", initialPID, initialRestarts)

	// Kill the process
	err = killProcessByPID(initialPID)
	if err != nil {
		t.Fatalf("Failed to kill process: %v", err)
	}
	t.Logf("Killed process PID %d", initialPID)

	// Wait and verify NO restart occurs
	time.Sleep(5 * time.Second)

	finalStatus := mp.Status()
	t.Logf("Final state: PID=%d, Running=%v, Restarts=%d",
		finalStatus.PID, finalStatus.Running, finalStatus.Restarts)

	// Process should be dead and not restarted
	if finalStatus.Running {
		t.Errorf("Process should NOT restart when AutoRestart=false, but it's still running with PID %d", finalStatus.PID)
	}

	if finalStatus.Restarts != initialRestarts {
		t.Errorf("Restart count should remain %d, got %d", initialRestarts, finalStatus.Restarts)
	}

	t.Logf("✓ Correctly did NOT auto-restart when AutoRestart=false")
}

// TestStopSIGTERMIgnoredFallsBackToSIGKILL verifies that ManagedProcess.Stop(wait)
// force-kills a SIGTERM-ignoring process within the wait window and reports StateStopped.
func TestStopSIGTERMIgnoredFallsBackToSIGKILL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling not applicable on Windows")
	}

	spec := process.Spec{
		Name:    "sigterm-ignore-manager",
		Command: "trap '' TERM; sleep 30",
	}
	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	if err := mp.Start(spec); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if ok := waitUntilState(t, mp, "running", 5*time.Second); !ok {
		t.Fatal("process did not reach running state")
	}

	// Stop with a short wait window; doStop must escalate to SIGKILL
	if err := mp.Stop(500 * time.Millisecond); err != nil {
		t.Fatalf("Stop returned unexpected error: %v", err)
	}

	st := mp.Status()
	if st.Running {
		t.Error("process still running after Stop()")
	}
	if st.State != "stopped" {
		t.Errorf("expected state 'stopped', got %q", st.State)
	}
}

// TestRapidStopStartNoStateCorruption verifies that rapid stop/start cycles do not
// allow a stale cmd.Wait() goroutine to corrupt the new process's state.
func TestRapidStopStartNoStateCorruption(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling not applicable on Windows")
	}

	spec := process.Spec{Name: "rapid-manager", Command: "sleep 30"}
	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	for i := 0; i < 3; i++ {
		if err := mp.Start(spec); err != nil {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}
		if ok := waitUntilState(t, mp, "running", 5*time.Second); !ok {
			t.Fatalf("iteration %d: process did not reach running state", i)
		}
		if err := mp.Stop(2 * time.Second); err != nil {
			t.Fatalf("iteration %d: Stop: %v", i, err)
		}
	}

	// After all cycles the process must be stopped, not corrupted into a false running state
	st := mp.Status()
	if st.Running {
		t.Error("process reported running after final stop — stale goroutine may have corrupted state")
	}
	if st.State != "stopped" {
		t.Errorf("expected state 'stopped', got %q", st.State)
	}
}

func waitUntilState(t *testing.T, mp *ManagedProcess, want string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if mp.Status().State == want {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// TestSetStartedResetsstopRequested verifies that SetStarted() clears the stopRequested
// flag on each new start, so that auto-restart and failed labelling work correctly after
// a normal stop/start cycle.
// Note: the Stop-failure-while-alive branch (SetStopRequested(false) in doStop) requires
// signal injection to test and is not covered here.
// TestStopZeroWaitSIGTERMIgnoredFallsBackToSIGKILL verifies that Stop(0) escalates to
// SIGKILL when the process ignores SIGTERM and ultimately records StateStopped.
func TestStopZeroWaitSIGTERMIgnoredFallsBackToSIGKILL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling not applicable on Windows")
	}

	spec := process.Spec{
		Name:    "stop-zero-sigterm-ignore",
		Command: "trap '' TERM; sleep 30",
	}
	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	if err := mp.Start(spec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if ok := waitUntilState(t, mp, "running", 5*time.Second); !ok {
		t.Fatal("process did not reach running state")
	}

	if err := mp.Stop(0); err != nil {
		t.Fatalf("Stop(0) returned unexpected error: %v", err)
	}

	st := mp.Status()
	if st.Running {
		t.Error("process still running after Stop(0)")
	}
	if st.State != "stopped" {
		t.Errorf("expected state 'stopped', got %q", st.State)
	}
}

func TestSetStartedResetsStopRequested(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal handling not applicable on Windows")
	}

	spec := process.Spec{
		Name:    "stop-req-reset",
		Command: "sleep 30",
	}
	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	if err := mp.Start(spec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if ok := waitUntilState(t, mp, "running", 5*time.Second); !ok {
		t.Fatal("process did not reach running state")
	}

	if err := mp.Stop(3 * time.Second); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// stopRequested is true after stop — expected
	if !mp.proc.StopRequested() {
		t.Error("stopRequested should be true after stop")
	}

	// Second start: SetStarted resets stopRequested to false
	if err := mp.Start(spec); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if ok := waitUntilState(t, mp, "running", 5*time.Second); !ok {
		t.Fatal("process did not restart")
	}

	if mp.proc.StopRequested() {
		t.Error("stopRequested should be false after new start — auto-restart and failed labelling would be broken")
	}
}

// --- Manager.Recover tests ---

// TestManagerRecoverAliveProcess verifies that Recover marks a still-running
// process as Running without restarting it.
func TestManagerRecoverAliveProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix signals")
	}

	dir := t.TempDir()
	pidFile := dir + "/nb.pid"

	spec := process.Spec{
		Name:        "nb-recover",
		Command:     "sleep 10",
		PIDFile:     pidFile,
		AutoRestart: false,
	}

	// Start the process so provisr writes the PID file.
	mgr1 := NewManager()
	if err := mgr1.Register(spec); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if ok := waitUntilManagerState(t, mgr1, spec.Name, "running", 3*time.Second); !ok {
		t.Fatal("process did not reach running state")
	}

	// Simulate provisr restart: new Manager, call Recover.
	mgr2 := NewManager()
	if err := mgr2.Recover(spec); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	st, err := mgr2.Status(spec.Name)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Running {
		t.Errorf("expected Running=true after Recover of alive process, got %+v", st)
	}

	// Cleanup
	_ = mgr1.Stop(spec.Name, 3*time.Second)
	_ = mgr2.Stop(spec.Name, 3*time.Second)
}

// TestManagerRecoverDeadProcess verifies that Recover marks a dead process as
// Stopped and does not restart it.
func TestManagerRecoverDeadProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix signals")
	}

	dir := t.TempDir()
	pidFile := dir + "/dead.pid"

	spec := process.Spec{
		Name:        "dead-recover",
		Command:     "sleep 10",
		PIDFile:     pidFile,
		AutoRestart: false,
	}

	// Start then kill the process externally so provisr cannot remove the PID
	// file. This reproduces the stale file left after a manager crash.
	mgr1 := NewManager()
	if err := mgr1.Register(spec); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if ok := waitUntilManagerState(t, mgr1, spec.Name, "running", 3*time.Second); !ok {
		t.Fatal("process did not reach running state")
	}

	st, err := mgr1.Status(spec.Name)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	stalePIDFile, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := syscall.Kill(-st.PID, syscall.SIGKILL); err != nil {
		t.Fatalf("external SIGKILL: %v", err)
	}
	if ok := waitUntilManagerState(t, mgr1, spec.Name, "stopped", 3*time.Second); !ok {
		t.Fatal("externally killed process did not reach stopped state")
	}
	// A live manager observes cmd.Wait and removes the file. Restore the exact
	// bytes captured before the kill to model a manager crash, where that
	// cleanup callback never runs.
	if err := os.WriteFile(pidFile, stalePIDFile, 0o600); err != nil {
		t.Fatalf("restore stale PID file: %v", err)
	}

	// New Manager: Recover should mark Stopped, not restart.
	mgr2 := NewManager()
	if err := mgr2.Recover(spec); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	// Give health check a moment — it must NOT restart.
	time.Sleep(200 * time.Millisecond)

	st, err = mgr2.Status(spec.Name)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Running {
		t.Errorf("expected Running=false after Recover of dead process, got %+v", st)
	}
}

// TestManagerRecoverNoPIDFile verifies that Recover with no PID file registers
// the process as Stopped without error.
func TestManagerRecoverNoPIDFile(t *testing.T) {
	spec := process.Spec{
		Name:    "no-pid-recover",
		Command: "sleep 10",
	}

	mgr := NewManager()
	if err := mgr.Recover(spec); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	st, err := mgr.Status(spec.Name)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Running {
		t.Errorf("expected Running=false when no PID file, got %+v", st)
	}
}

// TestManagerRecoverPIDReused verifies that Recover treats a PID file as stale
// when the recorded start_unix does not match the actual running process,
// preventing incorrect adoption of an unrelated process that reused the PID.
func TestManagerRecoverPIDReused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix process start-time detection")
	}

	dir := t.TempDir()
	pidFile := dir + "/reused.pid"

	// Write a PID file that points to this test process (which is definitely running)
	// but with start_unix=1 — a value that can never match any real process started
	// in the Unix era.  This simulates a PID that has been reused.
	livePID := os.Getpid()
	content := fmt.Sprintf("%d\n{}\n{\"start_unix\":1}\n", livePID)
	if err := os.WriteFile(pidFile, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	spec := process.Spec{
		Name:        "pid-reuse-recover",
		Command:     "sleep 10",
		PIDFile:     pidFile,
		AutoRestart: false,
	}

	mgr := NewManager()
	if err := mgr.Recover(spec); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	st, err := mgr.Status(spec.Name)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Running {
		t.Errorf("expected Running=false when PID is reused, got %+v", st)
	}
}

func waitUntilManagerState(t *testing.T, mgr *Manager, name, want string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := mgr.Status(name)
		if err == nil && st.State == want {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
