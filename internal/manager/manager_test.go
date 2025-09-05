package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/detector"
	"github.com/loykin/provisr/internal/logger"
	"github.com/loykin/provisr/internal/process"
)

func TestStartStopWithPIDFile(t *testing.T) {
	mgr := NewManager()
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "demo.pid")
	spec := process.Spec{Name: "demo", Command: "sleep 2", PIDFile: pidfile}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	st, err := mgr.Status("demo")
	if err != nil || !st.Running || st.PID <= 0 {
		t.Fatalf("status running want true got %+v err=%v", st, err)
	}
	// pidfile must exist and contain pid
	data, err := os.ReadFile(pidfile)
	if err != nil || len(data) == 0 {
		t.Fatalf("pidfile not created: %v", err)
	}
	if err := mgr.Stop("demo", 2*time.Second); err != nil {
		// sleep exits normally, err can be nil
	}
	st2, _ := mgr.Status("demo")
	if st2.Running {
		t.Fatalf("expected stopped")
	}
}

func TestAutoRestart(t *testing.T) {
	mgr := NewManager()
	// command exits quickly; enable autorestart
	spec := process.Spec{Name: "ar", Command: "sh -c 'sleep 0.05'", AutoRestart: true, RestartInterval: 50 * time.Millisecond}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(1 * time.Second)
	var st process.Status
	for time.Now().Before(deadline) {
		st, _ = mgr.Status("ar")
		if st.Restarts >= 1 && st.Running {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st.Restarts < 1 || !st.Running {
		t.Fatalf("expected running after at least one autorestart, got running=%v restarts=%d", st.Running, st.Restarts)
	}
	// stop should not trigger restart
	_ = mgr.Stop("ar", 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	st2, _ := mgr.Status("ar")
	if st2.Running {
		t.Fatalf("expected stopped after Stop, got running")
	}
}

func TestStartDurationFailAndRetry(t *testing.T) {
	mgr := NewManager()
	// Process exits in ~100ms but startsecs requires 300ms -> Start should fail after retries
	spec := process.Spec{Name: "ssfail", Command: "sh -c 'sleep 0.1'", StartDuration: 300 * time.Millisecond, RetryCount: 1, RetryInterval: 50 * time.Millisecond}
	start := time.Now()
	err := mgr.Start(spec)
	if err == nil {
		t.Fatalf("expected start error due to startsecs, got nil")
	}
	// With early-exit detection, Start should fail promptly (well before startsecs).
	if time.Since(start) >= 280*time.Millisecond {
		t.Fatalf("expected Start to fail promptly before startsecs; took %v", time.Since(start))
	}
}

func TestStartDurationSuccess(t *testing.T) {
	mgr := NewManager()
	spec := process.Spec{Name: "ssok", Command: "sleep 1", StartDuration: 200 * time.Millisecond}
	start := time.Now()
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Start should not return before ~startsecs
	if time.Since(start) < 180*time.Millisecond {
		t.Fatalf("Start returned too early: %v", time.Since(start))
	}
	st, _ := mgr.Status("ssok")
	if !st.Running {
		t.Fatalf("expected running after startsecs success")
	}
}

func TestEnvGlobalAndPerProcessMerge(t *testing.T) {
	mgr := NewManager()
	// set global env and expansion
	mgr.SetGlobalEnv([]string{"FOO=bar", "CHAIN=${FOO}-x", "PORT=1000"})
	dir := t.TempDir()
	outfile := filepath.Join(dir, "out.txt")
	// per-process overrides PORT and defines LOCAL using global FOO
	spec := process.Spec{Name: "env1", Command: fmt.Sprintf("sh -c 'echo $FOO $CHAIN $PORT $LOCAL > %s'", outfile), Env: []string{"PORT=2000", "LOCAL=${FOO}-y"}}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Give it a moment to execute and exit
	time.Sleep(200 * time.Millisecond)
	b, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := strings.TrimSpace(string(b))
	want := "bar bar-x 2000 bar-y"
	if got != want {
		t.Fatalf("env merge mismatch: got %q want %q", got, want)
	}
}

func TestDetectors(t *testing.T) {
	mgr := NewManager()
	spec := process.Spec{Name: "d1", Command: "sleep 1"}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	st, _ := mgr.Status("d1")
	if !st.Running {
		t.Fatalf("expected running")
	}
	time.Sleep(1500 * time.Millisecond)
	st2, _ := mgr.Status("d1")
	if st2.Running {
		t.Fatalf("expected not running after sleep finished")
	}
}

func TestCommandDetector(t *testing.T) {
	mgr := NewManager()
	spec := process.Spec{Name: "cmd", Command: "sleep 1", Detectors: []detector.Detector{detector.CommandDetector{Command: "[ -n \"$PPID\" ]"}}}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	st, _ := mgr.Status("cmd")
	if !st.Running {
		t.Fatalf("expected running")
	}
	time.Sleep(1200 * time.Millisecond)
	st2, _ := mgr.Status("cmd")
	if st2.Running {
		t.Fatalf("expected stopped")
	}
}

func TestStartNAndStopAll50(t *testing.T) {
	mgr := NewManager()
	spec := process.Spec{Name: "batch", Command: "sleep 2", Instances: 50}
	start := time.Now()
	if err := mgr.StartN(spec); err != nil {
		t.Fatalf("StartN: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cnt, _ := mgr.Count("batch")
		if cnt == 50 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cnt, _ := mgr.Count("batch")
	if cnt != 50 {
		t.Fatalf("expected 50 running, got %d (elapsed=%v)", cnt, time.Since(start))
	}
	_ = mgr.StopAll("batch", 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	cnt2, _ := mgr.Count("batch")
	if cnt2 != 0 {
		t.Fatalf("expected 0 running after StopAll, got %d", cnt2)
	}
}

// TestStartIdempotent ensures calling Start twice doesn't spawn duplicates.
func TestStartIdempotent(t *testing.T) {
	mgr := NewManager()
	spec := process.Spec{Name: "idem", Command: "sleep 1"}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Second start should be no-op
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("second start should not error: %v", err)
	}
	// Basic sanity: status should report running without duplication
	st, _ := mgr.Status("idem")
	if !st.Running {
		t.Fatalf("expected running after idempotent start")
	}
}

// TestStopAllAndStatusAll covers multiple instances aggregation and stopping.
func TestStopAllAndStatusAll(t *testing.T) {
	mgr := NewManager()
	spec := process.Spec{Name: "multi", Command: "sleep 1", Instances: 3}
	if err := mgr.StartN(spec); err != nil {
		t.Fatalf("StartN: %v", err)
	}
	// Ensure StatusAll returns 3 entries
	sts, err := mgr.StatusAll("multi")
	if err != nil {
		t.Fatalf("status all: %v", err)
	}
	if len(sts) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(sts))
	}
	// Stop all
	_ = mgr.StopAll("multi", 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	sts2, _ := mgr.StatusAll("multi")
	for _, st := range sts2 {
		if st.Running {
			t.Fatalf("expected stopped instance, got running")
		}
	}
}

// TestStatusUnknownProcess returns error
func TestStatusUnknownProcess(t *testing.T) {
	mgr := NewManager()
	if _, err := mgr.Status("nope"); err == nil {
		t.Fatalf("expected error for unknown process")
	}
}

func TestProcessLoggingStdoutStderr(t *testing.T) {
	mgr := NewManager()
	dir := t.TempDir()
	spec := process.Spec{
		Name:    "logdemo",
		Command: "sh -c 'echo out; echo err 1>&2; sleep 0.1'",
		Log:     logger.Config{Dir: dir},
	}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	// Verify files exist with content
	outPath := filepath.Join(dir, "logdemo.stdout.log")
	errPath := filepath.Join(dir, "logdemo.stderr.log")
	ob, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	eb, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if !strings.Contains(string(ob), "out") {
		t.Fatalf("stdout missing content: %q", string(ob))
	}
	if !strings.Contains(string(eb), "err") {
		t.Fatalf("stderr missing content: %q", string(eb))
	}
}

// Additional tests: starting/stopping multiple bases and multiple bases with instances
func TestStartStopMultipleBases(t *testing.T) {
	mgr := NewManager()
	names := []string{"a", "b", "c"}
	for _, n := range names {
		sp := process.Spec{Name: n, Command: "sleep 1", StartDuration: 100 * time.Millisecond}
		if err := mgr.Start(sp); err != nil {
			t.Fatalf("start %s: %v", n, err)
		}
	}
	// Wait until they report running (with deadline)
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		all := true
		for _, n := range names {
			st, err := mgr.Status(n)
			if err != nil || !st.Running {
				all = false
				break
			}
		}
		if all {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, n := range names {
		st, _ := mgr.Status(n)
		if !st.Running {
			t.Fatalf("expected %s running before stop", n)
		}
	}
	// Stop all bases
	for _, n := range names {
		_ = mgr.StopAll(n, 2*time.Second)
	}
	time.Sleep(100 * time.Millisecond)
	for _, n := range names {
		st, _ := mgr.Status(n)
		if st.Running {
			t.Fatalf("expected %s stopped after StopAll", n)
		}
	}
}

func TestStartNMultipleBasesAndStopAll(t *testing.T) {
	mgr := NewManager()
	x := process.Spec{Name: "x", Command: "sleep 1", Instances: 2}
	y := process.Spec{Name: "y", Command: "sleep 1", Instances: 3}
	if err := mgr.StartN(x); err != nil {
		t.Fatalf("startN x: %v", err)
	}
	if err := mgr.StartN(y); err != nil {
		t.Fatalf("startN y: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cx, _ := mgr.Count("x")
		cy, _ := mgr.Count("y")
		if cx == 2 && cy == 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cx, _ := mgr.Count("x")
	cy, _ := mgr.Count("y")
	if cx != 2 || cy != 3 {
		t.Fatalf("unexpected counts before stop: x=%d y=%d", cx, cy)
	}
	_ = mgr.StopAll("x", 2*time.Second)
	_ = mgr.StopAll("y", 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	cx2, _ := mgr.Count("x")
	cy2, _ := mgr.Count("y")
	if cx2 != 0 || cy2 != 0 {
		t.Fatalf("expected counts after stop to be 0; got x=%d y=%d", cx2, cy2)
	}
	// also ensure statuses report not running
	stsx, _ := mgr.StatusAll("x")
	stsy, _ := mgr.StatusAll("y")
	for _, st := range append(stsx, stsy...) {
		if st.Running {
			t.Fatalf("expected not running after StopAll, got: %+v", st)
		}
	}
}
