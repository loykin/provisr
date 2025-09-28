package job

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/loykin/provisr/internal/manager"
)

// Job represents a running job instance
type Job struct {
	mu      sync.RWMutex
	spec    Spec
	status  JobStatus
	manager *manager.Manager

	// Job execution state
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
	processes map[string]*JobProcess // process name -> JobProcess

	// Completion tracking
	completionCh chan struct{}
	errorCh      chan error
	completed    bool
}

// JobProcess represents a process instance within a job
type JobProcess struct {
	Name      string
	Index     int32 // For indexed completion mode
	Status    JobProcessStatus
	StartTime time.Time
	EndTime   *time.Time
	ExitCode  *int
	Error     error
}

// JobProcessStatus represents the status of a job process
type JobProcessStatus string

const (
	JobProcessStatusPending   JobProcessStatus = "Pending"
	JobProcessStatusRunning   JobProcessStatus = "Running"
	JobProcessStatusSucceeded JobProcessStatus = "Succeeded"
	JobProcessStatusFailed    JobProcessStatus = "Failed"
)

// NewJob creates a new job instance
func NewJob(spec Spec, mgr *manager.Manager) *Job {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply defaults
	spec.GetDefaults()

	job := &Job{
		spec:         spec,
		manager:      mgr,
		ctx:          ctx,
		cancel:       cancel,
		processes:    make(map[string]*JobProcess),
		completionCh: make(chan struct{}),
		errorCh:      make(chan error, 10),
		status: JobStatus{
			Phase:      JobPhasePending,
			Conditions: []Condition{},
		},
	}

	return job
}

// Start begins job execution
func (j *Job) Start() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.status.Phase != JobPhasePending {
		return fmt.Errorf("job %q is already started", j.spec.Name)
	}

	j.startTime = time.Now()
	j.status.StartTime = &j.startTime
	j.status.Phase = JobPhaseRunning

	// Apply active deadline if specified
	if j.spec.ActiveDeadlineSeconds != nil {
		deadline := time.Duration(*j.spec.ActiveDeadlineSeconds) * time.Second
		go j.enforceDeadline(deadline)
	}

	// Start job processes based on parallelism
	parallelism := int32(1)
	if j.spec.Parallelism != nil {
		parallelism = *j.spec.Parallelism
	}

	slog.Info("Starting job", "name", j.spec.Name, "parallelism", parallelism)

	// Create and start processes
	for i := int32(0); i < parallelism; i++ {
		processName := j.generateProcessName(i)
		jobProcess := &JobProcess{
			Name:      processName,
			Index:     i,
			Status:    JobProcessStatusPending,
			StartTime: time.Now(),
		}
		j.processes[processName] = jobProcess

		go j.runProcess(jobProcess)
	}

	// Start monitoring goroutine
	go j.monitor()

	j.addCondition(ConditionComplete, "False", "JobStarted", "Job has started execution")

	return nil
}

// Stop stops the job execution
func (j *Job) Stop() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.completed {
		return nil
	}

	slog.Info("Stopping job", "name", j.spec.Name)
	j.cancel()

	// Stop all processes
	for processName := range j.processes {
		if err := j.manager.Stop(processName, 5*time.Second); err != nil {
			slog.Warn("Failed to stop job process", "job", j.spec.Name, "process", processName, "error", err)
		}
	}

	j.setFailed("JobStopped", "Job was manually stopped")
	return nil
}

// Wait waits for job completion
func (j *Job) Wait() error {
	select {
	case <-j.completionCh:
		return nil
	case err := <-j.errorCh:
		return err
	case <-j.ctx.Done():
		return j.ctx.Err()
	}
}

// GetStatus returns the current job status
func (j *Job) GetStatus() JobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

// Done returns a channel that is closed when the job completes
func (j *Job) Done() <-chan struct{} {
	return j.completionCh
}

// GetSpec returns the job specification
func (j *Job) GetSpec() Spec {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.spec
}

