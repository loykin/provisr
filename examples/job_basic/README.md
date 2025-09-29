# Basic Job Example

This example demonstrates how to create and manage batch jobs in Provisr using configuration files.

## What it demonstrates

- Creating jobs from TOML configuration
- Parallel job execution with multiple instances
- Job completion tracking and requirements
- Retry policies and backoff limits
- Job timeouts and automatic cleanup
- Job status monitoring

## Prerequisites

No special setup required.

## Running the Example

```bash
cd examples/job_basic
go run main.go
```

## Expected Output

```
=== Job Basic Example ===
Loaded configuration with 1 process definitions
Job started successfully!
Monitoring job status...
Iteration 1: Found 2 job instances
  - hello-job-0: Running=true, PID=12345
  - hello-job-1: Running=true, PID=12346
Iteration 2: Found 2 job instances
  - hello-job-0: Running=false, PID=0
  - hello-job-1: Running=false, PID=0
All job instances completed!
Example completed
```

## Configuration (job_example.toml)

The example defines a job with the following characteristics:

```toml
[processes.spec]
name = "hello-job"
command = "echo 'Hello from job!'; sleep 2"
parallelism = 2                    # Run 2 instances in parallel
completions = 2                    # Need 2 successful completions
backoff_limit = 3                  # Allow up to 3 retries
restart_policy = "Never"
active_deadline_seconds = 30       # 30 second timeout
ttl_seconds_after_finished = 60    # Cleanup after 1 minute
```

## Key Job Configuration Options

### Parallelism & Completion
- **parallelism**: Maximum number of instances running simultaneously
- **completions**: Required number of successful completions
- **backoff_limit**: Maximum retry attempts for failed instances

### Timeout & Cleanup
- **active_deadline_seconds**: Total job timeout
- **ttl_seconds_after_finished**: Auto-cleanup delay after completion

### Restart Policy
- **Never**: Don't restart failed instances (rely on backoff_limit)
- **OnFailure**: Restart failed instances
- **Always**: Always restart instances

## Job Lifecycle

1. **Creation**: Job is created with specified parallelism
2. **Execution**: Multiple instances run in parallel
3. **Monitoring**: Track running/completed instances
4. **Completion**: Job succeeds when required completions are met
5. **Cleanup**: Automatic cleanup after TTL expires

## Job vs Process

- **Jobs**: Batch workloads that run to completion
- **Processes**: Long-running services that should stay running

Jobs are perfect for:
- Data processing tasks
- Batch computations
- One-time operations
- Parallel workloads

## Monitoring Jobs

```bash
# View job status
provisr status --name=hello-job

# View all job instances
provisr status --wildcard=hello-job-*

# View job logs
provisr logs --name=hello-job
```

## Advanced Features

For more advanced job features, see:
- `../job_advanced/` - Advanced job configuration
- `../embedded_job_lifecycle/` - Job lifecycle hooks
- `../cronjob_basic/` - Scheduled jobs
- `../job_config/` - Configuration-driven jobs

## Error Handling

The job will:
- Retry failed instances up to `backoff_limit`
- Fail the entire job if timeout is exceeded
- Continue running if some instances fail but enough succeed
- Clean up automatically after completion