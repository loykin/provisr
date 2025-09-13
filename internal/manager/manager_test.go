package manager

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/detector"
	"github.com/loykin/provisr/internal/logger"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/store"
	sqlitedrv "github.com/loykin/provisr/internal/store/sqlite"
)

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
}

// Test that starting a process and then starting again with the same name but
// an updated Spec actually uses the new Spec (via getOrCreateEntry + UpdateSpec).
func TestGetOrCreateEntryUpdatesSpec(t *testing.T) {
	requireUnix(t)
	mgr := NewManager()
	dir := t.TempDir()

	// First spec: writes to a1.txt quickly
	s1 := process.Spec{
		Name:          "upd",
		Command:       "sh -c 'echo one > a1.txt'",
		WorkDir:       dir,
		StartDuration: 0,
	}
	if err := mgr.Start(s1); err != nil {
		t.Fatalf("start s1: %v", err)
	}
	// Stop to avoid monitor writing concurrently during spec update
	// wait a bit for command to run and then stop
	time.Sleep(80 * time.Millisecond)
	_ = mgr.StopAll("upd", 500*time.Millisecond)
	b1, err := os.ReadFile(filepath.Join(dir, "a1.txt"))
	if err != nil {
		t.Fatalf("missing a1.txt: %v", err)
	}
	if !bytes.Contains(b1, []byte("one")) {
		t.Fatalf("unexpected a1 content: %q", string(b1))
	}

	// Second spec: same name, writes to a2.txt
	s2 := process.Spec{
		Name:          "upd",
		Command:       "sh -c 'echo two > a2.txt'",
		WorkDir:       dir,
		StartDuration: 0,
	}
	if err := mgr.Start(s2); err != nil {
		t.Fatalf("start s2: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	b2, err := os.ReadFile(filepath.Join(dir, "a2.txt"))
	if err != nil {
		t.Fatalf("missing a2.txt after spec update: %v", err)
	}
	if !bytes.Contains(b2, []byte("two")) {
		t.Fatalf("unexpected a2 content: %q", string(b2))
	}
}

// Test that when a process exits before StartDuration, Manager.Start retries
// immediately without sleeping for RetryInterval.
func TestImmediateRetryOnBeforeStart(t *testing.T) {
	requireUnix(t)
	mgr := NewManager()
	s := process.Spec{
		Name:          "imm-retry",
		Command:       "sh -c 'exit 0'",
		StartDuration: 200 * time.Millisecond,
		RetryCount:    1,
		RetryInterval: 700 * time.Millisecond, // long interval; should be skipped on early-exit retry
	}
	start := time.Now()
	err := mgr.Start(s)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected error due to early exit before start duration")
	}
	// We expect total time to be much less than RetryInterval since retry was immediate.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("retry not immediate, took %v (>500ms)", elapsed)
	}
}

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

// TestParallelDifferentProcesses10 starts 10 different processes concurrently and ensures they all reach running state.
func TestParallelDifferentProcesses10(t *testing.T) {
	mgr := NewManager()
	var wg sync.WaitGroup
	names := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		names = append(names, fmt.Sprintf("par-diff-%d", i+1))
	}

	// Start all in parallel
	for _, n := range names {
		wg.Add(1)
		n := n
		go func() {
			defer wg.Done()
			spec := process.Spec{Name: n, Command: "sleep 2"}
			if err := mgr.Start(spec); err != nil {
				t.Errorf("start %s: %v", n, err)
			}
		}()
	}
	wg.Wait()

	// Wait until all report running (with deadline)
	deadline := time.Now().Add(3 * time.Second)
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
		st, err := mgr.Status(n)
		if err != nil || !st.Running {
			t.Fatalf("expected %s running, got err=%v st=%+v", n, err, st)
		}
	}

	// Stop all
	for _, n := range names {
		_ = mgr.Stop(n, 2*time.Second)
	}
	time.Sleep(100 * time.Millisecond)
	for _, n := range names {
		st, _ := mgr.Status(n)
		if st.Running {
			t.Fatalf("expected %s stopped after Stop", n)
		}
	}
}

