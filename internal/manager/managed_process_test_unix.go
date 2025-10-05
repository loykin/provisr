//go:build !windows

package manager

import (
	"fmt"
	"syscall"
)

// killProcessByPID kills a process by PID for testing purposes on Unix systems
func killProcessByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

// getTestCommand returns a platform-appropriate test command
func getTestCommand(message string, duration int) string {
	return fmt.Sprintf("sh -c 'echo %s; sleep %d'", message, duration)
}

// getEnvTestCommand returns a command that echoes an environment variable
func getEnvTestCommand(varName string) string {
	return fmt.Sprintf("echo $%s", varName)
}

// getComplexTestCommand returns a complex test command for stress testing
func getComplexTestCommand() string {
	return "sh -c 'sleep 0.1 && echo started && sleep 2'"
}

// getSimpleTestCommand returns a simple test command
func getSimpleTestCommand(message string) string {
	return fmt.Sprintf("echo %s", message)
}

// getTrueCommand returns a command that always succeeds
func getTrueCommand() string {
	return "true"
}
