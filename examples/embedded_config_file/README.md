# Embedded config (file) example

This example shows how to start processes by loading a TOML configuration file using the public `provisr` wrapper.

Run it with:

```shell
go run ./examples/embedded_config_file
```

What it does:

- Loads environment and process specs from config/config.toml
- Applies global env
- Starts all listed processes (respecting instances)
- Prints JSON statuses grouped by base name
