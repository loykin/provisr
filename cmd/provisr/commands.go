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
	// Single process start - only resume existing registered process
	if f.Name == "" {
		return fmt.Errorf("process name is required")
	}

	return apiClient.StartProcess(f.Name)
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
	// Single process stop
	if f.Name == "" {
		return fmt.Errorf("process name is required")
	}

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

// GroupStart starts a group
func (c *command) GroupStart(f GroupFlags) error {
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
	err := apiClient.GroupStart(f.GroupName)
	if err != nil {
		return err
	}

	fmt.Printf("Started group: %s\n", f.GroupName)
	return nil
}

// GroupStop stops a group
func (c *command) GroupStop(f GroupFlags) error {
	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
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
	err := apiClient.GroupStop(f.GroupName, f.Wait)
	if err != nil {
		return err
	}

	fmt.Printf("Stopped group: %s\n", f.GroupName)
	return nil
}

// GroupStatus prints status for a group
func (c *command) GroupStatus(f GroupFlags) error {
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
	result, err := apiClient.GetGroupStatus(f.GroupName)
	if err != nil {
		return err
	}

	printJSON(result)
	return nil
}
