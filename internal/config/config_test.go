package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	mgrpkg "github.com/loykin/provisr/internal/manager"
	pg "github.com/loykin/provisr/internal/process_group"
)

func TestLoadSpecsFromTOML_Minimal(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "provisr.toml")
	data := `
[[processes]]
name = "demo"
command = "sleep 1"
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	specs, err := LoadSpecsFromTOML(file)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	s := specs[0]
	if s.Name != "demo" || s.Command != "sleep 1" {
		t.Fatalf("unexpected spec: %+v", s)
	}
}

func TestLoadSpecsFromTOML_Full_Base(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cfg.toml")
	data := `
[[processes]]
name = "web"
command = "sleep 2"
workdir = "/tmp"
env = ["A=1","B=2"]
pidfile = "/tmp/web.pid"
retries = 2
retry_interval = "200ms"
autorestart = true
restart_interval = "1s"
startsecs = "150ms"
instances = 3
  [[processes.detectors]]
  type = "pidfile"
  path = "/tmp/web.pid"
  [[processes.detectors]]
  type = "command"
  command = "true"
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	specs, err := LoadSpecsFromTOML(file)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	s := specs[0]
	if s.Name != "web" || s.Command != "sleep 2" || s.WorkDir != "/tmp" || len(s.Env) != 2 || s.PIDFile != "/tmp/web.pid" {
		t.Fatalf("unexpected base fields: %+v", s)
	}
	if s.RetryCount != 2 || s.RetryInterval.String() != "200ms" || s.StartDuration.String() != "150ms" || !s.AutoRestart || s.RestartInterval.String() != "1s" || s.Instances != 3 {
		t.Fatalf("unexpected control fields: %+v", s)
	}
}

func TestLoadSpecsFromTOML_Full_Detectors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cfg.toml")
	data := `
[[processes]]
name = "web"
command = "sleep 2"
workdir = "/tmp"
  [[processes.detectors]]
  type = "pidfile"
  path = "/tmp/web.pid"
  [[processes.detectors]]
  type = "command"
  command = "true"
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	specs, err := LoadSpecsFromTOML(file)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	s := specs[0]
	if len(s.Detectors) != 2 {
		t.Fatalf("expected 2 detectors, got %d", len(s.Detectors))
	}
}

func TestLoadGroupsFromTOML(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "groups.toml")
	data := `
[[processes]]
name = "a"
command = "sleep 1"

[[processes]]
name = "b"
command = "sleep 1"

