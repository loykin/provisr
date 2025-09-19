package logger

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	lj "gopkg.in/natefinch/lumberjack.v2"
)

// helper to close non-nil closers and ignore errors
func closeIf(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}

func TestWriters_WithDirOnly(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{File: FileConfig{Dir: dir}}
	outW, errW, err := cfg.ProcessWriters("demo")
	if err != nil {
		t.Fatalf("ProcessWriters error: %v", err)
	}
	if outW == nil || errW == nil {
		t.Fatalf("expected both writers non-nil when Dir is set")
	}
	// Write a bit and close to ensure files are created
	_, _ = outW.Write([]byte("hello-out\n"))
	_, _ = errW.Write([]byte("hello-err\n"))
	closeIf(outW)
	closeIf(errW)
	// Verify files exist at derived paths
	outPath := filepath.Join(dir, "demo.stdout.log")
	errPath := filepath.Join(dir, "demo.stderr.log")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("stdout log not created at %s: %v", outPath, err)
	}
	if _, err := os.Stat(errPath); err != nil {
		t.Fatalf("stderr log not created at %s: %v", errPath, err)
	}
}

func TestWriters_WithExplicitPaths(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "s.out.log")
	ep := filepath.Join(dir, "s.err.log")
	cfg := Config{File: FileConfig{StdoutPath: sp, StderrPath: ep}}
	outW, errW, err := cfg.ProcessWriters("ignored-name")
	if err != nil {
		t.Fatalf("ProcessWriters error: %v", err)
	}
	if outW == nil || errW == nil {
		t.Fatalf("expected both writers non-nil when explicit paths provided")
	}
	_, _ = outW.Write([]byte("x"))
	_, _ = errW.Write([]byte("y"))
	closeIf(outW)
	closeIf(errW)
	if _, err := os.Stat(sp); err != nil {
		t.Fatalf("stdout explicit path not created: %v", err)
	}
	if _, err := os.Stat(ep); err != nil {
		t.Fatalf("stderr explicit path not created: %v", err)
	}
}

func TestWriters_Defaults(t *testing.T) {
	cfg := Config{ /* zero values to trigger defaults */ }
	outW, errW, _ := cfg.ProcessWriters("n")
	// With no Dir and no explicit paths, ProcessWriters returns nils; ensure that's the case
	if outW != nil || errW != nil {
		t.Fatalf("expected nil writers when no Dir/stdout/stderr set")
	}
	// Now set explicit paths to instantiate lumberjack with defaults
	cfg = Config{File: FileConfig{StdoutPath: "x", StderrPath: "y"}}
	outW, errW, _ = cfg.ProcessWriters("n")
	ol, ok1 := outW.(*lj.Logger)
	el, ok2 := errW.(*lj.Logger)
	if !ok1 || !ok2 {
		t.Fatalf("writers are not lumberjack.Logger")
	}
	// defaults
	if ol.MaxSize != 10 || ol.MaxBackups != 3 || ol.MaxAge != 7 {
		t.Fatalf("unexpected defaults: size=%d backups=%d age=%d", ol.MaxSize, ol.MaxBackups, ol.MaxAge)
	}
	if el.MaxSize != 10 || el.MaxBackups != 3 || el.MaxAge != 7 {
		t.Fatalf("unexpected defaults (stderr): size=%d backups=%d age=%d", el.MaxSize, el.MaxBackups, el.MaxAge)
	}
	closeIf(outW)
	closeIf(errW)
}

func TestWriters_Overrides(t *testing.T) {
	// Custom values propagate
	cfg := Config{File: FileConfig{StdoutPath: "x2", StderrPath: "y2", MaxSizeMB: 1, MaxBackups: 9, MaxAgeDays: 11, Compress: true}}
	outW, errW, _ := cfg.ProcessWriters("n")
	ol := outW.(*lj.Logger)
	el := errW.(*lj.Logger)
	if ol.MaxSize != 1 || ol.MaxBackups != 9 || ol.MaxAge != 11 || !ol.Compress {
		t.Fatalf("unexpected overrides: size=%d backups=%d age=%d compress=%t", ol.MaxSize, ol.MaxBackups, ol.MaxAge, ol.Compress)
	}
	if el.MaxSize != 1 || el.MaxBackups != 9 || el.MaxAge != 11 || !el.Compress {
		t.Fatalf("unexpected overrides (stderr): size=%d backups=%d age=%d compress=%t", el.MaxSize, el.MaxBackups, el.MaxAge, el.Compress)
	}
	closeIf(outW)
	closeIf(errW)
}

func TestWriters_OnlyOneStream(t *testing.T) {
	dir := t.TempDir()
	// Only stdout
	cfg := Config{File: FileConfig{StdoutPath: filepath.Join(dir, "only-stdout.log")}}
	outW, errW, _ := cfg.ProcessWriters("n")
	if outW == nil || errW != nil {
		t.Fatalf("expected stdout writer only")
	}
	_, _ = outW.Write([]byte("a"))
	closeIf(outW)
	if _, err := os.Stat(filepath.Join(dir, "only-stdout.log")); err != nil {
		t.Fatalf("stdout not created: %v", err)
	}
	// Only stderr
	cfg = Config{File: FileConfig{StderrPath: filepath.Join(dir, "only-stderr.log")}}
	outW, errW, _ = cfg.ProcessWriters("n")
	if outW != nil || errW == nil {
		t.Fatalf("expected stderr writer only")
	}
	_, _ = errW.Write([]byte("b"))
	closeIf(errW)
	if _, err := os.Stat(filepath.Join(dir, "only-stderr.log")); err != nil {
		t.Fatalf("stderr not created: %v", err)
	}
}
