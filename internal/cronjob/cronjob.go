package cronjob

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/loykin/provisr/internal/job"
	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/robfig/cron/v3"
)

// CronJob represents a scheduled recurring job
type CronJob struct {
	mu      sync.RWMutex
	spec    CronJobSpec
	status  CronJobStatus
	manager *manager.Manager

	// Scheduling state
	ctx         context.Context
	cancel      context.CancelFunc
	scheduler   *cron.Cron
	entryID     cron.EntryID
	isScheduled bool

	// Job management
	activeJobs map[string]*job.Job
	jobHistory []*JobHistoryEntry
}

// JobHistoryEntry represents a completed job in the history
type JobHistoryEntry struct {
	Name           string
	StartTime      time.Time
	CompletionTime *time.Time
	Status         job.JobPhase
	Reason         string
}

// NewCronJob creates a new cronjob instance
func NewCronJob(spec CronJobSpec, mgr *manager.Manager) *CronJob {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply defaults
	spec.GetDefaults()

	// Create scheduler with timezone support
	var scheduler *cron.Cron
	if spec.TimeZone != nil && *spec.TimeZone != "" {
		if loc, err := time.LoadLocation(*spec.TimeZone); err == nil {
			scheduler = cron.New(cron.WithLocation(loc))
		} else {
			slog.Warn("Invalid timezone, using UTC", "timezone", *spec.TimeZone, "error", err)
			scheduler = cron.New()
		}
	} else {
		scheduler = cron.New()
	}

	return &CronJob{
		spec:       spec,
		manager:    mgr,
		ctx:        ctx,
		cancel:     cancel,
		scheduler:  scheduler,
		activeJobs: make(map[string]*job.Job),
		jobHistory: make([]*JobHistoryEntry, 0),
		status: CronJobStatus{
			Active: make([]*job.Reference, 0),
		},
	}
}

// Start starts the cronjob scheduling
func (c *CronJob) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isScheduled {
		return fmt.Errorf("cronjob %s is already scheduled", c.spec.Name)
	}

	if c.spec.Suspend != nil && *c.spec.Suspend {
		slog.Info("CronJob is suspended, not scheduling", "name", c.spec.Name)
		return nil
	}

	// Schedule the job
	entryID, err := c.scheduler.AddFunc(c.spec.Schedule, c.executeJob)
	if err != nil {
		return fmt.Errorf("failed to schedule cronjob %s: %w", c.spec.Name, err)
	}

	c.entryID = entryID
	c.isScheduled = true
	c.scheduler.Start()

	// Update next schedule metric
	nextSchedule := c.GetNextSchedule()
	if !nextSchedule.IsZero() {
		metrics.SetCronJobNextSchedule(c.spec.Name, float64(nextSchedule.Unix()))
	}

	slog.Info("CronJob scheduled", "name", c.spec.Name, "schedule", c.spec.Schedule)
	return nil
}

// Stop stops the cronjob scheduling
func (c *CronJob) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isScheduled {
		return
	}

	c.scheduler.Stop()
	c.isScheduled = false
	c.cancel()

	// Stop all active jobs based on concurrency policy
	for _, j := range c.activeJobs {
		_ = j.Stop()
	}

	slog.Info("CronJob stopped", "name", c.spec.Name)
}

// executeJob is called by the cron scheduler
func (c *CronJob) executeJob() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.status.LastScheduleTime = &now

	// Update metrics
	metrics.SetCronJobLastSchedule(c.spec.Name, float64(now.Unix()))

	// Check if we should skip due to concurrency policy
	if len(c.activeJobs) > 0 {
		switch ConcurrencyPolicy(c.spec.ConcurrencyPolicy) {
		case ConcurrencyPolicyForbid:
			slog.Info("Skipping job execution due to active job", "cronjob", c.spec.Name)
			return
		case ConcurrencyPolicyReplace:
			slog.Info("Replacing active jobs", "cronjob", c.spec.Name)
			for _, j := range c.activeJobs {
				_ = j.Stop()
			}
			c.activeJobs = make(map[string]*job.Job)
			c.status.Active = make([]*job.Reference, 0)
		case ConcurrencyPolicyAllow:
			// Continue with execution
		}
	}

	// Check starting deadline
	if c.spec.StartingDeadlineSeconds != nil {
		deadline := time.Duration(*c.spec.StartingDeadlineSeconds) * time.Second
		if time.Since(now) > deadline {
			slog.Warn("Job start deadline exceeded", "cronjob", c.spec.Name)
			return
		}
	}

	// Create and start the job
	jobName := fmt.Sprintf("%s-%d", c.spec.Name, now.Unix())
	jobSpec := c.spec.JobTemplate
	jobSpec.Name = jobName

	j := job.NewJob(jobSpec, c.manager)

	// Start the job
	if err := j.Start(); err != nil {
		slog.Error("Failed to start job", "cronjob", c.spec.Name, "job", jobName, "error", err)
		c.addToHistory(&JobHistoryEntry{
			Name:      jobName,
			StartTime: now,
			Status:    job.JobPhaseFailed,
			Reason:    fmt.Sprintf("Failed to start: %v", err),
		})

		// Update metrics for failed start
		metrics.IncCronJobTotal(c.spec.Name, string(job.JobPhaseFailed))
		return
	}

	// Update metrics for job start
	metrics.IncCronJobTotal(c.spec.Name, string(job.JobPhaseRunning))

	// Track the active job
	c.activeJobs[jobName] = j
	c.status.Active = append(c.status.Active, &job.Reference{
		Name: jobName,
	})

	// Monitor job completion in a separate goroutine
	go c.monitorJob(jobName, j)

	// Update next schedule metric after job starts
	nextSchedule := c.GetNextSchedule()
	if !nextSchedule.IsZero() {
		metrics.SetCronJobNextSchedule(c.spec.Name, float64(nextSchedule.Unix()))
	}

	slog.Info("Job started", "cronjob", c.spec.Name, "job", jobName)
}

