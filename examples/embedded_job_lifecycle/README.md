# Job Lifecycle Hooks Example

This example demonstrates how to use lifecycle hooks with Provisr jobs to implement setup, validation, notification, and cleanup operations.

## What it demonstrates

- PreStart hooks for preparation and validation
- PostStart hooks for notifications
- PostStop hooks for cleanup and result handling
- Different hook execution modes (blocking/async)
- Various failure handling strategies
- Job status monitoring and lifecycle tracking

## Key Concepts

### Lifecycle Hooks

Jobs support four types of lifecycle hooks:

1. **PreStart**: Run before the main job starts
2. **PostStart**: Run after the main job starts (but doesn't wait for completion)
3. **PreStop**: Run before stopping a job
4. **PostStop**: Run after the main job completes or is stopped

### Hook Configuration

Each hook has configurable properties:

- **Name**: Identifier for the hook
- **Command**: Shell command to execute
- **FailureMode**: How to handle failures (`Fail`, `Retry`, `Ignore`)
- **RunMode**: Execution mode (`Blocking`, `Async`)

## Running the Example

```bash
cd examples/embedded_job_lifecycle
go run main.go
```

## Expected Output

```
=== Provisr Job Lifecycle Hooks Example ===

--- Starting data processing job ---
Downloading input data...
Validating input data...
Processing data...
Notifying team that job started...
Data processing completed
Uploading results...
Cleaning up workspace...
Notifying team that job completed...

--- Monitoring job progress ---
Job status: Running (Active: 1, Succeeded: 0, Failed: 0)
Job status: Running (Active: 1, Succeeded: 0, Failed: 0)
Job status: Succeeded (Active: 0, Succeeded: 1, Failed: 0)

=== Job Example completed ===
```

## Hook Flow

The example demonstrates a typical data processing job with hooks:

```
PreStart Hooks (Sequential):
├── download-data (blocking, fail-on-error)
└── validate-input (blocking, fail-on-error)
         ↓
Main Job: "data-processor"
         ↓
PostStart Hook (Concurrent):
└── notify-start (async, ignore-errors)
         ↓
PostStop Hooks (Sequential):
├── upload-results (blocking, retry-on-error)
├── cleanup-workspace (blocking, ignore-errors)
└── notify-completion (async, ignore-errors)
```

## Failure Modes

- **FailureModeFail**: Stop job execution if hook fails
- **FailureModeRetry**: Retry the hook on failure (configurable attempts)
- **FailureModeIgnore**: Continue job execution even if hook fails

## Run Modes

- **RunModeBlocking**: Wait for hook to complete before continuing
- **RunModeAsync**: Run hook in background, don't wait for completion

## Use Cases

Perfect for scenarios requiring:

- **Data Processing**: Download → Process → Upload → Cleanup
- **Service Deployment**: Validate → Deploy → Health Check → Notify
- **Backup Jobs**: Prepare → Backup → Verify → Cleanup
- **CI/CD Pipelines**: Setup → Build → Test → Deploy → Notify

## Advanced Features

- Hook execution is logged and monitored
- Failed hooks can trigger job failure or retries
- Async hooks don't block job progression
- Hook status is tracked separately from main job status

## Related Examples

- `../job_basic/` - Basic job management
- `../job_advanced/` - Advanced job features
- `../embedded_lifecycle_hooks/` - General process lifecycle hooks
- `../cronjob_basic/` - Scheduled jobs with hooks