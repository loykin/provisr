package manager

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/loykin/provisr/internal/env"
	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/process"
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
	histSinks  []history.Sink
}

// NewManager creates a new manager
func NewManager() *Manager {
	return &Manager{
		processes:  make(map[string]*ManagedProcess),
		envManager: env.New(),
	}
}

// NewManagerWithStore has been removed. Use NewManager() and provide specs via Start/StartN as needed.

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

// SetStore removed: persistence via store is no longer supported.

// SetHistorySinks configures history sinks
func (m *Manager) SetHistorySinks(sinks ...history.Sink) {
	m.mu.Lock()
	m.histSinks = append([]history.Sink(nil), sinks...)
	m.mu.Unlock()
}

// Register registers and starts a new process
func (m *Manager) Register(spec process.Spec) error {
	up := m.ensureProcess(spec.Name)
	return up.Start(spec)
}

// RegisterN registers and starts N instances of a process
func (m *Manager) RegisterN(spec process.Spec) error {
	if spec.Instances <= 1 {
		return m.Register(spec)
	}

	// Register multiple instances with numbered names
	var firstErr error
	for i := 1; i <= spec.Instances; i++ {
		instanceSpec := spec
		instanceSpec.Name = fmt.Sprintf("%s-%d", spec.Name, i)

		if err := m.Register(instanceSpec); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Start starts an already registered process without creating a new one
func (m *Manager) Start(name string) error {
	m.mu.RLock()
	up := m.processes[name]
	m.mu.RUnlock()

	if up == nil {
		return fmt.Errorf("process %q is not registered", name)
	}

	// Get current spec from the managed process
	up.mu.RLock()
	proc := up.proc
	up.mu.RUnlock()

	if proc == nil {
		return fmt.Errorf("process %q has no process instance", name)
	}

	spec := proc.GetSpec()
	if spec == nil {
		return fmt.Errorf("process %q has no spec defined", name)
	}

	return up.Start(*spec)
}

// Stop stops a process without unregistering it
func (m *Manager) Stop(name string, wait time.Duration) error {
	m.mu.RLock()
	up := m.processes[name]
	m.mu.RUnlock()

	if up == nil {
		return fmt.Errorf("process %s not found", name)
	}

	return up.Stop(wait)
}

// Unregister stops and removes a process from management
func (m *Manager) Unregister(name string, wait time.Duration) error {
	m.mu.Lock()
	up := m.processes[name]
	if up == nil {
		m.mu.Unlock()
		return fmt.Errorf("process %s not found", name)
	}

	// Remove from processes map immediately to prevent new operations
	delete(m.processes, name)
	m.mu.Unlock()

	// Stop the process
	if err := up.Stop(wait); err != nil {
		return err
	}

	return nil
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

// UnregisterAll stops and unregisters all processes matching a base name pattern
func (m *Manager) UnregisterAll(base string, wait time.Duration) error {
	var processes []*ManagedProcess
	var names []string

	m.mu.Lock()
	for name, up := range m.processes {
		if m.matchesPattern(name, base) {
			processes = append(processes, up)
			names = append(names, name)
		}
	}

	// Remove all matched processes from map first
	for _, name := range names {
		delete(m.processes, name)
	}
	m.mu.Unlock()

	// Stop all processes
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

// UnregisterMatch stops and unregisters all processes matching a pattern (alias for UnregisterAll)
func (m *Manager) UnregisterMatch(pattern string, wait time.Duration) error {
	return m.UnregisterAll(pattern, wait)
}

// StatusMatch returns status for all processes matching a pattern (alias for StatusAll)
func (m *Manager) StatusMatch(pattern string) ([]process.Status, error) {
	return m.StatusAll(pattern)
}

// StatusAll returns status for all processes matching a pattern
func (m *Manager) StatusAll(base string) ([]process.Status, error) {
	statuses := make([]process.Status, 0) // Initialize as empty slice instead of nil

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

// Shutdown gracefully shuts down all processes
func (m *Manager) Shutdown() error {
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
		)
		// Inject shared history sinks so that events work immediately
		if len(m.histSinks) > 0 {
			up.SetHistory(m.histSinks...)
		}
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

	// Base name matching: "batch" matches "batch-1", "batch-2", etc.
	// This is for supporting instances created by StartN
	if strings.HasPrefix(name, pattern+"-") {
		return true
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

// ApplyConfig loads processes from PID files and reconciles running processes with the given specs.
// Behavior:
// 1) For each desired spec (expanding Instances), if a PID file is present and alive, recover it.
// 2) Otherwise, start the process from the spec.
// 3) Any managed process whose name is not present in the desired set will be gracefully shut down and cleaned up.
func (m *Manager) ApplyConfig(specs []process.Spec) error {
	// Build desired instances map: name -> instance spec
	desired := make(map[string]process.Spec)
	for _, s := range specs {
		if s.Instances <= 1 {
			ds := s
			ds.Name = s.Name
			desired[ds.Name] = ds
			continue
		}
		for i := 1; i <= s.Instances; i++ {
			ds := s
			ds.Name = fmt.Sprintf("%s-%d", s.Name, i)
			desired[ds.Name] = ds
		}
	}

	// First, ensure desired processes are running or recovered from PID files
	for name, ds := range desired {
		up := m.ensureProcess(name)

		// Try recover from PID file if configured
		if ds.PIDFile != "" {
			if pid, specFromFile, err := process.ReadPIDFile(ds.PIDFile); err == nil && pid > 0 {
				// Prefer spec from PID file if available (preserve historical details)
				if specFromFile != nil {
					// ensure instance-expanded name is set
					specFromFile.Name = name
					up.Recover(*specFromFile, pid)
				} else {
					ds.Name = name
					up.Recover(ds, pid)
				}
				// After recover, if alive state was false, we'll fall through to start
			}
		}

		// Check current status; if not running, register and start it
		st := up.Status()
		if !st.Running {
			_ = up.Start(ds)
		}
	}

	// Then, stop and cleanup processes that are no longer desired
	m.mu.RLock()
	existing := make(map[string]*ManagedProcess, len(m.processes))
	for n, up := range m.processes {
		existing[n] = up
	}
	m.mu.RUnlock()

	for name, up := range existing {
		if _, ok := desired[name]; !ok {
			_ = up.Shutdown()
			// Remove from map
			m.mu.Lock()
			delete(m.processes, name)
			m.mu.Unlock()
		}
	}

	return nil
}
