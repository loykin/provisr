//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// configureSysProcAttr sets platform-specific attributes for Unix-like systems.
// If spec.Detached is true, we create a new session (setsid) so the child is
// detached from the controlling terminal and survives parent exit cleanly.
// Otherwise, we place it in a new process group for signal handling.
func configureSysProcAttr(cmd *exec.Cmd, spec Spec) {
	attrs := &syscall.SysProcAttr{}
	if spec.Detached {
		attrs.Setsid = true // start the process in a new session
	} else {
		attrs.Setpgid = true // create a new process group for group signaling
	}
	cmd.SysProcAttr = attrs
}
