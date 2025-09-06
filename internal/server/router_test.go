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
	mng "github.com/loykin/provisr/internal/manager"
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
	rec := doReq(t, h, http.MethodPost, "/abc/start", map[string]any{"command": "/bin/true"})
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
	startBody := map[string]any{
		"name":      "demo",
		"command":   "go version",
		"instances": 2,
	}
	rec := doReq(t, h, http.MethodPost, "/start", startBody)
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
