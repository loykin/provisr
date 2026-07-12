package manager

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/loykin/provisr/core/history"
	"github.com/loykin/provisr/core/internal/env"
	"github.com/loykin/provisr/core/internal/process"
	"github.com/loykin/provisr/core/observability"
	"github.com/loykin/provisr/core/stats"
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
	groups    map[string]InstanceGroup

	// Shared resources
	envManager       *env.Env
	histSinks        []history.Sink
	metricsCollector stats.Collector
	metricsCtx       context.Context
	metricsCancel    context.CancelFunc
	emitter          *observability.Emitter
}

// NewManager creates a new manager
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		processes:     make(map[string]*ManagedProcess),
		groups:        make(map[string]InstanceGroup),
		envManager:    env.New(),
		metricsCtx:    ctx,
		metricsCancel: cancel,
		emitter:       observability.NewEmitter(),
	}
}

func (m *Manager) SetObservers(observers ...observability.Observer) {
	m.emitter.SetObservers(observers...)
}

func (m *Manager) Observe(event observability.Event) { m.emitter.Emit(event) }

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

// SetProcessMetricsCollector configures the process metrics collector
func (m *Manager) SetProcessMetricsCollector(collector stats.Collector) error {
	m.mu.Lock()
	m.metricsCollector = collector
	m.mu.Unlock()

	if collector != nil && collector.IsEnabled() {
		return collector.Start(m.metricsCtx, m.getProcessPIDs)
	}
	return nil
}

// getProcessPIDs returns a map of process names to PIDs for metrics collection
func (m *Manager) getProcessPIDs() map[string]int32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int32)
	for name, mp := range m.processes {
		status := mp.Status()
		if status.Running && status.PID > 0 {
			// Ensure PID fits in int32 range before conversion
			if status.PID <= 2147483647 {
				result[name] = int32(status.PID)
			}
		}
	}
	return result
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

// GetSpec returns the currently-registered spec for name, e.g. so a caller
// can prefill an edit form before calling Update.
func (m *Manager) GetSpec(name string) (process.Spec, error) {
	m.mu.RLock()
	up := m.processes[name]
	m.mu.RUnlock()

	if up == nil {
		return process.Spec{}, fmt.Errorf("process %s not found", name)
	}

	up.mu.RLock()
	proc := up.proc
	up.mu.RUnlock()

	if proc == nil {
		return process.Spec{}, fmt.Errorf("process %q has no process instance", name)
	}

	spec := proc.GetSpec()
	if spec == nil {
		return process.Spec{}, fmt.Errorf("process %q has no spec defined", name)
	}
	return *spec, nil
}

// Recover reads spec.PIDFile, marks the process Running if the recorded PID is
// still alive, or Stopped if it is dead. The process is never restarted.
// Call this once at startup to re-attach to processes that survived a provisr
// restart without triggering unwanted restarts of already-dead processes.
func (m *Manager) Recover(spec process.Spec) error {
	up := m.ensureProcess(spec.Name)

	if spec.PIDFile != "" {
		pid, specFromFile, err := process.VerifyPIDFile(spec.PIDFile)
		if err != nil {
			return fmt.Errorf("recover %q: reading PID file: %w", spec.Name, err)
		}
		if pid > 0 {
			s := specFromFile
			if s == nil {
				s = &spec
			}
			s.Name = spec.Name
			up.Recover(*s, pid)
			return nil
		}
	}

	// No PID file, content invalid, or PID identity mismatch — register as stopped.
	up.Recover(spec, 0)
	return nil
}

