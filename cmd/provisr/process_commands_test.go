package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCommand_GetProgramsDirectory(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	tests := []struct {
		name           string
		setupConfig    string
		expectedSubdir string
	}{
		{
			name:           "no_config_file",
			setupConfig:    "",
			expectedSubdir: "programs",
		},
		{
			name: "config_with_programs_directory",
			setupConfig: `programs_directory = "my-programs"
pid_dir = "./run"

[server]
listen = "127.0.0.1:8080"`,
			expectedSubdir: "my-programs",
		},
		{
			name: "config_without_programs_directory",
			setupConfig: `pid_dir = "./run"

[server]
listen = "127.0.0.1:8080"`,
			expectedSubdir: "programs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing config
			configPath := filepath.Join(tempDir, "config.toml")
			_ = os.Remove(configPath)

			// Create config file if needed
			if tt.setupConfig != "" {
				if err := os.WriteFile(configPath, []byte(tt.setupConfig), 0o644); err != nil {
					t.Fatalf("failed to create config file: %v", err)
				}
			}

			programsDir, err := cmd.getProgramsDirectory()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// For configs with relative paths, just check the subdirectory name
			// For absolute paths, do a full comparison
			if filepath.IsAbs(programsDir) {
				expectedPath := filepath.Join(tempDir, tt.expectedSubdir)
				// Use EvalSymlinks to resolve any symlinks for macOS /var -> /private/var
				actualPath, _ := filepath.EvalSymlinks(programsDir)
				expectedPath, _ = filepath.EvalSymlinks(expectedPath)
				if actualPath != expectedPath {
					t.Errorf("expected programs directory '%s', got '%s'", expectedPath, actualPath)
				}
			} else {
				// For relative paths, just check the directory name
				if programsDir != tt.expectedSubdir {
					t.Errorf("expected programs directory '%s', got '%s'", tt.expectedSubdir, programsDir)
				}
			}
		})
	}
}

// TestCommand_ReadProgramsDirectoryFromConfig is already tested in register_test.go

// TestCommand_IsProcessInConfigFile is already tested in register_test.go

// TestCommand_ValidateProcessSpec is already tested in register_test.go

// TestCommand_ParseProcessFile is already tested in register_test.go

func TestCommand_RegisterLocally_Integration(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	flags := RegisterFlags{
		Name:      "integration-test",
		Command:   "echo integration",
		WorkDir:   "/app",
		LogDir:    "/var/log/integration",
		AutoStart: true,
	}

	err := cmd.registerLocally(flags)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// Verify file was created
	programsDir := filepath.Join(tempDir, "programs")
	programFile := filepath.Join(programsDir, "integration-test.json")

	if _, err := os.Stat(programFile); os.IsNotExist(err) {
		t.Errorf("expected program file to be created at %s", programFile)
		return
	}

	// Verify file content
	data, err := os.ReadFile(programFile)
	if err != nil {
		t.Errorf("failed to read program file: %v", err)
		return
	}

	var programData map[string]interface{}
	if err := json.Unmarshal(data, &programData); err != nil {
		t.Errorf("failed to parse program file: %v", err)
		return
	}

	// Validate content
	if programData["name"] != "integration-test" {
		t.Errorf("expected name 'integration-test', got %v", programData["name"])
	}
	if programData["command"] != "echo integration" {
		t.Errorf("expected command 'echo integration', got %v", programData["command"])
	}
	if programData["work_dir"] != "/app" {
		t.Errorf("expected work_dir '/app', got %v", programData["work_dir"])
	}
	if programData["auto_restart"] != true {
		t.Errorf("expected auto_restart true, got %v", programData["auto_restart"])
	}

	// Verify log configuration
	logConfig, exists := programData["log"]
	if !exists {
		t.Error("expected log configuration")
		return
	}
	logMap, ok := logConfig.(map[string]interface{})
	if !ok {
		t.Error("expected log to be a map")
		return
	}
	fileConfig, exists := logMap["file"]
	if !exists {
		t.Error("expected log.file configuration")
		return
	}
	fileMap, ok := fileConfig.(map[string]interface{})
	if !ok {
		t.Error("expected log.file to be a map")
		return
	}
	if fileMap["dir"] != "/var/log/integration" {
		t.Errorf("expected log dir '/var/log/integration', got %v", fileMap["dir"])
	}
}

func TestCommand_RegisterFileLocally_Integration(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	// Create a test JSON file
	testSpec := map[string]interface{}{
		"name":         "file-test-app",
		"command":      "python app.py",
		"work_dir":     "/app",
		"auto_restart": true,
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"dir": "/var/log/app",
			},
		},
	}

	testData, err := json.MarshalIndent(testSpec, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test spec: %v", err)
	}

	testFile := filepath.Join(tempDir, "test-app.json")
	if err := os.WriteFile(testFile, testData, 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Register the file
	flags := RegisterFileFlags{
		FilePath: testFile,
	}

	err = cmd.registerFileLocally(flags)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// Verify file was copied to programs directory
	programsDir := filepath.Join(tempDir, "programs")
	programFile := filepath.Join(programsDir, "file-test-app.json")

	if _, err := os.Stat(programFile); os.IsNotExist(err) {
		t.Errorf("expected program file to be created at %s", programFile)
		return
	}

	// Verify content is the same
	copiedData, err := os.ReadFile(programFile)
	if err != nil {
		t.Errorf("failed to read copied file: %v", err)
		return
	}

	var copiedSpec map[string]interface{}
	if err := json.Unmarshal(copiedData, &copiedSpec); err != nil {
		t.Errorf("failed to parse copied file: %v", err)
		return
	}

	if copiedSpec["name"] != "file-test-app" {
		t.Errorf("expected name 'file-test-app', got %v", copiedSpec["name"])
	}
}
