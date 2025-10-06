package template

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerator_Generate(t *testing.T) {
	generator := NewGenerator()

	tests := []struct {
		name         string
		templateType TemplateType
		processName  string
		expectError  bool
		validate     func(*testing.T, *ProcessTemplate)
	}{
		{
			name:         "web_template",
			templateType: TypeWeb,
			processName:  "my-web-app",
			expectError:  false,
			validate: func(t *testing.T, template *ProcessTemplate) {
				if template.Name != "my-web-app" {
					t.Errorf("expected name 'my-web-app', got '%s'", template.Name)
				}
				if template.Command != "python -m http.server 8000" {
					t.Errorf("unexpected command: %s", template.Command)
				}
				if template.AutoRestart == nil || !*template.AutoRestart {
					t.Error("expected auto_restart to be true")
				}
				if len(template.Env) != 2 {
					t.Errorf("expected 2 env vars, got %d", len(template.Env))
				}
			},
		},
		{
			name:         "api_template",
			templateType: TypeAPI,
			processName:  "user-service",
			expectError:  false,
			validate: func(t *testing.T, template *ProcessTemplate) {
				if template.Name != "user-service" {
					t.Errorf("expected name 'user-service', got '%s'", template.Name)
				}
				if template.Priority == nil || *template.Priority != 10 {
					t.Error("expected priority to be 10")
				}
				if template.Log == nil || template.Log.File == nil {
					t.Error("expected log configuration")
				}
			},
		},
		{
			name:         "worker_template",
			templateType: TypeWorker,
			processName:  "data-worker",
			expectError:  false,
			validate: func(t *testing.T, template *ProcessTemplate) {
				if template.Priority == nil || *template.Priority != 20 {
					t.Error("expected priority to be 20")
				}
				if template.Command != "./worker" {
					t.Errorf("unexpected command: %s", template.Command)
				}
			},
		},
		{
			name:         "database_template",
			templateType: TypeDatabase,
			processName:  "mongo-db",
			expectError:  false,
			validate: func(t *testing.T, template *ProcessTemplate) {
				if template.Priority == nil || *template.Priority != 5 {
					t.Error("expected priority to be 5")
				}
				if !strings.Contains(template.Command, "mongod") {
					t.Errorf("expected mongod command, got: %s", template.Command)
				}
			},
		},
		{
			name:         "cron_template",
			templateType: TypeCron,
			processName:  "daily-task",
			expectError:  false,
			validate: func(t *testing.T, template *ProcessTemplate) {
				if template.AutoRestart == nil || *template.AutoRestart {
					t.Error("expected auto_restart to be false for cron")
				}
				if template.Priority == nil || *template.Priority != 30 {
					t.Error("expected priority to be 30")
				}
			},
		},
		{
			name:         "simple_template",
			templateType: TypeSimple,
			processName:  "hello-world",
			expectError:  false,
			validate: func(t *testing.T, template *ProcessTemplate) {
				if template.AutoRestart != nil {
					t.Error("expected no auto_restart for simple template")
				}
				if template.Log != nil {
					t.Error("expected no log config for simple template")
				}
				if !strings.Contains(template.Command, "hello-world") {
					t.Errorf("expected command to contain process name, got: %s", template.Command)
				}
			},
		},
		{
			name:         "invalid_template",
			templateType: "invalid",
			processName:  "test",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template, err := generator.Generate(tt.templateType, tt.processName)

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

			if template == nil {
				t.Error("expected non-nil template")
				return
			}

			if tt.validate != nil {
				tt.validate(t, template)
			}
		})
	}
}

func TestGenerator_GenerateJSON(t *testing.T) {
	generator := NewGenerator()

	tests := []struct {
		name         string
		templateType TemplateType
		processName  string
		expectError  bool
	}{
		{
			name:         "web_json",
			templateType: TypeWeb,
			processName:  "web-app",
			expectError:  false,
		},
		{
			name:         "api_json",
			templateType: TypeAPI,
			processName:  "api-service",
			expectError:  false,
		},
		{
			name:         "simple_json",
			templateType: TypeSimple,
			processName:  "simple-app",
			expectError:  false,
		},
		{
			name:         "invalid_json",
			templateType: "invalid",
			processName:  "test",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := generator.GenerateJSON(tt.templateType, tt.processName)

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

			// Validate JSON format
			var result map[string]interface{}
			if err := json.Unmarshal(jsonData, &result); err != nil {
				t.Errorf("invalid JSON generated: %v", err)
				return
			}

			// Check basic structure
			if result["name"] != tt.processName {
				t.Errorf("expected name '%s', got '%v'", tt.processName, result["name"])
			}

			if result["command"] == nil {
				t.Error("expected command field")
			}

			// Verify JSON is properly formatted (indented)
			if !strings.Contains(string(jsonData), "\n") {
				t.Error("expected formatted JSON with newlines")
			}
		})
	}
}

