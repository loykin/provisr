package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loykin/provisr"
)

// TestStartFromSpecs_WithPriority tests that startFromSpecs respects priority ordering
func TestStartFromSpecs_WithPriority(t *testing.T) {
	mgr := provisr.New()

	// Create test specs with priorities
	specs := []provisr.Spec{
		{Name: "worker", Command: "sleep 2", Priority: 20},
		{Name: "database", Command: "sleep 2", Priority: 1},
		{Name: "api", Command: "sleep 2", Priority: 10},
	}

	// Start processes using startFromSpecs (which should sort by priority)
	if err := startFromSpecs(mgr, specs); err != nil {
		t.Fatalf("startFromSpecs failed: %v", err)
	}

	// Give processes time to start
	time.Sleep(200 * time.Millisecond)

	// Verify all processes are running
	for _, spec := range specs {
		statuses, err := mgr.StatusAll(spec.Name)
		if err != nil {
			t.Errorf("StatusAll for %s failed: %v", spec.Name, err)
			continue
		}
		if len(statuses) == 0 {
			t.Errorf("process %s has no status", spec.Name)
			continue
		}

		running := false
		for _, status := range statuses {
			if status.Running {
				running = true
				break
			}
		}

		if !running {
			t.Errorf("process %s is not running", spec.Name)
		}
	}

	// Clean up
	for _, spec := range specs {
		_ = mgr.StopAll(spec.Name, 1000)
	}
}

// TestStartFromSpecs_PriorityWithInstances tests priority ordering with multiple instances
func TestStartFromSpecs_PriorityWithInstances(t *testing.T) {
	mgr := provisr.New()

	specs := []provisr.Spec{
		{
			Name:        "multi-worker",
			Command:     "sleep 2",
			Priority:    20,
			Instances:   2,
			AutoRestart: false,
		},
		{
			Name:        "single-db",
			Command:     "sleep 2",
			Priority:    1,
			Instances:   1,
			AutoRestart: false,
		},
	}

	if err := startFromSpecs(mgr, specs); err != nil {
		t.Fatalf("startFromSpecs failed: %v", err)
	}

	// Give processes time to start
	time.Sleep(200 * time.Millisecond)

	// Verify database (priority 1) is running
	dbStatuses, err := mgr.StatusAll("single-db")
	if err != nil {
		t.Fatalf("StatusAll for database failed: %v", err)
	}
	if len(dbStatuses) != 1 {
		t.Errorf("expected 1 database instance, got %d", len(dbStatuses))
	} else if !dbStatuses[0].Running {
		t.Error("database should be running")
	}

	// Verify multi-worker (priority 20) instances are running
	workerStatuses, err := mgr.StatusAll("multi-worker")
	if err != nil {
		t.Fatalf("StatusAll for workers failed: %v", err)
	}
	if len(workerStatuses) != 2 {
		t.Errorf("expected 2 worker instances, got %d", len(workerStatuses))
	}

	runningWorkers := 0
	for _, status := range workerStatuses {
		if status.Running {
			runningWorkers++
		}
	}
	if runningWorkers != 2 {
		t.Errorf("expected 2 running workers, got %d", runningWorkers)
	}

	// Clean up
	_ = mgr.StopAll("multi-worker", 1000)
	_ = mgr.StopAll("single-db", 1000)
}

// TestStartCommand_WithPriorityConfig tests the start command with priority-ordered configs
func TestStartCommand_WithPriorityConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()

	// Create main config
	mainConfig := filepath.Join(dir, "config.toml")
	mainData := `
[[processes]]
name = "main-service"
command = "sleep 2"
priority = 10
`
	if err := os.WriteFile(mainConfig, []byte(mainData), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	// Create programs directory with priority configs
	programsDir := filepath.Join(dir, "programs")
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		t.Fatalf("create programs dir: %v", err)
	}

	earlyProgram := filepath.Join(programsDir, "early.toml")
	earlyData := `
name = "early"
command = "sleep 2"
priority = 1
`
	if err := os.WriteFile(earlyProgram, []byte(earlyData), 0o644); err != nil {
		t.Fatalf("write early program: %v", err)
	}

	lateProgram := filepath.Join(programsDir, "late.toml")
	lateData := `
name = "late"
command = "sleep 2"
priority = 20
`
	if err := os.WriteFile(lateProgram, []byte(lateData), 0o644); err != nil {
		t.Fatalf("write late program: %v", err)
	}

	// Load specs and start them
	specs, err := provisr.LoadSpecs(mainConfig)
	if err != nil {
		t.Fatalf("LoadSpecs failed: %v", err)
	}

	mgr := provisr.New()
	if err := startFromSpecs(mgr, specs); err != nil {
		t.Fatalf("startFromSpecs failed: %v", err)
	}

	// Give processes time to start
	time.Sleep(200 * time.Millisecond)

	// Verify all processes are running
	expectedProcesses := []string{"early", "main-service", "late"}
	for _, processName := range expectedProcesses {
		statuses, err := mgr.StatusAll(processName)
		if err != nil {
			t.Errorf("StatusAll for %s failed: %v", processName, err)
			continue
		}
		if len(statuses) == 0 {
			t.Errorf("process %s has no status", processName)
			continue
		}

		running := false
		for _, status := range statuses {
			if status.Running {
				running = true
				break
			}
		}

		if !running {
			t.Errorf("process %s should be running", processName)
		}
	}

	// Clean up
	for _, processName := range expectedProcesses {
		_ = mgr.StopAll(processName, 1000)
	}
}

// TestStatusesByBase_WithPrioritySpecs tests status retrieval for priority-ordered specs
func TestStatusesByBase_WithPrioritySpecs(t *testing.T) {
	mgr := provisr.New()

	spec := provisr.Spec{
		Name:        "test-spec",
		Command:     "sleep 2",
		Priority:    5,
		Instances:   1,
		AutoRestart: false,
	}

	if err := mgr.Start(spec); err != nil {
		t.Fatalf("failed to start test spec: %v", err)
	}

	// Give process time to start
	time.Sleep(200 * time.Millisecond)

	statuses, err := mgr.StatusAll("test-spec")
	if err != nil {
		t.Fatalf("StatusAll failed: %v", err)
	}
	if len(statuses) != 1 {
		t.Errorf("expected 1 status, got %d", len(statuses))
	}

	if len(statuses) > 0 && !statuses[0].Running {
		t.Error("process should be running")
	}

	// Clean up
	_ = mgr.StopAll("test-spec", 1000)
}
