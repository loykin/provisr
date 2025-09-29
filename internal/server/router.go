package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/internal/auth"
	"github.com/loykin/provisr/internal/config"
	mng "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
	tlsutil "github.com/loykin/provisr/internal/tls"
)

// Router provides embeddable HTTP handlers for managing processes.
// Endpoints:
//   POST {basePath}/start        body: Spec JSON
//   POST {basePath}/stop         query: name=...&wait=1s (wait optional)
//   GET  {basePath}/status       query: name=... (instance) OR base=... (list)
// If both name and base are empty, returns 400.
// If base provided without name, returns list of statuses for base.
// If name provided, returns single status.
// basePath may be empty or start with '/'; no trailing slash.

type Router struct {
	mgr         *mng.Manager
	basePath    string
	authService *auth.AuthService
}

// APIEndpoints provides individual access to API handlers for custom registration
type APIEndpoints struct {
	mgr      *mng.Manager
	basePath string
}

// NewRouter constructs a new Router with configurable basePath.
// Example basePath: "/abc" results in /abc/start, /abc/stop, /abc/status.
func NewRouter(mgr *mng.Manager, basePath string) *Router {
	bp := sanitizeBase(basePath)
	return &Router{mgr: mgr, basePath: bp}
}

// NewAPIEndpoints constructs APIEndpoints for individual handler registration.
// This allows registering each API endpoint separately with custom middleware.
func NewAPIEndpoints(mgr *mng.Manager, basePath string) *APIEndpoints {
	bp := sanitizeBase(basePath)
	return &APIEndpoints{mgr: mgr, basePath: bp}
}

// Handler returns an http.Handler powered by gin that can be mounted in any server/mux.
func (r *Router) Handler() http.Handler {
	g := gin.New()
	g.Use(gin.Recovery())
	group := g.Group(r.basePath)
	group.POST("/register", r.handleRegister)
	group.POST("/start", r.handleStart)
	group.POST("/stop", r.handleStop)
	group.POST("/unregister", r.handleUnregister)
	group.GET("/status", r.handleStatus)
	group.GET("/group/status", r.handleGroupStatus)
	group.POST("/group/start", r.handleGroupStart)
	group.POST("/group/stop", r.handleGroupStop)
	group.GET("/debug/processes", r.handleDebugProcesses)
	group.GET("/metrics", r.handleProcessMetrics)
	group.GET("/metrics/history", r.handleProcessMetricsHistory)
	group.GET("/metrics/group", r.handleProcessMetricsGroup)

	// Add auth endpoints if auth service is available
	if r.authService != nil {
		authAPI := NewAuthAPI(r.authService)
		authAPI.RegisterAuthEndpoints(group)
	}

	return g
}

// NewServer starts a standalone HTTP server on addr using this router.
// The returned function can be called to shutdown the server immediately
// by closing the listener via http.Server's Close.
func NewServer(addr, basePath string, mgr *mng.Manager) (*http.Server, error) {
	r := NewRouter(mgr, basePath)
	server := &http.Server{
		Addr:              addr,
		Handler:           r.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start the server in a goroutine and handle potential errors
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	// Give the server a moment to start and catch immediate errors
	select {
	case err := <-serverErrCh:
		if err != nil {
			return nil, err
		}
	case <-time.After(100 * time.Millisecond):
		// Server started successfully or no immediate error
	}

	return server, nil
}

// NewTLSServer starts a standalone HTTPS server using TLS configuration.
// The returned function can be called to shutdown the server immediately
// by closing the listener via http.Server's Close.
func NewTLSServer(serverConfig config.ServerConfig, mgr *mng.Manager) (*http.Server, error) {
	r := NewRouter(mgr, serverConfig.BasePath)

	// Setup TLS configuration
	tlsConfig, err := tlsutil.SetupTLS(serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to setup TLS: %w", err)
	}

	server := &http.Server{
		Addr:              serverConfig.Listen,
		Handler:           r.Handler(),
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start the server in a goroutine and handle potential errors
	serverErrCh := make(chan error, 1)
	go func() {
		var err error
		if tlsConfig != nil {
			// Use HTTPS
			err = server.ListenAndServeTLS("", "")
		} else {
			// Use HTTP
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	// Give the server a moment to start and catch immediate errors
	select {
	case err := <-serverErrCh:
		if err != nil {
			return nil, err
		}
	case <-time.After(100 * time.Millisecond):
		// Server started successfully or no immediate error
	}

	return server, nil
}

// --- APIEndpoints Individual Handler Registration ---

// RegisterHandler returns the gin.HandlerFunc for process registration
func (e *APIEndpoints) RegisterHandler() gin.HandlerFunc {
	// Create a temporary router to reuse existing handler logic
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleRegister
}

// StartHandler returns the gin.HandlerFunc for starting processes
func (e *APIEndpoints) StartHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleStart
}

// StopHandler returns the gin.HandlerFunc for stopping processes
func (e *APIEndpoints) StopHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleStop
}

// StatusHandler returns the gin.HandlerFunc for getting process status
func (e *APIEndpoints) StatusHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleStatus
}

// UnregisterHandler returns the gin.HandlerFunc for unregistering processes
func (e *APIEndpoints) UnregisterHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleUnregister
}

