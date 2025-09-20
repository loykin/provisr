package manager

import (
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// TestManagedProcessAutoRestart verifies that ManagedProcess itself does NOT perform auto-restart anymore.
// Auto-restart is handled by Manager's reconciler; here we ensure MP stays stopped after an unexpected exit.
func TestManagedProcessAutoRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ManagedProcess auto-restart test")
	}
	// ManagedProcess no longer performs auto-restart; covered by Manager reconciler tests.
	t.Skip("deprecated: auto-restart is manager-responsibility now")

	spec := process.Spec{
		Name:            "test-managed-restart",
		Command:         "sh -c 'while true; do echo managed-process running; sleep 0.5; done'",
		AutoRestart:     true,
		RestartInterval: 300 * time.Millisecond,
	}

	envMerger := func(spec process.Spec) []string { return spec.Env }
	startLogger := func(proc *process.Process) {
		status := proc.Snapshot()
		t.Logf("Process started: %s (PID: %d)", status.Name, status.PID)
	}
	stopLogger := func(proc *process.Process, err error) {
		status := proc.Snapshot()
		t.Logf("Process stopped: %s (PID: %d), error: %v", status.Name, status.PID, err)
	}

	mp := NewManagedProcess(spec, envMerger, startLogger, stopLogger)
	defer mp.Stop(5 * time.Second)

	// Phase 1: Start the managed process
	t.Log("Phase 1: Starting ManagedProcess")
	startCmd := command{action: ActionStart, spec: spec, reply: make(chan error, 1)}
	mp.cmdChan <- startCmd
	err := <-startCmd.reply
	if err != nil {
		t.Fatalf("Failed to start managed process: %v", err)
	}

	// Wait for process to fully start
	time.Sleep(500 * time.Millisecond)

	// Verify initial state
	status := mp.Status()
	if !status.Running {
		t.Fatalf("Process should be running, got status: %+v", status)
	}

	initialPID := status.PID
	initialRestarts := status.Restarts
	t.Logf("✓ Initial state: PID=%d, Running=%v, Restarts=%d", initialPID, status.Running, initialRestarts)

	// Phase 2: Kill the process and verify auto-restart detection
	t.Log("Phase 2: Killing process to trigger auto-restart")

	err = syscall.Kill(initialPID, syscall.SIGKILL)
	if err != nil {
		t.Fatalf("Failed to kill process PID %d: %v", initialPID, err)
	}
	t.Logf("✓ Killed process PID %d", initialPID)

	// Phase 3: Wait for auto-restart to complete
	t.Log("Phase 3: Waiting for auto-restart detection and new process creation")

	maxWaitTime := spec.RestartInterval + 5*time.Second // Give extra time for detection and restart
	waitStart := time.Now()

	var restarted bool
	var newPID int
	var newRestarts int

	for time.Since(waitStart) < maxWaitTime {
		status := mp.Status()

		// Check if we have successfully restarted with new PID and incremented restart count
		if status.Running && status.PID != initialPID && status.PID > 0 && status.Restarts > initialRestarts {
			restarted = true
			newPID = status.PID
			newRestarts = status.Restarts
			break
		}

		t.Logf("Monitoring restart progress: PID=%d, Running=%v, Restarts=%d",
			status.PID, status.Running, status.Restarts)

		time.Sleep(100 * time.Millisecond)
	}

	if !restarted {
		t.Errorf("CRITICAL: Auto-restart failed in ManagedProcess!")

		// Debug information
		finalStatus := mp.Status()
		t.Logf("Final status after %v: %+v", maxWaitTime, finalStatus)

		// Check current DetectAlive status
		mp.mu.RLock()
		proc := mp.proc
		mp.mu.RUnlock()

		if proc != nil {
			alive, source := proc.DetectAlive()
			t.Logf("Current DetectAlive: alive=%v, source=%s", alive, source)
		}

		t.FailNow()
	}

	t.Logf("✓ Auto-restart successful: PID %d → %d, Restarts %d → %d",
		initialPID, newPID, initialRestarts, newRestarts)

	// Phase 4: Verify new process health and functionality
	t.Log("Phase 4: Verifying restarted process health")

	mp.mu.RLock()
	proc := mp.proc
	mp.mu.RUnlock()

	if proc != nil {
		alive, source := proc.DetectAlive()
		if !alive {
			t.Errorf("Restarted process should be alive, got alive=%v, source=%s", alive, source)
		} else {
			t.Logf("✓ Restarted process is healthy: alive=%v, source=%s", alive, source)
		}
	}

	// Phase 5: Verify restart counter accuracy
	finalStatus := mp.Status()
	if finalStatus.Restarts != initialRestarts+1 {
		t.Errorf("Restart count should be %d, got %d", initialRestarts+1, finalStatus.Restarts)
	} else {
		t.Logf("✓ Restart counter accurate: %d", finalStatus.Restarts)
	}
}