// monitorJob monitors job completion and updates status
func (c *CronJob) monitorJob(jobName string, j *job.Job) {
	// Wait for job completion
	select {
	case <-j.Done():
		// Job completed
	case <-c.ctx.Done():
		// CronJob was stopped
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove from active jobs
	delete(c.activeJobs, jobName)

	// Remove from active status
	for i, ref := range c.status.Active {
		if ref.Name == jobName {
			c.status.Active = append(c.status.Active[:i], c.status.Active[i+1:]...)
			break
		}
	}

	// Get job status and add to history
	status := j.GetStatus()
	completionTime := time.Now()

	var phase job.JobPhase
	var reason string

	if status.Succeeded > 0 {
		phase = job.JobPhaseSucceeded
		reason = "Job completed successfully"
		c.status.LastSuccessfulTime = &completionTime
	} else {
		phase = job.JobPhaseFailed
		reason = "Job failed"
	}

	// Update metrics for job completion
	duration := completionTime.Sub(*status.StartTime).Seconds()
	metrics.IncCronJobTotal(c.spec.Name, string(phase))
	metrics.ObserveCronJobDuration(c.spec.Name, string(phase), duration)

	c.addToHistory(&JobHistoryEntry{
		Name:           jobName,
		StartTime:      *status.StartTime,
		CompletionTime: &completionTime,
		Status:         phase,
		Reason:         reason,
	})

	slog.Info("Job completed", "cronjob", c.spec.Name, "job", jobName, "phase", phase)
}

// addToHistory adds a job to the history and maintains limits
func (c *CronJob) addToHistory(entry *JobHistoryEntry) {
	c.jobHistory = append(c.jobHistory, entry)

	// Maintain history limits
	successfulLimit := int(*c.spec.SuccessfulJobsHistoryLimit)
	failedLimit := int(*c.spec.FailedJobsHistoryLimit)

	var successful, failed []*JobHistoryEntry
	for _, h := range c.jobHistory {
		if h.Status == job.JobPhaseSucceeded {
			successful = append(successful, h)
		} else {
			failed = append(failed, h)
		}
	}

	// Keep only the most recent entries
	if len(successful) > successfulLimit {
		successful = successful[len(successful)-successfulLimit:]
	}
	if len(failed) > failedLimit {
		failed = failed[len(failed)-failedLimit:]
	}

	// Rebuild history
	c.jobHistory = make([]*JobHistoryEntry, 0, len(successful)+len(failed))
	c.jobHistory = append(c.jobHistory, successful...)
	c.jobHistory = append(c.jobHistory, failed...)
}

// GetSpec returns the cronjob specification
func (c *CronJob) GetSpec() CronJobSpec {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.spec
}

// GetStatus returns the cronjob status
func (c *CronJob) GetStatus() CronJobStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetHistory returns the job execution history
func (c *CronJob) GetHistory() []*JobHistoryEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	history := make([]*JobHistoryEntry, len(c.jobHistory))
	copy(history, c.jobHistory)
	return history
}

// IsScheduled returns whether the cronjob is currently scheduled
func (c *CronJob) IsScheduled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isScheduled
}

// GetNextSchedule returns the next scheduled execution time
func (c *CronJob) GetNextSchedule() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.isScheduled {
		entries := c.scheduler.Entries()
		for _, entry := range entries {
			if entry.ID == c.entryID {
				return entry.Next
			}
		}
	}

	return time.Time{}
}

// Suspend pauses the cronjob execution
func (c *CronJob) Suspend() {
	c.mu.Lock()
	defer c.mu.Unlock()

	suspend := true
	c.spec.Suspend = &suspend

	if c.isScheduled {
		c.scheduler.Remove(c.entryID)
		c.isScheduled = false
	}

	slog.Info("CronJob suspended", "name", c.spec.Name)
}

// Resume resumes the cronjob execution
func (c *CronJob) Resume() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	suspend := false
	c.spec.Suspend = &suspend

	if !c.isScheduled {
		return c.Start()
	}

	return nil
}
