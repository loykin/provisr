package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/loykin/provisr"
)

// This example loads a TOML config file and starts the defined processes using the public provisr facade.
func main() {
	mgr := provisr.New()

	// Determine config path:
	// 1) Prefer the example-local config (when running via `go run ./example/embedded_config_file`).
	// 2) Fallback to the repo-root config/config.toml when running from the example directory.
	cwd, _ := os.Getwd()
	exampleDir := cwd
	// If running from repo root, the example path will end with "/example/embedded_config_file" after package resolution,
	// but at runtime cwd is where `go run` was invoked. Try both candidates robustly.
	candidateLocal := filepath.Join("examples", "embedded_config_file", "config", "config.toml")
	candidateRel := filepath.Join("config", "config.toml")

	cfgPath := ""
	if _, err := os.Stat(candidateLocal); err == nil {
		cfgPath = candidateLocal
	} else if _, err := os.Stat(filepath.Join(exampleDir, candidateRel)); err == nil {
		cfgPath = filepath.Join(exampleDir, candidateRel)
	} else if _, err := os.Stat(candidateRel); err == nil {
		cfgPath = candidateRel
	} else {
		panic("could not locate config.toml: tried examples/embedded_config_file/config/config.toml and ./config/config.toml")
	}

	// Load complete configuration at once
	config, err := provisr.LoadConfig(cfgPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to load config from %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	// Apply global environment
	if len(config.GlobalEnv) > 0 {
		mgr.SetGlobalEnv(config.GlobalEnv)
	}

	// Get specs from config
	specs := config.Specs
	// Start all (graceful error handling: don't panic; report and continue)
	var hadError bool
	for _, sp := range specs {
		var startErr error
		if sp.Instances > 1 {
			startErr = mgr.StartN(sp)
		} else {
			startErr = mgr.Start(sp)
		}
		if startErr != nil {
			hadError = true
			_, _ = fmt.Fprintf(os.Stderr, "failed to start %s: %v\n", sp.Name, startErr)
			_, _ = fmt.Fprintln(os.Stderr, "Hints: check if required binaries exist (e.g., python3), and if the port is free. You can edit the example config under examples/embedded_config_file/config/config.toml to use a different port.")
		}
	}
	// Print statuses by base name
	statusMap := make(map[string][]provisr.Status)
	for _, sp := range specs {
		sts, _ := mgr.StatusAll(sp.Name)
		statusMap[sp.Name] = sts
	}
	b, _ := json.MarshalIndent(statusMap, "", "  ")
	fmt.Println(string(b))
	if hadError {
		os.Exit(2)
	}
}
