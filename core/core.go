// Package core provides the minimal, dependency-light process management API
// for provisr. It exposes a stable public facade over the internal manager,
// process, job, cronjob, and metrics packages without pulling in gin, auth,
// or any database driver.
//
// Applications that want to embed lightweight process control should import
// github.com/loykin/provisr/core. Applications that want the full provisr
// orchestrator (HTTP API, auth, history backends, config loading) should
// import github.com/loykin/provisr.
package core

import (
	"fmt"
	"time"

	"github.com/loykin/provisr/core/history"
	"github.com/loykin/provisr/core/internal/cronjob"
	"github.com/loykin/provisr/core/internal/detector"
	"github.com/loykin/provisr/core/internal/job"
	"github.com/loykin/provisr/core/internal/logger"
	"github.com/loykin/provisr/core/internal/manager"
	"github.com/loykin/provisr/core/internal/process"
	pg "github.com/loykin/provisr/core/internal/process_group"
	"github.com/loykin/provisr/core/observability"
	"github.com/loykin/provisr/core/stats"
)

// --- Process types ---

// Spec is the specification for a managed process.
type Spec = process.Spec

// Status describes the runtime state of a managed process.
type Status = process.Status

// LogLine is a single captured stdout/stderr line, used by the live-tail API.
type LogLine = process.LogLine

// DetectorConfig is a serializable detector definition embedded in a Spec.
type DetectorConfig = process.DetectorConfig

// --- Log config types ---

type LogConfig = logger.Config
type LogFileConfig = logger.FileConfig
type LogSlogConfig = logger.SlogConfig
type LogLevel = logger.LogLevel
type LogFormat = logger.Format

const (
	LogLevelDebug = logger.LevelDebug
	LogLevelInfo  = logger.LevelInfo
	LogLevelWarn  = logger.LevelWarn
	LogLevelError = logger.LevelError

	LogFormatText = logger.FormatText
	LogFormatJSON = logger.FormatJSON
)

// DefaultLogConfig returns the default logger configuration.
func DefaultLogConfig() LogConfig { return logger.DefaultConfig() }

// --- Detector types ---

// Detector is the interface for custom process readiness / liveness checks.
// Implement it and assign to Spec.Detectors to override default detection.
type Detector = detector.Detector
type CommandDetector = detector.CommandDetector
type PIDFileDetector = detector.PIDFileDetector
type PIDDetector = detector.PIDDetector

// --- Lifecycle types ---

type LifecycleHooks = process.LifecycleHooks
type Hook = process.Hook
type FailureMode = process.FailureMode
type RunMode = process.RunMode
type LifecyclePhase = process.LifecyclePhase

const (
	FailureModeIgnore = process.FailureModeIgnore
	FailureModeFail   = process.FailureModeFail
	FailureModeRetry  = process.FailureModeRetry

	RunModeBlocking = process.RunModeBlocking
	RunModeAsync    = process.RunModeAsync
)

// --- History types ---

// HistorySink is the interface implemented by history backends.
// External backends should import github.com/loykin/provisr/core/history.
type HistorySink = history.Sink
type HistoryReader = history.Reader
type HistoryEntry = history.Entry
type HistoryPruner = history.Pruner

// --- Manager facade ---

// ManagerInstanceGroup describes a named group of process instances.
type ManagerInstanceGroup = manager.InstanceGroup

// Manager is a thin facade over the internal manager. It provides a stable
// public API for embedding.
type Manager struct{ inner *manager.Manager }
type Observer = observability.Observer
type ObserverFunc = observability.ObserverFunc
type ObservationEvent = observability.Event

// New constructs a new Manager.
func New() *Manager { return &Manager{inner: manager.NewManager()} }

