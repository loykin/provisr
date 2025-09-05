package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
	pg "github.com/loykin/provisr/internal/process_group"
)

func TestLoadSpecsFromTOML_Minimal(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "provisr.toml")
	data := `
[[processes]]
name = "demo"
command = "sleep 1"
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	specs, err := LoadSpecsFromTOML(file)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	s := specs[0]
	if s.Name != "demo" || s.Command != "sleep 1" {
		t.Fatalf("unexpected spec: %+v", s)
	}
}

func TestLoadSpecsFromTOML_Full_Base(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cfg.toml")
	data := `
[[processes]]
name = "web"
command = "sleep 2"
workdir = "/tmp"
env = ["A=1","B=2"]
pidfile = "/tmp/web.pid"
retries = 2
retry_interval = "200ms"
autorestart = true
restart_interval = "1s"
startsecs = "150ms"
instances = 3
  [[processes.detectors]]
  type = "pidfile"
  path = "/tmp/web.pid"
  [[processes.detectors]]
  type = "command"
  command = "true"
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	specs, err := LoadSpecsFromTOML(file)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	s := specs[0]
	if s.Name != "web" || s.Command != "sleep 2" || s.WorkDir != "/tmp" || len(s.Env) != 2 || s.PIDFile != "/tmp/web.pid" {
		t.Fatalf("unexpected base fields: %+v", s)
	}
	if s.RetryCount != 2 || s.RetryInterval.String() != "200ms" || s.StartDuration.String() != "150ms" || !s.AutoRestart || s.RestartInterval.String() != "1s" || s.Instances != 3 {
		t.Fatalf("unexpected control fields: %+v", s)
	}
}

func TestLoadSpecsFromTOML_Full_Detectors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cfg.toml")
	data := `
[[processes]]
name = "web"
command = "sleep 2"
workdir = "/tmp"
  [[processes.detectors]]
  type = "pidfile"
  path = "/tmp/web.pid"
  [[processes.detectors]]
  type = "command"
  command = "true"
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	specs, err := LoadSpecsFromTOML(file)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	s := specs[0]
	if len(s.Detectors) != 2 {
		t.Fatalf("expected 2 detectors, got %d", len(s.Detectors))
	}
}

func TestLoadGroupsFromTOML(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "groups.toml")
	data := `
[[processes]]
name = "a"
command = "sleep 1"

[[processes]]
name = "b"
command = "sleep 1"

[[groups]]
name = "g1"
members = ["a", "b"]
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	groups, err := LoadGroupsFromTOML(file)
	if err != nil {
		t.Fatalf("load groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "g1" || len(groups[0].Members) != 2 {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	// try starting the group to ensure specs are usable
	mgr := process.NewManager()
	g := pg.New(mgr)
	if err := g.Start(groups[0]); err != nil {
		t.Fatalf("group start: %v", err)
	}
	_ = g.Stop(groups[0], 2*time.Second)
}

func TestLoadCronJobsFromTOML(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cron.toml")
	data := `
[[processes]]
name = "job1"
command = "echo hi"
schedule = "@every 100ms"
singleton = true

[[processes]]
name = "bad"
command = "echo bad"
schedule = "@every 1s"
autorestart = true
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	// Call and discard jobs on the first (expected error) attempt to avoid SA4006
	_, err := LoadCronJobsFromTOML(file)
	if err == nil {
		t.Fatalf("expected error due to autorestart=true in scheduled process")
	}
	var jobs []CronJob // declare here for the second successful load
	// Fix and reload
	data2 := `
[[processes]]
name = "job1"
command = "echo hi"
schedule = "@every 100ms"
`
	if err := os.WriteFile(file, []byte(data2), 0o644); err != nil {
		t.Fatalf("write toml2: %v", err)
	}
	jobs, err = LoadCronJobsFromTOML(file)
	if err != nil {
		t.Fatalf("load cron: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(jobs))
	}
	if jobs[0].Name != "job1" || jobs[0].Schedule == "" {
		t.Fatalf("unexpected cron job: %+v", jobs[0])
	}
}

// This test verifies that top-level env and per-process env loaded via TOML
// are merged and applied when starting through Manager using loaded specs.
func TestConfigEnvMergeIntegration(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	cfg := filepath.Join(dir, "env.toml")
	data := `
env = ["GLOB=G", "CHAIN=${GLOB}-x"]
[[processes]]
name = "env-merge"
command = "sh -c 'echo $GLOB $CHAIN $PORT $LOCAL > ` + out + `'"
env = ["PORT=2000", "LOCAL=${GLOB}-y"]
`
	if err := os.WriteFile(cfg, []byte(data), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	mgr := process.NewManager()
	genv, err := LoadEnvFromTOML(cfg)
	if err != nil {
		t.Fatalf("load env: %v", err)
	}
	mgr.SetGlobalEnv(genv)
	specs, err := LoadSpecsFromTOML(cfg)
	if err != nil {
		t.Fatalf("load specs: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if err := mgr.Start(specs[0]); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(bytes.TrimSpace(b))
	if got != "G G-x 2000 G-y" {
		t.Fatalf("unexpected output: %q", got)
	}
}

// This test verifies that retry settings from TOML are honored: a command exiting early
// before startsecs triggers retries. We observe it via elapsed time.
func TestConfigRetryHonored(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "retry.toml")
	// Run a short command exiting in ~50ms; set startsecs=200ms to force start failure; retries=2, interval=100ms
	data := `
[[processes]]
name = "bad"
command = "sh -c 'sleep 0.05'"
retries = 2
retry_interval = "100ms"
startsecs = "200ms"
`
	if err := os.WriteFile(cfg, []byte(data), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	specs, err := LoadSpecsFromTOML(cfg)
	if err != nil {
		t.Fatalf("load specs: %v", err)
	}
	mgr := process.NewManager()
	start := time.Now()
	err = mgr.Start(specs[0])
	if err == nil {
		t.Fatalf("expected start error for failing command")
	}
	elapsed := time.Since(start)
	// With 2 retries and 100ms interval, total sleep ~ 2*100ms = 200ms plus minimal overhead.
	if elapsed < 180*time.Millisecond {
		t.Fatalf("expected elapsed around retries, got %v", elapsed)
	}
}
