package process

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// PIDMeta holds additional identity information for a PID to avoid PID reuse issues.
// StartUnix is the process start time in Unix seconds (UTC/local agnostic for equality checks).
type PIDMeta struct {
	StartUnix int64 `json:"start_unix"`
}

// ReadPIDFileWithMeta reads a PID file written by Process.WritePIDFile.
// Returns PID, optional Spec, and optional Meta (may be nil if absent or unparsable).
func ReadPIDFileWithMeta(path string) (int, *Spec, *PIDMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, nil, err
	}
	content := string(b)
	// First line: PID
	first, rest, _ := strings.Cut(content, "\n")
	pidStr := strings.TrimSpace(first)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, nil, nil, err
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return pid, nil, nil, nil
	}

	// If there is a second JSON line, it could be either Spec (legacy extended)
	// or Meta (in newer format we append meta as a third line).
	secondJSON := rest
	thirdJSON := ""
	if l2, l3, ok := strings.Cut(rest, "\n"); ok {
		secondJSON = strings.TrimSpace(l2)
		thirdJSON = strings.TrimSpace(l3)
	}

	var spec Spec
	var specPtr *Spec
	if secondJSON != "" {
		if err := json.Unmarshal([]byte(secondJSON), &spec); err == nil {
			specPtr = &spec
		}
	}

	var metaPtr *PIDMeta
	if thirdJSON != "" {
		var meta PIDMeta
		if err := json.Unmarshal([]byte(thirdJSON), &meta); err == nil {
			metaPtr = &meta
		}
	}

	return pid, specPtr, metaPtr, nil
}

// ReadPIDFile is a compatibility wrapper returning only pid and spec.
func ReadPIDFile(path string) (int, *Spec, error) {
	pid, spec, _, err := ReadPIDFileWithMeta(path)
	return pid, spec, err
}
