package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startSleep starts a short-lived sleep process and returns *exec.Cmd already started
func startSleep(dur string) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("unsupported on windows")
	}
	// #nosec G204
	cmd := exec.Command("/bin/sh", "-c", "sleep "+dur)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// Test that PIDFileDetector validates meta start time correctly when it matches the real process
func TestPIDFileDetector_WithMetaMatches(t *testing.T) {
	requireUnix(t)
	cmd, err := startSleep("2")
	if err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	pid := cmd.Process.Pid
	// Allow the process to appear in proc table
	time.Sleep(20 * time.Millisecond)
	start := getProcStartUnix(pid)
	if start == 0 {
		// Best-effort: if start time not available on platform, skip
		t.Skip("process start time unavailable on this platform")
	}

	dir := t.TempDir()
	pidfile := filepath.Join(dir, "demo.pid")

	// Write pidfile with meta on third line (first line PID, second can be empty or spec)
	meta := pidMeta{StartUnix: start}
	mb, _ := json.Marshal(meta)
	content := strings.Join([]string{strconv.Itoa(pid), "{}", string(mb)}, "\n")
	if err := os.WriteFile(pidfile, []byte(content), 0o600); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}

	d := PIDFileDetector{PIDFile: pidfile}
	alive, err := d.Alive()
	if err != nil {
		t.Fatalf("Alive error: %v", err)
	}
	if !alive {
		t.Fatalf("expected alive with matching meta, got false")
	}
}

// Test that when meta mismatches actual start time, detector returns false even if PID exists
func TestPIDFileDetector_WithMetaMismatch(t *testing.T) {
	requireUnix(t)
	cmd, err := startSleep("2")
	if err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	pid := cmd.Process.Pid
	time.Sleep(20 * time.Millisecond)
	start := getProcStartUnix(pid)
	if start == 0 {
		t.Skip("process start time unavailable on this platform")
	}

	dir := t.TempDir()
	pidfile := filepath.Join(dir, "demo.pid")
	// Intentionally wrong start time
	meta := pidMeta{StartUnix: start + 12345}
	mb, _ := json.Marshal(meta)
	content := strings.Join([]string{strconv.Itoa(pid), "{}", string(mb)}, "\n")
	if err := os.WriteFile(pidfile, []byte(content), 0o600); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}

	d := PIDFileDetector{PIDFile: pidfile}
	alive, err := d.Alive()
	if err != nil {
		t.Fatalf("Alive error: %v", err)
	}
	if alive {
		t.Fatalf("expected not alive with mismatched meta, got true")
	}
}

// Fuzz PIDFileDetector.Alive with various malformed inputs to ensure robustness
func FuzzPIDFileDetector_Alive(f *testing.F) {
	f.Add("123\n{}\n{\"start_unix\":1}", true)
	f.Add("not-a-number\n", false)
	f.Add("\n\n{}\n{\"start_unix\":1}\n", false)
	f.Fuzz(func(t *testing.T, content string, addNL bool) {
		dir := t.TempDir()
		pf := filepath.Join(dir, "fuzz.pid")
		if addNL {
			content += "\n"
		}
		_ = os.WriteFile(pf, []byte(content), 0o600)
		_, _ = (PIDFileDetector{PIDFile: pf}).Alive() // Should never panic
	})
}