// runProcess runs a single process instance for the job
func (j *Job) runProcess(jobProcess *JobProcess) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Job process panicked", "job", j.spec.Name, "process", jobProcess.Name, "panic", r)
			j.handleProcessFailure(jobProcess, fmt.Errorf("process panicked: %v", r))
		}
	}()

	// Convert job spec to process spec
	processSpec := j.spec.ToProcessSpec()
	processSpec.Name = jobProcess.Name

	// Add job-specific environment variables
	processSpec.Env = append(processSpec.Env,
		fmt.Sprintf("JOB_NAME=%s", j.spec.Name),
		fmt.Sprintf("JOB_COMPLETION_INDEX=%d", jobProcess.Index),
	)

	// Register process with manager
	if err := j.manager.RegisterN(*processSpec); err != nil {
		slog.Error("Failed to register job process", "job", j.spec.Name, "process", jobProcess.Name, "error", err)
		j.handleProcessFailure(jobProcess, err)
		return
	}

	// Start process
	if err := j.manager.Start(jobProcess.Name); err != nil {
		slog.Error("Failed to start job process", "job", j.spec.Name, "process", jobProcess.Name, "error", err)
		j.handleProcessFailure(jobProcess, err)
		return
	}

	j.mu.Lock()
	jobProcess.Status = JobProcessStatusRunning
	j.status.Active++
	j.mu.Unlock()

	slog.Info("Job process started", "job", j.spec.Name, "process", jobProcess.Name, "index", jobProcess.Index)

	// Monitor process until completion
	for {
		select {
		case <-j.ctx.Done():
			return
		case <-time.After(1 * time.Second):
			status, err := j.manager.Status(jobProcess.Name)
			if err != nil {
				slog.Debug("Failed to get process status", "process", jobProcess.Name, "error", err)
				continue
			}

			if !status.Running {
				// Process completed
				endTime := time.Now()
				j.mu.Lock()
				jobProcess.EndTime = &endTime
				j.status.Active--

				// Extract exit code from ExitErr if available
				exitCode := 0
				if status.ExitErr != nil {
					exitCode = 1 // Default non-zero exit code for errors
				}

				if exitCode == 0 {
					jobProcess.Status = JobProcessStatusSucceeded
					j.status.Succeeded++
					slog.Info("Job process succeeded", "job", j.spec.Name, "process", jobProcess.Name, "index", jobProcess.Index)
				} else {
					jobProcess.Status = JobProcessStatusFailed
					jobProcess.ExitCode = &exitCode
					j.status.Failed++
					slog.Warn("Job process failed", "job", j.spec.Name, "process", jobProcess.Name, "index", jobProcess.Index, "exitCode", exitCode)
				}
				j.mu.Unlock()

				// Clean up process from manager
				_ = j.manager.Unregister(jobProcess.Name, 5*time.Second)
				return
			}
		}
	}
}

// monitor monitors job completion and handles termination conditions
func (j *Job) monitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-j.ctx.Done():
			return
		case <-ticker.C:
			if j.checkCompletion() {
				return
			}
		}
	}
}

// checkCompletion checks if the job has completed and updates status accordingly
func (j *Job) checkCompletion() bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.completed {
		return true
	}

	completions := int32(1)
	if j.spec.Completions != nil {
		completions = *j.spec.Completions
	}

	backoffLimit := int32(6)
	if j.spec.BackoffLimit != nil {
		backoffLimit = *j.spec.BackoffLimit
	}

	// Check if job succeeded
	if j.status.Succeeded >= completions {
		j.setSucceeded()
		return true
	}

	// Check if job failed (exceeded backoff limit)
	if j.status.Failed > backoffLimit {
		j.setFailed("BackoffLimitExceeded", fmt.Sprintf("Job has reached the specified backoff limit %d", backoffLimit))
		return true
	}

	// Check if we need to start more processes
	totalActive := j.status.Active
	totalNeeded := completions - j.status.Succeeded

	parallelism := int32(1)
	if j.spec.Parallelism != nil {
		parallelism = *j.spec.Parallelism
	}

	// Start more processes if needed and not exceeding parallelism
	if totalActive < parallelism && totalActive < totalNeeded && j.status.Failed <= backoffLimit {
		j.startAdditionalProcess()
	}

	return false
}

// startAdditionalProcess starts an additional process for the job
func (j *Job) startAdditionalProcess() {
	nextIndex := int32(len(j.processes))
	processName := j.generateProcessName(nextIndex)

	jobProcess := &JobProcess{
		Name:      processName,
		Index:     nextIndex,
		Status:    JobProcessStatusPending,
		StartTime: time.Now(),
	}
	j.processes[processName] = jobProcess

	go j.runProcess(jobProcess)
}

