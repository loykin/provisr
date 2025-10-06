package main

import (
	"fmt"
	"time"
)

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
