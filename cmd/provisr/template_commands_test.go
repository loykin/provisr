package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_GetTemplatesDirectory(t *testing.T) {
	cmd := &command{mgr: nil}

	expectedDir := "templates"
	actualDir := cmd.getTemplatesDirectory()

	if actualDir != expectedDir {
		t.Errorf("expected templates directory '%s', got '%s'", expectedDir, actualDir)
	}
}

func TestCommand_TemplateCreate(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	tests := []struct {
		name         string
		flags        TemplateCreateFlags
		expectError  bool
		validateFile func(t *testing.T, filePath string)
	}{
		{
			name: "create_web_template",
			flags: TemplateCreateFlags{
				Type: "web",
				Name: "my-web-app",
			},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				// Check if file exists
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("expected file %s to exist", filePath)
					return
				}

				// Read and validate content
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("failed to read file: %v", err)
					return
				}

				contentStr := string(content)
				if !strings.Contains(contentStr, "my-web-app") {
					t.Error("template should contain process name")
				}
				if !strings.Contains(contentStr, "python -m http.server") {
					t.Error("web template should contain web server command")
				}
			},
		},
		{
			name: "create_api_template",
			flags: TemplateCreateFlags{
				Type: "api",
				Name: "user-service",
			},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("failed to read file: %v", err)
					return
				}

				contentStr := string(content)
				if !strings.Contains(contentStr, "user-service") {
					t.Error("template should contain process name")
				}
				if !strings.Contains(contentStr, "priority") {
					t.Error("api template should contain priority field")
				}
			},
		},
		{
			name: "create_simple_template",
			flags: TemplateCreateFlags{
				Type: "simple",
				Name: "hello-world",
			},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("failed to read file: %v", err)
					return
				}

				contentStr := string(content)
				if !strings.Contains(contentStr, "hello-world") {
					t.Error("template should contain process name")
				}
				if !strings.Contains(contentStr, "echo") {
					t.Error("simple template should contain echo command")
				}
			},
		},
		{
			name: "create_template_with_custom_output",
			flags: TemplateCreateFlags{
				Type:   "worker",
				Name:   "data-worker",
				Output: filepath.Join(tempDir, "custom-worker.json"),
			},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				if !strings.HasSuffix(filePath, "custom-worker.json") {
					t.Errorf("expected custom output path, got %s", filePath)
				}
			},
		},
		{
			name: "default_name_from_type",
			flags: TemplateCreateFlags{
				Type: "database",
				// Name is empty, should default to "database-sample"
			},
			expectError: false,
			validateFile: func(t *testing.T, filePath string) {
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("failed to read file: %v", err)
					return
				}

				contentStr := string(content)
				if !strings.Contains(contentStr, "database-sample") {
					t.Error("template should contain default name 'database-sample'")
				}
			},
		},
		{
			name: "invalid_template_type",
			flags: TemplateCreateFlags{
				Type: "invalid-type",
				Name: "test",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.TemplateCreate(tt.flags)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Determine expected file path
			var expectedPath string
			if tt.flags.Output != "" {
				expectedPath = tt.flags.Output
			} else {
				templateName := tt.flags.Name
				if templateName == "" {
					templateName = tt.flags.Type + "-sample"
				}
				expectedPath = filepath.Join("templates", templateName+".json")
			}

			if tt.validateFile != nil {
				tt.validateFile(t, expectedPath)
			}
		})
	}
}

func TestCommand_TemplateCreate_FileExists(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	// Create templates directory and existing file
	templatesDir := filepath.Join(tempDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("failed to create templates directory: %v", err)
	}

	existingFile := filepath.Join(templatesDir, "existing-app.json")
	if err := os.WriteFile(existingFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	// Test without force flag - should fail
	flags := TemplateCreateFlags{
		Type: "web",
		Name: "existing-app",
	}

	err := cmd.TemplateCreate(flags)
	if err == nil {
		t.Error("expected error when file exists without force flag")
	}
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	// Test with force flag - should succeed
	flags.Force = true
	err = cmd.TemplateCreate(flags)
	if err != nil {
		t.Errorf("unexpected error with force flag: %v", err)
	}
}

func TestCommand_TemplateCreate_DirectoryCreation(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	// Templates directory should not exist initially
	templatesDir := filepath.Join(tempDir, "templates")
	if _, err := os.Stat(templatesDir); !os.IsNotExist(err) {
		t.Fatal("templates directory should not exist initially")
	}

	// Create template - should create directory
	flags := TemplateCreateFlags{
		Type: "web",
		Name: "test-app",
	}

	err := cmd.TemplateCreate(flags)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that directory was created
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		t.Error("templates directory should have been created")
	}

	// Check that file was created
	expectedFile := filepath.Join(templatesDir, "test-app.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Error("template file should have been created")
	}
}

func TestCommand_TemplateCreate_CustomOutputDirectory(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	cmd := &command{mgr: nil}

	// Test with custom output in subdirectory
	customDir := filepath.Join(tempDir, "custom", "path")
	customFile := filepath.Join(customDir, "my-template.json")

	flags := TemplateCreateFlags{
		Type:   "api",
		Name:   "custom-api",
		Output: customFile,
	}

	// This should fail because directory doesn't exist
	err := cmd.TemplateCreate(flags)
	if err == nil {
		t.Error("expected error when output directory doesn't exist")
	}

	// Create the directory and try again
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("failed to create custom directory: %v", err)
	}

	err = cmd.TemplateCreate(flags)
	if err != nil {
		t.Errorf("unexpected error with valid custom path: %v", err)
	}

	// Check that file was created at custom location
	if _, err := os.Stat(customFile); os.IsNotExist(err) {
		t.Error("custom template file should have been created")
	}
}
