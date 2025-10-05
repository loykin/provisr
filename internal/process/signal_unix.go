//go:build !windows

package process

import "syscall"

// killProcess sends a signal to a Unix process
func killProcess(pid int, signal syscall.Signal) error {
	return syscall.Kill(pid, signal)
}

// processExists checks if a process exists (for test compatibility)
func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
