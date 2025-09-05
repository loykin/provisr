package detector

// Detector is a strategy that determines if a process is running.
// Implementations may check PID file, PID number, or a custom script.
// It must be safe for concurrent use.
type Detector interface {
	// Alive returns true if the process is detected as running.
	Alive() (bool, error)
	// Describe returns a human-readable description of the detection method.
	Describe() string
}
