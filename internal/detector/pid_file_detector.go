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
	var metaStart int64
	if len(lines) >= 3 {
		var m pidMeta
		if err := json.Unmarshal([]byte(strings.TrimSpace(lines[2])), &m); err == nil {
			metaStart = m.StartUnix
		}
	} else if len(lines) >= 2 {
		// In case the second line is actually meta (unlikely), try parse
		var m pidMeta
		if err := json.Unmarshal([]byte(strings.TrimSpace(lines[1])), &m); err == nil && m.StartUnix > 0 {
			metaStart = m.StartUnix
		}
	}

	if metaStart > 0 {
		cur := getProcStartUnix(pid)
		if cur > 0 && cur != metaStart {
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
