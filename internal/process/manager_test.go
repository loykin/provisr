package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/detector"
)

func TestStartStopWithPIDFile(t *testing.T) {
	mgr := NewManager()
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "demo.pid")
	spec := Spec{Name: "demo", Command: "sleep 2", PIDFile: pidfile}
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
	spec := Spec{Name: "ar", Command: "sh -c 'sleep 0.05'", AutoRestart: true, RestartInterval: 50 * time.Millisecond}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(1 * time.Second)
	var st Status
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
	// Process exits in ~50ms but startsecs requires 200ms -> Start should fail after retries
	spec := Spec{Name: "ssfail", Command: "sh -c 'sleep 0.05'", StartDuration: 200 * time.Millisecond, RetryCount: 1, RetryInterval: 50 * time.Millisecond}
	start := time.Now()
	err := mgr.Start(spec)
	if err == nil {
		t.Fatalf("expected start error due to startsecs, got nil")
	}
	// ensure we waited at least startsecs once (approx); allow some slack
	if time.Since(start) < 180*time.Millisecond {
		t.Fatalf("expected Start to wait around startsecs; took %v", time.Since(start))
	}
}

func TestStartDurationSuccess(t *testing.T) {
	mgr := NewManager()
	spec := Spec{Name: "ssok", Command: "sleep 1", StartDuration: 200 * time.Millisecond}
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
	spec := Spec{Name: "env1", Command: fmt.Sprintf("sh -c 'echo $FOO $CHAIN $PORT $LOCAL > %s'", outfile), Env: []string{"PORT=2000", "LOCAL=${FOO}-y"}}
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
	spec := Spec{Name: "d1", Command: "sleep 1"}
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
	spec := Spec{Name: "cmd", Command: "sleep 1", Detectors: []detector.Detector{detector.CommandDetector{Command: "[ -n \"$PPID\" ]"}}}
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
	spec := Spec{Name: "batch", Command: "sleep 2", Instances: 50}
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
