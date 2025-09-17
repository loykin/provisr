package manager

import (
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// Mock functions for testing ManagedProcess
func mockEnvMerger(spec process.Spec) []string {
	return append([]string{"TEST_ENV=test"}, spec.Env...)
}

func mockStartLogger(_ *process.Process) {
	// Mock start logging
}

func mockStopLogger(_ *process.Process, _ error) {
	// Mock stop logging
}

func TestNewManagedProcess(t *testing.T) {
	spec := process.Spec{
		Name:    "test-process",
		Command: "echo hello",
	}

	mp := NewManagedProcess(
		spec,
		mockEnvMerger,
		mockStartLogger,
		mockStopLogger,
	)

	if mp == nil {
		t.Fatal("NewManagedProcess returned nil")
	}

	if mp.name != "test-process" {
		t.Errorf("Expected name 'test-process', got '%s'", mp.name)
	}

	if mp.state != StateStopped {
		t.Errorf("Expected initial state StateStopped, got %v", mp.state)
	}
}

func TestManagedProcessStatus(t *testing.T) {
	spec := process.Spec{
		Name:    "status-test",
		Command: "echo hello",
	}

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)

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

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)
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

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)

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

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)
	defer func() { _ = mp.Shutdown() }()

	// Initial state should be Stopped
	if mp.state != StateStopped {
		t.Errorf("Expected initial state StateStopped, got %v", mp.state)
	}

	// Start process
	err := mp.Start(spec)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// State should transition through Starting to Running
	// Note: This is timing-dependent, so we'll just check it's not Stopped
	if mp.state == StateStopped {
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

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)
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
		Command: "sleep 0.2",
	}

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)

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

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)
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

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)
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

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)

	// Try to stop a process that was never started
	err := mp.Stop(1 * time.Second)
	if err == nil {
		t.Error("Expected error when stopping non-running process")
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
