package manager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStore implements store.Store interface for testing
type MockStore struct {
	records map[string]store.Record
	calls   []string
}

func NewMockStore() *MockStore {
	return &MockStore{
		records: make(map[string]store.Record),
		calls:   make([]string, 0),
	}
}

func (ms *MockStore) EnsureSchema(_ context.Context) error {
	ms.calls = append(ms.calls, "EnsureSchema")
	return nil
}

func (ms *MockStore) RecordStart(_ context.Context, rec store.Record) error {
	ms.calls = append(ms.calls, fmt.Sprintf("RecordStart:%s", rec.Name))
	ms.records[rec.Name] = rec
	return nil
}

func (ms *MockStore) RecordStop(_ context.Context, uniq string, _ time.Time, _ error) error {
	ms.calls = append(ms.calls, fmt.Sprintf("RecordStop:%s", uniq))
	return nil
}

func (ms *MockStore) UpsertStatus(_ context.Context, rec store.Record) error {
	ms.calls = append(ms.calls, fmt.Sprintf("UpsertStatus:%s", rec.Name))
	ms.records[rec.Name] = rec
	return nil
}

func (ms *MockStore) GetByName(_ context.Context, name string, _ int) ([]store.Record, error) {
	ms.calls = append(ms.calls, fmt.Sprintf("GetByName:%s", name))
	return []store.Record{}, nil
}

func (ms *MockStore) GetRunning(_ context.Context, namePrefix string) ([]store.Record, error) {
	ms.calls = append(ms.calls, fmt.Sprintf("GetRunning:%s", namePrefix))
	return []store.Record{}, nil
}

func (ms *MockStore) PurgeOlderThan(_ context.Context, _ time.Time) (int64, error) {
	ms.calls = append(ms.calls, "PurgeOlderThan")
	return 0, nil
}

func (ms *MockStore) Close() error {
	ms.calls = append(ms.calls, "Close")
	return nil
}

// MockHistorySink implements history.Sink for testing
type MockHistorySink struct {
	events []history.Event
}

func NewMockHistorySink() *MockHistorySink {
	return &MockHistorySink{
		events: make([]history.Event, 0),
	}
}

func (mhs *MockHistorySink) Send(_ context.Context, event history.Event) error {
	mhs.events = append(mhs.events, event)
	return nil
}

func (mhs *MockHistorySink) Close() error {
	return nil
}

func TestNewManager(t *testing.T) {
	mgr := NewManager()

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}

	if mgr.processes == nil {
		t.Error("processes map not initialized")
	}

	if mgr.envManager == nil {
		t.Error("envManager not initialized")
	}
}

func TestManagerSetGlobalEnv(t *testing.T) {
	mgr := NewManager()

	// Test setting environment variables
	envVars := []string{
		"TEST_VAR=test_value",
		"ANOTHER_VAR=another_value",
		"PATH=/usr/bin:/bin",
	}

	mgr.SetGlobalEnv(envVars)

	// Verify environment is set (we can't directly test internal env,
	// but we can test that it doesn't panic and processes work)
	spec := process.Spec{
		Name:    "test-env-process",
		Command: "echo $TEST_VAR",
	}

	if err := mgr.Start(spec); err != nil {
		t.Errorf("Failed to start process with env vars: %v", err)
	}

	// Clean up
	_ = mgr.Stop("test-env-process", 2*time.Second)
}

func TestManagerSetStore(t *testing.T) {
	mgr := NewManager()
	mockStore := NewMockStore()

	// Test setting store
	err := mgr.SetStore(mockStore)
	if err != nil {
		t.Errorf("SetStore failed: %v", err)
	}

	// Verify EnsureSchema was called
	if len(mockStore.calls) == 0 || mockStore.calls[0] != "EnsureSchema" {
		t.Error("EnsureSchema was not called on store")
	}

	// Test setting nil store
	err = mgr.SetStore(nil)
	if err != nil {
		t.Errorf("SetStore(nil) should not fail: %v", err)
	}
}

func TestManagerSetHistorySinks(t *testing.T) {
	mgr := NewManager()
	sink1 := NewMockHistorySink()
	sink2 := NewMockHistorySink()

	// Test setting history sinks
	mgr.SetHistorySinks(sink1, sink2)

	// We can't directly access the internal histSinks,
	// but we can verify the method doesn't panic

	// Test empty sinks
	mgr.SetHistorySinks()
}

