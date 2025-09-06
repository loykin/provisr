# provisr

[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/coverage.json&cacheSeconds=60)](https://github.com/loykin/provisr/blob/gh-pages/shields/coverage.json)
[![Go Report Card](https://goreportcard.com/badge/github.com/loykin/provisr)](https://goreportcard.com/report/github.com/loykin/provisr)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/loykin/provisr/badge)](https://securityscorecards.dev/viewer/?uri=github.com/loykin/provisr)
![CodeQL](https://github.com/loykin/provisr/actions/workflows/codeql.yml/badge.svg)
[![Trivy](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/trivy.json&cacheSeconds=60)](https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/trivy.json)

A minimal supervisord-like process manager written in Go.

## Features

- Start/stop/status for processes and multiple instances
- Auto-restart, retry with interval, start duration window
- Pluggable detectors (pidfile, pid, command)
- Logging to rotating files via lumberjack
- Cron-like scheduler (@every duration)
- Process groups (start/stop/status together)
- Config via TOML (Cobra + Viper)
- Embeddable HTTP API (Gin-based) with configurable basePath and JSON I/O
- Wildcard support for querying/stopping processes via REST (e.g., demo-*, *worker*)
- Easy embedding into existing Gin and Echo apps (see examples)

## CLI quickstart

```shell
provisr start --name demo --cmd "sleep 10"
provisr status --name demo
provisr stop --name demo

# Using a config file
provisr start --config config/config.toml
provisr cron --config config/config.toml
provisr group-start --config config/config.toml --group backend
```

## HTTP API (REST)

- Base path is configurable; default used in examples is /api.
- Endpoints:
    - POST {base}/start — body is JSON Spec
    - POST {base}/stop — query: name= or base= or wildcard= (exactly one), optional wait=duration
    - GET {base}/status — query: name= or base= or wildcard= (exactly one)
- JSON fields are explicitly tagged (e.g., status fields like running, pid, started_at).

Examples (assuming server running on localhost:8080 and base /api):

Start N instances:

```shell
curl -s -X POST localhost:8080/api/start \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","command":"/bin/sh -c \"while true; do echo demo; sleep 5; done\"","instances":2}'
```

Get status by base:

```shell
curl -s 'localhost:8080/api/status?base=demo' | jq .
```

Get status by wildcard:

```shell
curl -s 'localhost:8080/api/status?wildcard=demo-*' | jq .
```

Stop by wildcard:

```shell
curl -s -X POST 'localhost:8080/api/stop?wildcard=demo-*' | jq .
```

## Starting the HTTP API server (CLI)

You can run a standalone REST server with the built-in command:

```shell
provisr serve --config config/config.toml
```

Notes:
- The server reads the [http_api] section from the TOML file.
- You can override config via flags: `--api-listen` and `--api-base`.
- If `http_api.enabled` is false or missing, you must provide `--api-listen` to start anyway.

Example TOML snippet (also present in config/config.toml):

```toml
[http_api]
enabled = true
listen = ":8080"
base_path = "/api"
```

Once running, use the endpoints described below.

## Embedding the API

- Gin example: examples/embedded_http_gin
- Echo example: examples/embedded_http_echo

Each example mounts the REST API and automatically starts a small demo process so you can immediately query /status.

To run the Gin example:

```shell
cd examples/embedded_http_gin
API_BASE=/api go run .
```

To run the Echo example:

```shell
cd examples/embedded_http_echo
API_BASE=/api go run .
```

## More examples

- examples/embedded — minimal embedding
- examples/embedded_process_group — process group management
- examples/embedded_logger — logging integration
- examples/embedded_metrics — Prometheus metrics
- examples/embedded_metrics_add — custom metrics
- examples/embedded_config_file — config-driven
- examples/embedded_config_structure — struct-driven configuration
