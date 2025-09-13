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

// helpers extracted to reduce main() cyclomatic complexity
// buildRoot creates the root command, wires flags and subcommands, and returns a binder to attach the metrics hook.
func buildRoot(mgr *provisr.Manager) (*cobra.Command, func()) {
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
		useOSEnv        bool
		envKVs          []string
		envFiles        []string
		metricsListen   string
		groupName       string
		apiListen       string
		apiBase         string
		nonBlocking     bool
		storeDSN        string
		noStore         bool
		// history flags
		histDisableStore bool
		histOSURL        string
		histOSIndex      string
		histCHURL        string
		histCHTable      string
	)

	root := &cobra.Command{Use: "provisr"}
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to TOML config file")
	root.PersistentFlags().BoolVar(&useOSEnv, "use-os-env", false, "inject current OS environment into global env")
	root.PersistentFlags().StringSliceVar(&envKVs, "env", nil, "additional KEY=VALUE to inject (repeatable)")
	root.PersistentFlags().StringSliceVar(&envFiles, "env-file", nil, "path to .env file(s) with KEY=VALUE lines (repeatable)")
	root.PersistentFlags().StringVar(&metricsListen, "metrics-listen", "", "address to serve Prometheus /metrics (e.g., :9090)")
	root.PersistentFlags().StringVar(&storeDSN, "store-dsn", "", "enable persistent store with DSN (e.g., sqlite:///path.db or postgres://...")
	root.PersistentFlags().BoolVar(&noStore, "no-store", false, "disable persistent store even if configured")
	// history-related flags
	root.PersistentFlags().BoolVar(&histDisableStore, "history-disable-store", false, "do not record history rows in the persistent store")
	root.PersistentFlags().StringVar(&histOSURL, "history-opensearch-url", "", "OpenSearch base URL (e.g., http://localhost:9200)")
	root.PersistentFlags().StringVar(&histOSIndex, "history-opensearch-index", "", "OpenSearch index name for history (e.g., provisr-history)")
	root.PersistentFlags().StringVar(&histCHURL, "history-clickhouse-url", "", "ClickHouse HTTP endpoint (e.g., http://localhost:8123)")
	root.PersistentFlags().StringVar(&histCHTable, "history-clickhouse-table", "", "ClickHouse table for history (e.g., default.provisr_history)")

	// start
	startCmd := &cobra.Command{Use: "start", Short: "Start process(es)", RunE: func(cmd *cobra.Command, args []string) error {
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
	}}
	startCmd.Flags().StringVar(&name, "name", "demo", "process name")
	startCmd.Flags().StringVar(&cmdStr, "cmd", "sleep 60", "command to run")
	startCmd.Flags().StringVar(&pidfile, "pidfile", "", "optional pidfile path")
	startCmd.Flags().IntVar(&retries, "retries", 0, "retry count on start failure")
	startCmd.Flags().DurationVar(&retryInterval, "retry-interval", 500*time.Millisecond, "retry interval on start failure")
	startCmd.Flags().BoolVar(&autoRestart, "autorestart", false, "restart automatically if the process dies")
	startCmd.Flags().DurationVar(&restartInterval, "restart-interval", time.Second, "interval before auto-restart")
	startCmd.Flags().DurationVar(&startDuration, "startsecs", 0, "time the process must stay up to be considered started")
	startCmd.Flags().IntVar(&instances, "instances", 1, "number of instances to start")

	// status
	statusCmd := &cobra.Command{Use: "status", Short: "Show status", RunE: func(cmd *cobra.Command, args []string) error {
		return cmdStatus(mgr, StatusFlags{ConfigPath: configPath, Name: name})
	}}
	statusCmd.Flags().StringVar(&name, "name", "demo", "process name")

	// stop
	stopCmd := &cobra.Command{Use: "stop", Short: "Stop process(es)", RunE: func(cmd *cobra.Command, args []string) error {
		return cmdStop(mgr, StopFlags{ConfigPath: configPath, Name: name, Wait: 3 * time.Second})
	}}
	stopCmd.Flags().StringVar(&name, "name", "demo", "process name")

	// cron
	cronCmd := &cobra.Command{Use: "cron", Short: "Run cron jobs from config (requires --config)", RunE: func(cmd *cobra.Command, args []string) error {
		return cmdCron(mgr, CronFlags{ConfigPath: configPath})
	}}

	// groups
	gStart := &cobra.Command{Use: "group-start", Short: "Start a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		return runGroupStart(mgr, GroupFlags{ConfigPath: configPath, GroupName: groupName})
	}}
	gStop := &cobra.Command{Use: "group-stop", Short: "Stop a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		return runGroupStop(mgr, GroupFlags{ConfigPath: configPath, GroupName: groupName, Wait: 3 * time.Second})
	}}
	gStatus := &cobra.Command{Use: "group-status", Short: "Show status for a group from config (requires --config and --group)", RunE: func(cmd *cobra.Command, args []string) error {
		return runGroupStatus(mgr, GroupFlags{ConfigPath: configPath, GroupName: groupName})
	}}
	gStart.Flags().StringVar(&groupName, "group", "", "group name from config")
	gStop.Flags().StringVar(&groupName, "group", "", "group name from config")
	gStatus.Flags().StringVar(&groupName, "group", "", "group name from config")

	// serve HTTP API
	serveCmd := &cobra.Command{Use: "serve", Short: "Start HTTP API server (reads http_api from --config)", RunE: func(cmd *cobra.Command, args []string) error {
		listen := apiListen
		base := apiBase
		if configPath != "" {
			if httpCfg, err := provisr.LoadHTTPAPI(configPath); err == nil && httpCfg != nil {
				if listen == "" {
					listen = httpCfg.Listen
				}
				if base == "" {
					base = httpCfg.BasePath
				}
				// If config explicitly disables, require explicit flag to override
				if !httpCfg.Enabled && apiListen == "" {
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
		if nonBlocking {
			return nil
		}
		select {}
	}}
	serveCmd.Flags().StringVar(&apiListen, "api-listen", "", "address to listen for HTTP API (e.g., :8080)")
	serveCmd.Flags().StringVar(&apiBase, "api-base", "", "base path for API endpoints (default from config or /api)")
	serveCmd.Flags().BoolVar(&nonBlocking, "non-blocking", false, "do not block; return immediately (useful for tests)")

	root.AddCommand(startCmd, statusCmd, stopCmd, cronCmd, gStart, gStop, gStatus, serveCmd)

	binder := func() {
		root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			// Configure persistent store based on flags/config
			currentDSN := ""
			if noStore {
				mgr.DisableStore()
			} else {
				dsn := storeDSN
				if dsn == "" && configPath != "" {
					if sc, err := provisr.LoadStore(configPath); err == nil && sc != nil && sc.Enabled && sc.DSN != "" {
						dsn = sc.DSN
					}
				}
				if dsn != "" {
					_ = mgr.SetStoreFromDSN(dsn)
					currentDSN = dsn
				}
			}
			// History: configure sinks (store-backed via DSN and/or external ones)
			var cfgSinks []provisr.HistorySink
			var inStoreEnabled *bool
			if configPath != "" {
				if hc, err := provisr.LoadHistory(configPath); err == nil && hc != nil {
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
			// Flags add external sinks; histDisableStore prevents adding store SQL sink
			var flagSinks []provisr.HistorySink
			if histOSURL != "" && histOSIndex != "" {
				flagSinks = append(flagSinks, provisr.NewOpenSearchHistorySink(histOSURL, histOSIndex))
			}
			if histCHURL != "" && histCHTable != "" {
				flagSinks = append(flagSinks, provisr.NewClickHouseHistorySink(histCHURL, histCHTable))
			}
			// Add store-backed SQL sink if we have a DSN and not explicitly disabled
			if !histDisableStore {
				// default is enabled when InStore is nil; enable if explicitly true too
				enableStoreSink := inStoreEnabled == nil || (inStoreEnabled != nil && *inStoreEnabled)
				if enableStoreSink && currentDSN != "" {
					if ss := provisr.NewSQLHistorySinkFromDSN(currentDSN); ss != nil {
						cfgSinks = append(cfgSinks, ss)
					}
				}
			}
			if len(flagSinks) > 0 {
				mgr.SetHistorySinks(flagSinks...)
			} else if len(cfgSinks) > 0 {
				mgr.SetHistorySinks(cfgSinks...)
			}
			// Start background reconciler (idempotent)
			mgr.StartReconciler(2 * time.Second)
			// Metrics
			if metricsListen != "" {
				go func() {
					_ = provisr.RegisterMetricsDefault()
					_ = provisr.ServeMetrics(metricsListen)
				}()
			}
		}
	}
	return root, binder
}
