package metrics

import (
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Package-level Prometheus collectors. They are registered via Register.
var (
	regOK atomic.Bool

	processStarts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "provisr",
			Subsystem: "process",
			Name:      "starts_total",
			Help:      "Number of successful process starts.",
		}, []string{"name"},
	)
	processRestarts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "provisr",
			Subsystem: "process",
			Name:      "restarts_total",
			Help:      "Number of auto restarts.",
		}, []string{"name"},
	)
	processStops = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "provisr",
			Subsystem: "process",
			Name:      "stops_total",
			Help:      "Number of stops (graceful or kill).",
		}, []string{"name"},
	)
	processStartDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "provisr",
			Subsystem: "process",
			Name:      "start_duration_seconds",
			Help:      "Observed start duration wait window when StartDuration > 0.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"name"},
	)
	runningInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "provisr",
			Subsystem: "process",
			Name:      "running_instances",
			Help:      "Current running instances per base process name.",
		}, []string{"base"},
	)
)

// Register registers all metrics with the provided registerer.
// It is safe to call multiple times; subsequent calls after success are no-ops.
func Register(r prometheus.Registerer) error {
	if regOK.Load() {
		return nil
	}
	cs := []prometheus.Collector{processStarts, processRestarts, processStops, processStartDuration, runningInstances}
	for _, c := range cs {
		if err := r.Register(c); err != nil {
			// If already registered, ignore (allows double Register with default registry)
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				_ = are // keep existing
				continue
			}
			return err
		}
	}
	regOK.Store(true)
	return nil
}

// Handler returns an http.Handler that serves Prometheus metrics for the DefaultGatherer.
// The caller is responsible for starting an HTTP server and wiring the route.
func Handler() http.Handler { return promhttp.Handler() }

// Below are lightweight helpers used by internal packages to record metrics.
// They no-op if Register hasn't been called.

func IncStart(name string) {
	if regOK.Load() {
		processStarts.WithLabelValues(name).Inc()
	}
}
func IncRestart(name string) {
	if regOK.Load() {
		processRestarts.WithLabelValues(name).Inc()
	}
}
func IncStop(name string) {
	if regOK.Load() {
		processStops.WithLabelValues(name).Inc()
	}
}
func ObserveStartDuration(name string, seconds float64) {
	if regOK.Load() {
		processStartDuration.WithLabelValues(name).Observe(seconds)
	}
}
func SetRunningInstances(base string, n int) {
	if regOK.Load() {
		runningInstances.WithLabelValues(base).Set(float64(n))
	}
}
