package cronjob

import (
	"testing"

	"github.com/loykin/provisr/internal/job"
)

func TestCronJobSpec_BasicCreation(t *testing.T) {
	startingDeadline := int64(100)
	successfulLimit := int32(3)
	failedLimit := int32(1)

	spec := CronJobSpec{
		Name:                       "test-cronjob",
		Schedule:                   "0 */6 * * *", // Every 6 hours
		ConcurrencyPolicy:          string(ConcurrencyPolicyForbid),
		StartingDeadlineSeconds:    &startingDeadline,
		SuccessfulJobsHistoryLimit: &successfulLimit,
		FailedJobsHistoryLimit:     &failedLimit,
		JobTemplate: job.Spec{
			Name:    "test-job-template",
			Command: "echo hello from cronjob",
			WorkDir: "/tmp",
		},
	}

	if spec.Name != "test-cronjob" {
		t.Errorf("Expected name test-cronjob, got %s", spec.Name)
	}
	if spec.Schedule != "0 */6 * * *" {
		t.Errorf("Expected schedule '0 */6 * * *', got %s", spec.Schedule)
	}
	if spec.ConcurrencyPolicy != string(ConcurrencyPolicyForbid) {
		t.Errorf("Expected concurrency policy Forbid, got %s", spec.ConcurrencyPolicy)
	}
	if spec.StartingDeadlineSeconds == nil || *spec.StartingDeadlineSeconds != 100 {
		t.Errorf("Expected starting deadline 100, got %v", spec.StartingDeadlineSeconds)
	}
	if spec.SuccessfulJobsHistoryLimit == nil || *spec.SuccessfulJobsHistoryLimit != 3 {
		t.Errorf("Expected successful jobs history limit 3, got %v", spec.SuccessfulJobsHistoryLimit)
	}
	if spec.FailedJobsHistoryLimit == nil || *spec.FailedJobsHistoryLimit != 1 {
		t.Errorf("Expected failed jobs history limit 1, got %v", spec.FailedJobsHistoryLimit)
	}
	if spec.JobTemplate.Name != "test-job-template" {
		t.Errorf("Expected job template name test-job-template, got %s", spec.JobTemplate.Name)
	}
}

func TestCronJobSpec_MinimalBasicCreation(t *testing.T) {
	spec := CronJobSpec{
		Name:     "minimal-cronjob",
		Schedule: "@daily",
		JobTemplate: job.Spec{
			Name:    "minimal-job",
			Command: "echo daily task",
		},
	}

	if spec.Name != "minimal-cronjob" {
		t.Errorf("Expected name minimal-cronjob, got %s", spec.Name)
	}
	if spec.Schedule != "@daily" {
		t.Errorf("Expected schedule '@daily', got %s", spec.Schedule)
	}
	if spec.ConcurrencyPolicy != "" {
		t.Errorf("Expected empty concurrency policy for minimal spec, got %s", spec.ConcurrencyPolicy)
	}
	if spec.JobTemplate.Command != "echo daily task" {
		t.Errorf("Expected job template command 'echo daily task', got %s", spec.JobTemplate.Command)
	}
}

func TestConcurrencyPolicy_BasicValues(t *testing.T) {
	policies := []ConcurrencyPolicy{
		ConcurrencyPolicyAllow,
		ConcurrencyPolicyForbid,
		ConcurrencyPolicyReplace,
	}

	expected := []string{"Allow", "Forbid", "Replace"}

	for i, policy := range policies {
		if string(policy) != expected[i] {
			t.Errorf("Expected policy %s, got %s", expected[i], string(policy))
		}
	}
}

func TestCronJobSpec_WithDifferentBasicSchedules(t *testing.T) {
	testCases := []struct {
		name     string
		schedule string
	}{
		{"every_minute", "* * * * *"},
		{"hourly", "0 * * * *"},
		{"daily", "0 0 * * *"},
		{"weekly", "0 0 * * 0"},
		{"monthly", "0 0 1 * *"},
		{"every_5_minutes", "*/5 * * * *"},
		{"at_specific_time", "30 14 * * *"}, // 2:30 PM daily
		{"cron_alias_hourly", "@hourly"},
		{"cron_alias_daily", "@daily"},
		{"cron_alias_weekly", "@weekly"},
		{"cron_alias_monthly", "@monthly"},
		{"cron_alias_yearly", "@yearly"},
		{"every_interval", "@every 1h30m"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			spec := CronJobSpec{
				Name:     "test-cronjob",
				Schedule: tc.schedule,
				JobTemplate: job.Spec{
					Name:    "test-job",
					Command: "echo test",
				},
			}

			if spec.Schedule != tc.schedule {
				t.Errorf("Expected schedule %s, got %s", tc.schedule, spec.Schedule)
			}
		})
	}
}

func TestCronJobSpec_WithBasicConcurrencyPolicies(t *testing.T) {
	testCases := []struct {
		name   string
		policy ConcurrencyPolicy
	}{
		{"allow_concurrent", ConcurrencyPolicyAllow},
		{"forbid_concurrent", ConcurrencyPolicyForbid},
		{"replace_concurrent", ConcurrencyPolicyReplace},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			spec := CronJobSpec{
				Name:              "test-cronjob",
				Schedule:          "@daily",
				ConcurrencyPolicy: string(tc.policy),
				JobTemplate: job.Spec{
					Name:    "test-job",
					Command: "echo test",
				},
			}

			if spec.ConcurrencyPolicy != string(tc.policy) {
				t.Errorf("Expected concurrency policy %s, got %s", tc.policy, spec.ConcurrencyPolicy)
			}
		})
	}
}

func TestCronJobSpec_WithComplexBasicJobTemplate(t *testing.T) {
	parallelism := int32(3)
	completions := int32(5)
	backoffLimit := int32(2)
	activeDeadline := int64(300)

	spec := CronJobSpec{
		Name:     "complex-cronjob",
		Schedule: "0 2 * * *", // 2 AM daily
		JobTemplate: job.Spec{
			Name:                  "complex-job",
			Command:               "bash /scripts/backup.sh",
			WorkDir:               "/data",
			Env:                   []string{"BACKUP_TYPE=daily", "RETENTION=7d"},
			Parallelism:           &parallelism,
			Completions:           &completions,
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &activeDeadline,
			CompletionMode:        "NonIndexed",
			RestartPolicy:         "OnFailure",
		},
	}

	jobTemplate := spec.JobTemplate

	if jobTemplate.Command != "bash /scripts/backup.sh" {
		t.Errorf("Expected command 'bash /scripts/backup.sh', got %s", jobTemplate.Command)
	}
	if jobTemplate.WorkDir != "/data" {
		t.Errorf("Expected work dir '/data', got %s", jobTemplate.WorkDir)
	}
	if len(jobTemplate.Env) != 2 {
		t.Errorf("Expected 2 env vars, got %d", len(jobTemplate.Env))
	}
	if *jobTemplate.Parallelism != 3 {
		t.Errorf("Expected parallelism 3, got %d", *jobTemplate.Parallelism)
	}
	if *jobTemplate.Completions != 5 {
		t.Errorf("Expected completions 5, got %d", *jobTemplate.Completions)
	}
	if jobTemplate.CompletionMode != "NonIndexed" {
		t.Errorf("Expected completion mode NonIndexed, got %s", jobTemplate.CompletionMode)
	}
	if jobTemplate.RestartPolicy != "OnFailure" {
		t.Errorf("Expected restart policy OnFailure, got %s", jobTemplate.RestartPolicy)
	}
}
