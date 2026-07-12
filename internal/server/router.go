package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/core"
	corehistory "github.com/loykin/provisr/core/history"
	"github.com/loykin/provisr/internal/auth"
	"github.com/loykin/provisr/internal/config"
	tlsutil "github.com/loykin/provisr/internal/tls"
	"github.com/loykin/provisr/internal/ui"
	apiwire "github.com/loykin/provisr/pkg/api"
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
	mgr           *core.Manager
	basePath      string
	authService   *auth.AuthService
	historyReader corehistory.Reader
	programsDir   string
	cronScheduler *core.CronScheduler
	jobManager    *core.JobManager
}

// APIEndpoints provides individual access to API handlers for custom registration
type APIEndpoints struct {
	mgr      *core.Manager
	basePath string
}

// NewRouter constructs a new Router with configurable basePath.
// Example basePath: "/abc" results in /abc/start, /abc/stop, /abc/status.
func NewRouter(mgr *core.Manager, basePath string) *Router {
	bp := sanitizeBase(basePath)
	return &Router{mgr: mgr, basePath: bp, jobManager: core.NewJobManager(mgr)}
}

// SetHistoryReader attaches a backend-neutral history reader to the Router.
// Adapter construction and lifetime belong to the composition root.
func (r *Router) SetHistoryReader(reader corehistory.Reader) { r.historyReader = reader }