// GroupStartHandler returns the gin.HandlerFunc for starting process groups
func (e *APIEndpoints) GroupStartHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleGroupStart
}

// GroupStopHandler returns the gin.HandlerFunc for stopping process groups
func (e *APIEndpoints) GroupStopHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleGroupStop
}

// GroupStatusHandler returns the gin.HandlerFunc for getting group status
func (e *APIEndpoints) GroupStatusHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleGroupStatus
}

// DebugProcessesHandler returns the gin.HandlerFunc for debug information
func (e *APIEndpoints) DebugProcessesHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleDebugProcesses
}

// ProcessMetricsHandler returns the gin.HandlerFunc for getting process metrics
func (e *APIEndpoints) ProcessMetricsHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleProcessMetrics
}

// ProcessMetricsHistoryHandler returns the gin.HandlerFunc for getting process metrics history
func (e *APIEndpoints) ProcessMetricsHistoryHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleProcessMetricsHistory
}

// ProcessMetricsGroupHandler returns the gin.HandlerFunc for getting group metrics
func (e *APIEndpoints) ProcessMetricsGroupHandler() gin.HandlerFunc {
	r := &Router{mgr: e.mgr, basePath: e.basePath}
	return r.handleProcessMetricsGroup
}

// RegisterAll registers all API endpoints to the provided gin router group
// This is equivalent to using the Router.Handler() but allows for custom middleware
func (e *APIEndpoints) RegisterAll(group *gin.RouterGroup) {
	group.POST("/register", e.RegisterHandler())
	group.POST("/start", e.StartHandler())
	group.POST("/stop", e.StopHandler())
	group.POST("/unregister", e.UnregisterHandler())
	group.GET("/status", e.StatusHandler())
	group.GET("/group/status", e.GroupStatusHandler())
	group.POST("/group/start", e.GroupStartHandler())
	group.POST("/group/stop", e.GroupStopHandler())
	group.GET("/debug/processes", e.DebugProcessesHandler())
	group.GET("/metrics", e.ProcessMetricsHandler())
	group.GET("/metrics/history", e.ProcessMetricsHistoryHandler())
	group.GET("/metrics/group", e.ProcessMetricsGroupHandler())
}

// --- Handlers ---

type errorResp struct {
	Error string `json:"error"`
}

type okResp struct {
	OK bool `json:"ok"`
}

// processSelector holds the parsed query parameters for process selection
type processSelector struct {
	name string
	base string
	wild string
	wait time.Duration
}

// parseProcessSelector extracts and validates process selector parameters from the request
func parseProcessSelector(c *gin.Context) (*processSelector, error) {
	name := c.Query("name")
	base := c.Query("base")
	wild := c.Query("wildcard")
	waitStr := c.Query("wait")
	wait := 2 * time.Second
	if waitStr != "" {
		if d, err := time.ParseDuration(waitStr); err == nil {
			wait = d
		}
	}

	// ensure exactly one selector is provided
	selCount := 0
	if name != "" {
		selCount++
	}
	if base != "" {
		selCount++
	}
	if wild != "" {
		selCount++
	}
	if selCount == 0 {
		return nil, fmt.Errorf("one of name, base, wildcard query param required")
	}
	if selCount > 1 {
		return nil, fmt.Errorf("exactly one of name, base, or wildcard must be provided")
	}

	// Validate process identifiers to avoid path traversal
	if name != "" && !isSafeName(name) {
		return nil, fmt.Errorf("invalid name: allowed [A-Za-z0-9._-] and no '..' or path separators")
	}
	if base != "" && !isSafeName(base) {
		return nil, fmt.Errorf("invalid base: allowed [A-Za-z0-9._-] and no '..' or path separators")
	}

	return &processSelector{
		name: name,
		base: base,
		wild: wild,
		wait: wait,
	}, nil
}

