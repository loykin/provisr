// Package logger re-exports logger types from core for external consumers.
package logger

import (
	"log/slog"

	"github.com/loykin/provisr/core"
)

type (
	Config     = core.LogConfig
	SlogConfig = core.LogSlogConfig
	FileConfig = core.LogFileConfig
	LogLevel   = core.LogLevel
	Format     = core.LogFormat
)

const (
	LevelDebug = core.LogLevelDebug
	LevelInfo  = core.LogLevelInfo
	LevelWarn  = core.LogLevelWarn
	LevelError = core.LogLevelError

	FormatText = core.LogFormatText
	FormatJSON = core.LogFormatJSON
)

// DefaultConfig returns the default logger configuration.
func DefaultConfig() Config { return core.DefaultLogConfig() }

// NewSlogger is a convenience wrapper that builds a *slog.Logger from cfg.
func NewSlogger(cfg Config) *slog.Logger { return cfg.NewSlogger() }
