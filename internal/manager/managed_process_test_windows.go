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
	// Use PowerShell for reliable sleep on Windows environments
	// Start-Sleep works even when ICMP (ping) is blocked
	return fmt.Sprintf("powershell -NoProfile -NonInteractive -Command \"Write-Output '%s'; Start-Sleep -Seconds %d\"", message, duration)
}

// getEnvTestCommand returns a command that echoes an environment variable
func getEnvTestCommand(varName string) string {
	return fmt.Sprintf("cmd /c \"echo %%%s%%\"", varName)
}

// getComplexTestCommand returns a complex test command for stress testing
func getComplexTestCommand() string {
	// Use PowerShell with multiple sleeps to ensure a multi-second runtime
	return "powershell -NoProfile -NonInteractive -Command \"Write-Output 'started'; Start-Sleep -Seconds 3; Start-Sleep -Seconds 3; Start-Sleep -Seconds 3; Start-Sleep -Seconds 3\""
}

// getSimpleTestCommand returns a simple test command
func getSimpleTestCommand(message string) string {
	return fmt.Sprintf("cmd /c \"echo %s\"", message)
}

// getTrueCommand returns a command that always succeeds
func getTrueCommand() string {
	return "cmd /c \"exit /b 0\""
}