// newRouterFromConfig constructs a Router and wires up an AuthService
// (if authCfg is present and enabled) and a history reader (if historyCfg
// enables in-store history) so their endpoints are mounted by Handler().
func newRouterFromConfig(mgr *core.Manager, basePath string, authCfg *config.AuthConfig, programsDir string, cronScheduler *core.CronScheduler, historyReader corehistory.Reader) (*Router, error) {
	r := NewRouter(mgr, basePath)
	r.programsDir = programsDir
	r.cronScheduler = cronScheduler
	if cronScheduler != nil {
		r.jobManager = cronScheduler.JobManager()
	}
	r.historyReader = historyReader

	if authCfg == nil || !authCfg.Enabled {
		return r, nil
	}

	authService, err := auth.NewAuthService(auth.AuthConfig{
		Store: auth.StoreConfig{
			Type:         authCfg.Store.Type,
			Migrate:      authCfg.Store.Migrate,
			Path:         authCfg.Store.Path,
			Host:         authCfg.Store.Host,
			Port:         authCfg.Store.Port,
			Database:     authCfg.Store.Database,
			Username:     authCfg.Store.Username,
			Password:     authCfg.Store.Password,
			SSLMode:      authCfg.Store.SSLMode,
			MaxOpenConns: authCfg.Store.MaxOpenConns,
			MaxIdleConns: authCfg.Store.MaxIdleConns,
		},
		JWTSecret:  authCfg.JWTSecret,
		TokenTTL:   authCfg.TokenTTL,
		BcryptCost: authCfg.BcryptCost,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth service: %w", err)
	}

	r.authService = authService
	return r, nil
}

// NewAPIEndpoints constructs APIEndpoints for individual handler registration.
// This allows registering each API endpoint separately with custom middleware.
func NewAPIEndpoints(mgr *core.Manager, basePath string) *APIEndpoints {
	bp := sanitizeBase(basePath)
	return &APIEndpoints{mgr: mgr, basePath: bp}
}

// noopMiddleware passes every request through unchanged; used when no
// AuthService is configured so process routes behave exactly as before.
func noopMiddleware(c *gin.Context) { c.Next() }

// Handler returns an http.Handler powered by gin that can be mounted in any server/mux.
func (r *Router) Handler() http.Handler {
	g := gin.New()
	g.Use(gin.Recovery())
	group := g.Group(r.basePath)

	authGin := gin.HandlerFunc(noopMiddleware)
	readPerm := gin.HandlerFunc(noopMiddleware)
	writePerm := gin.HandlerFunc(noopMiddleware)
	if r.authService != nil {
		mw := auth.NewMiddleware(r.authService, true)
		authGin = mw.GinAuth()
		readPerm = mw.GinRequirePermission("process", "read")
		writePerm = mw.GinRequirePermission("process", "write")
	}

	group.POST("/register", authGin, writePerm, r.handleRegister)
	group.POST("/update", authGin, writePerm, r.handleUpdate)
	group.POST("/start", authGin, writePerm, r.handleStart)
	group.POST("/stop", authGin, writePerm, r.handleStop)
	group.POST("/unregister", authGin, writePerm, r.handleUnregister)
	group.GET("/status", authGin, readPerm, r.handleStatus)
	group.GET("/group/status", authGin, readPerm, r.handleGroupStatus)
	group.POST("/group/start", authGin, writePerm, r.handleGroupStart)
	group.POST("/group/stop", authGin, writePerm, r.handleGroupStop)
	group.GET("/debug/processes", authGin, readPerm, r.handleDebugProcesses)
	group.GET("/metrics", authGin, readPerm, r.handleProcessMetrics)
	group.GET("/metrics/history", authGin, readPerm, r.handleProcessMetricsHistory)
	group.GET("/metrics/group", authGin, readPerm, r.handleProcessMetricsGroup)
	group.GET("/processes/:name/logs", authGin, readPerm, r.handleProcessLogs)
	group.GET("/processes/:name/spec", authGin, readPerm, r.handleGetSpec)

	// Add history endpoint if a history reader is available
	if r.historyReader != nil {
		group.GET("/history", authGin, readPerm, r.handleHistory)
	}

	jobReadPerm := gin.HandlerFunc(noopMiddleware)
	jobWritePerm := gin.HandlerFunc(noopMiddleware)
	if r.authService != nil {
		mw := auth.NewMiddleware(r.authService, true)
		jobReadPerm = mw.GinRequirePermission("job", "read")
		jobWritePerm = mw.GinRequirePermission("job", "write")
	}

	if r.jobManager != nil {
		group.GET("/jobs", authGin, jobReadPerm, r.handleListJobs)
		group.POST("/jobs", authGin, jobWritePerm, r.handleCreateJob)
		group.GET("/jobs/:name", authGin, jobReadPerm, r.handleGetJob)
		group.POST("/jobs/:name", authGin, jobWritePerm, r.handleUpdateJob)
		group.DELETE("/jobs/:name", authGin, jobWritePerm, r.handleDeleteJob)
	}

	// Add cronjob endpoints if a scheduler is available.
	if r.cronScheduler != nil {
		group.GET("/cronjobs", authGin, jobReadPerm, r.handleListCronJobs)
		group.POST("/cronjobs", authGin, jobWritePerm, r.handleCreateCronJob)
		group.GET("/cronjobs/:name", authGin, jobReadPerm, r.handleGetCronJob)
		group.POST("/cronjobs/:name", authGin, jobWritePerm, r.handleUpdateCronJob)
		group.DELETE("/cronjobs/:name", authGin, jobWritePerm, r.handleDeleteCronJob)
		group.GET("/cronjobs/:name/history", authGin, jobReadPerm, r.handleCronJobHistory)
		group.POST("/cronjobs/:name/suspend", authGin, jobWritePerm, r.handleSuspendCronJob)
		group.POST("/cronjobs/:name/resume", authGin, jobWritePerm, r.handleResumeCronJob)
		group.POST("/cronjobs/:name/trigger", authGin, jobWritePerm, r.handleTriggerCronJob)
	}

	// Unauthenticated, always-mounted: lets the UI tell whether it should
	// show a login gate at all. When auth is disabled, every other endpoint
	// above is already wide open (noopMiddleware), so the UI must skip
	// login entirely rather than get stuck on a login form that has no
	// backing /auth/login route to submit to.
	group.GET("/auth/status", func(c *gin.Context) {
		needsSetup := false
		if r.authService != nil {
			hasUsers, err := r.authService.HasAnyUsers(c.Request.Context())
			needsSetup = err == nil && !hasUsers
		}
		writeJSON(c, http.StatusOK, gin.H{"enabled": r.authService != nil, "needs_setup": needsSetup})
	})

	// Add auth endpoints if auth service is available. /login, /status, and
	// /bootstrap stay unauthenticated by design (RegisterAuthEndpoints keeps
	// them off these middlewares); /users* requires an admin token —
	// previously these were mounted on the bare group with no middleware at
	// all, so anyone could create their own admin user without a token.
	if r.authService != nil {
		mw := auth.NewMiddleware(r.authService, true)
		authAPI := NewAuthAPI(r.authService)
		authAPI.RegisterAuthEndpoints(group, authGin,
			mw.GinRequirePermission("user", "read"), mw.GinRequirePermission("user", "write"))
	}

	// Serve the embedded web UI (built via `make ui`) at /ui, single binary.
	uiHandler := http.StripPrefix("/ui", ui.Handler())
	g.GET("/ui", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/ui/") })
	g.Any("/ui/*proxyPath", gin.WrapH(uiHandler))

	return g
}

// NewServer starts a standalone HTTP server using this router.
// The returned function can be called to shutdown the server immediately
// by closing the listener via http.Server's Close.
func NewServer(serverConfig config.ServerConfig, mgr *core.Manager, cronScheduler *core.CronScheduler) (*http.Server, error) {
	return NewServerWithHistoryReader(serverConfig, mgr, cronScheduler, nil, "")
}

// NewServerWithHistoryReader starts an HTTP server with a history reader
// supplied by the composition root.
func NewServerWithHistoryReader(serverConfig config.ServerConfig, mgr *core.Manager, cronScheduler *core.CronScheduler, historyReader corehistory.Reader, programsDirectory string) (*http.Server, error) {
	r, err := newRouterFromConfig(mgr, serverConfig.BasePath, serverConfig.Auth, programsDirectory, cronScheduler, historyReader)
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		Addr:              serverConfig.Listen,
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

// NewTLSServerWithHistoryReader is the TLS equivalent of
// NewServerWithHistoryReader.
func NewTLSServerWithHistoryReader(serverConfig config.ServerConfig, mgr *core.Manager, cronScheduler *core.CronScheduler, historyReader corehistory.Reader, programsDirectory string) (*http.Server, error) {
	r, err := newRouterFromConfig(mgr, serverConfig.BasePath, serverConfig.Auth, programsDirectory, cronScheduler, historyReader)
	if err != nil {
		return nil, err
	}

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

type errorResp = apiwire.ErrorResponse

type okResp = apiwire.OKResponse

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

// bindAndValidateSpec decodes a core.Spec from the request body and validates
// its name and any path-like fields to avoid uncontrolled path usage. On
// failure it writes the error response itself and returns ok=false.
func bindAndValidateSpec(c *gin.Context) (spec core.Spec, ok bool) {
	if err := c.ShouldBindJSON(&spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid JSON: " + err.Error()})
		return spec, false
	}
	if spec.Name == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "spec.name required"})
		return spec, false
	}
	// Validate process name and any path-like fields to avoid uncontrolled path usage
	if !isSafeName(spec.Name) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid spec.name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return spec, false
	}
	if !isSafeAbsPath(spec.WorkDir) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid work_dir: must be absolute path without traversal"})
		return spec, false
	}
	if !isSafeAbsPath(spec.PIDFile) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid pid_file: must be absolute path without traversal"})
		return spec, false
	}
	if !isSafeAbsPath(spec.Log.File.Dir) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.dir: must be absolute path without traversal"})
		return spec, false
	}
	if !isSafeAbsPath(spec.Log.File.StdoutPath) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.stdoutPath: must be absolute path without traversal"})
		return spec, false
	}
	if !isSafeAbsPath(spec.Log.File.StderrPath) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.stderrPath: must be absolute path without traversal"})
		return spec, false
	}
	return spec, true
}

