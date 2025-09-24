package process

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/logger"
)

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("tests require sh/sleep on Unix-like systems")
	}
}

// waitUntil polls fn until it returns true or timeout expires.
func waitUntil(timeout, step time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(step)
	}
	return false
}

func TestTryStartWritesPIDAndStatus(t *testing.T) {
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
	if !st.Running || st.PID <= 0 || st.Name != "p1" {
		t.Fatalf("status not set after start: %+v", st)
	}
	if ok := waitUntil(1*time.Second, 20*time.Millisecond, func() bool {
		b, err := os.ReadFile(pidfile)
		return err == nil && len(strings.TrimSpace(string(b))) > 0
	}); !ok {
		t.Fatalf("pidfile not written in time")
	}
}

func TestConfigureCmdAppliesEnvWorkdirLogging(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	_ = os.MkdirAll(work, 0o755)
	logs := filepath.Join(dir, "logs")

	spec := Spec{
		Name:    "cfg",
		Command: "sh -c 'echo out; echo err 1>&2; sleep 0.05'",
		WorkDir: work,
		Log:     logger.Config{File: logger.FileConfig{Dir: logs}},
	}
	r := New(spec)
	mergedEnv := []string{"FOO=bar"}
	cmd := r.ConfigureCmd(mergedEnv)

	if cmd.Dir != work {
		t.Fatalf("workdir not applied: got %q want %q", cmd.Dir, work)
	}
	if len(cmd.Env) != len(mergedEnv) || cmd.Env[0] != "FOO=bar" {
		t.Fatalf("env not applied: got %#v", cmd.Env)
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("SysProcAttr Setpgid not set")
	}

	// Start and let it produce logs
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for process to exit by polling OS without calling Wait()
	pid := r.Snapshot().PID
	_ = waitUntil(2*time.Second, 20*time.Millisecond, func() bool { return syscall.Kill(pid, 0) != nil })
	// Allow file buffers to flush
	time.Sleep(50 * time.Millisecond)

	outPath := filepath.Join(logs, "cfg.stdout.log")
	errPath := filepath.Join(logs, "cfg.stderr.log")
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

// IsBeforeStartErr reports whether the error indicates the process exited before start duration elapsed.
func IsBeforeStartErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "exited before start duration")
}

func TestEnforceStartDurationEarlyExit(t *testing.T) {
	requireUnix(t)
	spec := Spec{Name: "early", Command: "sh -c 'sleep 0.05'"}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	d := 200 * time.Millisecond
	start := time.Now()
	err := r.EnforceStartDuration(d)
	if err == nil || !IsBeforeStartErr(err) {
		t.Fatalf("expected before-start error, got: %v", err)
	}
	if time.Since(start) >= d {
		t.Fatalf("expected prompt failure before start duration, took %v", time.Since(start))
	}
}

