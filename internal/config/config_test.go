package config

import (
	"os"
	"path/filepath"
	"testing"
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
