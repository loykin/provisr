package detector

import (
	"errors"
	"os/exec"
	"strings"
)

// CommandDetector runs a command that should succeed if the process is running.
type CommandDetector struct{ Command string }

// buildShellAwareCommand constructs an *exec.Cmd for a detector command.
// Avoids invoking a shell unless obvious shell metacharacters are present (G204 mitigation).
func buildShellAwareCommand(cmdStr string) *exec.Cmd {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return getTrueCommand()
	}
	if strings.ContainsAny(cmdStr, "|&;<>*?`$\"'(){}[]~") {
		return getShellCommand(cmdStr)
	}
	parts := strings.Fields(cmdStr)
	name := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	// #nosec G204
	return exec.Command(name, args...)
}

func (d CommandDetector) Alive() (bool, error) {
	cmd := buildShellAwareCommand(d.Command)
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// non-zero exit code means not alive
		return false, nil
	}
	return false, err
}

func (d CommandDetector) Describe() string { return "cmd:" + d.Command }
