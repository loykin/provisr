package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/loykin/provisr/internal/logger"
)

func main() {
	// Create a unified logging configuration with properly separated concerns
	unifiedCfg := logger.Config{
		// Structured logging configuration (slog)
		Slog: logger.SlogConfig{
			Level:      logger.LevelInfo,
			Format:     logger.FormatText,
			Color:      true,
			TimeStamps: true,
			Source:     true,
		},
		// Process file logging configuration
		File: logger.FileConfig{
			Dir:        "/tmp/provisr-unified-demo",
			MaxSizeMB:  5,
			MaxBackups: 2,
			MaxAgeDays: 1,
			Compress:   false,
		},
	}

	// Create structured logger for the application
	appLogger := unifiedCfg.NewSlogger()
	slog.SetDefault(appLogger)

	slog.Info("=== Provisr Unified Logging Demo ===")
	slog.Info("This demo shows the unified slog-based logging system")

	// Demo structured logging with various levels and attributes
	slog.Debug("Debug message (may not appear unless level is debug)")
	slog.Info("Info message with attributes",
		slog.String("feature", "unified_logging"),
		slog.Bool("slog_standard", true),
		slog.Int("version", 2))
	slog.Warn("Warning message", slog.String("status", "deprecated_types"))

	// Show color formatting
	if unifiedCfg.Slog.Color {
		slog.Info("Colored output enabled", slog.String("terminal", "supports_ansi"))
	}

	// Demonstrate process logger creation from same config
	processLogger := unifiedCfg.NewProcessLogger("demo-process")

	if processLogger != nil {
		slog.Info("Process logger created",
			slog.String("process", "demo-process"),
			slog.String("log_dir", unifiedCfg.File.Dir)) // Simulate process logging with structured log
		processLogger.Info("Process started",
			slog.Int("pid", 12345),
			slog.String("command", "demo-command"))

		processLogger.Warn("Process warning",
			slog.String("warning", "resource_usage_high"))

		time.Sleep(100 * time.Millisecond) // Give logger time to write
	}

	slog.Info("=== Demo Complete ===")
	slog.Info("Key benefits of unified logging:")
	slog.Info("  ✅ Single configuration for all logging")
	slog.Info("  ✅ slog as the standard foundation")
	slog.Info("  ✅ Colored terminal output")
	slog.Info("  ✅ Consistent structured logging")
	slog.Info("  ✅ Process file logging integration")

	// Cleanup
	if unifiedCfg.File.Dir != "" {
		// #nosec 104 example
		_ = os.RemoveAll(unifiedCfg.File.Dir)
		slog.Info("Cleaned up demo directory", slog.String("path", unifiedCfg.File.Dir))
	}
}
