# Embedded config (structure) example

This example demonstrates constructing a configuration in Go structs and launching a process without reading from a
file.

Run it with:

- go run ./example/embedded_config_structure

What it does:

- Builds a minimal internal/config.FileConfig with one process
- Converts the entry to a provisr.Spec in code
- Applies top-level env and starts the process
- Prints JSON status of the started process
