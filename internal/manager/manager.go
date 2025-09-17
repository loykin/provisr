package manager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/loykin/provisr/internal/env"
	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/store"
)

// Manager provides a cleaner interface with reduced complexity
// and clearer locking patterns compared to the old Manager-Handler-Supervisor trinity.
//
// Lock Hierarchy:
// 1. mu (manager-level lock) - protects processes map and shared resources
// 2. Individual ManagedProcess locks - managed internally
//
// This design eliminates the complex three-layer architecture and provides
// better debuggability and performance.
type Manager struct {
	// Manager-level state (protected by mu)
	mu        sync.RWMutex
	processes map[string]*ManagedProcess

	// Shared resources
	envManager *env.Env
	store      store.Store
	histSinks  []history.Sink

	// Reconciler control
	reconTicker *time.Ticker
	reconStop   chan struct{}
	reconWG     sync.WaitGroup
}

// NewManager creates a new manager
func NewManager() *Manager {
	return &Manager{
		processes:  make(map[string]*ManagedProcess),
		envManager: env.New(),
	}
}

// SetGlobalEnv configures global environment variables
func (m *Manager) SetGlobalEnv(kvs []string) {
	newEnv := m.envManager
	for _, kv := range kvs {
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			key := kv[:idx]
			value := kv[idx+1:]
			newEnv = newEnv.WithSet(key, value)
		}
	}

	m.mu.Lock()
	m.envManager = newEnv
	m.mu.Unlock()
}

// SetStore configures persistence store
func (m *Manager) SetStore(s store.Store) error {
	m.mu.Lock()
	m.store = s
	m.mu.Unlock()

	if s == nil {
		return nil
	}
	return s.EnsureSchema(context.Background())
}

// SetHistorySinks configures history sinks
func (m *Manager) SetHistorySinks(sinks ...history.Sink) {
	m.mu.Lock()
	m.histSinks = append([]history.Sink(nil), sinks...)
	m.mu.Unlock()
}

// Start starts a single process
func (m *Manager) Start(spec process.Spec) error {
	up := m.ensureProcess(spec.Name)
	return up.Start(spec)
}

