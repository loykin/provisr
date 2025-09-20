# Embedded config file example

This example shows how to start processes by loading a TOML configuration file with programs defined in separate files using the public `provisr` wrapper.

## Structure

```
config/
  config.toml          # Main configuration (global settings, server config)
  programs/            # Individual process definitions
    web.toml          # Web server process
    worker.toml       # Background worker process  
    cron-clean.toml   # Cron job process
```

## Run it

```shell
go run ./examples/embedded_config_file
```

## What it does

- Loads global configuration from `config/config.toml`
- Automatically loads individual process definitions from `config/programs/` directory
- Applies global environment variables to all processes
- Starts all processes with their specific configurations
- Supports TOML, JSON, and YAML formats for process files
- Prints JSON statuses grouped by base name

## Key features demonstrated

- **Programs directory**: Clean separation of process definitions
- **Mixed formats**: Different processes can use different file formats
- **Global environment**: Shared environment variables
- **Process-specific settings**: Each process has its own configuration file
- **Detectors**: Both PID file and command-based process detection
