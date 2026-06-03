package provisr

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	cfg "github.com/loykin/provisr/internal/config"
	"github.com/loykin/provisr/internal/cronjob"
	"github.com/loykin/provisr/internal/detector"
	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/job"
	"github.com/loykin/provisr/internal/logger"
	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
	pg "github.com/loykin/provisr/internal/process_group"
	iapi "github.com/loykin/provisr/internal/server"
	"github.com/prometheus/client_golang/prometheus"
)

// Re-export core types for external consumers.
// These are aliases so conversions are zero-cost.

// Process types
type Spec = process.Spec
type Status = process.Status

// Config types
type Config = cfg.Config
type ServerConfig = cfg.ServerConfig
type TLSConfig = cfg.TLSConfig
type AutoGenTLS = cfg.AutoGenTLS
type ServerAuthConfig = cfg.AuthConfig

// Log config types
type LogConfig = logger.Config
type LogFileConfig = logger.FileConfig
type LogSlogConfig = logger.SlogConfig

// Detector is the interface for custom process readiness / liveness checks.
// Implement it and assign to Spec.Detectors to override default detection.
type Detector = detector.Detector
type CommandDetector = detector.CommandDetector

// Lifecycle types
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

// Manager is a thin facade over internal/process.Manager.
// It provides a stable public API for embedding.

type Manager struct{ inner *manager.Manager }

type ManagerInstanceGroup = manager.InstanceGroup

type HistoryConfig = cfg.HistoryConfig

type HistorySink = history.Sink

type ProcessMetrics = metrics.ProcessMetrics

type ProcessMetricsCollector = metrics.ProcessMetricsCollector

type ProcessMetricsConfig = metrics.ProcessMetricsConfig

func New() *Manager { return &Manager{inner: manager.NewManager()} }

func (m *Manager) SetGlobalEnv(kvs []string)                       { m.inner.SetGlobalEnv(kvs) }
func (m *Manager) SetInstanceGroups(groups []ManagerInstanceGroup) { m.inner.SetInstanceGroups(groups) }
func (m *Manager) Register(s Spec) error                           { return m.inner.Register(s) }
func (m *Manager) RegisterN(s Spec) error                          { return m.inner.RegisterN(s) }
func (m *Manager) Start(name string) error                         { return m.inner.Start(name) }
func (m *Manager) ApplyConfig(specs []Spec) error                  { return m.inner.ApplyConfig(specs) }
func (m *Manager) Stop(name string, wait time.Duration) error {
	return m.inner.Stop(name, wait)
}
func (m *Manager) Unregister(name string, wait time.Duration) error {
	return m.inner.Unregister(name, wait)
}
func (m *Manager) StopAll(base string, wait time.Duration) error { return m.inner.StopAll(base, wait) }
func (m *Manager) UnregisterAll(base string, wait time.Duration) error {
	return m.inner.UnregisterAll(base, wait)
}
func (m *Manager) Status(name string) (Status, error)      { return m.inner.Status(name) }
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

// Process Metrics methods
func (m *Manager) GetProcessMetrics(name string) (metrics.ProcessMetrics, bool) {
	return m.inner.GetProcessMetrics(name)
}
func (m *Manager) GetProcessMetricsHistory(name string) ([]metrics.ProcessMetrics, bool) {
	return m.inner.GetProcessMetricsHistory(name)
}
func (m *Manager) GetAllProcessMetrics() map[string]metrics.ProcessMetrics {
	return m.inner.GetAllProcessMetrics()
}
func (m *Manager) IsProcessMetricsEnabled() bool {
	return m.inner.IsProcessMetricsEnabled()
}
func (m *Manager) SetProcessMetricsCollector(collector *metrics.ProcessMetricsCollector) error {
	return m.inner.SetProcessMetricsCollector(collector)
}

// Group facade
type Group struct{ inner *pg.Group }

type ServiceGroup = pg.ServiceGroup

func NewGroup(m *Manager) *Group { return &Group{inner: pg.New(m.inner)} }

func (g *Group) Start(gs ServiceGroup) error                    { return g.inner.Start(gs) }
func (g *Group) Stop(gs ServiceGroup, wait time.Duration) error { return g.inner.Stop(gs, wait) }
func (g *Group) Status(gs ServiceGroup) (map[string][]Status, error) {
	m, err := g.inner.Status(gs)
	return m, err
}

// Job facade
type JobManager struct{ inner *job.Manager }

type JobSpec = job.Spec        // alias
type JobStatus = job.JobStatus // alias

func NewJobManager(m *Manager) *JobManager {
	return &JobManager{inner: job.NewManager(m.inner)}
}

func (jm *JobManager) CreateJob(spec JobSpec) error { _, err := jm.inner.CreateJob(spec); return err }
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