// StartN starts N instances of a process
func (m *Manager) StartN(spec process.Spec) error {
	if spec.Instances <= 1 {
		return m.Start(spec)
	}

	// Start multiple instances with numbered names
	var firstErr error
	for i := 1; i <= spec.Instances; i++ {
		instanceSpec := spec
		instanceSpec.Name = fmt.Sprintf("%s-%d", spec.Name, i)

		if err := m.Start(instanceSpec); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Stop stops a process
func (m *Manager) Stop(name string, wait time.Duration) error {
	m.mu.RLock()
	up := m.processes[name]
	m.mu.RUnlock()

	if up == nil {
		return fmt.Errorf("process %s not found", name)
	}

	return up.Stop(wait)
}

// Status returns status for a single process
func (m *Manager) Status(name string) (process.Status, error) {
	m.mu.RLock()
	up := m.processes[name]
	m.mu.RUnlock()

	if up == nil {
		return process.Status{}, fmt.Errorf("process %s not found", name)
	}

	return up.Status(), nil
}

// StopAll stops all processes matching a base name pattern
func (m *Manager) StopAll(base string, wait time.Duration) error {
	var processes []*ManagedProcess

	m.mu.RLock()
	for name, up := range m.processes {
		if m.matchesPattern(name, base) {
			processes = append(processes, up)
		}
	}
	m.mu.RUnlock()

	var firstErr error
	for _, up := range processes {
		if err := up.Stop(wait); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// StopMatch stops all processes matching a pattern (alias for StopAll)
func (m *Manager) StopMatch(pattern string, wait time.Duration) error {
	return m.StopAll(pattern, wait)
}

// StatusMatch returns status for all processes matching a pattern (alias for StatusAll)
func (m *Manager) StatusMatch(pattern string) ([]process.Status, error) {
	return m.StatusAll(pattern)
}

// StatusAll returns status for all processes matching a pattern
func (m *Manager) StatusAll(base string) ([]process.Status, error) {
	var statuses []process.Status

	m.mu.RLock()
	for name, up := range m.processes {
		if m.matchesPattern(name, base) {
			statuses = append(statuses, up.Status())
		}
	}
	m.mu.RUnlock()

	return statuses, nil
}

// Count returns the number of running instances for a base name
func (m *Manager) Count(base string) (int, error) {
	count := 0

	m.mu.RLock()
	for name, up := range m.processes {
		if m.matchesPattern(name, base) {
			status := up.Status()
			if status.Running {
				count++
			}
		}
	}
	m.mu.RUnlock()

	return count, nil
}

// StartReconciler starts background reconciliation
func (m *Manager) StartReconciler(interval time.Duration) {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	m.mu.Lock()
	if m.reconTicker != nil {
		m.mu.Unlock()
		return // Already running
	}

	m.reconTicker = time.NewTicker(interval)
	m.reconStop = make(chan struct{})
	m.mu.Unlock()

	m.reconWG.Add(1)
	go m.runReconciler()
}

// StopReconciler stops background reconciliation
func (m *Manager) StopReconciler() {
	m.mu.Lock()
	if m.reconTicker == nil {
		m.mu.Unlock()
		return
	}

	m.reconTicker.Stop()
	close(m.reconStop)
	m.reconTicker = nil
	m.reconStop = nil
	m.mu.Unlock()

	m.reconWG.Wait()
}

// ReconcileOnce performs a single reconciliation cycle
func (m *Manager) ReconcileOnce() {
	m.mu.RLock()
	processes := make([]*ManagedProcess, 0, len(m.processes))
	for _, up := range m.processes {
		processes = append(processes, up)
	}
	m.mu.RUnlock()

	// Check each process for health and cleanup
	for _, up := range processes {
		m.reconcileProcess(up)
	}
}

// reconcileProcess performs reconciliation for a single process
func (m *Manager) reconcileProcess(up *ManagedProcess) {
	status := up.Status()

	// Check if process is in an inconsistent state
	if status.Running && status.PID == 0 {
		// Running but no PID - likely stale state
		_ = up.Stop(5 * time.Second)
		return
	}

	// Check if process died unexpectedly and needs cleanup
	if !status.Running && status.PID != 0 {
		// Process died but state not updated - force cleanup
		up.Reconcile()
		return
	}

	// For debugging: could log healthy processes
	// log.Printf("Process %s in state %s is healthy", status.Name, status.State)
} // runReconciler runs the background reconciliation loop
func (m *Manager) runReconciler() {
	defer m.reconWG.Done()

	// Local copies to avoid race conditions
	m.mu.RLock()
	ticker := m.reconTicker
	stopChan := m.reconStop
	m.mu.RUnlock()

	if ticker == nil || stopChan == nil {
		return
	}

	for {
		select {
		case <-ticker.C:
			m.ReconcileOnce()
		case <-stopChan:
			return
		}
	}
}

// Shutdown gracefully shuts down all processes
func (m *Manager) Shutdown() error {
	// Stop reconciler first
	m.StopReconciler()

	// Shut down all processes
	m.mu.RLock()
	processes := make([]*ManagedProcess, 0, len(m.processes))
	for _, up := range m.processes {
		processes = append(processes, up)
	}
	m.mu.RUnlock()

	for _, up := range processes {
		_ = up.Shutdown()
	}

	return nil
}

// ensureProcess gets or creates a ManagedProcess for the given name
func (m *Manager) ensureProcess(name string) *ManagedProcess {
	m.mu.RLock()
	up := m.processes[name]
	m.mu.RUnlock()

	if up != nil {
		return up
	}

	m.mu.Lock()
	// Double-check after acquiring write lock
	up = m.processes[name]
	if up == nil {
		// Create new ManagedProcess with injected dependencies
		spec := process.Spec{Name: name}
		up = NewManagedProcess(
			spec,
			m.mergeEnv,
			m.recordStart,
			m.recordStop,
		)
		m.processes[name] = up
	}
	m.mu.Unlock()

	return up
}

// matchesPattern checks if a name matches a base pattern (supports wildcards)
func (m *Manager) matchesPattern(name, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}

	// Exact match
	if name == pattern {
		return true
	}

	// Simple wildcard matching
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(pattern, "*") {
		// Pattern like "*server*"
		inner := strings.Trim(pattern, "*")
		return strings.Contains(name, inner)
	}

	if strings.HasSuffix(pattern, "*") {
		// Pattern like "web*"
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix)
	}

	if strings.HasPrefix(pattern, "*") {
		// Pattern like "*server"
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(name, suffix)
	}

	return false
}

// mergeEnv merges global and process-specific environment variables
func (m *Manager) mergeEnv(spec process.Spec) []string {
	m.mu.RLock()
	envManager := m.envManager
	m.mu.RUnlock()

	return envManager.Merge(spec.Env)
}

// recordStart records process start events
func (m *Manager) recordStart(_ *process.Process) {
	// This is a stub - simplified for now
	// In a full implementation, this would record to store and history sinks
}

// recordStop records process stop events
func (m *Manager) recordStop(_ *process.Process, _ error) {
	// This is a stub - simplified for now
	// In a full implementation, this would record to store and history sinks
}
