package process

import "time"

// Status mirrors process.Status to avoid import cycle; kept minimal for internal use.
type Status struct {
	Name       string
	Running    bool
	PID        int
	StartedAt  time.Time
	StoppedAt  time.Time
	ExitErr    error
	DetectedBy string
	Restarts   int
}
