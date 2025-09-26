package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_RegisterLocally(t *testing.T) {
	// Create temporary directory for test and set it as working directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	programsDir := filepath.Join(tempDir, "programs")
	cmd := &command{mgr: nil}

	tests := []struct {
		name       string
		flags      RegisterFlags
		expectFile string
		expectErr  bool
	}{
		{
			name: "successful_registration",
			flags: RegisterFlags{
				Name:      "test-process",
				Command:   "echo hello",
				WorkDir:   "/tmp",
				LogDir:    "/var/log",
				AutoStart: true,
			},
			expectFile: "test-process.json",
			expectErr:  false,
		},
		{
			name: "registration_without_optional_fields",
			flags: RegisterFlags{
				Name:    "simple-process",
				Command: "sleep 10",
			},
			expectFile: "simple-process.json",
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.registerLocally(tt.flags)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check if file was created
			filePath := filepath.Join(programsDir, tt.expectFile)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("expected file %s to be created", filePath)
				return
			}

			// Read and validate file contents
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Errorf("failed to read created file: %v", err)
				return
			}

			var programData map[string]interface{}
			if err := json.Unmarshal(data, &programData); err != nil {
				t.Errorf("failed to parse JSON: %v", err)
				return
			}

			// Validate basic fields
			if programData["name"] != tt.flags.Name {
				t.Errorf("expected name %q, got %q", tt.flags.Name, programData["name"])
			}
			if programData["command"] != tt.flags.Command {
				t.Errorf("expected command %q, got %q", tt.flags.Command, programData["command"])
			}

			// Validate optional fields
			if tt.flags.WorkDir != "" && programData["work_dir"] != tt.flags.WorkDir {
				t.Errorf("expected work_dir %q, got %q", tt.flags.WorkDir, programData["work_dir"])
			}

			// Validate log configuration if provided
			if tt.flags.LogDir != "" {
				logConfig, exists := programData["log"]
				if !exists {
					t.Error("expected log configuration but it was missing")
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
				if fileMap["dir"] != tt.flags.LogDir {
					t.Errorf("expected log dir %q, got %q", tt.flags.LogDir, fileMap["dir"])
				}
			}
		})
	}
}

func TestCommand_RegisterLocally_DuplicateName(t *testing.T) {
	// Create temporary directory for test and set it as working directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	flags := RegisterFlags{
		Name:    "duplicate-process",
		Command: "echo hello",
	}

	// First registration should succeed
	err := cmd.registerLocally(flags)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = cmd.registerLocally(flags)
	if err == nil {
		t.Error("expected error for duplicate process name but got none")
	}

	if err == nil {
		t.Errorf("expected 'already registered' error, got nil")
	} else if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' error, got: %v", err)
	}
}

func TestCommand_UnregisterLocally(t *testing.T) {
	// Create temporary directory for test and set it as working directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	programsDir := filepath.Join(tempDir, "programs")
	cmd := &command{mgr: nil}

	// Create test program files
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		t.Fatalf("failed to create programs directory: %v", err)
	}

	testFiles := []string{"test-process.json", "another-process.toml"}
	for _, filename := range testFiles {
		filePath := filepath.Join(programsDir, filename)
		if err := os.WriteFile(filePath, []byte("{}"), 0o644); err != nil {
			t.Fatalf("failed to create test file %s: %v", filename, err)
		}
	}

	tests := []struct {
		name      string
		process   string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "successful_unregister_json",
			process:   "test-process",
			expectErr: false,
		},
		{
			name:      "successful_unregister_toml",
			process:   "another-process",
			expectErr: false,
		},
		{
			name:      "process_not_found",
			process:   "non-existent",
			expectErr: true,
			errMsg:    "not registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := UnregisterFlags{Name: tt.process}
			err := cmd.unregisterLocally(flags)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check that file was removed
			extensions := []string{".json", ".toml", ".yaml", ".yml"}
			for _, ext := range extensions {
				filePath := filepath.Join(programsDir, tt.process+ext)
				if _, err := os.Stat(filePath); !os.IsNotExist(err) {
					t.Errorf("expected file %s to be removed", filePath)
				}
			}
		})
	}
}