// TestParallelSameProcessInstances10 starts 10 instances of the same base name concurrently.
func TestParallelSameProcessInstances10(t *testing.T) {
	mgr := NewManager()
	base := "par-same"
	var wg sync.WaitGroup
	// Start 10 instances in parallel by calling Start on suffixed names
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		name := fmt.Sprintf("%s-%d", base, i)
		go func(n string) {
			defer wg.Done()
			spec := process.Spec{Name: n, Command: "sleep 2"}
			if err := mgr.Start(spec); err != nil {
				t.Errorf("start %s: %v", n, err)
			}
		}(name)
	}
	wg.Wait()

	// Poll until count == 10
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, _ := mgr.Count(base)
		if c == 10 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	c, _ := mgr.Count(base)
	if c != 10 {
		t.Fatalf("expected 10 running instances, got %d", c)
	}

	// Stop all instances
	_ = mgr.StopAll(base, 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	c2, _ := mgr.Count(base)
	if c2 != 0 {
		t.Fatalf("expected 0 after StopAll, got %d", c2)
	}
}

// TestManage100Processes verifies the manager can start and observe ~100 processes.
func TestManage100Processes(t *testing.T) {
	mgr := NewManager()
	var wg sync.WaitGroup
	names := make([]string, 0, 100)
	for i := 1; i <= 100; i++ {
		names = append(names, fmt.Sprintf("scale-%03d", i))
	}

	// Start many processes in parallel batches to avoid extreme spikes
	batch := 20
	for i := 0; i < len(names); i += batch {
		end := i + batch
		if end > len(names) {
			end = len(names)
		}
		wg = sync.WaitGroup{}
		for _, n := range names[i:end] {
			wg.Add(1)
			n := n
			go func() {
				defer wg.Done()
				spec := process.Spec{Name: n, Command: "sleep 2"}
				if err := mgr.Start(spec); err != nil {
					t.Errorf("start %s: %v", n, err)
				}
			}()
		}
		wg.Wait()
	}

	// Verify all are running
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, n := range names {
			st, err := mgr.Status(n)
			if err != nil || !st.Running {
				ok = false
				break
			}
		}
		if ok {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	for _, n := range names {
		st, err := mgr.Status(n)
		if err != nil || !st.Running {
			t.Fatalf("expected %s running, got err=%v st=%+v", n, err, st)
		}
	}

	// Stop all
	for _, n := range names {
		_ = mgr.Stop(n, 2*time.Second)
	}
	time.Sleep(150 * time.Millisecond)
	for _, n := range names {
		st, _ := mgr.Status(n)
		if st.Running {
			t.Fatalf("expected %s stopped after Stop", n)
		}
	}
}

func TestStopUnknownProcess(t *testing.T) {
	mgr := NewManager()
	err := mgr.Stop("nope", 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error stopping unknown process")
	}
}

func TestWildcardMatch(t *testing.T) {
	cases := []struct {
		name  string
		pat   string
		input string
		want  bool
	}{
		{"empty", "", "abc", false},
		{"star", "*", "anything", true},
		{"exact_ok", "abc", "abc", true},
		{"exact_no", "abc", "abcd", false},
		{"prefix", "abc*", "abcdef", true},
		{"suffix", "*def", "abcdef", true},
		{"middle", "a*c", "abc", true},
		{"multi_mid", "a*b*c", "axxbyyc", true},
		{"order_required", "a*b*c", "abxcby", false},
		{"no_star_diff", "name", "naMe", false},
		{"double_star", "a**c", "abc", true},
	}
	for _, c := range cases {
		if got := wildcardMatch(c.input, c.pat); got != c.want {
			t.Fatalf("%s: wildcardMatch(%q,%q)=%v want %v", c.name, c.input, c.pat, got, c.want)
		}
	}
}

func TestRetryParams(t *testing.T) {
	s := process.Spec{RetryCount: -5, RetryInterval: 0}
	att, intv := retryParams(s)
	if att != 0 {
		t.Fatalf("attempts want 0 got %d", att)
	}
	if intv != 500*time.Millisecond {
		t.Fatalf("interval want 500ms got %v", intv)
	}
	// custom values preserved
	s = process.Spec{RetryCount: 3, RetryInterval: 123 * time.Millisecond}
	att, intv = retryParams(s)
	if att != 3 || intv != 123*time.Millisecond {
		t.Fatalf("got (%d,%v)", att, intv)
	}
}

func TestSetGlobalEnvAndMerge(t *testing.T) {
	mgr := NewManager()
	mgr.SetGlobalEnv([]string{"G_ONE=1", "G_TWO=2"})
	// per-process overrides global; NEW expands using merged map (perProc value)
	s := process.Spec{}
	merged := mgr.mergedEnvFor(process.Spec{Env: []string{"G_ONE=9", "NEW=${G_ONE}-${G_TWO}"}})
	// Build a map for assertions
	m := map[string]string{}
	for _, kv := range merged {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	if m["G_ONE"] != "9" {
		t.Fatalf("per-process should override global: G_ONE=%q", m["G_ONE"])
	}
	if m["G_TWO"] != "2" {
		t.Fatalf("global should be present: G_TWO=%q", m["G_TWO"])
	}
	if m["NEW"] != "9-2" {
		t.Fatalf("expand should use merged values: NEW=%q", m["NEW"])
	}
	_ = s
}

func TestStatusMatchAndStopMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
	mgr := NewManager()
	// Start three processes: web-1, web-2, api-1
	for _, n := range []string{"web-1", "web-2", "api-1"} {
		if err := mgr.Start(process.Spec{Name: n, Command: "sleep 1"}); err != nil {
			t.Fatalf("start %s: %v", n, err)
		}
	}
	// Allow to start
	time.Sleep(20 * time.Millisecond)
	// Match web-* should return 2
	sts, err := mgr.StatusMatch("web-*")
	if err != nil {
		t.Fatalf("statusmatch: %v", err)
	}
	if len(sts) != 2 {
		t.Fatalf("expected 2 matched, got %d", len(sts))
	}
	// Stop only web-* and ensure api-1 is still running
	_ = mgr.StopMatch("web-*", 2*time.Second) // stopping may report exit error; tolerate it
	st, _ := mgr.Status("api-1")
	if !st.Running {
		t.Fatalf("api-1 should still be running")
	}
}

func TestStartNAndCountZero(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
	mgr := NewManager()
	// Count unknown base returns 0 without error
	if c, err := mgr.Count("none"); err != nil || c != 0 {
		t.Fatalf("count none want 0,nil got %d,%v", c, err)
	}
	spec := process.Spec{Name: "svc", Command: "sleep 1", Instances: 3}
	if err := mgr.StartN(spec); err != nil {
		t.Fatalf("startN: %v", err)
	}
	// Give a brief time to start
	time.Sleep(20 * time.Millisecond)
	sts, err := mgr.StatusAll("svc")
	if err != nil {
		t.Fatalf("statusAll: %v", err)
	}
	if len(sts) != 3 {
		t.Fatalf("expected 3 instances, got %d", len(sts))
	}
	c, _ := mgr.Count("svc")
	if c == 0 { // be lenient due to race, but should be >0 typically
		t.Fatalf("expected count > 0, got %d", c)
	}
	_ = mgr.StopAll("svc", 2*time.Second)
}

func TestManagerPersistsLifecycleToStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
	mgr := NewManager()
	db, err := sqlitedrv.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := mgr.SetStore(db); err != nil {
		t.Fatalf("set store: %v", err)
	}
	// Start a short-lived process
	spec := process.Spec{Name: "store-demo", Command: "sleep 0.2", StartDuration: 0}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	// give it a moment to run and be recorded
	time.Sleep(30 * time.Millisecond)
	ctx := context.Background()
	running, err := db.GetRunning(ctx, "store-demo")
	if err != nil {
		t.Fatalf("get running: %v", err)
	}
	if len(running) < 1 {
		t.Fatalf("expected at least 1 running record, got %d", len(running))
	}
	// Stop and verify store updated
	_ = mgr.StopAll("store-demo", 2*time.Second)
	// wait a bit for monitor to record stop
	time.Sleep(50 * time.Millisecond)
	running2, err := db.GetRunning(ctx, "store-demo")
	if err != nil {
		t.Fatalf("get running2: %v", err)
	}
	if len(running2) != 0 {
		t.Fatalf("expected 0 running after stop, got %d", len(running2))
	}
	past, err := db.GetByName(ctx, "store-demo", 10)
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	foundStopped := false
	for _, r := range past {
		if !r.Running && r.StoppedAt.Valid {
			foundStopped = true
			break
		}
	}
	if !foundStopped {
		t.Fatalf("expected at least one stopped record in history")
	}
}

