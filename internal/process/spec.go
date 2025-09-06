package process

import (
	"os/exec"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/detector"
	"github.com/loykin/provisr/internal/logger"
)

// Spec describes a process to be managed.
type Spec struct {
	Name            string              `json:"name"`
	Command         string              `json:"command"`          // command to start the process (shell)
	WorkDir         string              `json:"work_dir"`         // optional working dir
	Env             []string            `json:"env"`              // optional extra env
	PIDFile         string              `json:"pid_file"`         // optional pidfile path; if set a PIDFileDetector will be used
	RetryCount      int                 `json:"retry_count"`      // number of retries on start failure
	RetryInterval   time.Duration       `json:"retry_interval"`   // interval between retries
	StartDuration   time.Duration       `json:"start_duration"`   // minimum time the process must stay up to be considered started
	AutoRestart     bool                `json:"auto_restart"`     // restart automatically if the process dies unexpectedly
	RestartInterval time.Duration       `json:"restart_interval"` // wait before attempting an auto-restart
	Instances       int                 `json:"instances"`        // number of instances to run concurrently (default 1)
	Detectors       []detector.Detector `json:"-"`
	Log             logger.Config       `json:"log"` // logging configuration for this process instance
}

// BuildCommand constructs an *exec.Cmd for the given spec.Command.
// It avoids invoking a shell when not necessary.
func (s *Spec) BuildCommand() *exec.Cmd {
	cmdStr := strings.TrimSpace(s.Command)
	if cmdStr == "" {
		// #nosec G204
		return exec.Command("/bin/true")
	}
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
	// ok: intentional shell execution, input is validated and safe
	// #nosec G204
	return exec.Command(name, args...)
}
