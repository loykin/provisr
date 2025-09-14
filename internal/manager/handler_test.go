package manager

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// mergeEnv passthrough helper for tests
func mergeEnv(s process.Spec) []string { return s.Env }

// record callbacks stubs
func recStart(*process.Process)       {}
func recStop(*process.Process, error) {}

func TestHandler_StartStop_Serialize(t *testing.T) {
	requireUnix(t)
	spec := process.Spec{Name: "h-ss", Command: "sleep 1"}
	h := newHandler(spec, mergeEnv, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.run(ctx)

	// Start
	r1 := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: r1}
	if err := <-r1; err != nil {
		t.Fatalf("start: %v", err)
	}

	// Assert running
	st := h.Status()
	if !st.Running || st.PID <= 0 {
		t.Fatalf("expected running after start, got %+v", st)
	}

	// Stop
	r2 := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStop, Wait: 2 * time.Second, Reply: r2}
	if err := <-r2; err != nil {
		t.Fatalf("stop: %v", err)
	}
	st2 := h.Status()
	if st2.Running {
		t.Fatalf("expected stopped after CtrlStop, got running")
	}
	if !h.StopRequested() {
		t.Fatalf("StopRequested should be true after stop")
	}
}

func TestHandler_DuplicateStartIgnored_Local(t *testing.T) {
	requireUnix(t)
	spec := process.Spec{Name: "h-dupe", Command: "sleep 1"}
	h := newHandler(spec, mergeEnv, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.run(ctx)

	ch1 := make(chan error, 1)
	ch2 := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: ch1}
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: ch2}

	if err := <-ch1; err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := <-ch2; err != nil {
		t.Fatalf("second start (ignored) returned error: %v", err)
	}

	// Confirm still single run by observing stable PID shortly after
	st1 := h.Status()
	time.Sleep(50 * time.Millisecond)
	st2 := h.Status()
	if !st1.Running || !st2.Running || st1.PID != st2.PID {
		t.Fatalf("duplicate start should not change PID; got %v -> %v", st1.PID, st2.PID)
	}

	// Cleanup
	reply := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStop, Wait: 2 * time.Second, Reply: reply}
	<-reply
}

func TestHandler_UpdateSpec_Applied(t *testing.T) {
	requireUnix(t)
	dir := t.TempDir()
	// First spec writes file a1.txt
	s1 := process.Spec{Name: "h-upd", Command: "sh -c 'echo one > a1.txt'", WorkDir: dir}
	h := newHandler(s1, mergeEnv, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.run(ctx)

	// Start with s1
	r1 := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: s1, Reply: r1}
	if err := <-r1; err != nil {
		t.Fatalf("start s1: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	b1, err := os.ReadFile(filepath.Join(dir, "a1.txt"))
	if err != nil || len(b1) == 0 {
		t.Fatalf("expected a1.txt to be written: %v", err)
	}

	// Update spec: writes a2.txt
	s2 := process.Spec{Name: "h-upd", Command: "sh -c 'echo two > a2.txt'", WorkDir: dir}
	upd := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlUpdateSpec, Spec: s2, Reply: upd}
	<-upd

	// Start again (no running process now; previous was short-lived)
	r2 := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: s2, Reply: r2}
	if err := <-r2; err != nil {
		t.Fatalf("start s2: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	b2, err := os.ReadFile(filepath.Join(dir, "a2.txt"))
	if err != nil || len(b2) == 0 {
		t.Fatalf("expected a2.txt to be written: %v", err)
	}
}

func TestHandler_StatusConsistency(t *testing.T) {
	requireUnix(t)
	spec := process.Spec{Name: "h-st", Command: "sleep 0.5"}
	h := newHandler(spec, mergeEnv, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.run(ctx)

	r := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: r}
	if err := <-r; err != nil {
		t.Fatalf("start: %v", err)
	}

	ok := waitUntil(1*time.Second, 20*time.Millisecond, func() bool {
		st := h.Status()
		return st.Running && st.PID > 0
	})
	if !ok {
		t.Fatalf("expected running status soon after start")
	}
}

func TestHandler_ShutdownMessage(t *testing.T) {
	requireUnix(t)
	spec := process.Spec{Name: "h-shdn", Command: "sleep 1"}
	h := newHandler(spec, mergeEnv, recStart, recStop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		h.run(ctx)
		close(done)
	}()

	// Start quickly
	r := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: r}
	if err := <-r; err != nil {
		t.Fatalf("start: %v", err)
	}

	// Request shutdown
	reply := make(chan error, 1)
	h.ctrl <- CtrlMsg{Type: CtrlShutdown, Reply: reply}
	if err := <-reply; err != nil {
		t.Fatalf("shutdown reply: %v", err)
	}

	// The run goroutine should exit soon
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("handler.run did not exit after shutdown")
	}
}
