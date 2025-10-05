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

// getTestCommand returns a platform-appropriate test command
func getTestCommand(message string, duration int) string {
	// Use ping for reliable delay on Windows - ping -n (duration+1) gives roughly duration seconds
	return fmt.Sprintf("cmd /c \"echo %s && ping -n %d 127.0.0.1 >nul\"", message, duration+1)
}

// getEnvTestCommand returns a command that echoes an environment variable
func getEnvTestCommand(varName string) string {
	return fmt.Sprintf("cmd /c \"echo %%%s%%\"", varName)
}

// getComplexTestCommand returns a complex test command for stress testing
func getComplexTestCommand() string {
	return "cmd /c \"ping -n 3 127.0.0.1 >nul && echo started && ping -n 5 127.0.0.1 >nul\""
}

// getSimpleTestCommand returns a simple test command
func getSimpleTestCommand(message string) string {
	return fmt.Sprintf("cmd /c \"echo %s\"", message)
}

// getTrueCommand returns a command that always succeeds
func getTrueCommand() string {
	return "cmd /c \"exit /b 0\""
}