func TestCommand_IsProcessInConfigFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	configContent := `
# Some config

[[processes]]
name = "protected-process"
command = "echo protected"

[[processes]]
name = "another-protected"
command = "sleep 1000"

[server]
listen = "127.0.0.1:8080"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Change working directory to temp directory so config.toml is found
	originalWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	cmd := &command{mgr: nil}

	tests := []struct {
		name     string
		process  string
		expected bool
	}{
		{
			name:     "process_in_config",
			process:  "protected-process",
			expected: true,
		},
		{
			name:     "another_process_in_config",
			process:  "another-protected",
			expected: true,
		},
		{
			name:     "process_not_in_config",
			process:  "not-protected",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmd.isProcessInConfigFile(tt.process)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCommand_RegisterFileLocally(t *testing.T) {
	// Create temporary directory for test and set it as working directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	tests := []struct {
		name         string
		fileContent  string
		expectErr    bool
		expectedName string
		errMsg       string
	}{
		{
			name: "valid_json_file",
			fileContent: `{
				"name": "test-app",
				"command": "echo hello",
				"work_dir": "/tmp",
				"auto_restart": true,
				"log": {
					"file": {
						"dir": "/var/log"
					}
				}
			}`,
			expectErr:    false,
			expectedName: "test-app",
		},
		{
			name: "minimal_valid_json",
			fileContent: `{
				"name": "simple-app",
				"command": "sleep 10"
			}`,
			expectErr:    false,
			expectedName: "simple-app",
		},
		{
			name: "missing_name_field",
			fileContent: `{
				"command": "echo hello"
			}`,
			expectErr: true,
			errMsg:    "'name' field is required",
		},
		{
			name: "missing_command_field",
			fileContent: `{
				"name": "no-command-app"
			}`,
			expectErr: true,
			errMsg:    "'command' field is required",
		},
		{
			name: "invalid_json",
			fileContent: `{
				"name": "broken-app"
				"command": "echo hello"
			}`,
			expectErr: true,
			errMsg:    "failed to parse JSON file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary test file
			testFile := filepath.Join(tempDir, tt.name+".json")
			if err := os.WriteFile(testFile, []byte(tt.fileContent), 0o644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			flags := RegisterFileFlags{FilePath: testFile}
			err := cmd.registerFileLocally(flags)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check if file was created in programs directory
			programsDir := filepath.Join(tempDir, "programs")
			targetFile := filepath.Join(programsDir, tt.expectedName+".json")

			if _, err := os.Stat(targetFile); os.IsNotExist(err) {
				t.Errorf("expected file %s to be created", targetFile)
				return
			}

			// Verify file content
			data, err := os.ReadFile(targetFile)
			if err != nil {
				t.Errorf("failed to read target file: %v", err)
				return
			}

			var spec map[string]interface{}
			if err := json.Unmarshal(data, &spec); err != nil {
				t.Errorf("failed to parse target file: %v", err)
				return
			}

			if spec["name"] != tt.expectedName {
				t.Errorf("expected name %q, got %q", tt.expectedName, spec["name"])
			}
		})
	}
}

func TestCommand_RegisterFileLocally_DuplicateName(t *testing.T) {
	// Create temporary directory for test and set it as working directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	// Create test JSON file
	fileContent := `{
		"name": "duplicate-app",
		"command": "echo hello"
	}`

	testFile := filepath.Join(tempDir, "test.json")
	if err := os.WriteFile(testFile, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	flags := RegisterFileFlags{FilePath: testFile}

	// First registration should succeed
	err := cmd.registerFileLocally(flags)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = cmd.registerFileLocally(flags)
	if err == nil {
		t.Error("expected error for duplicate process name but got none")
	}

	if err == nil {
		t.Errorf("expected 'already registered' error, got nil")
	} else if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' error, got: %v", err)
	}
}

func TestCommand_ParseProcessFile(t *testing.T) {
	tempDir := t.TempDir()
	cmd := &command{mgr: nil}

	tests := []struct {
		name        string
		content     string
		expectErr   bool
		expectedErr string
	}{
		{
			name: "valid_json",
			content: `{
				"name": "test",
				"command": "echo hello"
			}`,
			expectErr: false,
		},
		{
			name:        "file_not_exists",
			content:     "",
			expectErr:   true,
			expectedErr: "file does not exist",
		},
		{
			name: "invalid_json",
			content: `{
				"name": "test"
				"command": "echo"
			}`,
			expectErr:   true,
			expectedErr: "failed to parse JSON file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if tt.name == "file_not_exists" {
				filePath = filepath.Join(tempDir, "nonexistent.json")
			} else {
				filePath = filepath.Join(tempDir, tt.name+".json")
				if err := os.WriteFile(filePath, []byte(tt.content), 0o644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			spec, err := cmd.parseProcessFile(filePath)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.expectedErr != "" && !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing %q, got: %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if spec == nil {
				t.Error("expected non-nil spec")
			}
		})
	}
}

func TestCommand_ValidateProcessSpec(t *testing.T) {
	cmd := &command{mgr: nil}

	tests := []struct {
		name        string
		spec        map[string]interface{}
		expectErr   bool
		expectedErr string
	}{
		{
			name: "valid_spec",
			spec: map[string]interface{}{
				"name":    "test",
				"command": "echo hello",
			},
			expectErr: false,
		},
		{
			name: "valid_spec_with_optionals",
			spec: map[string]interface{}{
				"name":         "test",
				"command":      "echo hello",
				"work_dir":     "/tmp",
				"auto_restart": true,
				"log": map[string]interface{}{
					"file": map[string]interface{}{
						"dir": "/var/log",
					},
				},
			},
			expectErr: false,
		},
		{
			name:        "missing_name",
			spec:        map[string]interface{}{"command": "echo hello"},
			expectErr:   true,
			expectedErr: "'name' field is required",
		},
		{
			name:        "missing_command",
			spec:        map[string]interface{}{"name": "test"},
			expectErr:   true,
			expectedErr: "'command' field is required",
		},
		{
			name: "invalid_name_type",
			spec: map[string]interface{}{
				"name":    123,
				"command": "echo hello",
			},
			expectErr:   true,
			expectedErr: "'name' must be a non-empty string",
		},
		{
			name: "invalid_work_dir_type",
			spec: map[string]interface{}{
				"name":     "test",
				"command":  "echo hello",
				"work_dir": 123,
			},
			expectErr:   true,
			expectedErr: "'work_dir' must be a string",
		},
		{
			name: "invalid_log_structure",
			spec: map[string]interface{}{
				"name":    "test",
				"command": "echo hello",
				"log":     "invalid",
			},
			expectErr:   true,
			expectedErr: "'log' must be an object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.validateProcessSpec(tt.spec)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.expectedErr != "" && !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing %q, got: %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCommand_ReadProgramsDirectoryFromConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	configContent := `
programs_directory = "my-programs"
pid_dir = "./run"

[server]
listen = "127.0.0.1:8080"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	cmd := &command{mgr: nil}
	result, err := cmd.readProgramsDirectoryFromConfig(configPath)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != "my-programs" {
		t.Errorf("expected 'my-programs', got %q", result)
	}
}
