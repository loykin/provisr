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
	Name            string
	Command         string        // command to start the process (shell)
	WorkDir         string        // optional working dir
	Env             []string      // optional extra env
	PIDFile         string        // optional pidfile path; if set a PIDFileDetector will be used
	RetryCount      int           // number of retries on start failure
	RetryInterval   time.Duration // interval between retries
	StartDuration   time.Duration // minimum time the process must stay up to be considered started
	AutoRestart     bool          // restart automatically if the process dies unexpectedly
	RestartInterval time.Duration // wait before attempting an auto-restart
	Instances       int           // number of instances to run concurrently (default 1)
	Detectors       []detector.Detector
	Log             logger.Config // logging configuration for this process instance
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
	// #nosec G204
	return exec.Command(name, args...)
}