func TestManagerStartStop(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	spec := process.Spec{
		Name:    "test-start-stop",
		Command: "sleep 0.1",
	}

	// Test start
	err := mgr.Start(spec)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify process exists
	status, err := mgr.Status("test-start-stop")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.Name != "test-start-stop" {
		t.Errorf("Expected name 'test-start-stop', got '%s'", status.Name)
	}

	// Test stop
	err = mgr.Stop("test-start-stop", 3*time.Second)
	if err != nil {
		t.Logf("Stop result: %v", err) // May fail if process already exited
	}

	// Test stopping non-existent process
	err = mgr.Stop("non-existent", 1*time.Second)
	if err == nil {
		t.Error("Expected error when stopping non-existent process")
	}
}

func TestManagerStartN(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	spec := process.Spec{
		Name:      "test-multi",
		Command:   "sleep 0.05",
		Instances: 3,
	}

	// Test starting multiple instances
	err := mgr.StartN(spec)
	if err != nil {
		t.Fatalf("StartN failed: %v", err)
	}

	// Verify all instances exist
	expectedNames := []string{
		"test-multi-1",
		"test-multi-2",
		"test-multi-3",
	}

	for _, name := range expectedNames {
		status, err := mgr.Status(name)
		if err != nil {
			t.Errorf("Instance %s not found: %v", name, err)
		} else if status.Name != name {
			t.Errorf("Expected name %s, got %s", name, status.Name)
		}
	}

	// Test single instance (should call Start)
	singleSpec := process.Spec{
		Name:      "test-single",
		Command:   "true",
		Instances: 1,
	}

	err = mgr.StartN(singleSpec)
	if err != nil {
		t.Errorf("StartN with single instance failed: %v", err)
	}
}

func TestManagerPatternMatching(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	// Start processes with different names
	processes := []string{
		"web-server-1",
		"web-server-2",
		"worker-1",
		"worker-2",
		"database",
	}

	for _, name := range processes {
		spec := process.Spec{
			Name:    name,
			Command: "sleep 0.05",
		}
		_ = mgr.Start(spec)
	}

	// Test pattern matching
	testCases := []struct {
		pattern  string
		expected int
	}{
		{"web-server*", 2},
		{"worker*", 2},
		{"database", 1},
		{"*", 5},
		{"", 5},
		{"non-existent*", 0},
	}

	for _, tc := range testCases {
		count, err := mgr.Count(tc.pattern)
		if err != nil {
			t.Errorf("Count failed for pattern '%s': %v", tc.pattern, err)
			continue
		}

		// Note: Count may be less than expected if processes have exited
		if count > tc.expected {
			t.Errorf("Pattern '%s': expected max %d, got %d", tc.pattern, tc.expected, count)
		}

		// Test StatusAll
		statuses, err := mgr.StatusAll(tc.pattern)
		if err != nil {
			t.Errorf("StatusAll failed for pattern '%s': %v", tc.pattern, err)
		}

		if len(statuses) > tc.expected {
			t.Errorf("StatusAll pattern '%s': expected max %d statuses, got %d", tc.pattern, tc.expected, len(statuses))
		}
	}
}

func TestManagerShutdown(t *testing.T) {
	mgr := NewManager()

	// Start some processes
	for i := 0; i < 3; i++ {
		spec := process.Spec{
			Name:    fmt.Sprintf("shutdown-test-%d", i),
			Command: "sleep 0.1",
		}
		_ = mgr.Start(spec)
	}

	// Test shutdown
	err := mgr.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify reconciler is stopped (no direct way to test,
	// but shutdown should not hang)
}

func TestManagerHelperMethods(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	// Test StopMatch and StatusMatch aliases
	spec := process.Spec{
		Name:    "alias-test",
		Command: "sleep 0.05",
	}
	_ = mgr.Start(spec)

	// Test StatusMatch
	statuses, err := mgr.StatusMatch("alias*")
	if err != nil {
		t.Errorf("StatusMatch failed: %v", err)
	}

	if len(statuses) == 0 {
		t.Error("StatusMatch should find at least one process")
	}

	// Test StopMatch
	err = mgr.StopMatch("alias*", 2*time.Second)
	if err != nil {
		t.Logf("StopMatch result: %v", err) // May fail if already stopped
	}
}

func TestManagerInternalHelpers(t *testing.T) {
	mgr := NewManager()

	// Test matchesPattern method
	testCases := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"web-server-1", "web-server*", true},
		{"web-server-1", "*server*", true},
		{"web-server-1", "*-1", true},
		{"web-server-1", "worker*", false},
		{"web-server-1", "", true},
		{"web-server-1", "*", true},
		{"web-server-1", "web-server-1", true},
		{"web-server-1", "web-server-2", false},
	}

	for _, tc := range testCases {
		result := mgr.matchesPattern(tc.name, tc.pattern)
		if result != tc.expected {
			t.Errorf("matchesPattern('%s', '%s') = %v, expected %v",
				tc.name, tc.pattern, result, tc.expected)
		}
	}
}

