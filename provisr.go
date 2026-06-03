// Package provisr is the full orchestrator entry point: it bundles process
// management (re-exported from github.com/loykin/provisr/core) with an HTTP
// API, config loading, auth, and history-backend factory.
//
// Applications that want a lightweight, dependency-light embedding should
// import github.com/loykin/provisr/core directly.
package provisr

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/core"
	cfg "github.com/loykin/provisr/internal/config"
	"github.com/loykin/provisr/internal/history/factory"
	iapi "github.com/loykin/provisr/internal/server"
	"github.com/prometheus/client_golang/prometheus"
)

// --- Re-exports from core ---

// Process types
type Spec = core.Spec
type Status = core.Status
type DetectorConfig = core.DetectorConfig

// Log config types
type LogConfig = core.LogConfig
type LogFileConfig = core.LogFileConfig
type LogSlogConfig = core.LogSlogConfig

// Detector types
type Detector = core.Detector
type CommandDetector = core.CommandDetector
type PIDFileDetector = core.PIDFileDetector
type PIDDetector = core.PIDDetector

// Lifecycle types
type LifecycleHooks = core.LifecycleHooks
type Hook = core.Hook
type FailureMode = core.FailureMode
type RunMode = core.RunMode
type LifecyclePhase = core.LifecyclePhase

const (
	FailureModeIgnore = core.FailureModeIgnore
	FailureModeFail   = core.FailureModeFail
	FailureModeRetry  = core.FailureModeRetry

	RunModeBlocking = core.RunModeBlocking
	RunModeAsync    = core.RunModeAsync

	PhasePreStart  = core.PhasePreStart
	PhasePostStart = core.PhasePostStart
	PhasePreStop   = core.PhasePreStop
	PhasePostStop  = core.PhasePostStop
)

// Manager is the public process manager facade (alias of core.Manager).
type Manager = core.Manager
type ManagerInstanceGroup = core.ManagerInstanceGroup

// HistorySink is the interface for process event backends.
// The built-in factory supports opensearch://, postgres://, postgresql://, and sqlite://.
// For ClickHouse, import github.com/loykin/provisr/history/clickhouse separately.
type HistorySink = core.HistorySink

// Process metrics types
type ProcessMetrics = core.ProcessMetrics
type ProcessMetricsCollector = core.ProcessMetricsCollector
type ProcessMetricsConfig = core.ProcessMetricsConfig

// Group / Job / Cron facades (re-exports)
type Group = core.Group
type ServiceGroup = core.ServiceGroup
type JobManager = core.JobManager
type JobSpec = core.JobSpec
type JobStatus = core.JobStatus
type CronScheduler = core.CronScheduler
type CronJob = core.CronJob

// New constructs a new Manager.
func New() *Manager { return core.New() }

// NewGroup constructs a process group helper bound to the given Manager.
func NewGroup(m *Manager) *Group { return core.NewGroup(m) }

// NewJobManager constructs a JobManager bound to the given Manager.
func NewJobManager(m *Manager) *JobManager { return core.NewJobManager(m) }

// NewCronScheduler constructs a CronScheduler bound to the given Manager.
func NewCronScheduler(m *Manager) *CronScheduler { return core.NewCronScheduler(m) }

// NewProcessMetricsCollector constructs a new process metrics collector.
func NewProcessMetricsCollector(cfg ProcessMetricsConfig) *ProcessMetricsCollector {
	return core.NewProcessMetricsCollector(cfg)
}

// --- Config types (specific to the orchestrator) ---

type Config = cfg.Config
type ServerConfig = cfg.ServerConfig
type TLSConfig = cfg.TLSConfig
type AutoGenTLS = cfg.AutoGenTLS
type ServerAuthConfig = cfg.AuthConfig
type HistoryConfig = cfg.HistoryConfig

// LoadConfig parses a provisr configuration file.
func LoadConfig(path string) (*cfg.Config, error) { return cfg.LoadConfig(path) }

// NewSinkFromDSN creates a HistorySink from a DSN string.
// Supported schemes: opensearch://, elasticsearch://, postgres://, postgresql://, sqlite://.
// For ClickHouse, import github.com/loykin/provisr/history/clickhouse instead.
func NewSinkFromDSN(dsn string) (HistorySink, error) {
	return factory.NewSinkFromDSN(dsn)
}

// --- HTTP server / router facades ---

// NewHTTPServer starts an HTTP server exposing the internal API using the given manager.
func NewHTTPServer(addr, basePath string, m *Manager) (*http.Server, error) {
	return iapi.NewServer(addr, basePath, m)
}

// NewTLSServer starts an HTTPS server with TLS configuration from server config.
func NewTLSServer(serverConfig ServerConfig, m *Manager) (*http.Server, error) {
	return iapi.NewTLSServer(serverConfig, m)
}

// Router is a thin facade over the internal HTTP router for embedding into
// Gin, Echo, or any net/http-compatible mux.
type Router struct{ inner *iapi.Router }

// NewRouter constructs a Router with the given basePath (e.g. "/api").
func NewRouter(m *Manager, basePath string) *Router {
	return &Router{inner: iapi.NewRouter(m, basePath)}
}

// Handler returns the net/http.Handler for the provisr API.
func (r *Router) Handler() http.Handler { return r.inner.Handler() }

// APIEndpoints provides individual gin.HandlerFunc accessors so callers can
// attach per-route middleware before registering with a Gin router group.
type APIEndpoints struct{ inner *iapi.APIEndpoints }

// NewAPIEndpoints constructs an APIEndpoints facade with the given basePath.
func NewAPIEndpoints(m *Manager, basePath string) *APIEndpoints {
	return &APIEndpoints{inner: iapi.NewAPIEndpoints(m, basePath)}
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

// --- Metrics helpers (public facade) ---

func RegisterMetrics(r prometheus.Registerer) error { return core.RegisterMetrics(r) }
func RegisterMetricsDefault() error                 { return core.RegisterMetricsDefault() }

func RegisterMetricsWithProcessMetricsDefault(cfg ProcessMetricsConfig) error {
	return core.RegisterMetricsWithProcessMetricsDefault(cfg)
}

// ServeMetrics starts an HTTP server on addr exposing /metrics using the default registry.
// It returns any immediate listen error; otherwise it runs the server in the caller goroutine.
func ServeMetrics(addr string) error {
	http.Handle("/metrics", core.MetricsHandler())
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
