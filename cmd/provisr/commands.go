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

// stopViaAPI stops processes using the daemon API
func (c *command) stopViaAPI(f StopFlags, apiClient *APIClient) error {
	if f.ConfigPath != "" {
		// For config-based stops, we need to load config and stop each spec
		config, err := provisr.LoadConfig(f.ConfigPath)
		if err != nil {
			return err
		}

		for _, spec := range config.Specs {
			if err := apiClient.StopProcess(spec.Name, f.Wait); err != nil {
				return fmt.Errorf("failed to stop %s: %w", spec.Name, err)
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

	// Single process stop
	if err := apiClient.StopProcess(f.Name, f.Wait); err != nil {
		return err
	}

	// Get status and print
	result, err := apiClient.GetStatus(f.Name)
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

// Start Method-style handlers bound to a command with an embedded manager
func (c *command) Start(f StartFlags) error {
	// Always use API - default to local daemon if not specified
	apiUrl := f.APIUrl
	if apiUrl == "" {
		apiUrl = "http://127.0.0.1:8080/api" // Default local daemon
	}

	apiClient := NewAPIClient(apiUrl, f.APITimeout)
	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable at %s - please start daemon first with 'provisr serve'", apiUrl)
	}

	return c.startViaAPI(f, apiClient)
}

// Status prints status information, optionally loading specs from config for base queries
func (c *command) Status(f StatusFlags) error {
	// Always use API - default to local daemon if not specified
	apiUrl := f.APIUrl
	if apiUrl == "" {
		apiUrl = "http://127.0.0.1:8080/api" // Default local daemon
	}

	apiClient := NewAPIClient(apiUrl, f.APITimeout)
	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable at %s - please start daemon first with 'provisr serve'", apiUrl)
	}

	return c.statusViaAPI(f, apiClient)
}

// Stop stops processes by name/base from flags or config
func (c *command) Stop(f StopFlags) error {
	// Always use API - default to local daemon if not specified
	apiUrl := f.APIUrl
	if apiUrl == "" {
		apiUrl = "http://127.0.0.1:8080/api" // Default local daemon
	}

	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
	}

	apiClient := NewAPIClient(apiUrl, f.APITimeout)
	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable at %s - please start daemon first with 'provisr serve'", apiUrl)
	}

	return c.stopViaAPI(f, apiClient)
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
