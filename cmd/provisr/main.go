package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
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

// GlobalFlags holds minimal global/persistent flags for CLI commands
type GlobalFlags struct {
	ConfigPath string // Only config path for CLI commands
}

// ProcessFlags holds process-related flags
type ProcessFlags struct {
	Name            string
	CmdStr          string
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

// RegisterFlags holds flags for register command
type RegisterFlags struct {
	Name       string
	Command    string
	WorkDir    string
	LogDir     string
	AutoStart  bool
	APIUrl     string
	APITimeout time.Duration
}

// RegisterFileFlags holds flags for register-file command
type RegisterFileFlags struct {
	FilePath   string
	APIUrl     string
	APITimeout time.Duration
}

// UnregisterFlags holds flags for unregister command
type UnregisterFlags struct {
	Name       string
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
	registerFlags := &RegisterFlags{}
	registerFileFlags := &RegisterFileFlags{}
	unregisterFlags := &UnregisterFlags{}
	groupFlags := &GroupCommandFlags{}
	cronFlags := &CronFlags{}

	provisrCommand := command{mgr: mgr}

	root := createRootCommand(globalFlags)

	// Add subcommands
	root.AddCommand(
		createRegisterCommand(provisrCommand, registerFlags),
		createRegisterFileCommand(provisrCommand, registerFileFlags),
		createUnregisterCommand(provisrCommand, unregisterFlags),
		createStartCommand(provisrCommand, processFlags),
		createStatusCommand(provisrCommand, processFlags),
		createStopCommand(provisrCommand, processFlags),
		createCronCommand(provisrCommand, cronFlags),
		createGroupStartCommand(provisrCommand, groupFlags),
		createGroupStopCommand(provisrCommand, groupFlags),
		createGroupStatusCommand(provisrCommand, groupFlags),
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

// createRegisterCommand creates the register subcommand
func createRegisterCommand(provisrCommand command, registerFlags *RegisterFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new process",
		Long: `Register a new process by creating a program file in the programs directory.
This allows the process to be managed by the provisr daemon.

Examples:
  provisr register --name=web --command="python app.py" --work-dir=/app
  provisr register --name=api --command="./api-server" --log-dir=/var/log/api --auto-start`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Register(RegisterFlags{
				Name:       registerFlags.Name,
				Command:    registerFlags.Command,
				WorkDir:    registerFlags.WorkDir,
				LogDir:     registerFlags.LogDir,
				AutoStart:  registerFlags.AutoStart,
				APIUrl:     registerFlags.APIUrl,
				APITimeout: registerFlags.APITimeout,
			})
		},
	}

	// Add flags specific to register command
	cmd.Flags().StringVar(&registerFlags.Name, "name", "", "process name (required)")
	cmd.Flags().StringVar(&registerFlags.Command, "command", "", "command to run (required)")
	cmd.Flags().StringVar(&registerFlags.WorkDir, "work-dir", "", "working directory")
	cmd.Flags().StringVar(&registerFlags.LogDir, "log-dir", "", "log directory")
	cmd.Flags().BoolVar(&registerFlags.AutoStart, "auto-start", false, "auto-start process when daemon starts")

	// Remote daemon connection
	cmd.Flags().StringVar(&registerFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&registerFlags.APITimeout, "api-timeout", 10*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err)
	}
	if err := cmd.MarkFlagRequired("command"); err != nil {
		panic(err)
	}

	return cmd
}

