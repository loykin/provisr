package logger

import (
	"fmt"
	"io"
	"path/filepath"

	lj "gopkg.in/natefinch/lumberjack.v2"
)

// Default logging configuration constants
const (
	DefaultMaxSizeMB  = 10 // MB
	DefaultMaxBackups = 3  // number of backup files
	DefaultMaxAgeDays = 7  // days
)

// Config describes logging destinations for a process.
// If StdoutPath/StderrPath are empty, and Dir is set, files will be
// Dir/<name>.stdout.log and Dir/<name>.stderr.log
// Rotation parameters follow lumberjack semantics.
type Config struct {
	Dir        string // base directory for logs
	StdoutPath string // explicit stdout path overrides Dir
	StderrPath string // explicit stderr path overrides Dir
	MaxSizeMB  int    // megabytes before rotation (default 10)
	MaxBackups int    // number of backups to keep (default 3)
	MaxAgeDays int    // days to keep (default 7)
	Compress   bool   // Gzip rotated files
}

// Writers returns io.WriteClosers for stdout and stderr for given process name.
// name may include instance suffix (e.g., web-1).
func (c Config) Writers(name string) (io.WriteCloser, io.WriteCloser, error) {
	stdout := c.StdoutPath
	stderr := c.StderrPath
	if stdout == "" && c.Dir != "" {
		stdout = filepath.Join(c.Dir, fmt.Sprintf("%s.stdout.log", name))
	}
	if stderr == "" && c.Dir != "" {
		stderr = filepath.Join(c.Dir, fmt.Sprintf("%s.stderr.log", name))
	}
	var outW io.WriteCloser
	var errW io.WriteCloser
	if stdout != "" {
		outW = &lj.Logger{
			Filename:   stdout,
			MaxSize:    valOr(c.MaxSizeMB, DefaultMaxSizeMB),
			MaxBackups: valOr(c.MaxBackups, DefaultMaxBackups),
			MaxAge:     valOr(c.MaxAgeDays, DefaultMaxAgeDays),
			Compress:   c.Compress,
		}
	}
	if stderr != "" {
		errW = &lj.Logger{
			Filename:   stderr,
			MaxSize:    valOr(c.MaxSizeMB, DefaultMaxSizeMB),
			MaxBackups: valOr(c.MaxBackups, DefaultMaxBackups),
			MaxAge:     valOr(c.MaxAgeDays, DefaultMaxAgeDays),
			Compress:   c.Compress,
		}
	}
	return outW, errW, nil
}

func valOr(v int, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
