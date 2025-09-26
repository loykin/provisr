package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	mng "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
	"github.com/loykin/provisr/internal/server"
)

// Custom middleware for authentication (example)
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token != "Bearer valid-token" {
			log.Printf("Auth middleware: Invalid token for %s %s", c.Request.Method, c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		log.Printf("Auth middleware: Valid token for %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
}

// Custom middleware for logging (example)
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Printf("Request: %s %s - Status: %d - Duration: %v",
			c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	}
}

// Rate limiting middleware (example)
func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple rate limiting logic (in production, use proper rate limiting)
		log.Printf("Rate limit middleware: Processing %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	mgr := mng.NewManager()
	base := os.Getenv("API_BASE")
	if base == "" {
		base = "/api"
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Create API endpoints for individual registration
	endpoints := server.NewAPIEndpoints(mgr, base)

	// Create API group
	apiGroup := r.Group(base)

	// Register endpoints individually with different middleware

	// Status endpoint - no authentication needed, just logging
	apiGroup.GET("/status",
		loggingMiddleware(),
		endpoints.StatusHandler())

	// Start endpoint - requires auth and rate limiting
	apiGroup.POST("/start",
		loggingMiddleware(),
		authMiddleware(),
		rateLimitMiddleware(),
		endpoints.StartHandler())

	// Stop endpoint - requires auth but no rate limiting
	apiGroup.POST("/stop",
		loggingMiddleware(),
		authMiddleware(),
		endpoints.StopHandler())

	// Register endpoint - requires all middleware
	apiGroup.POST("/register",
		loggingMiddleware(),
		authMiddleware(),
		rateLimitMiddleware(),
		endpoints.RegisterHandler())

	// Unregister endpoint - requires auth
	apiGroup.POST("/unregister",
		loggingMiddleware(),
		authMiddleware(),
		endpoints.UnregisterHandler())

	// Group operations - different middleware combinations
	groupGroup := apiGroup.Group("/group")
	{
		// Group status - no auth needed
		groupGroup.GET("/status",
			loggingMiddleware(),
			endpoints.GroupStatusHandler())

		// Group start/stop - requires auth
		groupGroup.POST("/start",
			loggingMiddleware(),
			authMiddleware(),
			endpoints.GroupStartHandler())

		groupGroup.POST("/stop",
			loggingMiddleware(),
			authMiddleware(),
			endpoints.GroupStopHandler())
	}

	// Debug endpoints - requires auth and special logging
	debugGroup := apiGroup.Group("/debug")
	debugGroup.Use(loggingMiddleware(), authMiddleware())
	{
		debugGroup.GET("/processes", endpoints.DebugProcessesHandler())
	}

	// Alternative approach: Register all endpoints with common middleware
	// and then add specific middleware to individual routes
	//
	// commonGroup := r.Group(base + "/v2")
	// commonGroup.Use(loggingMiddleware()) // All endpoints get logging
	// endpoints.RegisterAll(commonGroup)
	//
	// // Then add auth to specific endpoints
	// commonGroup.Use(authMiddleware()).POST("/register", endpoints.RegisterHandler())

	// Start a demo process so you can test the endpoints
	_ = mgr.RegisterN(process.Spec{
		Name:      "demo",
		Command:   "/bin/sh -c 'while true; do echo demo; sleep 5; done'",
		Instances: 2,
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Println("Starting Gin server on :8080 with individual API registration")
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
	log.Printf("  curl -H 'Authorization: Bearer valid-token' http://localhost:8080%s/debug/processes\n", base)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
