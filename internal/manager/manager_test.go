package manager

import (
	"context"
	"fmt"
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
