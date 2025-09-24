package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
)

// ManagedProcess combines Manager-Handler-Supervisor responsibilities into a single,
// clear state machine with minimal locking and explicit coordination.
//
// Lock Hierarchy (to prevent deadlocks):
// 1. mu (state lock) - protects process state and control flags
// 2. Process internal locks (managed by process.Process)
//
// State Machine:
// Stopped -> Starting -> Running -> Stopping -> Stopped
type ManagedProcess struct {
	mu            sync.RWMutex
	state         processState
	proc          *process.Process
	restarts      uint32
	cmdChan       chan command
	doneChan      chan struct{}
	lastRestartAt time.Time
	history       []history.Sink
	envMerger     func(process.Spec) []string
}

// Recover seeds the process with a PID and spec loaded from a PID file and sets state accordingly.
func (up *ManagedProcess) Recover(spec process.Spec, pid int) {
	up.mu.Lock()
	if up.proc == nil {
		up.proc = process.New(spec)
	} else {
		up.proc.UpdateSpec(spec)
	}
	up.proc.SeedPID(pid)
	up.mu.Unlock()

	alive, _ := up.proc.DetectAlive()
	if alive {
		up.setState(StateRunning)
	} else {
		up.setState(StateStopped)
	}
}

type processState int32

const (
	StateStopped processState = iota
	StateStarting
	StateRunning
	StateStopping
)

