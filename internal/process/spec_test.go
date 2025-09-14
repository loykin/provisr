package process

import (
	"runtime"
	"strings"
	"testing"
)

func requireUnixSpec(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like shell")
	}
}

// Ensure that when the command string already includes an explicit
// shell invocation (e.g., "sh -c 'echo hi'"), we do not double-wrap
// it with another "/bin/sh -c" layer.
func TestBuildCommand_ExplicitShellNoDoubleWrap(t *testing.T) {
	requireUnixSpec(t)
	s := Spec{Name: "x", Command: "sh -c 'echo hi'"}
	cmd := s.BuildCommand()
	if len(cmd.Args) < 3 {
		t.Fatalf("unexpected argv: %#v", cmd.Args)
	}
	if cmd.Args[1] != "-c" {
		t.Fatalf("expected -c as second arg, got %#v", cmd.Args)
	}
	// The string after -c should be the original script, not another nested shell.
	if strings.HasPrefix(cmd.Args[2], "sh -c ") || strings.HasPrefix(cmd.Args[2], "/bin/sh -c ") {
		t.Fatalf("command was double-wrapped: %q", cmd.Args[2])
	}
}

// Sanity check: when metacharacters are present and no explicit shell prefix
// is provided, we should wrap with /bin/sh -c.
func TestBuildCommand_MetacharTriggersShell(t *testing.T) {
	requireUnixSpec(t)
	s := Spec{Name: "y", Command: "echo hi | wc -c"}
	cmd := s.BuildCommand()
	if len(cmd.Args) < 3 || cmd.Args[1] != "-c" {
		t.Fatalf("expected shell -c wrapping, got argv=%#v", cmd.Args)
	}
}
