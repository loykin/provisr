# Individual API Registration Example

This example demonstrates how to register provisr HTTP API endpoints individually with custom middleware for each endpoint.

## Features

- **Individual endpoint registration**: Each API endpoint can be registered separately
- **Per-endpoint middleware**: Different middleware can be applied to different endpoints
- **Custom authentication**: Example authentication middleware using Bearer tokens
- **Rate limiting**: Example rate limiting middleware
- **Flexible routing**: Endpoints can be grouped and organized as needed

## API Endpoints with Middleware

### Public Endpoints (No Authentication)
- `GET /api/status` - Process status (logging only)
- `GET /api/group/status` - Group status (logging only)

### Protected Endpoints (Authentication Required)
- `POST /api/start` - Start process (auth + rate limiting + logging)
- `POST /api/stop` - Stop process (auth + logging)
- `POST /api/register` - Register process (auth + rate limiting + logging)
- `POST /api/unregister` - Unregister process (auth + logging)
- `POST /api/group/start` - Start group (auth + logging)
- `POST /api/group/stop` - Stop group (auth + logging)
- `GET /api/debug/processes` - Debug info (auth + logging)

## Running the Example

```bash
cd examples/embedded_http_gin_individual
API_BASE=/api go run .
```

## Testing

### Public endpoints (no auth needed):
```bash
# Get process status
curl http://localhost:8080/api/status?name=demo

# Get group status
curl http://localhost:8080/api/group/status?group=test
```

### Protected endpoints (auth required):
```bash
# Start process (with auth)
curl -H 'Authorization: Bearer valid-token' \
     -X POST http://localhost:8080/api/start?name=demo

# Stop process (with auth)
curl -H 'Authorization: Bearer valid-token' \
     -X POST http://localhost:8080/api/stop?name=demo

# Register new process (with auth)
curl -H 'Authorization: Bearer valid-token' \
     -H 'Content-Type: application/json' \
     -X POST http://localhost:8080/api/register \
     -d '{"name":"test","command":"echo hello","instances":1}'

# Debug info (with auth)
curl -H 'Authorization: Bearer valid-token' \
     http://localhost:8080/api/debug/processes
```

### Invalid auth (should return 401):
```bash
curl -H 'Authorization: Bearer invalid-token' \
     -X POST http://localhost:8080/api/start?name=demo
```

## Implementation Details

### Individual Registration vs. Bulk Registration

```go
// Method 1: Individual registration with custom middleware
endpoints := server.NewAPIEndpoints(mgr, base)
apiGroup := r.Group(base)

// Each endpoint can have different middleware
apiGroup.GET("/status", loggingMiddleware(), endpoints.StatusHandler())
apiGroup.POST("/start", loggingMiddleware(), authMiddleware(), endpoints.StartHandler())

// Method 2: Bulk registration with common middleware
endpoints := server.NewAPIEndpoints(mgr, base)
apiGroup := r.Group(base)
apiGroup.Use(loggingMiddleware()) // Common middleware for all
endpoints.RegisterAll(apiGroup)   // Register all endpoints
```

### Custom Middleware Examples

The example includes several middleware functions:

- **Authentication**: Validates Bearer tokens
- **Logging**: Logs request details and timing
- **Rate Limiting**: Basic rate limiting (placeholder implementation)

### Flexible Grouping

Endpoints can be organized into logical groups:

```go
groupGroup := apiGroup.Group("/group")
groupGroup.Use(commonMiddleware())
{
    groupGroup.GET("/status", endpoints.GroupStatusHandler())
    groupGroup.POST("/start", endpoints.GroupStartHandler())
    groupGroup.POST("/stop", endpoints.GroupStopHandler())
}
```

This approach provides maximum flexibility for integrating provisr APIs into existing applications with specific security and operational requirements.