// createRegisterFileCommand creates the register-file subcommand
func createRegisterFileCommand(provisrCommand command, registerFileFlags *RegisterFileFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-file",
		Short: "Register a process from JSON file",
		Long: `Register a process by copying an existing JSON file to the programs directory.
The JSON file must contain valid process configuration.

Examples:
  provisr register-file --file=./my-process.json
  provisr register-file --file=./web-server.json --api-url=http://remote:8080/api

JSON file format example:
{
  "name": "web-server",
  "command": "python app.py",
  "work_dir": "/app",
  "auto_restart": true,
  "log": {
    "file": {
      "dir": "/var/log"
    }
  }
}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.RegisterFile(RegisterFileFlags{
				FilePath:   registerFileFlags.FilePath,
				APIUrl:     registerFileFlags.APIUrl,
				APITimeout: registerFileFlags.APITimeout,
			})
		},
	}

	// Add flags specific to register-file command
	cmd.Flags().StringVar(&registerFileFlags.FilePath, "file", "", "path to JSON file (required)")

	// Remote daemon connection
	cmd.Flags().StringVar(&registerFileFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&registerFileFlags.APITimeout, "api-timeout", 10*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("file"); err != nil {
		panic(err)
	}

	return cmd
}

// createUnregisterCommand creates the unregister subcommand
func createUnregisterCommand(provisrCommand command, unregisterFlags *UnregisterFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unregister",
		Short: "Unregister a process",
		Long: `Unregister a process by removing its program file from the programs directory.
This prevents the process from being managed by the provisr daemon.
Processes defined in config.toml cannot be unregistered.

Examples:
  provisr unregister --name=web
  provisr unregister --name=api --api-url=http://remote:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Unregister(UnregisterFlags{
				Name:       unregisterFlags.Name,
				APIUrl:     unregisterFlags.APIUrl,
				APITimeout: unregisterFlags.APITimeout,
			})
		},
	}

	// Add flags specific to unregister command
	cmd.Flags().StringVar(&unregisterFlags.Name, "name", "", "process name (required)")

	// Remote daemon connection
	cmd.Flags().StringVar(&unregisterFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&unregisterFlags.APITimeout, "api-timeout", 10*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err)
	}

	return cmd
}

// createStartCommand creates the start subcommand
func createStartCommand(provisrCommand command, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a process",
		Long: `Start a registered process with the specified name.
Processes must be registered first via config file and daemon.

Examples:
  provisr start --name=web
  provisr start --name=api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Start(StartFlags{
				Name:       processFlags.Name,
				APIUrl:     processFlags.APIUrl,
				APITimeout: processFlags.APITimeout,
			})
		},
	}

	// Add flags specific to start command
	cmd.Flags().StringVar(&processFlags.Name, "name", "", "process name (required)")

	// Remote daemon connection
	cmd.Flags().StringVar(&processFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&processFlags.APITimeout, "api-timeout", 10*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err) // This should never happen during setup
	}

	return cmd
}

// createStatusCommand creates the status subcommand
func createStatusCommand(provisrCommand command, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show process status",
		Long: `Show the status of processes managed by provisr.

Examples:
  provisr status                    # Show all processes
  provisr status --name=web         # Show specific process
  provisr status --api-url=http://remote:8080/api  # Remote status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Status(StatusFlags{
				Name:       processFlags.Name,
				APIUrl:     processFlags.APIUrl,
				APITimeout: processFlags.APITimeout,
				Detailed:   cmd.Flag("detailed").Changed,
			})
		},
	}
	cmd.Flags().StringVar(&processFlags.Name, "name", "", "process name (optional)")
	cmd.Flags().StringVar(&processFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&processFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")
	cmd.Flags().Bool("detailed", false, "show detailed info")
	return cmd
}

