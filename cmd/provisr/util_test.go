package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr"
)

func TestFindGroupByName(t *testing.T) {
	groups := []provisr.GroupSpec{{Name: "a"}, {Name: "b"}}
	if g := findGroupByName(groups, "b"); g == nil || g.Name != "b" {
		t.Fatalf("expected to find b, got %#v", g)
	}
	if g := findGroupByName(groups, "c"); g != nil {
		t.Fatalf("expected nil for missing group, got %#v", g)
	}
}

func TestPrintJSON(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { _ = w.Close(); os.Stdout = old; _ = r.Close() }()

	printJSON(map[string]int{"x": 1})
	_ = w.Close()
	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(r)
	s := outBuf.String()
	if !strings.Contains(s, "\"x\": 1") {
		t.Fatalf("unexpected JSON output: %q", s)
	}
}

func TestStartFromSpecsAndStatuses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}
	mgr := provisr.New()
	specs := []provisr.Spec{
		{Name: "u1", Command: "sleep 0.2", StartDuration: 10 * time.Millisecond},
		{Name: "u2", Command: "sleep 0.2", StartDuration: 10 * time.Millisecond},
	}
	if err := startFromSpecs(mgr, specs); err != nil {
		t.Fatalf("startFromSpecs: %v", err)
	}
	m := statusesByBase(mgr, specs)
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	_ = mgr.StopAll("u", 200*time.Millisecond)
}

func writeEnvTOML(t *testing.T, dir string, env []string) string {
	content := "env = [\n"
	for i, kv := range env {
		content += "\t\"" + kv + "\""
		if i < len(env)-1 {
			content += ","
		}
		content += "\n"
	}
	content += "]\n"
	p := filepath.Join(dir, "env.toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	return p
}

func TestApplyGlobalEnvFromFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix echo/sleep")
	}
	mgr := provisr.New()
	// Provide env via KVs and file; then start a process that writes env to a file.
	tdir := t.TempDir()
	envFile := writeEnvTOML(t, tdir, []string{"A=1", "B=2"})
	applyGlobalEnvFromFlags(mgr, false, []string{envFile}, []string{"C=3"})

	// Prepare a spec that prints a composite value from env
	outFile := filepath.Join(tdir, "out.txt")
	cmd := "sh -c 'echo " + "${A}-${B}-${C}" + " > " + outFile + "'"
	spec := provisr.Spec{Name: "envtest", Command: cmd, StartDuration: 0}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	// wait a bit and then check file
	time.Sleep(150 * time.Millisecond)
	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	s := strings.TrimSpace(string(b))
	if s != "1-2-3" {
		t.Fatalf("unexpected env expansion, got %q want %q", s, "1-2-3")
	}
	_ = mgr.StopAll("envtest", 200*time.Millisecond)
}

func TestPrintDetailedStatus(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test with empty statuses
	printDetailedStatus([]provisr.Status{})

	// Test with statuses
	statuses := []provisr.Status{
		{
			Name:       "test-process",
			State:      "running",
			Running:    true,
			PID:        1234,
			Restarts:   2,
			StartedAt:  time.Now().Add(-5 * time.Minute),
			DetectedBy: "exec:pid",
		},
		{
			Name:       "stopped-process",
			State:      "stopped",
			Running:    false,
			PID:        0,
			Restarts:   0,
			StartedAt:  time.Time{},
			DetectedBy: "",
		},
	}
	printDetailedStatus(statuses)

	// Restore stdout and read output
	w.Close()
	os.Stdout = old
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "test-process") {
		t.Error("Expected output to contain process name")
	}
	if !strings.Contains(output, "No processes found") {
		t.Error("Expected output to show no processes message for empty list")
	}
}

func TestPrintDetailedStatusByBase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix sleep")
	}

	// Create a test manager
	mgr := provisr.New()

	// Start a test process
	spec := provisr.Spec{
		Name:          "detail-test",
		Command:       "sleep 0.1",
		StartDuration: 10 * time.Millisecond,
	}
	err := mgr.Start(spec)
	if err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	defer mgr.Stop("detail-test", 100*time.Millisecond)

	// Wait a moment for process to start
	time.Sleep(50 * time.Millisecond)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test printDetailedStatusByBase
	specs := []provisr.Spec{spec}
	printDetailedStatusByBase(mgr, specs)

	// Restore stdout and read output
	w.Close()
	os.Stdout = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "detail-test") {
		t.Error("Expected output to contain process name in detailed status by base")
	}
}

func TestGetUptime(t *testing.T) {
	now := time.Now()

	// Test not running process
	st := provisr.Status{Running: false}
	uptime := getUptime(st)
	if uptime != "N/A" {
		t.Errorf("Expected N/A for non-running process, got %s", uptime)
	}

	// Test zero started time
	st = provisr.Status{Running: true, StartedAt: time.Time{}}
	uptime = getUptime(st)
	if uptime != "Unknown" {
		t.Errorf("Expected Unknown for zero start time, got %s", uptime)
	}

	// Test seconds uptime
	st = provisr.Status{Running: true, StartedAt: now.Add(-30 * time.Second)}
	uptime = getUptime(st)
	if !strings.HasSuffix(uptime, "s") {
		t.Errorf("Expected seconds format, got %s", uptime)
	}

	// Test minutes uptime
	st = provisr.Status{Running: true, StartedAt: now.Add(-5 * time.Minute)}
	uptime = getUptime(st)
	if !strings.HasSuffix(uptime, "m") {
		t.Errorf("Expected minutes format, got %s", uptime)
	}

	// Test hours uptime
	st = provisr.Status{Running: true, StartedAt: now.Add(-2*time.Hour - 30*time.Minute)}
	uptime = getUptime(st)
	if !strings.Contains(uptime, "h") || !strings.Contains(uptime, "m") {
		t.Errorf("Expected hours:minutes format, got %s", uptime)
	}
}
