package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	mng "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/server"
)

// Custom middleware for authentication (Echo version)
func echoAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := c.Request().Header.Get("Authorization")
			if token != "Bearer valid-token" {
				log.Printf("Auth middleware: Invalid token for %s %s", c.Request().Method, c.Request().URL.Path)
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			log.Printf("Auth middleware: Valid token for %s %s", c.Request().Method, c.Request().URL.Path)
			return next(c)
		}
	}
}

// Custom middleware for request logging (Echo version)
func echoLoggingMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)
			log.Printf("Request: %s %s - Status: %d - Duration: %v",
				c.Request().Method, c.Request().URL.Path, c.Response().Status, duration)
			return err
		}
	}
}

// Adapter to convert Gin handlers to Echo handlers
func ginToEcho(ginHandler gin.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Create a Gin context from Echo context
		ginCtx := &gin.Context{
			Request: c.Request(),
			Writer:  &responseWriterAdapter{c.Response()},
		}

		// Set up basic Gin context properties
		ginCtx.Set("echo_context", c)

		// Call the Gin handler
		ginHandler(ginCtx)

		return nil
	}
}

// Adapter to make Echo ResponseWriter compatible with Gin
type responseWriterAdapter struct {
	*echo.Response
}

func (rwa *responseWriterAdapter) Status() int {
	return rwa.Response.Status
}

func (rwa *responseWriterAdapter) Size() int {
	return int(rwa.Response.Size)
}

func (rwa *responseWriterAdapter) WriteString(s string) (int, error) {
	return rwa.Response.Write([]byte(s))
}

func (rwa *responseWriterAdapter) Written() bool {
	return rwa.Response.Committed
}

func (rwa *responseWriterAdapter) WriteHeaderNow() {
	// Echo handles this automatically
}

func (rwa *responseWriterAdapter) Pusher() http.Pusher {
	return nil // HTTP/2 push not supported in this adapter
}

func (rwa *responseWriterAdapter) CloseNotify() <-chan bool {
	// Echo doesn't support CloseNotify, return a closed channel
	ch := make(chan bool, 1)
	close(ch)
	return ch
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	mgr := mng.NewManager()
	base := os.Getenv("API_BASE")
	if base == "" {
		base = "/api"
	}

	// Create Echo instance
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Create API endpoints for individual registration
	endpoints := server.NewAPIEndpoints(mgr, base)

	// Create API group
	apiGroup := e.Group(base)

	// Register endpoints individually with different middleware combinations

	// Status endpoint - no authentication needed, just logging
	apiGroup.GET("/status",
		ginToEcho(endpoints.StatusHandler()),
		echoLoggingMiddleware())

	// Start endpoint - requires auth and logging
	apiGroup.POST("/start",
		ginToEcho(endpoints.StartHandler()),
		echoLoggingMiddleware(),
		echoAuthMiddleware())

	// Stop endpoint - requires auth
	apiGroup.POST("/stop",
		ginToEcho(endpoints.StopHandler()),
		echoLoggingMiddleware(),
		echoAuthMiddleware())

	// Register endpoint - requires auth and logging
	apiGroup.POST("/register",
		ginToEcho(endpoints.RegisterHandler()),
		echoLoggingMiddleware(),
		echoAuthMiddleware())

	// Unregister endpoint - requires auth
	apiGroup.POST("/unregister",
		ginToEcho(endpoints.UnregisterHandler()),
		echoLoggingMiddleware(),
		echoAuthMiddleware())

	// Group operations
	groupGroup := apiGroup.Group("/group")
	{
		// Group status - no auth needed
		groupGroup.GET("/status",
			ginToEcho(endpoints.GroupStatusHandler()),
			echoLoggingMiddleware())

		// Group start/stop - requires auth
		groupGroup.POST("/start",
			ginToEcho(endpoints.GroupStartHandler()),
			echoLoggingMiddleware(),
			echoAuthMiddleware())

		groupGroup.POST("/stop",
			ginToEcho(endpoints.GroupStopHandler()),
			echoLoggingMiddleware(),
			echoAuthMiddleware())
	}

	// Debug endpoints - requires auth
	debugGroup := apiGroup.Group("/debug")
	debugGroup.Use(echoLoggingMiddleware(), echoAuthMiddleware())
	{
		debugGroup.GET("/processes", ginToEcho(endpoints.DebugProcessesHandler()))
	}

	// Start a demo process
	_ = mgr.RegisterN(process.Spec{
		Name:      "demo",
		Command:   "/bin/sh -c 'while true; do echo demo; sleep 5; done'",
		Instances: 2,
	})

	// Start server in a goroutine
	go func() {
		log.Println("Starting Echo server on :8080 with individual API registration")
		log.Println("Base path:", base)
		log.Println("")
		log.Println("Example requests:")
		log.Println("  # No auth needed:")
		log.Printf("  curl http://localhost:8080%s/status?name=demo\n", base)
		log.Printf("  curl http://localhost:8080%s/group/status?group=test\n", base)
		log.Println("")
		log.Println("  # Auth needed (use Authorization: Bearer valid-token):")
		log.Printf("  curl -H 'Authorization: Bearer valid-token' -X POST http://localhost:8080%s/start?name=demo\n", base)
		log.Printf("  curl -H 'Authorization: Bearer valid-token' -X POST http://localhost:8080%s/stop?name=demo\n", base)

		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			log.Fatal("shutting down the server:", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exited")
}
