package cronjob

import (
	"github.com/loykin/provisr/core/internal/job"
	"github.com/loykin/provisr/core/observability"
)

// JobRunner is the minimum job-management capability required by scheduling.
type JobRunner interface {
	CreateJob(job.Spec) (*job.Job, error)
	Observe(observability.Event)
}
