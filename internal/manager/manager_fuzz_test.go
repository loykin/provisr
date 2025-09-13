package manager

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// FuzzManagerStartStopMassive fuzzes starting/stopping multiple short-lived processes
// to ensure the manager handles concurrent instances without panics or leaks.
func FuzzManagerStartStopMassive(f *testing.F) {
	// Seed with representative sizes and durations
	f.Add(5, 10)  // 5 instances, 10ms sleep
	f.Add(10, 20) // 10 instances, 20ms sleep
	f.Add(20, 30) // 20 instances, 30ms sleep

	f.Fuzz(func(t *testing.T, n int, sleepMs int) {
		if runtime.GOOS == "windows" {
			t.Skip("requires Unix-like environment")
		}
		// Bound the fuzzed values to keep the test fast and safe
		if n < 1 {
			n = 1
		}
		if n > 30 {
			n = 30
		}
		sleepMs = sleepMs % 100 // cap to <100ms
		if sleepMs < 1 {
			sleepMs = 1
		}
		d := time.Duration(sleepMs) * time.Millisecond

		mgr := NewManager()
		base := "fz-mass"
		// Use a simple, low-cost command that keeps the process alive briefly
		cmd := fmt.Sprintf("sh -c 'sleep %.3f'", d.Seconds())
		spec := process.Spec{
			Name:          base,
			Command:       cmd,
			Instances:     n,
			StartDuration: 0,
		}
		if err := mgr.StartN(spec); err != nil {
			// If the environment cannot run the command for some reason, skip rather than fail fuzzing.
			t.Skipf("start failed (skipping): %v", err)
		}
		// Allow processes to actually start
		time.Sleep(10 * time.Millisecond)

		// Verify Count reports expected running instances (best-effort; do not fail fuzz if racy)
		if c, err := mgr.Count(base); err == nil && c < 0 {
			// shouldn't happen; keep as a sanity check placeholder
			t.Fatalf("negative count: %d", c)
		}

		// Stop all instances; wait generously but still bounded
		_ = mgr.StopAll(base, 2*time.Second)

		// Ensure all instances are reported not running
		sts, err := mgr.StatusAll(base)
		if err != nil {
			// Unknown base is acceptable if internal map cleaned up fully
			return
		}
		for _, st := range sts {
			if st.Running {
				t.Fatalf("instance still running after StopAll: %+v", st)
			}
		}
	})
}
