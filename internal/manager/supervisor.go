package manager

import (
	"context"
	"time"

	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
)

// supervisor observes one handler's process and applies policies (autorestart/backoff/metrics/history).
// It must be created and owned by Manager. It never accesses process directly except for
// read-only snapshots and monitor coordination hooks.
// All lifecycle operations (start/stop) are invoked via the handler's control channel.

type supervisor struct {
	h      *handler
	ctx    context.Context
	cancel context.CancelFunc
	// callbacks for history persistence (provided by Manager)
	recordStart func(*process.Process)
	recordStop  func(*process.Process, error)
	// internal: whether we've already observed the first run for this handler
	seenFirstRun bool
}

func newSupervisor(ctx context.Context, h *handler, recStart func(*process.Process), recStop func(*process.Process, error)) *supervisor {
	cctx, cancel := context.WithCancel(ctx)
	return &supervisor{h: h, ctx: cctx, cancel: cancel, recordStart: recStart, recordStop: recStop}
}

func (s *supervisor) Shutdown() { s.cancel() }

func (s *supervisor) Run() {
	// Track last observed run identity to attach waiter once per run.
	var lastPID int
	var lastStartedAt time.Time
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		st := s.h.Snapshot()
		alive, _ := s.h.proc.DetectAlive()
		if alive {
			if st.PID != 0 && (st.PID != lastPID || !st.StartedAt.Equal(lastStartedAt)) {
				// New run detected: attach waiter once.
				if s.h.proc.MonitoringStartIfNeeded() {
					go s.waitAndHandleExit()
					// Record start exactly once per run here (centralized observability, W4).
					name := s.h.Spec().Name
					metrics.IncStart(name)
					if d := s.h.Spec().StartDuration; d > 0 {
						metrics.ObserveStartDuration(name, d.Seconds())
					}
					if s.seenFirstRun {
						_ = s.h.proc.IncRestarts()
						metrics.IncRestart(name)
					} else {
						s.seenFirstRun = true
					}
					if s.recordStart != nil {
						s.recordStart(s.h.proc)
					}
				}
				lastPID, lastStartedAt = st.PID, st.StartedAt
			}
		} else {
			// Not alive. If AutoRestart desired and stop not requested, try to start.
			s.tryAutoStart()
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *supervisor) waitAndHandleExit() {
	// Ensure we wait on cmd.Wait and transition process state.
	cmd := s.h.proc.CopyCmd()
	var err error
	if cmd != nil {
		err = cmd.Wait()
	}
	s.h.proc.CloseWaitDone()
	s.h.proc.MarkExited(err)
	// Close any log writers now that the process has exited.
	s.h.proc.CloseWriters()
	s.h.proc.MonitoringStop()
	// Metrics and history for stop
	name := s.h.Spec().Name
	metrics.IncStop(name)
	if s.recordStop != nil {
		s.recordStop(s.h.proc, err)
	}
	// Decide on restart
	s.tryAutoStart()
}

func (s *supervisor) tryAutoStart() {
	if s.h.StopRequested() {
		return
	}
	spec := s.h.Spec()
	if !spec.AutoRestart {
		return
	}
	// Enforce minimal restart interval
	restInt := spec.RestartInterval
	if restInt <= 0 {
		restInt = 50 * time.Millisecond
	}
	t := time.NewTimer(restInt)
	select {
	case <-t.C:
	case <-s.ctx.Done():
		if !t.Stop() {
			<-t.C
		}
		return
	}
	// Attempt start with backoff policy migrated here.
	attempts := spec.RetryCount
	if attempts < 0 {
		attempts = 0
	}
	interval := spec.RetryInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	for i := 0; i <= attempts; i++ {
		// double-check stop flag each iteration
		if s.h.StopRequested() || s.ctx.Err() != nil {
			return
		}
		reply := make(chan error, 1)
		s.h.ctrl <- CtrlMsg{Type: CtrlStart, Spec: spec, Reply: reply}
		err := <-reply
		if err == nil {
			// Successful start; observability is handled in Run() when the new run is observed.
			return
		}
		// Backoff: immediate retry if early exit before startsecs; else sleep
		if i < attempts {
			if !process.IsBeforeStartErr(err) {
				time.Sleep(interval)
			}
			continue
		}
		// Exhausted attempts: give up for this cycle
		return
	}
}
