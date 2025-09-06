package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/loykin/provisr"
)

func applyGlobalEnvFromFlags(mgr *provisr.Manager, useOSEnv bool, envFiles []string, envKVs []string) {
	if useOSEnv {
		mgr.SetGlobalEnv(os.Environ())
	}
	if len(envFiles) > 0 {
		for _, f := range envFiles {
			if pairs, err := provisr.LoadEnv(f); err == nil && len(pairs) > 0 {
				mgr.SetGlobalEnv(pairs)
			}
		}
	}
	if len(envKVs) > 0 {
		mgr.SetGlobalEnv(envKVs)
	}
}

func startFromSpecs(mgr *provisr.Manager, specs []provisr.Spec) error {
	for _, sp := range specs {
		if sp.Instances > 1 {
			if err := mgr.StartN(sp); err != nil {
				return err
			}
		} else {
			if err := mgr.Start(sp); err != nil {
				return err
			}
		}
	}
	return nil
}

func statusesByBase(mgr *provisr.Manager, specs []provisr.Spec) map[string][]provisr.Status {
	all := make(map[string][]provisr.Status)
	for _, sp := range specs {
		sts, _ := mgr.StatusAll(sp.Name)
		all[sp.Name] = sts
	}
	return all
}

func findGroupByName(groups []provisr.GroupSpec, name string) *provisr.GroupSpec {
	for i := range groups {
		if groups[i].Name == name {
			return &groups[i]
		}
	}
	return nil
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