func TestGenerator_GetSupportedTypes(t *testing.T) {
	generator := NewGenerator()
	types := generator.GetSupportedTypes()

	expectedTypes := []string{"web", "api", "worker", "database", "cron", "simple"}

	if len(types) != len(expectedTypes) {
		t.Errorf("expected %d supported types, got %d", len(expectedTypes), len(types))
	}

	typeMap := make(map[string]bool)
	for _, typ := range types {
		typeMap[typ] = true
	}

	for _, expected := range expectedTypes {
		if !typeMap[expected] {
			t.Errorf("expected type '%s' not found in supported types", expected)
		}
	}
}

func TestTemplateAliases(t *testing.T) {
	generator := NewGenerator()

	// Test that aliases work the same as primary types
	aliases := map[TemplateType]TemplateType{
		TypeWebapp:     TypeWeb,
		TypeService:    TypeAPI,
		TypeBackground: TypeWorker,
		TypeDB:         TypeDatabase,
		TypeScheduled:  TypeCron,
		TypeBasic:      TypeSimple,
	}

	for alias, primary := range aliases {
		t.Run(string(alias)+"_alias", func(t *testing.T) {
			aliasTemplate, err := generator.Generate(alias, "test")
			if err != nil {
				t.Errorf("unexpected error with alias '%s': %v", alias, err)
				return
			}

			primaryTemplate, err := generator.Generate(primary, "test")
			if err != nil {
				t.Errorf("unexpected error with primary '%s': %v", primary, err)
				return
			}

			// Commands should be the same (key identifier)
			if aliasTemplate.Command != primaryTemplate.Command {
				t.Errorf("alias '%s' and primary '%s' generate different commands", alias, primary)
			}
		})
	}
}

func TestTemplateToMap(t *testing.T) {
	generator := NewGenerator()

	// Test with a complex template
	autoRestart := true
	priority := 10
	template := &ProcessTemplate{
		Name:        "test-service",
		Command:     "./app",
		WorkDir:     "/app",
		AutoRestart: &autoRestart,
		Priority:    &priority,
		Log: &LogConfig{
			File: &FileLogConfig{
				Dir: "/var/log/test",
			},
		},
		Env: []string{"VAR1=value1", "VAR2=value2"},
		Extra: map[string]interface{}{
			"custom_field": "custom_value",
		},
	}

	result := generator.templateToMap(template)

	// Check all fields are present
	if result["name"] != "test-service" {
		t.Errorf("expected name 'test-service', got '%v'", result["name"])
	}

	if result["command"] != "./app" {
		t.Errorf("expected command './app', got '%v'", result["command"])
	}

	if result["work_dir"] != "/app" {
		t.Errorf("expected work_dir '/app', got '%v'", result["work_dir"])
	}

	if result["auto_restart"] != true {
		t.Errorf("expected auto_restart true, got '%v'", result["auto_restart"])
	}

	if result["priority"] != 10 {
		t.Errorf("expected priority 10, got '%v'", result["priority"])
	}

	if result["custom_field"] != "custom_value" {
		t.Errorf("expected custom_field 'custom_value', got '%v'", result["custom_field"])
	}

	// Check log structure
	logConfig, ok := result["log"].(map[string]interface{})
	if !ok {
		t.Error("expected log to be a map")
		return
	}

	fileConfig, ok := logConfig["file"].(map[string]interface{})
	if !ok {
		t.Error("expected log.file to be a map")
		return
	}

	if fileConfig["dir"] != "/var/log/test" {
		t.Errorf("expected log dir '/var/log/test', got '%v'", fileConfig["dir"])
	}

	// Check env array
	env, ok := result["env"].([]string)
	if !ok {
		t.Error("expected env to be []string")
		return
	}

	if len(env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(env))
	}
}
