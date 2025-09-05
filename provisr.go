package provisr

import (
	"net/http"
	"time"

	cfg "github.com/loykin/provisr/internal/config"
	"github.com/loykin/provisr/internal/cron"
	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
	pg "github.com/loykin/provisr/internal/process_group"
	"github.com/prometheus/client_golang/prometheus"
)

// Re-export core types for external consumers.
// These are aliases so conversions are zero-cost.

type Spec = process.Spec

type Status = process.Status

// Manager is a thin facade over internal/process.Manager.
// It provides a stable public API for embedding.

type Manager struct{ inner *manager.Manager }

func New() *Manager { return &Manager{inner: manager.NewManager()} }

func (m *Manager) SetGlobalEnv(kvs []string) { m.inner.SetGlobalEnv(kvs) }

func (m *Manager) Start(s Spec) error  { return m.inner.Start(s) }
func (m *Manager) StartN(s Spec) error { return m.inner.StartN(s) }
func (m *Manager) Stop(name string, wait time.Duration) error {
	return m.inner.Stop(name, wait)
}
func (m *Manager) StopAll(base string, wait time.Duration) error { return m.inner.StopAll(base, wait) }
func (m *Manager) Status(name string) (Status, error)            { return m.inner.Status(name) }
func (m *Manager) StatusAll(base string) ([]Status, error)       { return m.inner.StatusAll(base) }
func (m *Manager) Count(base string) (int, error)                { return m.inner.Count(base) }

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

// Cron facade

type CronScheduler struct{ inner *cron.Scheduler }

type CronJob = cron.Job // alias; use pointer when adding to avoid copying atomics

func NewCronScheduler(m *Manager) *CronScheduler {
	return &CronScheduler{inner: cron.NewScheduler(m.inner)}
}

func (s *CronScheduler) Add(j *CronJob) error { return s.inner.Add(j) }
func (s *CronScheduler) Start() error         { return s.inner.Start() }
func (s *CronScheduler) Stop()                { s.inner.Stop() }

// Config helpers (forwarders to internal/config)

func LoadEnv(path string) ([]string, error)           { return cfg.LoadEnvFromTOML(path) }
func LoadGlobalEnv(path string) ([]string, error)     { return cfg.LoadGlobalEnv(path) }
func LoadSpecs(path string) ([]Spec, error)           { return cfg.LoadSpecsFromTOML(path) }
func LoadGroups(path string) ([]pg.GroupSpec, error)  { return cfg.LoadGroupsFromTOML(path) }
func LoadCronJobs(path string) ([]cfg.CronJob, error) { return cfg.LoadCronJobsFromTOML(path) }

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
