package detector

import (
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

func (d PIDFileDetector) Alive() (bool, error) {
	data, err := os.ReadFile(d.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	// Support extended pidfile format: first line is PID, rest may contain metadata.
	firstLine, _, _ := strings.Cut(string(data), "\n")
	pidStr := strings.TrimSpace(firstLine)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false, fmt.Errorf("invalid pid in %s: %w", d.PIDFile, err)
	}
	return pidAlive(pid), nil
}

func (d PIDFileDetector) Describe() string { return "pidfile:" + d.PIDFile }

// PIDDetector detects by a provided PID number.
type PIDDetector struct{ PID int }

func (d PIDDetector) Alive() (bool, error) { return pidAlive(d.PID), nil }
func (d PIDDetector) Describe() string     { return fmt.Sprintf("pid:%d", d.PID) }
