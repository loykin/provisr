//go:build windows

package process

import (
	"strings"
	"testing"
)

func TestBuildCommand_EmptyCommand_Windows(t *testing.T) {
	spec := Spec{
		Name:    "test",
		Command: "",
	}
	cmd := spec.BuildCommand()

	// On Windows, should be cmd.exe with /c rem
	if !strings.Contains(cmd.Path, "cmd") {
		t.Errorf("expected cmd for empty command, got %q", cmd.Path)
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "/c" {
		t.Errorf("expected /c argument, got %v", cmd.Args)
	}
}