// writeProgramFile writes spec as a program file (the same discriminated-
// union {type, spec} shape `loadProgramEntries` reads at boot) so a process
// or cronjob registered/updated via the HTTP API survives a daemon restart,
// the same way `provisr register`'s program files do. A no-op if no
// programs directory is configured.
func (r *Router) writeProgramFile(name, kind string, spec any) error {
	if r.programsDir == "" {
		return nil
	}
	if err := os.MkdirAll(r.programsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create programs directory: %w", err)
	}
	doc := map[string]any{"type": kind, "spec": spec}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal program file: %w", err)
	}
	path := filepath.Join(r.programsDir, name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write program file: %w", err)
	}
	return nil
}

// persistProgramFile writes a process spec as a program file. See writeProgramFile.
func (r *Router) persistProgramFile(spec core.Spec) error {
	return r.writeProgramFile(spec.Name, "process", spec)
}

// persistCronJobFile writes a cronjob spec as a program file. See writeProgramFile.
func (r *Router) persistCronJobFile(spec core.CronJob) error {
	return r.writeProgramFile(spec.Name, "cronjob", spec)
}

func (r *Router) handleRegister(c *gin.Context) {
	spec, ok := bindAndValidateSpec(c)
	if !ok {
		return
	}
	// Persist before registering: a filesystem error here should not leave a
	// process running that a restart would then fail to recreate silently.
	if err := r.persistProgramFile(spec); err != nil {
		writeJSON(c, http.StatusInternalServerError, errorResp{Error: err.Error()})
		return
	}
	if err := r.mgr.RegisterN(spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// handleUpdate replaces the spec of an already-registered process and
// restarts it immediately under the new spec. query: wait=1s (optional).
func (r *Router) handleUpdate(c *gin.Context) {
	spec, ok := bindAndValidateSpec(c)
	if !ok {
		return
	}
	wait := 5 * time.Second
	if w := c.Query("wait"); w != "" {
		d, err := time.ParseDuration(w)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid wait duration: " + err.Error()})
			return
		}
		wait = d
	}
	if err := r.mgr.Update(spec, wait); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	if err := r.persistProgramFile(spec); err != nil {
		writeJSON(c, http.StatusInternalServerError, errorResp{Error: err.Error()})
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
	Status        core.Status `json:"status"`
	InternalState string      `json:"internal_state"`
	HealthCheck   string      `json:"health_check"`
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

// historyResp wraps a page of history rows with the total row count so
// callers can compute page counts without a separate request.
type historyResp = apiwire.HistoryResponse

// handleHistory returns recorded process lifecycle events (start/stop), newest
// first. Query params: name (optional, filters to one process), limit
// (optional, default 100, max 500), offset (optional, default 0).
func (r *Router) handleHistory(c *gin.Context) {
	name := c.Query("name")
	limit := 100
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: "limit must be a number"})
			return
		}
		limit = n
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: "offset must be a number"})
			return
		}
		offset = n
	}

	rows, err := r.historyReader.List(c.Request.Context(), name, limit, offset)
	if err != nil {
		writeJSON(c, http.StatusInternalServerError, errorResp{Error: err.Error()})
		return
	}
	total, err := r.historyReader.Count(c.Request.Context(), name)
	if err != nil {
		writeJSON(c, http.StatusInternalServerError, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, historyResp{Rows: rows, Total: total})
}

