//go:build !windows

package process

import "os/exec"

// getShellCommand returns a shell command for Unix systems
func getShellCommand(script string) *exec.Cmd {
	// #nosec G204
	return exec.Command("/bin/sh", "-c", script)
}

// getTrueCommand returns a command that always succeeds on Unix systems
func getTrueCommand() *exec.Cmd {
	// #nosec G204
	return exec.Command("/bin/true")
}
