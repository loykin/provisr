// Package stats defines the optional process resource statistics port used by
// the core. Collection and export implementations live in adapter packages.
package stats

import (
	"context"
	"time"
)

type ProcessMetrics struct {
	PID        int32     `json:"pid"`
	Name       string    `json:"name"`
	CPUPercent float64   `json:"cpu_percent"`
	MemoryMB   float64   `json:"memory_mb"`
	MemoryRSS  uint64    `json:"memory_rss"`
	MemoryVMS  uint64    `json:"memory_vms"`
	MemorySwap uint64    `json:"memory_swap,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	NumThreads int32     `json:"num_threads"`
	NumFDs     int32     `json:"num_fds,omitempty"`
}

type Collector interface {
	Start(context.Context, func() map[string]int32) error
	Stop()
	IsEnabled() bool
	GetMetrics(string) (ProcessMetrics, bool)
	GetHistory(string) ([]ProcessMetrics, bool)
	GetAllMetrics() map[string]ProcessMetrics
}
