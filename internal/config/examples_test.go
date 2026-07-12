package config

import (
	"path/filepath"
	"testing"
)

func TestCronJobExampleConfigLoads(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "cronjob_basic", "cronjob_example.toml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(%q): %v", path, err)
	}
	if cfg.Server == nil {
		t.Fatal("example config must include a server section")
	}
	if len(cfg.CronJobs) != 1 || cfg.CronJobs[0].Name != "hello-cronjob" {
		t.Fatalf("CronJobs = %+v, want hello-cronjob", cfg.CronJobs)
	}
}
