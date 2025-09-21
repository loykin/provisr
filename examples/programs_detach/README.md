# Programs Directory + Detached Process + Store Recovery Demo

This example demonstrates how to:
- Load processes from a programs_directory
- Start a process in OS-level detached mode (Spec.detached = true)
- Enable the persistent store so the manager can recover and reattach after restart
- Verify that after killing the provisr daemon, the detached child keeps running
- Restart provisr and ensure it reattaches to the still-running process via PID file and store
- Stop the process and confirm it shuts down, with state persisted

## Prerequisites
- macOS/Linux or Windows (Unix demo script targets macOS/Linux)
- Built provisr binary at the repository root (run `go build -o provisr` from repo root)
- `curl` and `jq` installed for the demo script output (optional but recommended)

## Files
- config.toml — points to the `programs/` directory, enables the store (SQLite), and exposes HTTP API
- programs/detached-worker.toml — defines a long-running, detached process with a PID file
- demo.sh — runs the end-to-end scenario

## Run the demo

From the repository root:

```bash
# Build provisr if not already built
go build -o provisr

# Run the demo
bash examples/programs_detach/demo.sh
```

The script will:
1) Start `provisr serve` with this example config
2) Confirm the detached process is running via the API
3) Kill the provisr daemon (manager) and verify the child process keeps running using the PID file
4) Restart `provisr serve` with the same store; verify it reattaches (status shows running)
5) Stop the process through the API/CLI and verify it stops

All state is stored in the configured SQLite file so the manager can reload/reattach on restart.
