package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/process"
)

// Start Method-style handlers bound to a command with an embedded manager
func (c *command) Start(f StartFlags) error {
	// Try to use authenticated API client first
	apiClient, err := c.createAuthenticatedAPIClient(f.APIUrl, f.APITimeout)
	if err != nil {
		return err
	}

	// Default to local daemon if no URL specified and no session
	if apiClient.baseURL == "" {
		apiClient = NewAPIClient("http://127.0.0.1:8080/api", f.APITimeout)
	}

	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable - please start daemon first with 'provisr serve'")
	}

	return c.startViaAPI(f, apiClient)
}

// startViaAPI starts processes using the daemon API
func (c *command) startViaAPI(f StartFlags, apiClient *APIClient) error {
	// Single process start - only resume existing registered process
	if f.Name == "" {
		return fmt.Errorf("process name is required")
	}

	return apiClient.StartProcess(f.Name)
}

// Status prints status information, optionally loading specs from config for base queries
func (c *command) Status(f StatusFlags) error {
	// Try to use authenticated API client first
	apiClient, err := c.createAuthenticatedAPIClient(f.APIUrl, f.APITimeout)
	if err != nil {
		return err
	}

	// Default to local daemon if no URL specified and no session
	if apiClient.baseURL == "" {
		apiClient = NewAPIClient("http://127.0.0.1:8080/api", f.APITimeout)
	}

	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable - please start daemon first with 'provisr serve'")
	}

	return c.statusViaAPI(f, apiClient)
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

// Stop stops processes by name/base from flags or config
func (c *command) Stop(f StopFlags) error {
	// Try to use authenticated API client first
	apiClient, err := c.createAuthenticatedAPIClient(f.APIUrl, f.APITimeout)
	if err != nil {
		return err
	}

	// Default to local daemon if no URL specified and no session
	if apiClient.baseURL == "" {
		apiClient = NewAPIClient("http://127.0.0.1:8080/api", f.APITimeout)
	}

	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
	}

	if !apiClient.IsReachable() {
		return fmt.Errorf("daemon not reachable - please start daemon first with 'provisr serve'")
	}

	return c.stopViaAPI(f, apiClient)
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

// Register registers a new process by creating a program file
func (c *command) Register(f RegisterFlags) error {
	if f.APIUrl != "" {
		apiClient := NewAPIClient(f.APIUrl, f.APITimeout)
		if !apiClient.IsReachable() {
			return fmt.Errorf("daemon not reachable at %s", f.APIUrl)
		}
		return c.registerViaAPI(f, apiClient)
	}

	// Local registration - create program file
	return c.registerLocally(f)
}

// registerViaAPI registers a process via the daemon API
func (c *command) registerViaAPI(f RegisterFlags, apiClient *APIClient) error {
	// Create process spec for API
	spec := map[string]interface{}{
		"name":         f.Name,
		"command":      f.Command,
		"work_dir":     f.WorkDir,
		"auto_restart": f.AutoStart, // AutoStart maps to auto_restart
	}

	if f.LogDir != "" {
		spec["log"] = map[string]interface{}{
			"file": map[string]interface{}{
				"dir": f.LogDir,
			},
		}
	}

	return apiClient.RegisterProcess(spec)
}

// registerLocally creates a program file in the programs directory
func (c *command) registerLocally(f RegisterFlags) error {
	// Get programs directory from config
	programsDir, err := c.getProgramsDirectory()
	if err != nil {
		return err
	}

	// Create programs directory if it doesn't exist
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create programs directory: %w", err)
	}

	// Create process spec
	spec := process.Spec{
		Name:        f.Name,
		Command:     f.Command,
		WorkDir:     f.WorkDir,
		AutoRestart: f.AutoStart, // AutoStart maps to AutoRestart in Spec
	}

	// Add log configuration if provided
	if f.LogDir != "" {
		spec.Log.File.Dir = f.LogDir
	}

	// Create program file as JSON
	programFile := filepath.Join(programsDir, f.Name+".json")

	// Check if program already exists
	if _, err := os.Stat(programFile); err == nil {
		return fmt.Errorf("process '%s' is already registered", f.Name)
	}

	// Convert spec to JSON-friendly format
	programData := map[string]interface{}{
		"name":         spec.Name,
		"command":      spec.Command,
		"work_dir":     spec.WorkDir,
		"auto_restart": spec.AutoRestart,
	}

	if f.LogDir != "" {
		programData["log"] = map[string]interface{}{
			"file": map[string]interface{}{
				"dir": f.LogDir,
			},
		}
	}

	// Write JSON file
	jsonData, err := json.MarshalIndent(programData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal program data: %w", err)
	}

	if err := os.WriteFile(programFile, jsonData, 0o644); err != nil {
		return fmt.Errorf("failed to write program file: %w", err)
	}

	fmt.Printf("Process '%s' registered successfully in %s\n", f.Name, programFile)
	return nil
}

