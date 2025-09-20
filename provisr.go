package provisr

import (
	"net/http"
	"sort"
	"strings"
	"time"

	cfg "github.com/loykin/provisr/internal/config"
	"github.com/loykin/provisr/internal/cron"
	"github.com/loykin/provisr/internal/history"
	history_factory "github.com/loykin/provisr/internal/history/factory"
	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
	pg "github.com/loykin/provisr/internal/process_group"
	iapi "github.com/loykin/provisr/internal/server"
	storfactory "github.com/loykin/provisr/internal/store/factory"
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

func (m *Manager) SetGlobalEnv(kvs []string) { m.inner.SetGlobalEnv(kvs) }

// SetStoreFromDSN Store controls
func (m *Manager) SetStoreFromDSN(dsn string) error {
	s, err := storfactory.NewFromDSN(dsn)
	if err != nil {
		return err
	}
	return m.inner.SetStore(s)
}
func (m *Manager) DisableStore() { _ = m.inner.SetStore(nil) }

func (m *Manager) SetHistorySinks(sinks ...HistorySink) { m.inner.SetHistorySinks(sinks...) }

func (m *Manager) Start(s Spec) error  { return m.inner.Start(s) }
func (m *Manager) StartN(s Spec) error { return m.inner.StartN(s) }
func (m *Manager) Stop(name string, wait time.Duration) error {
	return m.inner.Stop(name, wait)
}
func (m *Manager) StopAll(base string, wait time.Duration) error { return m.inner.StopAll(base, wait) }
func (m *Manager) Status(name string) (Status, error)            { return m.inner.Status(name) }
func (m *Manager) StatusAll(base string) ([]Status, error)       { return m.inner.StatusAll(base) }
func (m *Manager) Count(base string) (int, error)                { return m.inner.Count(base) }

func (m *Manager) ReconcileOnce()                  { m.inner.ReconcileOnce() }
func (m *Manager) StartReconciler(d time.Duration) { m.inner.StartReconciler(d) }
func (m *Manager) StopReconciler()                 { m.inner.StopReconciler() }

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

func NewOpenSearchHistorySink(baseURL, index string) HistorySink {
	sink, _ := history_factory.NewSinkFromDSN("opensearch://" + strings.TrimPrefix(baseURL, "http://") + "/" + index)
	return sink
}
func NewClickHouseHistorySink(baseURL, table string) HistorySink {
	sink, _ := history_factory.NewSinkFromDSN("clickhouse://" + strings.TrimPrefix(baseURL, "http://") + "?table=" + table)
	return sink
}

// SortSpecsByPriority sorts specs by priority (lower numbers first) and returns a new slice
func SortSpecsByPriority(specs []Spec) []Spec {
	sortedSpecs := make([]Spec, len(specs))
	copy(sortedSpecs, specs)
	sort.SliceStable(sortedSpecs, func(i, j int) bool {
		return sortedSpecs[i].Priority < sortedSpecs[j].Priority
	})
	return sortedSpecs
}