func (r *Router) handleRegister(c *gin.Context) {
	var spec process.Spec
	// ok: safe path checked
	if err := c.ShouldBindJSON(&spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid JSON: " + err.Error()})
		return
	}
	if spec.Name == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "spec.name required"})
		return
	}
	// Validate process name and any path-like fields to avoid uncontrolled path usage
	if !isSafeName(spec.Name) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid spec.name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return
	}
	if !isSafeAbsPath(spec.WorkDir) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid work_dir: must be absolute path without traversal"})
		return
	}
	if !isSafeAbsPath(spec.PIDFile) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid pid_file: must be absolute path without traversal"})
		return
	}
	if !isSafeAbsPath(spec.Log.File.Dir) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.dir: must be absolute path without traversal"})
		return
	}
	if !isSafeAbsPath(spec.Log.File.StdoutPath) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.stdoutPath: must be absolute path without traversal"})
		return
	}
	if !isSafeAbsPath(spec.Log.File.StderrPath) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.stderrPath: must be absolute path without traversal"})
		return
	}
	// ok: safe path checked
	if err := r.mgr.RegisterN(spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleStop(c *gin.Context) {
	selector, err := parseProcessSelector(c)
	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	if selector.base != "" {
		err = r.mgr.StopAll(selector.base, selector.wait)
	} else if selector.wild != "" {
		err = r.mgr.StopMatch(selector.wild, selector.wait)
	} else {
		// single process by name
		err = r.mgr.Stop(selector.name, selector.wait)
	}

	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleStatus(c *gin.Context) {
	name := c.Query("name")
	base := c.Query("base")
	wild := c.Query("wildcard")
	// ensure exactly one selector is provided
	selCount := 0
	if name != "" {
		selCount++
	}
	if base != "" {
		selCount++
	}
	if wild != "" {
		selCount++
	}
	if selCount == 0 {
		// readiness/health probe: no selector provided
		writeJSON(c, http.StatusOK, okResp{OK: true})
		return
	}
	if selCount > 1 {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "only one of name, base, wildcard must be provided"})
		return
	}
	if base != "" {
		sts, err := r.mgr.StatusAll(base)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
			return
		}
		writeJSON(c, http.StatusOK, sts)
		return
	}
	if wild != "" {
		sts, err := r.mgr.StatusMatch(wild)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
			return
		}
		writeJSON(c, http.StatusOK, sts)
		return
	}
	st, err := r.mgr.Status(name)
	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, st)
}

// Debug endpoints for troubleshooting

type debugProcessInfo struct {
	Status        process.Status `json:"status"`
	InternalState string         `json:"internal_state"`
	HealthCheck   string         `json:"health_check"`
}

func (r *Router) handleDebugProcesses(c *gin.Context) {
	// Get all processes with detailed debug information
	pattern := c.DefaultQuery("pattern", "*")

	statuses, err := r.mgr.StatusAll(pattern)
	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	debugInfos := make([]debugProcessInfo, len(statuses))
	for i, status := range statuses {
		debugInfos[i] = debugProcessInfo{
			Status:        status,
			InternalState: status.State, // Already includes state machine state
			HealthCheck:   getHealthStatus(status),
		}
	}

	writeJSON(c, http.StatusOK, debugInfos)
}

func getHealthStatus(status process.Status) string {
	if !status.Running {
		return "not_running"
	}

	if status.PID == 0 {
		return "no_pid"
	}

	if status.State != "running" {
		return "transitioning"
	}

	return "healthy"
}

func (r *Router) handleStart(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "name parameter required"})
		return
	}

	// Validate process name to avoid path traversal
	if !isSafeName(name) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return
	}

	// Start only existing registered process
	if err := r.mgr.Start(name); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleUnregister(c *gin.Context) {
	selector, err := parseProcessSelector(c)
	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	if selector.name != "" {
		err = r.mgr.Unregister(selector.name, selector.wait)
	} else if selector.base != "" {
		err = r.mgr.UnregisterAll(selector.base, selector.wait)
	} else {
		err = r.mgr.UnregisterMatch(selector.wild, selector.wait)
	}

	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleGroupStatus(c *gin.Context) {
	groupName := c.Query("group")
	if groupName == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "group parameter required"})
		return
	}

	// Validate group name to avoid path traversal
	if !isSafeName(groupName) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid group name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return
	}

	groupStatus, err := r.mgr.InstanceGroupStatus(groupName)
	if err != nil {
		writeJSON(c, http.StatusNotFound, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, groupStatus)
}

