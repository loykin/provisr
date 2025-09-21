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

// ProcessFlags holds process-related flags
type ProcessFlags struct {
	Name            string
	CmdStr          string
	PIDFile         string
	Retries         uint32
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
	GroupName  string
	APIUrl     string
	APITimeout time.Duration
}

// buildRoot creates the root command with improved structure
func buildRoot(mgr *provisr.Manager) (*cobra.Command, func()) {
	globalFlags := &GlobalFlags{}
	processFlags := &ProcessFlags{}
	groupFlags := &GroupCommandFlags{}
	cronFlags := &CronFlags{}

	provisrCommand := command{mgr: mgr}

	root := createRootCommand(globalFlags)

	// Add subcommands
	root.AddCommand(
		createStartCommand(provisrCommand, globalFlags, processFlags),
		createStatusCommand(provisrCommand, globalFlags, processFlags),
		createStopCommand(provisrCommand, globalFlags, processFlags),
		createCronCommand(provisrCommand, globalFlags, cronFlags),
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
	cmd.Flags().Uint32Var(&processFlags.Retries, "retries", 0, "retry attempts on failure")
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
	cmd.Flags().StringVar(&processFlags.Name, "name", "", "process name (optional)")
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
	cmd.Flags().StringVar(&processFlags.Name, "name", "", "process name (optional)")
	cmd.Flags().StringVar(&processFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&processFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
	return cmd
}

// createCronCommand creates the cron subcommand
func createCronCommand(provisrCommand command, globalFlags *GlobalFlags, cronFlags *CronFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Control scheduled jobs via daemon (REST)",
		Long: `Cron jobs are executed by the provisr daemon started with 'serve'.
This command communicates with the running daemon via REST to verify readiness.

Examples:
  provisr cron --config=config.toml                 # Verify daemon is running and has loaded cron jobs
  provisr cron --config=config.toml --api-url=http://remote:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Cron(CronFlags{
				ConfigPath:  globalFlags.ConfigPath,
				APIUrl:      cronFlags.APIUrl,
				APITimeout:  cronFlags.APITimeout,
				NonBlocking: true, // CLI should not block; daemon runs scheduler
			})
		},
	}
	cmd.Flags().StringVar(&cronFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&cronFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
	return cmd
}

// createGroupStartCommand creates the group-start subcommand
func createGroupStartCommand(provisrCommand command, globalFlags *GlobalFlags, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-start",
		Short: "Start a process group",
		Long: `Start all processes in a named group from config file.

Example:
  provisr group-start --config=config.toml --group=webstack --api-url=http://127.0.0.1:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStart(GroupFlags{
				ConfigPath: globalFlags.ConfigPath,
				GroupName:  groupFlags.GroupName,
				APIUrl:     groupFlags.APIUrl,
				APITimeout: groupFlags.APITimeout,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name from config")
	cmd.Flags().StringVar(&groupFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&groupFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
	return cmd
}

// createGroupStopCommand creates the group-stop subcommand
func createGroupStopCommand(provisrCommand command, globalFlags *GlobalFlags, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-stop",
		Short: "Stop a process group",
		Long: `Stop all processes in a named group from config file.

Example:
  provisr group-stop --config=config.toml --group=webstack --api-url=http://127.0.0.1:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStop(GroupFlags{
				ConfigPath: globalFlags.ConfigPath,
				GroupName:  groupFlags.GroupName,
				APIUrl:     groupFlags.APIUrl,
				APITimeout: groupFlags.APITimeout,
				Wait:       3 * time.Second,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name from config")
	cmd.Flags().StringVar(&groupFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&groupFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
	return cmd
}

// createGroupStatusCommand creates the group-status subcommand
func createGroupStatusCommand(provisrCommand command, globalFlags *GlobalFlags, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-status",
		Short: "Show group status",
		Long: `Show status of all processes in a named group from config file.

Example:
  provisr group-status --config=config.toml --group=webstack --api-url=http://127.0.0.1:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStatus(GroupFlags{
				ConfigPath: globalFlags.ConfigPath,
				GroupName:  groupFlags.GroupName,
				APIUrl:     groupFlags.APIUrl,
				APITimeout: groupFlags.APITimeout,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name from config")
	cmd.Flags().StringVar(&groupFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&groupFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
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
  provisr serve config.toml         # Start with specific config file
  provisr serve --daemonize         # Run as daemon in background
  provisr serve --daemonize --pidfile=/var/run/provisr.pid  # Daemon with PID file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleServeCommand(serveFlags, args)
		},
	}

	// Add daemonize flags
	cmd.Flags().BoolVar(&serveFlags.Daemonize, "daemonize", false, "run as daemon in background")
	cmd.Flags().StringVar(&serveFlags.PidFile, "pidfile", "", "write daemon PID to file")
	cmd.Flags().StringVar(&serveFlags.LogFile, "logfile", "", "redirect daemon logs to file")

	return cmd
}

func runSimpleServeCommand(flags *ServeFlags, args []string) error {
	// Handle daemonization first
	if flags.Daemonize {
		return daemonize(flags.PidFile, flags.LogFile)
	}

	configPath := flags.ConfigPath
	if len(args) > 0 {
		configPath = args[0]
	}

	if configPath == "" {
		return fmt.Errorf("config file required for serve command. Use --config=config.toml or provide as argument")
	}

	// Setup daemon cleanup if running as daemon (child process)
	if flags.PidFile != "" && os.Getppid() == 1 {
		defer func() { _ = removePidFile(flags.PidFile) }()
	}

	// Create manager
	mgr := provisr.New()

	// Load unified config once
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Apply global environment
	// Set global environment - 직접 필드 접근
	mgr.SetGlobalEnv(cfg.GlobalEnv)

	// Setup store from config
	if cfg.Store != nil && cfg.Store.Enabled {
		if err := mgr.SetStoreFromDSN(cfg.Store.DSN); err != nil {
			return fmt.Errorf("error setting up store: %w", err)
		}
	}

	// Setup history from config
	var historySinks []provisr.HistorySink
	if cfg.History != nil && cfg.History.Enabled {
		if cfg.History.OpenSearchURL != "" && cfg.History.OpenSearchIndex != "" {
			historySinks = append(historySinks, provisr.NewOpenSearchHistorySink(cfg.History.OpenSearchURL, cfg.History.OpenSearchIndex))
		}
		if cfg.History.ClickHouseURL != "" && cfg.History.ClickHouseTable != "" {
			historySinks = append(historySinks, provisr.NewClickHouseHistorySink(cfg.History.ClickHouseURL, cfg.History.ClickHouseTable))
		}

		// Note: Store history control would need additional Manager method
		// For now, we assume it's enabled by default if store is enabled
	}

	if len(historySinks) > 0 {
		mgr.SetHistorySinks(historySinks...)
	}

	// Setup metrics from config
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		if cfg.Metrics.Listen != "" {
			go func() {
				if err := provisr.ServeMetrics(cfg.Metrics.Listen); err != nil {
					fmt.Printf("Metrics server error: %v\n", err)
				}
			}()
		}
	}

	// Check Server config (was HTTP config)
	if cfg.Server == nil {
		return fmt.Errorf("server must be configured to run serve command")
	}

	// Auto-start all processes from config
	for _, spec := range cfg.Specs {
		if err := mgr.StartN(spec); err != nil {
			fmt.Printf("Warning: failed to start process %s: %v\n", spec.Name, err)
		} else {
			fmt.Printf("Auto-started process: %s\n", spec.Name)
		}
	}

	// Start cron scheduler (if any cron jobs are defined)
	var cronScheduler *provisr.CronScheduler
	if len(cfg.CronJobs) > 0 {
		cronScheduler = provisr.NewCronScheduler(mgr)
		for _, j := range cfg.CronJobs {
			jb := provisr.CronJob{Name: j.Name, Spec: j.Spec, Schedule: j.Schedule, Singleton: j.Singleton}
			if err := cronScheduler.Add(&jb); err != nil {
				return fmt.Errorf("failed to add cron job %s: %w", j.Name, err)
			}
		}
		if err := cronScheduler.Start(); err != nil {
			return fmt.Errorf("failed to start cron scheduler: %w", err)
		}
		fmt.Printf("Started cron scheduler with %d job(s)\n", len(cfg.CronJobs))
	}

	// Create and start HTTP server
	fmt.Printf("Starting provisr server on %s%s\n", cfg.Server.Listen, cfg.Server.BasePath)
	server, err := provisr.NewHTTPServer(cfg.Server.Listen, cfg.Server.BasePath, mgr)
	if err != nil {
		return fmt.Errorf("failed to create HTTP server: %w", err)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	if cronScheduler != nil {
		cronScheduler.Stop()
	}
	return server.Close()
}
