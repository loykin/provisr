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
	root, bind := buildRoot(mgr)
	// set up metrics hook
	bind()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// GlobalFlags holds all global/persistent flags
type GlobalFlags struct {
	ConfigPath       string
	UseOSEnv         bool
	EnvKVs           []string
	EnvFiles         []string
	MetricsListen    string
	StoreDSN         string
	NoStore          bool
	HistDisableStore bool
	HistOSURL        string
	HistOSIndex      string
	HistCHURL        string
	HistCHTable      string
}

// ProcessFlags holds process-related flags
type ProcessFlags struct {
	Name            string
	CmdStr          string
	PIDFile         string
	Retries         int
	RetryInterval   time.Duration
	AutoRestart     bool
	RestartInterval time.Duration
	StartDuration   time.Duration
	Instances       int
}

// GroupCommandFlags holds group-related flags
type GroupCommandFlags struct {
	GroupName string
}

// APIFlags holds API server flags
type APIFlags struct {
	Listen      string
	Base        string
	NonBlocking bool
}

// buildRoot creates the root command with improved structure
func buildRoot(mgr *provisr.Manager) (*cobra.Command, func()) {
	globalFlags := &GlobalFlags{}
	processFlags := &ProcessFlags{}
	groupFlags := &GroupCommandFlags{}
	apiFlags := &APIFlags{}

	provisrCommand := command{mgr: mgr}

	root := createRootCommand(globalFlags)

	// Add subcommands
	root.AddCommand(
		createStartCommand(provisrCommand, globalFlags, processFlags),
		createStatusCommand(provisrCommand, globalFlags, processFlags),
		createStopCommand(provisrCommand, globalFlags, processFlags),
		createCronCommand(provisrCommand, globalFlags),
		createGroupStartCommand(provisrCommand, globalFlags, groupFlags),
		createGroupStopCommand(provisrCommand, globalFlags, groupFlags),
		createGroupStatusCommand(provisrCommand, globalFlags, groupFlags),
		createServeCommand(mgr, globalFlags, apiFlags),
	)

	binder := createPreRunBinder(mgr, globalFlags, root)
	return root, binder
}

// createRootCommand creates the root command with persistent flags
func createRootCommand(flags *GlobalFlags) *cobra.Command {
	root := &cobra.Command{Use: "provisr"}

	// Add persistent flags
	root.PersistentFlags().StringVar(&flags.ConfigPath, "config", "", "path to TOML config file")
	root.PersistentFlags().BoolVar(&flags.UseOSEnv, "use-os-env", false, "inject current OS environment into global env")
	root.PersistentFlags().StringSliceVar(&flags.EnvKVs, "env", nil, "additional KEY=VALUE to inject (repeatable)")
	root.PersistentFlags().StringSliceVar(&flags.EnvFiles, "env-file", nil, "path to .env file(s) with KEY=VALUE lines (repeatable)")
	root.PersistentFlags().StringVar(&flags.MetricsListen, "metrics-listen", "", "address to serve Prometheus /metrics (e.g., :9090)")
	root.PersistentFlags().StringVar(&flags.StoreDSN, "store-dsn", "", "enable persistent store with DSN (e.g., sqlite:///path.db or postgres://...")
	root.PersistentFlags().BoolVar(&flags.NoStore, "no-store", false, "disable persistent store even if configured")

	// History-related flags
	root.PersistentFlags().BoolVar(&flags.HistDisableStore, "history-disable-store", false, "do not record history rows in the persistent store")
	root.PersistentFlags().StringVar(&flags.HistOSURL, "history-opensearch-url", "", "OpenSearch base URL (e.g., http://localhost:9200)")
	root.PersistentFlags().StringVar(&flags.HistOSIndex, "history-opensearch-index", "", "OpenSearch index name for history (e.g., provisr-history)")
	root.PersistentFlags().StringVar(&flags.HistCHURL, "history-clickhouse-url", "", "ClickHouse HTTP endpoint (e.g., http://localhost:8123)")
	root.PersistentFlags().StringVar(&flags.HistCHTable, "history-clickhouse-table", "", "ClickHouse table for history (e.g., default.provisr_history)")

	return root
}

