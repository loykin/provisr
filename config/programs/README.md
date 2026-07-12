# Programs directory

This directory contains process and CronJob definitions loaded by
`config/config.toml`. The daemon loads these files during startup and starts
the configured workloads in ascending priority order.

Start the server from the repository root:

```bash
./provisr serve config/config.toml
```

Use another terminal for process operations:

```bash
./provisr status --name long-sleeper
./provisr stop --name long-sleeper
./provisr start --name long-sleeper
./provisr group-status --group backend
```

When authentication is enabled, bootstrap or log in through `/ui` first so
the CLI has a saved session.

Supported program-file entity types are:

- `type = "process"` for long-running or finite processes.
- `type = "cronjob"` for scheduled Job templates.

Standalone `type = "job"` program files are not supported; create Jobs
through `/ui`, the Go API, or `POST /api/jobs`.

Process auto-restart, detector, logging, environment, lifecycle hook, and
priority settings belong under the file's `[spec]` table.
