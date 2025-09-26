package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/loykin/provisr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// embedded_metrics_add:
// Demonstrates adding provisr metrics to an existing Prometheus setup
// with a custom registry and HTTP mux.
func main() {
	// Imagine you already have a custom Prometheus registry you use in your app.
	reg := prometheus.NewRegistry()

	// Register provisr (process manager) metrics into your registry.
	if err := provisr.RegisterMetrics(reg); err != nil {
		panic(err)
	}

	// Expose metrics via your own HTTP mux and address.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	addr := ":9100"
	go func() {
		fmt.Println("Serving custom Prometheus metrics at http://localhost" + addr + "/metrics")
		srv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	// Use provisr as usual.
	mgr := provisr.New()

	// Start a couple of short-lived demo processes to generate some metrics.
	if err := mgr.Register(provisr.Spec{Name: "metrics-add-a", Command: "sleep 1"}); err != nil {
		panic(err)
	}
	if err := mgr.Register(provisr.Spec{Name: "metrics-add-b", Command: "sleep 2"}); err != nil {
		panic(err)
	}

	fmt.Println("Started metrics-add-a and metrics-add-b; scrape /metrics on :9100 to see provisr counters and gauges…")
	// Wait until the first process exits and then stop the second one to record stops as well.
	time.Sleep(1500 * time.Millisecond)
	_ = mgr.Stop("metrics-add-b", 2*time.Second)
	fmt.Println("Stopped metrics-add-b. Check /metrics for provisr_process_* metrics.")
}
