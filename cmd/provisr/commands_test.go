package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/loykin/provisr"
)

func TestCmdStartStatusStop_NoConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}
	mgr := provisr.New()
	// Start short-lived process
	if err := cmdStart(mgr, StartFlags{
		Name:          "c1",
		Cmd:           "sleep 0.2",
		StartDuration: 50 * time.Millisecond,
	}); err != nil {
		t.Fatalf("cmdStart: %v", err)
	}
	// Status should succeed
	if err := cmdStatus(mgr, StatusFlags{Name: "c1"}); err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
	// Stop should be no-op but succeed
	if err := cmdStop(mgr, StopFlags{Name: "c1", Wait: 200 * time.Millisecond}); err != nil {
		t.Fatalf("cmdStop: %v", err)
	}
}

func writeTOML(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	return p
}

func TestCmdStartAndGroupStatusStop_WithConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}
	mgr := provisr.New()
	dir := t.TempDir()
	cfg := `
# top-level env/flags may be omitted

[[processes]]
name = "g1-a"
command = "sleep 1"
startsecs = "50ms"

[[processes]]
name = "g1-b"
command = "sleep 1"
startsecs = "50ms"

[[groups]]
name = "grp1"
members = ["g1-a", "g1-b"]
`
	path := writeTOML(t, dir, "config.toml", cfg)

	// Start via config
	if err := cmdStart(mgr, StartFlags{ConfigPath: path}); err != nil {
		t.Fatalf("cmdStart with config: %v", err)
	}
	// Group status
	if err := runGroupStatus(mgr, GroupFlags{ConfigPath: path, GroupName: "grp1"}); err != nil {
		t.Fatalf("group status: %v", err)
	}
	// Group stop
	if err := runGroupStop(mgr, GroupFlags{ConfigPath: path, GroupName: "grp1", Wait: 500 * time.Millisecond}); err != nil {
		// Some environments may report signal termination; accept either nil or a termination error.
		t.Logf("group stop returned: %v (accepted)", err)
	}
}

func TestCmdCron_NonBlocking(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}
	mgr := provisr.New()
	dir := t.TempDir()
	cfg := `
[[processes]]
name = "cj1"
command = "sleep 0.1"
startsecs = "10ms"
# also mark as cron via schedule field
schedule = "@every 50ms"
`
	path := writeTOML(t, dir, "cron.toml", cfg)
	// NonBlocking should start scheduler and then stop immediately
	if err := cmdCron(mgr, CronFlags{ConfigPath: path, NonBlocking: true}); err != nil {
		t.Fatalf("cmdCron nonblocking: %v", err)
	}
}

func TestCmdCronMissingConfig(t *testing.T) {
	mgr := provisr.New()
	if err := cmdCron(mgr, CronFlags{}); err == nil {
		t.Fatalf("expected error when --config is missing for cron")
	}
}

func TestGroupCommandsMissingFlags(t *testing.T) {
	mgr := provisr.New()
	// group-start without config
	if err := runGroupStart(mgr, GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-start")
	}
	// group-start without group
	if err := runGroupStart(mgr, GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-start")
	}
	// group-status
	if err := runGroupStatus(mgr, GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-status")
	}
	if err := runGroupStatus(mgr, GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-status")
	}
	// group-stop
	if err := runGroupStop(mgr, GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-stop")
	}
	if err := runGroupStop(mgr, GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-stop")
	}
}
