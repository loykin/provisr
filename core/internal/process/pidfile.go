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

// ReadPIDFileWithMeta reads a PID file written by Process.WritePIDFile.
// Returns PID, optional Spec (nil when the spec line is absent), and optional
// Meta (nil when the meta line is absent).
// A JSON line that is present but unparseable is treated as file corruption
// and causes an error — absent lines are the only acceptable nil case.
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

	// Validate PID is in reasonable range
	if pid <= 0 {
		return 0, nil, nil, fmt.Errorf("invalid PID %d: must be positive", pid)
	}
	if pid > 4194304 { // Linux default max PID
		return 0, nil, nil, fmt.Errorf("invalid PID %d: exceeds maximum", pid)
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
		if err := json.Unmarshal([]byte(secondJSON), &spec); err != nil {
			return 0, nil, nil, fmt.Errorf("malformed spec in PID file %q: %w", path, err)
		}
		specPtr = &spec
	}

	var metaPtr *PIDMeta
	if thirdJSON != "" {
		var meta PIDMeta
		if err := json.Unmarshal([]byte(thirdJSON), &meta); err != nil {
			return 0, nil, nil, fmt.Errorf("malformed meta in PID file %q: %w", path, err)
		}
		metaPtr = &meta
	}

	return pid, specPtr, metaPtr, nil
}

// ReadPIDFile is a compatibility wrapper returning only pid and spec.
func ReadPIDFile(path string) (int, *Spec, error) {
	pid, spec, _, err := ReadPIDFileWithMeta(path)
	return pid, spec, err
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
	rawPID, spec, meta, err := ReadPIDFileWithMeta(path)
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
	// Defensive: ReadPIDFileWithMeta returns error for pid<=0, but guard anyway.
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
