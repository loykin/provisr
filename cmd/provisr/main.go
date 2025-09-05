package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/loykin/provisr"
	"github.com/spf13/cobra"
)

func applyGlobalEnvFromFlags(mgr *provisr.Manager, useOSEnv bool, envFiles []string, envKVs []string) {
	if useOSEnv {
		mgr.SetGlobalEnv(os.Environ())
	}
	if len(envFiles) > 0 {
		for _, f := range envFiles {
			if pairs, err := provisr.LoadEnv(f); err == nil && len(pairs) > 0 {
				mgr.SetGlobalEnv(pairs)
			}
		}
	}
	if len(envKVs) > 0 {
		mgr.SetGlobalEnv(envKVs)
	}
}

func startFromSpecs(mgr *provisr.Manager, specs []provisr.Spec) error {
	for _, sp := range specs {
		if sp.Instances > 1 {
			if err := mgr.StartN(sp); err != nil {
				return err
			}
		} else {
			if err := mgr.Start(sp); err != nil {
				return err
			}
		}
	}
	return nil
}

func statusesByBase(mgr *provisr.Manager, specs []provisr.Spec) map[string][]provisr.Status {
	all := make(map[string][]provisr.Status)
	for _, sp := range specs {
		sts, _ := mgr.StatusAll(sp.Name)
		all[sp.Name] = sts
	}
	return all
}

func findGroupByName(groups []provisr.GroupSpec, name string) *provisr.GroupSpec {
	for i := range groups {
		if groups[i].Name == name {
			return &groups[i]
		}
	}
	return nil
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

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
			if configPath != "" {
				if genv, err := provisr.LoadGlobalEnv(configPath); err == nil && len(genv) > 0 {
					mgr.SetGlobalEnv(genv)
				}
				specs, err := provisr.LoadSpecs(configPath)
				if err != nil {
					return err
				}
				if err := startFromSpecs(mgr, specs); err != nil {
					return err
				}
				printJSON(statusesByBase(mgr, specs))
				return nil
			}
			// Apply global env from flags when not using config
			applyGlobalEnvFromFlags(mgr, useOSEnv, envFiles, envKVs)
			sp := provisr.Spec{
				Name:            name,
				Command:         cmdStr,
				PIDFile:         pidfile,
				RetryCount:      retries,
				RetryInterval:   retryInterval,
				StartDuration:   startDuration,
				AutoRestart:     autoRestart,
				RestartInterval: restartInterval,
				Instances:       instances,
			}
			if instances > 1 {
				return mgr.StartN(sp)
			} else {
				return mgr.Start(sp)
			}
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
			if configPath != "" {
				specs, err := provisr.LoadSpecs(configPath)
				if err != nil {
					return err
				}
				printJSON(statusesByBase(mgr, specs))
				return nil
			}
			sts, _ := mgr.StatusAll(name)
			printJSON(sts)
			return nil
		},
	}
	cmdStatus.Flags().StringVar(&name, "name", "demo", "process name")

	cmdStop := &cobra.Command{
		Use:   "stop",
		Short: "Stop process(es)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath != "" {
				specs, err := provisr.LoadSpecs(configPath)
				if err != nil {
					return err
				}
				for _, sp := range specs {
					_ = mgr.StopAll(sp.Name, 3*time.Second)
				}
				printJSON(statusesByBase(mgr, specs))
				return nil
			}
			_ = mgr.StopAll(name, 3*time.Second)
			sts, _ := mgr.StatusAll(name)
			printJSON(sts)
			return nil
		},
	}
	cmdStop.Flags().StringVar(&name, "name", "demo", "process name")

	// Cron subcommand: runs scheduled jobs defined in config
	cmdCron := &cobra.Command{
		Use:   "cron",
		Short: "Run cron jobs from config (requires --config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				return fmt.Errorf("cron requires --config file with processes having schedule")
			}
			if genv, err := provisr.LoadGlobalEnv(configPath); err == nil && len(genv) > 0 {
				mgr.SetGlobalEnv(genv)
			}
			jobs, err := provisr.LoadCronJobs(configPath)
			if err != nil {
				return err
			}
			sch := provisr.NewCronScheduler(mgr)
			for _, j := range jobs {
				jb := provisr.CronJob{Name: j.Name, Spec: j.Spec, Schedule: j.Schedule, Singleton: j.Singleton}
				if err := sch.Add(jb); err != nil {
					return err
				}
			}
			if err := sch.Start(); err != nil {
				return err
			}
			// Block until interrupted
			select {}
		},
	}

	// Group subcommands
	var groupName string
	cmdGroupStart := &cobra.Command{Use: "group-start", Short: "Start a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		if configPath == "" {
			return fmt.Errorf("group-start requires --config")
		}
		if groupName == "" {
			return fmt.Errorf("group-start requires --group name")
		}
		if genv, err := provisr.LoadGlobalEnv(configPath); err == nil && len(genv) > 0 {
			mgr.SetGlobalEnv(genv)
		}
		groups, err := provisr.LoadGroups(configPath)
		if err != nil {
			return err
		}
		gs := findGroupByName(groups, groupName)
		if gs == nil {
			return fmt.Errorf("group %s not found in config", groupName)
		}
		g := provisr.NewGroup(mgr)
		return g.Start(*gs)
	}}
	cmdGroupStop := &cobra.Command{Use: "group-stop", Short: "Stop a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		if configPath == "" {
			return fmt.Errorf("group-stop requires --config")
		}
		if groupName == "" {
			return fmt.Errorf("group-stop requires --group name")
		}
		groups, err := provisr.LoadGroups(configPath)
		if err != nil {
			return err
		}
		gs := findGroupByName(groups, groupName)
		if gs == nil {
			return fmt.Errorf("group %s not found in config", groupName)
		}
		g := provisr.NewGroup(mgr)
		return g.Stop(*gs, 3*time.Second)
	}}
	cmdGroupStatus := &cobra.Command{Use: "group-status", Short: "Show status for a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		if configPath == "" {
			return fmt.Errorf("group-status requires --config")
		}
		if groupName == "" {
			return fmt.Errorf("group-status requires --group name")
		}
		groups, err := provisr.LoadGroups(configPath)
		if err != nil {
			return err
		}
		gs := findGroupByName(groups, groupName)
		if gs == nil {
			return fmt.Errorf("group %s not found in config", groupName)
		}
		g := provisr.NewGroup(mgr)
		stmap, err := g.Status(*gs)
		if err != nil {
			return err
		}
		printJSON(stmap)
		return nil
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
