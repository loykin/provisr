package main

import (
	"fmt"
	"time"

	"github.com/loykin/provisr"
)

type command struct {
	mgr *provisr.Manager
}

// startViaAPI starts processes using the daemon API
func (c *command) startViaAPI(f StartFlags, apiClient *APIClient) error {
	if f.ConfigPath != "" {
		// For config-based starts, we need to load config and start each spec
		config, err := provisr.LoadConfig(f.ConfigPath)
		if err != nil {
			return err
		}

		for _, spec := range config.Specs {
			if err := apiClient.StartProcess(spec); err != nil {
				return fmt.Errorf("failed to start %s: %w", spec.Name, err)
			}
		}

		// Get status and print
		result, err := apiClient.GetStatus("")
		if err != nil {
			return err
		}
		printJSON(result)
		return nil
	}

	// Single process start
	spec := provisr.Spec{
		Name:            f.Name,
		Command:         f.Cmd,
		PIDFile:         f.PIDFile,
		RetryCount:      f.Retries,
		RetryInterval:   f.RetryInterval,
		StartDuration:   f.StartDuration,
		AutoRestart:     f.AutoRestart,
		RestartInterval: f.RestartInterval,
		Instances:       f.Instances,
	}

	return apiClient.StartProcess(spec)
}

// statusViaAPI gets status using the daemon API
func (c *command) statusViaAPI(f StatusFlags, apiClient *APIClient) error {
	result, err := apiClient.GetStatus(f.Name)
	if err != nil {
		return err
	}

	if f.Detailed {
		// For detailed status, we might need to format differently
		// For now, just print the JSON
		printJSON(result)
	} else {
		printJSON(result)
	}

	return nil
}

// Start Method-style handlers bound to a command with an embedded manager
func (c *command) Start(f StartFlags) error {
	// If API URL is specified, try daemon API
	if f.APIUrl != "" {
		apiClient := NewAPIClient(f.APIUrl, f.APITimeout)
		if apiClient.IsReachable() {
			return c.startViaAPI(f, apiClient)
		}
		return fmt.Errorf("daemon not reachable at %s", f.APIUrl)
	}

	// No API URL specified - use direct manager
	return c.startViaManager(f)
}

// startViaManager uses direct manager (fallback mode)
func (c *command) startViaManager(f StartFlags) error {
	mgr := c.mgr
	if f.ConfigPath != "" {
		config, err := provisr.LoadConfig(f.ConfigPath)
		if err != nil {
			return err
		}

		if len(config.GlobalEnv) > 0 {
			mgr.SetGlobalEnv(config.GlobalEnv)
		}

		if err := startFromSpecs(mgr, config.Specs); err != nil {
			return err
		}
		printJSON(statusesByBase(mgr, config.Specs))
		return nil
	}
	// Apply global env from flags when not using config
	applyGlobalEnvFromFlags(mgr, f.UseOSEnv, f.EnvKVs)
	sp := provisr.Spec{
		Name:            f.Name,
		Command:         f.Cmd,
		PIDFile:         f.PIDFile,
		RetryCount:      f.Retries,
		RetryInterval:   f.RetryInterval,
		StartDuration:   f.StartDuration,
		AutoRestart:     f.AutoRestart,
		RestartInterval: f.RestartInterval,
		Instances:       f.Instances,
	}
	if f.Instances > 1 {
		return mgr.StartN(sp)
	}
	return mgr.Start(sp)
}

// Status prints status information, optionally loading specs from config for base queries
func (c *command) Status(f StatusFlags) error {
	// If API URL is specified, try daemon API
	if f.APIUrl != "" {
		apiClient := NewAPIClient(f.APIUrl, f.APITimeout)
		if apiClient.IsReachable() {
			return c.statusViaAPI(f, apiClient)
		}
		return fmt.Errorf("daemon not reachable at %s", f.APIUrl)
	}

	// No API URL specified - use direct manager
	return c.statusViaManager(f)
}

// statusViaManager uses direct manager (fallback mode)
func (c *command) statusViaManager(f StatusFlags) error {
	mgr := c.mgr
	if f.ConfigPath != "" {
		config, err := provisr.LoadConfig(f.ConfigPath)
		if err != nil {
			return err
		}
		if f.Detailed {
			printDetailedStatusByBase(mgr, config.Specs)
		} else {
			printJSON(statusesByBase(mgr, config.Specs))
		}
		return nil
	}

	sts, _ := mgr.StatusAll(f.Name)
	if f.Detailed {
		printDetailedStatus(sts)
	} else {
		printJSON(sts)
	}
	return nil
}

// Stop stops processes by name/base from flags or config
func (c *command) Stop(f StopFlags) error {
	mgr := c.mgr
	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
	}
	if f.ConfigPath != "" {
		config, err := provisr.LoadConfig(f.ConfigPath)
		if err != nil {
			return err
		}
		for _, sp := range config.Specs {
			_ = mgr.StopAll(sp.Name, f.Wait)
		}
		printJSON(statusesByBase(mgr, config.Specs))
		return nil
	}
	_ = mgr.StopAll(f.Name, f.Wait)
	sts, _ := mgr.StatusAll(f.Name)
	printJSON(sts)
	return nil
}

// Cron runs cron scheduler based on config
func (c *command) Cron(f CronFlags) error {
	mgr := c.mgr
	if f.ConfigPath == "" {
		return fmt.Errorf("cron requires --config file with processes having schedule")
	}

	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	if len(config.GlobalEnv) > 0 {
		mgr.SetGlobalEnv(config.GlobalEnv)
	}

	sch := provisr.NewCronScheduler(mgr)
	for _, j := range config.CronJobs {
		jb := provisr.CronJob{Name: j.Name, Spec: j.Spec, Schedule: j.Schedule, Singleton: j.Singleton}
		if err := sch.Add(&jb); err != nil {
			return err
		}
	}
	if err := sch.Start(); err != nil {
		return err
	}
	if f.NonBlocking {
		// For tests: stop immediately
		sch.Stop()
		return nil
	}
	// Block forever in production
	select {}
}

// GroupStart starts a group from config
func (c *command) GroupStart(f GroupFlags) error {
	mgr := c.mgr
	if f.ConfigPath == "" {
		return fmt.Errorf("group-start requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-start requires --group name")
	}

	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	if len(config.GlobalEnv) > 0 {
		mgr.SetGlobalEnv(config.GlobalEnv)
	}

	gs := findGroupByName(config.GroupSpecs, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}
	g := provisr.NewGroup(mgr)
	return g.Start(*gs)
}

// GroupStop stops a group from config
func (c *command) GroupStop(f GroupFlags) error {
	mgr := c.mgr
	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
	}
	if f.ConfigPath == "" {
		return fmt.Errorf("group-stop requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-stop requires --group name")
	}
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}
	gs := findGroupByName(config.GroupSpecs, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}
	g := provisr.NewGroup(mgr)
	return g.Stop(*gs, f.Wait)
}

// GroupStatus prints status for a group from config
func (c *command) GroupStatus(f GroupFlags) error {
	mgr := c.mgr
	if f.ConfigPath == "" {
		return fmt.Errorf("group-status requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-status requires --group name")
	}
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}
	gs := findGroupByName(config.GroupSpecs, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}
	g := provisr.NewGroup(mgr)
	stmap, err := g.Status(*gs)
	if err != nil {
		return err
	}
	printJSON(stmap)
	return nil
}

func (c *command) runGroupStatus(f GroupFlags) error {
	mgr := c.mgr
	return (&command{mgr: mgr}).GroupStatus(f)
}
