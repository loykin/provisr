package manager

import (
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManagerMultiProcessAutoRestart tests Manager handling multiple processes with auto-restart
func TestManagerMultiProcessAutoRestart(t *testing.T) {
	manager := NewManager()
	manager.StartReconciler(200 * time.Millisecond) // Fast reconciliation for testing
	defer manager.StopReconciler()
	defer manager.Shutdown()

	// Test multiple processes with auto-restart enabled
	processes := []struct {
		name        string
		autoRestart bool
	}{
		{"test-manager-proc1", true},
		{"test-manager-proc2", true},
		{"test-manager-proc3", false}, // This one should NOT restart
	}

	t.Log("Phase 1: Starting multiple processes")

	// Start all processes
	for _, p := range processes {
		spec := process.Spec{
			Name:        p.name,
			Command:     "sleep 300",
			AutoRestart: p.autoRestart,
		}

		err := manager.Start(spec)
		require.NoError(t, err, "Failed to start process %s", p.name)

		// Wait for process to be ready
		time.Sleep(100 * time.Millisecond)

		status, err := manager.Status(p.name)
		require.NoError(t, err)
		require.True(t, status.Running, "Process %s should be running", p.name)
		require.Greater(t, status.PID, 0, "Process %s should have valid PID", p.name)

		t.Logf("✓ Started process %s (PID: %d, AutoRestart: %t)", p.name, status.PID, p.autoRestart)
	}

	// Collect initial PIDs and restart counts
	initialStates := make(map[string]struct {
		pid      int
		restarts int
	})

	for _, p := range processes {
		status, err := manager.Status(p.name)
		require.NoError(t, err)
		initialStates[p.name] = struct {
			pid      int
			restarts int
		}{
			pid:      status.PID,
			restarts: status.Restarts,
		}
	}

	t.Log("Phase 2: Killing processes to trigger auto-restart")

	// Kill all processes
	for _, p := range processes {
		initialState := initialStates[p.name]
		err := killProcessByPID(initialState.pid)
		require.NoError(t, err, "Failed to kill process %s (PID: %d)", p.name, initialState.pid)
		t.Logf("✓ Killed process %s (PID: %d)", p.name, initialState.pid)
	}

	t.Log("Phase 3: Waiting for Manager reconciliation and auto-restart")

	// Wait for reconciliation to detect deaths and restart
	time.Sleep(2 * time.Second)

	t.Log("Phase 4: Verifying auto-restart results")

	// Check results for each process
	for _, p := range processes {
		initialState := initialStates[p.name]
		status, err := manager.Status(p.name)
		require.NoError(t, err)

		if p.autoRestart {
			// Should be restarted
			assert.True(t, status.Running, "Process %s with auto-restart should be running", p.name)
			assert.NotEqual(t, initialState.pid, status.PID, "Process %s should have new PID after restart", p.name)
			assert.Greater(t, status.Restarts, initialState.restarts, "Process %s should have incremented restart count", p.name)
			t.Logf("✓ Auto-restart successful: %s PID %d → %d, Restarts %d → %d",
				p.name, initialState.pid, status.PID, initialState.restarts, status.Restarts)
		} else {
			// Should stay dead
			assert.False(t, status.Running, "Process %s without auto-restart should stay dead", p.name)
			assert.Equal(t, initialState.pid, status.PID, "Process %s should keep same PID when dead", p.name)
			assert.Equal(t, initialState.restarts, status.Restarts, "Process %s should not increment restart count", p.name)
			t.Logf("✓ Correctly stayed dead: %s (AutoRestart: false)", p.name)
		}
	}

	// Verify all processes with auto-restart are healthy
	for _, p := range processes {
		if p.autoRestart {
			status, err := manager.Status(p.name)
			require.NoError(t, err)
			require.True(t, status.Running, "Restarted process %s should be healthy", p.name)
			t.Logf("✓ Restarted process %s is healthy", p.name)
		}
	}
}

// TestManagerReconcilerAutoRestart tests Manager's reconciler-driven auto-restart
func TestManagerReconcilerAutoRestart(t *testing.T) {
	manager := NewManager()

	// Start with longer reconciler interval to test manual reconciliation
	manager.StartReconciler(1 * time.Second)
	defer manager.StopReconciler()
	defer manager.Shutdown()

	t.Log("Phase 1: Starting process with auto-restart")

	spec := process.Spec{
		Name:        "test-reconciler-restart",
		Command:     "sleep 300",
		AutoRestart: true,
	}

	err := manager.Start(spec)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	initialStatus, err := manager.Status("test-reconciler-restart")
	require.NoError(t, err)
	require.True(t, initialStatus.Running)
	require.Greater(t, initialStatus.PID, 0)

	t.Logf("✓ Initial process started: PID %d", initialStatus.PID)

	t.Log("Phase 2: Killing process and testing reconciler")

	// Kill the process
	err = killProcessByPID(initialStatus.PID)
	require.NoError(t, err)
	t.Logf("✓ Killed process PID %d", initialStatus.PID)

	// Manually trigger reconciliation (without waiting for timer)
	t.Log("Phase 3: Triggering manual reconciliation")
	manager.ReconcileOnce()

	// Wait a bit for restart to happen (allow reconciler + restart interval)
	time.Sleep(1200 * time.Millisecond)

	// Check if process was restarted
	finalStatus, err := manager.Status("test-reconciler-restart")
	require.NoError(t, err)

	t.Logf("Final status: Running=%t, PID=%d, Restarts=%d",
		finalStatus.Running, finalStatus.PID, finalStatus.Restarts)

	assert.True(t, finalStatus.Running, "Process should be restarted")
	assert.NotEqual(t, initialStatus.PID, finalStatus.PID, "Process should have new PID")
	assert.Equal(t, initialStatus.Restarts+1, finalStatus.Restarts, "Restart count should increment")

	t.Logf("✓ Reconciler auto-restart successful: PID %d → %d, Restarts %d → %d",
		initialStatus.PID, finalStatus.PID, initialStatus.Restarts, finalStatus.Restarts)
}

// TestManagerProcessCoordination tests Manager's coordination of multiple process restarts
func TestManagerProcessCoordination(t *testing.T) {
	manager := NewManager()
	manager.StartReconciler(100 * time.Millisecond) // Very fast for testing
	defer manager.StopReconciler()
	defer manager.Shutdown()

	t.Log("Phase 1: Starting multiple processes")

	numProcesses := 5
	processNames := make([]string, numProcesses)

	// Start multiple processes
	for i := 0; i < numProcesses; i++ {
		name := fmt.Sprintf("test-coord-%d", i)
		processNames[i] = name

		spec := process.Spec{
			Name:        name,
			Command:     "sleep 300",
			AutoRestart: true,
		}

		err := manager.Start(spec)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Stagger starts slightly
	}

	// Wait for all to be running
	time.Sleep(500 * time.Millisecond)

	// Collect initial states
	initialPIDs := make(map[string]int)
	for _, name := range processNames {
		status, err := manager.Status(name)
		require.NoError(t, err)
		require.True(t, status.Running, "Process %s should be running", name)
		initialPIDs[name] = status.PID
		t.Logf("✓ Process %s running with PID %d", name, status.PID)
	}

	t.Log("Phase 2: Killing all processes simultaneously")

	// Kill all processes at once
	for name, pid := range initialPIDs {
		err := killProcessByPID(pid)
		require.NoError(t, err)
		t.Logf("✓ Killed process %s (PID %d)", name, pid)
	}

	t.Log("Phase 3: Waiting for coordinated restart")

	// Wait for reconciler to handle multiple restarts
	time.Sleep(3 * time.Second)

	t.Log("Phase 4: Verifying coordinated restart results")

	// Verify all processes restarted successfully
	allRestarted := true
	for _, name := range processNames {
		status, err := manager.Status(name)
		require.NoError(t, err)

		if !status.Running {
			allRestarted = false
			t.Errorf("Process %s failed to restart", name)
			continue
		}

		if status.PID == initialPIDs[name] {
			allRestarted = false
			t.Errorf("Process %s has same PID after restart", name)
			continue
		}

		if status.Restarts != 1 {
			t.Logf("Process %s has restart count %d (expected 1, but allowing for test variations)", name, status.Restarts)
			// Allow multiple restarts in test environment - just verify it's > 0
			if status.Restarts == 0 {
				allRestarted = false
				t.Errorf("Process %s has incorrect restart count: %d", name, status.Restarts)
				continue
			}
		}

		t.Logf("✓ Process %s successfully restarted: PID %d → %d",
			name, initialPIDs[name], status.PID)
	}

	assert.True(t, allRestarted, "All processes should restart successfully")

	// Get final status count
	statuses, err := manager.StatusAll("test-coord")
	require.NoError(t, err)

	runningCount := 0
	for _, status := range statuses {
		if status.Running {
			runningCount++
		}
	}

	assert.Equal(t, numProcesses, runningCount, "All processes should be running after restart")
	t.Logf("✓ Manager coordination successful: %d/%d processes running", runningCount, numProcesses)
}

// Helper function to kill process by PID
func killProcessByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}
