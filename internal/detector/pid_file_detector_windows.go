//go:build windows

package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// pidAlive returns true if a process with given pid exists on Windows.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Try to open the process with PROCESS_QUERY_INFORMATION
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)

	// If we can open it, the process exists
	return true
}

// PIDFileDetector detects a process via a PID file.
type PIDFileDetector struct {
	PIDFile string
}

type pidMeta struct {
	StartUnix int64 `json:"start_unix"`
}

func (d PIDFileDetector) Alive() (bool, error) {
	data, err := os.ReadFile(d.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	// Support extended pidfile format: first line is PID, then optional Spec JSON and optional Meta JSON.
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return false, fmt.Errorf("empty pidfile: %s", d.PIDFile)
	}
	pidStr := strings.TrimSpace(lines[0])
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false, fmt.Errorf("invalid pid in %s: %w", d.PIDFile, err)
	}

	// Try parse meta from 3rd line
	var meta pidMeta
	if len(lines) >= 3 && strings.TrimSpace(lines[2]) != "" {
		if err := json.Unmarshal([]byte(lines[2]), &meta); err == nil && meta.StartUnix > 0 {
			// Verify the process start time matches
			actualStart := getProcStartUnix(pid)
			if actualStart > 0 && actualStart != meta.StartUnix {
				// PID reuse detected
				return false, nil
			}
		}
	}

	return pidAlive(pid), nil
}

func (d PIDFileDetector) Describe() string {
	return fmt.Sprintf("pidfile:%s", d.PIDFile)
}

// getProcStartUnix is already implemented in procstart_windows.go
