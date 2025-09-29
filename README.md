# provisr

[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/coverage.json&cacheSeconds=60)](https://github.com/loykin/provisr/blob/gh-pages/shields/coverage.json)
[![Go Report Card](https://goreportcard.com/badge/github.com/loykin/provisr)](https://goreportcard.com/report/github.com/loykin/provisr)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/loykin/provisr/badge)](https://securityscorecards.dev/viewer/?uri=github.com/loykin/provisr)
![CodeQL](https://github.com/loykin/provisr/actions/workflows/codeql.yml/badge.svg)
[![Trivy](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/trivy.json&cacheSeconds=60)](https://raw.githubusercontent.com/loykin/provisr/gh-pages/shields/trivy.json)

A minimal supervisord-like process manager written in Go.

## Features

- **Process management**: Start/stop/status for processes and multiple instances
- **Auto-restart**: Configurable retry logic with intervals and failure detection
- **Lifecycle hooks**: Kubernetes-style hooks (pre_start, post_start, pre_stop, post_stop) with failure modes
- **Job execution**: Kubernetes-style Jobs for one-time tasks with parallelism and retry logic
- **Cron scheduling**: Kubernetes-style CronJobs for recurring tasks
- **Process groups**: Manage related processes together with scaling support
- **HTTP API**: RESTful API with JSON I/O, embeddable in Gin/Echo applications
- **Metrics**: Prometheus metrics for monitoring processes, jobs, and cronjobs
- **Configuration**: TOML/YAML/JSON config with hot reload and reconciliation
- **Security**: TLS support, input validation, and secure PID management

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

# Manage process groups
provisr group-start --group backend
provisr group-stop --group backend
```

### Process Registration

```shell
# Register processes
provisr register --name web --command "python app.py" --work-dir /app
provisr register-file --file ./process-config.json

# Remote registration
provisr register --name api --command "./server" --api-url http://remote:8080/api

# Remove processes
provisr unregister --name web
```

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

### Server Configuration

```toml
[server]
enabled = true
listen = ":8080"
base_path = "/api"
pidfile = "/var/run/provisr.pid"  # For daemonization
logfile = "/var/log/provisr.log"   # Optional
```

```shell
# Start server
provisr serve --config config/config.toml

# Start as daemon
provisr serve --config config/config.toml --daemonize
```

## Authentication

Provisr supports multiple authentication methods for securing both HTTP API and CLI access:

### Authentication Methods

- **Basic Auth**: Username/password authentication
- **Client Credentials**: OAuth2-style client_id/client_secret
- **JWT Tokens**: JSON Web Tokens for stateless authentication

### Configuration

Enable authentication in your `config.toml`:

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
password = "admin"  # Change this immediately!
email = "admin@localhost"
```

### CLI Session Management

The `provisr login` command provides persistent session management:

```shell
# Login with username/password
provisr login --username=admin --password=secret

# Login with client credentials
provisr login --method=client_secret --client-id=client_123 --client-secret=secret456

# Login to remote server
provisr login --server-url=http://remote:8080/api --username=admin --password=secret

# Use authenticated commands (session is automatically used)
provisr status
provisr start --name=myapp
provisr stop --name=myapp

# Check session status
cat ~/.provisr/session.json

# Logout when done
provisr logout
```

### User & Client Management

```shell
# Create users
provisr auth user create --username=operator --password=secret --roles=operator

# Create API clients
provisr auth client create --name="CI Client" --scopes=operator

# List users and clients
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

# Basic authentication (legacy)
curl -u admin:password http://localhost:8080/api/status
```

### Session Security

- Sessions stored in `~/.provisr/session.json` with 0600 permissions
- Automatic token expiration and cleanup
- Server URL validation prevents token reuse on wrong servers
- Support for multiple server sessions (logout and login to switch)

## Storage

Provisr uses a flexible storage layer that supports multiple backends:

### Supported Backends

- **SQLite**: File-based database (default)
- **PostgreSQL**: Production-ready relational database

### Generic Store Interface

```go
// Create store using factory
config := store.Config{
    Type: "sqlite",
    Path: "data.db",
}
store, _ := store.CreateStore(config)

// Or use specific auth store
authStore, _ := store.NewAuthStore(config)
```

### Features

- **Transaction Support**: ACID transactions across operations
- **Connection Pooling**: Configurable connection limits
- **Type Safety**: Generic interfaces with compile-time safety
- **Extensible**: Easy to add new storage backends

## Configuration

Provisr supports three entity types: `process`, `job`, and `cronjob`. Each can be defined in:
- Main config file under `[[processes]]` sections
- Individual files in the programs directory (TOML/YAML/JSON)

### Process Example

```toml
# Long-running process
type = "process"
[spec]
name = "web"
command = "sh -c 'while true; do echo web; sleep 2; done'"
priority = 10
```

### Job Example

```toml
# One-time job execution
type = "job"
[spec]
name = "data-processor"
command = "python process_data.py"
parallelism = 3                    # Run 3 instances in parallel
completions = 10                   # Need 10 successful completions
backoff_limit = 5                  # Allow up to 5 retries
completion_mode = "NonIndexed"     # Standard completion mode
restart_policy = "OnFailure"       # Restart failed instances
active_deadline_seconds = 3600     # Job timeout: 1 hour
ttl_seconds_after_finished = 300   # Auto-cleanup after 5 minutes
```

### CronJob Example

```toml
# Recurring scheduled job
type = "cronjob"
[spec]
name = "daily-backup"
schedule = "0 2 * * *"                    # Run at 2 AM every day
concurrency_policy = "Forbid"             # Don't allow concurrent executions
successful_jobs_history_limit = 3         # Keep 3 successful job records
failed_jobs_history_limit = 1             # Keep 1 failed job record

# Job template - defines the job that gets created
[spec.job_template]
name = "backup-job"
command = "bash /scripts/backup.sh"
parallelism = 1
completions = 1
backoff_limit = 2
restart_policy = "OnFailure"
active_deadline_seconds = 7200            # 2 hour timeout
```

Groups reference program names:

```toml
[[groups]]
name = "webstack"
members = ["web", "api"]
```

## Jobs and CronJobs

Provisr implements Kubernetes-style Jobs and CronJobs for task execution and scheduling.

### Jobs

Jobs are used for one-time task execution with support for:

- **Parallelism**: Run multiple instances concurrently
- **Completions**: Specify required number of successful completions
- **Backoff limit**: Configure retry attempts for failed instances
- **Restart policies**: `Never` or `OnFailure`
- **Completion modes**: `NonIndexed` (default) or `Indexed`
- **Timeouts**: Active deadline for job execution
- **TTL**: Automatic cleanup after completion

#### Job Configuration Fields

| Field                        | Type     | Description                                       |
|------------------------------|----------|---------------------------------------------------|
| `name`                       | string   | Job name (required)                               |
| `command`                    | string   | Command to execute (required)                     |
| `work_dir`                   | string   | Working directory                                 |
| `env`                        | []string | Environment variables                             |
| `parallelism`                | int32    | Number of parallel instances (default: 1)         |
| `completions`                | int32    | Required successful completions (default: 1)      |
| `backoff_limit`              | int32    | Maximum retry attempts (default: 6)               |
| `completion_mode`            | string   | "NonIndexed" or "Indexed" (default: "NonIndexed") |
| `restart_policy`             | string   | "Never" or "OnFailure" (default: "Never")         |
| `active_deadline_seconds`    | int64    | Job timeout in seconds                            |
| `ttl_seconds_after_finished` | int32    | Auto-cleanup delay                                |

### CronJobs

CronJobs schedule Jobs to run periodically with support for:

- **Cron expressions**: Standard cron syntax or `@every` shortcuts
- **Concurrency policies**: Control overlapping executions
- **History limits**: Manage job record retention
- **Timezone support**: Schedule in specific timezones
- **Suspension**: Temporarily pause scheduling

#### CronJob Configuration Fields

| Field                           | Type    | Description                                        |
|---------------------------------|---------|----------------------------------------------------|
| `name`                          | string  | CronJob name (required)                            |
| `schedule`                      | string  | Cron expression or @every (required)               |
| `job_template`                  | JobSpec | Template for created jobs (required)               |
| `concurrency_policy`            | string  | "Allow", "Forbid", or "Replace" (default: "Allow") |
| `suspend`                       | bool    | Pause scheduling (default: false)                  |
| `successful_jobs_history_limit` | int32   | Keep successful jobs (default: 3)                  |
| `failed_jobs_history_limit`     | int32   | Keep failed jobs (default: 1)                      |
| `starting_deadline_seconds`     | int64   | Start deadline for missed schedules                |
| `time_zone`                     | string  | Timezone for schedule (default: UTC)               |

#### Schedule Examples

```
# Cron expressions
"0 2 * * *"        # Daily at 2 AM
"0 */4 * * *"      # Every 4 hours
"0 9 * * 1-5"      # Weekdays at 9 AM

# @every shortcuts
"@every 30s"       # Every 30 seconds
"@every 5m"        # Every 5 minutes
"@every 1h"        # Every hour
"@every 24h"       # Every 24 hours
```

### Job and CronJob Management

Use the programmatic API for dynamic job management:

```go
package main

import (
	"fmt"
	"time"
	"github.com/loykin/provisr"
)

func main() {
	// Create managers
	mgr := provisr.New()
	jobMgr := provisr.NewJobManager(mgr)
	scheduler := provisr.NewCronScheduler(mgr)

	// Create a job
	jobSpec := provisr.JobSpec{
		Name:        "my-job",
		Command:     "echo 'Hello from job!'",
		Parallelism: int32Ptr(2),
		Completions: int32Ptr(2),
	}

	err := jobMgr.CreateJob(jobSpec)
	if err != nil {
		panic(err)
	}

	// Monitor job status
	for {
		status, exists := jobMgr.GetJob("my-job")
		if !exists {
			break
		}
		if status.Phase == "Succeeded" || status.Phase == "Failed" {
			fmt.Printf("Job completed: %s\n", status.Phase)
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Create a cronjob
	cronSpec := provisr.CronJob{
		Name:        "my-cronjob",
		Schedule:    "@every 10s",
		JobTemplate: jobSpec,
	}

	err = scheduler.Add(cronSpec)
	if err != nil {
		panic(err)
	}
}

func int32Ptr(i int32) *int32 { return &i }
```

See the `examples/` directory for complete working examples of Jobs and CronJobs.

## Lifecycle Hooks

Provisr supports Kubernetes-style lifecycle hooks that allow you to run commands at specific points in a process, job, or cronjob lifecycle. This is similar to init-containers and lifecycle hooks in Kubernetes.

### Hook Phases

Lifecycle hooks can be configured for four phases:

- **PreStart**: Run before the main process starts (blocking by default)
- **PostStart**: Run after the main process starts (can be async)
- **PreStop**: Run before stopping the main process (blocking by default)
- **PostStop**: Run after the main process stops (blocking by default)

### Hook Configuration

Each hook supports the following configuration:

| Field         | Type   | Description                                    | Default   |
|---------------|--------|------------------------------------------------|-----------|
| `name`        | string | Hook name (required)                           | -         |
| `command`     | string | Command to execute (required)                  | -         |
| `failure_mode`| string | "fail", "ignore", or "retry"                   | "fail"    |
| `run_mode`    | string | "blocking" or "async"                          | "blocking"|
| `timeout`     | string | Hook execution timeout (e.g., "30s", "5m")    | "30s"     |

#### Failure Modes

- **fail**: Stop the operation if hook fails (default)
- **ignore**: Continue despite hook failure
- **retry**: Retry the hook once, then fail if still failing

#### Run Modes

- **blocking**: Wait for hook to complete before continuing (default)
- **async**: Start hook and continue immediately (useful for notifications)

### Process Lifecycle Hooks

Add lifecycle hooks to process specifications:

```toml
# Process with lifecycle hooks
type = "process"
[spec]
name = "web-app"
command = "python app.py"
auto_restart = true

[spec.lifecycle]
# Pre-start hooks (setup phase)
[[spec.lifecycle.pre_start]]
name = "check-dependencies"
command = "curl -f http://database:5432/health"
failure_mode = "fail"
run_mode = "blocking"
timeout = "30s"

[[spec.lifecycle.pre_start]]
name = "migrate-database"
command = "python manage.py migrate"
failure_mode = "fail"
run_mode = "blocking"

# Post-start hooks (after process starts)
[[spec.lifecycle.post_start]]
name = "warmup-cache"
command = "curl -X POST http://localhost:8080/api/warmup"
failure_mode = "ignore"
run_mode = "async"

[[spec.lifecycle.post_start]]
name = "health-check"
command = "curl -f http://localhost:8080/health"
failure_mode = "ignore"
run_mode = "blocking"
timeout = "60s"

# Pre-stop hooks (graceful shutdown)
[[spec.lifecycle.pre_stop]]
name = "drain-connections"
command = "curl -X POST http://localhost:8080/api/drain"
failure_mode = "ignore"
run_mode = "blocking"
timeout = "30s"

# Post-stop hooks (cleanup)
[[spec.lifecycle.post_stop]]
name = "cleanup-temp-files"
command = "rm -rf /tmp/app-*"
failure_mode = "ignore"
run_mode = "blocking"

[[spec.lifecycle.post_stop]]
name = "backup-logs"
command = "tar -czf /backups/app-logs-$(date +%Y%m%d).tar.gz /var/log/app/"
failure_mode = "ignore"
run_mode = "async"
```

### Job Lifecycle Hooks

Jobs inherit the same lifecycle hook system:

```toml
type = "job"
[spec]
name = "data-processor"
command = "python process_data.py"
parallelism = 2
completions = 5

[spec.lifecycle]
# Setup hooks
[[spec.lifecycle.pre_start]]
name = "download-data"
command = "aws s3 sync s3://data-bucket /tmp/input/"
failure_mode = "fail"
run_mode = "blocking"

[[spec.lifecycle.pre_start]]
name = "validate-input"
command = "python validate_data.py /tmp/input/"
failure_mode = "fail"
run_mode = "blocking"

# Cleanup hooks
[[spec.lifecycle.post_stop]]
name = "upload-results"
command = "aws s3 sync /tmp/output/ s3://results-bucket/"
failure_mode = "retry"
run_mode = "blocking"

[[spec.lifecycle.post_stop]]
name = "cleanup-workspace"
command = "rm -rf /tmp/input /tmp/output"
failure_mode = "ignore"
run_mode = "blocking"
```

### CronJob Lifecycle Hooks

CronJobs support two levels of lifecycle hooks:

1. **CronJob-level hooks**: Applied to every scheduled job execution
2. **JobTemplate hooks**: Defined in the job template

Hooks are merged when a job is created, with CronJob-level hooks taking precedence.

```toml
type = "cronjob"
[spec]
name = "daily-backup"
schedule = "0 2 * * *"
concurrency_policy = "Forbid"

# CronJob-level hooks (applied to every execution)
[spec.lifecycle]
[[spec.lifecycle.pre_start]]
name = "check-maintenance-mode"
command = "curl -f http://maintenance-api/status"
failure_mode = "fail"
run_mode = "blocking"

[[spec.lifecycle.post_stop]]
name = "update-metrics"
command = "curl -X POST http://metrics-api/backup-completed"
failure_mode = "ignore"
run_mode = "async"

# Job template with its own hooks
[spec.job_template]
name = "backup-job"
command = "bash /scripts/backup.sh"

[spec.job_template.lifecycle]
[[spec.job_template.lifecycle.pre_start]]
name = "check-disk-space"
command = "df -h | grep '/backup' | awk '{print $4}' | sed 's/%//' | awk '$1 < 90'"
failure_mode = "fail"
run_mode = "blocking"

[[spec.job_template.lifecycle.post_stop]]
name = "verify-backup"
command = "bash /scripts/verify-backup.sh"
failure_mode = "retry"
run_mode = "blocking"
```

### Programmatic Hook Configuration

Configure lifecycle hooks programmatically:

```go
package main

import (
    "time"
    "github.com/loykin/provisr"
    "github.com/loykin/provisr/internal/process"
)

func main() {
    mgr := provisr.New()

    spec := provisr.Spec{
        Name:    "web-app",
        Command: "python app.py",
        Lifecycle: process.LifecycleHooks{
            PreStart: []process.Hook{
                {
                    Name:        "setup-env",
                    Command:     "source /app/setup.sh",
                    FailureMode: process.FailureModeFail,
                    RunMode:     process.RunModeBlocking,
                    Timeout:     30 * time.Second,
                },
            },
            PostStart: []process.Hook{
                {
                    Name:        "notify-started",
                    Command:     "curl -X POST http://notifications/app-started",
                    FailureMode: process.FailureModeIgnore,
                    RunMode:     process.RunModeAsync,
                },
            },
            PreStop: []process.Hook{
                {
                    Name:        "graceful-shutdown",
                    Command:     "curl -X POST http://localhost:8080/shutdown",
                    FailureMode: process.FailureModeIgnore,
                    RunMode:     process.RunModeBlocking,
                    Timeout:     15 * time.Second,
                },
            },
            PostStop: []process.Hook{
                {
                    Name:        "cleanup",
                    Command:     "rm -rf /tmp/app-cache",
                    FailureMode: process.FailureModeIgnore,
                    RunMode:     process.RunModeBlocking,
                },
            },
        },
    }

    if err := mgr.Register(spec); err != nil {
        panic(err)
    }

    if err := mgr.Start("web-app"); err != nil {
        panic(err)
    }
}
```

### Examples

See the `examples/` directory for complete lifecycle hook examples:

- `examples/embedded_lifecycle_hooks/` - Basic programmatic lifecycle hooks
- `examples/embedded_lifecycle_config/` - Configuration-driven lifecycle hooks
- `examples/embedded_lifecycle_failure_modes/` - Different failure modes and behaviors
- `examples/embedded_job_lifecycle/` - Job-specific lifecycle hooks

### Security Considerations

- **Command validation**: Hook commands are validated to prevent path traversal attacks
- **Environment isolation**: Hooks run with the same environment as the main process
- **Timeout enforcement**: All hooks have configurable timeouts to prevent hanging
- **Failure isolation**: Hook failures don't affect other hooks (except in fail mode)
- **Logging**: All hook executions are logged with detailed status information

## TLS Configuration

Provisr supports HTTPS with flexible TLS configuration options for secure communication.

### Server TLS Configuration

Configure TLS in your `config.toml` under the `[server.tls]` section:

#### Option 1: Auto-generated Self-signed Certificates (Development)

```toml
[server]
listen = ":8443"
base_path = "/api"

[server.tls]
enabled = true
dir = "./tls"
auto_generate = true

[server.tls.auto_gen]
common_name = "localhost"
dns_names = ["localhost", "127.0.0.1", "provisr.local"]
ip_addresses = ["127.0.0.1"]
valid_days = 365
```

#### Option 2: Manual Certificate Files (Production)

```toml
[server]
listen = ":8443"

[server.tls]
enabled = true
cert_file = "/etc/ssl/certs/provisr.crt"
key_file = "/etc/ssl/private/provisr.key"
```

#### Option 3: Directory-based Certificates

```toml
[server.tls]
enabled = true
dir = "/etc/provisr/tls"
# Looks for tls.crt, tls.key, and tls_ca.crt in the directory
```

### Client TLS Configuration

When connecting to HTTPS endpoints, clients support various TLS options:

```go
// Basic HTTPS client
config := client.DefaultTLSConfig()
c := client.New(config)

// Insecure client (skip verification - development only)
config := client.InsecureConfig()
c := client.New(config)

// Custom TLS client with CA certificate
config := client.Config{
BaseURL: "https://provisr.example.com:8443/api",
TLS: &client.TLSClientConfig{
Enabled:    true,
CACert:     "/path/to/ca.crt",
ServerName: "provisr.example.com",
SkipVerify: false,
},
}
c := client.New(config)
```

### TLS Configuration Priority

1. **cert_file + key_file**: Explicit certificate files (highest priority)
2. **dir + auto_generate=true**: Auto-generate certificates in directory
3. **dir**: Use existing certificates from directory

### Security Notes

- **Development**: Use `auto_generate = true` for quick setup with self-signed certificates
- **Production**: Always use certificates from a trusted CA
- **File Permissions**: Ensure private keys have restrictive permissions (0600)
- **TLS Versions**: Supports TLS 1.2 and 1.3 (1.3 is default)

## Embedding

Embed provisr into existing Gin or Echo applications:

```go
// Basic integration - register all endpoints
endpoints := server.NewAPIEndpoints(mgr, "/api")
endpoints.RegisterAll(r.Group("/api"))

// Individual endpoints with custom middleware
apiGroup := r.Group("/api")
apiGroup.GET("/status", loggingMiddleware(), endpoints.StatusHandler())
apiGroup.POST("/start", authMiddleware(), endpoints.StartHandler())
```

See `examples/embedded_http_gin` and `examples/embedded_http_echo` for complete examples.

## Metrics

Prometheus metrics for processes, jobs, and cronjobs:

```go
// Enable metrics
provisr.RegisterMetricsDefault()

// Serve metrics endpoint
go provisr.ServeMetrics(":9090")
```

Available metrics: process starts/stops/restarts, job completions, cronjob schedules. See `examples/embedded_metrics` for details.

## Examples

### Framework Integration
- `embedded_http_gin` / `embedded_http_echo` - Embed API into web frameworks
- `embedded_client` - Client/daemon interaction patterns

### Process Management
- `embedded` - Basic embedding and process management
- `embedded_process_group` - Process group operations
- `programs_directory` - Directory-based configuration

### Jobs and Scheduling
- `job_basic` / `job_advanced` - One-time job execution
- `cronjob_basic` - Scheduled recurring jobs
- `job_config` - Job configuration examples

### Advanced Features
- `embedded_lifecycle_hooks` - Lifecycle hook patterns
- `embedded_metrics` - Prometheus metrics integration
- `embedded_logger` - Custom logging setup

### Storage & Authentication
- `auth_basic` - Authentication system usage and API examples
- `auth_login` - CLI login and session management examples
- `store_basic` - Generic store interface examples

See the `examples/` directory for complete implementations.

## File Locations

- **PID files**: Written to track running processes. Defaults to `<pid_dir>/<name>.pid`
- **Logs**: Written to `<log.dir>/<name>.stdout.log` and `<log.dir>/<name>.stderr.log`
- **Config**: Main config typically `config/config.toml`, programs directory for individual process files

Paths must be absolute when using HTTP API. File rotation is handled automatically with configurable limits.

## Security

- Input validation prevents path traversal attacks
- PID reuse protection using process start time verification
- Run with least privileges and restrict file system access

## License

MIT
