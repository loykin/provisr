package manager

import (
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
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
	err = syscall.Kill(initialPID, syscall.SIGKILL)
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
