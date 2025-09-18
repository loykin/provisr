package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/loykin/provisr"
	"github.com/loykin/provisr/internal/config"
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

// GlobalFlags holds minimal global/persistent flags for CLI commands
type GlobalFlags struct {
	ConfigPath string // Only config path for CLI commands
}

// ServeFlags holds minimal serve-specific flags (mostly config path)
type ServeFlags struct {
	ConfigPath string
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
	// API connection
	APIUrl     string
	APITimeout time.Duration
}

// GroupCommandFlags holds group-related flags
type GroupCommandFlags struct {
	GroupName string
}

// buildRoot creates the root command with improved structure
func buildRoot(mgr *provisr.Manager) (*cobra.Command, func()) {
	globalFlags := &GlobalFlags{}
	processFlags := &ProcessFlags{}
	groupFlags := &GroupCommandFlags{}

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
		createServeCommand(globalFlags),
	)

	return root, func() {
		// No complex pre-run setup needed for simplified CLI
	}
}

// createRootCommand creates the root command with minimal persistent flags
func createRootCommand(flags *GlobalFlags) *cobra.Command {
	root := &cobra.Command{
		Use:   "provisr",
		Short: "Process management and supervision tool",
		Long: `Provisr is a lightweight process manager for starting, stopping, 
and monitoring processes locally or via remote daemon connection.

Examples:
  provisr start --name=myapp --cmd="python app.py"
  provisr status --name=myapp
  provisr serve                     # Start daemon
  provisr status --api-url=http://remote:8080/api  # Remote status`,
	}

	// Only essential flags for CLI commands
	root.PersistentFlags().StringVar(&flags.ConfigPath, "config", "", "path to TOML config file (optional)")

	return root
}

// createStartCommand creates the start subcommand
func createStartCommand(provisrCommand command, globalFlags *GlobalFlags, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a process",
		Long: `Start a process with the specified name and command.

Examples:
  provisr start --name=web --cmd="python app.py"
  provisr start --name=api --cmd="node server.js" --instances=3
  provisr start --name=worker --cmd="./worker" --autorestart`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Start(StartFlags{
				ConfigPath:      globalFlags.ConfigPath,
				Name:            processFlags.Name,
				Cmd:             processFlags.CmdStr,
				PIDFile:         processFlags.PIDFile,
				Retries:         processFlags.Retries,
				RetryInterval:   processFlags.RetryInterval,
				AutoRestart:     processFlags.AutoRestart,
				APIUrl:          processFlags.APIUrl,
				APITimeout:      processFlags.APITimeout,
				RestartInterval: processFlags.RestartInterval,
				StartDuration:   processFlags.StartDuration,
				Instances:       processFlags.Instances,
			})
		},
	}

	// Add flags specific to start command
	cmd.Flags().StringVar(&processFlags.Name, "name", "demo", "process name")
	cmd.Flags().StringVar(&processFlags.CmdStr, "cmd", "sleep 60", "command to execute")
	cmd.Flags().StringVar(&processFlags.PIDFile, "pidfile", "", "pidfile path (optional)")
	cmd.Flags().IntVar(&processFlags.Retries, "retries", 0, "retry attempts on failure")
	cmd.Flags().DurationVar(&processFlags.RetryInterval, "retry-interval", 500*time.Millisecond, "retry delay")
	cmd.Flags().BoolVar(&processFlags.AutoRestart, "autorestart", false, "auto-restart on exit")
	cmd.Flags().DurationVar(&processFlags.RestartInterval, "restart-interval", time.Second, "restart delay")
	cmd.Flags().DurationVar(&processFlags.StartDuration, "startsecs", 0, "time to stay up to be 'started'")
	cmd.Flags().IntVar(&processFlags.Instances, "instances", 1, "number of instances")

	// Remote daemon connection
	cmd.Flags().StringVar(&processFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&processFlags.APITimeout, "api-timeout", 10*time.Second, "request timeout")

	return cmd
}

// createStatusCommand creates the status subcommand
func createStatusCommand(provisrCommand command, globalFlags *GlobalFlags, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show process status",
		Long: `Show the status of processes managed by provisr.

Examples:
  provisr status                    # Show all processes
  provisr status --name=web         # Show specific process
  provisr status --watch            # Live monitoring
  provisr status --api-url=http://remote:8080/api  # Remote status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Status(StatusFlags{
				ConfigPath: globalFlags.ConfigPath,
				Name:       processFlags.Name,
				APIUrl:     processFlags.APIUrl,
				APITimeout: processFlags.APITimeout,
				Detailed:   cmd.Flag("detailed").Changed,
				Watch:      cmd.Flag("watch").Changed,
				Interval:   3 * time.Second, // Default watch interval
			})
		},
	}
	cmd.Flags().StringVar(&processFlags.Name, "name", "demo", "process name (optional)")
	cmd.Flags().StringVar(&processFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&processFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
	cmd.Flags().Bool("detailed", false, "show detailed info")
	cmd.Flags().Bool("watch", false, "live monitoring")
	return cmd
}

// createStopCommand creates the stop subcommand
func createStopCommand(provisrCommand command, globalFlags *GlobalFlags, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a process",
		Long: `Stop processes managed by provisr.

Examples:
  provisr stop --name=web           # Stop specific process
  provisr stop                      # Stop all processes
  provisr stop --api-url=http://remote:8080/api  # Remote stop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Stop(StopFlags{
				ConfigPath: globalFlags.ConfigPath,
				Name:       processFlags.Name,
				APIUrl:     processFlags.APIUrl,
				APITimeout: processFlags.APITimeout,
				Wait:       3 * time.Second,
			})
		},
	}
	cmd.Flags().StringVar(&processFlags.Name, "name", "demo", "process name (optional)")
	cmd.Flags().StringVar(&processFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&processFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
	return cmd
}

