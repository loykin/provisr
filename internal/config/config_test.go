package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

func TestLoadConfig_Minimal(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.toml")
	data := `
use_os_env = false
env_files = []
env = []
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	config, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Check default values
	if config.UseOSEnv != false {
		t.Errorf("expected UseOSEnv=false, got %v", config.UseOSEnv)
	}
	if len(config.EnvFiles) != 0 {
		t.Errorf("expected empty EnvFiles, got %v", config.EnvFiles)
	}
	if len(config.Env) != 0 {
		t.Errorf("expected empty Env, got %v", config.Env)
	}

	// Computed fields should be empty
	if len(config.Specs) != 0 {
		t.Errorf("expected empty Specs, got %d", len(config.Specs))
	}
	if len(config.CronJobs) != 0 {
		t.Errorf("expected empty CronJobs, got %d", len(config.CronJobs))
	}
}

func TestLoadConfig_WithEnvironment(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.toml")
	data := `
use_os_env = true
env = ["APP_NAME=provisr", "DEBUG=true"]
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	config, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Check environment settings
	if !config.UseOSEnv {
		t.Errorf("expected UseOSEnv=true, got %v", config.UseOSEnv)
	}
	if len(config.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(config.Env))
	}

	// GlobalEnv should be computed
	if len(config.GlobalEnv) < 2 {
		t.Errorf("expected GlobalEnv to contain at least 2 entries, got %d", len(config.GlobalEnv))
	}
}

