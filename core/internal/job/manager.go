package job

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/loykin/provisr/core/observability"
)

// Manager manages jobs
type Manager struct {
	mu             sync.RWMutex
	jobs           map[string]*Job
	processManager ProcessRunner
}

// NewManager creates a new job manager
func NewManager(processManager ProcessRunner) *Manager {
	return &Manager{
		jobs:           make(map[string]*Job),
		processManager: processManager,
	}
}

// CreateJob creates and starts a new job.
// If DependsOn is set, the job enters Pending state and starts only after all
// listed jobs succeed. If any dependency fails, this job is marked Failed.
func (m *Manager) CreateJob(spec Spec) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid job spec: %w", err)
	}

	if _, exists := m.jobs[spec.Name]; exists {
		return nil, fmt.Errorf("job %q already exists", spec.Name)
	}

	j := NewJob(spec, m.processManager)
	m.jobs[spec.Name] = j

	if len(spec.DependsOn) > 0 {
		// Collect dep references while holding the lock to avoid TOCTOU.
		depJobs := make([]*Job, 0, len(spec.DependsOn))
		for _, depName := range spec.DependsOn {
			dep, exists := m.jobs[depName]
			if !exists {
				delete(m.jobs, spec.Name)
				return nil, fmt.Errorf("job %q: dependency %q not found", spec.Name, depName)
			}
			depJobs = append(depJobs, dep)
		}
		slog.Info("Job created, waiting for dependencies", "name", spec.Name, "deps", spec.DependsOn)
		go watchDependencies(j, depJobs)
	} else {
		if err := j.Start(); err != nil {
			delete(m.jobs, spec.Name)
			return nil, fmt.Errorf("failed to start job: %w", err)
		}
		slog.Info("Job created and started", "name", spec.Name)
	}

	m.processManager.Observe(observability.Event{Kind: observability.JobStarted, Name: spec.Name, Phase: string(JobPhaseRunning)})

	return j, nil
}

// watchDependencies waits for all deps to finish, then starts j or marks it failed.
func watchDependencies(j *Job, deps []*Job) {
	for _, dep := range deps {
		<-dep.Done()
	}
	for _, dep := range deps {
		if dep.GetStatus().Phase != JobPhaseSucceeded {
			depName := dep.GetSpec().Name
			depPhase := dep.GetStatus().Phase
			j.FailWithReason("DependencyFailed",
				fmt.Sprintf("dependency %q did not succeed (phase: %s)", depName, depPhase))
			slog.Warn("Job blocked by failed dependency", "job", j.GetSpec().Name, "dep", depName, "phase", depPhase)
			return
		}
	}
	spec := j.GetSpec()
	slog.Info("All dependencies satisfied, starting job", "job", spec.Name)
	if err := j.Start(); err != nil {
		j.FailWithReason("StartFailed", fmt.Sprintf("failed to start after deps: %v", err))
	}
}

// GetJob retrieves a job by name
func (m *Manager) GetJob(name string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, exists := m.jobs[name]
	return job, exists
}

// ListJobs returns all jobs
func (m *Manager) ListJobs() map[string]*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make(map[string]*Job)
	for name, job := range m.jobs {
		jobs[name] = job
	}
	return jobs
}

// UpdateJob updates a job (stops the old one and starts a new one)
func (m *Manager) UpdateJob(name string, spec Spec) error {
	if spec.Name != name {
		return fmt.Errorf("job name cannot be changed from %q to %q", name, spec.Name)
	}
	if err := spec.Validate(); err != nil {
		return fmt.Errorf("invalid job spec: %w", err)
	}

	m.mu.RLock()
	old, exists := m.jobs[name]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("job %q not found", name)
	}
	oldSpec := old.GetSpec()
	for _, depName := range spec.DependsOn {
		if _, ok := m.jobs[depName]; !ok {
			m.mu.RUnlock()
			return fmt.Errorf("job %q: dependency %q not found", spec.Name, depName)
		}
	}
	m.mu.RUnlock()

	// Delete existing job
	if err := m.DeleteJob(name); err != nil {
		return fmt.Errorf("failed to delete existing job: %w", err)
	}

	// Create new job with updated spec
	if _, err := m.CreateJob(spec); err != nil {
		if _, restoreErr := m.CreateJob(oldSpec); restoreErr != nil {
			return fmt.Errorf("failed to create updated job: %v; failed to restore original job: %w", err, restoreErr)
		}
		return fmt.Errorf("failed to create updated job; original job restored: %w", err)
	}
	return nil
}

// DeleteJob deletes a job
func (m *Manager) DeleteJob(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[name]
	if !exists {
		return fmt.Errorf("job %q not found", name)
	}

	// Stop and cleanup job
	if err := job.Stop(); err != nil {
		slog.Warn("Failed to stop job during deletion", "name", name, "error", err)
	}

	if err := job.Cleanup(); err != nil {
		slog.Warn("Failed to cleanup job during deletion", "name", name, "error", err)
	}

	delete(m.jobs, name)

	// Update metrics
	m.processManager.Observe(observability.Event{Kind: observability.JobDeleted, Name: name})

	slog.Info("Job deleted", "name", name)
	return nil
}

func (m *Manager) Observe(event observability.Event) { m.processManager.Observe(event) }

// CleanupCompletedJobs removes completed jobs based on their TTL
func (m *Manager) CleanupCompletedJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var toDelete []string
	for name, job := range m.jobs {
		status := job.GetStatus()

		// Skip if job is still running
		if status.Phase == JobPhaseRunning || status.Phase == JobPhasePending {
			continue
		}

		// Check TTL
		spec := job.GetSpec()
		if spec.TTLSecondsAfterFinished != nil && status.CompletionTime != nil {
			ttl := time.Duration(*spec.TTLSecondsAfterFinished) * time.Second
			if time.Since(*status.CompletionTime) > ttl {
				toDelete = append(toDelete, name)
			}
		}
	}

	// Delete expired jobs
	for _, name := range toDelete {
		job := m.jobs[name]
		if err := job.Cleanup(); err != nil {
			slog.Warn("Failed to cleanup expired job", "name", name, "error", err)
		}
		delete(m.jobs, name)
		slog.Info("Cleaned up expired job", "name", name)
	}
}

// GetJobsByPattern returns jobs matching a pattern
func (m *Manager) GetJobsByPattern(pattern string) map[string]*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make(map[string]*Job)
	for name, job := range m.jobs {
		if matchesPattern(name, pattern) {
			jobs[name] = job
		}
	}
	return jobs
}

// StartCleanupWorker starts a background worker to clean up completed jobs
func (m *Manager) StartCleanupWorker() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			m.CleanupCompletedJobs()
		}
	}()
}

// Shutdown gracefully shuts down the job manager
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []string

	// Stop all jobs
	for name, job := range m.jobs {
		if err := job.Stop(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to stop job %s: %v", name, err))
		}
		if err := job.Cleanup(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to cleanup job %s: %v", name, err))
		}
	}

	// Clear maps
	m.jobs = make(map[string]*Job)

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errors, "; "))
	}

	slog.Info("Job manager shutdown completed")
	return nil
}

// GetJobStatus returns status for all jobs
func (m *Manager) GetJobStatus() map[string]JobStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]JobStatus)
	for name, job := range m.jobs {
		status[name] = job.GetStatus()
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
