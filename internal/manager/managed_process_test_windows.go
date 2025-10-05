//go:build windows

package manager

import (
	"fmt"
	"os"
	"strings"
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
	// Use multiple ping commands for reliable delay on Windows
	// Each ping -n 2 takes about 1 second, so we repeat it duration times
	pingCommands := make([]string, duration)
	for i := 0; i < duration; i++ {
		pingCommands[i] = "ping -n 2 127.0.0.1 >nul"
	}
	allPings := strings.Join(pingCommands, " && ")
	return fmt.Sprintf("cmd /c \"echo %s && %s\"", message, allPings)
}

// getEnvTestCommand returns a command that echoes an environment variable
func getEnvTestCommand(varName string) string {
	return fmt.Sprintf("cmd /c \"echo %%%s%%\"", varName)
}

// getComplexTestCommand returns a complex test command for stress testing
func getComplexTestCommand() string {
	// Use a simple loop with ping for Windows - guaranteed to run longer than StartDuration (200ms)
	// This creates a 10+ second process that should reliably stay in "starting" state during the test
	return "cmd /c \"echo started && ping -n 3 127.0.0.1 >nul && ping -n 3 127.0.0.1 >nul && ping -n 3 127.0.0.1 >nul && ping -n 3 127.0.0.1 >nul\""
}

// getSimpleTestCommand returns a simple test command
func getSimpleTestCommand(message string) string {
	return fmt.Sprintf("cmd /c \"echo %s\"", message)
}

// getTrueCommand returns a command that always succeeds
func getTrueCommand() string {
	return "cmd /c \"exit /b 0\""
}
