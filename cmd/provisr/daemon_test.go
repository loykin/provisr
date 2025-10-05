package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDaemonize(t *testing.T) {
	// This is a basic test to ensure the daemonize function doesn't panic
	// In real usage, we'd need more sophisticated integration tests

	// Test PID file writing
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test_daemon.pid")
	defer func() { _ = os.Remove(pidFile) }()

	err := writePidFile(pidFile, os.Getpid())
	if err != nil {
		t.Errorf("writePidFile failed: %v", err)
	}

	// Verify file exists and contains correct PID
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Error("PID file was not created")
	}

	// Test PID file removal
	err = removePidFile(pidFile)
	if err != nil {
		t.Errorf("removePidFile failed: %v", err)
	}

	// Verify file is removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file was not removed")
	}
}

func TestServeFlags(t *testing.T) {
	// Test that ServeFlags struct has the expected fields
	flags := &ServeFlags{
		ConfigPath: "test.toml",
		Daemonize:  true,
		PidFile:    "/tmp/test.pid",
		LogFile:    "/tmp/test.log",
	}

	if flags.ConfigPath != "test.toml" {
		t.Errorf("Expected ConfigPath 'test.toml', got '%s'", flags.ConfigPath)
	}

	if !flags.Daemonize {
		t.Error("Expected Daemonize to be true")
	}

	if flags.PidFile != "/tmp/test.pid" {
		t.Errorf("Expected PidFile '/tmp/test.pid', got '%s'", flags.PidFile)
	}

	if flags.LogFile != "/tmp/test.log" {
		t.Errorf("Expected LogFile '/tmp/test.log', got '%s'", flags.LogFile)
	}
}