// createStartCommand creates the start subcommand
func createStartCommand(provisrCommand command, globalFlags *GlobalFlags, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start process(es)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Start(StartFlags{
				ConfigPath:      globalFlags.ConfigPath,
				UseOSEnv:        globalFlags.UseOSEnv,
				EnvKVs:          globalFlags.EnvKVs,
				EnvFiles:        globalFlags.EnvFiles,
				Name:            processFlags.Name,
				Cmd:             processFlags.CmdStr,
				PIDFile:         processFlags.PIDFile,
				Retries:         processFlags.Retries,
				RetryInterval:   processFlags.RetryInterval,
				AutoRestart:     processFlags.AutoRestart,
				RestartInterval: processFlags.RestartInterval,
				StartDuration:   processFlags.StartDuration,
				Instances:       processFlags.Instances,
			})
		},
	}

	// Add flags specific to start command
	cmd.Flags().StringVar(&processFlags.Name, "name", "demo", "process name")
	cmd.Flags().StringVar(&processFlags.CmdStr, "cmd", "sleep 60", "command to run")
	cmd.Flags().StringVar(&processFlags.PIDFile, "pidfile", "", "optional pidfile path")
	cmd.Flags().IntVar(&processFlags.Retries, "retries", 0, "retry count on start failure")
	cmd.Flags().DurationVar(&processFlags.RetryInterval, "retry-interval", 500*time.Millisecond, "retry interval on start failure")
	cmd.Flags().BoolVar(&processFlags.AutoRestart, "autorestart", false, "restart automatically if the process dies")
	cmd.Flags().DurationVar(&processFlags.RestartInterval, "restart-interval", time.Second, "interval before auto-restart")
	cmd.Flags().DurationVar(&processFlags.StartDuration, "startsecs", 0, "time the process must stay up to be considered started")
	cmd.Flags().IntVar(&processFlags.Instances, "instances", 1, "number of instances to start")

	return cmd
}

// createStatusCommand creates the status subcommand
func createStatusCommand(provisrCommand command, globalFlags *GlobalFlags, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Status(StatusFlags{
				ConfigPath: globalFlags.ConfigPath,
				Name:       processFlags.Name,
				Detailed:   cmd.Flag("detailed").Changed,
				Watch:      cmd.Flag("watch").Changed,
				Interval:   3 * time.Second, // Default watch interval
			})
		},
	}
	cmd.Flags().StringVar(&processFlags.Name, "name", "demo", "process name")
	cmd.Flags().Bool("detailed", false, "show detailed status including state machine info")
	cmd.Flags().Bool("watch", false, "continuously monitor process status")
	return cmd
}

// createStopCommand creates the stop subcommand
func createStopCommand(provisrCommand command, globalFlags *GlobalFlags, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop process(es)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Stop(StopFlags{
				ConfigPath: globalFlags.ConfigPath,
				Name:       processFlags.Name,
				Wait:       3 * time.Second,
			})
		},
	}
	cmd.Flags().StringVar(&processFlags.Name, "name", "demo", "process name")
	return cmd
}

// createCronCommand creates the cron subcommand
func createCronCommand(provisrCommand command, globalFlags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "cron",
		Short: "Run cron jobs from config (requires --config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Cron(CronFlags{ConfigPath: globalFlags.ConfigPath})
		},
	}
}

// createGroupStartCommand creates the group-start subcommand
func createGroupStartCommand(provisrCommand command, globalFlags *GlobalFlags, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-start",
		Short: "Start a group from config (requires --config and --group)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStart(GroupFlags{
				ConfigPath: globalFlags.ConfigPath,
				GroupName:  groupFlags.GroupName,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name from config")
	return cmd
}

// createGroupStopCommand creates the group-stop subcommand
func createGroupStopCommand(provisrCommand command, globalFlags *GlobalFlags, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-stop",
		Short: "Stop a group from config (requires --config and --group)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStop(GroupFlags{
				ConfigPath: globalFlags.ConfigPath,
				GroupName:  groupFlags.GroupName,
				Wait:       3 * time.Second,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name from config")
	return cmd
}

// createGroupStatusCommand creates the group-status subcommand
func createGroupStatusCommand(provisrCommand command, globalFlags *GlobalFlags, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-status",
		Short: "Show status for a group from config (requires --config and --group)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStatus(GroupFlags{
				ConfigPath: globalFlags.ConfigPath,
				GroupName:  groupFlags.GroupName,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name from config")
	return cmd
}

// createServeCommand creates the serve subcommand
func createServeCommand(mgr *provisr.Manager, globalFlags *GlobalFlags, apiFlags *APIFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP API server (reads http_api from --config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServeCommand(mgr, globalFlags.ConfigPath, apiFlags)
		},
	}

	cmd.Flags().StringVar(&apiFlags.Listen, "api-listen", "", "address to listen for HTTP API (e.g., :8080)")
	cmd.Flags().StringVar(&apiFlags.Base, "api-base", "", "base path for API endpoints (default from config or /api)")
	cmd.Flags().BoolVar(&apiFlags.NonBlocking, "non-blocking", false, "do not block; return immediately (useful for tests)")

	return cmd
}

