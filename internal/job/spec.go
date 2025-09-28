package job

import (
	"fmt"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// Spec defines a one-time job execution (similar to k8s Job)
type Spec struct {
	Name                    string   `json:"name" mapstructure:"name"`
	Command                 string   `json:"command" mapstructure:"command"`
	WorkDir                 string   `json:"work_dir" mapstructure:"work_dir"`
	Env                     []string `json:"env" mapstructure:"env"`
	TTLSecondsAfterFinished *int32   `json:"ttl_seconds_after_finished" mapstructure:"ttl_seconds_after_finished"` // Delete job after completion
	ActiveDeadlineSeconds   *int64   `json:"active_deadline_seconds" mapstructure:"active_deadline_seconds"`       // Job timeout
	BackoffLimit            *int32   `json:"backoff_limit" mapstructure:"backoff_limit"`                           // Retry attempts (default 6)
	Parallelism             *int32   `json:"parallelism" mapstructure:"parallelism"`                               // Number of parallel pods (default 1)
	Completions             *int32   `json:"completions" mapstructure:"completions"`                               // Required successful completions (default 1)
	CompletionMode          string   `json:"completion_mode" mapstructure:"completion_mode"`                       // "NonIndexed" or "Indexed"
	RestartPolicy           string   `json:"restart_policy" mapstructure:"restart_policy"`                         // "Never", "OnFailure"
}

// JobStatus represents the current status of a job
type JobStatus struct {
	Phase                   JobPhase                 `json:"phase"`
	StartTime               *time.Time               `json:"start_time,omitempty"`
	CompletionTime          *time.Time               `json:"completion_time,omitempty"`
	Active                  int32                    `json:"active"`    // Number of active pods
	Succeeded               int32                    `json:"succeeded"` // Number of succeeded pods
	Failed                  int32                    `json:"failed"`    // Number of failed pods
	Conditions              []Condition              `json:"conditions,omitempty"`
	UncountedTerminatedPods *UncountedTerminatedPods `json:"uncounted_terminated_pods,omitempty"`
}

// JobPhase represents the phase of job execution
type JobPhase string

const (
	JobPhasePending   JobPhase = "Pending"
	JobPhaseRunning   JobPhase = "Running"
	JobPhaseSucceeded JobPhase = "Succeeded"
	JobPhaseFailed    JobPhase = "Failed"
)

// JobConditionType represents the type of job condition
type ConditionType string

const (
	ConditionComplete ConditionType = "Complete"
	ConditionFailed   ConditionType = "Failed"
)

// JobCondition describes current condition of a job
type Condition struct {
	Type               ConditionType `json:"type"`
	Status             string        `json:"status"` // True, False, Unknown
	LastProbeTime      *time.Time    `json:"last_probe_time,omitempty"`
	LastTransitionTime *time.Time    `json:"last_transition_time,omitempty"`
	Reason             string        `json:"reason,omitempty"`
	Message            string        `json:"message,omitempty"`
}

// UncountedTerminatedPods holds UIDs of terminated pods that haven't been added to job status counters
type UncountedTerminatedPods struct {
	Succeeded []string `json:"succeeded,omitempty"`
	Failed    []string `json:"failed,omitempty"`
}

// Reference represents a reference to a job
type Reference struct {
	Name string `json:"name"`
	UID  string `json:"uid,omitempty"`
}

// RestartPolicy defines restart behavior for failed jobs
type RestartPolicy string

const (
	RestartPolicyNever     RestartPolicy = "Never"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
)

// CompletionMode defines completion behavior
type CompletionMode string

const (
	CompletionModeNonIndexed CompletionMode = "NonIndexed"
	CompletionModeIndexed    CompletionMode = "Indexed"
)

// Validate enforces JobSpec invariants
func (j *Spec) Validate() error {
	if strings.TrimSpace(j.Name) == "" {
		return fmt.Errorf("job requires name")
	}
	if strings.TrimSpace(j.Command) == "" {
		return fmt.Errorf("job %q requires command", j.Name)
	}

	// Validate restart policy
	if j.RestartPolicy != "" && j.RestartPolicy != string(RestartPolicyNever) && j.RestartPolicy != string(RestartPolicyOnFailure) {
		return fmt.Errorf("job %q: invalid restart_policy %q, must be 'Never' or 'OnFailure'", j.Name, j.RestartPolicy)
	}

	// Validate completion mode
	if j.CompletionMode != "" && j.CompletionMode != string(CompletionModeNonIndexed) && j.CompletionMode != string(CompletionModeIndexed) {
		return fmt.Errorf("job %q: invalid completion_mode %q, must be 'NonIndexed' or 'Indexed'", j.Name, j.CompletionMode)
	}

	// Validate numeric fields
	if j.BackoffLimit != nil && *j.BackoffLimit < 0 {
		return fmt.Errorf("job %q: backoff_limit cannot be negative", j.Name)
	}
	if j.Parallelism != nil && *j.Parallelism <= 0 {
		return fmt.Errorf("job %q: parallelism must be greater than 0", j.Name)
	}
	if j.Completions != nil && *j.Completions <= 0 {
		return fmt.Errorf("job %q: completions must be greater than 0", j.Name)
	}
	if j.ActiveDeadlineSeconds != nil && *j.ActiveDeadlineSeconds <= 0 {
		return fmt.Errorf("job %q: active_deadline_seconds must be greater than 0", j.Name)
	}
	if j.TTLSecondsAfterFinished != nil && *j.TTLSecondsAfterFinished < 0 {
		return fmt.Errorf("job %q: ttl_seconds_after_finished cannot be negative", j.Name)
	}

	return nil
}

// ToProcessSpec converts a JobSpec to a process.Spec for execution
func (j *Spec) ToProcessSpec() *process.Spec {
	spec := &process.Spec{
		Name:        j.Name,
		Command:     j.Command,
		WorkDir:     j.WorkDir,
		Env:         append([]string(nil), j.Env...), // Copy slice
		AutoRestart: false,                           // Jobs don't auto-restart by default
	}

	// Configure restart policy
	if j.RestartPolicy == string(RestartPolicyOnFailure) {
		spec.AutoRestart = true
		if j.BackoffLimit != nil {
			spec.RetryCount = uint32(*j.BackoffLimit)
		} else {
			spec.RetryCount = 6 // Default k8s backoff limit
		}
	}

	// Configure parallelism as instances
	if j.Parallelism != nil {
		spec.Instances = int(*j.Parallelism)
	} else {
		spec.Instances = 1
	}

	return spec
}

// GetDefaults returns default values for JobSpec fields
func (j *Spec) GetDefaults() {
	if j.BackoffLimit == nil {
		backoff := int32(6)
		j.BackoffLimit = &backoff
	}
	if j.Parallelism == nil {
		parallelism := int32(1)
		j.Parallelism = &parallelism
	}
	if j.Completions == nil {
		completions := int32(1)
		j.Completions = &completions
	}
	if j.RestartPolicy == "" {
		j.RestartPolicy = string(RestartPolicyNever)
	}
	if j.CompletionMode == "" {
		j.CompletionMode = string(CompletionModeNonIndexed)
	}
}
