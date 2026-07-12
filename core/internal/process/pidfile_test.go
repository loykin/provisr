package process

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/core/internal/detector"
)

func TestPIDFileContainsPIDAndSpec(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p1.pid")
	spec := Spec{Name: "p1", Command: "sleep 0.2", PIDFile: pidfile}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	st := r.Snapshot()
	if st.PID <= 0 {
		t.Fatalf("invalid PID in snapshot: %v", st.PID)
	}
	// Wait until file appears and contains at least the PID
	ok := waitUntil(1*time.Second, 20*time.Millisecond, func() bool {
		b, err := os.ReadFile(pidfile)
		if err != nil {
			return false
		}
		first, _, _ := strings.Cut(string(b), "\n")
		first = strings.TrimSpace(first)
		pid, err := strconv.Atoi(first)
		return err == nil && pid > 0
	})
	if !ok {
		t.Fatalf("pidfile not written in time")
	}
	pid, specOut, _, err := ReadPIDFileWithMeta(pidfile)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if pid != st.PID {
		t.Fatalf("pid mismatch: got %d want %d", pid, st.PID)
	}
	if specOut == nil || specOut.Name != spec.Name || specOut.Command != spec.Command {
		t.Fatalf("spec not persisted correctly: %+v", specOut)
	}
}

func TestWritePIDFile_IncludesMetaAndDetectorValidates(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p1.pid")
	spec := Spec{Name: "p1", Command: "sleep 1", PIDFile: pidfile}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	// wait for pidfile to be written with lines
	ok := waitUntil(2*time.Second, 20*time.Millisecond, func() bool {
		b, err := os.ReadFile(pidfile)
		if err != nil {
			return false
		}
		return strings.Count(string(b), "\n") >= 2 // at least 3 lines total (two newlines)
	})
	if !ok {
		t.Fatalf("pidfile with meta not written in time")
	}

	b, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (pid,spec,meta), got %d", len(lines))
	}
	// Parse meta JSON from third line
	type meta struct {
		StartUnix int64 `json:"start_unix"`
	}
	var m meta
	if err := json.Unmarshal([]byte(strings.TrimSpace(lines[2])), &m); err != nil {
		t.Fatalf("meta unmarshal: %v (line=%q)", err, lines[2])
	}
	if m.StartUnix <= 0 {
		t.Fatalf("expected positive StartUnix in meta, got %d", m.StartUnix)
	}

	// Validate detector sees it alive
	d := detector.PIDFileDetector{PIDFile: pidfile}
	alive, derr := d.Alive()
	if derr != nil {
		t.Fatalf("detector alive err: %v", derr)
	}
	if !alive {
		t.Fatalf("expected detector to report alive with correct meta")
	}
}

func TestVerifyPIDFile_AbsentFile(t *testing.T) {
	pid, spec, err := VerifyPIDFile("/tmp/provisr-test-nonexistent-99999.pid")
	if err != nil {
		t.Fatalf("expected nil error for absent file, got %v", err)
	}
	if pid != 0 || spec != nil {
		t.Errorf("expected (0, nil) for absent file, got pid=%d spec=%v", pid, spec)
	}
}

func TestVerifyPIDFile_InvalidPID(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "invalid.pid")
	// Content error: non-numeric PID.
	if err := os.WriteFile(pidfile, []byte("not-a-pid\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pid, _, err := VerifyPIDFile(pidfile)
	if err != nil {
		t.Fatalf("expected nil error for content error, got %v", err)
	}
	if pid != 0 {
		t.Errorf("expected pid=0 for invalid PID content, got %d", pid)
	}
}

func TestVerifyPIDFile_ZeroPID(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "zero.pid")
	if err := os.WriteFile(pidfile, []byte("0\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pid, _, err := VerifyPIDFile(pidfile)
	if err != nil {
		t.Fatalf("expected nil error for zero PID, got %v", err)
	}
	if pid != 0 {
		t.Errorf("expected pid=0 for zero PID content, got %d", pid)
	}
}

func TestVerifyPIDFile_NoMeta(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "legacy.pid")
	// Legacy format: only PID, no spec or meta.
	if err := os.WriteFile(pidfile, []byte("12345\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pid, _, err := VerifyPIDFile(pidfile)
	if err != nil {
		t.Fatalf("VerifyPIDFile: %v", err)
	}
	// Without meta we cannot perform identity check; raw PID is returned as-is.
	if pid != 12345 {
		t.Errorf("expected pid=12345 (no meta to verify), got %d", pid)
	}
}

// TestVerifyPIDFile_MalformedMeta verifies that a present-but-unparseable meta
// line is treated as file corruption: the PID is rejected (not trusted).
func TestVerifyPIDFile_MalformedMeta(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "bad-meta.pid")
	// Valid PID + valid spec, but meta line is not valid JSON.
	content := fmt.Sprintf("%d\n{}\nnot-valid-json\n", os.Getpid())
	if err := os.WriteFile(pidfile, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pid, _, err := VerifyPIDFile(pidfile)
	if err != nil {
		t.Fatalf("expected nil error (content error must not propagate), got %v", err)
	}
	if pid != 0 {
		t.Errorf("expected pid=0 for malformed meta, got %d", pid)
	}
}

// TestVerifyPIDFile_MalformedSpec verifies that a present-but-unparseable spec
// line is treated as file corruption: the PID is rejected (not trusted).
func TestVerifyPIDFile_MalformedSpec(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "bad-spec.pid")
	content := fmt.Sprintf("%d\nnot-valid-json\n", os.Getpid())
	if err := os.WriteFile(pidfile, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pid, _, err := VerifyPIDFile(pidfile)
	if err != nil {
		t.Fatalf("expected nil error (content error must not propagate), got %v", err)
	}
	if pid != 0 {
		t.Errorf("expected pid=0 for malformed spec, got %d", pid)
	}
}

// TestVerifyPIDFile_PIDReuse verifies that a PID file whose recorded start time
// does not match the current process's start time is rejected (PID reuse).
func TestVerifyPIDFile_PIDReuse(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "reused.pid")

	// Write a PID file pointing to the current test process (definitely running)
	// but with start_unix=1 — a time that will never match a real process.
	livePID := os.Getpid()
	content := fmt.Sprintf("%d\n{}\n{\"start_unix\":1}\n", livePID)
	if err := os.WriteFile(pidfile, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// getProcStartUnix should return a value far greater than 1, so the mismatch
	// triggers the PID-reuse guard and VerifyPIDFile must return pid=0.
	pid, _, err := VerifyPIDFile(pidfile)
	if err != nil {
		t.Fatalf("VerifyPIDFile: %v", err)
	}
	if pid != 0 {
		t.Errorf("expected pid=0 when start_unix mismatch (PID reuse), got %d", pid)
	}
}