// createStopCommand creates the stop subcommand
func createStopCommand(provisrCommand command, processFlags *ProcessFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a process",
		Long: `Stop processes managed by provisr.

Examples:
  provisr stop --name=web           # Stop specific process
  provisr stop --name=web --wait=5s # Stop with custom wait time
  provisr stop --api-url=http://remote:8080/api  # Remote stop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var waitDuration time.Duration
			if cmd.Flag("wait").Changed {
				waitDuration, _ = cmd.Flags().GetDuration("wait")
			} else {
				waitDuration = 3 * time.Second
			}
			return provisrCommand.Stop(StopFlags{
				Name:       processFlags.Name,
				APIUrl:     processFlags.APIUrl,
				APITimeout: processFlags.APITimeout,
				Wait:       waitDuration,
			})
		},
	}
	cmd.Flags().StringVar(&processFlags.Name, "name", "", "process name (required)")
	cmd.Flags().Duration("wait", 3*time.Second, "time to wait for graceful shutdown")
	cmd.Flags().StringVar(&processFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&processFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err) // This should never happen during setup
	}
	return cmd
}

// createCronCommand creates the cron subcommand
func createCronCommand(provisrCommand command, cronFlags *CronFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Control scheduled jobs via daemon (REST)",
		Long: `Cron jobs are executed by the provisr daemon started with 'serve'.
This command communicates with the running daemon via REST to verify readiness.

Examples:
  provisr cron                 # Verify daemon is running and has loaded cron jobs
  provisr cron --api-url=http://remote:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.Cron(CronFlags{
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
func createGroupStartCommand(provisrCommand command, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-start",
		Short: "Start a process group",
		Long: `Start all processes in a named group.

Example:
  provisr group-start --group=webstack
  provisr group-start --group=webstack --api-url=http://127.0.0.1:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStart(GroupFlags{
				GroupName:  groupFlags.GroupName,
				APIUrl:     groupFlags.APIUrl,
				APITimeout: groupFlags.APITimeout,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name (required)")
	cmd.Flags().StringVar(&groupFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&groupFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("group"); err != nil {
		panic(err) // This should never happen during setup
	}
	return cmd
}

// createGroupStopCommand creates the group-stop subcommand
func createGroupStopCommand(provisrCommand command, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-stop",
		Short: "Stop a process group",
		Long: `Stop all processes in a named group.

Example:
  provisr group-stop --group=webstack
  provisr group-stop --group=webstack --wait=5s
  provisr group-stop --group=webstack --api-url=http://127.0.0.1:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var waitDuration time.Duration
			if cmd.Flag("wait").Changed {
				waitDuration, _ = cmd.Flags().GetDuration("wait")
			} else {
				waitDuration = 3 * time.Second
			}
			return provisrCommand.GroupStop(GroupFlags{
				GroupName:  groupFlags.GroupName,
				APIUrl:     groupFlags.APIUrl,
				APITimeout: groupFlags.APITimeout,
				Wait:       waitDuration,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name (required)")
	cmd.Flags().Duration("wait", 3*time.Second, "time to wait for graceful shutdown")
	cmd.Flags().StringVar(&groupFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&groupFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("group"); err != nil {
		panic(err) // This should never happen during setup
	}
	return cmd
}

// createGroupStatusCommand creates the group-status subcommand
func createGroupStatusCommand(provisrCommand command, groupFlags *GroupCommandFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-status",
		Short: "Show group status",
		Long: `Show status of all processes in a named group.

Example:
  provisr group-status --group=webstack
  provisr group-status --group=webstack --api-url=http://127.0.0.1:8080/api`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return provisrCommand.GroupStatus(GroupFlags{
				GroupName:  groupFlags.GroupName,
				APIUrl:     groupFlags.APIUrl,
				APITimeout: groupFlags.APITimeout,
			})
		},
	}
	cmd.Flags().StringVar(&groupFlags.GroupName, "group", "", "group name (required)")
	cmd.Flags().StringVar(&groupFlags.APIUrl, "api-url", "", "remote daemon URL (e.g. http://host:8080/api)")
	cmd.Flags().DurationVar(&groupFlags.APITimeout, "api-timeout", 30*time.Second, "request timeout")

	// Mark required flags
	if err := cmd.MarkFlagRequired("group"); err != nil {
		panic(err) // This should never happen during setup
	}
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
  provisr serve --daemonize         # Run as daemon in background (daemon pidfile configured via [server].pidfile)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleServeCommand(serveFlags, args)
		},
	}

	// Add daemonize flags
	cmd.Flags().BoolVar(&serveFlags.Daemonize, "daemonize", false, "run as daemon in background")
	cmd.Flags().StringVar(&serveFlags.LogFile, "logfile", "", "redirect daemon logs to file")

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

	// Load unified config once
	cfg, err := provisr.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Enforce that pid_dir is configured and usable for PID file creation
	if cfg.PIDDir == "" {
		return fmt.Errorf("pid_dir must be set in the config to determine where to write process PID files")
	}
	pidDir := cfg.PIDDir
	if !filepath.IsAbs(pidDir) {
		pidDir = filepath.Join(filepath.Dir(configPath), pidDir)
	}
	if err := os.MkdirAll(pidDir, 0o750); err != nil {
		return fmt.Errorf("failed to create pid_dir %s: %w", pidDir, err)
	}

	// If daemonize is requested, now that we have cfg.Server, use its pidfile/logfile
	if flags.Daemonize {
		pidfile := ""
		logfile := flags.LogFile
		if cfg.Server != nil {
			pidfile = cfg.Server.PidFile
			if logfile == "" {
				logfile = cfg.Server.LogFile
			}
		}
		return daemonize(pidfile, logfile)
	}

	// Create manager (PID-only management; persistent store removed)
	mgr := provisr.New()

	// Apply global environment
	// Set global environment - 직접 필드 접근
	mgr.SetGlobalEnv(cfg.GlobalEnv)

	// Convert and set group definitions
	managerGroups := make([]provisr.ManagerGroupSpec, len(cfg.GroupSpecs))
	for i, group := range cfg.GroupSpecs {
		managerGroups[i] = provisr.ManagerGroupSpec{
			Name:    group.Name,
			Members: group.Members,
		}
	}
	mgr.SetGroups(managerGroups)

	// Setup metrics from config
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		// Configure process metrics if enabled
		if cfg.Metrics.ProcessMetrics != nil && cfg.Metrics.ProcessMetrics.Enabled {
			processMetricsConfig := provisr.ProcessMetricsConfig{
				Enabled:     cfg.Metrics.ProcessMetrics.Enabled,
				Interval:    cfg.Metrics.ProcessMetrics.Interval,
				MaxHistory:  cfg.Metrics.ProcessMetrics.MaxHistory,
				HistorySize: cfg.Metrics.ProcessMetrics.HistorySize,
			}

			// Register metrics with process metrics support
			if err := provisr.RegisterMetricsWithProcessMetricsDefault(processMetricsConfig); err != nil {
				fmt.Printf("Warning: failed to register process metrics: %v\n", err)
			}

			// Create and configure process metrics collector
			collector := provisr.NewProcessMetricsCollector(processMetricsConfig)
			if err := mgr.SetProcessMetricsCollector(collector); err != nil {
				fmt.Printf("Warning: failed to setup process metrics collector: %v\n", err)
			} else {
				fmt.Printf("Started process metrics collection (interval: %v, history: %d)\n",
					processMetricsConfig.Interval,
					processMetricsConfig.MaxHistory)
			}
		} else {
			// Register standard metrics only
			if err := provisr.RegisterMetricsDefault(); err != nil {
				fmt.Printf("Warning: failed to register metrics: %v\n", err)
			}
		}

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

	// Apply config: recover from PID files, start missing, and cleanup removed processes
	if err := mgr.ApplyConfig(cfg.Specs); err != nil {
		fmt.Printf("Warning: failed to apply config: %v\n", err)
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

	// Create and start HTTP/HTTPS server
	protocol := "HTTP"
	var server *http.Server

	if cfg.Server.TLS != nil && cfg.Server.TLS.Enabled {
		protocol = "HTTPS"
		server, err = provisr.NewTLSServer(*cfg.Server, mgr)
		if err != nil {
			return fmt.Errorf("failed to create HTTPS server: %w", err)
		}
	} else {
		server, err = provisr.NewHTTPServer(cfg.Server.Listen, cfg.Server.BasePath, mgr)
		if err != nil {
			return fmt.Errorf("failed to create HTTP server: %w", err)
		}
	}

	fmt.Printf("Starting provisr %s server on %s%s\n", protocol, cfg.Server.Listen, cfg.Server.BasePath)

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