[[groups]]
name = "g1"
members = ["a", "b"]
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	groups, err := LoadGroupsFromTOML(file)
	if err != nil {
		t.Fatalf("load groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "g1" || len(groups[0].Members) != 2 {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	// try starting the group to ensure specs are usable
	mgr := mgrpkg.NewManager()
	g := pg.New(mgr)
	if err := g.Start(groups[0]); err != nil {
		t.Fatalf("group start: %v", err)
	}
	_ = g.Stop(groups[0], 2*time.Second)
}

func TestLoadCronJobsFromTOML(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cron.toml")
	data := `
[[processes]]
name = "job1"
command = "echo hi"
schedule = "@every 100ms"
singleton = true

[[processes]]
name = "bad"
command = "echo bad"
schedule = "@every 1s"
autorestart = true
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	// Call and discard jobs on the first (expected error) attempt to avoid SA4006
	_, err := LoadCronJobsFromTOML(file)
	if err == nil {
		t.Fatalf("expected error due to autorestart=true in scheduled process")
	}
	var jobs []CronJob // declare here for the second successful load
	// Fix and reload
	data2 := `
[[processes]]
name = "job1"
command = "echo hi"
schedule = "@every 100ms"
`
	if err := os.WriteFile(file, []byte(data2), 0o644); err != nil {
		t.Fatalf("write toml2: %v", err)
	}
	jobs, err = LoadCronJobsFromTOML(file)
	if err != nil {
		t.Fatalf("load cron: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(jobs))
	}
	if jobs[0].Name != "job1" || jobs[0].Schedule == "" {
		t.Fatalf("unexpected cron job: %+v", jobs[0])
	}
}

// This test verifies that top-level env and per-process env loaded via TOML
// are merged and applied when starting through Manager using loaded specs.
func TestConfigEnvMergeIntegration(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	cfg := filepath.Join(dir, "env.toml")
	data := `
env = ["GLOB=G", "CHAIN=${GLOB}-x"]
[[processes]]
name = "env-merge"
command = "sh -c 'echo $GLOB $CHAIN $PORT $LOCAL > ` + out + `'"
env = ["PORT=2000", "LOCAL=${GLOB}-y"]
`
	if err := os.WriteFile(cfg, []byte(data), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	mgr := mgrpkg.NewManager()
	genv, err := LoadEnvFromTOML(cfg)
	if err != nil {
		t.Fatalf("load env: %v", err)
	}
	mgr.SetGlobalEnv(genv)
	specs, err := LoadSpecsFromTOML(cfg)
	if err != nil {
		t.Fatalf("load specs: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if err := mgr.Start(specs[0]); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(bytes.TrimSpace(b))
	if got != "G G-x 2000 G-y" {
		t.Fatalf("unexpected output: %q", got)
	}
}

// This test verifies that retry settings from TOML are honored: a command exiting early
// before startsecs triggers retries. We observe it via elapsed time.
func TestConfigRetryHonored(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "retry.toml")
	// Run a short command exiting in ~100ms; set startsecs=300ms to force start failure; retries=2, interval=100ms
	data := `
[[processes]]
name = "bad"
command = "sh -c 'sleep 0.1'"
retries = 2
retry_interval = "100ms"
startsecs = "300ms"
`
	if err := os.WriteFile(cfg, []byte(data), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	specs, err := LoadSpecsFromTOML(cfg)
	if err != nil {
		t.Fatalf("load specs: %v", err)
	}
	mgr := mgrpkg.NewManager()
	start := time.Now()
	err = mgr.Start(specs[0])
	if err == nil {
		t.Fatalf("expected start error for failing command")
	}
	elapsed := time.Since(start)
	// With startsecs=300ms and immediate retries on early exit, elapsed should be close to the start window,
	// not too short. Allow some margin for scheduling; require at least ~280ms.
	if elapsed < 280*time.Millisecond {
		t.Fatalf("expected elapsed around retries/start window (>=280ms), got %v", elapsed)
	}
}

func TestLoadSpecsUnknownDetector(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[processes]]
name = "x"
command = "true"
[[processes.detectors]]
type = "unknown"
`
	p := filepath.Join(dir, "c.toml")
	if err := os.WriteFile(p, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSpecsFromTOML(p); err == nil {
		t.Fatalf("expected error for unknown detector type")
	}
}

func TestLoadCronJobsInvalidFlags(t *testing.T) {
	dir := t.TempDir()
	// autorestart true -> error
	toml1 := `
[[processes]]
name = "a"
command = "true"
schedule = "@every 1s"
autorestart = true
`
	p1 := filepath.Join(dir, "a.toml")
	if err := os.WriteFile(p1, []byte(toml1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCronJobsFromTOML(p1); err == nil {
		t.Fatalf("expected error for autorestart=true in cron job")
	}
	// instances > 1 -> error
	toml2 := `
[[processes]]
name = "b"
command = "true"
schedule = "@every 1s"
instances = 2
`
	p2 := filepath.Join(dir, "b.toml")
	if err := os.WriteFile(p2, []byte(toml2), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCronJobsFromTOML(p2); err == nil {
		t.Fatalf("expected error for instances>1 in cron job")
	}
}

func TestLoadEnvFileInvalidPath(t *testing.T) {
	if _, err := LoadEnvFile("/definitely/not/exist.env"); err == nil {
		t.Fatalf("expected error for missing env file")
	}
}

func TestLoadEnvFromTOML_TopLevelOnly(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.toml")
	// env_files should be ignored by LoadEnvFromTOML; only top-level env returned
	dotenv := filepath.Join(dir, ".env")
	_ = os.WriteFile(dotenv, []byte("A=1\n"), 0o644)
	data := "" +
		"env = [\"X=9\", \"Y=2\"]\n" +
		"env_files = [\"" + dotenv + "\"]\n"
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	pairs, err := LoadEnvFromTOML(p)
	if err != nil {
		t.Fatalf("LoadEnvFromTOML: %v", err)
	}
	if len(pairs) != 2 { // only X and Y
		t.Fatalf("expected 2 items, got %d: %v", len(pairs), pairs)
	}
}

func TestLoadEnvFile_MalformedLinesIgnored(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "#comment\n\nNOEQUAL\nKEY=VAL\nTRAIL= space \n =noval\n" // malformed lines should be ignored, spaces trimmed
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	pairs, err := LoadEnvFile(p)
	if err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	m := map[string]string{}
	for _, kv := range pairs {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	if m["KEY"] != "VAL" {
		t.Fatalf("expected KEY=VAL, got %v", m)
	}
	if m["TRAIL"] != "space" { // trimmed
		t.Fatalf("expected TRAIL=space, got %v", m["TRAIL"])
	}
}

func TestMergeLogCfgPrecedenceViaSpecs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.toml")
	data := `
log = { dir = "/tmp/base", stdout = "/tmp/base.out", stderr = "/tmp/base.err", max_size_mb = 10, max_backups = 2, max_age_days = 7, compress = true }
[[processes]]
name = "svc"
command = "true"
log = { dir = "/tmp/ovr", stdout = "/tmp/ovr.out", max_size_mb = 20 }
`
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	specs, err := LoadSpecsFromTOML(p)
	if err != nil {
		t.Fatalf("LoadSpecsFromTOML: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec")
	}
	lg := specs[0].Log
	// per-process overrides Dir, StdoutPath, MaxSizeMB; others inherited from top-level
	if lg.Dir != "/tmp/ovr" || lg.StdoutPath != "/tmp/ovr.out" || lg.StderrPath != "/tmp/base.err" || lg.MaxSizeMB != 20 || lg.MaxBackups != 2 || lg.MaxAgeDays != 7 || !lg.Compress {
		t.Fatalf("unexpected merged log cfg: %+v", lg)
	}
}

func TestLoadGroupsFromTOML_Errors(t *testing.T) {
	dir := t.TempDir()
	base := `
[[processes]]
name = "a"
command = "true"
`
	// missing name
	p1 := filepath.Join(dir, "g1.toml")
	_ = os.WriteFile(p1, []byte(base+"\n[[groups]]\nmembers=[\"a\"]\n"), 0o644)
	if _, err := LoadGroupsFromTOML(p1); err == nil {
		t.Fatalf("expected error for missing group name")
	}
	// empty members
	p2 := filepath.Join(dir, "g2.toml")
	_ = os.WriteFile(p2, []byte(base+"\n[[groups]]\nname=\"g\"\nmembers=[]\n"), 0o644)
	if _, err := LoadGroupsFromTOML(p2); err == nil {
		t.Fatalf("expected error for empty members")
	}
	// unknown member
	p3 := filepath.Join(dir, "g3.toml")
	_ = os.WriteFile(p3, []byte(base+"\n[[groups]]\nname=\"g\"\nmembers=[\"x\"]\n"), 0o644)
	if _, err := LoadGroupsFromTOML(p3); err == nil {
		t.Fatalf("expected error for unknown member")
	}
}

func TestLoadCronJobs_SingletonDefaultAndExplicit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cron.toml")
	data := `
[[processes]]
name = "a"
command = "true"
schedule = "@every 1s"
# no singleton -> default true

[[processes]]
name = "b"
command = "true"
schedule = "@every 1s"
singleton = false
`
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	jobs, err := LoadCronJobsFromTOML(p)
	if err != nil {
		t.Fatalf("LoadCronJobsFromTOML: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs")
	}
	if !jobs[0].Singleton {
		t.Fatalf("expected default singleton=true")
	}
	if jobs[1].Singleton {
		t.Fatalf("expected explicit singleton=false")
	}
}

func TestLoadHTTPAPIFromTOML_PresentAndAbsent(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.toml")
	p2 := filepath.Join(dir, "b.toml")
	data1 := `
[http_api]
enabled = true
listen = ":9000"
base_path = "/api"
`
	if err := os.WriteFile(p1, []byte(data1), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadHTTPAPIFromTOML(p1)
	if err != nil {
		t.Fatalf("LoadHTTPAPIFromTOML: %v", err)
	}
	if cfg == nil || !cfg.Enabled || cfg.Listen != ":9000" || cfg.BasePath != "/api" {
		t.Fatalf("unexpected http api cfg: %+v", cfg)
	}
	// absent section
	if err := os.WriteFile(p2, []byte("# no http_api\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg2, err := LoadHTTPAPIFromTOML(p2)
	if err != nil {
		t.Fatalf("LoadHTTPAPIFromTOML: %v", err)
	}
	if cfg2 != nil {
		t.Fatalf("expected nil cfg when section absent, got %+v", cfg2)
	}
}
