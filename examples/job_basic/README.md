# Basic Job Example

This example demonstrates how to create and monitor a batch Job through the Go API.

## What it demonstrates

- Creating Jobs with `JobManager`
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
Job created successfully!
Monitoring job status...
Iteration 1: phase=Running active=2 succeeded=0 failed=0
Iteration 2: phase=Succeeded active=0 succeeded=2 failed=0
Example completed
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

The standalone CLI has no Job-specific status or logs command. Inspect Jobs
through `/ui/jobs`, `GET /api/jobs`, or the `JobManager` methods demonstrated
by this example.

## Advanced Features

For more advanced job features, see:
- `../job_advanced/` - Advanced job configuration
- `../embedded_job_lifecycle/` - Job lifecycle hooks
- `../cronjob_basic/` - Scheduled jobs

## Error Handling

The job will:
- Retry failed instances up to `backoff_limit`
- Fail the entire job if timeout is exceeded
- Continue running if some instances fail but enough succeed
- Clean up automatically after completion
