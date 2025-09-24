package main

import (
	"fmt"
	"strings"
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

		// Only get status if we have specs
		if len(config.Specs) > 0 {
			// Get status and print
			result, err := apiClient.GetStatus("")
			if err != nil {
				return err
			}
			printJSON(result)
		}
		return nil
	}

	// Single process start
	spec := provisr.Spec{
		Name:            f.Name,
		Command:         f.Cmd,
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

		var firstUnexpectedErr error
		for _, spec := range config.Specs {
			if err := apiClient.StopProcess(spec.Name, f.Wait); err != nil {
				if !isExpectedShutdownError(err) && firstUnexpectedErr == nil {
					firstUnexpectedErr = fmt.Errorf("failed to stop %s: %w", spec.Name, err)
				}
			}
		}

		// Get status and print
		result, err := apiClient.GetStatus("")
		if err != nil {
			return err
		}
		printJSON(result)
		return firstUnexpectedErr
	}

	// Single process stop
	if err := apiClient.StopProcess(f.Name, f.Wait); err != nil {
		if !isExpectedShutdownError(err) {
			return err
		}
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

// Cron verifies cron scheduler via daemon (REST). The actual scheduler runs inside the daemon started by 'serve'.
func (c *command) Cron(f CronFlags) error {
	if f.ConfigPath == "" {
		return fmt.Errorf("cron requires --config (daemon reads cron jobs from this file)")
	}
	// Always use API - default to local daemon if not specified
	apiUrl := f.APIUrl
	if apiUrl == "" {
		apiUrl = "http://127.0.0.1:8080/api" // Default local daemon
	}
	apiClient := NewAPIClient(apiUrl, f.APITimeout)
	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable at %s - please start daemon first with 'provisr serve'", apiUrl)
	}
	// Optionally check that daemon is healthy and responding with a status list
	if _, err := apiClient.GetStatus(""); err != nil {
		return err
	}
	// Success: daemon manages cron; CLI does not run scheduler locally
	fmt.Println("Cron scheduler is managed by the daemon. Jobs defined in the config are executed by 'provisr serve'.")
	return nil
}

// GroupStart starts a group from config
func (c *command) GroupStart(f GroupFlags) error {
	if f.ConfigPath == "" {
		return fmt.Errorf("group-start requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-start requires --group name")
	}

	// Always use API - default to local daemon if not specified
	apiUrl := f.APIUrl
	if apiUrl == "" {
		apiUrl = "http://127.0.0.1:8080/api" // Default local daemon
	}

	apiClient := NewAPIClient(apiUrl, f.APITimeout)
	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable at %s - please start daemon first with 'provisr serve'", apiUrl)
	}

	return c.groupStartViaAPI(f, apiClient)
}

// groupStartViaAPI starts a group using the daemon API
func (c *command) groupStartViaAPI(f GroupFlags, apiClient *APIClient) error {
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	gs := findGroupByName(config.GroupSpecs, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}

	// Start each member of the group
	for _, member := range gs.Members {
		if err := apiClient.StartProcess(member); err != nil {
			return fmt.Errorf("failed to start %s: %w", member.Name, err)
		}
	}

	return nil
}

// GroupStop stops a group from config
func (c *command) GroupStop(f GroupFlags) error {
	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
	}
	if f.ConfigPath == "" {
		return fmt.Errorf("group-stop requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-stop requires --group name")
	}

	// Always use API - default to local daemon if not specified
	apiUrl := f.APIUrl
	if apiUrl == "" {
		apiUrl = "http://127.0.0.1:8080/api" // Default local daemon
	}

	apiClient := NewAPIClient(apiUrl, f.APITimeout)
	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable at %s - please start daemon first with 'provisr serve'", apiUrl)
	}

	return c.groupStopViaAPI(f, apiClient)
}

