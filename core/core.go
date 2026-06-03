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
	"net/http"
	"time"

	"github.com/loykin/provisr/core/history"
	"github.com/loykin/provisr/core/internal/cronjob"
	"github.com/loykin/provisr/core/internal/detector"
	"github.com/loykin/provisr/core/internal/job"
	"github.com/loykin/provisr/core/internal/logger"
	"github.com/loykin/provisr/core/internal/manager"
	"github.com/loykin/provisr/core/internal/metrics"
	"github.com/loykin/provisr/core/internal/process"
	pg "github.com/loykin/provisr/core/internal/process_group"
	"github.com/prometheus/client_golang/prometheus"
)

// --- Process types ---

// Spec is the specification for a managed process.
type Spec = process.Spec

// Status describes the runtime state of a managed process.
type Status = process.Status

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

	PhasePreStart  = process.PhasePreStart
	PhasePostStart = process.PhasePostStart
	PhasePreStop   = process.PhasePreStop
	PhasePostStop  = process.PhasePostStop
)

// --- History types ---

// HistorySink is the interface implemented by history backends.
// External backends should import github.com/loykin/provisr/core/history.
type HistorySink = history.Sink

// --- Manager facade ---

// ManagerInstanceGroup describes a named group of process instances.
type ManagerInstanceGroup = manager.InstanceGroup

// Manager is a thin facade over the internal manager. It provides a stable
// public API for embedding.
type Manager struct{ inner *manager.Manager }

// New constructs a new Manager.
func New() *Manager { return &Manager{inner: manager.NewManager()} }

func (m *Manager) SetHistorySinks(sinks ...HistorySink) { m.inner.SetHistorySinks(sinks...) }
func (m *Manager) SetGlobalEnv(kvs []string)            { m.inner.SetGlobalEnv(kvs) }
func (m *Manager) SetInstanceGroups(groups []ManagerInstanceGroup) {
	m.inner.SetInstanceGroups(groups)
}
func (m *Manager) Register(s Spec) error          { return m.inner.Register(s) }
func (m *Manager) RegisterN(s Spec) error         { return m.inner.RegisterN(s) }
func (m *Manager) Start(name string) error        { return m.inner.Start(name) }
func (m *Manager) ApplyConfig(specs []Spec) error { return m.inner.ApplyConfig(specs) }
func (m *Manager) Stop(name string, wait time.Duration) error {
	return m.inner.Stop(name, wait)
}
func (m *Manager) StopMatch(pattern string, wait time.Duration) error {
	return m.inner.StopMatch(pattern, wait)
}
func (m *Manager) Unregister(name string, wait time.Duration) error {
	return m.inner.Unregister(name, wait)
}
func (m *Manager) UnregisterMatch(pattern string, wait time.Duration) error {
	return m.inner.UnregisterMatch(pattern, wait)
}
func (m *Manager) StopAll(base string, wait time.Duration) error { return m.inner.StopAll(base, wait) }
func (m *Manager) UnregisterAll(base string, wait time.Duration) error {
	return m.inner.UnregisterAll(base, wait)
}
func (m *Manager) Status(name string) (Status, error)              { return m.inner.Status(name) }
func (m *Manager) StatusAll(base string) ([]Status, error)         { return m.inner.StatusAll(base) }
func (m *Manager) StatusMatch(pattern string) ([]Status, error)    { return m.inner.StatusMatch(pattern) }
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

type ProcessMetrics = metrics.ProcessMetrics
type ProcessMetricsCollector = metrics.ProcessMetricsCollector
type ProcessMetricsConfig = metrics.ProcessMetricsConfig

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
func (m *Manager) SetProcessMetricsCollector(collector *ProcessMetricsCollector) error {
	return m.inner.SetProcessMetricsCollector(collector)
}

// NewProcessMetricsCollector constructs a new collector for process resource metrics.
func NewProcessMetricsCollector(config ProcessMetricsConfig) *ProcessMetricsCollector {
	return metrics.NewProcessMetricsCollector(config)
}

// RegisterMetrics registers the provisr Prometheus metrics with the given registerer.
func RegisterMetrics(r prometheus.Registerer) error { return metrics.Register(r) }

// RegisterMetricsDefault registers metrics with the default Prometheus registry.
func RegisterMetricsDefault() error { return metrics.Register(prometheus.DefaultRegisterer) }

// RegisterMetricsWithProcessMetricsDefault registers metrics including the
// process metrics collector against the default Prometheus registry.
func RegisterMetricsWithProcessMetricsDefault(cfg ProcessMetricsConfig) error {
	return metrics.RegisterWithProcessMetrics(prometheus.DefaultRegisterer, cfg)
}

// MetricsHandler returns an http.Handler for /metrics using the default registry.
func MetricsHandler() http.Handler { return metrics.Handler() }

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
func (jm *JobManager) ListJobs() map[string]JobStatus { return jm.inner.GetJobStatus() }
func (jm *JobManager) UpdateJob(name string, spec JobSpec) error {
	return jm.inner.UpdateJob(name, spec)
}
func (jm *JobManager) DeleteJob(name string) error { return jm.inner.DeleteJob(name) }
func (jm *JobManager) Shutdown() error             { return jm.inner.Shutdown() }

// --- CronScheduler facade ---

type CronJob = cronjob.CronJobSpec

type CronScheduler struct{ inner *cronjob.Manager }

// NewCronScheduler constructs a CronScheduler bound to the given Manager.
func NewCronScheduler(m *Manager) *CronScheduler {
	return &CronScheduler{inner: cronjob.NewManager(m.inner)}
}

func (s *CronScheduler) Add(j CronJob) error { _, err := s.inner.CreateCronJob(j); return err }
func (s *CronScheduler) Start() error        { return nil } // CronJobs start automatically when created
func (s *CronScheduler) Stop() error         { return s.inner.Shutdown() }
