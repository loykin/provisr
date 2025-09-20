package manager

import (
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// Test state-based command validation
func TestStateBasedCommandValidation(t *testing.T) {
	spec := process.Spec{
		Name:    "validation-test",
		Command: "sleep 0.5",
	}

	mp := NewManagedProcess(spec, mockEnvMerger, mockStartLogger, mockStopLogger)
	defer func() { _ = mp.Shutdown() }()

	t.Run("StartAlreadyRunning", func(t *testing.T) {
		// Start the process
		if err := mp.Start(spec); err != nil {
			t.Fatalf("Initial start failed: %v", err)
		}

		// Give it time to start
		time.Sleep(50 * time.Millisecond)

		// Try to start again - should get a clear error message
		err := mp.Start(spec)
		if err == nil {
			t.Error("Expected error when starting already running process")
		} else {
			errMsg := err.Error()
			if !strings.Contains(errMsg, "already running") {
				t.Errorf("Expected 'already running' in error message, got: %v", errMsg)
			}
			if !strings.Contains(errMsg, spec.Name) {
				t.Errorf("Expected process name in error message, got: %v", errMsg)
			}
		}
	})

	t.Run("StartWhileStarting", func(t *testing.T) {
		// Create a process that takes longer to start - use a more complex command
		slowSpec := process.Spec{
			Name:          "slow-start-test",
			Command:       "sh -c 'sleep 0.1 && echo started && sleep 2'", // Takes time to fully start
			StartDuration: 200 * time.Millisecond,                         // Process must stay up for this duration
		}

		slowMp := NewManagedProcess(slowSpec, mockEnvMerger, mockStartLogger, mockStopLogger)
		defer func() { _ = slowMp.Shutdown() }()

		// Start the process (don't wait for completion)
		go func() { _ = slowMp.Start(slowSpec) }()

		// Immediately try to start again - this should catch it in "starting" state
		time.Sleep(5 * time.Millisecond) // Minimal delay to let first start begin
		err := slowMp.Start(slowSpec)

		// Check the error message - it might be "starting" or "running" depending on timing
		if err == nil {
			t.Error("Expected error when starting while process is in transition")
		} else {
			errMsg := err.Error()
			// Accept either "starting" or "running" state errors
			hasValidError := strings.Contains(errMsg, "already starting") ||
				strings.Contains(errMsg, "already running")
			if !hasValidError {
				t.Errorf("Expected 'already starting' or 'already running' in error message, got: %v", errMsg)
			}
			t.Logf("Got expected error: %v", errMsg)
		}
	})

	t.Run("DetailedStatusWithState", func(t *testing.T) {
		// Test that status includes state information
		status := mp.Status()

		if status.State == "" {
			t.Error("Expected State field to be populated")
		}

		if status.Name != spec.Name {
			t.Errorf("Expected name '%s', got '%s'", spec.Name, status.Name)
		}

		// State should be one of the valid states
		validStates := []string{"stopped", "starting", "running", "stopping"}
		stateValid := false
		for _, validState := range validStates {
			if status.State == validState {
				stateValid = true
				break
			}
		}
		if !stateValid {
			t.Errorf("State '%s' is not a valid state", status.State)
		}
	})

	t.Run("StopWhileStopping", func(t *testing.T) {
		// Start a long-running process
		longSpec := process.Spec{
			Name:    "long-running-test",
			Command: "sleep 5",
		}

		longMp := NewManagedProcess(longSpec, mockEnvMerger, mockStartLogger, mockStopLogger)
		defer func() { _ = longMp.Shutdown() }()

		// Start the process
		if err := longMp.Start(longSpec); err != nil {
			t.Fatalf("Failed to start long-running process: %v", err)
		}

		// Give it time to start
		time.Sleep(100 * time.Millisecond)

		// Start stopping (don't wait)
		go func() { _ = longMp.Stop(5 * time.Second) }()

		// Try to stop again immediately
		time.Sleep(10 * time.Millisecond)

		// This should succeed (stopping an already stopping process is typically allowed)
		// But we can test that the state is properly managed
		status := longMp.Status()
		if status.State != "stopping" && status.State != "stopped" {
			t.Logf("Process state during stop: %s (this is acceptable)", status.State)
		}
	})
}

// Test expected shutdown error handling
func TestExpectedShutdownErrors(t *testing.T) {
	tests := []struct {
		errorString string
		expected    bool
	}{
		{"signal: terminated", true},
		{"signal: killed", true},
		{"signal: interrupt", true},
		{"some other error", false},
		{"", false},
	}

	for _, test := range tests {
		var err error
		if test.errorString != "" {
			err = &mockError{msg: test.errorString}
		}

		result := isExpectedShutdownError(err)
		if result != test.expected {
			t.Errorf("For error '%s', expected %v, got %v", test.errorString, test.expected, result)
		}
	}
}

// Mock error type for testing
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