// isExpectedShutdownError checks if the error is expected during shutdown
func isExpectedShutdownError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for common shutdown signals and patterns
	return errStr == "signal: terminated" ||
		errStr == "signal: killed" ||
		errStr == "signal: interrupt" ||
		errStr == "exit status 1" || // Common exit code
		errStr == "exit status 130" || // Ctrl+C
		errStr == "exit status 143" || // SIGTERM
		// Also handle wrapped errors from stop process
		errStr == "failed to stop process: signal: terminated" ||
		errStr == "failed to stop process: signal: killed" ||
		errStr == "failed to stop process: signal: interrupt" ||
		// Handle API error responses that contain shutdown signals
		errStr == "API error: signal: terminated" ||
		errStr == "API error: signal: killed" ||
		errStr == "API error: signal: interrupt" ||
		// Handle nested API error responses
		errStr == "API error: failed to stop process: signal: terminated" ||
		errStr == "API error: failed to stop process: signal: killed" ||
		errStr == "API error: failed to stop process: signal: interrupt" ||
		// Check if error string contains shutdown signals (more flexible)
		strings.Contains(errStr, "signal: terminated") ||
		strings.Contains(errStr, "signal: killed") ||
		strings.Contains(errStr, "signal: interrupt")
}

// groupStopViaAPI stops a group using the daemon API
func (c *command) groupStopViaAPI(f GroupFlags, apiClient *APIClient) error {
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	gs := findGroupByName(config.GroupSpecs, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}

	// Stop each member of the group by base name (stops all instances), ignoring expected shutdown errors
	var firstUnexpectedErr error
	for _, member := range gs.Members {
		if err := apiClient.StopAll(member.Name, f.Wait); err != nil {
			if !isExpectedShutdownError(err) && firstUnexpectedErr == nil {
				firstUnexpectedErr = fmt.Errorf("failed to stop %s: %w", member.Name, err)
			}
			// Continue with other members even if this one had an error
		}
	}

	return firstUnexpectedErr
}

// GroupStatus prints status for a group from config
func (c *command) GroupStatus(f GroupFlags) error {
	if f.ConfigPath == "" {
		return fmt.Errorf("group-status requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-status requires --group name")
	}

	// Always use API - default to local daemon if not specified
	apiUrl := f.APIUrl
	if apiUrl == "" {
		apiUrl = "http://127.0.0.1:8080/api" // Default local daemon
	}

	apiClient := NewAPIClient(apiUrl, f.APITimeout)
	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable at %s - please start daemon first with 'provisr serve'", apiUrl)
	}

	return c.groupStatusViaAPI(f, apiClient)
}

// groupStatusViaAPI gets group status using the daemon API
func (c *command) groupStatusViaAPI(f GroupFlags, apiClient *APIClient) error {
	config, err := provisr.LoadConfig(f.ConfigPath)
	if err != nil {
		return err
	}

	gs := findGroupByName(config.GroupSpecs, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}

	// Get status for each member of the group
	groupStatus := make(map[string]interface{})
	for _, member := range gs.Members {
		if member.Instances > 1 {
			// Handle multiple instances
			instanceStatuses := make(map[string]interface{})
			for i := 1; i <= member.Instances; i++ {
				instanceName := fmt.Sprintf("%s-%d", member.Name, i)
				result, err := apiClient.GetStatus(instanceName)
				if err != nil {
					instanceStatuses[instanceName] = map[string]interface{}{
						"name":    instanceName,
						"running": false,
						"error":   err.Error(),
					}
				} else {
					instanceStatuses[instanceName] = result
				}
			}
			groupStatus[member.Name] = instanceStatuses
		} else {
			// Single instance
			result, err := apiClient.GetStatus(member.Name)
			if err != nil {
				groupStatus[member.Name] = map[string]interface{}{
					"name":    member.Name,
					"running": false,
					"error":   err.Error(),
				}
			} else {
				groupStatus[member.Name] = result
			}
		}
	}

	printJSON(groupStatus)
	return nil
}
