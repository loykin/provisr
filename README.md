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
- Robust liveness via detectors (pidfile, pid, command) with PID reuse protection using process start time metadata
- Unified slog-based structured logging with rotating file logs (lumberjack)
- Cron-like scheduler (@every duration)
- Process groups (start/stop/status together)
- Config-driven reconciliation: recover running processes from PID files, start missing ones, and gracefully stop/remove programs no longer in config
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
- Input validation: the server validates spec inputs to avoid unsafe filesystem path usage.
    - name: must match [A-Za-z0-9._-], must not contain ".." or path separators.
    - work_dir, pid_file, log.dir, log.stdoutPath, log.stderrPath: if provided, must be absolute paths
      without traversal (cleaned form only, e.g., no "..").

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

- The server requires a config file and reads the [server] section from it.
- At startup, the manager applies the config once: it recovers processes from PID files (when configured), starts missing ones, and gracefully stops/removes programs not present in the config.
- Daemonization is supported: use `--daemonize`. The daemon PID file path is configured via `[server].pidfile` in the config. For logs, use `--logfile` or set `[server].logfile` in the config.

Example TOML snippet (also present in config/config.toml):

```toml
[server]
enabled = true
listen = ":8080"
base_path = "/api"
```

Once running, use the endpoints described below.

## Configuration model (unified)

Provisr uses a single, unified schema for process definitions in both the main config and the programs directory.

- Discriminated union entries: each entry has `type` and `spec`.
    - `type = "process"` | `"cronjob"`
    - `spec` contains the fields for a process.Spec (and for cronjob: schedule, etc. live inside its spec structure).
- Inline definitions live under `[[processes]]` blocks in the main config.
- The programs directory (set via `programs_directory = "programs"`) contains per-program files in the same schema.
  Supported extensions: .toml, .yaml/.yml, .json.
- Legacy plain per-file process specs are not supported.

Example inline (TOML):

```toml
[[processes]]
type = "process"
[processes.spec]
name = "web"
command = "sh -c 'while true; do echo web; sleep 2; done'"
priority = 10
```

Example programs file (programs/web.toml) with the same schema:

```toml
type = "process"
[spec]
name = "web"
command = "sh -c 'while true; do echo web; sleep 2; done'"
priority = 10
```

Groups reference program names:

```toml
[[groups]]
name = "webstack"
members = ["web", "api"]
```

Cron jobs can also be defined with `type = "cronjob"` in either place. The `provisr cron --config=config.toml` command
runs them.

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
- examples/embedded_client — client/daemon interaction demo (uses examples/embedded_client/daemon-config.toml)
- examples/embedded_http_gin — embed the HTTP API into a Gin app
- examples/embedded_http_echo — embed the HTTP API into an Echo app
- examples/embedded_process_group — process group management
- examples/embedded_logger — logging integration
- examples/embedded_metrics — Prometheus metrics
- examples/embedded_metrics_add — custom metrics
- examples/embedded_config_file — config-driven
- examples/embedded_manager — manager-driven config apply (uses Manager.ApplyConfig)
- examples/embedded_config_structure — struct-driven configuration
- examples/programs_directory — directory-based programs loading with groups and priorities
- examples/programs_detach — demonstrate detached worker managed via programs config

## Files and Paths

This project reads/writes a few files at well-defined locations. The defaults and rules are:

- Working directory (spec.work_dir)
    - If provided, the started process runs with this as its cwd.
    - Must be an absolute path without traversal (e.g., /var/apps/demo). Relative paths are rejected by the HTTP API.

- PID file (spec.pid_file)
    - If provided, the manager writes the child PID to this file immediately after a successful start.
    - You can also configure a default directory for PID files via `pid_dir` in the main config. When set, any process spec without an explicit `pid_file` will default to `<pid_dir>/<name>.pid` (resolved relative to the config file if not absolute).
    - Extended format for safety and recovery:
        1) First line: PID
        2) Second line: JSON-encoded Spec snapshot (optional; used to recover process details on restart)
        3) Third line: JSON-encoded meta with `{ "start_unix": <seconds> }` (optional; used to verify PID identity)
    - Older single-line and two-line formats remain supported for backward compatibility.
    - The PIDFile detector validates that the PID refers to the same process by comparing the recorded start time with the current process start time, preventing PID reuse mistakes.
    - The parent directory is created if missing (mode 0750).
    - Must be an absolute, cleaned path (e.g., /var/run/provisr/demo.pid) when submitted via HTTP API; the CLI/config can use relative paths which are resolved against the config file directory.

- Logs (spec.log)
    - If log.stdoutPath or log.stderrPath are set, logs are written exactly to those files.
    - Otherwise, if log.dir is set, files are created as:
        - <log.dir>/<name>.stdout.log
        - <log.dir>/<name>.stderr.log
    - The directory log.dir is created if needed (mode 0750).
    - Rotation is handled by lumberjack (MaxSizeMB, MaxBackups, MaxAgeDays, Compress).
    - Example: with name "web-1" and log.dir "/var/log/provisr", stdout goes to /var/log/provisr/web-1.stdout.log.

- Config file
    - Example configuration is in config/config.toml. CLI examples use:
        - provisr start --config config/config.toml
    - You can keep your own TOML anywhere and pass the path via --config.

- Examples
    - See the examples/ directory for runnable samples. Some include their own config directories, e.g.,
      examples/embedded_config_file/config/config.toml.

- Naming and path rules (validated by the HTTP API)
    - name: allowed characters [A-Za-z0-9._-]; must not contain ".." or path separators.
    - work_dir, pid_file, log.dir, log.stdoutPath, log.stderrPath: if provided, must be absolute paths without
      traversal (cleaned form; no "..").

## Security notes

- The HTTP API performs input validation for process specs to mitigate uncontrolled path usage (CodeQL: "Uncontrolled
  data used in path expression").
- PID reuse protection: PID file meta includes the process start time and is validated against the live process using platform-native methods (procfs on Linux, sysctl on Darwin/BSD via gopsutil, WinAPI on Windows). No external `ps` calls are used.
- Even with validation, run the server with least privileges and restrict log directories and pid file locations to
  trusted paths.

## Notes and breaking changes

- Persistence store removed: internal/store has been deleted. The manager now operates purely via in-memory state and PID files for recovery.
- Serve flags simplified: `provisr serve` requires a config file. Daemonization uses `--daemonize`; the daemon PID file is configured via `[server].pidfile`. API listen/base are configured via the TOML `[server]` section.
- Config-driven reconciliation: at startup, processes are recovered from PID files when available; processes not present in the config are gracefully shut down and cleaned up.
