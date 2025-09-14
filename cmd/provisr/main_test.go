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

func TestMetricsFlagStartsServer(t *testing.T) {
	// Start with metrics listen on a random high port and exit immediately via --help to ensure flag parsing path.
	// We cannot easily probe the server without races here, so just ensure it doesn't crash.
	cmd := exec.Command("go", "run", ".", "--metrics-listen", ":0", "--help")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("metrics --help should succeed: %v out=%s", err, out)
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

func TestBuildRootMetricsBinderHelp(t *testing.T) {
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()

	root.SetArgs([]string{"--metrics-listen", ":0", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute help with metrics flag: %v", err)
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

func TestStoreFlagsPrecedence(t *testing.T) {
	// Case 1: config enables store -> store file created
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()
	dir := t.TempDir()
	storeFile := filepath.Join(dir, "a.db")
	cfg := writeStoreCfg(t, dir, true, "sqlite://"+storeFile)
	root.SetArgs([]string{"--config", cfg, "status", "--name", "x"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute with config store: %v", err)
	}
	if _, err := os.Stat(storeFile); err != nil {
		t.Fatalf("expected sqlite file to be created, err=%v", err)
	}

	// Case 2: --no-store overrides enabled config -> file should NOT be created
	mgr2 := provisr.New()
	root2, bind2 := buildRoot(mgr2)
	bind2()
	file2 := filepath.Join(dir, "b.db")
	cfg2 := writeStoreCfg(t, dir, true, "sqlite://"+file2)
	root2.SetArgs([]string{"--config", cfg2, "--no-store", "status", "--name", "x"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("execute with --no-store: %v", err)
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Fatalf("expected sqlite file NOT to exist with --no-store, got err=%v", err)
	}

	// Case 3: --store-dsn overrides config -> target file3 created
	mgr3 := provisr.New()
	root3, bind3 := buildRoot(mgr3)
	bind3()
	file3 := filepath.Join(dir, "c.db")
	cfg3 := writeStoreCfg(t, dir, false, "sqlite://"+filepath.Join(dir, "unused.db"))
	root3.SetArgs([]string{"--config", cfg3, "--store-dsn", "sqlite://" + file3, "status", "--name", "x"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("execute with --store-dsn: %v", err)
	}
	if _, err := os.Stat(file3); err != nil {
		t.Fatalf("expected sqlite file3 to be created, err=%v", err)
	}
}

func TestServeConfigDisabledRequiresFlag(t *testing.T) {
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()
	dir := t.TempDir()
	cfg := writeHTTPCfg(t, dir, false, "", "/api")
	root.SetArgs([]string{"serve", "--config", cfg})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error when http_api.enabled=false and no --api-listen")
	}
	// Now override with flag and non-blocking -> success
	root.SetArgs([]string{"serve", "--config", cfg, "--api-listen", ":0", "--non-blocking"})
	if err := root.Execute(); err != nil {
		t.Fatalf("serve with override should succeed: %v", err)
	}
}

func TestServeConfigEnabledUsesConfig(t *testing.T) {
	mgr := provisr.New()
	root, bind := buildRoot(mgr)
	bind()
	dir := t.TempDir()
	cfg := writeHTTPCfg(t, dir, true, ":0", "/x")
	root.SetArgs([]string{"serve", "--config", cfg, "--non-blocking"})
	if err := root.Execute(); err != nil {
		t.Fatalf("serve with enabled config should succeed: %v", err)
	}
}
