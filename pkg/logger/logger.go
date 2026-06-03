// Package logger re-exports internal/logger for external consumers.
package logger

import (
	"log/slog"

	"github.com/loykin/provisr/internal/logger"
)

type (
	Config     = logger.Config
	SlogConfig = logger.SlogConfig
	FileConfig = logger.FileConfig
	LogLevel   = logger.LogLevel
	Format     = logger.Format
)

const (
	LevelDebug = logger.LevelDebug
	LevelInfo  = logger.LevelInfo
	LevelWarn  = logger.LevelWarn
	LevelError = logger.LevelError

	FormatText = logger.FormatText
	FormatJSON = logger.FormatJSON
)

// DefaultConfig returns the default logger configuration.
func DefaultConfig() Config { return logger.DefaultConfig() }

// NewSlogger is a convenience wrapper that builds a *slog.Logger from cfg.
func NewSlogger(cfg Config) *slog.Logger { return cfg.NewSlogger() }