// Unregister removes a process by deleting its program file
func (c *command) Unregister(f UnregisterFlags) error {
	if f.APIUrl != "" {
		apiClient := NewAPIClient(f.APIUrl, f.APITimeout)
		if !apiClient.IsReachable() {
			return fmt.Errorf("daemon not reachable at %s", f.APIUrl)
		}
		return c.unregisterViaAPI(f, apiClient)
	}

	// Local unregistration - delete program file
	return c.unregisterLocally(f)
}

// unregisterViaAPI unregisters a process via the daemon API
func (c *command) unregisterViaAPI(f UnregisterFlags, apiClient *APIClient) error {
	return apiClient.UnregisterProcess(f.Name)
}

// unregisterLocally removes a program file from the programs directory
func (c *command) unregisterLocally(f UnregisterFlags) error {
	// Check if process is defined in config.toml
	if c.isProcessInConfigFile(f.Name) {
		return fmt.Errorf("cannot unregister process '%s': it is defined in config.toml", f.Name)
	}

	// Get programs directory from config
	programsDir, err := c.getProgramsDirectory()
	if err != nil {
		return err
	}

	// Find and remove program file
	extensions := []string{".json", ".toml", ".yaml", ".yml"}
	var foundFile string

	for _, ext := range extensions {
		programFile := filepath.Join(programsDir, f.Name+ext)
		if _, err := os.Stat(programFile); err == nil {
			foundFile = programFile
			break
		}
	}

	if foundFile == "" {
		return fmt.Errorf("process '%s' is not registered", f.Name)
	}

	if err := os.Remove(foundFile); err != nil {
		return fmt.Errorf("failed to remove program file: %w", err)
	}

	fmt.Printf("Process '%s' unregistered successfully (removed %s)\n", f.Name, foundFile)
	return nil
}

// RegisterFile registers a process from an existing JSON file
func (c *command) RegisterFile(f RegisterFileFlags) error {
	if f.APIUrl != "" {
		apiClient := NewAPIClient(f.APIUrl, f.APITimeout)
		if !apiClient.IsReachable() {
			return fmt.Errorf("daemon not reachable at %s", f.APIUrl)
		}
		return c.registerFileViaAPI(f, apiClient)
	}

	// Local file registration
	return c.registerFileLocally(f)
}

// registerFileViaAPI registers a process from file via the daemon API
func (c *command) registerFileViaAPI(f RegisterFileFlags, apiClient *APIClient) error {
	// Read and parse the JSON file
	spec, err := c.parseProcessFile(f.FilePath)
	if err != nil {
		return err
	}

	return apiClient.RegisterProcess(spec)
}

// registerFileLocally copies a JSON file to the programs directory
func (c *command) registerFileLocally(f RegisterFileFlags) error {
	// Validate and parse the JSON file first
	spec, err := c.parseProcessFile(f.FilePath)
	if err != nil {
		return err
	}

	// Extract process name from the parsed spec
	processName, ok := spec["name"].(string)
	if !ok || processName == "" {
		return fmt.Errorf("process name is required in JSON file")
	}

	// Get programs directory
	programsDir, err := c.getProgramsDirectory()
	if err != nil {
		return err
	}

	// Create programs directory if it doesn't exist
	if err := os.MkdirAll(programsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create programs directory: %w", err)
	}

	// Determine target file name
	targetFile := filepath.Join(programsDir, processName+".json")

	// Check if process already exists
	if _, err := os.Stat(targetFile); err == nil {
		return fmt.Errorf("process '%s' is already registered", processName)
	}

	// Copy the file to programs directory
	sourceData, err := os.ReadFile(f.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(targetFile, sourceData, 0o644); err != nil {
		return fmt.Errorf("failed to write program file: %w", err)
	}

	fmt.Printf("Process '%s' registered successfully from %s to %s\n", processName, f.FilePath, targetFile)
	return nil
}

// parseProcessFile reads and validates a process configuration file
func (c *command) parseProcessFile(filePath string) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON
	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse JSON file: %w", err)
	}

	// Basic validation
	if err := c.validateProcessSpec(spec); err != nil {
		return nil, fmt.Errorf("invalid process specification: %w", err)
	}

	return spec, nil
}

