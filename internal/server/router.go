package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	mng "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
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
	group.POST("/start", r.handleStart)
	group.POST("/stop", r.handleStop)
	group.GET("/status", r.handleStatus)
	group.GET("/debug/processes", r.handleDebugProcesses)
	group.POST("/debug/reconcile", r.handleDebugReconcile)
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
	go func() { _ = server.ListenAndServe() }()
	return server, nil
}

// --- Handlers ---

type errorResp struct {
	Error string `json:"error"`
}

type okResp struct {
	OK bool `json:"ok"`
}

func (r *Router) handleStart(c *gin.Context) {
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
	if !isSafeAbsPath(spec.Log.Dir) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.dir: must be absolute path without traversal"})
		return
	}
	if !isSafeAbsPath(spec.Log.StdoutPath) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.stdoutPath: must be absolute path without traversal"})
		return
	}
	if !isSafeAbsPath(spec.Log.StderrPath) {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "invalid log.stderrPath: must be absolute path without traversal"})
		return
	}
	// ok: safe path checked
	if err := r.mgr.StartN(spec); err != nil {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
		return
	}
	writeJSON(c, http.StatusOK, okResp{OK: true})
}

func (r *Router) handleStop(c *gin.Context) {
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
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "one of name, base, wildcard query param required"})
		return
	}
	if selCount > 1 {
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "only one of name, base, wildcard must be provided"})
		return
	}
	if base != "" {
		if err := r.mgr.StopAll(base, wait); err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
			return
		}
		writeJSON(c, http.StatusOK, okResp{OK: true})
		return
	}
	if wild != "" {
		if err := r.mgr.StopMatch(wild, wait); err != nil {
			writeJSON(c, http.StatusBadRequest, errorResp{Error: err.Error()})
			return
		}
		writeJSON(c, http.StatusOK, okResp{OK: true})
		return
	}
	if err := r.mgr.Stop(name, wait); err != nil {
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
		writeJSON(c, http.StatusBadRequest, errorResp{Error: "one of name, base, wildcard query param required"})
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

func (r *Router) handleDebugReconcile(c *gin.Context) {
	// Trigger manual reconciliation
	r.mgr.ReconcileOnce()
	writeJSON(c, http.StatusOK, okResp{OK: true})
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
