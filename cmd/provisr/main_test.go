package main

import (
	"bytes"
	"os"
	"os/exec"
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
