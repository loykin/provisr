# Embedded logger example

This example demonstrates how to capture stdout and stderr for a process using provisr's built-in logging (
lumberjack-based rotation).

What it does:

- Creates a provisr.Manager
- Sets a logging directory (by default under your system temp dir)
- Starts a short command that writes to stdout and stderr
- Prints paths to the created log files

Run it with:

```shell
go run ./examples/embedded_logger
```

Optional:

- Set PROVISR_LOG_DIR to choose a custom log directory, e.g.
  PROVISR_LOG_DIR=/tmp/provisr-logs go run ./examples/embedded_logger

You should see files like:

- <logDir>/embedded-logger-demo.stdout.log
- <logDir>/embedded-logger-demo.stderr.log

These files will contain lines written by the demo process to its stdout and stderr, respectively.
