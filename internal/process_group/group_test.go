package process_group

import (
	"encoding/json"
	"testing"
	"time"

	mgrpkg "github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/process"
)

func TestGroupStartStopBasic(t *testing.T) {
	mgr := mgrpkg.NewManager()
	g := New(mgr)
	gs := GroupSpec{
		Name: "grp1",
		Members: []process.Spec{
			{Name: "a", Command: "sleep 1"},
			{Name: "b", Command: "sleep 1"},
		},
	}
	if err := g.Start(gs); err != nil {
		t.Fatalf("start group: %v", err)
	}
	stmap, err := g.Status(gs)
	if err != nil {
		t.Fatalf("status group: %v", err)
	}
	if len(stmap["a"]) == 0 || len(stmap["b"]) == 0 {
		t.Fatalf("expected statuses for members, got: %v", toJSON(stmap))
	}
	// stop
	if err := g.Stop(gs, 2*time.Second); err != nil {
		// best-effort, ignore
	}
	time.Sleep(100 * time.Millisecond)
	stmap2, _ := g.Status(gs)
	// both should be stopped (Running false)
	for name, sts := range stmap2 {
		for _, st := range sts {
			if st.Running {
				t.Fatalf("member %s still running after StopGroup", name)
			}
		}
	}
}

func TestGroupRollbackOnFailure(t *testing.T) {
	mgr := mgrpkg.NewManager()
	g := New(mgr)
	gs := GroupSpec{
		Name: "grp2",
		Members: []process.Spec{
			{Name: "ok", Command: "sleep 1"},
			{Name: "bad", Command: "sh -c 'exit 1'", StartDuration: 50 * time.Millisecond},
		},
	}
	err := g.Start(gs)
	if err == nil {
		t.Fatalf("expected error starting group with bad member")
	}
	// previously started member should be stopped by rollback
	time.Sleep(100 * time.Millisecond)
	sts, _ := mgr.StatusAll("ok")
	for _, st := range sts {
		if st.Running {
			t.Fatalf("expected ok member stopped due to rollback, got running")
		}
	}
}

func TestGroupWithInstances(t *testing.T) {
	mgr := mgrpkg.NewManager()
	g := New(mgr)
	gs := GroupSpec{
		Name: "grp3",
		Members: []process.Spec{
			{Name: "batch", Command: "sleep 1", Instances: 3},
		},
	}
	if err := g.Start(gs); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Debug: Check what processes exist
	statuses, err := mgr.StatusAll("*")
	if err != nil {
		t.Logf("StatusAll error: %v", err)
	} else {
		t.Logf("Found %d processes total", len(statuses))
		for _, st := range statuses {
			t.Logf("Process: name=%s, running=%v, pid=%d", st.Name, st.Running, st.PID)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cnt, _ := mgr.Count("batch")
		t.Logf("Current batch count: %d", cnt)
		if cnt == 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cnt, _ := mgr.Count("batch")
	if cnt != 3 {
		t.Fatalf("expected 3 running instances, got %d", cnt)
	}
	_ = g.Stop(gs, 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	cnt2, _ := mgr.Count("batch")
	if cnt2 != 0 {
		t.Fatalf("expected 0 after StopGroup, got %d", cnt2)
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