func (r *Router) handleGroupStart(c *gin.Context) {
	groupName := c.Query("group")
	if groupName == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "group parameter required"})
		return
	}

	// Validate group name to avoid path traversal
	if !isSafeName(groupName) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid group name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return
	}

	err := r.mgr.InstanceGroupStart(groupName)
	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleGroupStop(c *gin.Context) {
	groupName := c.Query("group")
	if groupName == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "group parameter required"})
		return
	}

	// Validate group name to avoid path traversal
	if !isSafeName(groupName) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid group name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return
	}

	// Parse wait parameter
	waitStr := c.Query("wait")
	wait := 3 * time.Second // default
	if waitStr != "" {
		if d, err := time.ParseDuration(waitStr); err == nil {
			wait = d
		}
	}

	err := r.mgr.InstanceGroupStop(groupName, wait)
	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleProcessMetrics(c *gin.Context) {
	name := c.Query("name")

	if name != "" {
		// Get metrics for a specific process
		if !isSafeName(name) {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
			return
		}

		metrics, found := r.mgr.GetProcessMetrics(name)
		if !found {
			writeJSON(c, http.StatusNotFound, errorResp{Error: "process not found or metrics not available"})
			return
		}

		writeJSON(c, http.StatusOK, metrics)
	} else {
		// Get metrics for all processes
		allMetrics := r.mgr.GetAllProcessMetrics()
		if !r.mgr.IsProcessMetricsEnabled() {
			writeJSON(c, http.StatusServiceUnavailable, errorResp{Error: "process metrics collection is disabled"})
			return
		}

		writeJSON(c, http.StatusOK, allMetrics)
	}
}

func (r *Router) handleProcessMetricsHistory(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "name parameter required"})
		return
	}

	// Validate process name to avoid path traversal
	if !isSafeName(name) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return
	}

	if !r.mgr.IsProcessMetricsEnabled() {
		writeJSON(c, http.StatusServiceUnavailable, errorResp{Error: "process metrics collection is disabled"})
		return
	}

	history, found := r.mgr.GetProcessMetricsHistory(name)
	if !found {
		writeJSON(c, http.StatusNotFound, errorResp{Error: "process not found or metrics history not available"})
		return
	}

	writeJSON(c, http.StatusOK, map[string]interface{}{
		"process": name,
		"history": history,
	})
}

func (r *Router) handleProcessMetricsGroup(c *gin.Context) {
	base := c.Query("base")
	if base == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "base parameter required"})
		return
	}

	// Validate base name to avoid path traversal
	if !isSafeName(base) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid base: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return
	}

	if !r.mgr.IsProcessMetricsEnabled() {
		writeJSON(c, http.StatusServiceUnavailable, errorResp{Error: "process metrics collection is disabled"})
		return
	}

	allMetrics := r.mgr.GetAllProcessMetrics()
	groupMetrics := make(map[string]interface{})
	var totalCPU float64
	var totalMemory float64
	var processCount int

	// Filter metrics for processes matching the base pattern
	for name, metrics := range allMetrics {
		// Check if this process belongs to the base group (e.g., demo-app-1, demo-app-2 belong to demo-app)
		if strings.HasPrefix(name, base+"-") || name == base {
			groupMetrics[name] = metrics
			totalCPU += metrics.CPUPercent
			totalMemory += metrics.MemoryMB
			processCount++
		}
	}

	if processCount == 0 {
		writeJSON(c, http.StatusNotFound, errorResp{Error: "no processes found for base pattern"})
		return
	}

	result := map[string]interface{}{
		"base":          base,
		"process_count": processCount,
		"total_cpu":     totalCPU,
		"total_memory":  totalMemory,
		"avg_cpu":       totalCPU / float64(processCount),
		"avg_memory":    totalMemory / float64(processCount),
		"processes":     groupMetrics,
	}

	writeJSON(c, http.StatusOK, result)
}