func TestEnforceStartDurationSuccess(t *testing.T) {
	requireUnix(t)
	d := 150 * time.Millisecond
	spec := Spec{Name: "ok", Command: "sleep 0.3"}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	start := time.Now()
	if err := r.EnforceStartDuration(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < d-20*time.Millisecond {
		t.Fatalf("EnforceStartDuration returned too early: %v < %v", elapsed, d)
	}
}

func TestStopRequestedToggle(t *testing.T) {
	r := New(Spec{Name: "x", Command: "sleep 0.2"})
	if r.StopRequested() {
		t.Fatalf("default StopRequested should be false")
	}
	r.SetStopRequested(true)
	if !r.StopRequested() {
		t.Fatalf("StopRequested should be true after SetStopRequested(true)")
	}
	r.SetStopRequested(false)
	if r.StopRequested() {
		t.Fatalf("StopRequested should be false after SetStopRequested(false)")
	}
}

func TestCloseWritersRemovePIDFileAndDetectAlive(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p.pid")
	r := New(Spec{Name: "alive", Command: "sleep 0.3", PIDFile: pidfile})
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	// After start, PID file should exist
	if ok := waitUntil(1*time.Second, 20*time.Millisecond, func() bool { _, err := os.Stat(pidfile); return err == nil }); !ok {
		t.Fatalf("pidfile missing after start")
	}
	// DetectAlive should report true via exec:pid
	if ok, src := r.DetectAlive(); !ok || !strings.Contains(src, "exec:pid") {
		t.Fatalf("DetectAlive expected true,exec:pid got %v,%q", ok, src)
	}
	// Close writers should be safe even if defaults (devnull) were used
	r.CloseWriters()
	// Kill and wait for death
	_ = r.Kill()
	if !waitUntil(2*time.Second, 20*time.Millisecond, func() bool { alive, _ := r.DetectAlive(); return !alive }) {
		t.Fatalf("expected process to be dead after Kill")
	}
	// RemovePIDFile should remove the file and be idempotent
	r.RemovePIDFile()
	if _, err := os.Stat(pidfile); !os.IsNotExist(err) {
		t.Fatalf("pidfile should be removed, stat err=%v", err)
	}
	r.RemovePIDFile() // second time should be no-op
}

func TestDetectorsAndUpdateSpec(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "p.pid")
	r := New(Spec{Name: "d", Command: "sleep 0.2", PIDFile: pidfile})
	// with PIDFile set, detectors should include pidfile detector
	dets := r.detectors()
	if len(dets) == 0 {
		t.Fatalf("expected at least one detector")
	}
	found := false
	for _, d := range dets {
		if strings.HasPrefix(d.Describe(), "pidfile:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pidfile detector present")
	}
	// UpdateSpec should change fields used by ConfigureCmd
	work := filepath.Join(dir, "work")
	_ = os.MkdirAll(work, 0o755)
	r.UpdateSpec(Spec{Name: "d", Command: "sh -c 'exit 0'", WorkDir: work})
	cmd := r.ConfigureCmd([]string{"X=1"})
	if cmd.Dir != work {
		t.Fatalf("ConfigureCmd did not apply updated WorkDir: %q", cmd.Dir)
	}
	if len(cmd.Env) == 0 || cmd.Env[0] != "X=1" {
		t.Fatalf("ConfigureCmd did not apply merged env")
	}
	// Start to ensure nothing crashes with updated spec
	_ = r.TryStart(cmd)
	// Wait for process to exit via OS polling
	pid := r.Snapshot().PID
	_ = waitUntil(2*time.Second, 20*time.Millisecond, func() bool { return syscall.Kill(pid, 0) != nil })
	// ensure EnforceStartDuration with 0 is no-op
	if err := r.EnforceStartDuration(0); err != nil {
		t.Fatalf("EnforceStartDuration(0) unexpected err: %v", err)
	}
}

func TestProcessKillWithoutMonitor(t *testing.T) {
	requireUnix(t)
	r := New(Spec{Name: "kill-nomon", Command: "sleep 10"})
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	_ = r.Kill()
	if !waitUntil(2*time.Second, 20*time.Millisecond, func() bool { alive, _ := r.DetectAlive(); return !alive }) {
		t.Fatalf("expected process to be dead after Kill")
	}
}

func TestProcessDetectAliveParallel(t *testing.T) {
	requireUnix(t)
	r := New(Spec{Name: "alive-par", Command: "sleep 0.3"})
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for {
				alive, _ := r.DetectAlive()
				if !alive {
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}
	// Wait for process to exit without calling Wait()
	pid := r.Snapshot().PID
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Give a brief moment for internal waiter to mark exit
	time.Sleep(20 * time.Millisecond)
	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatalf("goroutine %d did not finish", i)
		}
	}
}

func BenchmarkDetectAlive(b *testing.B) {
	spec := Spec{
		Name:    "benchmark-process",
		Command: "sleep 2",
	}

	proc := New(spec)
	cmd := proc.ConfigureCmd(nil)
	if err := proc.TryStart(cmd); err != nil {
		b.Fatalf("Failed to start process: %v", err)
	}
	defer func() { _ = proc.Kill() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proc.DetectAlive()
	}
}