// validateProcessSpec validates the basic structure of a process spec
func (c *command) validateProcessSpec(spec map[string]interface{}) error {
	// Check required fields
	name, nameExists := spec["name"]
	if !nameExists {
		return fmt.Errorf("'name' field is required")
	}

	nameStr, nameOK := name.(string)
	if !nameOK || nameStr == "" {
		return fmt.Errorf("'name' must be a non-empty string")
	}

	command, commandExists := spec["command"]
	if !commandExists {
		return fmt.Errorf("'command' field is required")
	}

	commandStr, commandOK := command.(string)
	if !commandOK || commandStr == "" {
		return fmt.Errorf("'command' must be a non-empty string")
	}

	// Validate optional fields if present
	if workDir, exists := spec["work_dir"]; exists {
		if _, ok := workDir.(string); !ok {
			return fmt.Errorf("'work_dir' must be a string")
		}
	}

	if autoRestart, exists := spec["auto_restart"]; exists {
		if _, ok := autoRestart.(bool); !ok {
			return fmt.Errorf("'auto_restart' must be a boolean")
		}
	}

	// Validate log configuration if present
	if logConfig, exists := spec["log"]; exists {
		logMap, ok := logConfig.(map[string]interface{})
		if !ok {
			return fmt.Errorf("'log' must be an object")
		}

		if fileConfig, fileExists := logMap["file"]; fileExists {
			fileMap, fileOK := fileConfig.(map[string]interface{})
			if !fileOK {
				return fmt.Errorf("'log.file' must be an object")
			}

			if dir, dirExists := fileMap["dir"]; dirExists {
				if _, dirOK := dir.(string); !dirOK {
					return fmt.Errorf("'log.file.dir' must be a string")
				}
			}
		}
	}

	return nil
}

// getProgramsDirectory returns the programs directory path from config
func (c *command) getProgramsDirectory() (string, error) {
	// Try to find config file in current directory first
	configPath := "config.toml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// If not found, use default programs directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, "programs"), nil
	}

	// Read programs_directory from config.toml if it exists
	programsDir, err := c.readProgramsDirectoryFromConfig(configPath)
	if err != nil {
		// If config reading fails, use default
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, "programs"), nil
	}

	// Convert relative path to absolute
	if !filepath.IsAbs(programsDir) {
		configDir := filepath.Dir(configPath)
		programsDir = filepath.Join(configDir, programsDir)
	}

	return programsDir, nil
}

// readProgramsDirectoryFromConfig reads the programs_directory setting from config.toml
func (c *command) readProgramsDirectoryFromConfig(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	// Simple TOML parsing to find programs_directory
	// This is a basic implementation - in a full version, you'd use a TOML library
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "programs_directory") {
			// Extract value: programs_directory = "value"
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, `"`)
				return value, nil
			}
		}
	}

	// Default programs directory if not specified
	return "programs", nil
}

// isProcessInConfigFile checks if a process is defined in the main config.toml file
func (c *command) isProcessInConfigFile(processName string) bool {
	configPath := "config.toml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	// Simple check for process definitions in config.toml
	content := string(data)

	// Check for [[processes]] sections with the given name
	lines := strings.Split(content, "\n")
	inProcessSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Start of a processes section
		if strings.Contains(line, "[[processes]]") {
			inProcessSection = true
			continue
		}

		// Start of another section
		if strings.HasPrefix(line, "[[") && !strings.Contains(line, "[[processes]]") {
			inProcessSection = false
			continue
		}

		// Check for name field in processes section
		if inProcessSection && strings.HasPrefix(line, "name") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"`)
				if name == processName {
					return true
				}
			}
		}
	}

	return false
}
