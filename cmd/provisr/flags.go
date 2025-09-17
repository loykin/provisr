package main

import "time"

// StartFlags Flag structs to decouple cobra from logic for testing.
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
	Detailed   bool          // Show detailed state information
	Watch      bool          // Watch mode for continuous monitoring
	Interval   time.Duration // Watch interval
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
