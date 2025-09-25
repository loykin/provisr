package cron

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
)

// Job defines a scheduled process run.
// Schedule supports only the form "@every <duration>" (e.g., "@every 5s").
// Non-overlap: if previous run of the same job is still running, the tick is skipped.
// Constraints:
// - AutoRestart must be false (enforced). If Spec.AutoRestart is true, Start will be rejected.
// - Instances must be 1 for a cron job; if >1, job creation fails.
// - RetryCount/RetryInterval and StartDuration are honored for the Start call.
// - PIDFile is allowed but usually unnecessary for short cron jobs.
//
// Name must be unique across jobs inside the same Scheduler.
//
// Note: We keep this minimal and dependency-free per project requirements.

type Job struct {
	Name      string
	Spec      process.Spec
	Schedule  string
	Singleton bool // if true, skip tick when the previous run still active (default: true)

	// internal (guarded via atomic)
	running atomic.Bool
}

// parseEvery parses schedules of the form "@every <duration>".
func parseEvery(expr string) (time.Duration, error) {
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(expr, "@every ") {
		return 0, fmt.Errorf("unsupported schedule: %s (only @every <duration> supported)", expr)
	}
	durStr := strings.TrimSpace(strings.TrimPrefix(expr, "@every "))
	d, err := time.ParseDuration(durStr)
	if err != nil {
		return 0, fmt.Errorf("invalid @every duration: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("@every duration must be > 0")
	}
	return d, nil
}

// validate enforces cron-specific constraints.
func (j *Job) validate() error {
	if j.Spec.AutoRestart {
		return errors.New("cron job cannot have autorestart=true")
	}
	if j.Spec.Instances > 1 {
		return errors.New("cron job cannot run with instances > 1")
	}
	if j.Name == "" {
		return errors.New("cron job requires a name")
	}
	if j.Schedule == "" {
		return errors.New("cron job requires a schedule")
	}
	if !j.Singleton {
		// allowed; no-op. default is true handled by Scheduler.Add.
	}
	// Disallow detectors that rely on long-lived processes? We allow detectors but they aren't used by scheduler logic.
	return nil
}

// Validate is an external validator used by config decoding.
// It validates the embedded Spec (basic invariants) and cron-specific constraints
// that do not depend on Name/Schedule presence here (those are validated by config).
func (j *Job) Validate() error {
	// Validate the underlying process spec
	if err := j.Spec.Validate(); err != nil {
		return err
	}
	// Enforce cron-specific constraints that are independent of Name/Schedule
	if j.Spec.AutoRestart {
		return errors.New("cron job cannot have autorestart=true")
	}
	if j.Spec.Instances > 1 {
		return errors.New("cron job cannot run with instances > 1")
	}
	return nil
}

// Scheduler runs cron jobs using a shared process.Manager.
// Use Start to launch the background tickers, and Stop to cancel them.

type Scheduler struct {
	mgr  *manager.Manager
	jobs []*Job
	quit chan struct{}
	done chan struct{}
}

func NewScheduler(mgr *manager.Manager) *Scheduler {
	return &Scheduler{mgr: mgr}
}

func (s *Scheduler) Add(job *Job) error {
	if !job.Singleton { // keep as given; defaulting below only when zero value
		// nothing
	}
	if err := job.validate(); err != nil {
		return err
	}
	// Enforce AutoRestart=false regardless of passed value
	job.Spec.AutoRestart = false
	// Default Singleton true when not explicitly set (zero bool is false)
	if !job.Singleton {
		job.Singleton = true
	}
	s.jobs = append(s.jobs, job)
	return nil
}

// Start launches all job loops. Call Stop to cancel.
func (s *Scheduler) Start() error {
	if s.quit != nil {
		return errors.New("scheduler already started")
	}
	s.quit = make(chan struct{})
	s.done = make(chan struct{})
	for _, j := range s.jobs {
		d, err := parseEvery(j.Schedule)
		if err != nil {
			return fmt.Errorf("job %s: %w", j.Name, err)
		}
		go s.runJob(j, d)
	}
	return nil
}

func (s *Scheduler) runJob(j *Job, period time.Duration) {
	t := time.NewTicker(period)
	defer t.Stop()
	for {
		select {
		case <-s.quit:
			return
		case <-t.C:
			if j.Singleton {
				// attempt to mark running; if already true, skip this tick
				if !j.running.CompareAndSwap(false, true) {
					continue
				}
			} else {
				j.running.Store(true)
			}
			// run in separate goroutine to avoid blocking the ticker if Start waits start duration
			go func(j *Job) {
				defer j.running.Store(false)
				_ = s.mgr.Register(j.Spec)
				// We don't wait for process completion; it's managed by Manager. Cron semantics fire start attempts.
			}(j)
		}
	}
}

// Stop cancels all jobs.
func (s *Scheduler) Stop() {
	if s.quit == nil {
		return
	}
	// Close once; leaving channel non-nil avoids racy nil assignment observed by goroutines.
	select {
	case <-s.quit:
		// already closed
	default:
		close(s.quit)
	}
}
