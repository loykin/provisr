package main

import (
	"fmt"
	"time"

	"github.com/loykin/provisr"
)

// embedded_metrics: start a metrics HTTP server and a demo process.
func main() {
	// Register metrics and serve on :9090
	if err := provisr.RegisterMetricsDefault(); err != nil {
		panic(err)
	}
	go func() {
		fmt.Println("Serving Prometheus metrics at http://localhost:9090/metrics")
		if err := provisr.ServeMetrics(":9090"); err != nil {
			panic(err)
		}
	}()

	mgr := provisr.New()
	// Start a small process
	spec := provisr.Spec{Name: "metrics-demo", Command: "sleep 2"}
	if err := mgr.Register(spec); err != nil {
		panic(err)
	}
	fmt.Println("Started metrics-demo; scrape /metrics while it runs...")
	time.Sleep(2500 * time.Millisecond)
	_ = mgr.Stop("metrics-demo", 2*time.Second)
	fmt.Println("Stopped metrics-demo. Check counters in /metrics.")
}
