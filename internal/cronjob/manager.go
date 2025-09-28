package cronjob

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/metrics"
)

// Manager manages cronjobs
type Manager struct {
	mu             sync.RWMutex
	cronJobs       map[string]*CronJob
	processManager *manager.Manager
}

// NewManager creates a new cronjob manager
func NewManager(processManager *manager.Manager) *Manager {
	return &Manager{
		cronJobs:       make(map[string]*CronJob),
		processManager: processManager,
	}
}

// CreateCronJob creates and starts a new cronjob
func (m *Manager) CreateCronJob(spec CronJobSpec) (*CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate spec
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid cronjob spec: %w", err)
	}

	// Check for duplicate name
	if _, exists := m.cronJobs[spec.Name]; exists {
		return nil, fmt.Errorf("cronjob %q already exists", spec.Name)
	}

	// Create cronjob
	cronJob := NewCronJob(spec, m.processManager)
	m.cronJobs[spec.Name] = cronJob

	// Start cronjob
	if err := cronJob.Start(); err != nil {
		delete(m.cronJobs, spec.Name)
		return nil, fmt.Errorf("failed to start cronjob: %w", err)
	}

	// Update metrics
	metrics.IncCronJobActive(spec.Name)

	slog.Info("CronJob created and started", "name", spec.Name, "schedule", spec.Schedule)
	return cronJob, nil
}

// GetCronJob retrieves a cronjob by name
func (m *Manager) GetCronJob(name string) (*CronJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cronJob, exists := m.cronJobs[name]
	return cronJob, exists
}

// ListCronJobs returns all cronjobs
func (m *Manager) ListCronJobs() map[string]*CronJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cronJobs := make(map[string]*CronJob)
	for name, cronJob := range m.cronJobs {
		cronJobs[name] = cronJob
	}
	return cronJobs
}

// UpdateCronJob updates a cronjob specification
func (m *Manager) UpdateCronJob(name string, spec CronJobSpec) error {
	m.mu.RLock()
	cronJob, exists := m.cronJobs[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("cronjob %q not found", name)
	}

	// Stop the old cronjob
	cronJob.Stop()

	// Create new cronjob with updated spec
	m.mu.Lock()
	newCronJob := NewCronJob(spec, m.processManager)
	m.cronJobs[name] = newCronJob
	m.mu.Unlock()

	// Start the new cronjob
	if err := newCronJob.Start(); err != nil {
		return fmt.Errorf("failed to start updated cronjob: %w", err)
	}

	slog.Info("CronJob updated", "name", name)
	return nil
}

// SuspendCronJob suspends a cronjob
func (m *Manager) SuspendCronJob(name string) error {
	m.mu.RLock()
	cronJob, exists := m.cronJobs[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("cronjob %q not found", name)
	}

	cronJob.Suspend()
	return nil
}

// ResumeCronJob resumes a cronjob
func (m *Manager) ResumeCronJob(name string) error {
	m.mu.RLock()
	cronJob, exists := m.cronJobs[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("cronjob %q not found", name)
	}

	return cronJob.Resume()
}

// DeleteCronJob deletes a cronjob
func (m *Manager) DeleteCronJob(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cronJob, exists := m.cronJobs[name]
	if !exists {
		return fmt.Errorf("cronjob %q not found", name)
	}

	// Stop cronjob
	cronJob.Stop()

	delete(m.cronJobs, name)

	// Update metrics
	metrics.DecCronJobActive(name)

	slog.Info("CronJob deleted", "name", name)
	return nil
}

// GetCronJobsByPattern returns cronjobs matching a pattern
func (m *Manager) GetCronJobsByPattern(pattern string) map[string]*CronJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cronJobs := make(map[string]*CronJob)
	for name, cronJob := range m.cronJobs {
		if matchesPattern(name, pattern) {
			cronJobs[name] = cronJob
		}
	}
	return cronJobs
}

// Shutdown gracefully shuts down the cronjob manager
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []string

	// Stop all cronjobs
	for name, cronJob := range m.cronJobs {
		cronJob.Stop()
		slog.Info("Stopped cronjob during shutdown", "name", name)
	}

	// Clear map
	m.cronJobs = make(map[string]*CronJob)

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errors, "; "))
	}

	slog.Info("CronJob manager shutdown completed")
	return nil
}

// GetCronJobStatus returns status for all cronjobs
func (m *Manager) GetCronJobStatus() map[string]CronJobStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]CronJobStatus)
	for name, cronJob := range m.cronJobs {
		status[name] = cronJob.GetStatus()
	}
	return status
}

// matchesPattern checks if a name matches a simple pattern
func matchesPattern(name, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Simple wildcard matching
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			return strings.HasPrefix(name, prefix)
		}
		if strings.HasPrefix(pattern, "*") {
			suffix := strings.TrimPrefix(pattern, "*")
			return strings.HasSuffix(name, suffix)
		}
	}
	return name == pattern
}
