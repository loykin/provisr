//go:build !windows

package detector

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// pidAlive returns true if a process with given pid exists (or EPERM).
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
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
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n")), "\n")
	if len(lines) != 3 {
		return false, fmt.Errorf("invalid pidfile %s: expected 3 lines", d.PIDFile)
	}
	pidStr := strings.TrimSpace(lines[0])
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false, fmt.Errorf("invalid pid in %s: %w", d.PIDFile, err)
	}

	if !json.Valid([]byte(strings.TrimSpace(lines[1]))) {
		return false, fmt.Errorf("invalid spec in pidfile %s", d.PIDFile)
	}
	var meta pidMeta
	if err := json.Unmarshal([]byte(strings.TrimSpace(lines[2])), &meta); err != nil || meta.StartUnix <= 0 {
		return false, fmt.Errorf("invalid metadata in pidfile %s", d.PIDFile)
	}
	if meta.StartUnix > 0 {
		cur := getProcStartUnix(pid)
		if cur > 0 && cur != meta.StartUnix {
			return false, nil // PID reused; not our process
		}
	}

	return pidAlive(pid), nil
}

func (d PIDFileDetector) Describe() string { return "pidfile:" + d.PIDFile }

// PIDDetector detects by a provided PID number.
type PIDDetector struct{ PID int }

func (d PIDDetector) Alive() (bool, error) { return pidAlive(d.PID), nil }
func (d PIDDetector) Describe() string     { return fmt.Sprintf("pid:%d", d.PID) }
