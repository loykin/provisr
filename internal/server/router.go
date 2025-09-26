package server

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	mgr      *mng.Manager
	basePath string
}

// NewRouter constructs a new Router with configurable basePath.
// Example basePath: "/abc" results in /abc/start, /abc/stop, /abc/status.
func NewRouter(mgr *mng.Manager, basePath string) *Router {
	bp := sanitizeBase(basePath)
	return &Router{mgr: mgr, basePath: bp}
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
	group.GET("/debug/processes", r.handleDebugProcesses)
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
