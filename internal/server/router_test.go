package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/core"
	corehistory "github.com/loykin/provisr/core/history"
	"github.com/loykin/provisr/internal/config"
)

type fakeHistoryReader struct {
	rows  []corehistory.Entry
	total int
}

func (f fakeHistoryReader) List(context.Context, string, int, int) ([]corehistory.Entry, error) {
	return f.rows, nil
}

func (f fakeHistoryReader) Count(context.Context, string) (int, error) { return f.total, nil }

func TestHistoryReaderIsInjected(t *testing.T) {
	r := NewRouter(core.New(), "")
	r.SetHistoryReader(fakeHistoryReader{
		rows:  []corehistory.Entry{{Name: "worker", PID: 42, Status: "running"}},
		total: 1,
	})
	rec := doReq(t, r.Handler(), http.MethodGet, "/history", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("history expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Rows  []corehistory.Entry `json:"rows"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Total != 1 || len(response.Rows) != 1 || response.Rows[0].Name != "worker" {
		t.Fatalf("unexpected history response: %+v", response)
	}
}

func setupRouter(t *testing.T, base string) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := core.New()
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
	spec := core.Spec{Command: "/bin/true"} // missing name - should fail
	rec := doReq(t, h, http.MethodPost, "/abc/start", spec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterFailureRollsBackProgramFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	programsDir := t.TempDir()
	r := NewRouter(core.New(), "")
	r.programsDir = programsDir

	spec := core.Spec{Name: "broken", Command: "true", WorkDir: filepath.Join(programsDir, "missing")}
	rec := doReq(t, r.Handler(), http.MethodPost, "/register", spec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(programsDir, "broken.json")); !os.IsNotExist(err) {
		t.Fatalf("failed registration must not leave a program file; stat error: %v", err)
	}
	if _, err := r.mgr.Status("broken"); err == nil {
		t.Fatal("failed registration must not leave a managed process")
	}
}

func TestRegisterCollisionDoesNotRemoveExistingProcess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	programsDir := t.TempDir()
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	r := NewRouter(mgr, "")
	r.programsDir = programsDir
	original := core.Spec{Name: "existing", Command: "sleep 5", Instances: 1}
	if err := mgr.Register(original); err != nil {
		t.Fatal(err)
	}
	if err := r.persistProgramFile(original); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(programsDir, "existing.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r.Handler(), http.MethodPost, "/register", core.Spec{Name: "existing", Command: "false", Instances: 1})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	status, err := mgr.Status("existing")
	if err != nil || !status.Running {
		t.Fatalf("existing process was removed by failed registration: %+v, %v", status, err)
	}
	spec, err := mgr.GetSpec("existing")
	if err != nil || spec.Command != original.Command {
		t.Fatalf("existing process spec changed: %+v, %v", spec, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing program file changed after registration collision")
	}
}

func TestUpdateFailureRestoresRuntimeAndProgramFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	programsDir := t.TempDir()
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	r := NewRouter(mgr, "")
	r.programsDir = programsDir
	original := core.Spec{Name: "stable", Command: "sleep 5", Instances: 1}
	if err := mgr.Register(original); err != nil {
		t.Fatal(err)
	}
	if err := r.persistProgramFile(original); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(programsDir, "stable.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	broken := original
	broken.WorkDir = filepath.Join(programsDir, "missing")
	rec := doReq(t, r.Handler(), http.MethodPost, "/update", broken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed update did not restore the original program file")
	}
	spec, err := mgr.GetSpec("stable")
	if err != nil || spec.WorkDir != "" {
		t.Fatalf("failed update did not restore runtime spec: %+v, %v", spec, err)
	}
}

func TestUnregisterFailureRestoresProgramFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	programsDir := t.TempDir()
	r := NewRouter(core.New(), "")
	r.programsDir = programsDir
	path := filepath.Join(programsDir, "missing.json")
	original := []byte(`{"type":"process","spec":{"name":"missing","command":"true"}}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r.Handler(), http.MethodPost, "/unregister?name=missing", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Fatal("failed unregister did not restore the program file")
	}
}

func TestUnregisterNumberedInstanceRemovesExactSetAndBaseFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	programsDir := t.TempDir()
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	r := NewRouter(mgr, "")
	r.programsDir = programsDir
	spec := core.Spec{Name: "workers", Command: "sleep 5", Instances: 2}
	if err := mgr.RegisterN(spec); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Register(core.Spec{Name: "workers-canary", Command: "sleep 5", Instances: 1}); err != nil {
		t.Fatal(err)
	}
	if err := r.persistProgramFile(spec); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r.Handler(), http.MethodPost, "/unregister?name=workers-1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, name := range []string{"workers-1", "workers-2"} {
		if _, err := mgr.Status(name); err == nil {
			t.Fatalf("%s remained registered", name)
		}
	}
	if _, err := mgr.Status("workers-canary"); err != nil {
		t.Fatalf("unrelated prefixed process was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(programsDir, "workers.json")); !os.IsNotExist(err) {
		t.Fatalf("base program file was not removed: %v", err)
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
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestStatusUnknown(t *testing.T) {
	h := setupRouter(t, "")
	rec := doReq(t, h, http.MethodGet, "/status?name=unknown", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGroupsAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := core.New()
	mgr.SetInstanceGroups([]core.ManagerInstanceGroup{
		{Name: "workers", Members: []core.Spec{{Name: "worker", Instances: 2}}},
		{Name: "api", Members: []core.Spec{{Name: "server"}}},
	})
	h := NewRouter(mgr, "").Handler()
	rec := doReq(t, h, http.MethodGet, "/groups", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("groups expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var groups []struct {
		Name    string `json:"name"`
		Members []struct {
			Name      string `json:"name"`
			Instances int    `json:"instances"`
		} `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &groups); err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].Name != "api" || groups[1].Members[0].Instances != 2 {
		t.Fatalf("unexpected groups response: %+v", groups)
	}
}

