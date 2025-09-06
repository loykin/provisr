package process

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	b, err := os.ReadFile(pidfile)
	if err != nil || len(strings.TrimSpace(string(b))) == 0 {
		t.Fatalf("pidfile not written: %v, content=%q", err, string(b))
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
		Log:     logger.Config{Dir: logs},
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
	// Wait for process to exit and simulate monitor behavior to close waitDone
	c := r.CopyCmd()
	done := make(chan struct{})
	go func() {
		_ = c.Wait()
		r.CloseWaitDone()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("process did not exit in time")
	}
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

func TestEnforceStartDurationEarlyExit(t *testing.T) {
	requireUnix(t)
	spec := Spec{Name: "early", Command: "sh -c 'sleep 0.05'"}
	r := New(spec)
	cmd := r.ConfigureCmd(nil)
	if err := r.TryStart(cmd); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Simulate monitor: wait for exit then close waitDone and mark exited
	c := r.CopyCmd()
	go func() {
		err := c.Wait()
		r.CloseWaitDone()
		r.MarkExited(err)
	}()

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
	// Simulate monitor that will signal after the process exits (after 300ms),
	// so EnforceStartDuration should return on its deadline before that.
	c := r.CopyCmd()
	go func() {
		err := c.Wait()
		r.CloseWaitDone()
		r.MarkExited(err)
	}()

	start := time.Now()
	if err := r.EnforceStartDuration(d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < d-20*time.Millisecond {
		t.Fatalf("EnforceStartDuration returned too early: %v < %v", elapsed, d)
	}
}
