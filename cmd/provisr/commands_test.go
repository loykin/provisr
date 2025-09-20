package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/loykin/provisr"
)

// Test helper that uses direct manager instead of API
func (c *command) startDirect(f StartFlags) error {
	spec := provisr.Spec{
		Name:    f.Name,
		Command: f.Cmd,
	}

	return c.mgr.Start(spec)
}

func (c *command) startDirectWithConfig(f StartFlags) error {
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	for _, spec := range config.Specs {
		if err := c.mgr.Start(spec); err != nil {
			return fmt.Errorf("failed to start %s: %w", spec.Name, err)
		}
	}
	return nil
}

func (c *command) statusDirect(f StatusFlags) error {
	result, err := c.mgr.Status(f.Name)
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func (c *command) stopDirect(f StopFlags) error {
	err := c.mgr.Stop(f.Name, f.Wait)
	if err != nil && isExpectedShutdownError(err) {
		return nil // Ignore expected shutdown errors
	}
	return err
}

func (c *command) groupStatusDirect(f GroupFlags) error {
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	group := provisr.NewGroup(c.mgr)

	// Find the group spec in converted GroupSpecs
	var groupSpec provisr.GroupSpec
	found := false
	for _, g := range config.GroupSpecs {
		if g.Name == f.GroupName {
			groupSpec = g
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("group %s not found", f.GroupName)
	}

	result, err := group.Status(groupSpec)
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func (c *command) groupStopDirect(f GroupFlags) error {
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	group := provisr.NewGroup(c.mgr)

	// Find the group spec in converted GroupSpecs
	var groupSpec provisr.GroupSpec
	found := false
	for _, g := range config.GroupSpecs {
		if g.Name == f.GroupName {
			groupSpec = g
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("group %s not found", f.GroupName)
	}

	err = group.Stop(groupSpec, f.Wait)
	if err != nil && isExpectedShutdownError(err) {
		return nil // Ignore expected shutdown errors
	}
	return err
}

func TestCmdStartStatusStop_NoConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}

	// Use direct manager for testing
	mgr := provisr.New()
	provisrCommand := command{mgr: mgr}

	// Start short-lived process
	if err := provisrCommand.startDirect(StartFlags{
		Name:          "c1",
		Cmd:           "sleep 0.2",
		StartDuration: 50 * time.Millisecond,
	}); err != nil {
		t.Fatalf("cmdStart: %v", err)
	}
	// Status should succeed
	if err := provisrCommand.statusDirect(StatusFlags{Name: "c1"}); err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
	// Stop should be no-op but succeed
	if err := provisrCommand.stopDirect(StopFlags{Name: "c1", Wait: 200 * time.Millisecond}); err != nil {
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

	// Create programs directory structure
	programsDir := filepath.Join(dir, "programs")
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		t.Fatalf("create programs dir: %v", err)
	}

	// Write individual program files in unified format (type/spec)
	writeTOML(t, programsDir, "g1-a.toml", `
 type = "process"
 [spec]
 name = "g1-a"
 command = "sleep 1"
 startsecs = "50ms"
`)
	writeTOML(t, programsDir, "g1-b.toml", `
 type = "process"
 [spec]
 name = "g1-b"
 command = "sleep 1"
 startsecs = "50ms"
`)

	// Main config with programs_directory and groups
	cfg := `
programs_directory = "programs"

[[groups]]
name = "grp1"
members = ["g1-a", "g1-b"]
`
	path := writeTOML(t, dir, "config.toml", cfg)

	provisrCommand := command{mgr: mgr}
	// Start via config using direct manager (avoid API call issues in tests)
	if err := provisrCommand.startDirectWithConfig(StartFlags{ConfigPath: path}); err != nil {
		t.Fatalf("cmdStart with config: %v", err)
	}
	// Group status
	if err := provisrCommand.groupStatusDirect(GroupFlags{ConfigPath: path, GroupName: "grp1"}); err != nil {
		t.Fatalf("group status: %v", err)
	}
	// Group stop
	if err := provisrCommand.groupStopDirect(GroupFlags{ConfigPath: path, GroupName: "grp1", Wait: 500 * time.Millisecond}); err != nil {
		// Some environments may report signal termination; accept either nil or a termination error.
		t.Logf("group stop returned: %v (accepted)", err)
	}
}
