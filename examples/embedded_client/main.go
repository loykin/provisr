package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/loykin/provisr/internal/logger"
	"github.com/loykin/provisr/pkg/client"
)

func main() {
	// Create a colorful slog logger for demonstration using unified config
	logCfg := logger.DefaultConfig()
	logCfg.Slog.Level = logger.LevelInfo
	logCfg.Slog.Format = logger.FormatText
	logCfg.Slog.Color = true
	logCfg.Slog.TimeStamps = true
	logCfg.Slog.Source = false

	// In CI, use plain text without color
	if os.Getenv("CI") == "true" {
		logCfg.Slog.Color = false
	}

	slogger := logCfg.NewSlogger()
	slog.SetDefault(slogger)

	slog.Info("Starting provisr client demo",
		slog.Bool("color_enabled", logCfg.Slog.Color),
		slog.String("format", string(logCfg.Slog.Format)),
	)

	// Create a provisr client with logger
	cfg := client.DefaultConfig()
	cfg.Logger = slogger
	provisrClient := client.New(cfg)

	ctx := context.Background()

	// In CI environment, be more tolerant of daemon not being available
	if os.Getenv("CI") == "true" {
		slog.Info("CI environment detected - checking daemon connectivity with timeout")

		// Use a shorter context timeout in CI
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		if !provisrClient.IsReachable(timeoutCtx) {
			slog.Warn("Provisr daemon not reachable in CI environment")
			slog.Info("This is expected if running without daemon setup")
			slog.Info("In production, ensure daemon is running with: provisr serve daemon-config.toml")
			return
		}
	} else {
		// Check if provisr daemon is reachable
		if !provisrClient.IsReachable(ctx) {
			slog.Error("Provisr daemon not reachable",
				slog.String("command", "provisr serve examples/embedded_client/daemon-config.toml"),
			)
			os.Exit(1)
		}
	}

	slog.Info("Connected to provisr daemon successfully")

	// Start a process
	startReq := client.StartRequest{
		Name:      "my-worker",
		Command:   "sleep 5", // Shorter duration for CI
		Instances: 1,         // Fewer instances for CI
	}

	slog.Info("Starting process",
		slog.String("name", startReq.Name),
		slog.String("command", startReq.Command),
	)

	if err := provisrClient.StartProcess(ctx, startReq); err != nil {
		slog.Error("Start failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Process started successfully", slog.String("name", startReq.Name))

	// In CI, give some time for process to run then exit
	if os.Getenv("CI") == "true" {
		slog.Info("CI mode - waiting briefly for process to run")
		time.Sleep(2 * time.Second)
		slog.Info("Example completed successfully in CI environment")
	} else {
		slog.Info("Demo completed - process is running in background")
		slog.Info("Check status with: curl http://localhost:8080/api/status")
	}
}
