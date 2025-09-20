package process

import (
	"testing"
	"time"
)

func TestEnforceStartDurationFail(t *testing.T) {
	spec := Spec{
		Name:    "test-fail",
		Command: "false", // 'false' command exits immediately with status 1
	}

	p := New(spec)

	// Start process (should succeed)
	env := []string{}
	cmd := p.ConfigureCmd(env)
	err := p.TryStart(cmd)
	if err != nil {
		t.Fatalf("TryStart should succeed: %v", err)
	}

	t.Logf("Process started with PID: %d", cmd.Process.Pid)

	// Check if WaitDoneChan is available
	waitDone := p.WaitDoneChan()
	t.Logf("WaitDoneChan available: %v", waitDone != nil)

	// Wait a bit to see if process exits quickly
	time.Sleep(100 * time.Millisecond)

	alive, source := p.DetectAlive()
	t.Logf("After 100ms: DetectAlive=%v, source=%s", alive, source)

	// Enforce start duration (should fail because process exits immediately)
	err = p.EnforceStartDuration(50 * time.Millisecond)
	if err == nil {
		t.Fatalf("EnforceStartDuration should fail for quickly exiting process")
	}

	t.Logf("✅ Got expected error: %v", err)
}
