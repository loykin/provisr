package manager

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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
	// Immutable after creation
	name string

	// State lock (minimal scope, clearly defined)
	mu    sync.RWMutex
	state processState
	spec  process.Spec
	proc  *process.Process

	// Control channels (lock-free communication)
	cmdChan  chan command
	doneChan chan struct{}

	// Atomic counters (lock-free metrics)
	restarts int64

	// Callbacks (injected dependencies)
	envMerger   func(process.Spec) []string
	startLogger func(*process.Process)
	stopLogger  func(*process.Process, error)
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
	startLogger func(*process.Process),
	stopLogger func(*process.Process, error),
) *ManagedProcess {
	up := &ManagedProcess{
		name:        spec.Name,
		state:       StateStopped,
		spec:        spec,
		proc:        process.New(spec),
		cmdChan:     make(chan command, 16), // Buffered to prevent blocking
		doneChan:    make(chan struct{}),
		envMerger:   envMerger,
		startLogger: startLogger,
		stopLogger:  stopLogger,
	}

	go up.runStateMachine()
	return up
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
	state := up.state
	proc := up.proc
	up.mu.RUnlock()

	if proc == nil {
		return process.Status{
			Name:     up.name,
			Running:  false,
			Restarts: int(atomic.LoadInt64(&up.restarts)),
		}
	}

	// Get process status (process handles its own locking)
	status := proc.Snapshot()
	alive, detectedBy := proc.DetectAlive()

	// Ensure name and state are properly set
	status.Name = up.name
	status.Running = alive && state == StateRunning
	status.DetectedBy = detectedBy
	status.Restarts = int(atomic.LoadInt64(&up.restarts))
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

// Reconcile performs health check and cleanup for this process
func (up *ManagedProcess) Reconcile() {
	up.checkProcessHealth()
}

// runStateMachine is the core state machine (single goroutine, no races)
func (up *ManagedProcess) runStateMachine() {
	defer close(up.doneChan)

	ticker := time.NewTicker(100 * time.Millisecond) // Health check interval
	defer ticker.Stop()

	for {
		select {
		case cmd := <-up.cmdChan:
			up.handleCommand(cmd)

		case <-ticker.C:
			up.checkProcessHealth()
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
	name := up.name // capture name while under lock
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
	up.spec = newSpec
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

	// Record metrics and history (lock-free)
	metrics.IncStart(up.name)
	if up.startLogger != nil {
		up.startLogger(up.proc)
	}

	// Start monitoring process exit
	go up.monitorProcessExit()

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
		return fmt.Errorf("failed to stop process: %w", err)
	}

	up.setState(StateStopped)

	// Record metrics
	metrics.IncStop(up.name)
	if up.stopLogger != nil {
		up.stopLogger(up.proc, nil)
	}

	return nil
}

// handleUpdateSpec updates the process specification
func (up *ManagedProcess) handleUpdateSpec(newSpec process.Spec) error {
	up.mu.Lock()
	up.spec = newSpec
	up.proc.UpdateSpec(newSpec)
	up.mu.Unlock()

	return nil
}

// handleShutdown performs graceful shutdown
func (up *ManagedProcess) handleShutdown() error {
	// Stop process if running
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

// isExpectedShutdownError checks if the error is expected during shutdown
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
	name := up.name                  // capture name while under lock
	up.mu.Unlock()

	// Record state transition metrics (outside lock to avoid holding lock too long)
	metrics.RecordStateTransition(name, oldStateStr, newStateStr)

	// Update current state metrics - set old state to 0, new state to 1
	metrics.SetCurrentState(name, oldStateStr, false)
	metrics.SetCurrentState(name, newStateStr, true)
}

// checkProcessHealth monitors process health and handles auto-restart
func (up *ManagedProcess) checkProcessHealth() {
	up.mu.RLock()
	currentState := up.state
	spec := up.spec
	up.mu.RUnlock()

	if currentState != StateRunning {
		return
	}

	alive, _ := up.proc.DetectAlive()
	if !alive {
		// Process died, handle auto-restart
		up.setState(StateStopped)

		if up.stopLogger != nil {
			up.stopLogger(up.proc, fmt.Errorf("process exited unexpectedly"))
		}

		if spec.AutoRestart {
			// Increment restart count
			atomic.AddInt64(&up.restarts, 1)
			metrics.IncRestart(up.name)

			// Wait before restart if specified
			if spec.RestartInterval > 0 {
				time.Sleep(spec.RestartInterval)
			}

			// Attempt restart
			go func() {
				cmd := command{
					action: ActionStart,
					spec:   spec,
					reply:  make(chan error, 1),
				}
				up.cmdChan <- cmd
				<-cmd.reply // Wait for completion (ignore error for auto-restart)
			}()
		}
	}
}

// monitorProcessExit waits for process to exit and updates state
func (up *ManagedProcess) monitorProcessExit() {
	if !up.proc.MonitoringStartIfNeeded() {
		return // Already being monitored
	}

	defer up.proc.MonitoringStop()

	// Wait for process to exit
	cmd := up.proc.CopyCmd()
	if cmd == nil {
		return
	}

	err := cmd.Wait()
	up.proc.CloseWaitDone()
	up.proc.MarkExited(err)
	up.proc.CloseWriters()

	// Transition state if we're still running
	up.mu.RLock()
	currentState := up.state
	up.mu.RUnlock()

	if currentState == StateRunning {
		up.setState(StateStopped)
	}
}
