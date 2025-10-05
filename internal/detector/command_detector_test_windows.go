//go:build windows

package detector

import (
	"strings"
	"testing"
)

func TestBuildShellAwareCommand_Windows(t *testing.T) {
	// empty -> cmd /c rem
	c := buildShellAwareCommand("")
	if c.Path == "" || !strings.Contains(c.String(), "cmd") {
		t.Fatalf("expected cmd, got %q (%q)", c.Path, c.String())
	}
	// simple no metachar -> direct exec
	c = buildShellAwareCommand("echo hello")
	if len(c.Args) == 0 || c.Args[0] != "echo" {
		t.Fatalf("expected direct exec echo, got %#v", c.Args)
	}
	// with shell meta -> cmd /c
	c = buildShellAwareCommand("echo hi | findstr hi")
	if len(c.Args) < 2 || c.Args[0] != "cmd" || c.Args[1] != "/c" {
		t.Fatalf("expected cmd /c, got %#v", c.Args)
	}
}

func TestCommandDetectorAlive_Windows(t *testing.T) {
	// A command that exits 0 -> Alive true
	d := CommandDetector{Command: "cmd /c rem"}
	alive, err := d.Alive()
	if err != nil || !alive {
		t.Fatalf("cmd /c rem should be alive, got alive=%v err=%v", alive, err)
	}
	if d.Describe() != "cmd:cmd /c rem" {
		t.Fatalf("Describe mismatch: %q", d.Describe())
	}

	// A command that exits non-zero -> Alive false, nil error
	d = CommandDetector{Command: "cmd /c exit 3"}
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
