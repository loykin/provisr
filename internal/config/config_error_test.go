package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSpecsUnknownDetector(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[processes]]
name = "x"
command = "true"
[[processes.detectors]]
type = "unknown"
`
	p := filepath.Join(dir, "c.toml")
	if err := os.WriteFile(p, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSpecsFromTOML(p); err == nil {
		t.Fatalf("expected error for unknown detector type")
	}
}

func TestLoadCronJobsInvalidFlags(t *testing.T) {
	dir := t.TempDir()
	// autorestart true -> error
	toml1 := `
[[processes]]
name = "a"
command = "true"
schedule = "@every 1s"
autorestart = true
`
	p1 := filepath.Join(dir, "a.toml")
	if err := os.WriteFile(p1, []byte(toml1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCronJobsFromTOML(p1); err == nil {
		t.Fatalf("expected error for autorestart=true in cron job")
	}
	// instances > 1 -> error
	toml2 := `
[[processes]]
name = "b"
command = "true"
schedule = "@every 1s"
instances = 2
`
	p2 := filepath.Join(dir, "b.toml")
	if err := os.WriteFile(p2, []byte(toml2), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCronJobsFromTOML(p2); err == nil {
		t.Fatalf("expected error for instances>1 in cron job")
	}
}

func TestLoadEnvFileInvalidPath(t *testing.T) {
	if _, err := LoadEnvFile("/definitely/not/exist.env"); err == nil {
		t.Fatalf("expected error for missing env file")
	}
}