// logsSinceResp is the response body for the live-tail polling endpoint.
type logsSinceResp struct {
	Lines []core.LogLine `json:"lines"`
	Next  uint64         `json:"next"`
}

// handleProcessLogs returns captured stdout/stderr lines for a process
// since the given offset, for polling-based live tail. Query params:
// since (optional, default 0), limit (optional, default 200, max 1000).
func (r *Router) handleProcessLogs(c *gin.Context) {
	name := c.Param("name")

	var since uint64
	if v := c.Query("since"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: "since must be a non-negative number"})
			return
		}
		since = n
	}

	limit := 200
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: "limit must be a number"})
			return
		}
		limit = n
	}
	if limit > 1000 {
		limit = 1000
	}

	lines, next, err := r.mgr.LogsSince(name, since, limit)
	if err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, logsSinceResp{Lines: lines, Next: next})
}

// handleGetSpec returns the currently-registered spec for a process, e.g. so
// a UI can prefill an edit form before calling POST /update.
func (r *Router) handleGetSpec(c *gin.Context) {
	name := c.Param("name")

	spec, err := r.mgr.GetSpec(name)
	if err != nil {
		writeJSON(c, http.StatusNotFound, errorResp{Error: err.Error()})
		return
	}

	writeJSON(c, http.StatusOK, spec)
}

