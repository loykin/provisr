# Programs directory example

This example keeps the main server configuration in `config.toml` and loads
individual TOML, JSON, YAML, and YML process definitions from `programs/`.

From this directory, start a daemon built from the repository:

```bash
../../provisr serve config.toml
```

The server listens on `127.0.0.1:8080` and exposes its UI at
`http://127.0.0.1:8080/ui`.

Use another terminal to inspect or control named processes and groups:

```bash
../../provisr status --name frontend
../../provisr stop --name frontend
../../provisr start --name frontend
../../provisr group-status --group web-services
../../provisr group-stop --group infrastructure
```

The example demonstrates:

- Multiple program-file formats.
- Ascending-priority startup.
- Multiple process instances.
- Auto-restart and detector configuration.
- Global and per-process environment variables.
- Process groups.
- Rotating logs below `/tmp/provisr-programs-demo`.

Configuration is validated when `serve` starts. There is no separate
`--dry-run` command.
