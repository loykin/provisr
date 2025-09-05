package process

import "time"

// Status represents a runtime state of managed process.
type Status struct {
	Name       string
	Running    bool
	PID        int
	StartedAt  time.Time
	StoppedAt  time.Time
	ExitErr    error
	DetectedBy string // which detector reported alive
	Restarts   int
}
