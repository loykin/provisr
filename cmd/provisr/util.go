package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/loykin/provisr"
)

func applyGlobalEnvFromFlags(mgr *provisr.Manager, useOSEnv bool, envKVs []string) {
	if useOSEnv {
		mgr.SetGlobalEnv(os.Environ())
	}
	if len(envKVs) > 0 {
		mgr.SetGlobalEnv(envKVs)
	}
}

func startFromSpecs(mgr *provisr.Manager, specs []provisr.Spec) error {
	// Simple priority sort
	sortedSpecs := make([]provisr.Spec, len(specs))
	copy(sortedSpecs, specs)
	sort.SliceStable(sortedSpecs, func(i, j int) bool {
		return sortedSpecs[i].Priority < sortedSpecs[j].Priority
	})

	for _, sp := range sortedSpecs {
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

// printDetailedStatus prints detailed status information for processes
func printDetailedStatus(statuses []provisr.Status) {
	if len(statuses) == 0 {
		fmt.Println("No processes found")
		return
	}

	fmt.Printf("%-20s %-10s %-10s %-6s %-8s %-8s %-10s\n",
		"NAME", "STATE", "RUNNING", "PID", "RESTARTS", "UPTIME", "DETECTED_BY")
	fmt.Println(strings.Repeat("-", 80))

	for _, st := range statuses {
		uptime := getUptime(st)
		fmt.Printf("%-20s %-10s %-10v %-6d %-8d %-8s %-10s\n",
			st.Name, st.State, st.Running, st.PID, st.Restarts, uptime, st.DetectedBy)
	}
}

// printDetailedStatusByBase prints detailed status grouped by base name
func printDetailedStatusByBase(mgr *provisr.Manager, specs []provisr.Spec) {
	for _, sp := range specs {
		sts, _ := mgr.StatusAll(sp.Name)
		fmt.Printf("\n=== %s ===\n", sp.Name)
		printDetailedStatus(sts)
	}
}

// getUptime calculates and formats process uptime
func getUptime(st provisr.Status) string {
	if !st.Running {
		return "N/A"
	}

	if st.StartedAt.IsZero() {
		return "Unknown"
	}

	uptime := time.Since(st.StartedAt)
	if uptime < time.Minute {
		return fmt.Sprintf("%ds", int(uptime.Seconds()))
	} else if uptime < time.Hour {
		return fmt.Sprintf("%dm", int(uptime.Minutes()))
	} else {
		hours := int(uptime.Hours())
		minutes := int(uptime.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}
