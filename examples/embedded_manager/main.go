package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/loykin/provisr"
)

// Example: Using provisr.Manager to manage processes with config-driven ApplyConfig.
//
// This example demonstrates:
//  1. Loading a unified config TOML file.
//  2. Applying it to the Manager via ApplyConfig, which will:
//     - recover already running processes from PID files (if configured),
//     - start missing ones,
//     - and gracefully stop/remove programs not present in the config.
//  3. Printing statuses.
func main() {
	mgr := provisr.New()

	// Try to locate the example's own config first, then fall back to repo config.
	candidateLocal := filepath.Join("examples", "embedded_manager", "config", "config.toml")
	candidateRel := filepath.Join("config", "config.toml")

	cfgPath := ""
	if _, err := os.Stat(candidateLocal); err == nil {
		cfgPath = candidateLocal
	} else if _, err := os.Stat(candidateRel); err == nil {
		cfgPath = candidateRel
	} else {
		panic("could not locate config.toml for embedded_manager example")
	}

	cfg, err := provisr.LoadConfig(cfgPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Apply global env from config (if any)
	if len(cfg.GlobalEnv) > 0 {
		mgr.SetGlobalEnv(cfg.GlobalEnv)
	}

	// Apply config to manager (recover/start/cleanup)
	if err := mgr.ApplyConfig(cfg.Specs); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ApplyConfig error: %v\n", err)
	}

	// Give processes a moment to start
	time.Sleep(200 * time.Millisecond)

	// Collect and print statuses for each program base name
	statusMap := make(map[string][]provisr.Status)
	for _, sp := range cfg.Specs {
		sts, _ := mgr.StatusAll(sp.Name)
		statusMap[sp.Name] = sts
	}
	b, _ := json.MarshalIndent(statusMap, "", "  ")
	fmt.Println(string(b))
}