// createCronCommand creates the cron subcommand
func createCronCommand(provisrCommand command, globalFlags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "cron",
		Short: "Run scheduled jobs from config",
		Long: `Execute cron jobs defined in the configuration file.

Example:
  provisr cron --config=config.toml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Cron(CronFlags{ConfigPath: globalFlags.ConfigPath})
		},
	}
}

// createGroupStartCommand creates the group-start subcommand
func createGroupStartCommand(provisrCommand command, globalFlags *GlobalFlags, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-start",
		Short: "Start a process group",
		Long: `Start all processes in a named group from config file.

Example:
  provisr group-start --config=config.toml --group=webstack`,
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
		Short: "Stop a process group",
		Long: `Stop all processes in a named group from config file.

Example:
  provisr group-stop --config=config.toml --group=webstack`,
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
		Short: "Show group status",
		Long: `Show status of all processes in a named group from config file.

Example:
  provisr group-status --config=config.toml --group=webstack`,
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
func createServeCommand(globalFlags *GlobalFlags) *cobra.Command {
	serveFlags := &ServeFlags{
		ConfigPath: globalFlags.ConfigPath,
	}

	cmd := &cobra.Command{
		Use:   "serve [config.toml]",
		Short: "Start the provisr daemon",
		Long: `Start the provisr daemon server to manage processes.
All configuration is loaded from config.toml file.

Examples:
  provisr serve                     # Start daemon (uses --config)
  provisr serve config.toml         # Start with specific config file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleServeCommand(serveFlags, args)
		},
	}

	// No flags needed - everything configured via TOML file

	return cmd
}

func runSimpleServeCommand(flags *ServeFlags, args []string) error {
	configPath := flags.ConfigPath
	if len(args) > 0 {
		configPath = args[0]
	}

	if configPath == "" {
		return fmt.Errorf("config file required for serve command. Use --config=config.toml or provide as argument")
	}

	// Create manager
	mgr := provisr.New()

	// Load and apply global environment from config
	if globalEnv, err := provisr.LoadGlobalEnv(configPath); err == nil {
		mgr.SetGlobalEnv(globalEnv)
	}

	// Load and setup store from config
	if storeCfg, err := config.LoadStoreFromTOML(configPath); err == nil && storeCfg != nil && storeCfg.Enabled {
		if err := mgr.SetStoreFromDSN(storeCfg.DSN); err != nil {
			return fmt.Errorf("error setting up store: %w", err)
		}
	}

	// Load and setup history from config
	var historySinks []provisr.HistorySink
	if historyCfg, err := config.LoadHistoryFromTOML(configPath); err == nil && historyCfg != nil && historyCfg.Enabled {
		if historyCfg.OpenSearchURL != "" && historyCfg.OpenSearchIndex != "" {
			historySinks = append(historySinks, provisr.NewOpenSearchHistorySink(historyCfg.OpenSearchURL, historyCfg.OpenSearchIndex))
		}
		if historyCfg.ClickHouseURL != "" && historyCfg.ClickHouseTable != "" {
			historySinks = append(historySinks, provisr.NewClickHouseHistorySink(historyCfg.ClickHouseURL, historyCfg.ClickHouseTable))
		}

		// Note: Store history control would need additional Manager method
		// For now, we assume it's enabled by default if store is enabled
	}

	if len(historySinks) > 0 {
		mgr.SetHistorySinks(historySinks...)
	}

	// Setup metrics from config
	if metricsCfg, err := config.LoadMetricsFromTOML(configPath); err == nil && metricsCfg != nil && metricsCfg.Enabled {
		if metricsCfg.Listen != "" {
			go func() {
				if err := provisr.ServeMetrics(metricsCfg.Listen); err != nil {
					fmt.Printf("Metrics server error: %v\n", err)
				}
			}()
		}
	}

	// Load HTTP API config and start server
	httpCfg, err := config.LoadHTTPAPIFromTOML(configPath)
	if err != nil {
		return fmt.Errorf("error loading HTTP API config: %w", err)
	}

	if httpCfg == nil || !httpCfg.Enabled {
		return fmt.Errorf("HTTP API must be enabled in config file to run serve command")
	}

	// Create and start HTTP server
	fmt.Printf("Starting provisr server on %s%s\n", httpCfg.Listen, httpCfg.BasePath)
	server, err := provisr.NewHTTPServer(httpCfg.Listen, httpCfg.BasePath, mgr)
	if err != nil {
		return fmt.Errorf("failed to create HTTP server: %w", err)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	return server.Close()
}
