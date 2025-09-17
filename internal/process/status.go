package process

import "time"

// Status mirrors process.Status to avoid import cycle; kept minimal for internal use.
type Status struct {
	Name       string    `json:"name"`
	Running    bool      `json:"running"`
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
	StoppedAt  time.Time `json:"stopped_at"`
	ExitErr    error     `json:"exit_error,omitempty"`
	DetectedBy string    `json:"detected_by"`
	Restarts   int       `json:"restarts"`
	State      string    `json:"state"` // State machine state: stopped, starting, running, stopping
}