func (m *Manager) SetHistorySinks(sinks ...HistorySink) { m.inner.SetHistorySinks(sinks...) }
func (m *Manager) SetObservers(observers ...Observer)   { m.inner.SetObservers(observers...) }
func (m *Manager) SetGlobalEnv(kvs []string)            { m.inner.SetGlobalEnv(kvs) }
func (m *Manager) SetInstanceGroups(groups []ManagerInstanceGroup) {
	m.inner.SetInstanceGroups(groups)
}
func (m *Manager) Register(s Spec) error          { return m.inner.Register(s) }
func (m *Manager) RegisterN(s Spec) error         { return m.inner.RegisterN(s) }
func (m *Manager) Start(name string) error        { return m.inner.Start(name) }
func (m *Manager) Recover(s Spec) error           { return m.inner.Recover(s) }
func (m *Manager) ApplyConfig(specs []Spec) error { return m.inner.ApplyConfig(specs) }
func (m *Manager) Stop(name string, wait time.Duration) error {
	return m.inner.Stop(name, wait)
}
func (m *Manager) Update(s Spec, wait time.Duration) error {
	return m.inner.Update(s, wait)
}
func (m *Manager) GetSpec(name string) (Spec, error) {
	return m.inner.GetSpec(name)
}
func (m *Manager) Unregister(name string, wait time.Duration) error {
	return m.inner.Unregister(name, wait)
}
func (m *Manager) StopAll(base string, wait time.Duration) error { return m.inner.StopAll(base, wait) }
func (m *Manager) UnregisterAll(base string, wait time.Duration) error {
	return m.inner.UnregisterAll(base, wait)
}
func (m *Manager) Status(name string) (Status, error) { return m.inner.Status(name) }
func (m *Manager) LogsSince(name string, since uint64, limit int) ([]LogLine, uint64, error) {
	return m.inner.LogsSince(name, since, limit)
}
func (m *Manager) StatusAll(base string) ([]Status, error) { return m.inner.StatusAll(base) }
func (m *Manager) InstanceGroupStatus(groupName string) (map[string][]Status, error) {
	return m.inner.InstanceGroupStatus(groupName)
}
func (m *Manager) InstanceGroupStart(groupName string) error {
	return m.inner.InstanceGroupStart(groupName)
}
func (m *Manager) InstanceGroupStop(groupName string, wait time.Duration) error {
	return m.inner.InstanceGroupStop(groupName, wait)
}
func (m *Manager) Count(base string) (int, error) { return m.inner.Count(base) }

// Shutdown gracefully stops all managed processes and releases resources.
// Call this when the embedding application is shutting down (e.g. on SIGTERM).
func (m *Manager) Shutdown() error { return m.inner.Shutdown() }

// --- Process metrics ---

type ProcessMetrics = stats.ProcessMetrics
type ProcessMetricsCollector = stats.Collector

func (m *Manager) GetProcessMetrics(name string) (ProcessMetrics, bool) {
	return m.inner.GetProcessMetrics(name)
}
func (m *Manager) GetProcessMetricsHistory(name string) ([]ProcessMetrics, bool) {
	return m.inner.GetProcessMetricsHistory(name)
}
func (m *Manager) GetAllProcessMetrics() map[string]ProcessMetrics {
	return m.inner.GetAllProcessMetrics()
}
func (m *Manager) IsProcessMetricsEnabled() bool {
	return m.inner.IsProcessMetricsEnabled()
}
func (m *Manager) SetProcessMetricsCollector(collector ProcessMetricsCollector) error {
	return m.inner.SetProcessMetricsCollector(collector)
}

// --- Group facade ---

type ServiceGroup = pg.ServiceGroup

type Group struct{ inner *pg.Group }

// NewGroup constructs a process group helper bound to the given Manager.
func NewGroup(m *Manager) *Group { return &Group{inner: pg.New(m.inner)} }

func (g *Group) Start(gs ServiceGroup) error                    { return g.inner.Start(gs) }
func (g *Group) Stop(gs ServiceGroup, wait time.Duration) error { return g.inner.Stop(gs, wait) }
func (g *Group) Status(gs ServiceGroup) (map[string][]Status, error) {
	return g.inner.Status(gs)
}

// --- Job facade ---

type JobSpec = job.Spec
type JobStatus = job.JobStatus

type JobManager struct{ inner *job.Manager }

// NewJobManager constructs a JobManager bound to the given Manager.
func NewJobManager(m *Manager) *JobManager {
	return &JobManager{inner: job.NewManager(m.inner)}
}

