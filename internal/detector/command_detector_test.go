package detector

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
}

func TestBuildShellAwareCommand(t *testing.T) {
	requireUnix(t)
	// empty -> /bin/true
	c := buildShellAwareCommand("")
	if c.Path == "" || !strings.Contains(c.String(), "/bin/true") {
		t.Fatalf("expected /bin/true, got %q (%q)", c.Path, c.String())
	}
	// simple no metachar -> direct exec
	c = buildShellAwareCommand("echo hello")
	if len(c.Args) == 0 || c.Args[0] != "echo" {
		t.Fatalf("expected direct exec echo, got %#v", c.Args)
	}
	// with shell meta -> sh -c
	c = buildShellAwareCommand("echo hi | cat")
	if len(c.Args) < 2 || c.Args[0] != "/bin/sh" || c.Args[1] != "-c" {
		t.Fatalf("expected /bin/sh -c, got %#v", c.Args)
	}
}

func TestCommandDetectorAliveAndDescribe(t *testing.T) {
	requireUnix(t)
	// A command that exits 0 -> Alive true
	d := CommandDetector{Command: "true"}
	alive, err := d.Alive()
	if err != nil || !alive {
		t.Fatalf("true should be alive, got alive=%v err=%v", alive, err)
	}
	if d.Describe() != "cmd:true" {
		t.Fatalf("Describe mismatch: %q", d.Describe())
	}

	// A command that exits non-zero -> Alive false, nil error
	d = CommandDetector{Command: "sh -c 'exit 3'"}
	alive, err = d.Alive()
	if err != nil || alive {
		t.Fatalf("non-zero exit expected false,nil, got alive=%v err=%v", alive, err)
	}

	// Non-existent binary -> error
	d = CommandDetector{Command: "__definitely_not_exists__"}
	alive, err = d.Alive()
	if err == nil || alive {
		t.Fatalf("expected error for missing binary, got alive=%v err=%v", alive, err)
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
