package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/internal/logger"
	mng "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
)

func setupRouter(t *testing.T, base string) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := mng.NewManager()
	r := NewRouter(mgr, base)
	return r.Handler()
}

func doReq(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestStartMissingName(t *testing.T) {
	h := setupRouter(t, "/abc")
	spec := process.Spec{Command: "/bin/true"} // missing name - should fail
	rec := doReq(t, h, http.MethodPost, "/abc/start", spec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestStopAllOKNoProcs(t *testing.T) {
	h := setupRouter(t, "")
	rec := doReq(t, h, http.MethodPost, "/stop?base=nothing", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStopRequiresParam(t *testing.T) {
	h := setupRouter(t, "")
	rec := doReq(t, h, http.MethodPost, "/stop", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestStatusRequiresParam(t *testing.T) {
	h := setupRouter(t, "/base")
	rec := doReq(t, h, http.MethodGet, "/base/status", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestStatusUnknown(t *testing.T) {
	h := setupRouter(t, "")
	rec := doReq(t, h, http.MethodGet, "/status?name=unknown", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestWildcardStatusAndStop(t *testing.T) {
	h := setupRouter(t, "")
	// start 2 instances via API
	startSpec := process.Spec{
		Name:      "demo",
		Command:   "go version",
		Instances: 2,
	}
	rec := doReq(t, h, http.MethodPost, "/start", startSpec)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// query wildcard
	rec = doReq(t, h, http.MethodGet, "/status?wildcard=demo-*", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var arr []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(arr))
	}
	// stop by wildcard (wait a bit to ensure quick commands already exited)
	// not strictly necessary, but avoids exit error propagation in some OSes
	// where a process may still be alive momentarily.
	time.Sleep(50 * time.Millisecond)
	rec = doReq(t, h, http.MethodPost, "/stop?wildcard=demo-*", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stop expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStartInvalidNameAndPaths(t *testing.T) {
	h := setupRouter(t, "")
	// invalid name
	badNameSpec := process.Spec{Name: "../bad", Command: "go version"} // invalid name - should fail
	rec := doReq(t, h, http.MethodPost, "/start", badNameSpec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid name expected 400, got %d", rec.Code)
	}

	// invalid workdir (relative)
	badWorkDirSpec := process.Spec{Name: "ok", Command: "go version", WorkDir: "rel/path"} // relative path - should fail
	rec = doReq(t, h, http.MethodPost, "/start", badWorkDirSpec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid workdir expected 400, got %d", rec.Code)
	}
	// invalid pid file (relative)
	spec1 := process.Spec{
		Name:    "ok",
		Command: "go version",
		PIDFile: "pid.pid", // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/start", spec1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid pidfile expected 400, got %d", rec.Code)
	}

	// invalid log paths (relative)
	spec2 := process.Spec{
		Name:    "ok",
		Command: "go version",
		Log:     logger.Config{File: logger.FileConfig{Dir: "logs"}}, // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/start", spec2)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid log.dir expected 400, got %d", rec.Code)
	}

	spec3 := process.Spec{
		Name:    "ok",
		Command: "go version",
		Log:     logger.Config{File: logger.FileConfig{StdoutPath: "out.log"}}, // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/start", spec3)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid log.stdoutPath expected 400, got %d", rec.Code)
	}

	spec4 := process.Spec{
		Name:    "ok",
		Command: "go version",
		Log:     logger.Config{File: logger.FileConfig{StderrPath: "err.log"}}, // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/start", spec4)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid log.stderrPath expected 400, got %d", rec.Code)
	}
}

func TestSelectorsMutualExclusion(t *testing.T) {
	h := setupRouter(t, "")
	// stop: too many selectors
	rec := doReq(t, h, http.MethodPost, "/stop?name=a&base=b", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("stop too many selectors expected 400, got %d", rec.Code)
	}
	// status: too many selectors
	rec = doReq(t, h, http.MethodGet, "/status?name=a&wildcard=*", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status too many selectors expected 400, got %d", rec.Code)
	}
}

func TestStartThenStatusByBaseAndName(t *testing.T) {
	h := setupRouter(t, "/api/") // ensure base sanitization works
	// successful start
	startSpec := process.Spec{
		Name:    "svc",
		Command: "go version",
	}
	rec := doReq(t, h, http.MethodPost, "/api/start", startSpec)
	if rec.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// status by base should return an array (len>=1)
	rec = doReq(t, h, http.MethodGet, "/api/status?base=svc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status base expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var arr []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &arr)
	if len(arr) < 1 {
		t.Fatalf("expected at least 1 status, got %d", len(arr))
	}
	// status by name should return an object
	rec = doReq(t, h, http.MethodGet, "/api/status?name=svc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status name expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNewServerStartClose(t *testing.T) {
	// ensure NewServer returns a server and can be closed quickly
	mgr := mng.NewManager()
	srv, err := NewServer("127.0.0.1:0", "/x", mgr)
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	// Close immediately; we don't assert more here, just exercise the code path
	_ = srv.Close()
}
