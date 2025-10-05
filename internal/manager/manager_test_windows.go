//go:build windows

package manager

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// TestStateBasedCommandValidation_Windows provides Windows-specific test implementation
func TestStateBasedCommandValidation_Windows(t *testing.T) {
	mockEnvMerger := func(spec process.Spec) []string { return spec.Env }

	// Use a very simple process for basic state validation
	spec := process.Spec{
		Name:    "state-test",
		Command: getTestCommand("state-test", 2), // 2 second process
	}

	mp := NewManagedProcess(spec, mockEnvMerger)
	defer func() { _ = mp.Shutdown() }()

	t.Run("StartAlreadyRunning_Windows", func(t *testing.T) {
		// Start the process
		if err := mp.Start(spec); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}

		// Wait a bit to ensure it's fully running
		time.Sleep(100 * time.Millisecond)

		// Try to start again - should get "already running" error
		err := mp.Start(spec)
		if err == nil {
			t.Error("Expected error when starting already running process")
		} else {
			errMsg := err.Error()
			if !strings.Contains(errMsg, "already running") {
				t.Errorf("Expected 'already running' in error message, got: %v", errMsg)
			}
			t.Logf("Got expected error: %v", errMsg)
		}
	})

	t.Run("StartWhileStarting_Windows", func(t *testing.T) {
		// Use a longer-running process for Windows
		slowSpec := process.Spec{
			Name:          "slow-start-test-windows",
			Command:       getTestCommand("slow-test", 10), // 10 second process
			StartDuration: 500 * time.Millisecond,          // Longer duration for Windows
		}

		slowMp := NewManagedProcess(slowSpec, mockEnvMerger)
		defer func() { _ = slowMp.Shutdown() }()

		// Start the process asynchronously
		startChan := make(chan error, 1)
		go func() {
			startChan <- slowMp.Start(slowSpec)
		}()

		// Wait longer for Windows process to get into starting state
		time.Sleep(50 * time.Millisecond)

		// Try to start again
		err := slowMp.Start(slowSpec)

		// Check for any kind of conflict error (starting or running)
		if err == nil {
			t.Error("Expected error when starting while process is in transition")
		} else {
			errMsg := err.Error()
			// On Windows, accept broader range of conflict errors
			hasValidError := strings.Contains(errMsg, "already starting") ||
				strings.Contains(errMsg, "already running") ||
				strings.Contains(errMsg, "in transition") ||
				strings.Contains(errMsg, "process exited before start duration")

			if !hasValidError {
				t.Errorf("Expected conflict error, got: %v", errMsg)
			}
			t.Logf("Got expected error: %v", errMsg)
		}

		// Wait for first start to complete or fail
		select {
		case startErr := <-startChan:
			t.Logf("First start completed with: %v", startErr)
		case <-time.After(2 * time.Second):
			t.Log("First start still running after 2 seconds")
		}
	})

	t.Run("DetailedStatusWithState_Windows", func(t *testing.T) {
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
			t.Errorf("Invalid state '%s', expected one of: %v", status.State, validStates)
		}

		t.Logf("Process state: %s", status.State)
	})

	t.Run("StopWhileStopping_Windows", func(t *testing.T) {
		// Start a process first
		if err := mp.Start(spec); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}

		// Wait for it to be running
		time.Sleep(100 * time.Millisecond)

		// Stop it asynchronously
		go func() { _ = mp.Stop(5 * time.Second) }()

		// Give Windows a moment to start stopping
		time.Sleep(20 * time.Millisecond)

		// Try to stop again
		err := mp.Stop(5 * time.Second)

		// This might succeed or fail depending on timing - both are acceptable on Windows
		if err != nil {
			errMsg := err.Error()
			// Accept various Windows-specific stop conflict errors
			acceptableErrors := []string{
				"already stopping",
				"already stopped",
				"not running",
			}

			hasAcceptableError := false
			for _, acceptableErr := range acceptableErrors {
				if strings.Contains(errMsg, acceptableErr) {
					hasAcceptableError = true
					break
				}
			}

			if !hasAcceptableError {
				t.Errorf("Unexpected error when stopping: %v", errMsg)
			}
			t.Logf("Got error: %v", errMsg)
		} else {
			t.Log("Second stop succeeded (acceptable on Windows)")
		}
	})
}

func init() {
	// Ensure this test only runs on Windows
	if runtime.GOOS != "windows" {
		panic("This test file should only be compiled on Windows")
	}
}
