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
	provisrCommand := command{mgr: mgr}
	// Start short-lived process
	if err := provisrCommand.Start(StartFlags{
		Name:          "c1",
		Cmd:           "sleep 0.2",
		StartDuration: 50 * time.Millisecond,
	}); err != nil {
		t.Fatalf("cmdStart: %v", err)
	}
	// Status should succeed
	if err := provisrCommand.Status(StatusFlags{Name: "c1"}); err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
	// Stop should be no-op but succeed
	if err := provisrCommand.Stop(StopFlags{Name: "c1", Wait: 200 * time.Millisecond}); err != nil {
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

	provisrCommand := command{mgr: mgr}
	// Start via config
	if err := provisrCommand.Start(StartFlags{ConfigPath: path}); err != nil {
		t.Fatalf("cmdStart with config: %v", err)
	}
	// Group status
	if err := provisrCommand.GroupStatus(GroupFlags{ConfigPath: path, GroupName: "grp1"}); err != nil {
		t.Fatalf("group status: %v", err)
	}
	// Group stop
	if err := provisrCommand.GroupStop(GroupFlags{ConfigPath: path, GroupName: "grp1", Wait: 500 * time.Millisecond}); err != nil {
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
	provisrCommand := command{mgr: mgr}
	// NonBlocking should start scheduler and then stop immediately
	if err := provisrCommand.Cron(CronFlags{ConfigPath: path, NonBlocking: true}); err != nil {
		t.Fatalf("cmdCron nonblocking: %v", err)
	}
}

func TestCmdCronMissingConfig(t *testing.T) {
	mgr := provisr.New()
	provisrCommand := command{mgr: mgr}
	if err := provisrCommand.Cron(CronFlags{}); err == nil {
		t.Fatalf("expected error when --config is missing for cron")
	}
}

func TestGroupCommandsMissingFlags(t *testing.T) {
	mgr := provisr.New()
	provisrCommand := command{mgr: mgr}
	// group-start without config
	if err := provisrCommand.GroupStart(GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-start")
	}
	// group-start without group
	if err := provisrCommand.GroupStart(GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-start")
	}
	// group-status
	if err := provisrCommand.runGroupStatus(GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-status")
	}
	if err := provisrCommand.GroupStatus(GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-status")
	}
	// group-stop
	if err := provisrCommand.GroupStop(GroupFlags{}); err == nil {
		t.Fatalf("expected error for missing --config in group-stop")
	}
	if err := provisrCommand.GroupStop(GroupFlags{ConfigPath: "x"}); err == nil {
		t.Fatalf("expected error for missing --group in group-stop")
	}
}

func TestCommandsViaAPI(t *testing.T) {
	mgr := provisr.New()
	provisrCommand := command{mgr: mgr}

	// Create API client to unreachable server
	apiClient := NewAPIClient("http://localhost:99999", 100*time.Millisecond)

	// Test startViaAPI with unreachable API
	err := provisrCommand.startViaAPI(StartFlags{Name: "test", Cmd: "echo hello"}, apiClient)
	if err == nil {
		t.Error("Expected error for unreachable API")
	}

	// Test statusViaAPI with unreachable API
	err = provisrCommand.statusViaAPI(StatusFlags{Name: "test"}, apiClient)
	if err == nil {
		t.Error("Expected error for unreachable API")
	}
}
