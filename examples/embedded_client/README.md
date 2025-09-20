# Embedded Client Example

This example demonstrates how to embed provisr client functionality in your own applications using the `pkg/client` package.

## Overview

The `pkg/client` package provides a clean, context-aware HTTP client for communicating with provisr daemon. This enables you to:

- **Start and stop processes** programmatically
- **Query process status** with flexible matching (name, base pattern, wildcard)  
- **Integrate provisr** into larger applications and workflows
- **Build custom UIs** for process management
- **Automate deployments** and process lifecycle management

## Prerequisites

1. **Provisr daemon must be running:**
   ```bash
   # Create a simple config file
   cat > daemon-config.toml << EOF
   [server]
   enabled = true
   listen = "127.0.0.1:8080"
   base_path = "/api"
   EOF
   
   # Start the daemon
   provisr serve daemon-config.toml
   ```

2. **Verify daemon is running:**
   ```bash
   curl http://localhost:8080/api/status?base=non-existent || echo "API responding"
   ```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/loykin/provisr/pkg/client"
)

func main() {
    // Create client with default configuration
    client := client.New(client.DefaultConfig())
    ctx := context.Background()
    
    // Check connectivity
    if !client.IsReachable(ctx) {
        log.Fatal("Cannot connect to provisr daemon")
    }
    
    // Start a process
    if err := client.StartProcess(ctx, client.StartRequest{
        Name:      "my-service",
        Command:   "python -m http.server 9000",
        Instances: 2,
        AutoRestart: true,
        Priority:  10,
        Environment: []string{
            "ENV=production",
            "PORT=9000",
        },
    }); err != nil {
        log.Fatal("Failed to start process:", err)
    }
    
    fmt.Println("✓ Process started successfully!")
}
```

## Complete Example

The `main.go` in this directory shows basic usage:

```bash
# Run the example (make sure daemon is running first)
go run main.go
```

## API Reference

### Client Creation

```go
// Default configuration (localhost:8080/api, 10s timeout)
client := client.New(client.DefaultConfig())

// Custom configuration
client := client.New(client.Config{
    BaseURL: "http://remote-host:8080/api",
    Timeout: 30 * time.Second,
})
```

### Starting Processes

```go
req := client.StartRequest{
    Name:            "web-server",           // Required: process name
    Command:         "python server.py",    // Required: command to run
    Instances:       3,                     // Number of instances (default: 1)
    WorkDir:         "/app",                // Working directory
    PIDFile:         "/tmp/web.pid",        // PID file location
    AutoRestart:     true,                  // Auto-restart on exit
    RestartInterval: 2 * time.Second,       // Delay between restarts
    Priority:        10,                    // Start priority (lower = earlier)
    Retries:         3,                     // Retry attempts on failure
    RetryInterval:   500 * time.Millisecond, // Delay between retries
    StartDuration:   1 * time.Second,       // Time to stay up to be considered "started"
    Environment:     []string{"ENV=prod"},  // Environment variables
}

err := client.StartProcess(ctx, req)
```

### Querying Status

**Note**: Status and Stop methods are not yet implemented in the current client. 
When implemented, they will work like this:

```go
// Get status by exact name
statuses, err := client.GetStatus(ctx, client.StatusQuery{
    Name: "web-server-1",  // Exact process name
})

// Get all instances matching a base name
statuses, err := client.GetStatus(ctx, client.StatusQuery{
    Base: "web-server",    // Matches web-server-1, web-server-2, etc.
})

// Get processes matching a wildcard pattern  
statuses, err := client.GetStatus(ctx, client.StatusQuery{
    Wildcard: "web-*",     // Matches web-server, web-proxy, etc.
})

// Process status information
for _, status := range statuses {
    fmt.Printf("Process: %s\n", status.Name)
    fmt.Printf("  Running: %t\n", status.Running)
    fmt.Printf("  PID: %d\n", status.PID)
    fmt.Printf("  Started: %v\n", status.StartedAt)
    if !status.Running {
        fmt.Printf("  Exit Code: %d\n", status.ExitCode)
        fmt.Printf("  Error: %s\n", status.Error)
    }
}
```

### Stopping Processes

```go
// Stop by exact name
err := client.StopProcess(ctx, client.StopRequest{
    Name: "web-server-1",
    Wait: 5 * time.Second,  // Grace period before force kill
})

// Stop all instances of a base name
err := client.StopProcess(ctx, client.StopRequest{
    Base: "web-server",
    Wait: 3 * time.Second,
})

