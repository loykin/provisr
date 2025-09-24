package process

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/detector"
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
	pid, specOut, err := ReadPIDFile(pidfile)
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

func TestReadPIDFileLegacyFormat(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "legacy.pid")
	if err := os.WriteFile(pidfile, []byte("12345\n"), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	pid, specOut, err := ReadPIDFile(pidfile)
	if err != nil {
		t.Fatalf("ReadPIDFile legacy: %v", err)
	}
	if pid != 12345 {
		t.Fatalf("pid mismatch: got %d want 12345", pid)
	}
	if specOut != nil {
		t.Fatalf("expected nil spec for legacy pidfile, got %+v", specOut)
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
