package process

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PIDMeta holds additional identity information for a PID to avoid PID reuse issues.
// StartUnix is the process start time in Unix seconds (UTC/local agnostic for equality checks).
type PIDMeta struct {
	StartUnix int64 `json:"start_unix"`
}

// ReadPIDFile reads the canonical three-line PID file written by Process:
// PID, JSON-encoded Spec, and JSON-encoded PIDMeta.
func ReadPIDFile(path string) (int, *Spec, *PIDMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, nil, err
	}
	content := strings.ReplaceAll(string(b), "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 3 {
		return 0, nil, nil, fmt.Errorf("invalid PID file %q: expected 3 lines, got %d", path, len(lines))
	}
	first := lines[0]
	pidStr := strings.TrimSpace(first)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, nil, nil, err
	}

	// Validate PID is in reasonable range
	if pid <= 0 {
		return 0, nil, nil, fmt.Errorf("invalid PID %d: must be positive", pid)
	}
	if pid > 4194304 { // Linux default max PID
		return 0, nil, nil, fmt.Errorf("invalid PID %d: exceeds maximum", pid)
	}
	var spec Spec
	if err := json.Unmarshal([]byte(strings.TrimSpace(lines[1])), &spec); err != nil {
		return 0, nil, nil, fmt.Errorf("malformed spec in PID file %q: %w", path, err)
	}
	var meta PIDMeta
	if err := json.Unmarshal([]byte(strings.TrimSpace(lines[2])), &meta); err != nil {
		return 0, nil, nil, fmt.Errorf("malformed meta in PID file %q: %w", path, err)
	}
	if meta.StartUnix <= 0 {
		return 0, nil, nil, fmt.Errorf("invalid meta in PID file %q: start_unix must be positive", path)
	}
	return pid, &spec, &meta, nil
}

// VerifyPIDFile reads the PID file at path and performs best-effort identity
// verification via start-time comparison when meta is present.
// It does NOT check whether the process is currently alive; that is left to
// the caller (e.g. ManagedProcess.Recover → DetectAlive).
//
// Return semantics:
//   - (pid>0, spec, nil) — PID passed identity verification (or no meta to check against)
//   - (0,    spec, nil) — file absent, invalid content, or PID identity mismatch (not an error for callers)
//   - (0,    nil,  err) — OS-level I/O error (permissions, disk fault)
//
// Identity check: when meta.StartUnix > 0 and getProcStartUnix returns a
// non-zero value that differs from meta.StartUnix, the PID is considered reused
// and pid=0 is returned.  When getProcStartUnix returns 0 (platform limitation
// or early boot), the check is skipped and the raw PID is returned as-is.
func VerifyPIDFile(path string) (int, *Spec, error) {
	rawPID, spec, meta, err := ReadPIDFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			// Content error (invalid PID value, parse failure) → no usable PID.
			// Don't propagate: callers should see this as "no process to recover".
			return 0, nil, nil
		}
		return 0, nil, err // OS-level error (permissions, I/O)
	}
	// Defensive: ReadPIDFile returns error for pid<=0, but guard anyway.
	if rawPID <= 0 {
		return 0, spec, nil
	}
	if meta != nil && meta.StartUnix > 0 {
		cur := getProcStartUnix(rawPID)
		if cur > 0 && cur != meta.StartUnix {
			return 0, spec, nil // PID reused
		}
		// cur==0 → platform can't determine start time; skip check and trust the PID.
	}
	return rawPID, spec, nil
}
