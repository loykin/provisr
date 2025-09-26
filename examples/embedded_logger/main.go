package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/loykin/provisr"
)

// embedded_logger: demonstrate per-process log output using provisr's logger integration.
// It starts a short command that writes to stdout and stderr, then shows where the logs are stored.
func main() {
	mgr := provisr.New()

	// Determine log directory: use PROVISR_LOG_DIR if set, otherwise a temp directory.
	logDir := os.Getenv("PROVISR_LOG_DIR")
	if logDir == "" {
		logDir = filepath.Join(os.TempDir(), fmt.Sprintf("provisr-logs-%d", time.Now().UnixNano()))
	}
	_ = os.MkdirAll(logDir, 0o750)

	spec := provisr.Spec{
		Name:    "embedded-logger-demo",
		Command: "sh -c 'echo hello-out; echo hello-err 1>&2; sleep 0.2'",
		// Configure logging: using directory-based default file names
		Log: provisr.Spec{}.Log, // placeholder to access type; replaced below
	}
	// Provide an explicit config instance: the Log field uses UnifiedConfig
	// which supports both file logging and structured logging with slog
	// We don't have direct access to UnifiedConfig here, but Spec.Log is exported,
	// so we can assign fields on it in place.
	spec.Log.File.Dir = logDir

	if err := mgr.Register(spec); err != nil {
		panic(err)
	}
	// Give the process time to write logs and finish
	time.Sleep(400 * time.Millisecond)
	_ = mgr.Stop(spec.Name, 2*time.Second)

	stdoutPath := filepath.Join(logDir, "embedded-logger-demo.stdout.log")
	stderrPath := filepath.Join(logDir, "embedded-logger-demo.stderr.log")

	fmt.Println("Embedded logger example")
	fmt.Println("  Log directory:", logDir)
	fmt.Println("  Stdout log:", stdoutPath)
	fmt.Println("  Stderr log:", stderrPath)
	fmt.Println("Tip: set PROVISR_LOG_DIR to choose a custom log directory.")
}
