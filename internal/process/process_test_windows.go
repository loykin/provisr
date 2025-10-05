//go:build windows

package process

import (
	"os/exec"
	"testing"
)

// checkSysProcAttrs verifies Windows-specific process attributes
func checkSysProcAttrs(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	// On Windows, we don't check Setpgid as it doesn't exist
	// Just verify that SysProcAttr is configured if needed
	// Windows-specific checks can be added here if needed
}
