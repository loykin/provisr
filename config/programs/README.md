# Programs Directory - Process Configurations

This directory contains TOML configuration files for individual processes managed by provisr.

## Architecture Overview

provisr uses a **daemon-first architecture**:

```
┌─────────────────┐    HTTP API    ┌──────────────────┐
│ ./provisr serve │ ◄─────────────► │ Background Tasks │
│   (Daemon)      │                │ - Health Check   │
│                 │                │ - AutoRestart    │
└─────────────────┘                │ - Reconciler     │
        ▲                          └──────────────────┘
        │ HTTP Client
        │
┌───────────────────────────────────────────────────────┐
│ CLI Commands (HTTP Clients)                          │
│ - ./provisr start  → POST /api/start                 │
│ - ./provisr stop   → POST /api/stop                  │
│ - ./provisr status → GET  /api/status                │
└───────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# 1. Start the daemon (required for all operations)
./provisr serve --api-listen :8080 --config config/config.toml &

# 2. Start processes (CLI communicates with daemon via HTTP API)
./provisr start --config config/config.toml

# 3. Monitor status in real-time
./provisr status --config config/config.toml

# 4. Check individual process logs  
tail -f /tmp/provisr-logs/*.log

# 5. Stop processes
./provisr stop --config config/config.toml
```

## Available Processes

| Process         | Priority | AutoRestart | Description              |
|-----------------|----------|-------------|--------------------------|
| **cleanup**     | 0        | ❌          | Demo cleanup task        |
| **long-sleeper** | 10       | ✅          | Long-running sleep test  |

## Key Features

### 🔄 **AutoRestart**
Processes with `autorestart = true` automatically restart when they die:

```toml
# config/programs/long-sleeper.toml
autorestart = true
restart_interval = "1s"

[[detectors]]
type = "command" 
command = "pgrep -f 'sleep 300'"
```

### 📊 **Priority Ordering**
Higher priority processes start first:

```toml
# cleanup.toml
priority = 0    # Starts first

# long-sleeper.toml  
priority = 10   # Starts second
```

### 🔍 **Process Detection**
Built-in detectors for health checking:

- **exec:pid** - Default PID-based detection
- **command** - Custom shell command detection

## Daemon Management

### Start Daemon
```bash
# Foreground (with logs)
./provisr serve --config config/config.toml --api-listen :8080

# Background (daemon mode)
nohup ./provisr serve --config config/config.toml --api-listen :8080 > serve.log 2>&1 &
```

### API Endpoints
- `GET /api/status` - Process status
- `POST /api/start` - Start process
- `POST /api/stop` - Stop process  
- `POST /api/debug/reconcile` - Manual reconciliation

## Testing AutoRestart

```bash
# Start daemon
./provisr serve --config config/config.toml --api-listen :8080 &

# Start process with autorestart
./provisr start --name test --cmd "sleep 300" --auto_restart true --config ""

# Kill process to test restart
PID=$(./provisr status --name test | jq -r '.pid')
kill -9 $PID

# Verify restart (should show new PID and restarts: 1)
sleep 3
./provisr status --name test
```

## Log Files

All process output goes to `/tmp/provisr-logs/`:

- `long-sleeper.log` - Long-running sleep process logs
- `cleanup.log` - Cleanup task output
- Custom processes create their own log files based on process name

## Compatibility

✅ **Cross-Platform**: Works on macOS, Linux, and Unix systems  
✅ **No Dependencies**: Uses only standard POSIX shell commands  
✅ **Self-Contained**: Creates own log directory and files  
✅ **Daemon Architecture**: Consistent process management through single daemon
