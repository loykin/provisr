package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	lj "gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel represents the structured log level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Format represents the structured log format
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Default process logging configuration constants
const (
	DefaultMaxSizeMB  = 10 // MB
	DefaultMaxBackups = 3  // number of backup files
	DefaultMaxAgeDays = 7  // days
)

// SlogConfig contains configuration for structured logging (slog)
type SlogConfig struct {
	Level      LogLevel  `json:"level" mapstructure:"level"`           // Log level (debug, info, warn, error)
	Format     Format    `json:"format" mapstructure:"format"`         // Log format (text, json)
	Color      bool      `json:"color" mapstructure:"color"`           // Enable colored output
	TimeStamps bool      `json:"timestamps" mapstructure:"timestamps"` // Include timestamps
	Source     bool      `json:"source" mapstructure:"source"`         // Include source file/line
	Output     io.Writer `json:"-" mapstructure:"-"`                   // Output writer (default: os.Stderr)
}

// FileConfig contains configuration for process file logging
type FileConfig struct {
	Dir        string `json:"dir" mapstructure:"dir"`                 // base directory for logs
	StdoutPath string `json:"stdoutPath" mapstructure:"stdout"`       // explicit stdout path overrides Dir
	StderrPath string `json:"stderrPath" mapstructure:"stderr"`       // explicit stderr path overrides Dir
	MaxSizeMB  int    `json:"maxSizeMB" mapstructure:"max_size_mb"`   // megabytes before rotation (default 10)
	MaxBackups int    `json:"maxBackups" mapstructure:"max_backups"`  // number of backups to keep (default 3)
	MaxAgeDays int    `json:"maxAgeDays" mapstructure:"max_age_days"` // days to keep (default 7)
	Compress   bool   `json:"compress" mapstructure:"compress"`       // Gzip rotated files
}

// Config provides unified configuration by composing SlogConfig and FileConfig
type Config struct {
	Slog SlogConfig `json:"slog" mapstructure:",squash"`
	File FileConfig `json:"file" mapstructure:",squash"`
}

// DefaultConfig returns default unified configuration
func DefaultConfig() Config {
	return Config{
		Slog: SlogConfig{
			Level:      LevelInfo,
			Format:     FormatText,
			Color:      true,
			TimeStamps: true,
			Source:     false,
			Output:     nil,
		},
		File: FileConfig{
			MaxSizeMB:  DefaultMaxSizeMB,
			MaxBackups: DefaultMaxBackups,
			MaxAgeDays: DefaultMaxAgeDays,
			Compress:   false,
		},
	}
}

// NewSlogger creates a structured logger from unified config
func (c *Config) NewSlogger() *slog.Logger {
	output := c.Slog.Output
	if output == nil {
		output = os.Stderr
	}

	var handler slog.Handler

	switch c.Slog.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level:     stringToSlogLevel(string(c.Slog.Level)),
			AddSource: c.Slog.Source,
		})
	case FormatText:
		if c.Slog.Color && isTerminal(output) {
			handler = NewColorTextHandler(output, &slog.HandlerOptions{
				Level:     stringToSlogLevel(string(c.Slog.Level)),
				AddSource: c.Slog.Source,
			}, c.Slog.TimeStamps)
		} else {
			handler = slog.NewTextHandler(output, &slog.HandlerOptions{
				Level:     stringToSlogLevel(string(c.Slog.Level)),
				AddSource: c.Slog.Source,
			})
		}
	default:
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{
			Level:     stringToSlogLevel(string(c.Slog.Level)),
			AddSource: c.Slog.Source,
		})
	}

	return slog.New(handler)
}

// ProcessWriters creates file writers for process stdout/stderr logging
func (c *Config) ProcessWriters(processName string) (stdout, stderr io.WriteCloser, err error) {
	if c.File.Dir == "" && c.File.StdoutPath == "" && c.File.StderrPath == "" {
		return nil, nil, nil
	}

	var outPath, errPath string

	if c.File.StdoutPath != "" {
		outPath = c.File.StdoutPath
	} else if c.File.Dir != "" {
		outPath = filepath.Join(c.File.Dir, processName+".stdout.log")
	}

	if c.File.StderrPath != "" {
		errPath = c.File.StderrPath
	} else if c.File.Dir != "" {
		errPath = filepath.Join(c.File.Dir, processName+".stderr.log")
	}

	if outPath != "" {
		stdout = &lj.Logger{
			Filename:   outPath,
			MaxSize:    c.getMaxSizeMB(),
			MaxBackups: c.getMaxBackups(),
			MaxAge:     c.getMaxAgeDays(),
			Compress:   c.File.Compress,
		}
	}

	if errPath != "" {
		stderr = &lj.Logger{
			Filename:   errPath,
			MaxSize:    c.getMaxSizeMB(),
			MaxBackups: c.getMaxBackups(),
			MaxAge:     c.getMaxAgeDays(),
			Compress:   c.File.Compress,
		}
	}

	return stdout, stderr, nil
}

// NewProcessLogger creates a structured logger for a specific process
func (c *Config) NewProcessLogger(processName string) *slog.Logger {
	logger := c.NewSlogger()
	return logger.With(slog.String("process", processName))
}

func (c *Config) getMaxSizeMB() int {
	if c.File.MaxSizeMB > 0 {
		return c.File.MaxSizeMB
	}
	return DefaultMaxSizeMB
}

func (c *Config) getMaxBackups() int {
	if c.File.MaxBackups > 0 {
		return c.File.MaxBackups
	}
	return DefaultMaxBackups
}

func (c *Config) getMaxAgeDays() int {
	if c.File.MaxAgeDays > 0 {
		return c.File.MaxAgeDays
	}
	return DefaultMaxAgeDays
}

func stringToSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return f.Fd() == uintptr(os.Stdout.Fd()) || f.Fd() == uintptr(os.Stderr.Fd())
	}
	return false
}