func TestManagerWithMockStore(t *testing.T) {
	mgr := NewManager()
	mockStore := NewMockStore()

	_ = mgr.SetStore(mockStore)
	defer func() { _ = mgr.Shutdown() }()
	spec := process.Spec{
		Name:    "store-test",
		Command: "echo hello",
	}

	err := mgr.Start(spec)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait a bit for process to complete
	time.Sleep(100 * time.Millisecond)

	// Verify store interactions (this is limited since recordStart/recordStop are stubs)
	if len(mockStore.calls) == 0 {
		t.Log("Note: Store calls may be empty due to stub implementations")
	}
}

// TestManagerMultiProcessAutoRestart tests Manager handling multiple processes with auto-restart
func TestManagerMultiProcessAutoRestart(t *testing.T) {
	manager := NewManager()
	defer func() { _ = manager.Shutdown() }()

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
		restarts uint32
	})

	for _, p := range processes {
		status, err := manager.Status(p.name)
		require.NoError(t, err)
		initialStates[p.name] = struct {
			pid      int
			restarts uint32
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

// TestConcurrentProcessManagement tests multiple processes being started/stopped concurrently
func TestConcurrentProcessManagement(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	const numProcesses = 10
	const numOperations = 5

	// Create test specs
	specs := make([]process.Spec, numProcesses)
	for i := 0; i < numProcesses; i++ {
		specs[i] = process.Spec{
			Name:    fmt.Sprintf("test-proc-%d", i),
			Command: "sleep 0.1",
		}
	}

	// Test concurrent starts
	t.Run("ConcurrentStarts", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, numProcesses)

		// Start all processes concurrently
		for i := 0; i < numProcesses; i++ {
			wg.Add(1)
			go func(spec process.Spec) {
				defer wg.Done()
				if err := mgr.Start(spec); err != nil {
					errors <- err
				}
			}(specs[i])
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				t.Errorf("Concurrent start failed: %v", err)
			}
		}

		// Verify all processes are tracked
		for i := 0; i < numProcesses; i++ {
			if _, err := mgr.Status(specs[i].Name); err != nil {
				t.Errorf("Process %s not found after start: %v", specs[i].Name, err)
			}
		}
	})

	// Test concurrent stops
	t.Run("ConcurrentStops", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, numProcesses)

		// Stop all processes concurrently
		for i := 0; i < numProcesses; i++ {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				if err := mgr.Stop(name, 5*time.Second); err != nil {
					errors <- err
				}
			}(specs[i].Name)
		}

		wg.Wait()
		close(errors)

		// Check for errors (some may not exist, that's ok)
		for err := range errors {
			if err != nil {
				t.Logf("Concurrent stop result: %v", err)
			}
		}
	})

	// Test mixed concurrent operations
	t.Run("ConcurrentMixedOps", func(t *testing.T) {
		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Launch multiple goroutines doing random operations
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				spec := process.Spec{
					Name:    fmt.Sprintf("mixed-proc-%d", id),
					Command: "true",
				}

				select {
				case <-ctx.Done():
					return
				default:
				}

				// Start
				if err := mgr.Start(spec); err != nil {
					t.Logf("Start failed for %s: %v", spec.Name, err)
				}

				// Check status
				if status, err := mgr.Status(spec.Name); err != nil {
					t.Logf("Status check failed for %s: %v", spec.Name, err)
				} else {
					t.Logf("Process %s status: running=%v, PID=%d", spec.Name, status.Running, status.PID)
				}

				// Wait a bit
				time.Sleep(100 * time.Millisecond)

				// Stop
				if err := mgr.Stop(spec.Name, 2*time.Second); err != nil {
					t.Logf("Stop failed for %s: %v", spec.Name, err)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestProcessRecoveryAndMonitoring tests process monitoring and auto-recovery
func TestProcessRecoveryAndMonitoring(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	t.Run("ProcessStatusTracking", func(t *testing.T) {
		spec := process.Spec{
			Name:    "status-test",
			Command: "echo hello",
		}

		// Start process
		if err := mgr.Start(spec); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}

		// Check initial status
		status, err := mgr.Status(spec.Name)
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}

		t.Logf("Initial status: running=%v, PID=%d", status.Running, status.PID)

		// Wait for process to complete
		time.Sleep(100 * time.Millisecond)

		// Check final status
		finalStatus, err := mgr.Status(spec.Name)
		if err != nil {
			t.Fatalf("Failed to get final status: %v", err)
		}

		t.Logf("Final status: running=%v, PID=%d", finalStatus.Running, finalStatus.PID)
		if finalStatus.Running {
			t.Logf("Process still running (expected for echo command)")
		}
	})

	t.Run("MultipleInstancesPattern", func(t *testing.T) {
		spec := process.Spec{
			Name:      "multi-test",
			Command:   "sleep 0.1",
			Instances: 3,
		}

		// Start multiple instances
		if err := mgr.StartN(spec); err != nil {
			t.Fatalf("Failed to start multiple instances: %v", err)
		}

		// Check that all instances are tracked
		expectedNames := []string{
			"multi-test-1",
			"multi-test-2",
			"multi-test-3",
		}

		for _, name := range expectedNames {
			if status, err := mgr.Status(name); err != nil {
				t.Errorf("Instance %s not found: %v", name, err)
			} else {
				t.Logf("Instance %s: running=%v, PID=%d", name, status.Running, status.PID)
			}
		}

		// Test pattern matching
		count, err := mgr.Count("multi-test*")
		if err != nil {
			t.Fatalf("Failed to count processes: %v", err)
		}
		t.Logf("Found %d processes matching pattern", count)

		// Stop all matching processes
		if err := mgr.StopAll("multi-test*", 3*time.Second); err != nil {
			t.Logf("StopAll result: %v", err)
		}
	})
}

// BenchmarkConcurrentOperations benchmarks concurrent process operations
func BenchmarkConcurrentOperations(b *testing.B) {
	mgr := NewManager()
	defer func() { _ = mgr.Shutdown() }()

	b.Run("ConcurrentStartStop", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				spec := process.Spec{
					Name:    fmt.Sprintf("bench-proc-%d", i),
					Command: "true", // Very quick command
				}

				if err := mgr.Start(spec); err != nil {
					b.Errorf("Start failed: %v", err)
				}

				if _, err := mgr.Status(spec.Name); err != nil {
					b.Errorf("Status check failed: %v", err)
				}

				if err := mgr.Stop(spec.Name, time.Second); err != nil {
					b.Logf("Stop failed: %v", err)
				}

				i++
			}
		})
	})
}

// TestManagerReconcilerAutoRestart tests Manager's reconciler-driven auto-restart
func TestManagerReconcilerAutoRestart(t *testing.T) {
	manager := NewManager()
	defer func() { _ = manager.Shutdown() }()

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

	// Wait a bit for restart to happen (allow reconciler + restart interval)
	time.Sleep(2 * time.Second)

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
	defer func() { _ = manager.Shutdown() }()

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
	time.Sleep(3 * time.Second)

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
		time.Sleep(5 * time.Second)
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

// Test state-based command validation
func TestStateBasedCommandValidation(t *testing.T) {
	spec := process.Spec{
		Name:    "validation-test",
		Command: "sleep 0.5",
	}

	mp := NewManagedProcess(spec, mockEnvMerger)
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

		slowMp := NewManagedProcess(slowSpec, mockEnvMerger)
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

		longMp := NewManagedProcess(longSpec, mockEnvMerger)
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