func TestLoadConfig_WithServer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.toml")
	data := `
[server]
listen = ":8080"
base_path = "/api"
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	config, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if config.Server == nil {
		t.Fatal("expected Server config, got nil")
	}
	if config.Server.Listen != ":8080" {
		t.Errorf("expected Listen ':8080', got %s", config.Server.Listen)
	}
	if config.Server.BasePath != "/api" {
		t.Errorf("expected BasePath '/api', got %s", config.Server.BasePath)
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/file.toml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestComputeGlobalEnv_Simple(t *testing.T) {
	env, err := computeGlobalEnv(false, []string{}, []string{"TEST=value", "APP=test"})
	if err != nil {
		t.Fatalf("computeGlobalEnv error: %v", err)
	}

	if len(env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(env))
	}

	// Should be sorted
	expected := []string{"APP=test", "TEST=value"}
	for i, expectedVar := range expected {
		if i >= len(env) || env[i] != expectedVar {
			t.Errorf("expected env[%d] = %s, got %s", i, expectedVar, env[i])
		}
	}
}

func TestStringToDurationHook(t *testing.T) {
	hook := stringToDurationHook()

	tests := []struct {
		name     string
		from     reflect.Type
		to       reflect.Type
		data     interface{}
		expected interface{}
		hasError bool
	}{
		{
			name:     "valid duration string",
			from:     reflect.TypeOf(""),
			to:       reflect.TypeOf(time.Duration(0)),
			data:     "5s",
			expected: 5 * time.Second,
			hasError: false,
		},
		{
			name:     "empty string",
			from:     reflect.TypeOf(""),
			to:       reflect.TypeOf(time.Duration(0)),
			data:     "",
			expected: time.Duration(0),
			hasError: false,
		},
		{
			name:     "whitespace string",
			from:     reflect.TypeOf(""),
			to:       reflect.TypeOf(time.Duration(0)),
			data:     "   ",
			expected: time.Duration(0),
			hasError: false,
		},
		{
			name:     "invalid duration string",
			from:     reflect.TypeOf(""),
			to:       reflect.TypeOf(time.Duration(0)),
			data:     "invalid",
			expected: nil,
			hasError: true,
		},
		{
			name:     "non-string to duration (passthrough)",
			from:     reflect.TypeOf(123),
			to:       reflect.TypeOf(time.Duration(0)),
			data:     123,
			expected: 123,
			hasError: false,
		},
		{
			name:     "string to non-duration (passthrough)",
			from:     reflect.TypeOf(""),
			to:       reflect.TypeOf(""),
			data:     "test",
			expected: "test",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := hook.(func(reflect.Type, reflect.Type, interface{}) (interface{}, error))(tt.from, tt.to, tt.data)

			if tt.hasError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestDecodeTo(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]any
		expectErr bool
	}{
		{
			name: "valid process spec",
			input: map[string]any{
				"name":    "test-process",
				"command": "echo hello",
			},
			expectErr: false,
		},
		{
			name: "with duration",
			input: map[string]any{
				"name":           "test-process",
				"command":        "echo hello",
				"start_duration": "5s",
			},
			expectErr: false,
		},
		{
			name: "invalid duration",
			input: map[string]any{
				"name":           "test-process",
				"command":        "echo hello",
				"start_duration": "invalid-duration",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeTo[process.Spec](tt.input)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result.Name != tt.input["name"] {
					t.Errorf("expected name %v, got %v", tt.input["name"], result.Name)
				}
			}
		})
	}
}

func TestDecodeProcessEntry(t *testing.T) {
	tests := []struct {
		name        string
		pc          ProcessConfig
		ctx         string
		expectSpec  bool
		expectCron  bool
		expectErr   bool
		errContains string
	}{
		{
			name: "valid process entry",
			pc: ProcessConfig{
				Type: "process",
				Spec: map[string]any{
					"name":    "test-process",
					"command": "echo hello",
				},
			},
			ctx:        "test context",
			expectSpec: true,
			expectCron: false,
			expectErr:  false,
		},
		{
			name: "empty type defaults to process",
			pc: ProcessConfig{
				Type: "",
				Spec: map[string]any{
					"name":    "test-process",
					"command": "echo hello",
				},
			},
			ctx:        "test context",
			expectSpec: true,
			expectCron: false,
			expectErr:  false,
		},
		{
			name: "valid cronjob entry",
			pc: ProcessConfig{
				Type: "cronjob",
				Spec: map[string]any{
					"name": "test-cron",
					"spec": map[string]any{
						"name":    "test-cron-spec",
						"command": "echo hello",
					},
					"schedule": "@every 1m",
				},
			},
			ctx:        "test context",
			expectSpec: true,
			expectCron: true,
			expectErr:  false,
		},
		{
			name: "invalid process type",
			pc: ProcessConfig{
				Type: "invalid",
				Spec: map[string]any{
					"name":    "test",
					"command": "echo hello",
				},
			},
			ctx:         "test context",
			expectSpec:  false,
			expectCron:  false,
			expectErr:   true,
			errContains: "unknown process type",
		},
		{
			name: "invalid process spec",
			pc: ProcessConfig{
				Type: "process",
				Spec: map[string]any{
					"name":    "", // invalid empty name
					"command": "echo hello",
				},
			},
			ctx:         "test context",
			expectSpec:  false,
			expectCron:  false,
			expectErr:   true,
			errContains: "decode process spec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, cron, err := decodeProcessEntry(tt.pc, tt.ctx)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if tt.expectSpec && spec.Name == "" {
					t.Error("expected valid spec but got empty name")
				}
				if !tt.expectSpec && spec.Name != "" {
					t.Error("expected no spec but got one")
				}
				if tt.expectCron && cron == nil {
					t.Error("expected cron job but got nil")
				}
				if !tt.expectCron && cron != nil {
					t.Error("expected no cron job but got one")
				}
			}
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		expected    map[string]string
		expectErr   bool
		errContains string
	}{
		{
			name:    "valid env file",
			content: "VAR1=value1\nVAR2=value2\n",
			expected: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			expectErr: false,
		},
		{
			name:    "env file with comments",
			content: "# This is a comment\nVAR1=value1\n# Another comment\nVAR2=value2\n",
			expected: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			expectErr: false,
		},
		{
			name:    "env file with empty lines",
			content: "VAR1=value1\n\nVAR2=value2\n\n",
			expected: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			expectErr: false,
		},
		{
			name:    "env file with quoted values",
			content: `VAR1="quoted value"` + "\n" + `VAR2='single quoted'` + "\n",
			expected: map[string]string{
				"VAR1": "quoted value",
				"VAR2": "single quoted",
			},
			expectErr: false,
		},
		{
			name:    "env file with whitespace",
			content: "  VAR1  =  value1  \n  VAR2=value2\n",
			expected: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			expectErr: false,
		},
		{
			name:        "invalid env line",
			content:     "INVALID_LINE_WITHOUT_EQUALS\n",
			expected:    nil,
			expectErr:   true,
			errContains: "invalid env line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envFile := filepath.Join(tmpDir, tt.name+".env")
			err := os.WriteFile(envFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			result, err := loadEnvFile(envFile)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := loadEnvFile("/nonexistent/path/file.env")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		if !strings.Contains(err.Error(), "failed to read env file") {
			t.Errorf("expected file read error, got: %v", err)
		}
	})
}

func TestConvertDetectorConfigs(t *testing.T) {
	tests := []struct {
		name         string
		spec         *process.Spec
		expectErr    bool
		errContains  string
		numDetectors int
	}{
		{
			name: "no detector configs",
			spec: &process.Spec{
				DetectorConfigs: nil,
			},
			expectErr:    false,
			numDetectors: 0,
		},
		{
			name: "empty detector configs",
			spec: &process.Spec{
				DetectorConfigs: []process.DetectorConfig{},
			},
			expectErr:    false,
			numDetectors: 0,
		},
		{
			name: "pidfile detector",
			spec: &process.Spec{
				DetectorConfigs: []process.DetectorConfig{
					{Type: "pidfile", Path: "/tmp/test.pid"},
				},
			},
			expectErr:    false,
			numDetectors: 1,
		},
		{
			name: "command detector",
			spec: &process.Spec{
				DetectorConfigs: []process.DetectorConfig{
					{Type: "command", Command: "pgrep test"},
				},
			},
			expectErr:    false,
			numDetectors: 1,
		},
		{
			name: "multiple detectors",
			spec: &process.Spec{
				DetectorConfigs: []process.DetectorConfig{
					{Type: "pidfile", Path: "/tmp/test.pid"},
					{Type: "command", Command: "pgrep test"},
				},
			},
			expectErr:    false,
			numDetectors: 2,
		},
		{
			name: "unsupported detector type",
			spec: &process.Spec{
				DetectorConfigs: []process.DetectorConfig{
					{Type: "unsupported", Path: "/tmp/test.pid"},
				},
			},
			expectErr:   true,
			errContains: "unknown detector type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := convertDetectorConfigs(tt.spec)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(tt.spec.Detectors) != tt.numDetectors {
					t.Errorf("expected %d detectors, got %d", tt.numDetectors, len(tt.spec.Detectors))
				}
			}
		})
	}
}
