# Unified Logging Demo

This example demonstrates the unified logging configuration in provisr using the standard library slog as the foundation.

What it shows:
- One configuration struct (internal/logger.Config) drives both:
  - Structured application logging (slog)
  - Process file logging (rotating files via lumberjack)
- Consistent, colored, structured console output
- How to obtain a process-file logger from the same config

## Run

```bash
cd examples/unified_logging_demo
go run .
```

Expected behavior:
- Console shows structured, colored log lines produced via slog
- A process-oriented logger writes to files under /tmp/provisr-unified-demo (created for the demo)
- The demo cleans up the temporary directory before exit

If you want to keep the log files for inspection, comment out the cleanup block in main.go near the end.

## Key API

- logger.Config: High-level config for both slog (console) and file logging
- logger.Config.NewSlogger(): returns a *slog.Logger configured as requested
- logger.Config.NewProcessLogger(name): returns a *slog.Logger that writes to files for the given process name

## Notes

- slog is now the standard logging backend across provisr
- Process Spec.Log uses the same internal/logger.Config, so application and managed processes share a consistent logging story
