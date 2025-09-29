# Basic CronJob Example

This example demonstrates how to create and manage scheduled jobs (cronjobs) in Provisr using both programmatic API and configuration files.

## What it demonstrates

- Creating cronjobs with schedules
- Concurrency policies (Forbid, Allow, Replace)
- Job templates and execution
- History limits for successful/failed jobs
- Basic scheduling using cron expressions

## Prerequisites

No special setup required.

## Running the Example

### Method 1: Using Go code

```bash
cd examples/cronjob_basic
go run main.go
```

### Method 2: Using configuration file

```bash
cd examples/cronjob_basic
provisr serve cronjob_example.toml
```

## Expected Output

The example creates a cronjob that runs every 5 seconds:

```
=== CronJob Basic Example ===
Creating cronjob: hello-cronjob
CronJob created successfully: hello-cronjob
Schedule: @every 5s
Letting cronjob run for 20 seconds...
CronJob has been running - check the logs for executions
Example completed - cronjob will continue running until process ends
```

Each scheduled execution will output:
```
Hello from cronjob at Mon Sep 30 10:15:20 JST 2025!
```

## Configuration Files

### cronjob_example.toml

Demonstrates TOML-based cronjob configuration:

- **Schedule**: `@every 5s` (every 5 seconds)
- **Concurrency Policy**: `Forbid` (no concurrent executions)
- **History Limits**: Keep 3 successful jobs, 1 failed job
- **Job Template**: Defines the actual command to execute

## Key Features

### Schedule Formats

Supports both cron expressions and shortcuts:
- `@every 5s` - Every 5 seconds
- `@every 1m` - Every minute
- `0 */5 * * * *` - Every 5 minutes (cron format)
- `@hourly`, `@daily`, `@weekly`, `@monthly`

### Concurrency Policies

- **Forbid**: Skip if previous job still running
- **Allow**: Allow concurrent executions
- **Replace**: Stop previous job and start new one

### History Management

- `successful_jobs_history_limit`: Number of completed jobs to keep
- `failed_jobs_history_limit`: Number of failed jobs to keep

## Architecture

```
CronScheduler
    ↓
CronJob (hello-cronjob)
    ↓
JobTemplate (defines what to run)
    ↓
Job Instances (actual executions)
```

## Monitoring

To see cronjob status and history:

```bash
# View all process status
provisr status

# View specific cronjob status
provisr status --name=hello-cronjob

# View job execution history
provisr logs --name=hello-cronjob
```

## Advanced Usage

For more complex cronjob scenarios, see:
- `../job_basic/` - Individual job management
- `../job_advanced/` - Advanced job features
- `../embedded_job_lifecycle/` - Job lifecycle events

## Cleanup

The example doesn't create persistent files, but cronjobs will continue running until the process is stopped.