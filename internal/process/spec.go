package process

import (
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
