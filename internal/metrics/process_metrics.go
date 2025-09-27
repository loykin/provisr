package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v4/process"
)

// ProcessMetrics holds CPU and memory metrics for a single process
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
	NumFDs     int32     `json:"num_fds,omitempty"` // Unix only
}

// ProcessMetricsHistory stores historical metrics for a process
type ProcessMetricsHistory struct {
	ProcessName string           `json:"process_name"`
	Metrics     []ProcessMetrics `json:"metrics"`
	MaxSize     int              `json:"max_size"`
	mu          sync.RWMutex
	// Optimization: track the start index for circular buffer
	startIdx int
	count    int
}

// ProcessMetricsCollector manages CPU and memory monitoring for managed processes
type ProcessMetricsCollector struct {
	enabled    bool
	interval   time.Duration
	history    map[string]*ProcessMetricsHistory
	historyMu  sync.RWMutex
	maxHistory int
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup

	// Prometheus metrics for process monitoring
	processCPUPercent *prometheus.GaugeVec
	processMemoryMB   *prometheus.GaugeVec
	processNumThreads *prometheus.GaugeVec
	processNumFDs     *prometheus.GaugeVec
}

// ProcessMetricsConfig holds configuration for process metrics collection
type ProcessMetricsConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	Interval    time.Duration `mapstructure:"interval"`
	MaxHistory  int           `mapstructure:"max_history"`
	HistorySize int           `mapstructure:"history_size"` // alias for MaxHistory
}

// NewProcessMetricsCollector creates a new process metrics collector
func NewProcessMetricsCollector(config ProcessMetricsConfig) *ProcessMetricsCollector {
	maxHistory := config.MaxHistory
	if maxHistory == 0 {
		maxHistory = config.HistorySize
	}
	if maxHistory == 0 {
		maxHistory = 100 // default
	}

	interval := config.Interval
	if interval == 0 {
		interval = 5 * time.Second // default
	}

	return &ProcessMetricsCollector{
		enabled:    config.Enabled,
		interval:   interval,
		history:    make(map[string]*ProcessMetricsHistory),
		maxHistory: maxHistory,
		stopCh:     make(chan struct{}),
		processCPUPercent: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "cpu_percent",
				Help:      "CPU usage percentage for managed processes.",
			}, []string{"name"},
		),
		processMemoryMB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "memory_mb",
				Help:      "Memory usage in MB for managed processes.",
			}, []string{"name"},
		),
		processNumThreads: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "num_threads",
				Help:      "Number of threads for managed processes.",
			}, []string{"name"},
		),
		processNumFDs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "num_fds",
				Help:      "Number of file descriptors for managed processes (Unix only).",
			}, []string{"name"},
		),
	}
}

// RegisterMetrics registers the process metrics with the provided registerer
func (c *ProcessMetricsCollector) RegisterMetrics(r prometheus.Registerer) error {
	if !c.enabled {
		return nil
	}

	collectors := []prometheus.Collector{
		c.processCPUPercent,
		c.processMemoryMB,
		c.processNumThreads,
	}

	// Only register FD metrics on Unix systems
	if runtime.GOOS != "windows" {
		collectors = append(collectors, c.processNumFDs)
	}

	for _, collector := range collectors {
		if err := r.Register(collector); err != nil {
			// Ignore already registered errors
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				continue
			}
			return err
		}
	}

	return nil
}

// Start begins the periodic collection of process metrics
func (c *ProcessMetricsCollector) Start(ctx context.Context, getProcesses func() map[string]int32) error {
	if !c.enabled {
		return nil
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				processes := getProcesses()
				c.collectMetrics(processes)
			}
		}
	}()

	return nil
}

// Stop stops the metrics collection
func (c *ProcessMetricsCollector) Stop() {
	if !c.enabled {
		return
	}

	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.wg.Wait()
}

// collectMetrics collects CPU and memory metrics for the given processes
func (c *ProcessMetricsCollector) collectMetrics(processes map[string]int32) {
	timestamp := time.Now()

	// Batch process all metrics collection to reduce lock contention
	metricsResults := make(map[string]ProcessMetrics)

	for name, pid := range processes {
		if pid <= 0 {
			continue
		}

		metrics, err := c.getProcessMetrics(name, pid, timestamp)
		if err != nil {
			slog.Debug("Failed to collect metrics for process", "name", name, "pid", pid, "error", err)
			continue
		}

		metricsResults[name] = *metrics
	}

	// Batch update Prometheus metrics and history
	for name, metrics := range metricsResults {
		// Update Prometheus metrics
		c.processCPUPercent.WithLabelValues(name).Set(metrics.CPUPercent)
		c.processMemoryMB.WithLabelValues(name).Set(metrics.MemoryMB)
		c.processNumThreads.WithLabelValues(name).Set(float64(metrics.NumThreads))

		if runtime.GOOS != "windows" && metrics.NumFDs > 0 {
			c.processNumFDs.WithLabelValues(name).Set(float64(metrics.NumFDs))
		}

		// Store in history
		c.addToHistory(name, metrics)
	}

	// Clean up metrics for processes that no longer exist
	c.cleanupMetrics(processes)
}

