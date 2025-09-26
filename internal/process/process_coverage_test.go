package process

import (
	"syscall"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/logger"
)

func TestProcess_SeedPID(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "echo hello",
		Log:     logger.Config{},
	}

	proc := New(spec)

	proc.SeedPID(12345)
}

func TestProcess_GetName(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "echo hello",
		Log:     logger.Config{},
	}

	proc := New(spec)
	name := proc.GetName()

	if name != "test-process" {
		t.Errorf("expected name to be 'test-process', got %q", name)
	}
}

func TestProcess_GetSpec(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "echo hello",
		Log:     logger.Config{},
	}

	proc := New(spec)
	retrievedSpec := proc.GetSpec()

	if retrievedSpec.Name != spec.Name {
		t.Errorf("expected spec name to be %q, got %q", spec.Name, retrievedSpec.Name)
	}

	if retrievedSpec.Command != spec.Command {
		t.Errorf("expected spec command to be %q, got %q", spec.Command, retrievedSpec.Command)
	}
}

func TestProcess_GetAutoStart(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "echo hello",
		Log:     logger.Config{},
	}

	proc := New(spec)

	autoStart := proc.GetAutoStart()
	if autoStart != false {
		t.Errorf("expected default autoStart to be false, got %v", autoStart)
	}
}

func TestProcess_StopWithSignal(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "sleep 10",
		Log:     logger.Config{},
	}

	proc := New(spec)

	cmd := spec.BuildCommand()
	err := proc.TryStart(cmd)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	if proc.cmd == nil || proc.cmd.Process == nil {
		t.Skip("process not properly started, skipping signal test")
	}

	err = proc.StopWithSignal(syscall.SIGKILL)
	if err != nil {
		t.Errorf("StopWithSignal failed: %v", err)
	}

	// Give a brief moment for signal handling; we only assert no error was returned.
	time.Sleep(50 * time.Millisecond)
}

func TestProcess_Stop(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "sleep 10",
		Log:     logger.Config{},
	}

	proc := New(spec)

	cmd := spec.BuildCommand()
	err := proc.TryStart(cmd)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	if proc.cmd == nil || proc.cmd.Process == nil {
		t.Skip("process not properly started, skipping stop test")
	}

	err = proc.Stop(3 * time.Second)
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Give a brief moment for stop handling; we only assert no error was returned.
	time.Sleep(50 * time.Millisecond)
}

func TestProcess_CloseWriters_Coverage(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "echo hello",
		Log:     logger.Config{},
	}

	proc := New(spec)

	proc.CloseWriters()
}

func TestProcess_DetectAlive_NilCmd(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "echo hello",
		Log:     logger.Config{},
	}

	proc := New(spec)

	alive, _ := proc.DetectAlive()
	if alive != false {
		t.Error("DetectAlive should return false when cmd is nil")
	}
}

func TestProcess_StopRequested_SetStopRequested(t *testing.T) {
	spec := Spec{
		Name:    "test-process",
		Command: "echo hello",
		Log:     logger.Config{},
	}

	proc := New(spec)

	if proc.StopRequested() {
		t.Error("StopRequested should initially be false")
	}

	proc.SetStopRequested(true)

	if !proc.StopRequested() {
		t.Error("StopRequested should be true after setting to true")
	}

	proc.SetStopRequested(false)

	if proc.StopRequested() {
		t.Error("StopRequested should be false after setting to false")
	}
}

func TestProcess_EnforceStartDuration_Success(t *testing.T) {
	spec := Spec{
		Name:          "test-process",
		Command:       "sleep 1",
		StartDuration: 100 * time.Millisecond,
		Log:           logger.Config{},
	}

	proc := New(spec)

	cmd := spec.BuildCommand()
	err := proc.TryStart(cmd)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = proc.EnforceStartDuration(100 * time.Millisecond)
	if err != nil {
		t.Errorf("EnforceStartDuration failed: %v", err)
	}
}