type jobResp struct {
	core.JobSpec
	Status core.JobStatus `json:"status"`
}

func (r *Router) jobResponse(name string) (jobResp, bool) {
	spec, ok := r.jobManager.GetJobSpec(name)
	if !ok {
		return jobResp{}, false
	}
	status, _ := r.jobManager.GetJob(name)
	return jobResp{JobSpec: spec, Status: status}, true
}

func (r *Router) handleListJobs(c *gin.Context) {
	specs := r.jobManager.ListJobSpecs()
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)

	resp := make([]jobResp, 0, len(names))
	for _, name := range names {
		if job, ok := r.jobResponse(name); ok {
			resp = append(resp, job)
		}
	}
	writeJSON(c, http.StatusOK, resp)
}

func (r *Router) handleGetJob(c *gin.Context) {
	name := c.Param("name")
	job, ok := r.jobResponse(name)
	if !ok {
		writeJSON(c, http.StatusNotFound, errorResp{Error: fmt.Sprintf("job %q not found", name)})
		return
	}
	writeJSON(c, http.StatusOK, job)
}

func bindAndValidateJob(c *gin.Context) (spec core.JobSpec, ok bool) {
	if err := c.ShouldBindJSON(&spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid JSON: " + err.Error()})
		return spec, false
	}
	if spec.Name == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "spec.name required"})
		return spec, false
	}
	if !isSafeName(spec.Name) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid spec.name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return spec, false
	}
	return spec, true
}

