# Embedded Manager Example

This example shows how to use the public provisr.Manager to manage processes using a unified TOML config, leveraging Manager.ApplyConfig to:

- recover running processes from PID files (when configured),
- start missing ones,
- and gracefully stop/remove programs not present in the config.

## Run

From the repository root:

```bash
go run ./examples/embedded_manager
```

The example will:

1) load the config at examples/embedded_manager/config/config.toml (fallback: ./config/config.toml),
2) apply the specs to the manager via ApplyConfig,
3) print current statuses as JSON.

You can run it multiple times; if any configured program uses a PID file, an already running instance will be recovered instead of started again.

## Config format

The example uses the unified configuration schema used across the project. Minimal snippet:

```toml
[log]
# Optional global log defaults (not required)

[metrics]
# Optional metrics section (not used by this example)

[[processes]]
type = "process"
[processes.spec]
name = "mgr-echo"
command = "sh -c 'echo from manager example; sleep 1'"

[[processes]]
type = "process"
[processes.spec]
name = "mgr-sleeper"
command = "sleep 2"
```

See the provided `config/config.toml` in this directory for a complete example.
