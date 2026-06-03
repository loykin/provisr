//go:build windows

package process

import "os/exec"

// getShellCommand returns a shell command for Windows systems
func getShellCommand(script string) *exec.Cmd {
	// #nosec G204
	return exec.Command("cmd", "/c", script)
}

// getTrueCommand returns a command that always succeeds on Windows systems
func getTrueCommand() *exec.Cmd {
	// #nosec G204
	return exec.Command("cmd", "/c", "rem")
}