func (r *Router) handleCreateJob(c *gin.Context) {
	spec, ok := bindAndValidateJob(c)
	if !ok {
		return
	}
	if err := r.jobManager.CreateJob(spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleUpdateJob(c *gin.Context) {
	name := c.Param("name")
	spec, ok := bindAndValidateJob(c)
	if !ok {
		return
	}
	spec.Name = name
	if err := r.jobManager.UpdateJob(name, spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleDeleteJob(c *gin.Context) {
	name := c.Param("name")
	if err := r.jobManager.DeleteJob(name); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// cronJobResp is the wire shape for a single cronjob: the spec fields
// flattened (via anonymous embedding) plus its live status and next run time.
type cronJobResp struct {
	core.CronJob
	Status       core.CronJobStatus `json:"status"`
	NextSchedule *time.Time         `json:"next_schedule,omitempty"`
}

func (r *Router) cronJobResponse(name string) (cronJobResp, bool) {
	spec, ok := r.cronScheduler.Get(name)
	if !ok {
		return cronJobResp{}, false
	}
	status, _ := r.cronScheduler.Status(name)
	resp := cronJobResp{CronJob: spec, Status: status}
	if next, ok := r.cronScheduler.NextSchedule(name); ok && !next.IsZero() {
		resp.NextSchedule = &next
	}
	return resp, true
}

// handleListCronJobs returns every registered cronjob with its live status,
// sorted by name for a stable response order.
func (r *Router) handleListCronJobs(c *gin.Context) {
	specs := r.cronScheduler.List()
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)

	resp := make([]cronJobResp, 0, len(names))
	for _, name := range names {
		if cj, ok := r.cronJobResponse(name); ok {
			resp = append(resp, cj)
		}
	}
	writeJSON(c, http.StatusOK, resp)
}

// handleGetCronJob returns a single cronjob's spec, status, and next run time.
func (r *Router) handleGetCronJob(c *gin.Context) {
	name := c.Param("name")
	cj, ok := r.cronJobResponse(name)
	if !ok {
		writeJSON(c, http.StatusNotFound, errorResp{Error: fmt.Sprintf("cronjob %q not found", name)})
		return
	}
	writeJSON(c, http.StatusOK, cj)
}

// handleCronJobHistory returns the recent run history (capped by the spec's
// history limits) for a single cronjob.
func (r *Router) handleCronJobHistory(c *gin.Context) {
	name := c.Param("name")
	history, ok := r.cronScheduler.History(name)
	if !ok {
		writeJSON(c, http.StatusNotFound, errorResp{Error: fmt.Sprintf("cronjob %q not found", name)})
		return
	}
	writeJSON(c, http.StatusOK, history)
}

// bindAndValidateCronJob decodes a core.CronJob from the request body and
// validates its name. Schedule/job-template validation happens in
// CronJobSpec.Validate(), called by CreateCronJob/UpdateCronJob.
func bindAndValidateCronJob(c *gin.Context) (spec core.CronJob, ok bool) {
	if err := c.ShouldBindJSON(&spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid JSON: " + err.Error()})
		return spec, false
	}
	if spec.Name == "" {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "spec.name required"})
		return spec, false
	}
	if !isSafeName(spec.Name) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid spec.name: allowed [A-Za-z0-9._-] and no '..' or path separators"})
		return spec, false
	}
	// The job template's own Name is overwritten per-run (CreateJobFromTemplate
	// generates "<cronjob>-<unix ts>"), but Spec.Validate() still requires it
	// to be non-empty — default it to the cronjob's name so callers don't
	// need to know about this quirk.
	if spec.JobTemplate.Name == "" {
		spec.JobTemplate.Name = spec.Name
	}
	return spec, true
}

// handleCreateCronJob registers and schedules a new cronjob.
func (r *Router) handleCreateCronJob(c *gin.Context) {
	spec, ok := bindAndValidateCronJob(c)
	if !ok {
		return
	}
	// Persist before scheduling, same rationale as handleRegister.
	if err := r.persistCronJobFile(spec); err != nil {
		writeJSON(c, http.StatusInternalServerError, errorResp{Error: err.Error()})
		return
	}
	if err := r.cronScheduler.Add(spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// handleUpdateCronJob replaces an already-registered cronjob's spec,
// stopping the old schedule and starting the new one immediately.
func (r *Router) handleUpdateCronJob(c *gin.Context) {
	name := c.Param("name")
	spec, ok := bindAndValidateCronJob(c)
	if !ok {
		return
	}
	spec.Name = name
	if err := r.cronScheduler.Update(name, spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	if err := r.persistCronJobFile(spec); err != nil {
		writeJSON(c, http.StatusInternalServerError, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// handleDeleteCronJob stops and removes a cronjob, and its program file if any.
func (r *Router) handleDeleteCronJob(c *gin.Context) {
	name := c.Param("name")
	if err := r.cronScheduler.Delete(name); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	_ = r.removeProgramFile(name)
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// handleSuspendCronJob pauses a cronjob's schedule without removing it.
func (r *Router) handleSuspendCronJob(c *gin.Context) {
	name := c.Param("name")
	if err := r.cronScheduler.Suspend(name); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	if spec, ok := r.cronScheduler.Get(name); ok {
		_ = r.persistCronJobFile(spec)
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// handleResumeCronJob re-schedules a previously-suspended cronjob.
func (r *Router) handleResumeCronJob(c *gin.Context) {
	name := c.Param("name")
	if err := r.cronScheduler.Resume(name); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	if spec, ok := r.cronScheduler.Get(name); ok {
		_ = r.persistCronJobFile(spec)
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// handleTriggerCronJob runs a cronjob's job template immediately, out of
// band from its schedule.
func (r *Router) handleTriggerCronJob(c *gin.Context) {
	name := c.Param("name")
	if err := r.cronScheduler.Trigger(name); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func getHealthStatus(status core.Status) string {
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

	// Best-effort: remove a program file persisted by /register or /update
	// for this exact name, so an unregistered process doesn't silently come
	// back on the next daemon restart. Only handled for the single-name
	// selector; base/wildcard unregisters don't know which program files (if
	// any) correspond to the matched processes.
	if selector.name != "" {
		_ = r.removeProgramFile(selector.name)
	}

	writeJSON(c, http.StatusOK, okResp{OK: true})
}

// removeProgramFile deletes the program file for name, if any. A no-op if no
// programs directory is configured or no such file exists.
func (r *Router) removeProgramFile(name string) error {
	if r.programsDir == "" {
		return nil
	}
	path := filepath.Join(r.programsDir, name+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove program file: %w", err)
	}
	return nil
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
