# Programs Directory Example

This example demonstrates how to use provisr with a **programs directory** approach, where process definitions are separated from the main configuration file.

## Structure

```
config.toml              # Main config with global settings and groups
programs/                # Directory containing individual process definitions
├── frontend.toml        # TOML format process definition
├── api-server.json      # JSON format process definition  
├── worker.yaml          # YAML format process definition
├── scheduler.yml        # YML format process definition
├── database.yml         # Database service definition
└── cache-redis.json     # Cache service definition
```

## Features Demonstrated

- **Multiple file formats**: TOML, JSON, YAML, and YML are all supported
- **Priority-based startup**: Processes start in order of priority (lower numbers first)
- **Process groups**: Logical grouping of related processes for batch operations
- **Multiple instances**: Some processes run multiple instances
- **Auto-restart**: Configurable restart behavior for long-running services
- **Mixed process types**: Both auto-terminating jobs (workers) and long-running services
- **Structured logging**: Each process has its own log directory
- **Environment variables**: Global and per-process environment settings

## Running the Example

1. **Start the daemon**:
   ```bash
   provisr serve config.toml
   ```

2. **Check status**:
   ```bash
   provisr status --api-url http://127.0.0.1:8080/api
   ```

3. **Start all processes**:
   ```bash
   provisr start --config config.toml
   ```

4. **Check group status**:
   ```bash
   provisr group-status --config config.toml --group web-services
   provisr group-status --config config.toml --group background-tasks
   provisr group-status --config config.toml --group infrastructure
   ```

6. **Run the demo**:
   ```bash
   # Interactive demonstration of all features
   ./demo.sh
   ```
7. **Stop processes**:
   ```bash
   provisr stop --config config.toml --name frontend
   provisr group-stop --config config.toml --group infrastructure
   provisr group-stop --config config.toml --group background-tasks
   ```

## Process Details

### Frontend (frontend.toml)
- Simulates a web frontend server on port 3000
- Priority: 10 (starts after API server)
- Auto-restarts on failure
- Single instance

### API Server (api-server.json) 
- Simulates a REST API server
- Priority: 5 (starts first)
- Runs 2 instances for load balancing
- Auto-restarts on failure

### Worker (worker.yaml)
- Background job processor
- Priority: 20 (starts last)
- Runs 3 worker instances
- Processes 60 jobs then exits (no auto-restart)

### Scheduler (scheduler.yml)
- Task scheduler service
- Priority: 15 (starts before workers)
- Single instance with auto-restart
- Ticks every 10 seconds

### Database (database.yml)  
- Database service simulation
- Priority: 1 (starts first - infrastructure)
- Single instance with auto-restart
- Long-running with connection simulation

### Cache Redis (cache-redis.json)
- Redis cache simulation  
- Priority: 2 (starts after database)
- Single instance with auto-restart
- Memory caching simulation

## Log Files

All logs are written to `/tmp/provisr-programs-demo/` with separate directories per service:

```
/tmp/provisr-programs-demo/
├── frontend/
│   ├── frontend.out.log
│   └── frontend.err.log
├── api/
│   ├── api.out.log
│   └── api.err.log
├── worker/
│   ├── worker.out.log
│   └── worker.err.log
├── scheduler/
│   ├── scheduler.out.log
│   └── scheduler.err.log
├── database/
│   ├── database.out.log
│   └── database.err.log
└── redis/
    ├── redis.out.log
    └── redis.err.log
```

## Testing Configuration

You can validate the configuration without starting processes:

```bash
# Test config loading
provisr start --config config.toml --dry-run

# Check what processes would be loaded
provisr status --config config.toml
```