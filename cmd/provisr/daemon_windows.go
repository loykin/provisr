//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// configureDaemonAttrs sets Windows-specific daemon attributes
func configureDaemonAttrs(cmd *exec.Cmd) {
	// Set process attributes for daemon on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // CREATE_NO_WINDOW
	}
}

// isDaemonSupported returns true if daemonization is supported on this platform
func isDaemonSupported() bool {
	return true // Windows supports background processes
}
