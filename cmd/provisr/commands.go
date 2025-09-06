package main

import (
	"fmt"
	"time"

	"github.com/loykin/provisr"
)

// Flag structs to decouple cobra from logic for testing.
type StartFlags struct {
	ConfigPath      string
	UseOSEnv        bool
	EnvKVs          []string
	EnvFiles        []string
	Name            string
	Cmd             string
	PIDFile         string
	Retries         int
	RetryInterval   time.Duration
	AutoRestart     bool
	RestartInterval time.Duration
	StartDuration   time.Duration
	Instances       int
}

type StatusFlags struct {
	ConfigPath string
	Name       string
}

type StopFlags struct {
	ConfigPath string
	Name       string
	Wait       time.Duration
}

type CronFlags struct {
	ConfigPath string
	// For tests we can set NonBlocking to avoid infinite block
	NonBlocking bool
}

type GroupFlags struct {
	ConfigPath string
	GroupName  string
	Wait       time.Duration
}

func cmdStart(mgr *provisr.Manager, f StartFlags) error {
	if f.ConfigPath != "" {
		if genv, err := provisr.LoadGlobalEnv(f.ConfigPath); err == nil && len(genv) > 0 {
			mgr.SetGlobalEnv(genv)
		}
		specs, err := provisr.LoadSpecs(f.ConfigPath)
		if err != nil {
			return err
		}
		if err := startFromSpecs(mgr, specs); err != nil {
			return err
		}
		printJSON(statusesByBase(mgr, specs))
		return nil
	}
	// Apply global env from flags when not using config
	applyGlobalEnvFromFlags(mgr, f.UseOSEnv, f.EnvFiles, f.EnvKVs)
	sp := provisr.Spec{
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
	if f.Instances > 1 {
		return mgr.StartN(sp)
	}
	return mgr.Start(sp)
}

func cmdStatus(mgr *provisr.Manager, f StatusFlags) error {
	if f.ConfigPath != "" {
		specs, err := provisr.LoadSpecs(f.ConfigPath)
		if err != nil {
			return err
		}
		printJSON(statusesByBase(mgr, specs))
		return nil
	}
	sts, _ := mgr.StatusAll(f.Name)
	printJSON(sts)
	return nil
}

func cmdStop(mgr *provisr.Manager, f StopFlags) error {
	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
	}
	if f.ConfigPath != "" {
		specs, err := provisr.LoadSpecs(f.ConfigPath)
		if err != nil {
			return err
		}
		for _, sp := range specs {
			_ = mgr.StopAll(sp.Name, f.Wait)
		}
		printJSON(statusesByBase(mgr, specs))
		return nil
	}
	_ = mgr.StopAll(f.Name, f.Wait)
	sts, _ := mgr.StatusAll(f.Name)
	printJSON(sts)
	return nil
}

func cmdCron(mgr *provisr.Manager, f CronFlags) error {
	if f.ConfigPath == "" {
		return fmt.Errorf("cron requires --config file with processes having schedule")
	}
	if genv, err := provisr.LoadGlobalEnv(f.ConfigPath); err == nil && len(genv) > 0 {
		mgr.SetGlobalEnv(genv)
	}
	jobs, err := provisr.LoadCronJobs(f.ConfigPath)
	if err != nil {
		return err
	}
	sch := provisr.NewCronScheduler(mgr)
	for _, j := range jobs {
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

func runGroupStart(mgr *provisr.Manager, f GroupFlags) error {
	if f.ConfigPath == "" {
		return fmt.Errorf("group-start requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-start requires --group name")
	}
	if genv, err := provisr.LoadGlobalEnv(f.ConfigPath); err == nil && len(genv) > 0 {
		mgr.SetGlobalEnv(genv)
	}
	groups, err := provisr.LoadGroups(f.ConfigPath)
	if err != nil {
		return err
	}
	gs := findGroupByName(groups, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}
	g := provisr.NewGroup(mgr)
	return g.Start(*gs)
}

func runGroupStop(mgr *provisr.Manager, f GroupFlags) error {
	if f.Wait <= 0 {
		f.Wait = 3 * time.Second
	}
	if f.ConfigPath == "" {
		return fmt.Errorf("group-stop requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-stop requires --group name")
	}
	groups, err := provisr.LoadGroups(f.ConfigPath)
	if err != nil {
		return err
	}
	gs := findGroupByName(groups, f.GroupName)
	if gs == nil {
		return fmt.Errorf("group %s not found in config", f.GroupName)
	}
	g := provisr.NewGroup(mgr)
	return g.Stop(*gs, f.Wait)
}

func runGroupStatus(mgr *provisr.Manager, f GroupFlags) error {
	if f.ConfigPath == "" {
		return fmt.Errorf("group-status requires --config")
	}
	if f.GroupName == "" {
		return fmt.Errorf("group-status requires --group name")
	}
	groups, err := provisr.LoadGroups(f.ConfigPath)
	if err != nil {
		return err
	}
	gs := findGroupByName(groups, f.GroupName)
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
