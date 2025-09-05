# Embedded metrics add (custom registry) example

This example shows how to integrate provisr metrics into an existing Prometheus
metrics setup using your own Prometheus registry and HTTP mux.

What it does:
- Creates a custom Prometheus registry
- Registers provisr process metrics into that registry
- Serves /metrics from that registry on :9100 via a custom http.ServeMux
- Starts a couple of demo processes to produce metrics

Run it with:

```shell
go run ./examples/embeded_metrics_add
```

Then open:

- http://localhost:9100/metrics

You should see provisr metric families like:
- provisr_process_starts_total
- provisr_process_restarts_total
- provisr_process_stops_total
- provisr_process_start_duration_seconds
- provisr_process_running_instances

Tip: Compare this with the basic example in example/embedded_metrics which uses
provisr helper functions RegisterMetricsDefault + ServeMetrics to expose metrics
from the default registry.