func (jm *JobManager) CreateJob(spec JobSpec) error {
	_, err := jm.inner.CreateJob(spec)
	return err
}
func (jm *JobManager) GetJob(name string) (JobStatus, bool) {
	j, exists := jm.inner.GetJob(name)
	if !exists {
		return JobStatus{}, false
	}
	return j.GetStatus(), true
}
func (jm *JobManager) GetJobSpec(name string) (JobSpec, bool) {
	j, exists := jm.inner.GetJob(name)
	if !exists {
		return JobSpec{}, false
	}
	return j.GetSpec(), true
}
func (jm *JobManager) ListJobs() map[string]JobStatus { return jm.inner.GetJobStatus() }
func (jm *JobManager) ListJobSpecs() map[string]JobSpec {
	jobs := jm.inner.ListJobs()
	specs := make(map[string]JobSpec, len(jobs))
	for name, j := range jobs {
		specs[name] = j.GetSpec()
	}
	return specs
}
func (jm *JobManager) UpdateJob(name string, spec JobSpec) error {
	return jm.inner.UpdateJob(name, spec)
}
func (jm *JobManager) DeleteJob(name string) error { return jm.inner.DeleteJob(name) }
func (jm *JobManager) Shutdown() error             { return jm.inner.Shutdown() }

// --- CronScheduler facade ---

type CronJob = cronjob.CronJobSpec
type CronJobStatus = cronjob.CronJobStatus
type CronJobHistoryEntry = cronjob.JobHistoryEntry

type CronScheduler struct {
	inner *cronjob.Manager
	jobs  *JobManager
}

func NewCronScheduler(jm *JobManager) *CronScheduler {
	return &CronScheduler{inner: cronjob.NewManager(jm.inner), jobs: jm}
}

func (s *CronScheduler) JobManager() *JobManager {
	return s.jobs
}

func (s *CronScheduler) Add(j CronJob) error { _, err := s.inner.CreateCronJob(j); return err }
func (s *CronScheduler) Start() error        { return nil } // CronJobs start automatically when created
func (s *CronScheduler) Stop() error         { return s.inner.Shutdown() }

// Get returns the spec for a single cronjob, e.g. to prefill an edit form.
func (s *CronScheduler) Get(name string) (CronJob, bool) {
	cj, ok := s.inner.GetCronJob(name)
	if !ok {
		return CronJob{}, false
	}
	return cj.GetSpec(), true
}

// List returns every registered cronjob's spec, keyed by name.
func (s *CronScheduler) List() map[string]CronJob {
	out := make(map[string]CronJob)
	for name, cj := range s.inner.ListCronJobs() {
		out[name] = cj.GetSpec()
	}
	return out
}

// Status returns the current status (active runs, last schedule/success time)
// for a single cronjob.
func (s *CronScheduler) Status(name string) (CronJobStatus, bool) {
	cj, ok := s.inner.GetCronJob(name)
	if !ok {
		return CronJobStatus{}, false
	}
	return cj.GetStatus(), true
}

// NextSchedule returns the next time a cronjob is due to run, or the zero
// time if it isn't currently scheduled (e.g. suspended).
func (s *CronScheduler) NextSchedule(name string) (time.Time, bool) {
	cj, ok := s.inner.GetCronJob(name)
	if !ok {
		return time.Time{}, false
	}
	return cj.GetNextSchedule(), true
}

// History returns the run history (most recent completions, capped by the
// spec's history limits) for a single cronjob.
func (s *CronScheduler) History(name string) ([]CronJobHistoryEntry, bool) {
	cj, ok := s.inner.GetCronJob(name)
	if !ok {
		return nil, false
	}
	entries := cj.GetHistory()
	out := make([]CronJobHistoryEntry, len(entries))
	for i, e := range entries {
		out[i] = *e
	}
	return out, true
}

// Update replaces a cronjob's spec, stopping the old schedule and starting
// the new one (equivalent to Delete+Add under the same name).
func (s *CronScheduler) Update(name string, j CronJob) error {
	return s.inner.UpdateCronJob(name, j)
}

// Suspend pauses a cronjob's schedule without removing it.
func (s *CronScheduler) Suspend(name string) error { return s.inner.SuspendCronJob(name) }

// Resume re-schedules a previously-suspended cronjob.
func (s *CronScheduler) Resume(name string) error { return s.inner.ResumeCronJob(name) }

// Delete stops and removes a cronjob.
func (s *CronScheduler) Delete(name string) error { return s.inner.DeleteCronJob(name) }

// Trigger runs a cronjob's job template immediately, out of band from its
// schedule (subject to the same concurrency policy as a normal firing).
func (s *CronScheduler) Trigger(name string) error {
	cj, ok := s.inner.GetCronJob(name)
	if !ok {
		return fmt.Errorf("cronjob %q not found", name)
	}
	cj.TriggerNow()
	return nil
}