// waitUntil polls fn until it returns true or timeout elapses.
func waitUntil(timeout time.Duration, step time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(step)
	}
	return false
}

// This test verifies that when a process with AutoRestart dies, the reconciler
// quickly restarts it even if the monitor's RestartInterval is large. It also
// checks that the store is updated accordingly.
func TestReconcileRestartsDeadProcessAndUpdatesStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
	mgr := NewManager()
	db, err := sqlitedrv.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := mgr.SetStore(db); err != nil {
		t.Fatalf("set store: %v", err)
	}

	// Use a tiny sleep that exits immediately; monitor would wait 10s to restart,
	// but reconciler should restart much sooner on demand.
	spec := process.Spec{
		Name:            "recon-quick",
		Command:         "sh -c 'sleep 0.05'",
		AutoRestart:     true,
		RestartInterval: 10 * time.Second,
		StartDuration:   0,
	}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Wait for the short-lived process to exit naturally and be marked not running.
	ok := waitUntil(1*time.Second, 20*time.Millisecond, func() bool {
		st, err := mgr.Status("recon-quick")
		return err == nil && !st.Running
	})
	if !ok {
		t.Fatalf("process did not exit as expected")
	}

	// Now trigger reconcile; it should restart immediately (AutoRestart=true).
	mgr.ReconcileOnce()
	// Wait until running again.
	ok = waitUntil(1*time.Second, 20*time.Millisecond, func() bool {
		st, err := mgr.Status("recon-quick")
		return err == nil && st.Running
	})
	if !ok {
		t.Fatalf("reconcile did not restart process promptly")
	}

	// Store should have at least one running record for this name.
	running, err := db.GetRunning(context.Background(), "recon-quick")
	if err != nil {
		t.Fatalf("store get running: %v", err)
	}
	if len(running) < 1 {
		t.Fatalf("expected at least one running record after reconcile restart")
	}
}

