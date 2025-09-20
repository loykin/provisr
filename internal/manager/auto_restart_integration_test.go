package manager

import (
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// TestAutoRestartIntegration tests the complete auto-restart flow
func TestAutoRestartIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	// ManagedProcess-level auto-restart has been removed; manager-only restart is covered in TestManagerRestartOnly.
	t.Skip("deprecated: auto-restart is manager-responsibility now")
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a test spec similar to api-server
	spec := process.Spec{
		Name:            "test-auto-restart",
		Command:         "sh -c 'while true; do echo test-server running; sleep 1; done'",
		AutoRestart:     true,
		RestartInterval: 500 * time.Millisecond,
	}

	// Create managed process using the actual constructor
	envMerger := func(spec process.Spec) []string { return spec.Env }
	startLogger := func(proc *process.Process) {}
	stopLogger := func(proc *process.Process, err error) {}

	mp := NewManagedProcess(spec, envMerger, startLogger, stopLogger)
	defer mp.Stop(5 * time.Second)

	// Start the managed process
	startCmd := command{action: ActionStart, spec: spec, reply: make(chan error, 1)}
	mp.cmdChan <- startCmd
	err := <-startCmd.reply
	if err != nil {
		t.Fatalf("Failed to start managed process: %v", err)
	}

	// Wait for startup
	time.Sleep(500 * time.Millisecond)

	// Phase 1: Verify process is running
	t.Log("Phase 1: Verifying process starts correctly")

	status := mp.Status()
	if !status.Running {
		t.Fatalf("Process should be running, got status: %+v", status)
	}

	initialPID := status.PID
	initialRestarts := status.Restarts
	t.Logf("✓ Process started with PID: %d, restarts: %d", initialPID, initialRestarts)

	// Phase 2: Kill the process and verify auto-restart
	t.Log("Phase 2: Killing process and verifying auto-restart")

	// Kill the process
	err = syscall.Kill(initialPID, syscall.SIGKILL)
	if err != nil {
		t.Fatalf("Failed to kill process PID %d: %v", initialPID, err)
	}
	t.Logf("✓ Killed process PID %d", initialPID)

	// Wait for auto-restart (should happen within restart interval + buffer)
	maxWaitTime := spec.RestartInterval + 3*time.Second
	waitStart := time.Now()

	var restarted bool
	var newPID int
	var newRestarts int

	for time.Since(waitStart) < maxWaitTime {
		status := mp.Status()

		// Check if we have a new PID and restart count increased
		if status.Running && status.PID != initialPID && status.PID > 0 && status.Restarts > initialRestarts {
			restarted = true
			newPID = status.PID
			newRestarts = status.Restarts
			break
		}

		t.Logf("Waiting for restart... Current status: PID=%d, Running=%v, Restarts=%d",
			status.PID, status.Running, status.Restarts)

		time.Sleep(200 * time.Millisecond)
	}

	if !restarted {
		t.Errorf("CRITICAL: Auto-restart failed! Process was not restarted within %v", maxWaitTime)

		// Debug information
		finalStatus := mp.Status()
		t.Logf("Final status: %+v", finalStatus)

		// Check if the process is actually detected as dead
		mp.mu.RLock()
		proc := mp.proc
		mp.mu.RUnlock()

		if proc != nil {
			alive, source := proc.DetectAlive()
			t.Logf("DetectAlive result: alive=%v, source=%s", alive, source)
		}

		t.FailNow()
	}

	t.Logf("✓ Process successfully restarted with new PID: %d (restarts: %d)", newPID, newRestarts)

	// Phase 3: Verify new process is actually running and healthy
	t.Log("Phase 3: Verifying new process is healthy")

	mp.mu.RLock()
	proc := mp.proc
	mp.mu.RUnlock()

	if proc != nil {
		alive, source := proc.DetectAlive()
		if !alive {
			t.Errorf("New process should be alive, got alive=%v, source=%s", alive, source)
		} else {
			t.Logf("✓ New process is healthy: alive=%v, source=%s", alive, source)
		}
	}
}

// TestDetectAliveFalsePositiveInManager tests for false positives in the manager context
func TestDetectAliveFalsePositiveInManager(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	spec := process.Spec{
		Name:        "test-false-positive",
		Command:     "sh -c 'echo starting; sleep 10'",
		AutoRestart: false, // Don't auto-restart for this test
	}

	envMerger := func(spec process.Spec) []string { return spec.Env }
	startLogger := func(proc *process.Process) {}
	stopLogger := func(proc *process.Process, err error) {}

	mp := NewManagedProcess(spec, envMerger, startLogger, stopLogger)
	defer mp.Stop(5 * time.Second)

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

	// Kill it
	err = syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		t.Fatalf("Failed to kill process: %v", err)
	}

	// Wait for process to die
	time.Sleep(500 * time.Millisecond)

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