// runServeCommand handles the serve command logic
func runServeCommand(mgr *provisr.Manager, configPath string, apiFlags *APIFlags) error {
	listen := apiFlags.Listen
	base := apiFlags.Base

	if configPath != "" {
		if httpCfg, err := provisr.LoadHTTPAPI(configPath); err == nil && httpCfg != nil {
			if listen == "" {
				listen = httpCfg.Listen
			}
			if base == "" {
				base = httpCfg.BasePath
			}
			// If config explicitly disables, require explicit flag to override
			if !httpCfg.Enabled && apiFlags.Listen == "" {
				return fmt.Errorf("http_api.enabled=false (or missing); provide --api-listen to start anyway")
			}
		}
	}

	if listen == "" {
		listen = ":8080"
	}
	if base == "" {
		base = "/api"
	}

	if _, err := provisr.NewHTTPServer(listen, base, mgr); err != nil {
		return err
	}

	if apiFlags.NonBlocking {
		return nil
	}

	select {} // Block forever
}

// createPreRunBinder creates the function that binds PersistentPreRun logic
func createPreRunBinder(mgr *provisr.Manager, globalFlags *GlobalFlags, root *cobra.Command) func() {
	return func() {
		root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			setupPersistentStore(mgr, globalFlags)
			setupHistorySinks(mgr, globalFlags)
			setupBackgroundServices(mgr, globalFlags)
		}
	}
}

// setupPersistentStore configures the persistent store based on flags/config
func setupPersistentStore(mgr *provisr.Manager, flags *GlobalFlags) string {
	if flags.NoStore {
		mgr.DisableStore()
		return ""
	}

	dsn := flags.StoreDSN
	if dsn == "" && flags.ConfigPath != "" {
		if sc, err := provisr.LoadStore(flags.ConfigPath); err == nil && sc != nil && sc.Enabled && sc.DSN != "" {
			dsn = sc.DSN
		}
	}

	if dsn != "" {
		_ = mgr.SetStoreFromDSN(dsn)
		return dsn
	}
	return ""
}

// setupHistorySinks configures history sinks based on flags/config
func setupHistorySinks(mgr *provisr.Manager, flags *GlobalFlags) {
	currentDSN := setupPersistentStore(mgr, flags)

	// Load config-based sinks
	var cfgSinks []provisr.HistorySink
	var inStoreEnabled *bool

	if flags.ConfigPath != "" {
		if hc, err := provisr.LoadHistory(flags.ConfigPath); err == nil && hc != nil {
			inStoreEnabled = hc.InStore
			if hc.Enabled {
				if hc.OpenSearchURL != "" && hc.OpenSearchIndex != "" {
					cfgSinks = append(cfgSinks, provisr.NewOpenSearchHistorySink(hc.OpenSearchURL, hc.OpenSearchIndex))
				}
				if hc.ClickHouseURL != "" && hc.ClickHouseTable != "" {
					cfgSinks = append(cfgSinks, provisr.NewClickHouseHistorySink(hc.ClickHouseURL, hc.ClickHouseTable))
				}
			}
		}
	}

	// Add flag-based sinks
	var flagSinks []provisr.HistorySink
	if flags.HistOSURL != "" && flags.HistOSIndex != "" {
		flagSinks = append(flagSinks, provisr.NewOpenSearchHistorySink(flags.HistOSURL, flags.HistOSIndex))
	}
	if flags.HistCHURL != "" && flags.HistCHTable != "" {
		flagSinks = append(flagSinks, provisr.NewClickHouseHistorySink(flags.HistCHURL, flags.HistCHTable))
	}

	// Add store-backed SQL sink if appropriate
	if !flags.HistDisableStore && currentDSN != "" {
		// Default is enabled when InStore is nil; enable if explicitly true too
		var enableStoreSink bool
		if inStoreEnabled == nil {
			enableStoreSink = true // default behavior
		} else {
			enableStoreSink = *inStoreEnabled
		}

		if enableStoreSink {
			if ss := provisr.NewSQLHistorySinkFromDSN(currentDSN); ss != nil {
				cfgSinks = append(cfgSinks, ss)
			}
		}
	}

	// Set the appropriate sinks
	if len(flagSinks) > 0 {
		mgr.SetHistorySinks(flagSinks...)
	} else if len(cfgSinks) > 0 {
		mgr.SetHistorySinks(cfgSinks...)
	}
}

// setupBackgroundServices starts background services (reconciler, metrics)
func setupBackgroundServices(mgr *provisr.Manager, flags *GlobalFlags) {
	// Start background reconciler (idempotent)
	mgr.StartReconciler(2 * time.Second)

	// Start metrics server if requested
	if flags.MetricsListen != "" {
		go func() {
			_ = provisr.RegisterMetricsDefault()
			_ = provisr.ServeMetrics(flags.MetricsListen)
		}()
	}
}