// TestManagedProcessMultipleRestarts tests multiple consecutive restarts
func TestManagedProcessMultipleRestarts(t *testing.T) {
	// ManagedProcess no longer performs auto-restart; covered by Manager reconciler tests.
	if testing.Short() {
		t.Skip("skipping multiple restart test")
	}
	t.Skip("deprecated: auto-restart is manager-responsibility now")
	if testing.Short() {
		t.Skip("skipping multiple restart test")
	}

	spec := process.Spec{
		Name:            "test-multi-restart",
		Command:         "sh -c 'echo process-start-$$; sleep 10'", // Short-lived process
		AutoRestart:     true,
		RestartInterval: 200 * time.Millisecond,
	}

	envMerger := func(spec process.Spec) []string { return spec.Env }
	startLogger := func(proc *process.Process) {}
	stopLogger := func(proc *process.Process, err error) {}

	mp := NewManagedProcess(spec, envMerger, startLogger, stopLogger)
	defer mp.Stop(3 * time.Second)

	// Start the process
	startCmd := command{action: ActionStart, spec: spec, reply: make(chan error, 1)}
	mp.cmdChan <- startCmd
	err := <-startCmd.reply
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	initialStatus := mp.Status()
	initialRestarts := initialStatus.Restarts
	t.Logf("Initial restarts: %d", initialRestarts)

	// Kill the process multiple times and verify each restart
	killCount := 3
	for i := 0; i < killCount; i++ {
		t.Logf("Kill cycle %d/%d", i+1, killCount)

		// Get current PID
		status := mp.Status()
		if !status.Running {
			t.Errorf("Process should be running before kill %d", i+1)
			continue
		}

		currentPID := status.PID
		currentRestarts := status.Restarts

		// Kill the process
		err = syscall.Kill(currentPID, syscall.SIGKILL)
		if err != nil {
			t.Logf("Kill %d failed: %v", i+1, err)
			continue
		}

		// Wait for restart
		maxWait := spec.RestartInterval + 3*time.Second
		waitStart := time.Now()
		restarted := false

		for time.Since(waitStart) < maxWait {
			status := mp.Status()
			if status.Running && status.PID != currentPID && status.Restarts > currentRestarts {
				restarted = true
				t.Logf("✓ Restart %d: PID %d → %d, Restarts %d → %d",
					i+1, currentPID, status.PID, currentRestarts, status.Restarts)
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		if !restarted {
			t.Errorf("Restart %d failed", i+1)
		}

		// Small delay between kills
		time.Sleep(300 * time.Millisecond)
	}

	// Verify final restart count
	finalStatus := mp.Status()
	expectedRestarts := initialRestarts + killCount
	if finalStatus.Restarts < expectedRestarts {
		t.Errorf("Expected at least %d restarts, got %d", expectedRestarts, finalStatus.Restarts)
	} else {
		t.Logf("✓ Multiple restarts successful: %d total restarts", finalStatus.Restarts)
	}
}

// TestManagedProcessNoAutoRestart tests that processes without auto-restart don't restart
func TestManagedProcessNoAutoRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping no auto-restart test")
	}

	spec := process.Spec{
		Name:        "test-no-restart",
		Command:     "sh -c 'echo no-restart-test; sleep 5'",
		AutoRestart: false, // Explicitly disable auto-restart
	}

	envMerger := func(spec process.Spec) []string { return spec.Env }
	startLogger := func(proc *process.Process) {}
	stopLogger := func(proc *process.Process, err error) {}

	mp := NewManagedProcess(spec, envMerger, startLogger, stopLogger)
	defer mp.Stop(2 * time.Second)

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
	time.Sleep(2 * time.Second)

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

// TestManagedProcessRestartTiming tests that restart intervals are respected
func TestManagedProcessRestartTiming(t *testing.T) {
	// ManagedProcess no longer performs auto-restart; covered by Manager reconciler tests.
	if testing.Short() {
		t.Skip("skipping restart timing test")
	}
	t.Skip("deprecated: auto-restart is manager-responsibility now")
	if testing.Short() {
		t.Skip("skipping restart timing test")
	}

	restartInterval := 1 * time.Second
	spec := process.Spec{
		Name:            "test-restart-timing",
		Command:         "sh -c 'echo timing-test; sleep 10'",
		AutoRestart:     true,
		RestartInterval: restartInterval,
	}

	envMerger := func(spec process.Spec) []string { return spec.Env }
	startLogger := func(proc *process.Process) {}
	stopLogger := func(proc *process.Process, err error) {}

	mp := NewManagedProcess(spec, envMerger, startLogger, stopLogger)
	defer mp.Stop(3 * time.Second)

	// Start the process
	startCmd := command{action: ActionStart, spec: spec, reply: make(chan error, 1)}
	mp.cmdChan <- startCmd
	err := <-startCmd.reply
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Kill the process and measure restart time
	status := mp.Status()
	initialPID := status.PID

	killTime := time.Now()
	err = syscall.Kill(initialPID, syscall.SIGKILL)
	if err != nil {
		t.Fatalf("Failed to kill process: %v", err)
	}

	// Monitor for restart
	maxWait := restartInterval + 3*time.Second
	var restartTime time.Time

	for time.Since(killTime) < maxWait {
		status := mp.Status()
		if status.Running && status.PID != initialPID {
			restartTime = time.Now()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if restartTime.IsZero() {
		t.Fatalf("Process did not restart within expected time")
	}

	actualInterval := restartTime.Sub(killTime)
	t.Logf("Restart timing: killed at %v, restarted at %v, actual interval: %v",
		killTime.Format("15:04:05.000"), restartTime.Format("15:04:05.000"), actualInterval)

	// The restart should take at least the configured interval
	// (but may take longer due to detection time + system scheduling)
	if actualInterval < restartInterval {
		t.Errorf("Restart happened too quickly: expected >= %v, got %v", restartInterval, actualInterval)
	}

	// But shouldn't take too much longer (allow 2x tolerance)
	maxExpected := restartInterval * 3
	if actualInterval > maxExpected {
		t.Errorf("Restart took too long: expected <= %v, got %v", maxExpected, actualInterval)
	} else {
		t.Logf("✓ Restart timing within acceptable range")
	}
}
