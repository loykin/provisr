//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// configureDaemonAttrs sets Unix-specific daemon attributes
func configureDaemonAttrs(cmd *exec.Cmd) {
	// Set process attributes for daemon
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}
}
