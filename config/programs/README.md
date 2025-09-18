# Programs Directory Configuration

The `config/programs/` directory allows you to manage individual process configurations separately instead of defining all processes in the main `config.toml` file. Each process can have its own priority for startup ordering.

## Directory Structure

```
config/
├── config.toml          # Main configuration (global settings, groups, etc.)
└── programs/           # Individual process configurations
    ├── web.toml        # Web server process
    ├── worker.toml     # Worker process
    ├── external.toml   # External process
    └── cron-clean.toml # Cron job process
```

## Startup Priority

Processes are started in priority order (lower numbers start first):

- **priority = 0**: Cron jobs and utilities (cron-clean)
- **priority = 5**: Infrastructure services (external)
- **priority = 10**: Application services (web)
- **priority = 20**: Background workers (worker)

## Usage

### Main Configuration File (config.toml)

The main configuration file now contains:
- Global environment variables
- Global log defaults
- HTTP API configuration
- Store configuration
- History configuration
- Process groups definitions

### Individual Process Files (programs/*.toml)

Each process can have its own configuration file in `config/programs/`. The file name should be descriptive (e.g., `web.toml`, `api-server.toml`, `database-backup.toml`).

Example process configuration (`config/programs/web.toml`):

```toml
# Web server process configuration
name = "web"
command = "python3 -m http.server 8080"
workdir = "/tmp"
env = ["ENV=prod", "PORT=8080"]
pidfile = "/tmp/web.pid"
priority = 10  # Start after infrastructure (lower numbers start first)
retries = 3
retry_interval = "500ms"
startsecs = "2s"
autorestart = true
restart_interval = "1s"
instances = 3

# Per-process logging configuration
[log]
stdout = "/tmp/provisr-logs/web.stdout.log"
stderr = "/tmp/provisr-logs/web.stderr.log"

# Process detectors
[[detectors]]
type = "pidfile"
path = "/tmp/web.pid"

[[detectors]]
type = "command"
command = "pgrep -f 'python3 -m http.server 8080' >/dev/null"
```

## Benefits

1. **Better Organization**: Each process has its own configuration file
2. **Easier Management**: Add, remove, or modify processes without affecting others
3. **Version Control**: Changes to individual processes can be tracked separately
4. **Team Collaboration**: Different team members can work on different processes
5. **Modularity**: Process configurations can be shared across environments

## Backward Compatibility

The system maintains backward compatibility:
- If processes are defined in the main `config.toml`, they will be loaded
- If processes are defined in `config/programs/*.toml`, they will also be loaded
- Both sources are merged together (though duplicates should be avoided)

## Migration Guide

To migrate from a single `config.toml` to the programs directory structure:

1. Create the `config/programs/` directory
2. For each process in your `[[processes]]` section:
   - Create a new file `config/programs/{process-name}.toml`
   - Move the process configuration to this file
   - Remove the `[[processes]]` wrapper (the file itself represents one process)
3. Remove the `[[processes]]` sections from the main `config.toml`
4. Keep global settings, groups, and other configurations in the main file

The configuration loader will automatically scan the `programs/` directory and load all `.toml` files as individual process configurations.