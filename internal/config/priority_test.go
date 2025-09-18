package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/provisr/internal/process"
)

// TestLoadSpecsFromTOML_WithPriority tests loading process specs with priority field
func TestLoadSpecsFromTOML_WithPriority(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "priority.toml")
	data := `
[[processes]]
name = "high-priority"
command = "sleep 1"
priority = 5

[[processes]]
name = "low-priority"
command = "sleep 1" 
priority = 20

[[processes]]
name = "default-priority"
command = "sleep 1"
# No priority specified - should default to 0
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	specs, err := LoadSpecsFromTOML(file)
	if err != nil {
		t.Fatalf("load specs: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("expected 3 specs, got %d", len(specs))
	}

	// Verify priority values
	specMap := make(map[string]int)
	for _, spec := range specs {
		specMap[spec.Name] = spec.Priority
	}

	if specMap["high-priority"] != 5 {
		t.Errorf("expected high-priority to have priority 5, got %d", specMap["high-priority"])
	}
	if specMap["low-priority"] != 20 {
		t.Errorf("expected low-priority to have priority 20, got %d", specMap["low-priority"])
	}
	if specMap["default-priority"] != 0 {
		t.Errorf("expected default-priority to have priority 0, got %d", specMap["default-priority"])
	}
}

// TestLoadSpecsFromTOML_ProgramsDirectoryWithPriority tests loading from programs directory with priority
func TestLoadSpecsFromTOML_ProgramsDirectoryWithPriority(t *testing.T) {
	dir := t.TempDir()

	// Create main config file
	mainConfig := filepath.Join(dir, "config.toml")
	mainData := `
# Main config with no processes, only programs directory will be used
env = ["GLOBAL=test"]
`
	if err := os.WriteFile(mainConfig, []byte(mainData), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	// Create programs directory
	programsDir := filepath.Join(dir, "programs")
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		t.Fatalf("create programs dir: %v", err)
	}

	// Create individual program files with different priorities
	files := map[string]string{
		"database.toml": `
name = "database"
command = "sleep 5"
priority = 1
retries = 3`,

		"api.toml": `
name = "api"
command = "sleep 2"
priority = 10
autorestart = true`,

		"worker.toml": `
name = "worker" 
command = "sleep 1"
priority = 20`,

		"monitoring.toml": `
name = "monitoring"
command = "sleep 1"
priority = 30`,
	}

	for filename, content := range files {
		filePath := filepath.Join(programsDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
	}

	specs, err := LoadSpecsFromTOML(mainConfig)
	if err != nil {
		t.Fatalf("load specs from programs directory: %v", err)
	}

	if len(specs) != 4 {
		t.Fatalf("expected 4 specs from programs directory, got %d", len(specs))
	}

	// Verify all processes loaded with correct priorities
	specMap := make(map[string]int)
	for _, spec := range specs {
		specMap[spec.Name] = spec.Priority
	}

	expected := map[string]int{
		"database":   1,
		"api":        10,
		"worker":     20,
		"monitoring": 30,
	}

	for name, expectedPriority := range expected {
		if actualPriority, exists := specMap[name]; !exists {
			t.Errorf("process %s not found in loaded specs", name)
		} else if actualPriority != expectedPriority {
			t.Errorf("process %s: expected priority %d, got %d", name, expectedPriority, actualPriority)
		}
	}
}

// TestLoadSpecsFromTOML_MixedSourcesWithPriority tests loading from both main config and programs directory
func TestLoadSpecsFromTOML_MixedSourcesWithPriority(t *testing.T) {
	dir := t.TempDir()

	// Create main config file with some processes
	mainConfig := filepath.Join(dir, "config.toml")
	mainData := `
env = ["GLOBAL=test"]

[[processes]]
name = "main-service"
command = "sleep 3"
priority = 15
`
	if err := os.WriteFile(mainConfig, []byte(mainData), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	// Create programs directory with additional processes
	programsDir := filepath.Join(dir, "programs")
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		t.Fatalf("create programs dir: %v", err)
	}

	programFile := filepath.Join(programsDir, "program-service.toml")
	programData := `