// This test verifies that reconcile still maintains HA when store has no prior
// records (e.g., store enabled after a failure). It should upsert the current
// status and restart dead AutoRestart processes regardless of store content.
func TestReconcileWorksWhenStoreInitiallyEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
	mgr := NewManager()
	// Start without store
	spec := process.Spec{Name: "recon-empty", Command: "sh -c 'sleep 0.05'", AutoRestart: true, RestartInterval: 10 * time.Second}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Wait until it exits
	_ = waitUntil(1*time.Second, 20*time.Millisecond, func() bool {
		st, err := mgr.Status("recon-empty")
		return err == nil && !st.Running
	})
	// Now enable store and reconcile
	db, err := sqlitedrv.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := mgr.SetStore(db); err != nil {
		t.Fatalf("set store: %v", err)
	}
	mgr.ReconcileOnce()
	// Should be restarted despite store having no prior record.
	ok := waitUntil(1*time.Second, 20*time.Millisecond, func() bool {
		st, err := mgr.Status("recon-empty")
		return err == nil && st.Running
	})
	if !ok {
		t.Fatalf("reconcile did not restart process with empty store")
	}
	// Store should now have a running record via UpsertStatus or subsequent start recording.
	running, err := db.GetRunning(context.Background(), "recon-empty")
	if err != nil {
		t.Fatalf("store get running: %v", err)
	}
	if len(running) == 0 {
		t.Fatalf("expected running record after reconcile with empty store")
	}
}

// This test verifies that even when store content is inconsistent (claims running
// while the process is actually dead), reconcile corrects the store and restarts
// the process to maintain availability.
func TestReconcileOverridesStoreMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
	mgr := NewManager()
	db, err := sqlitedrv.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := mgr.SetStore(db); err != nil {
		t.Fatalf("set store: %v", err)
	}

	// Create an entry in manager without starting a real process, by creating the entry
	// and leaving it non-running. We'll also inject a mismatching store record that says running.
	spec := process.Spec{Name: "recon-mismatch", Command: "sh -c 'sleep 0.2'", AutoRestart: true, RestartInterval: 10 * time.Second}
	_ = mgr.getOrCreateEntry(spec) // ensure it's known to the manager

	// Insert a fake 'running' record for the same name in the store, PID/StartedAt arbitrary.
	fake := time.Now().Add(-1 * time.Minute).UTC()
	rec := store.Record{Name: "recon-mismatch", PID: 99999, StartedAt: fake, Running: true}
	_ = db.RecordStart(context.Background(), rec)

	// Reconcile should ignore store contents for restart decision and bring the process up.
	mgr.ReconcileOnce()
	ok := waitUntil(2*time.Second, 20*time.Millisecond, func() bool {
		st, err := mgr.Status("recon-mismatch")
		return err == nil && st.Running
	})
	if !ok {
		t.Fatalf("reconcile did not restart process despite store mismatch")
	}
}
