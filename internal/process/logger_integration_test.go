package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/logger"
)

func TestProcessLoggingStdoutStderr(t *testing.T) {
	mgr := NewManager()
	dir := t.TempDir()
	spec := Spec{
		Name:    "logdemo",
		Command: "sh -c 'echo out; echo err 1>&2; sleep 0.1'",
		Log:     logger.Config{Dir: dir},
	}
	if err := mgr.Start(spec); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	// Verify files exist with content
	outPath := filepath.Join(dir, "logdemo.stdout.log")
	errPath := filepath.Join(dir, "logdemo.stderr.log")
	ob, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	eb, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if !strings.Contains(string(ob), "out") {
		t.Fatalf("stdout missing content: %q", string(ob))
	}
	if !strings.Contains(string(eb), "err") {
		t.Fatalf("stderr missing content: %q", string(eb))
	}
}
