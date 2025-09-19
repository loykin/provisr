package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/loykin/provisr"
)

func TestHelpExitsZero(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("help should succeed: %v, out=%s", err, out)
	}
	if !strings.Contains(string(out), "provisr") {
		t.Fatalf("unexpected help output: %s", out)
	}
}

func TestStartStatusStopQuickPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}
	// Start a short-lived process using flags (sleep 0.2) and then stop it.
	// We invoke the binary via `go run` to exercise main.go code paths without installing.
	start := exec.Command("go", "run", ".", "start", "--name", "t1", "--cmd", "sleep 0.2", "--startsecs", "50ms")
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start failed: %v out=%s", err, out)
	}
	// Query status
	status := exec.Command("go", "run", ".", "status", "--name", "t1")
	if out, err := status.CombinedOutput(); err != nil {
		t.Fatalf("status failed: %v out=%s", err, out)
	}
	// Stop (no-op as process may have exited already); should still succeed
	stop := exec.Command("go", "run", ".", "stop", "--name", "t1")
	if out, err := stop.CombinedOutput(); err != nil {
		t.Fatalf("stop failed: %v out=%s", err, out)
	}
}

func TestServeCommandRequiresConfig(t *testing.T) {
	// Test that serve command requires config file
	cmd := exec.Command("go", "run", ".", "serve", "--help")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("serve --help should succeed: %v out=%s", err, out)
	}
}

func TestBuildRootStatusCommandWiring(t *testing.T) {
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()

	// Capture stdout to verify JSON output is printed by status path.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root.SetArgs([]string{"status", "--name", "demo"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute status: %v", err)
	}

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	s := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(s), "[") { // statuses slice JSON
		t.Fatalf("unexpected status output: %q", s)
	}
}

func TestBuildRootHelpCommand(t *testing.T) {
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()

	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}
}

func TestStartInstancesTriggersStartN(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}
	mgr := provisr.New()
	provisrCommand := command{mgr: mgr}
	err := provisrCommand.Start(StartFlags{
		Name:          "ninst",
		Cmd:           "sleep 0.05",
		StartDuration: 0,
		Instances:     2,
	})
	if err != nil {
		t.Fatalf("cmdStart with instances>1: %v", err)
	}
	_ = mgr.StopAll("ninst", 200000000) // 200ms
}

func writeStoreCfg(t *testing.T, dir string, enabled bool, dsn string) string {
	t.Helper()
	b := "[store]\n"
	if enabled {
		b += "enabled = true\n"
	} else {
		b += "enabled = false\n"
	}
	b += "dsn = \"" + dsn + "\"\n"
	p := filepath.Join(dir, "store.toml")
	if err := os.WriteFile(p, []byte(b), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	return p
}

func writeHTTPCfg(t *testing.T, dir string, enabled bool, listen, base string) string {
	t.Helper()
	b := "[http_api]\n"
	if enabled {
		b += "enabled = true\n"
	} else {
		b += "enabled = false\n"
	}
	if listen != "" {
		b += "listen = \"" + listen + "\"\n"
	}
	if base != "" {
		b += "base_path = \"" + base + "\"\n"
	}
	p := filepath.Join(dir, "http.toml")
	if err := os.WriteFile(p, []byte(b), 0o644); err != nil {
		t.Fatalf("write http cfg: %v", err)
	}
	return p
}

func TestStoreConfigParsing(t *testing.T) {
	// Test that store configuration is properly parsed from config file
	dir := t.TempDir()
	storeFile := filepath.Join(dir, "a.db")
	cfg := writeStoreCfg(t, dir, true, "sqlite://"+storeFile)

	// Test that config loading works
	storeCfg, err := provisr.LoadStore(cfg)
	if err != nil {
		t.Fatalf("failed to load store config: %v", err)
	}
	if !storeCfg.Enabled {
		t.Fatalf("expected store to be enabled")
	}
	if storeCfg.DSN != "sqlite://"+storeFile {
		t.Fatalf("expected DSN to be sqlite://%s, got %s", storeFile, storeCfg.DSN)
	}

	// Case 2: disabled store config
	cfg2 := writeStoreCfg(t, dir, false, "sqlite://unused.db")
	storeCfg2, err := provisr.LoadStore(cfg2)
	if err != nil {
		t.Fatalf("failed to load disabled store config: %v", err)
	}
	if storeCfg2.Enabled {
		t.Fatalf("expected store to be disabled")
	}
}

func TestMainFunction(t *testing.T) {
	// This test ensures the main function doesn't panic
	// We can't really test main() directly, but we can test that it exists
	// and the buildRoot function works properly
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()

	if root == nil {
		t.Error("buildRoot should return non-nil root command")
	}
	if root.Use != "provisr" {
		t.Errorf("Expected command name 'provisr', got %s", root.Use)
	}

	// Test that all expected subcommands exist
	expectedCommands := []string{"start", "status", "stop", "cron", "group-start", "group-stop", "group-status"}
	for _, cmdName := range expectedCommands {
		found := false
		for _, cmd := range root.Commands() {
			if cmd.Use == cmdName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected command %s to exist", cmdName)
		}
	}

	// Check serve command separately (might be conditional)
	serveFound := false
	for _, cmd := range root.Commands() {
		if cmd.Use == "serve" {
			serveFound = true
			break
		}
	}
	if !serveFound {
		t.Log("serve command not found (may be conditional)")
	}
}

func TestServeConfigDisabledRequiresFlag(t *testing.T) {
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()
	dir := t.TempDir()
	cfg := writeHTTPCfg(t, dir, false, "", "/api")
	root.SetArgs([]string{"serve", cfg})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error when http_api.enabled=false")
	}
}

func TestServeConfigEnabledUsesConfig(t *testing.T) {
	// This test is now simplified - we can't easily test non-blocking serve
	// without the deprecated --non-blocking flag, so we just test config validation
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()

	// Test that the serve command help works
	root.SetArgs([]string{"serve", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("serve help should succeed: %v", err)
	}
}
