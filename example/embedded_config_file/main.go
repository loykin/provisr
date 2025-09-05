package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/loykin/provisr"
)

// This example loads a TOML config file and starts the defined processes using the public provisr facade.
func main() {
	mgr := provisr.New()
	// Use the sample config in the repo (adjust path if running from a different cwd)
	cfgPath := filepath.Join("config", "config.toml")
	// Load top-level env and apply
	if genv, err := provisr.LoadEnv(cfgPath); err == nil && len(genv) > 0 {
		mgr.SetGlobalEnv(genv)
	}
	// Load specs
	specs, err := provisr.LoadSpecs(cfgPath)
	if err != nil {
		panic(err)
	}
	// Start all
	for _, sp := range specs {
		if sp.Instances > 1 {
			if err := mgr.StartN(sp); err != nil {
				panic(err)
			}
		} else {
			if err := mgr.Start(sp); err != nil {
				panic(err)
			}
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
}