name = "program-service"
command = "sleep 2"
priority = 5
`
	if err := os.WriteFile(programFile, []byte(programData), 0o644); err != nil {
		t.Fatalf("write program file: %v", err)
	}

	specs, err := LoadSpecsFromTOML(mainConfig)
	if err != nil {
		t.Fatalf("load specs: %v", err)
	}

	if len(specs) != 2 {
		t.Fatalf("expected 2 specs (1 main + 1 programs), got %d", len(specs))
	}

	// Verify both processes loaded with correct priorities
	specMap := make(map[string]int)
	for _, spec := range specs {
		specMap[spec.Name] = spec.Priority
	}

	if specMap["main-service"] != 15 {
		t.Errorf("main-service: expected priority 15, got %d", specMap["main-service"])
	}
	if specMap["program-service"] != 5 {
		t.Errorf("program-service: expected priority 5, got %d", specMap["program-service"])
	}
}

// TestSortSpecsByPriority tests the priority-based sorting functionality
func TestSortSpecsByPriority(t *testing.T) {
	// Create specs with various priorities (unsorted)
	specs := []process.Spec{
		{Name: "worker", Priority: 20},
		{Name: "database", Priority: 1},
		{Name: "api", Priority: 10},
		{Name: "cache", Priority: 5},
		{Name: "monitoring", Priority: 30},
		{Name: "web", Priority: 10}, // Same priority as api
	}

	sorted := SortSpecsByPriority(specs)

	// Verify original slice is not modified
	if specs[0].Name != "worker" || specs[0].Priority != 20 {
		t.Errorf("original slice was modified")
	}

	// Verify sorted order
	expected := []string{"database", "cache", "api", "web", "worker", "monitoring"}
	expectedPriorities := []int{1, 5, 10, 10, 20, 30}

	if len(sorted) != len(expected) {
		t.Fatalf("expected %d sorted specs, got %d", len(expected), len(sorted))
	}

	for i, expectedName := range expected {
		if sorted[i].Name != expectedName {
			t.Errorf("position %d: expected %s, got %s", i, expectedName, sorted[i].Name)
		}
		if sorted[i].Priority != expectedPriorities[i] {
			t.Errorf("position %d (%s): expected priority %d, got %d",
				i, sorted[i].Name, expectedPriorities[i], sorted[i].Priority)
		}
	}
}

// TestSortSpecsByPriority_EmptySlice tests sorting empty slice
func TestSortSpecsByPriority_EmptySlice(t *testing.T) {
	specs := []process.Spec{}
	sorted := SortSpecsByPriority(specs)

	if len(sorted) != 0 {
		t.Errorf("expected empty slice, got length %d", len(sorted))
	}
}

// TestSortSpecsByPriority_SingleItem tests sorting single item
func TestSortSpecsByPriority_SingleItem(t *testing.T) {
	specs := []process.Spec{
		{Name: "only", Priority: 42},
	}

	sorted := SortSpecsByPriority(specs)

	if len(sorted) != 1 {
		t.Fatalf("expected 1 item, got %d", len(sorted))
	}
	if sorted[0].Name != "only" || sorted[0].Priority != 42 {
		t.Errorf("single item not preserved correctly: %+v", sorted[0])
	}
}

// TestSortSpecsByPriority_AllSamePriority tests stable sort behavior
func TestSortSpecsByPriority_AllSamePriority(t *testing.T) {
	specs := []process.Spec{
		{Name: "first", Priority: 10},
		{Name: "second", Priority: 10},
		{Name: "third", Priority: 10},
	}

	sorted := SortSpecsByPriority(specs)

	// Should maintain original order for same priority (stable sort)
	expected := []string{"first", "second", "third"}
	for i, expectedName := range expected {
		if sorted[i].Name != expectedName {
			t.Errorf("position %d: expected %s, got %s (stable sort failed)",
				i, expectedName, sorted[i].Name)
		}
	}
}

// TestSortSpecsByPriority_NegativePriorities tests negative priorities
func TestSortSpecsByPriority_NegativePriorities(t *testing.T) {
	specs := []process.Spec{
		{Name: "normal", Priority: 10},
		{Name: "urgent", Priority: -5},
		{Name: "critical", Priority: -10},
		{Name: "default", Priority: 0},
	}

	sorted := SortSpecsByPriority(specs)

	expected := []string{"critical", "urgent", "default", "normal"}
	expectedPriorities := []int{-10, -5, 0, 10}

	for i, expectedName := range expected {
		if sorted[i].Name != expectedName {
			t.Errorf("position %d: expected %s, got %s", i, expectedName, sorted[i].Name)
		}
		if sorted[i].Priority != expectedPriorities[i] {
			t.Errorf("position %d (%s): expected priority %d, got %d",
				i, sorted[i].Name, expectedPriorities[i], sorted[i].Priority)
		}
	}
}

// TestLoadCronJobsFromTOML_WithPriority tests loading cron jobs with priority
func TestLoadCronJobsFromTOML_WithPriority(t *testing.T) {
	dir := t.TempDir()

	// Create main config with cron job
	mainConfig := filepath.Join(dir, "config.toml")
	mainData := `
[[processes]]
name = "backup"
command = "echo backup"
schedule = "@every 1h"
priority = 5
autorestart = false
instances = 1
`
	if err := os.WriteFile(mainConfig, []byte(mainData), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	// Create programs directory with cron job
	programsDir := filepath.Join(dir, "programs")
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		t.Fatalf("create programs dir: %v", err)
	}

	cronFile := filepath.Join(programsDir, "cleanup.toml")
	cronData := `
name = "cleanup"
command = "echo cleanup"
schedule = "@every 30m"
priority = 10
`
	if err := os.WriteFile(cronFile, []byte(cronData), 0o644); err != nil {
		t.Fatalf("write cron file: %v", err)
	}

	jobs, err := LoadCronJobsFromTOML(mainConfig)
	if err != nil {
		t.Fatalf("load cron jobs: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 cron jobs, got %d", len(jobs))
	}

	// Verify priorities are preserved in cron jobs
	jobMap := make(map[string]int)
	for _, job := range jobs {
		jobMap[job.Name] = job.Spec.Priority
	}

	if jobMap["backup"] != 5 {
		t.Errorf("backup job: expected priority 5, got %d", jobMap["backup"])
	}
	if jobMap["cleanup"] != 10 {
		t.Errorf("cleanup job: expected priority 10, got %d", jobMap["cleanup"])
	}
}
