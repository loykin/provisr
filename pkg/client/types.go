package client

import "time"

// StartRequest represents a request to start a process
type StartRequest struct {
	Name            string        `json:"name"`
	Command         string        `json:"command"`
	WorkDir         string        `json:"work_dir,omitempty"`
	PIDFile         string        `json:"pid_file,omitempty"`
	Retries         int           `json:"retries,omitempty"`
	RetryInterval   time.Duration `json:"retry_interval,omitempty"`
	AutoRestart     bool          `json:"autorestart,omitempty"`
	RestartInterval time.Duration `json:"restart_interval,omitempty"`
	StartDuration   time.Duration `json:"start_duration,omitempty"`
	Instances       int           `json:"instances,omitempty"`
	Environment     []string      `json:"environment,omitempty"`
	Priority        int           `json:"priority,omitempty"`
}

// StopRequest represents a request to stop processes
type StopRequest struct {
	Name     string        `json:"name,omitempty"`
	Base     string        `json:"base,omitempty"`
	Wildcard string        `json:"wildcard,omitempty"`
	Wait     time.Duration `json:"wait,omitempty"`
}

// StatusQuery represents query parameters for status endpoint
type StatusQuery struct {
	Name     string
	Base     string
	Wildcard string
}

// ProcessStatus represents the status of a single process
type ProcessStatus struct {
	Name      string    `json:"name"`
	Running   bool      `json:"running"`
	PID       int       `json:"pid,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	StoppedAt time.Time `json:"stopped_at,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error string `json:"error"`
}
