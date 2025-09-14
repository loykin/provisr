package manager

import (
	"context"
	"sync"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// CtrlType enumerates control message kinds handled by handler.
type CtrlType int

const (
	CtrlStart CtrlType = iota
	CtrlStop
	CtrlUpdateSpec
	CtrlShutdown
)

// CtrlMsg is a control-plane message sent to a handler to serialize lifecycle ops.
type CtrlMsg struct {
	Type  CtrlType
	Spec  process.Spec
	Wait  time.Duration
	Reply chan error
}

// handler owns the control path and monitoring for a single process.
type handler struct {
	mu   sync.RWMutex
	spec process.Spec
	proc *process.Process
	ctrl chan CtrlMsg
	// injected callbacks (no direct Manager dependency)
	recordStart func(*process.Process)
	recordStop  func(*process.Process, error)
	mergeEnv    func(process.Spec) []string
	// guard against duplicate concurrent start attempts
	starting bool
}

func newHandler(spec process.Spec, mergeEnv func(process.Spec) []string, recStart func(*process.Process), recStop func(*process.Process, error)) *handler {
	return &handler{
		spec:        spec,
		proc:        process.New(spec),
		ctrl:        make(chan CtrlMsg, 16),
		recordStart: recStart,
		recordStop:  recStop,
		mergeEnv:    mergeEnv,
	}
}

func (h *handler) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			_ = h.stopNow(3 * time.Second)
			return
		case msg := <-h.ctrl:
			var err error
			switch msg.Type {
			case CtrlStart:
				// Apply incoming spec, if provided, to ensure latest config at start time.
				if msg.Spec.Name != "" {
					h.mu.Lock()
					h.spec = msg.Spec
					h.proc.UpdateSpec(h.spec)
					h.mu.Unlock()
				}
				// avoid duplicate starts while a start is already in-flight
				alive, _ := h.proc.DetectAlive()
				if alive {
					err = nil
					break
				}
				// set starting flag if not already
				h.mu.Lock()
				if h.starting {
					h.mu.Unlock()
					// ignore duplicate start request
					err = nil
					break
				}
				h.starting = true
				h.mu.Unlock()
				err = h.startWithRetries()
				// clear starting flag now that attempt finished
				h.mu.Lock()
				h.starting = false
				h.mu.Unlock()
			case CtrlStop:
				err = h.stopNow(msg.Wait)
			case CtrlUpdateSpec:
				h.mu.Lock()
				h.spec = msg.Spec
				h.proc.UpdateSpec(h.spec)
				h.mu.Unlock()
			case CtrlShutdown:
				_ = h.stopNow(3 * time.Second)
				if msg.Reply != nil {
					msg.Reply <- nil
				}
				return
			}
			if msg.Reply != nil {
				msg.Reply <- err
			}
		}
	}
}

func (h *handler) startWithRetries() error {
	// Dieted: single attempt start + success judgment only; policy/metrics moved to Supervisor.
	alive, _ := h.proc.DetectAlive()
	if alive {
		return nil
	}
	if err := h.tryStartOnce(); err != nil {
		return err
	}
	// Enforce start duration; caller (Supervisor) handles metrics/history/backoff.
	return h.postStart()
}

func (h *handler) tryStartOnce() error {
	h.mu.RLock()
	s := h.spec
	merge := h.mergeEnv
	h.mu.RUnlock()
	mergedEnv := []string(nil)
	if merge != nil {
		mergedEnv = merge(s)
	}
	cmd := h.proc.ConfigureCmd(mergedEnv)
	if err := h.proc.TryStart(cmd); err != nil {
		return err
	}
	return nil
}

func (h *handler) postStart() error {
	h.mu.RLock()
	s := h.spec
	h.mu.RUnlock()
	// Enforce start duration to ensure the process stays up for the configured time.
	if err := h.proc.EnforceStartDuration(s.StartDuration); err != nil {
		h.proc.RemovePIDFile()
		h.proc.MarkExited(err)
		return err
	}
	return nil
}

func (h *handler) stopNow(wait time.Duration) error {
	alive, _ := h.proc.DetectAlive()
	if !alive {
		return nil
	}
	// Pure stop; metrics/history handled by Supervisor.
	_ = h.proc.Stop(wait)
	// For control-plane Stop, treat termination as success regardless of exit error.
	return nil
}

// Status returns an externally consumable process.Status snapshot.
func (h *handler) Status() process.Status {
	alive, by := h.proc.DetectAlive()
	rs := h.proc.Snapshot()
	return process.Status{
		Name:       rs.Name,
		Running:    alive,
		PID:        rs.PID,
		StartedAt:  rs.StartedAt,
		StoppedAt:  rs.StoppedAt,
		ExitErr:    rs.ExitErr,
		DetectedBy: by,
		Restarts:   rs.Restarts,
	}
}

// Spec returns a copy of the current spec.
func (h *handler) Spec() process.Spec {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.spec
}

// Snapshot returns the raw internal process snapshot.
func (h *handler) Snapshot() process.Status {
	return h.proc.Snapshot()
}

// StopRequested returns whether stop has been requested.
func (h *handler) StopRequested() bool {
	return h.proc.StopRequested()
}
