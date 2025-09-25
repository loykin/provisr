package provisr

import (
	"net/http"
	"time"

	cfg "github.com/loykin/provisr/internal/config"
	"github.com/loykin/provisr/internal/cron"
	"github.com/loykin/provisr/internal/history"
	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
	pg "github.com/loykin/provisr/internal/process_group"
	iapi "github.com/loykin/provisr/internal/server"
	"github.com/prometheus/client_golang/prometheus"
)

// Re-export core types for external consumers.
// These are aliases so conversions are zero-cost.

type Spec = process.Spec

type Status = process.Status

// Manager is a thin facade over internal/process.Manager.
// It provides a stable public API for embedding.

type Manager struct{ inner *manager.Manager }

type HistoryConfig = cfg.HistoryConfig

type HistorySink = history.Sink

func New() *Manager { return &Manager{inner: manager.NewManager()} }

func (m *Manager) SetGlobalEnv(kvs []string)      { m.inner.SetGlobalEnv(kvs) }
func (m *Manager) Register(s Spec) error          { return m.inner.Register(s) }
func (m *Manager) RegisterN(s Spec) error         { return m.inner.RegisterN(s) }
func (m *Manager) Start(name string) error        { return m.inner.Start(name) }
func (m *Manager) ApplyConfig(specs []Spec) error { return m.inner.ApplyConfig(specs) }
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
func (m *Manager) Count(base string) (int, error)          { return m.inner.Count(base) }

// Group facade
type Group struct{ inner *pg.Group }

type GroupSpec = pg.GroupSpec

func NewGroup(m *Manager) *Group { return &Group{inner: pg.New(m.inner)} }

func (g *Group) Start(gs GroupSpec) error                    { return g.inner.Start(gs) }
func (g *Group) Stop(gs GroupSpec, wait time.Duration) error { return g.inner.Stop(gs, wait) }
func (g *Group) Status(gs GroupSpec) (map[string][]Status, error) {
	m, err := g.inner.Status(gs)
	return m, err
}

type CronScheduler struct{ inner *cron.Scheduler }

type CronJob = cron.Job // alias; use pointer when adding to avoid copying atomics

func NewCronScheduler(m *Manager) *CronScheduler {
	return &CronScheduler{inner: cron.NewScheduler(m.inner)}
}

func (s *CronScheduler) Add(j *CronJob) error { return s.inner.Add(j) }
func (s *CronScheduler) Start() error         { return s.inner.Start() }
func (s *CronScheduler) Stop()                { s.inner.Stop() }

func LoadConfig(path string) (*cfg.Config, error) {
	return cfg.LoadConfig(path)
} // NewHTTPServer starts an HTTP server exposing the internal API using the given manager.
func NewHTTPServer(addr, basePath string, m *Manager) (*http.Server, error) {
	return iapi.NewServer(addr, basePath, m.inner)
}

// Metrics helpers (public facade)

func RegisterMetrics(r prometheus.Registerer) error { return metrics.Register(r) }
func RegisterMetricsDefault() error                 { return metrics.Register(prometheus.DefaultRegisterer) }

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
