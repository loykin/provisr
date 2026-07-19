// Package api contains stable HTTP wire contracts shared by transports and
// API clients. It deliberately contains no Gin handlers or storage adapters.
package api

import corehistory "github.com/loykin/provisr/core/history"

type ErrorResponse struct {
	Error string `json:"error"`
}

type OKResponse struct {
	OK bool `json:"ok"`
}

type HistoryResponse struct {
	Rows  []corehistory.Entry `json:"rows"`
	Total int                 `json:"total"`
}

type GroupMember struct {
	Name      string `json:"name"`
	Instances int    `json:"instances"`
}

type GroupInfo struct {
	Name    string        `json:"name"`
	Members []GroupMember `json:"members"`
	State   string        `json:"state"`
	Running int           `json:"running"`
	Total   int           `json:"total"`
}

// RuntimeStatus contains only non-sensitive capability state for the web UI.
type RuntimeStatus struct {
	AuthEnabled          bool `json:"auth_enabled"`
	MetricsEnabled       bool `json:"metrics_enabled"`
	HistoryEnabled       bool `json:"history_enabled"`
	CronSchedulerEnabled bool `json:"cron_scheduler_enabled"`
	ProgramPersistence   bool `json:"program_persistence"`
	ConfiguredGroupCount int  `json:"configured_group_count"`
}