// getProcessMetrics retrieves CPU and memory metrics for a single process
func (c *ProcessMetricsCollector) getProcessMetrics(name string, pid int32, timestamp time.Time) (*ProcessMetrics, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to create process handle: %w", err)
	}

	// Get CPU percentage (this may require a previous call for accurate calculation)
	cpuPercent, err := proc.CPUPercent()
	if err != nil {
		slog.Debug("Failed to get CPU percent", "name", name, "pid", pid, "error", err)
		cpuPercent = 0 // Use 0 if we can't get CPU usage
	}

	// Get memory info
	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	// Get number of threads
	numThreads, err := proc.NumThreads()
	if err != nil {
		slog.Debug("Failed to get thread count", "name", name, "pid", pid, "error", err)
		numThreads = 0
	}

	metrics := &ProcessMetrics{
		PID:        pid,
		Name:       name,
		CPUPercent: cpuPercent,
		MemoryMB:   float64(memInfo.RSS) / 1024 / 1024, // Convert bytes to MB
		MemoryRSS:  memInfo.RSS,
		MemoryVMS:  memInfo.VMS,
		Timestamp:  timestamp,
		NumThreads: numThreads,
	}

	// Get memory swap if available
	if memInfo.Swap > 0 {
		metrics.MemorySwap = memInfo.Swap
	}

	// Get file descriptor count (Unix only)
	if runtime.GOOS != "windows" {
		if numFDs, err := proc.NumFDs(); err == nil {
			metrics.NumFDs = numFDs
		}
	}

	return metrics, nil
}

// addToHistory adds metrics to the historical data using a circular buffer approach
func (c *ProcessMetricsCollector) addToHistory(name string, metrics ProcessMetrics) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()

	history, exists := c.history[name]
	if !exists {
		history = &ProcessMetricsHistory{
			ProcessName: name,
			Metrics:     make([]ProcessMetrics, c.maxHistory),
			MaxSize:     c.maxHistory,
			startIdx:    0,
			count:       0,
		}
		c.history[name] = history
	}

	history.mu.Lock()
	defer history.mu.Unlock()

	// Use circular buffer approach for O(1) operations
	if history.count < history.MaxSize {
		// Still filling the buffer
		history.Metrics[history.count] = metrics
		history.count++
	} else {
		// Buffer is full, overwrite oldest entry
		history.Metrics[history.startIdx] = metrics
		history.startIdx = (history.startIdx + 1) % history.MaxSize
	}
}

// cleanupMetrics removes metrics for processes that no longer exist
func (c *ProcessMetricsCollector) cleanupMetrics(activeProcesses map[string]int32) {
	c.historyMu.RLock()
	var toDelete []string
	for name := range c.history {
		if _, exists := activeProcesses[name]; !exists {
			toDelete = append(toDelete, name)
		}
	}
	c.historyMu.RUnlock()

	if len(toDelete) > 0 {
		c.historyMu.Lock()
		for _, name := range toDelete {
			delete(c.history, name)
			// Remove from Prometheus metrics
			c.processCPUPercent.DeleteLabelValues(name)
			c.processMemoryMB.DeleteLabelValues(name)
			c.processNumThreads.DeleteLabelValues(name)
			if runtime.GOOS != "windows" {
				c.processNumFDs.DeleteLabelValues(name)
			}
		}
		c.historyMu.Unlock()
	}
}

// GetMetrics returns the latest metrics for a specific process
func (c *ProcessMetricsCollector) GetMetrics(name string) (ProcessMetrics, bool) {
	if !c.enabled {
		return ProcessMetrics{}, false
	}

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	history, exists := c.history[name]
	if !exists {
		return ProcessMetrics{}, false
	}

	history.mu.RLock()
	defer history.mu.RUnlock()

	if history.count == 0 {
		return ProcessMetrics{}, false
	}

	// Return the most recent metrics from circular buffer
	var latestIdx int
	if history.count < history.MaxSize {
		latestIdx = history.count - 1
	} else {
		latestIdx = (history.startIdx - 1 + history.MaxSize) % history.MaxSize
	}
	return history.Metrics[latestIdx], true
}

// GetHistory returns the historical metrics for a specific process
func (c *ProcessMetricsCollector) GetHistory(name string) ([]ProcessMetrics, bool) {
	if !c.enabled {
		return nil, false
	}

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	history, exists := c.history[name]
	if !exists {
		return nil, false
	}

	history.mu.RLock()
	defer history.mu.RUnlock()

	if history.count == 0 {
		return nil, false
	}

	// Return metrics in chronological order from circular buffer
	result := make([]ProcessMetrics, history.count)
	if history.count < history.MaxSize {
		// Buffer not full yet, copy in order
		copy(result, history.Metrics[:history.count])
	} else {
		// Buffer is full, need to reconstruct order
		// Copy from startIdx to end
		n1 := copy(result, history.Metrics[history.startIdx:])
		// Copy from beginning to startIdx
		copy(result[n1:], history.Metrics[:history.startIdx])
	}
	return result, true
}

// GetAllMetrics returns the latest metrics for all processes
func (c *ProcessMetricsCollector) GetAllMetrics() map[string]ProcessMetrics {
	if !c.enabled {
		return make(map[string]ProcessMetrics)
	}

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	result := make(map[string]ProcessMetrics)
	for name, history := range c.history {
		history.mu.RLock()
		if history.count > 0 {
			// Get latest metrics from circular buffer
			var latestIdx int
			if history.count < history.MaxSize {
				latestIdx = history.count - 1
			} else {
				latestIdx = (history.startIdx - 1 + history.MaxSize) % history.MaxSize
			}
			result[name] = history.Metrics[latestIdx]
		}
		history.mu.RUnlock()
	}

	return result
}

// IsEnabled returns whether metrics collection is enabled
func (c *ProcessMetricsCollector) IsEnabled() bool {
	return c.enabled
}

// SetEnabled enables or disables metrics collection
func (c *ProcessMetricsCollector) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// AddToHistoryForTesting adds metrics to history for testing purposes
func (c *ProcessMetricsCollector) AddToHistoryForTesting(name string, metrics ProcessMetrics) {
	c.addToHistory(name, metrics)
}