// Stop by wildcard pattern
err := client.StopProcess(ctx, client.StopRequest{
    Wildcard: "temp-*",
    Wait: 1 * time.Second,
})
```

## Error Handling

The client provides detailed error information:

```go
if err := client.StartProcess(ctx, req); err != nil {
    switch {
    case strings.Contains(err.Error(), "API error:"):
        // Server returned an error response
        fmt.Printf("Server error: %v\n", err)
    case strings.Contains(err.Error(), "marshal request:"):
        // Invalid request data
        fmt.Printf("Request error: %v\n", err)
    case strings.Contains(err.Error(), "do request:"):
        // Network/connectivity error
        fmt.Printf("Network error: %v\n", err)
    default:
        fmt.Printf("Unknown error: %v\n", err)
    }
}
```

## Use Cases

### 1. Web Dashboard
```go
// Build a web interface for process management
func handleStartProcess(w http.ResponseWriter, r *http.Request) {
    var req client.StartRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), 400)
        return
    }
    
    if err := provisrClient.StartProcess(r.Context(), req); err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    w.WriteHeader(200)
}
```

### 2. CI/CD Integration
```go
// Deploy a new version in your deployment pipeline  
func deployNewVersion(version string) error {
    ctx := context.Background()
    
    // Start new version
    if err := client.StartProcess(ctx, client.StartRequest{
        Name:      fmt.Sprintf("app-%s", version),
        Command:   fmt.Sprintf("./app --version=%s", version),
        Instances: 3,
        Priority:  10,
    }); err != nil {
        return fmt.Errorf("failed to start new version: %w", err)
    }
    
    // Wait for health check, then stop old version...
    return nil
}
```

### 3. Process Monitoring
```go
// Monitor processes and send alerts
func monitorProcesses() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        statuses, err := client.GetStatus(ctx, client.StatusQuery{
            Wildcard: "critical-*",
        })
        if err != nil {
            log.Printf("Failed to get status: %v", err)
            continue
        }
        
        for _, status := range statuses {
            if !status.Running {
                sendAlert(fmt.Sprintf("Process %s is down!", status.Name))
            }
        }
    }
}
```

## Configuration Examples

### Remote Daemon
```go
config := client.Config{
    BaseURL: "http://production-server:8080/api",
    Timeout: 30 * time.Second,
}
```

### Development vs Production
```go
func newClient() *client.Client {
    config := client.DefaultConfig()
    
    if os.Getenv("ENV") == "production" {
        config.BaseURL = "http://provisr-prod:8080/api"
        config.Timeout = 60 * time.Second
    }
    
    return client.New(config)
}
```

## Structured Logging

This example demonstrates provisr's advanced structured logging capabilities using Go's `slog` package:

### Features

- **Colored Output**: Beautiful colored logs in terminal environments
- **Multiple Formats**: Support for both text and JSON formats
- **Configurable Levels**: Debug, Info, Warn, and Error levels
- **Source Information**: Optional file/line information for debugging
- **CI-Friendly**: Automatically disables colors in CI environments

### Logger Configuration

```go
loggerCfg := logger.LoggerConfig{
    Level:      logger.LevelInfo,     // debug, info, warn, error
    Format:     logger.FormatText,    // text, json
    Color:      true,                 // colorized output
    TimeStamps: true,                 // include timestamps
    Source:     false,                // include source file/line
}

slogger := logger.NewSlogger(loggerCfg)
slog.SetDefault(slogger)
```

### Client Integration

The provisr client supports structured logging:

```go
cfg := client.DefaultConfig()
cfg.Logger = slogger  // Use custom logger
provisrClient := client.New(cfg)
```

## Testing

```bash
# Start daemon in one terminal
provisr serve examples/embedded_client/daemon-config.toml

# Run the example in another terminal  
go run main.go

# Verify processes started (if you have curl)
curl -s 'http://localhost:8080/api/status?base=my-worker' | jq '.'
```

### CI/Automated Testing

This example is designed to run in CI environments:

- **CI Detection**: Automatically detects `CI=true` environment variable
- **Graceful Degradation**: Continues execution even if daemon is not available
- **Timeout Handling**: Uses shorter timeouts in CI environments
- **Automated Setup**: CI workflow automatically starts daemon before running the example

The CI workflow includes:
1. Building provisr binary
2. Starting daemon in background  
3. Waiting for daemon readiness
4. Running the embedded_client example
5. Cleaning up daemon process

## Troubleshooting

**"Provisr daemon not reachable"**
- Ensure daemon is running: `provisr serve daemon-config.toml`
- Check daemon logs for HTTP API configuration
- Verify daemon is listening on the expected port
- Check firewall/network connectivity for remote daemons

**"HTTP 404" errors**
- Verify the API base path in daemon configuration
- Check that HTTP API is enabled in daemon config

**Process start failures**
- Check command is valid and executable
- Verify working directory exists and is accessible
- Check environment variables and paths