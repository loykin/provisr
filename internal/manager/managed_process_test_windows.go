//go:build windows

package manager

import (
	"fmt"
	"os"
)

// killProcessForTest kills a process for testing purposes on Windows
func killProcessForTest(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// killProcessByPID kills a process by PID for testing purposes on Windows
func killProcessByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
