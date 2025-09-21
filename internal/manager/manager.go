package manager

import (
	"context"
	"encoding/json"
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
}

// NewManager creates a new manager
func NewManager() *Manager {
	return &Manager{
		processes:  make(map[string]*ManagedProcess),
		envManager: env.New(),
	}
}

// NewManagerWithStore creates a new manager wired with the provided store.
// It ensures the store schema and preloads previously managed processes
// from the store, restoring their specs and last known states.
func NewManagerWithStore(s store.Store) (*Manager, error) {
	m := &Manager{
		processes:  make(map[string]*ManagedProcess),
		envManager: env.New(),
		store:      s,
	}
	if s != nil {
		if err := s.EnsureSchema(context.Background()); err != nil {
			return nil, err
		}
		// Preload existing processes from store
		recs, err := s.List(context.Background())
		if err == nil {
			for _, r := range recs {
				var spec process.Spec
				if strings.TrimSpace(r.SpecJSON) != "" {
					_ = json.Unmarshal([]byte(r.SpecJSON), &spec)
				}
				if strings.TrimSpace(spec.Name) == "" {
					spec.Name = r.Name
				}
				up := NewManagedProcess(spec, m.mergeEnv)
				up.SetStore(s)
				if len(m.histSinks) > 0 {
					up.SetHistory(m.histSinks...)
				}
				// Seed PID from store so we can reattach and send signals even without cmd
				if r.PID > 0 {
					up.proc.SeedPID(r.PID)
				}
				// Determine current state by detection to ensure live processes remain monitored
				if alive, _ := up.proc.DetectAlive(); alive {
					up.setState(StateRunning)
				} else if st, ok := parseProcessState(r.LastStatus); ok {
					up.setState(st)
				}
				m.processes[spec.Name] = up
			}
		}
	}
	return m, nil
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
		// Inject shared resources so that persistence/history work immediately
		if m.store != nil {
			up.SetStore(m.store)
		}
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

// parseProcessState maps a string status to processState
func parseProcessState(s string) (processState, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stopped":
		return StateStopped, true
	case "starting":
		return StateStarting, true
	case "running":
		return StateRunning, true
	case "stopping":
		return StateStopping, true
	default:
		return StateStopped, false
	}
}
