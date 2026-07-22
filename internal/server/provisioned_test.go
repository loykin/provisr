package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/core"
)

// setupCronRouter builds a Router wired with a CronScheduler, mirroring what
// newRouterFromConfig does for `provisr serve` — the plain NewRouter used
// elsewhere in this package leaves cronScheduler nil, which skips cronjob
// route registration entirely.
func setupCronRouter(t *testing.T, programsDir string) (*Router, *core.Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := core.New()
	jm := core.NewJobManager(mgr)
	scheduler := core.NewCronScheduler(jm)
	r := NewRouter(mgr, "")
	r.programsDir = programsDir
	r.cronScheduler = scheduler
	r.jobManager = jm
	return r, mgr
}

// --- Processes: config.toml `[[processes]]` entries are locked ---

func TestUnregisterInlineConfiguredProcessIsBlocked(t *testing.T) {
	programsDir := t.TempDir()
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	r := NewRouter(mgr, "")
	r.programsDir = programsDir

	if err := mgr.Register(core.Spec{Name: "cfgproc", Command: "sleep 5", InlineConfig: true}); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r.Handler(), http.MethodPost, "/unregister?name=cfgproc", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if status, err := mgr.Status("cfgproc"); err != nil || !status.Running {
		t.Fatalf("inline-configured process must survive a blocked unregister: %+v, %v", status, err)
	}
}

func TestUpdateInlineConfiguredProcessIsBlocked(t *testing.T) {
	programsDir := t.TempDir()
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	r := NewRouter(mgr, "")
	r.programsDir = programsDir

	if err := mgr.Register(core.Spec{Name: "cfgproc", Command: "sleep 5", InlineConfig: true}); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r.Handler(), http.MethodPost, "/update", core.Spec{Name: "cfgproc", Command: "echo changed"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	spec, err := mgr.GetSpec("cfgproc")
	if err != nil || spec.Command != "sleep 5" {
		t.Fatalf("inline-configured process spec must be unchanged: %+v, %v", spec, err)
	}
}

// TestUnregisterAPIRegisteredProcessStillWorks guards against the blocking
// check being too broad — a process registered through this same API (no
// InlineConfig) must still be fully manageable.
func TestUnregisterAPIRegisteredProcessStillWorks(t *testing.T) {
	programsDir := t.TempDir()
	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	r := NewRouter(mgr, "")
	r.programsDir = programsDir

	rec := doReq(t, r.Handler(), http.MethodPost, "/register", core.Spec{Name: "apiproc", Command: "sleep 5"})
	if rec.Code != http.StatusOK {
		t.Fatalf("register expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, r.Handler(), http.MethodPost, "/unregister?name=apiproc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unregister expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := mgr.Status("apiproc"); err == nil {
		t.Fatal("apiproc should be gone after a successful unregister")
	}
}

// TestUnregisterRemovesHandAuthoredNonJSONProgramFile guards the multi-
// extension fix: a process loaded from a hand-authored .toml program file
// (not `[[processes]]`, and not written by this API) must still be fully
// unregisterable, and its .toml file actually removed — not silently
// ignored because removeProgramFile used to only ever look for ".json".
func TestUnregisterRemovesHandAuthoredNonJSONProgramFile(t *testing.T) {
	programsDir := t.TempDir()
	tomlPath := filepath.Join(programsDir, "handauthored.toml")
	if err := os.WriteFile(tomlPath, []byte("type = \"process\"\n[spec]\nname = \"handauthored\"\ncommand = \"sleep 5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := core.New()
	defer func() { _ = mgr.Shutdown() }()
	r := NewRouter(mgr, "")
	r.programsDir = programsDir
	if err := mgr.Register(core.Spec{Name: "handauthored", Command: "sleep 5"}); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r.Handler(), http.MethodPost, "/unregister?name=handauthored", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(tomlPath); !os.IsNotExist(err) {
		t.Fatalf("expected handauthored.toml to be removed, stat error: %v", err)
	}
}

// --- CronJobs: same rule, applied to delete/update/suspend/resume ---

func TestCronJobMutationsBlockedForInlineConfigured(t *testing.T) {
	r, _ := setupCronRouter(t, t.TempDir())
	spec := core.CronJob{
		Name:         "cfgcron",
		Schedule:     "@every 1h",
		JobTemplate:  core.JobSpec{Name: "cfgcron", Command: "echo hi"},
		InlineConfig: true,
	}
	if err := r.cronScheduler.Add(spec); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"delete", http.MethodDelete, "/cronjobs/cfgcron", nil},
		{"update", http.MethodPost, "/cronjobs/cfgcron", core.CronJob{Name: "cfgcron", Schedule: "@every 2h", JobTemplate: core.JobSpec{Command: "echo bye"}}},
		{"suspend", http.MethodPost, "/cronjobs/cfgcron/suspend", nil},
		{"resume", http.MethodPost, "/cronjobs/cfgcron/resume", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doReq(t, r.Handler(), tc.method, tc.path, tc.body)
			if rec.Code != http.StatusConflict {
				t.Fatalf("%s: expected 409, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	// Untouched by any of the above: still present with its original schedule.
	got, ok := r.cronScheduler.Get("cfgcron")
	if !ok || got.Schedule != "@every 1h" {
		t.Fatalf("inline-configured cronjob must be unchanged: %+v, %v", got, ok)
	}
}

// TestCronJobMutationsAllowedForAPIRegistered guards against the blocking
// check being too broad, the cronjob equivalent of
// TestUnregisterAPIRegisteredProcessStillWorks.
func TestCronJobMutationsAllowedForAPIRegistered(t *testing.T) {
	r, _ := setupCronRouter(t, t.TempDir())

	rec := doReq(t, r.Handler(), http.MethodPost, "/cronjobs", core.CronJob{
		Name:        "apicron",
		Schedule:    "@every 1h",
		JobTemplate: core.JobSpec{Name: "apicron", Command: "echo hi"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, r.Handler(), http.MethodPost, "/cronjobs/apicron/suspend", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("suspend expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, r.Handler(), http.MethodDelete, "/cronjobs/apicron", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := r.cronScheduler.Get("apicron"); ok {
		t.Fatal("apicron should be gone after a successful delete")
	}
}

// TestCreateCronJobRejectsDuplicateName guards a gap found alongside the
// provisioned-lock work: handleCreateCronJob used to persist (and, per the
// multi-extension writeProgramFile fix, potentially delete an existing
// hand-authored program file for) a colliding name before ever checking
// whether the cronjob already existed.
func TestCreateCronJobRejectsDuplicateName(t *testing.T) {
	r, _ := setupCronRouter(t, t.TempDir())
	spec := core.CronJob{Name: "dup", Schedule: "@every 1h", JobTemplate: core.JobSpec{Name: "dup", Command: "echo hi"}}
	if err := r.cronScheduler.Add(spec); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, r.Handler(), http.MethodPost, "/cronjobs", spec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate name, got %d: %s", rec.Code, rec.Body.String())
	}
}
