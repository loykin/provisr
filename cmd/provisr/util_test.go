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
