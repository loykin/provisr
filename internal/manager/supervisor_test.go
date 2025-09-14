package manager

import (
	"context"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/logger"
	"github.com/loykin/provisr/internal/process"
)

// Local helpers to avoid name collisions with other test files.
func requireUnixS(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix-like environment")
	}
}

func waitUntilS(timeout, step time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(step)
	}
	return false
}

// merge env passthrough
func mergeEnvS(s process.Spec) []string { return s.Env }

// Test that supervisor observes a start and a natural stop and invokes record callbacks exactly once per event.
func TestSupervisor_RecordsStartAndStop(t *testing.T) {
	requireUnixS(t)
	dir := t.TempDir()
	spec := process.Spec{
		Name:    "sup-observe",
		Command: "sh -c 'echo hi; sleep 0.2'",
		Log:     logger.Config{Dir: filepath.Join(dir, "logs")},
	}
	var starts, stops int32
	recStart := func(*process.Process) { atomic.AddInt32(&starts, 1) }
	recStop := func(*process.Process, error) { atomic.AddInt32(&stops, 1) }

	h := newHandler(spec, mergeEnvS, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.run(ctx)

	// Start the process
	r := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: r}
	if err := <-r; err != nil {
		t.Fatalf("start error: %v", err)
	}

	// Run supervisor
	sctx, scancel := context.WithCancel(context.Background())
	defer scancel()
	sup := newSupervisor(sctx, h, recStart, recStop)
	go sup.Run()

	// Wait until process exits naturally and supervisor records stop
	ok := waitUntilS(2*time.Second, 20*time.Millisecond, func() bool {
		st := h.Snapshot()
		return !st.Running && atomic.LoadInt32(&stops) >= 1
	})
	if !ok {
		t.Fatalf("supervisor did not observe stop in time")
	}

	// Start should have been recorded once for the single run
	if atomic.LoadInt32(&starts) != 1 {
		t.Fatalf("expected 1 start record, got %d", starts)
	}
}

// Test that supervisor performs auto-restart and suppresses it after an explicit Stop (StopRequested=true).
func TestSupervisor_AutoRestartThenStopSuppress(t *testing.T) {
	requireUnixS(t)
	spec := process.Spec{
		Name:            "sup-restart",
		Command:         "sh -c 'sleep 0.05'",
		AutoRestart:     true,
		RestartInterval: 50 * time.Millisecond,
	}
	var startCount int32
	recStart := func(*process.Process) { atomic.AddInt32(&startCount, 1) }
	recStop := func(*process.Process, error) {}

	h := newHandler(spec, mergeEnvS, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.run(ctx)

	// Initial start
	r := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: r}
	if err := <-r; err != nil {
		t.Fatalf("start error: %v", err)
	}

	// Start supervisor
	sctx, scancel := context.WithCancel(context.Background())
	defer scancel()
	sup := newSupervisor(sctx, h, recStart, recStop)
	go sup.Run()

	// Expect at least one auto-restart (recordStart called at least twice: first run + one restart)
	ok := waitUntilS(2*time.Second, 20*time.Millisecond, func() bool {
		rs := h.Snapshot().Restarts
		return rs >= 1 && atomic.LoadInt32(&startCount) >= 2
	})
	if !ok {
		st := h.Snapshot()
		t.Fatalf("expected at least one restart; restarts=%d startCount=%d running=%v", st.Restarts, startCount, st.Running)
	}

	// Now stop and ensure no further restarts occur for a grace period
	r2 := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStop, Wait: 1 * time.Second, Reply: r2}

	// After stop, process should be not running and startCount should remain stable
	prev := atomic.LoadInt32(&startCount)
	// Wait grace window greater than RestartInterval
	time.Sleep(200 * time.Millisecond)
	st := h.Status()
	if st.Running {
		t.Fatalf("expected not running after stop, got running")
	}
	if atomic.LoadInt32(&startCount) != prev {
		t.Fatalf("startCount changed after stop; got %d want %d", startCount, prev)
	}
}

// Test that supervisor shutdown stops monitoring and does not attempt new restarts after cancellation.
func TestSupervisor_ShutdownStopsMonitoring(t *testing.T) {
	requireUnixS(t)
	spec := process.Spec{
		Name:            "sup-shutdown",
		Command:         "sleep 2",
		AutoRestart:     true,
		RestartInterval: 50 * time.Millisecond,
	}
	var startCount int32
	recStart := func(*process.Process) { atomic.AddInt32(&startCount, 1) }
	recStop := func(*process.Process, error) {}

	h := newHandler(spec, mergeEnvS, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.run(ctx)

	// Start the process
	r := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: r}
	if err := <-r; err != nil {
		t.Fatalf("start error: %v", err)
	}

	// Start supervisor then immediately shut it down
	sctx, scancel := context.WithCancel(context.Background())
	sup := newSupervisor(sctx, h, recStart, recStop)
	go sup.Run()
	// Give it a brief moment to attach monitoring
	time.Sleep(50 * time.Millisecond)
	sup.Shutdown()
	scancel()

	// Stopping the handler should complete and no further starts should be recorded.
	r2 := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStop, Wait: 1 * time.Second, Reply: r2}

	// Ensure not running and stable start count after a grace period (no restarts without supervisor)
	prev := atomic.LoadInt32(&startCount)
	time.Sleep(150 * time.Millisecond)
	st := h.Status()
	if st.Running {
		t.Fatalf("expected not running after stop with supervisor shutdown")
	}
	if atomic.LoadInt32(&startCount) != prev {
		t.Fatalf("unexpected new start after supervisor shutdown; got %d want %d", startCount, prev)
	}
}
