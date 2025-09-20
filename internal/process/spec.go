package process

import (
	"os/exec"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/detector"
	"github.com/loykin/provisr/internal/logger"
)

// DetectorConfig represents a detector configuration that can be parsed from config files
type DetectorConfig struct {
	Type    string `json:"type" mapstructure:"type"`
	Path    string `json:"path" mapstructure:"path"`
	Command string `json:"command" mapstructure:"command"`
}

// Spec describes a process to be managed.
// All logging is now handled through slog-based structured logging.
type Spec struct {
	Name            string              `json:"name"`
	Command         string              `json:"command"`          // command to start the process (shell)
	WorkDir         string              `json:"work_dir"`         // optional working dir
	Env             []string            `json:"env"`              // optional extra env
	PIDFile         string              `json:"pid_file"`         // optional pidfile path; if set a PIDFileDetector will be used
	Priority        int                 `json:"priority"`         // startup priority (lower numbers start first, default 0)
	RetryCount      int                 `json:"retry_count"`      // number of retries on start failure
	RetryInterval   time.Duration       `json:"retry_interval"`   // interval between retries
	StartDuration   time.Duration       `json:"start_duration"`   // minimum time the process must stay up to be considered started
	AutoRestart     bool                `json:"auto_restart"`     // restart automatically if the process dies unexpectedly
	RestartInterval time.Duration       `json:"restart_interval"` // wait before attempting an auto-restart
	Instances       int                 `json:"instances"`        // number of instances to run concurrently (default 1)
	Detectors       []detector.Detector `json:"-" mapstructure:"-"`
	DetectorConfigs []DetectorConfig    `json:"detectors" mapstructure:"detectors"` // for config parsing
	Log             logger.Config       `json:"log"`                                // unified slog-based logging configuration
}

// BuildCommand constructs an *exec.Cmd for the given spec.Command.
// It avoids invoking a shell when not necessary, and it also respects
// an explicit shell invocation already present in the command string
// (e.g., "sh -c 'echo hi'"), avoiding double-wrapping with another shell.
func (s *Spec) BuildCommand() *exec.Cmd {
	cmdStr := strings.TrimSpace(s.Command)
	if cmdStr == "" {
		// #nosec G204
		return exec.Command("/bin/true")
	}
	// If the command already explicitly uses a shell, honor it without adding another layer.
	if _, afterC, ok := parseExplicitShell(cmdStr); ok {
		// Always use absolute shell path to avoid PATH dependency when Env is overridden.
		// #nosec G204
		return exec.Command("/bin/sh", "-c", afterC)
	}
	// Fallback: when metacharacters are present, use /bin/sh -c
	if strings.ContainsAny(cmdStr, "|&;<>*?`$\"'(){}[]~") {
		// #nosec G204
		return exec.Command("/bin/sh", "-c", cmdStr)
	}
	parts := strings.Fields(cmdStr)
	name := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	// ok: intentional execution, input is validated and safe
	// #nosec G204
	return exec.Command(name, args...)
}

// parseExplicitShell detects patterns like "sh -c <ARG>" or "/bin/sh -c <ARG>" at the
// beginning of cmdStr. It returns (shellPath, afterCArg, true) when matched.
// It preserves the substring after "-c " verbatim to avoid breaking quoting.
func parseExplicitShell(cmdStr string) (string, string, bool) {
	trim := strings.TrimLeft(cmdStr, " \t")
	candidates := []string{"sh -c ", "/bin/sh -c ", "/usr/bin/sh -c "}
	for _, p := range candidates {
		if strings.HasPrefix(trim, p) {
			after := trim[len(p):]
			// If after is wrapped in single or double quotes, strip one pair so that
			// we pass the actual script to the shell (the outer quotes would otherwise
			// inhibit parsing/redirection inside the script).
			if n := len(after); n >= 2 {
				if (after[0] == '\'' && after[n-1] == '\'') || (after[0] == '"' && after[n-1] == '"') {
					after = after[1 : n-1]
				}
			}
			return strings.Fields(p)[0], after, true
		}
	}
	return "", "", false
}
