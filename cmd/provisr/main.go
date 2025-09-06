package main

import (
	"fmt"
	"os"
	"time"

	"github.com/loykin/provisr"
	"github.com/spf13/cobra"
)

func main() {
	mgr := provisr.New()
	// If metrics flag is set, start an HTTP server for Prometheus.
	// We need to capture the flag value after creating root; start server in PersistentPreRun.

	var (
		configPath      string
		name            string
		cmdStr          string
		pidfile         string
		retries         int
		retryInterval   time.Duration
		autoRestart     bool
		restartInterval time.Duration
		startDuration   time.Duration
		instances       int
	)

	root := &cobra.Command{Use: "provisr"}
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to TOML config file")
	// global env injection options
	var useOSEnv bool
	var envKVs []string
	var envFiles []string
	var metricsListen string
	root.PersistentFlags().BoolVar(&useOSEnv, "use-os-env", false, "inject current OS environment into global env")
	root.PersistentFlags().StringSliceVar(&envKVs, "env", nil, "additional KEY=VALUE to inject (repeatable)")
	root.PersistentFlags().StringSliceVar(&envFiles, "env-file", nil, "path to .env file(s) with KEY=VALUE lines (repeatable)")
	root.PersistentFlags().StringVar(&metricsListen, "metrics-listen", "", "address to serve Prometheus /metrics (e.g., :9090)")

	cmdStart := &cobra.Command{
		Use:   "start",
		Short: "Start process(es)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdStart(mgr, StartFlags{
				ConfigPath:      configPath,
				UseOSEnv:        useOSEnv,
				EnvKVs:          envKVs,
				EnvFiles:        envFiles,
				Name:            name,
				Cmd:             cmdStr,
				PIDFile:         pidfile,
				Retries:         retries,
				RetryInterval:   retryInterval,
				AutoRestart:     autoRestart,
				RestartInterval: restartInterval,
				StartDuration:   startDuration,
				Instances:       instances,
			})
		},
	}
	cmdStart.Flags().StringVar(&name, "name", "demo", "process name")
	cmdStart.Flags().StringVar(&cmdStr, "cmd", "sleep 60", "command to run")
	cmdStart.Flags().StringVar(&pidfile, "pidfile", "", "optional pidfile path")
	cmdStart.Flags().IntVar(&retries, "retries", 0, "retry count on start failure")
	cmdStart.Flags().DurationVar(&retryInterval, "retry-interval", 500*time.Millisecond, "retry interval on start failure")
	cmdStart.Flags().BoolVar(&autoRestart, "autorestart", false, "restart automatically if the process dies")
	cmdStart.Flags().DurationVar(&restartInterval, "restart-interval", time.Second, "interval before auto-restart")
	cmdStart.Flags().DurationVar(&startDuration, "startsecs", 0, "time the process must stay up to be considered started")
	cmdStart.Flags().IntVar(&instances, "instances", 1, "number of instances to start")

	cmdStatus := &cobra.Command{
		Use:   "status",
		Short: "Show status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdStatus(mgr, StatusFlags{ConfigPath: configPath, Name: name})
		},
	}
	cmdStatus.Flags().StringVar(&name, "name", "demo", "process name")

	cmdStop := &cobra.Command{
		Use:   "stop",
		Short: "Stop process(es)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdStop(mgr, StopFlags{ConfigPath: configPath, Name: name, Wait: 3 * time.Second})
		},
	}
	cmdStop.Flags().StringVar(&name, "name", "demo", "process name")

	// Cron subcommand: runs scheduled jobs defined in config
	cmdCron := &cobra.Command{
		Use:   "cron",
		Short: "Run cron jobs from config (requires --config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdCron(mgr, CronFlags{ConfigPath: configPath})
		},
	}

	// Group subcommands
	var groupName string
	cmdGroupStart := &cobra.Command{Use: "group-start", Short: "Start a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		return runGroupStart(mgr, GroupFlags{ConfigPath: configPath, GroupName: groupName})
	}}
	cmdGroupStop := &cobra.Command{Use: "group-stop", Short: "Stop a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		return runGroupStop(mgr, GroupFlags{ConfigPath: configPath, GroupName: groupName, Wait: 3 * time.Second})
	}}
	cmdGroupStatus := &cobra.Command{Use: "group-status", Short: "Show status for a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		return runGroupStatus(mgr, GroupFlags{ConfigPath: configPath, GroupName: groupName})
	}}
	cmdGroupStart.Flags().StringVar(&groupName, "group", "", "group name from config")
	cmdGroupStop.Flags().StringVar(&groupName, "group", "", "group name from config")
	cmdGroupStatus.Flags().StringVar(&groupName, "group", "", "group name from config")

	// Start metrics server if requested, using PersistentPreRun
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if metricsListen != "" {
			// lazy import: use provisr wrapper to expose metrics register/handler via internal package? We'll import internal/metrics directly here.
			go func() {
				// Register against default
				_ = provisr.RegisterMetricsDefault()
				_ = provisr.ServeMetrics(metricsListen)
			}()
		}
	}

	root.AddCommand(cmdStart, cmdStatus, cmdStop, cmdCron, cmdGroupStart, cmdGroupStop, cmdGroupStatus)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
