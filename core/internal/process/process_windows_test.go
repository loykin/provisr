//go:build windows

package process

import (
	"os/exec"
	"testing"
)

func checkSysProcAttrs(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
}