func TestRuntimeStatusDoesNotExposeSecrets(t *testing.T) {
	rec := doReq(t, setupRouter(t, ""), http.MethodGet, "/settings/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings status expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("secret")) || bytes.Contains(rec.Body.Bytes(), []byte("dsn")) {
		t.Fatalf("runtime status exposed a sensitive field: %s", rec.Body.String())
	}
}

func TestTemplatePreviewAPI(t *testing.T) {
	h := setupRouter(t, "")
	rec := doReq(t, h, http.MethodGet, "/templates", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("template types expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, h, http.MethodGet, "/templates/worker?name=my-worker", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("template preview expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var spec core.Spec
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Name != "my-worker" || spec.Command == "" {
		t.Fatalf("unexpected template: %+v", spec)
	}
}

func TestAPIEndpointsRegisterAllIncludesManagerBackedSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	if err := mgr.Register(core.Spec{Name: "embedded", Command: "sleep 5", Instances: 1}); err != nil {
		t.Fatal(err)
	}

	g := gin.New()
	api := g.Group("/api")
	NewAPIEndpoints(mgr, "/api").RegisterAll(api)

	checks := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/processes/embedded/spec", nil},
		{http.MethodGet, "/api/processes/embedded/logs", nil},
		{http.MethodGet, "/api/templates", nil},
		{http.MethodGet, "/api/templates/worker", nil},
		{http.MethodPost, "/api/update", core.Spec{Name: "embedded", Command: "sleep 5", Instances: 1}},
	}
	for _, check := range checks {
		rec := doReq(t, g, check.method, check.path, check.body)
		if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
			t.Errorf("%s %s was not registered: %d %s", check.method, check.path, rec.Code, rec.Body.String())
		}
	}
}

func TestJobsAPI(t *testing.T) {
	h := setupRouter(t, "")
	spec := core.JobSpec{Name: "job-api", Command: "go version"}

	rec := doReq(t, h, http.MethodPost, "/jobs", spec)
	if rec.Code != http.StatusOK {
		t.Fatalf("create job expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, h, http.MethodGet, "/jobs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list jobs expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var jobs []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("failed to parse jobs json: %v", err)
	}
	if len(jobs) != 1 || jobs[0]["name"] != "job-api" {
		t.Fatalf("unexpected jobs response: %+v", jobs)
	}

	rec = doReq(t, h, http.MethodGet, "/jobs/job-api", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get job expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, h, http.MethodDelete, "/jobs/job-api", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete job expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWildcardStatusAndStop(t *testing.T) {
	h := setupRouter(t, "")
	// start 2 instances via API
	startSpec := core.Spec{
		Name:      "demo",
		Command:   "go version",
		Instances: 2,
	}
	// First register the process
	rec := doReq(t, h, http.MethodPost, "/register", startSpec)
	if rec.Code != http.StatusOK {
		t.Fatalf("register expected 200, got %d: %s", rec.Code, rec.Body.String())
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

func TestStartByBase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	if err := mgr.RegisterN(core.Spec{Name: "base-start", Command: "go version", Instances: 2}); err != nil {
		t.Fatal(err)
	}
	h := NewRouter(mgr, "").Handler()
	rec := doReq(t, h, http.MethodPost, "/start?base=base-start", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start base expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStartInvalidNameAndPaths(t *testing.T) {
	h := setupRouter(t, "")
	// invalid name
	badNameSpec := core.Spec{Name: "../bad", Command: "go version"} // invalid name - should fail
	rec := doReq(t, h, http.MethodPost, "/register", badNameSpec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid name expected 400, got %d", rec.Code)
	}

	// invalid workdir (relative)
	badWorkDirSpec := core.Spec{Name: "ok", Command: "go version", WorkDir: "rel/path"} // relative path - should fail
	rec = doReq(t, h, http.MethodPost, "/register", badWorkDirSpec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid workdir expected 400, got %d", rec.Code)
	}
	// invalid pid file (relative)
	spec1 := core.Spec{
		Name:    "ok",
		Command: "go version",
		PIDFile: "pid.pid", // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/register", spec1)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid pidfile expected 400, got %d", rec.Code)
	}

	// invalid log paths (relative)
	spec2 := core.Spec{
		Name:    "ok",
		Command: "go version",
		Log:     core.LogConfig{File: core.LogFileConfig{Dir: "logs"}}, // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/register", spec2)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid log.dir expected 400, got %d", rec.Code)
	}

	spec3 := core.Spec{
		Name:    "ok",
		Command: "go version",
		Log:     core.LogConfig{File: core.LogFileConfig{StdoutPath: "out.log"}}, // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/register", spec3)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid log.stdoutPath expected 400, got %d", rec.Code)
	}

	spec4 := core.Spec{
		Name:    "ok",
		Command: "go version",
		Log:     core.LogConfig{File: core.LogFileConfig{StderrPath: "err.log"}}, // relative path - should fail
	}
	rec = doReq(t, h, http.MethodPost, "/register", spec4)
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
	startSpec := core.Spec{
		Name:    "svc",
		Command: "go version",
	}
	rec := doReq(t, h, http.MethodPost, "/api/register", startSpec)
	if rec.Code != http.StatusOK {
		t.Fatalf("register expected 200, got %d: %s", rec.Code, rec.Body.String())
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
	mgr := core.New()
	srv, err := NewServer(config.ServerConfig{Listen: "127.0.0.1:0", BasePath: "/x"}, mgr, nil)
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	// Close immediately; we don't assert more here, just exercise the code path
	_ = srv.Close()
}
