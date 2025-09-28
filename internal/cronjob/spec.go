package cronjob

import (
	"fmt"
	"time"

	"github.com/loykin/provisr/internal/job"
	"github.com/loykin/provisr/internal/process"
	"github.com/robfig/cron/v3"
)

// CronJobSpec defines a recurring job execution (similar to k8s CronJob)
type CronJobSpec struct {
	Name                       string                 `json:"name" mapstructure:"name"`
	Schedule                   string                 `json:"schedule" mapstructure:"schedule"`                                           // Cron expression
	JobTemplate                job.Spec               `json:"job_template" mapstructure:"job_template"`                                   // Template for created jobs
	ConcurrencyPolicy          string                 `json:"concurrency_policy" mapstructure:"concurrency_policy"`                       // "Allow", "Forbid", "Replace"
	Suspend                    *bool                  `json:"suspend" mapstructure:"suspend"`                                             // Pause scheduling
	SuccessfulJobsHistoryLimit *int32                 `json:"successful_jobs_history_limit" mapstructure:"successful_jobs_history_limit"` // Keep successful jobs (default 3)
	FailedJobsHistoryLimit     *int32                 `json:"failed_jobs_history_limit" mapstructure:"failed_jobs_history_limit"`         // Keep failed jobs (default 1)
	StartingDeadlineSeconds    *int64                 `json:"starting_deadline_seconds" mapstructure:"starting_deadline_seconds"`         // Deadline for starting job if missed
	TimeZone                   *string                `json:"time_zone" mapstructure:"time_zone"`                                         // Time zone for cron
	Lifecycle                  process.LifecycleHooks `json:"lifecycle" mapstructure:"lifecycle"`                                         // CronJob-level lifecycle hooks (applied to each scheduled job)
}

// ConcurrencyPolicy defines how to handle concurrent executions
type ConcurrencyPolicy string

const (
	ConcurrencyPolicyAllow   ConcurrencyPolicy = "Allow"   // Allow concurrent executions
	ConcurrencyPolicyForbid  ConcurrencyPolicy = "Forbid"  // Skip execution if previous is still running
	ConcurrencyPolicyReplace ConcurrencyPolicy = "Replace" // Cancel previous and start new
)

// CronJobStatus represents the current status of a cronjob
type CronJobStatus struct {
	Active             []*job.Reference `json:"active,omitempty"`               // List of currently running jobs
	LastScheduleTime   *time.Time       `json:"last_schedule_time,omitempty"`   // Last time job was scheduled
	LastSuccessfulTime *time.Time       `json:"last_successful_time,omitempty"` // Last time job completed successfully
}

// GetDefaults applies default values to the spec
func (s *CronJobSpec) GetDefaults() {
	if s.ConcurrencyPolicy == "" {
		s.ConcurrencyPolicy = string(ConcurrencyPolicyAllow)
	}
	if s.Suspend == nil {
		suspend := false
		s.Suspend = &suspend
	}
	if s.SuccessfulJobsHistoryLimit == nil {
		limit := int32(3)
		s.SuccessfulJobsHistoryLimit = &limit
	}
	if s.FailedJobsHistoryLimit == nil {
		limit := int32(1)
		s.FailedJobsHistoryLimit = &limit
	}

	// Apply defaults to job template
	s.JobTemplate.GetDefaults()
}

// Validate validates the cronjob spec
func (s *CronJobSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("cronjob name is required")
	}
	if s.Schedule == "" {
		return fmt.Errorf("cronjob schedule is required")
	}

	// Validate cron expression
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(s.Schedule); err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", s.Schedule, err)
	}

	// Validate concurrency policy
	switch ConcurrencyPolicy(s.ConcurrencyPolicy) {
	case ConcurrencyPolicyAllow, ConcurrencyPolicyForbid, ConcurrencyPolicyReplace:
		// Valid
	default:
		return fmt.Errorf("invalid concurrency policy %q", s.ConcurrencyPolicy)
	}

	// Validate job template
	if err := s.JobTemplate.Validate(); err != nil {
		return fmt.Errorf("invalid job template: %w", err)
	}

	// Validate cronjob-level lifecycle hooks
	if err := s.Lifecycle.Validate(); err != nil {
		return fmt.Errorf("cronjob %q: lifecycle validation failed: %w", s.Name, err)
	}

	return nil
}

// CreateJobFromTemplate creates a job.Spec from the CronJob template, merging lifecycle hooks
func (s *CronJobSpec) CreateJobFromTemplate(jobName string) job.Spec {
	// Copy the job template
	jobSpec := s.JobTemplate
	jobSpec.Name = jobName

	// Merge lifecycle hooks: CronJob hooks take precedence over JobTemplate hooks
	if s.Lifecycle.HasAnyHooks() {
		// Start with JobTemplate lifecycle hooks
		mergedLifecycle := jobSpec.Lifecycle.DeepCopy()

		// Merge CronJob-level hooks (CronJob hooks take precedence)
		if len(s.Lifecycle.PreStart) > 0 {
			mergedLifecycle.PreStart = append(s.Lifecycle.PreStart, mergedLifecycle.PreStart...)
		}
		if len(s.Lifecycle.PostStart) > 0 {
			mergedLifecycle.PostStart = append(mergedLifecycle.PostStart, s.Lifecycle.PostStart...)
		}
		if len(s.Lifecycle.PreStop) > 0 {
			mergedLifecycle.PreStop = append(s.Lifecycle.PreStop, mergedLifecycle.PreStop...)
		}
		if len(s.Lifecycle.PostStop) > 0 {
			mergedLifecycle.PostStop = append(mergedLifecycle.PostStop, s.Lifecycle.PostStop...)
		}

		jobSpec.Lifecycle = mergedLifecycle
	}

	return jobSpec
}
