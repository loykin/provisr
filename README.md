# provisr

A minimal supervisord-like process manager written in Go.

Features:
- Start/stop/status for processes and multiple instances
- Auto-restart, retry with interval, start duration window
- Pluggable detectors (pidfile, pid, command)
- Logging to rotating files via lumberjack
- Cron-like scheduler (@every duration)
- Process groups (start/stop/status together)
- Config via TOML (Cobra + Viper)
- Embeddable public facade (see examples)

CLI usage examples:
- provisr start --name demo --cmd "sleep 10"
- provisr status --name demo
- provisr stop --name demo
- provisr start --config config/config.toml
- provisr cron --config config/config.toml
- provisr group-start --config config/config.toml --group backend

Examples:
- Embedded single process: example/embeded
- Embedded process group: example/embeded_process_group