type CronScheduler struct{ inner *cronjob.Manager }

type CronJob = cronjob.CronJobSpec // alias

func NewCronScheduler(m *Manager) *CronScheduler {
	return &CronScheduler{inner: cronjob.NewManager(m.inner)}
}

func (s *CronScheduler) Add(j CronJob) error { _, err := s.inner.CreateCronJob(j); return err }
func (s *CronScheduler) Start() error        { return nil } // CronJobs start automatically when created
func (s *CronScheduler) Stop() error         { return s.inner.Shutdown() }

func LoadConfig(path string) (*cfg.Config, error) {
	return cfg.LoadConfig(path)
}

// NewHTTPServer starts an HTTP server exposing the internal API using the given manager.
func NewHTTPServer(addr, basePath string, m *Manager) (*http.Server, error) {
	return iapi.NewServer(addr, basePath, m.inner)
}

// NewTLSServer starts an HTTPS server with TLS configuration from server config.
func NewTLSServer(serverConfig ServerConfig, m *Manager) (*http.Server, error) {
	return iapi.NewTLSServer(serverConfig, m.inner)
}

// Router is a thin facade over the internal HTTP router for embedding into
// Gin, Echo, or any net/http-compatible mux.
type Router struct{ inner *iapi.Router }

// NewRouter constructs a Router with the given basePath (e.g. "/api").
func NewRouter(m *Manager, basePath string) *Router {
	return &Router{inner: iapi.NewRouter(m.inner, basePath)}
}

// Handler returns the net/http.Handler for the provisr API.
func (r *Router) Handler() http.Handler { return r.inner.Handler() }

// APIEndpoints provides individual gin.HandlerFunc accessors so callers can
// attach per-route middleware before registering with a Gin router group.
type APIEndpoints struct{ inner *iapi.APIEndpoints }

// NewAPIEndpoints constructs an APIEndpoints facade with the given basePath.
func NewAPIEndpoints(m *Manager, basePath string) *APIEndpoints {
	return &APIEndpoints{inner: iapi.NewAPIEndpoints(m.inner, basePath)}
}

func (e *APIEndpoints) RegisterHandler() gin.HandlerFunc    { return e.inner.RegisterHandler() }
func (e *APIEndpoints) StartHandler() gin.HandlerFunc       { return e.inner.StartHandler() }
func (e *APIEndpoints) StopHandler() gin.HandlerFunc        { return e.inner.StopHandler() }
func (e *APIEndpoints) StatusHandler() gin.HandlerFunc      { return e.inner.StatusHandler() }
func (e *APIEndpoints) UnregisterHandler() gin.HandlerFunc  { return e.inner.UnregisterHandler() }
func (e *APIEndpoints) GroupStartHandler() gin.HandlerFunc  { return e.inner.GroupStartHandler() }
func (e *APIEndpoints) GroupStopHandler() gin.HandlerFunc   { return e.inner.GroupStopHandler() }
func (e *APIEndpoints) GroupStatusHandler() gin.HandlerFunc { return e.inner.GroupStatusHandler() }
func (e *APIEndpoints) DebugProcessesHandler() gin.HandlerFunc {
	return e.inner.DebugProcessesHandler()
}
func (e *APIEndpoints) ProcessMetricsHandler() gin.HandlerFunc {
	return e.inner.ProcessMetricsHandler()
}
func (e *APIEndpoints) ProcessMetricsHistoryHandler() gin.HandlerFunc {
	return e.inner.ProcessMetricsHistoryHandler()
}
func (e *APIEndpoints) ProcessMetricsGroupHandler() gin.HandlerFunc {
	return e.inner.ProcessMetricsGroupHandler()
}
func (e *APIEndpoints) RegisterAll(group *gin.RouterGroup) { e.inner.RegisterAll(group) }

// Metrics helpers (public facade)

func RegisterMetrics(r prometheus.Registerer) error { return metrics.Register(r) }
func RegisterMetricsDefault() error                 { return metrics.Register(prometheus.DefaultRegisterer) }

func RegisterMetricsWithProcessMetricsDefault(processMetricsConfig ProcessMetricsConfig) error {
	return metrics.RegisterWithProcessMetrics(prometheus.DefaultRegisterer, processMetricsConfig)
}

func NewProcessMetricsCollector(config ProcessMetricsConfig) *ProcessMetricsCollector {
	return metrics.NewProcessMetricsCollector(config)
}

// ServeMetrics starts an HTTP server on addr exposing /metrics using the default registry.
// It returns any immediate listen error; otherwise it runs the server in the caller goroutine.
func ServeMetrics(addr string) error {
	http.Handle("/metrics", metrics.Handler())
	srv := &http.Server{
		Addr:              addr,
		Handler:           nil,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv.ListenAndServe()
}