func (s processState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

type command struct {
	action commandAction
	spec   process.Spec
	wait   time.Duration
	reply  chan error
}

type commandAction int

const (
	ActionStart commandAction = iota
	ActionStop
	ActionUpdateSpec
	ActionShutdown
)

// NewManagedProcess creates a new unified process manager
func NewManagedProcess(
	spec process.Spec,
	envMerger func(process.Spec) []string,
) *ManagedProcess {
	up := &ManagedProcess{
		state:     StateStopped,
		proc:      process.New(spec),
		cmdChan:   make(chan command, 16), // Buffered to prevent blocking
		doneChan:  make(chan struct{}),
		envMerger: envMerger,
	}

	go up.runStateMachine()
	return up
}

// SetHistory configures history sinks (thread-safe)
func (up *ManagedProcess) SetHistory(sinks ...history.Sink) {
	up.mu.Lock()
	up.history = append([]history.Sink(nil), sinks...)
	up.mu.Unlock()
}

// Start initiates process start (non-blocking)
func (up *ManagedProcess) Start(spec process.Spec) error {
	reply := make(chan error, 1)

	select {
	case up.cmdChan <- command{action: ActionStart, spec: spec, reply: reply}:
		return <-reply
	case <-up.doneChan:
		return fmt.Errorf("process manager shutting down")
	}
}

// Stop initiates process stop (non-blocking)
func (up *ManagedProcess) Stop(wait time.Duration) error {
	reply := make(chan error, 1)

	select {
	case up.cmdChan <- command{action: ActionStop, wait: wait, reply: reply}:
		return <-reply
	case <-up.doneChan:
		return fmt.Errorf("process manager shutting down")
	}
}

// Status returns current status (lock-minimal)
func (up *ManagedProcess) Status() process.Status {
	up.mu.RLock()
	//name := up.name
	restarts := up.restarts
	state := up.state
	proc := up.proc
	spec := proc.GetSpec()
	up.mu.RUnlock()

	if proc == nil {
		return process.Status{}
	}

	// Get process status (process handles its own locking)
	status := proc.Snapshot()
	alive, detectedBy := proc.DetectAlive()

	// Ensure name and state are properly set
	status.Name = spec.Name
	status.Running = alive && state == StateRunning
	status.DetectedBy = detectedBy
	status.Restarts = restarts
	status.State = state.String() // Add state machine state

	return status
}

// UpdateSpec updates process specification
func (up *ManagedProcess) UpdateSpec(spec process.Spec) error {
	reply := make(chan error, 1)

	select {
	case up.cmdChan <- command{action: ActionUpdateSpec, spec: spec, reply: reply}:
		return <-reply
	case <-up.doneChan:
		return fmt.Errorf("process manager shutting down")
	}
}

// Shutdown gracefully shuts down the process manager
func (up *ManagedProcess) Shutdown() error {
	reply := make(chan error, 1)

	select {
	case up.cmdChan <- command{action: ActionShutdown, reply: reply}:
		return <-reply
	case <-up.doneChan:
		return nil // Already shut down
	}
}

// runStateMachine is the core state machine (single goroutine, no races)
func (up *ManagedProcess) runStateMachine() {
	defer close(up.doneChan)

	ticker := time.NewTicker(1 * time.Second) // Health check interval
	defer ticker.Stop()

	for {
		select {
		case cmd := <-up.cmdChan:
			up.handleCommand(cmd)

		case <-ticker.C:
			up.checkProcessHealth()

			// Auto-restart when process is stopped and autoRestart is enabled
			if up.proc != nil && up.proc.GetAutoStart() {
				up.mu.RLock()
				currentState := up.state
				proc := up.proc
				//spec := up.spec
				spec := proc.GetSpec()
				last := up.lastRestartAt
				up.mu.RUnlock()

				if currentState == StateStopped && proc != nil && !proc.StopRequested() {
					alive, _ := proc.DetectAlive()
					if !alive {
						// Respect restart interval from spec (default small delay)
						interval := spec.RestartInterval
						if interval <= 0 {
							interval = 3 * time.Second
						}
						if time.Since(last) >= interval {
							// Attempt restart with last known spec
							if err := up.doStart(*spec); err == nil {
								up.mu.Lock()
								up.lastRestartAt = time.Now()
								up.restarts++
								up.mu.Unlock()
							}
						}
					}
				}
			}
		}
	}
}

// handleCommand processes commands with clear state transitions
func (up *ManagedProcess) handleCommand(cmd command) {
	var err error

	switch cmd.action {
	case ActionStart:
		err = up.handleStart(cmd.spec)
	case ActionStop:
		err = up.handleStop(cmd.wait)
	case ActionUpdateSpec:
		err = up.handleUpdateSpec(cmd.spec)
	case ActionShutdown:
		err = up.handleShutdown()
		if cmd.reply != nil {
			cmd.reply <- err
		}
		return // Exit state machine
	}

	if cmd.reply != nil {
		cmd.reply <- err
	}
}

// handleStart manages start logic with clear state transitions
func (up *ManagedProcess) handleStart(newSpec process.Spec) error {
	up.mu.Lock()
	currentState := up.state
	proc := up.proc
	spec := proc.GetSpec()
	name := spec.Name
	up.mu.Unlock()

	switch currentState {
	case StateRunning:
		// Already running, check if process is actually alive
		if alive, _ := up.proc.DetectAlive(); alive {
			snapshot := up.proc.Snapshot()
			return fmt.Errorf("process '%s' is already running (PID: %d, state: %s)",
				name, snapshot.PID, currentState.String())
		}

		// Process died, transition to stopped and try start
		up.setState(StateStopped)
		fallthrough

	case StateStopped:
		return up.doStart(newSpec)

	case StateStarting:
		return fmt.Errorf("process '%s' is already starting, please wait or stop first", name)

	case StateStopping:
		return fmt.Errorf("process '%s' is currently stopping, please wait for stop to complete", name)

	default:
		return fmt.Errorf("invalid state for start: %v", currentState)
	}
}

// doStart performs the actual start operation
func (up *ManagedProcess) doStart(newSpec process.Spec) error {
	up.setState(StateStarting)

	// Update spec and process
	up.mu.Lock()
	//up.spec = newSpec
	up.proc.UpdateSpec(newSpec)
	up.mu.Unlock()

	// Start process (this is the heavy operation, done outside critical sections)
	env := up.envMerger(newSpec)
	cmd := up.proc.ConfigureCmd(env)

	if err := up.proc.TryStart(cmd); err != nil {
		up.setState(StateStopped)
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Enforce start duration if specified
	if newSpec.StartDuration > 0 {
		if err := up.proc.EnforceStartDuration(newSpec.StartDuration); err != nil {
			up.proc.RemovePIDFile()
			up.proc.MarkExited(err)
			up.setState(StateStopped)
			return fmt.Errorf("process exited before start duration: %w", err)
		}
	}

	// Successfully started
	up.setState(StateRunning)

	// Record metrics and persist
	metrics.IncStart(newSpec.Name)
	up.persistStart()

	return nil
}

// handleStop manages stop logic
func (up *ManagedProcess) handleStop(wait time.Duration) error {
	up.mu.RLock()
	currentState := up.state
	up.mu.RUnlock()

	switch currentState {
	case StateStopped:
		return nil // Already stopped

	case StateStarting, StateRunning:
		return up.doStop(wait)

	case StateStopping:
		return fmt.Errorf("process already stopping")

	default:
		return fmt.Errorf("invalid state for stop: %v", currentState)
	}
}

// doStop performs the actual stop operation
func (up *ManagedProcess) doStop(wait time.Duration) error {
	up.setState(StateStopping)

	up.proc.SetStopRequested(true)

	if err := up.proc.Stop(wait); err != nil {
		up.setState(StateStopped) // Force state transition even on error
		up.persistStop()
		return fmt.Errorf("failed to stop process: %w", err)
	}

	up.setState(StateStopped)
	up.persistStop()

	// Record metrics
	metrics.IncStop(up.proc.GetName())

	return nil
}

// handleUpdateSpec updates the process specification
func (up *ManagedProcess) handleUpdateSpec(newSpec process.Spec) error {
	up.mu.Lock()
	up.proc.UpdateSpec(newSpec)
	up.mu.Unlock()

	return nil
}

// handleShutdown performs graceful shutdown
func (up *ManagedProcess) handleShutdown() error {
	err := up.handleStop(3 * time.Second)
	if err != nil && !isExpectedShutdownError(err) {
		return err
	}

	// Clean up resources
	up.mu.Lock()
	if up.proc != nil {
		up.proc.RemovePIDFile()
	}
	up.mu.Unlock()

	return nil
}

func isExpectedShutdownError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for common shutdown signals and patterns
	return errStr == "signal: terminated" ||
		errStr == "signal: killed" ||
		errStr == "signal: interrupt" ||
		errStr == "exit status 1" || // Common exit code
		errStr == "exit status 130" || // Ctrl+C
		errStr == "exit status 143" || // SIGTERM
		// Also handle wrapped errors from stop process
		errStr == "failed to stop process: signal: terminated" ||
		errStr == "failed to stop process: signal: killed" ||
		errStr == "failed to stop process: signal: interrupt"
}

// setState safely updates state (minimal lock scope)
func (up *ManagedProcess) setState(newState processState) {
	up.mu.Lock()
	oldState := up.state
	oldStateStr := oldState.String() // capture string representation while under lock
	up.state = newState
	newStateStr := newState.String() // capture string representation while under lock
	name := up.proc.GetName()        // capture name while under lock
	up.mu.Unlock()

	// Record state transition metrics (outside lock to avoid holding lock too long)
	metrics.RecordStateTransition(name, oldStateStr, newStateStr)

	// Update current state metrics - set old state to 0, new state to 1
	metrics.SetCurrentState(name, oldStateStr, false)
	metrics.SetCurrentState(name, newStateStr, true)
}

// checkProcessHealth monitors process health and transitions state.
func (up *ManagedProcess) checkProcessHealth() {
	up.mu.RLock()
	currentState := up.state
	up.mu.RUnlock()
	if currentState != StateRunning {
		return
	}

	alive, _ := up.proc.DetectAlive()
	if !alive {
		// Process died; transition to stopped and persist stop event.
		up.setState(StateStopped)
		up.persistStop()

		// Auto-restart (if enabled) is handled by the runStateMachine ticker below.
	} else if up.proc.StopRequested() {
		// No-op: stopping is driven explicitly via doStop.
	}
}

// // --- Persistence & History helpers ---
func (up *ManagedProcess) makeRecordLocked() history.Record {
	// Assumes up.mu is at least RLocked (or Locked)
	st := up.proc.Snapshot()
	spec := up.proc.GetSpec()
	b, _ := json.Marshal(spec)
	return history.Record{
		Name:       spec.Name,
		PID:        st.PID,
		LastStatus: up.state.String(),
		UpdatedAt:  time.Now().UTC(),
		SpecJSON:   string(b),
	}
}

func (up *ManagedProcess) persistStart() {
	up.mu.Lock()
	now := time.Now().UTC()
	st := up.proc.Snapshot()
	spec := up.proc.GetSpec()
	sinks := append([]history.Sink(nil), up.history...)
	up.mu.Unlock()

	if len(sinks) > 0 {
		rec := history.Record{Name: spec.Name, PID: st.PID, LastStatus: StateRunning.String(), UpdatedAt: now}
		if b, err := json.Marshal(spec); err == nil {
			rec.SpecJSON = string(b)
		}
		evt := history.Event{Type: history.EventStart, OccurredAt: now, Record: rec}
		for _, h := range sinks {
			_ = h.Send(context.Background(), evt)
		}
	}
}

func (up *ManagedProcess) persistStop() {
	up.mu.RLock()
	now := time.Now().UTC()
	sinks := append([]history.Sink(nil), up.history...)
	st := up.proc.Snapshot()
	spec := up.proc.GetSpec()
	up.mu.RUnlock()

	// Minimal record for event consumers
	rec := history.Record{Name: spec.Name, PID: st.PID, LastStatus: StateStopped.String(), UpdatedAt: now}
	if b, err := json.Marshal(spec); err == nil {
		rec.SpecJSON = string(b)
	}
	ctx := context.Background()
	if len(sinks) > 0 {
		evt := history.Event{Type: history.EventStop, OccurredAt: now, Record: rec}
		for _, h := range sinks {
			_ = h.Send(ctx, evt)
		}
	}
}
