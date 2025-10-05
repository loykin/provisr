//go:build !windows

package process

import (
	"os/exec"
	"testing"
)

// checkSysProcAttrs verifies Unix-specific process attributes
func checkSysProcAttrs(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("SysProcAttr Setpgid not set")
	}
}
