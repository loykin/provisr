# Individual API Registration Example (Echo Framework)

This example demonstrates how to register provisr HTTP API endpoints individually with the Echo web framework, including custom middleware for each endpoint.

## Features

- **Echo Framework Integration**: Shows how to use provisr APIs with Echo instead of Gin
- **Individual endpoint registration**: Each API endpoint registered separately
- **Per-endpoint middleware**: Different middleware applied to different endpoints
- **Gin-to-Echo adapter**: Converts Gin handlers to work with Echo
- **Custom authentication**: Example authentication middleware for Echo
- **Graceful shutdown**: Proper server shutdown handling

## API Endpoints with Middleware

### Public Endpoints (No Authentication)
- `GET /api/status` - Process status (logging only)
- `GET /api/group/status` - Group status (logging only)

### Protected Endpoints (Authentication Required)
- `POST /api/start` - Start process (auth + logging)
- `POST /api/stop` - Stop process (auth + logging)
- `POST /api/register` - Register process (auth + logging)
- `POST /api/unregister` - Unregister process (auth + logging)
- `POST /api/group/start` - Start group (auth + logging)
- `POST /api/group/stop` - Stop group (auth + logging)
- `GET /api/debug/processes` - Debug info (auth + logging)

## Running the Example

```bash
cd examples/embedded_http_echo_individual
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
```

## Implementation Details

### Gin-to-Echo Adapter

Since provisr's internal handlers are built for Gin, this example includes an adapter:

```go
func ginToEcho(ginHandler gin.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Create a Gin context from Echo context
        ginCtx := &gin.Context{
            Request: c.Request(),
            Writer:  &responseWriterAdapter{c.Response()},
        }
        ginHandler(ginCtx)
        return nil
    }
}
```

### Echo-Specific Middleware

```go
// Authentication middleware for Echo
func echoAuthMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            token := c.Request().Header.Get("Authorization")
            if token != "Bearer valid-token" {
                return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
            }
            return next(c)
        }
    }
}
```

### Individual Endpoint Registration

```go
endpoints := server.NewAPIEndpoints(mgr, base)
apiGroup := e.Group(base)

// Each endpoint can have different middleware
apiGroup.GET("/status",
    ginToEcho(endpoints.StatusHandler()),
    echoLoggingMiddleware())

apiGroup.POST("/start",
    ginToEcho(endpoints.StartHandler()),
    echoLoggingMiddleware(),
    echoAuthMiddleware())
```

### Graceful Shutdown

The example includes proper graceful shutdown handling:

```go
// Graceful shutdown
quit := make(chan os.Signal, 1)
signal.Notify(quit, os.Interrupt)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := e.Shutdown(ctx); err != nil {
    log.Fatal("Server forced to shutdown:", err)
}
```

## Comparison with Gin Example

- **Framework**: Uses Echo instead of Gin
- **Middleware**: Echo-specific middleware functions
- **Adapter**: Requires Gin-to-Echo adapter for handler compatibility
- **Shutdown**: Includes graceful shutdown (optional in Gin example)
- **Routing**: Echo's group-based routing (similar to Gin)

This approach allows Echo users to integrate provisr APIs with full control over middleware and routing while maintaining compatibility with the existing Gin-based handlers.