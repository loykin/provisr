//go:build !windows

package manager

import (
	"fmt"
	"syscall"
)

// killProcessForTest kills a process for testing purposes on Unix systems
func killProcessForTest(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

// killProcessByPID kills a process by PID for testing purposes on Unix systems
func killProcessByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}
