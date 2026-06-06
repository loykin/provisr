# provisr

[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/coverage.json&cacheSeconds=60)](https://github.com/loykin/provisr/blob/gh-pages/shields/coverage.json)
[![Go Report Card](https://goreportcard.com/badge/github.com/loykin/provisr)](https://goreportcard.com/report/github.com/loykin/provisr)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/loykin/provisr/badge)](https://securityscorecards.dev/viewer/?uri=github.com/loykin/provisr)
![CodeQL](https://github.com/loykin/provisr/actions/workflows/codeql.yml/badge.svg)
[![Trivy](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/trivy.json&cacheSeconds=60)](https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/trivy.json)

Go process supervisor — embeddable library or standalone daemon, single binary, no Python, no containers.

## Why provisr

| | Docker | systemd | supervisor | provisr |
|---|:---:|:---:|:---:|:---:|
| No Python required | ✓ | ✓ | ✗ | ✓ |
| No containers | ✗ | ✓ | ✓ | ✓ |
| Embeddable as a Go library | ✗ | ✗ | ✗ | ✓ |
| No root required | ✗ | ✗ | ✓ | ✓ |
| Single binary | ✗ | ✗ | ✗ | ✓ |

**Good fit when:**
- Docker/k8s is overkill for a single host (ML workstations, dev servers, CI runners)
- You want to manage child processes from inside a Go application
- You need a daemon without systemd or without root
- You want supervisor-style process management without pulling in Python

## Install

```shell
# CLI binary
go install github.com/loykin/provisr/cmd/provisr@latest

# Library — lightweight core module
go get github.com/loykin/provisr/core

# Library — full module (HTTP API, auth, history)
go get github.com/loykin/provisr
```

## Using as a Library

provisr is split into modules so you can pull in only what you need:

| Module | Use case | Extra deps |
|--------|----------|------------|
| `github.com/loykin/provisr/core` | Process/job control only | prometheus, cron, gopsutil |
| `github.com/loykin/provisr` | Full orchestrator (HTTP API, auth, config, history) | gin, jwt, sqlite, postgres |
| `github.com/loykin/provisr/history/clickhouse` | ClickHouse history backend | clickhouse-go |

### Lightweight embedding (core only)

```go
import "github.com/loykin/provisr/core"

mgr := core.New()
mgr.Register(core.Spec{Name: "notebook", Command: "jupyter notebook --no-browser"})
mgr.Start("notebook")
```

### Full orchestrator

```go
import "github.com/loykin/provisr"

mgr := provisr.New()
// HTTP API, config loading, history backends, auth — all available
```

## Quick Start

### Basic Process Management

```shell
# Register and manage a simple process
provisr register --name demo --command "sleep 10"
provisr start --name demo
provisr status --name demo
provisr stop --name demo
```

### Config-driven Workflow

```shell
# Start daemon with configuration file
provisr serve --config config/config.toml

# Login with admin credentials
provisr login admin  # Password will be prompted securely

# Manage process groups
provisr group-start --group backend
provisr group-stop --group backend
```

## Features

- **Process management**: start/stop/status for processes and multiple instances
- **Auto-restart**: configurable retry logic with intervals and failure detection
- **Lifecycle hooks**: Kubernetes-style hooks (pre_start, post_start, pre_stop, post_stop) with failure modes
- **Job execution**: Kubernetes-style Jobs for one-time tasks with parallelism and retry logic
- **Job dependencies (DAG)**: `DependsOn` field lets jobs wait for upstream jobs before starting
- **Cron scheduling**: Kubernetes-style CronJobs for recurring tasks
- **Process groups**: manage related processes together with scaling support
- **HTTP API**: RESTful API with JSON I/O, embeddable in Gin/Echo applications
- **Metrics**: Prometheus metrics for monitoring processes, jobs, and cronjobs
- **Output streaming**: inject `io.Writer` into `Spec.Log.File.StdoutWriter` / `StderrWriter` for real-time output capture
- **Configuration**: TOML/YAML/JSON config with hot reload and reconciliation
- **Security**: TLS support, input validation, and secure PID management
- **Lightweight core**: `github.com/loykin/provisr/core` for embedding without gin, jwt, or database deps

## HTTP API

### Endpoints

- `POST /api/start` - Start processes (JSON body with process spec)
- `GET /api/status` - Get process status (query: name, base, or wildcard)
- `POST /api/stop` - Stop processes (query: name, base, or wildcard)

### Examples

```shell
# Start multiple instances
curl -X POST localhost:8080/api/start \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","command":"echo hello","instances":2}'

# Check status
curl 'localhost:8080/api/status?base=demo'

# Stop with wildcard
curl -X POST 'localhost:8080/api/stop?wildcard=demo-*'
```

### Embed into Gin or Echo

```go
import "github.com/loykin/provisr"

mgr := provisr.New()

// Mount all endpoints under a Gin router group
router := provisr.NewRouter(mgr, "/api")
r.Any("/api/*any", gin.WrapH(router.Handler()))

// Or use individual handlers with custom middleware
apiGroup := r.Group("/api")
apiGroup.GET("/status", loggingMiddleware(), endpoints.StatusHandler())
apiGroup.POST("/start", authMiddleware(), endpoints.StartHandler())
```

### Output Streaming

Capture process stdout/stderr at runtime without polling log files:

```go
pr, pw := io.Pipe()

mgr.Register(provisr.Spec{
    Name:    "notebook",
    Command: "jupyter notebook --no-browser",
    Log: provisr.LogConfig{
        File: provisr.LogFileConfig{
            StdoutWriter: pw,
        },
    },
})

go io.Copy(os.Stdout, pr)
```

### Server Configuration

```toml
[server]
enabled = true
listen = ":8080"
base_path = "/api"
pidfile = "/var/run/provisr.pid"  # For daemonization
logfile = "/var/log/provisr.log"  # Optional
```

```shell
# Start server
provisr serve --config config/config.toml

# Start as daemon
provisr serve --config config/config.toml --daemonize
```

## Configuration

Provisr supports three entity types: `process`, `job`, and `cronjob`. Each can be defined in:
- Main config file under `[[processes]]` sections
- Individual files in the programs directory (TOML/YAML/JSON)

### Process Example

```toml
type = "process"
[spec]
name = "web"
command = "sh -c 'while true; do echo web; sleep 2; done'"
auto_restart = true
restart_interval = "3s"
priority = 10
```

### Job Example

```toml
type = "job"
[spec]
name = "data-processor"
command = "python process_data.py"
parallelism = 3
completions = 10
backoff_limit = 5
completion_mode = "NonIndexed"
restart_policy = "OnFailure"
active_deadline_seconds = 3600
ttl_seconds_after_finished = 300
```

### CronJob Example

```toml
type = "cronjob"
[spec]
name = "daily-backup"
schedule = "0 2 * * *"
concurrency_policy = "Forbid"
successful_jobs_history_limit = 3
failed_jobs_history_limit = 1

[spec.job_template]
name = "backup-job"
command = "bash /scripts/backup.sh"
parallelism = 1
completions = 1
backoff_limit = 2
restart_policy = "OnFailure"
active_deadline_seconds = 7200
```

Groups reference program names:

```toml
[[groups]]
name = "webstack"
members = ["web", "api"]
```

## Jobs and CronJobs

### Jobs

Jobs are used for one-time task execution with support for:

- **Parallelism**: run multiple instances concurrently
- **Completions**: specify required number of successful completions
- **Backoff limit**: configure retry attempts for failed instances
- **Restart policies**: `Never` or `OnFailure`
- **Completion modes**: `NonIndexed` (default) or `Indexed`
- **Timeouts**: active deadline for job execution
- **TTL**: automatic cleanup after completion

#### Job Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Job name (required) |
| `command` | string | Command to execute (required) |
| `work_dir` | string | Working directory |
| `env` | []string | Environment variables |
| `parallelism` | int32 | Number of parallel instances (default: 1) |
| `completions` | int32 | Required successful completions (default: 1) |
| `backoff_limit` | int32 | Maximum retry attempts (default: 6) |
| `completion_mode` | string | `NonIndexed` or `Indexed` (default: `NonIndexed`) |
| `restart_policy` | string | `Never` or `OnFailure` (default: `Never`) |
| `active_deadline_seconds` | int64 | Job timeout in seconds |
| `ttl_seconds_after_finished` | int32 | Auto-cleanup delay |
| `depends_on` | []string | Jobs that must succeed before this job starts |

### Job Dependencies (DAG)

Use `DependsOn` to form a dependency graph — a job waits for all named jobs to succeed before starting:

```go
jobMgr.CreateJob(provisr.JobSpec{Name: "stage-a", Command: "python stage_a.py"})
jobMgr.CreateJob(provisr.JobSpec{
    Name:      "stage-b",
    Command:   "python stage_b.py",
    DependsOn: []string{"stage-a"},
})
```

If an upstream job fails, all downstream jobs are immediately marked failed.

### CronJobs

#### CronJob Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | CronJob name (required) |
| `schedule` | string | Cron expression or `@every` (required) |
| `job_template` | JobSpec | Template for created jobs (required) |
| `concurrency_policy` | string | `Allow`, `Forbid`, or `Replace` (default: `Allow`) |
| `suspend` | bool | Pause scheduling (default: false) |
| `successful_jobs_history_limit` | int32 | Keep successful jobs (default: 3) |
| `failed_jobs_history_limit` | int32 | Keep failed jobs (default: 1) |
| `starting_deadline_seconds` | int64 | Start deadline for missed schedules |
| `time_zone` | string | Timezone for schedule (default: UTC) |

#### Schedule Examples

```
"0 2 * * *"     # Daily at 2 AM
"0 */4 * * *"   # Every 4 hours
"0 9 * * 1-5"   # Weekdays at 9 AM
"@every 30s"    # Every 30 seconds
"@every 5m"     # Every 5 minutes
"@every 1h"     # Every hour
```

### Programmatic Job Management

```go
mgr := provisr.New()
jobMgr := provisr.NewJobManager(mgr)
scheduler := provisr.NewCronScheduler(mgr)

err := jobMgr.CreateJob(provisr.JobSpec{
    Name:        "my-job",
    Command:     "echo hello",
    Parallelism: int32Ptr(2),
    Completions: int32Ptr(2),
})

cronSpec := provisr.CronJob{
    Name:        "my-cronjob",
    Schedule:    "@every 10s",
    JobTemplate: jobSpec,
}
scheduler.Add(cronSpec)

func int32Ptr(i int32) *int32 { return &i }
```

## Lifecycle Hooks

Hooks run commands at specific points in a process lifecycle, similar to Kubernetes init-containers and lifecycle hooks.

### Hook Phases

- **PreStart**: run before the main process starts (blocking by default)
- **PostStart**: run after the main process starts (can be async)
- **PreStop**: run before stopping the main process (blocking by default)
- **PostStop**: run after the main process stops (blocking by default)

### Hook Configuration Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `name` | string | Hook name (required) | - |
| `command` | string | Command to execute (required) | - |
| `failure_mode` | string | `fail`, `ignore`, or `retry` | `fail` |
| `run_mode` | string | `blocking` or `async` | `blocking` |
| `timeout` | string | Execution timeout (e.g. `30s`, `5m`) | `30s` |

### TOML Example

```toml
type = "process"
[spec]
name = "web-app"
command = "python app.py"
auto_restart = true

[spec.lifecycle]
[[spec.lifecycle.pre_start]]
name = "check-dependencies"
command = "curl -f http://database:5432/health"
failure_mode = "fail"
timeout = "30s"

[[spec.lifecycle.pre_start]]
name = "migrate-database"
command = "python manage.py migrate"
failure_mode = "fail"

[[spec.lifecycle.post_start]]
name = "warmup-cache"
command = "curl -X POST http://localhost:8080/api/warmup"
failure_mode = "ignore"
run_mode = "async"

[[spec.lifecycle.pre_stop]]
name = "drain-connections"
command = "curl -X POST http://localhost:8080/api/drain"
failure_mode = "ignore"
timeout = "30s"

[[spec.lifecycle.post_stop]]
name = "cleanup-temp-files"
command = "rm -rf /tmp/app-*"
failure_mode = "ignore"
```

### Programmatic Example

```go
spec := provisr.Spec{
    Name:    "web-app",
    Command: "python app.py",
    Lifecycle: provisr.LifecycleHooks{
        PreStart: []provisr.Hook{
            {
                Name:        "setup-env",
                Command:     "source /app/setup.sh",
                FailureMode: provisr.FailureModeFail,
                RunMode:     provisr.RunModeBlocking,
                Timeout:     30 * time.Second,
            },
        },
        PostStart: []provisr.Hook{
            {
                Name:        "notify-started",
                Command:     "curl -X POST http://notifications/app-started",
                FailureMode: provisr.FailureModeIgnore,
                RunMode:     provisr.RunModeAsync,
            },
        },
        PreStop: []provisr.Hook{
            {
                Name:        "graceful-shutdown",
                Command:     "curl -X POST http://localhost:8080/shutdown",
                FailureMode: provisr.FailureModeIgnore,
                Timeout:     15 * time.Second,
            },
        },
    },
}
mgr.Register(spec)
```

## Authentication

### Methods

- **Basic Auth**: username/password
- **Client Credentials**: OAuth2-style client_id/client_secret
- **JWT Tokens**: stateless JSON Web Tokens

### Configuration

```toml
[auth]
enabled = true
database_path = "auth.db"
database_type = "sqlite"

[auth.jwt]
secret = "your-secret-key-change-this-in-production"
expires_in = "24h"

[auth.admin]
auto_create = true
username = "admin"
password = "admin"  # Change immediately
email = "admin@localhost"
```

### CLI Session Management

```shell
# Login
provisr login --username=admin --password=secret

# Login with client credentials
provisr login --method=client_secret --client-id=client_123 --client-secret=secret456

# Login to remote server
provisr login --server-url=http://remote:8080/api --username=admin --password=secret

# Session is used automatically for subsequent commands
provisr status
provisr start --name=myapp

# Logout
provisr logout
```

### User and Client Management

```shell
provisr auth user create --username=operator --password=secret --roles=operator
provisr auth client create --name="CI Client" --scopes=operator
provisr auth user list
provisr auth client list
```

### HTTP API Authentication

```shell
# Login via API
curl -X POST localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"method":"basic","username":"admin","password":"secret"}'

# Use JWT token
curl -H "Authorization: Bearer <jwt-token>" http://localhost:8080/api/status

# Basic auth
curl -u admin:password http://localhost:8080/api/status
```

## Metrics

```go
// Enable and serve Prometheus metrics
provisr.RegisterMetricsDefault()
go provisr.ServeMetrics(":9090")
```

Available metrics: process starts/stops/restarts, job completions, cronjob schedules.

## TLS

### Option 1: Auto-generated self-signed (development)

```toml
[server]
listen = ":8443"

[server.tls]
enabled = true
dir = "./tls"
auto_generate = true

[server.tls.auto_gen]
common_name = "localhost"
dns_names = ["localhost", "127.0.0.1"]
ip_addresses = ["127.0.0.1"]
valid_days = 365
```

### Option 2: Certificate files (production)

```toml
[server.tls]
enabled = true
cert_file = "/etc/ssl/certs/provisr.crt"
key_file  = "/etc/ssl/private/provisr.key"
```

### Client TLS

```go
config := client.Config{
    BaseURL: "https://provisr.example.com:8443/api",
    TLS: &client.TLSClientConfig{
        Enabled:    true,
        CACert:     "/path/to/ca.crt",
        ServerName: "provisr.example.com",
    },
}
c := client.New(config)
```

## Storage

| Backend | Config |
|---------|--------|
| SQLite (default) | `database_type = "sqlite"`, `database_path = "data.db"` |
| PostgreSQL | `database_type = "postgres"`, `database_path = "postgres://..."` |
| ClickHouse | `github.com/loykin/provisr/history/clickhouse` |

## Testing

```shell
# Unit tests
go test ./...

# With race detector
go test -race ./...

# Integration tests
make test-integration

# Full CI pipeline
make ci
```

## File Locations

- **PID files**: `<pid_dir>/<name>.pid`
- **Logs**: `<log.dir>/<name>.stdout.log` and `<log.dir>/<name>.stderr.log`
- **Config**: `config/config.toml` (main), programs directory for individual process files
- **Sessions**: `~/.provisr/session.json` (0600 permissions)

## Examples

Runnable examples are in the `examples/` directory:

| Directory | What it shows |
|-----------|---------------|
| `embedded` | Basic embedding |
| `embedded_http_gin` / `embedded_http_echo` | Web framework integration |
| `embedded_process_group` | Process group management |
| `embedded_lifecycle_hooks` | Lifecycle hook patterns |
| `embedded_metrics` | Prometheus metrics |
| `embedded_logger` | Log configuration |
| `job_basic` / `job_advanced` | One-time job execution |
| `cronjob_basic` | Cron scheduling |
| `auth_basic` / `auth_login` | Authentication |
| `tls_example` | TLS setup |
| `store_basic` | Storage backend usage |

```shell
go run ./examples/embedded
go run ./examples/embedded_http_gin
```

## Security

- Input validation prevents path traversal attacks
- PID reuse protection using process start time verification
- Run with least privileges and restrict file system access
- TLS 1.2 and 1.3 supported (1.3 default)

## License

MIT
