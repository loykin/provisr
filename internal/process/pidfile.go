package process

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// ReadPIDFile reads a PID file written by Process.WritePIDFile.
// It returns the PID and, if present, the JSON-encoded Spec that follows.
// For legacy files that contain only the PID, spec will be nil.
func ReadPIDFile(path string) (int, *Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, err
	}
	// Split into first line (pid) and the rest (json)
	pidLine, rest, _ := strings.Cut(string(b), "\n")
	pidStr := strings.TrimSpace(pidLine)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, nil, err
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return pid, nil, nil
	}
	var spec Spec
	if err := json.Unmarshal([]byte(rest), &spec); err != nil {
		// Return PID even if spec cannot be parsed
		return pid, nil, nil
	}
	return pid, &spec, nil
}
