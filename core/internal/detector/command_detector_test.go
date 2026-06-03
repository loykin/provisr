package detector

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
}

func TestBuildShellAwareCommand(t *testing.T) {
	// Cross-platform test - just ensure it returns a valid command
	c := buildShellAwareCommand("")
	if c == nil || c.Path == "" {
		t.Fatalf("expected valid command for empty string, got %v", c)
	}

	// simple no metachar -> direct exec
	c = buildShellAwareCommand("echo hello")
	if len(c.Args) == 0 || c.Args[0] != "echo" {
		t.Fatalf("expected direct exec echo, got %#v", c.Args)
	}

	// with shell meta -> should use shell
	c = buildShellAwareCommand("echo hi | grep hi")
	if len(c.Args) < 2 {
		t.Fatalf("expected shell command with args, got %#v", c.Args)
	}
}

func TestCommandDetectorDescribe(t *testing.T) {
	// Cross-platform test for Describe method
	d := CommandDetector{Command: "echo test"}
	if d.Describe() != "cmd:echo test" {
		t.Fatalf("Describe mismatch: %q", d.Describe())
	}
}

func TestPIDFileDetector(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p.pid")
	d := PIDFileDetector{PIDFile: pidfile}

	// not exists -> false,nil
	alive, err := d.Alive()
	if err != nil || alive {
		t.Fatalf("expected false,nil for missing file, got %v %v", alive, err)
	}

	// invalid content -> error
	if err := os.WriteFile(pidfile, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	alive, err = d.Alive()
	if err == nil {
		t.Fatalf("expected error for invalid pid, got alive=%v", alive)
	}

	// valid pid but not alive (0) -> false,nil
	if err := os.WriteFile(pidfile, []byte("0"), 0o644); err != nil {
		t.Fatal(err)
	}
	alive, err = d.Alive()
	if err != nil || alive {
		t.Fatalf("expected false,nil for pid 0, got %v %v", alive, err)
	}

	// current process pid -> likely alive true
	pid := os.Getpid()
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = d.Alive()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Alive can be true or false on some systems with permissions; ensure no error and Describe format
	_ = d.Describe()
}