// Update replaces the spec of an already-registered process and immediately
// restarts it under the new spec (stop, then start). The process must already
// be registered; use Register/RegisterN to create a new one.
func (m *Manager) Update(spec process.Spec, wait time.Duration) error {
	m.mu.RLock()
	up := m.processes[spec.Name]
	m.mu.RUnlock()

	if up == nil {
		return fmt.Errorf("process %s not found", spec.Name)
	}

	if err := up.Stop(wait); err != nil {
		return fmt.Errorf("update %q: stop failed: %w", spec.Name, err)
	}

	return up.Start(spec)
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

// LogsSince returns captured stdout/stderr lines for name since the given
// offset, plus the offset to pass as `since` on the next poll.
func (m *Manager) LogsSince(name string, since uint64, limit int) ([]process.LogLine, uint64, error) {
	m.mu.RLock()
	up := m.processes[name]
	m.mu.RUnlock()

	if up == nil {
		return nil, since, fmt.Errorf("process %s not found", name)
	}

	lines, next := up.LogsSince(since, limit)
	return lines, next, nil
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

// StatusAll returns status for all processes matching a pattern, sorted by
// name. Go's map iteration order is randomized per call, so without this
// sort the same query would return processes in a different order every
// time — visible as rows shuffling position on every poll in the UI.
func (m *Manager) StatusAll(base string) ([]process.Status, error) {
	statuses := make([]process.Status, 0) // Initialize as empty slice instead of nil

	m.mu.RLock()
	for name, up := range m.processes {
		if m.matchesPattern(name, base) {
			statuses = append(statuses, up.Status())
		}
	}
	m.mu.RUnlock()

	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })

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
	// Stop metrics collection first
	if m.metricsCancel != nil {
		m.metricsCancel()
	}

	m.mu.RLock()
	collector := m.metricsCollector
	m.mu.RUnlock()

	if collector != nil {
		collector.Stop()
	}

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
			m.emitter,
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
			// VerifyPIDFile performs identity verification (start-time check).
			// Missing or invalid content means there is no process to recover.
			// I/O errors must abort to avoid starting a duplicate process when
			// the existing PID file cannot be inspected.
			pid, specFromFile, err := process.VerifyPIDFile(ds.PIDFile)
			if err != nil {
				return fmt.Errorf("apply config %q: reading PID file: %w", name, err)
			}
			if pid > 0 {
				// Prefer spec from PID file if available (preserve historical details)
				if specFromFile != nil {
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

// InstanceGroup defines a group of processes to be managed together
// where each member can have multiple instances (e.g., web-1, web-2, web-3)
type InstanceGroup struct {
	Name    string
	Members []process.Spec
}

// SetInstanceGroups configures the instance group definitions
func (m *Manager) SetInstanceGroups(groups []InstanceGroup) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing groups
	m.groups = make(map[string]InstanceGroup)

	// Set new groups
	for _, group := range groups {
		m.groups[group.Name] = group
	}
}

// GetInstanceGroup returns the instance group specification by name
func (m *Manager) GetInstanceGroup(name string) (InstanceGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[name]
	if !exists {
		return InstanceGroup{}, fmt.Errorf("instance group %s not found", name)
	}
	return group, nil
}

// InstanceGroupStatus returns status of all processes in an instance group
func (m *Manager) InstanceGroupStatus(groupName string) (map[string][]process.Status, error) {
	group, err := m.GetInstanceGroup(groupName)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]process.Status)

	for _, member := range group.Members {
		// Get all instances of this member (e.g., web-1, web-2)
		statuses, err := m.StatusAll(member.Name)
		if err != nil {
			// If no processes found, add empty entry
			result[member.Name] = []process.Status{}
		} else {
			result[member.Name] = statuses
		}
	}

	return result, nil
}

// InstanceGroupStart starts all processes in an instance group
func (m *Manager) InstanceGroupStart(groupName string) error {
	group, err := m.GetInstanceGroup(groupName)
	if err != nil {
		return err
	}

	var firstError error
	for _, member := range group.Members {
		// Start all instances of this member
		for i := 1; i <= member.Instances; i++ {
			instanceName := member.Name
			if member.Instances > 1 {
				instanceName = fmt.Sprintf("%s-%d", member.Name, i)
			}
			if err := m.Start(instanceName); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("failed to start %s: %w", instanceName, err)
				}
				// Continue starting other processes even if one fails
			}
		}
	}

	return firstError
}

// InstanceGroupStop stops all processes in an instance group
func (m *Manager) InstanceGroupStop(groupName string, wait time.Duration) error {
	group, err := m.GetInstanceGroup(groupName)
	if err != nil {
		return err
	}

	var firstError error
	for _, member := range group.Members {
		// Stop all instances of this member base
		if err := m.StopAll(member.Name, wait); err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("failed to stop %s: %w", member.Name, err)
			}
			// Continue stopping other processes even if one fails
		}
	}

	return firstError
}

// GetProcessMetrics returns the latest metrics for a specific process
func (m *Manager) GetProcessMetrics(name string) (stats.ProcessMetrics, bool) {
	m.mu.RLock()
	collector := m.metricsCollector
	m.mu.RUnlock()

	if collector == nil {
		return stats.ProcessMetrics{}, false
	}

	return collector.GetMetrics(name)
}

// GetProcessMetricsHistory returns the historical metrics for a specific process
func (m *Manager) GetProcessMetricsHistory(name string) ([]stats.ProcessMetrics, bool) {
	m.mu.RLock()
	collector := m.metricsCollector
	m.mu.RUnlock()

	if collector == nil {
		return nil, false
	}

	return collector.GetHistory(name)
}

// GetAllProcessMetrics returns the latest metrics for all processes
func (m *Manager) GetAllProcessMetrics() map[string]stats.ProcessMetrics {
	m.mu.RLock()
	collector := m.metricsCollector
	m.mu.RUnlock()

	if collector == nil {
		return make(map[string]stats.ProcessMetrics)
	}

	return collector.GetAllMetrics()
}

// IsProcessMetricsEnabled returns whether process metrics collection is enabled
func (m *Manager) IsProcessMetricsEnabled() bool {
	m.mu.RLock()
	collector := m.metricsCollector
	m.mu.RUnlock()

	return collector != nil && collector.IsEnabled()
}
