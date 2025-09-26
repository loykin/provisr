package main

import "time"

// StartFlags Flag structs to decouple cobra from logic for testing.
type StartFlags struct {
	Name string
	// Remote daemon connection
	APIUrl     string
	APITimeout time.Duration
}

type StatusFlags struct {
	Name     string
	Detailed bool // Show detailed state information
	// Remote daemon connection
	APIUrl     string
	APITimeout time.Duration
}

type StopFlags struct {
	Name string
	Wait time.Duration
	// Remote daemon connection
	APIUrl     string
	APITimeout time.Duration
}

type CronFlags struct {
	// For tests we can set NonBlocking to avoid infinite block
	NonBlocking bool
	// Remote daemon connection
	APIUrl     string
	APITimeout time.Duration
}

type GroupFlags struct {
	GroupName string
	Wait      time.Duration
	// Remote daemon connection
	APIUrl     string
	APITimeout time.Duration
}

type ServeFlags struct {
	ConfigPath string
	Daemonize  bool
	PidFile    string
	LogFile    string
}
