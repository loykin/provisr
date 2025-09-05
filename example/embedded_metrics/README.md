# Embedded metrics example

This example shows how to expose Prometheus metrics from provisr and start a simple process.

Run it with:

- go run ./example/embedded_metrics

It will:

- Register Prometheus metrics
- Start an HTTP server on :9090 serving /metrics
- Start a short-lived process (sleep 2)

You can then open http://localhost:9090/metrics and observe metrics like:

- provisr_process_starts_total{ name="metrics-demo" }
- provisr_process_stops_total{ name="metrics-demo" }
- provisr_process_restarts_total{ name="..." }
- provisr_process_start_duration_seconds_bucket{ name="..." }
- provisr_process_running_instances{ base="..." }