// setSucceeded marks the job as succeeded
func (j *Job) setSucceeded() {
	if j.completed {
		return
	}

	j.completed = true
	completionTime := time.Now()
	j.status.CompletionTime = &completionTime
	j.status.Phase = JobPhaseSucceeded

	j.addCondition(ConditionComplete, "True", "JobCompleted", "Job completed successfully")

	slog.Info("Job completed successfully", "name", j.spec.Name, "duration", completionTime.Sub(j.startTime))

	j.cancel()
	close(j.completionCh)

	// Schedule TTL cleanup if specified
	if j.spec.TTLSecondsAfterFinished != nil {
		go j.scheduleTTLCleanup()
	}
}

// setFailed marks the job as failed
func (j *Job) setFailed(reason, message string) {
	if j.completed {
		return
	}

	j.completed = true
	completionTime := time.Now()
	j.status.CompletionTime = &completionTime
	j.status.Phase = JobPhaseFailed

	j.addCondition(ConditionFailed, "True", reason, message)

	slog.Error("Job failed", "name", j.spec.Name, "reason", reason, "message", message, "duration", completionTime.Sub(j.startTime))

	j.cancel()
	j.errorCh <- fmt.Errorf("job failed: %s", message)
	close(j.completionCh)

	// Schedule TTL cleanup if specified
	if j.spec.TTLSecondsAfterFinished != nil {
		go j.scheduleTTLCleanup()
	}
}

// handleProcessFailure handles failure of a job process
func (j *Job) handleProcessFailure(jobProcess *JobProcess, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	jobProcess.Status = JobProcessStatusFailed
	jobProcess.Error = err
	if jobProcess.EndTime == nil {
		endTime := time.Now()
		jobProcess.EndTime = &endTime
	}

	j.status.Active--
	j.status.Failed++
}

// enforceDeadline enforces the active deadline for the job
func (j *Job) enforceDeadline(deadline time.Duration) {
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	select {
	case <-timer.C:
		j.mu.Lock()
		if !j.completed {
			j.setFailed("DeadlineExceeded", fmt.Sprintf("Job was active longer than specified deadline %s", deadline))
		}
		j.mu.Unlock()
	case <-j.ctx.Done():
		return
	}
}

// scheduleTTLCleanup schedules cleanup of the job after TTL expires
func (j *Job) scheduleTTLCleanup() {
	if j.spec.TTLSecondsAfterFinished == nil {
		return
	}

	ttl := time.Duration(*j.spec.TTLSecondsAfterFinished) * time.Second
	timer := time.NewTimer(ttl)
	defer timer.Stop()

	select {
	case <-timer.C:
		slog.Info("Cleaning up job due to TTL expiration", "name", j.spec.Name, "ttl", ttl)
		// The job manager should handle the actual cleanup
	case <-j.ctx.Done():
		return
	}
}

// addCondition adds a condition to the job status
func (j *Job) addCondition(conditionType ConditionType, status, reason, message string) {
	now := time.Now()
	condition := Condition{
		Type:               conditionType,
		Status:             status,
		LastProbeTime:      &now,
		LastTransitionTime: &now,
		Reason:             reason,
		Message:            message,
	}

	// Remove existing condition of the same type
	var newConditions []Condition
	for _, c := range j.status.Conditions {
		if c.Type != conditionType {
			newConditions = append(newConditions, c)
		}
	}
	newConditions = append(newConditions, condition)
	j.status.Conditions = newConditions
}

// generateProcessName generates a unique process name for the job
func (j *Job) generateProcessName(index int32) string {
	if j.spec.CompletionMode == string(CompletionModeIndexed) {
		return fmt.Sprintf("%s-%d", j.spec.Name, index)
	}
	return fmt.Sprintf("%s-%d", j.spec.Name, time.Now().UnixNano())
}

// Cleanup cleans up job resources
func (j *Job) Cleanup() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.cancel()

	// Remove all processes from manager
	var errors []string
	for processName := range j.processes {
		if err := j.manager.Unregister(processName, 5*time.Second); err != nil {
			errors = append(errors, fmt.Sprintf("failed to unregister process %s: %v", processName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}
