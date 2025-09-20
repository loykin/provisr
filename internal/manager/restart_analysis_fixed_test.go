package manager

import (
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManagerRestartOnly tests that Manager exclusively handles restart logic
func TestManagerRestartOnly(t *testing.T) {
	t.Log("=== Manager-Only Restart Test ===")

	// Create manager
	manager := NewManager()
	defer func() { _ = manager.Shutdown() }()
	t.Log("✓ Manager reconciler started")

	// Test process spec with a command that exists on macOS
	spec := process.Spec{
		Name:        "restart-test",
		Command:     "sleep 60", // Use sleep command which exists on macOS
		AutoRestart: true,
	} // Start process through Manager
	err := manager.Start(spec)
	require.NoError(t, err)
	t.Log("✓ Process started through Manager")

	// Wait for process to be fully running
	time.Sleep(500 * time.Millisecond)

	// Get initial status
	status, err := manager.Status("restart-test")
	require.NoError(t, err)
	require.True(t, status.Running, "Process should be running")
	originalPID := status.PID
	originalRestarts := status.Restarts
	t.Logf("✓ Initial state: PID=%d, Restarts=%d", originalPID, originalRestarts)

	// Test 1: Single kill and restart
	err = syscall.Kill(originalPID, syscall.SIGKILL)
	require.NoError(t, err)
	t.Logf("✓ Killed process PID %d", originalPID)

	// Wait for Manager to restart
	time.Sleep(2 * time.Second)

	status, err = manager.Status("restart-test")
	require.NoError(t, err)
	phase1PID := status.PID
	phase1Restarts := status.Restarts
	t.Logf("✓ After restart: PID=%d, Restarts=%d", phase1PID, phase1Restarts)

	// Verify exactly 1 restart occurred
	assert.Equal(t, originalRestarts+1, phase1Restarts, "Should have exactly 1 restart")
	assert.NotEqual(t, originalPID, phase1PID, "PID should have changed after restart")

	// Test 2: Rapid successive kills
	t.Log("--- Rapid successive kills test ---")
	initialRapidRestarts := phase1Restarts

	for i := 1; i <= 3; i++ {
		// Get current status
		currentStatus, err := manager.Status("restart-test")
		require.NoError(t, err)
		require.True(t, currentStatus.Running, "Process should be running before kill")

		// Kill the process
		err = syscall.Kill(currentStatus.PID, syscall.SIGKILL)
		require.NoError(t, err)
		t.Logf("Rapid kill #%d: PID %d", i, currentStatus.PID)

		// Wait for restart
		time.Sleep(3 * time.Second)
	}

	// Final check - should have exactly initialRapidRestarts + 3
	finalStatus, err := manager.Status("restart-test")
	require.NoError(t, err)
	finalRestarts := finalStatus.Restarts
	expectedRestarts := initialRapidRestarts + 3

	t.Logf("✓ Final result: %d restarts from 3 kills", finalRestarts-initialRapidRestarts)

	// Verify exactly 1:1 ratio
	if finalRestarts == expectedRestarts {
		t.Log("✅ Perfect 1:1 kill-to-restart ratio achieved")
	} else {
		t.Errorf("❌ Expected %d total restarts, got %d", expectedRestarts, finalRestarts)
	}
}
