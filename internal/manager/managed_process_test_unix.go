//go:build !windows

package manager

import "syscall"

// killProcessForTest kills a process for testing purposes on Unix systems
func killProcessForTest(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
