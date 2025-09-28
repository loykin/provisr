package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
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

// InstanceMetrics represents metrics for a single process instance
type InstanceMetrics struct {
	ProcessName string `json:"process_name"`
	InstanceID  string `json:"instance_id"`
	ProcessMetrics
}

// ProcessAggregatedMetrics holds aggregated metrics for all instances of a process
type ProcessAggregatedMetrics struct {
	ProcessName     string            `json:"process_name"`
	TotalInstances  int               `json:"total_instances"`
	AvgCPUPercent   float64           `json:"avg_cpu_percent"`
	TotalMemoryMB   float64           `json:"total_memory_mb"`
	AvgMemoryMB     float64           `json:"avg_memory_mb"`
	TotalNumThreads int32             `json:"total_num_threads"`
	TotalNumFDs     int32             `json:"total_num_fds"`
	Instances       []InstanceMetrics `json:"instances"`
	Timestamp       time.Time         `json:"timestamp"`
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

// ProcessInstanceHistory stores historical metrics for process instances
type ProcessInstanceHistory struct {
	ProcessName string                      `json:"process_name"`
	Instances   map[string][]ProcessMetrics `json:"instances"` // instanceID -> metrics history
	MaxSize     int                         `json:"max_size"`
	mu          sync.RWMutex
}

// ProcessMetricsCollector manages CPU and memory monitoring for managed processes
type ProcessMetricsCollector struct {
	enabled         bool
	interval        time.Duration
	history         map[string]*ProcessMetricsHistory  // legacy: processName -> history
	instanceHistory map[string]*ProcessInstanceHistory // processName -> instance history
	historyMu       sync.RWMutex
	maxHistory      int
	stopCh          chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup

	// Prometheus metrics for process monitoring with consistent labels
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

// parseProcessName extracts process name and instance ID from full name
// Examples: "app-1" -> ("app", "1"), "app" -> ("app", "0")
func parseProcessName(fullName string) (processName, instanceID string) {
	parts := strings.Split(fullName, "-")
	if len(parts) >= 2 {
		// Check if last part is numeric
		if _, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			processName = strings.Join(parts[:len(parts)-1], "-")
			instanceID = parts[len(parts)-1]
			return
		}
	}
	// If no numeric suffix, use full name as process name and "0" as instance
	return fullName, "0"
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
		enabled:         config.Enabled,
		interval:        interval,
		history:         make(map[string]*ProcessMetricsHistory),
		instanceHistory: make(map[string]*ProcessInstanceHistory),
		maxHistory:      maxHistory,
		stopCh:          make(chan struct{}),
		processCPUPercent: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "cpu_percent",
				Help:      "CPU usage percentage for managed processes.",
			}, []string{"process_name", "instance_id"},
		),
		processMemoryMB: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "memory_mb",
				Help:      "Memory usage in MB for managed processes.",
			}, []string{"process_name", "instance_id"},
		),
		processNumThreads: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "num_threads",
				Help:      "Number of threads for managed processes.",
			}, []string{"process_name", "instance_id"},
		),
		processNumFDs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "provisr",
				Subsystem: "process",
				Name:      "num_fds",
				Help:      "Number of file descriptors for managed processes (Unix only).",
			}, []string{"process_name", "instance_id"},
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
		processName, instanceID := parseProcessName(name)

		// Update Prometheus metrics with consistent labels
		c.processCPUPercent.WithLabelValues(processName, instanceID).Set(metrics.CPUPercent)
		c.processMemoryMB.WithLabelValues(processName, instanceID).Set(metrics.MemoryMB)
		c.processNumThreads.WithLabelValues(processName, instanceID).Set(float64(metrics.NumThreads))

		if runtime.GOOS != "windows" && metrics.NumFDs > 0 {
			c.processNumFDs.WithLabelValues(processName, instanceID).Set(float64(metrics.NumFDs))
		}

		// Store in both legacy and new history structures
		c.addToHistory(name, metrics)
		c.addToInstanceHistory(processName, instanceID, metrics)
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

// addToInstanceHistory adds metrics to the instance-based historical data
func (c *ProcessMetricsCollector) addToInstanceHistory(processName, instanceID string, metrics ProcessMetrics) {
	c.historyMu.Lock()
	defer c.historyMu.Unlock()

	history, exists := c.instanceHistory[processName]
	if !exists {
		history = &ProcessInstanceHistory{
			ProcessName: processName,
			Instances:   make(map[string][]ProcessMetrics),
			MaxSize:     c.maxHistory,
		}
		c.instanceHistory[processName] = history
	}

	history.mu.Lock()
	defer history.mu.Unlock()

	// Initialize instance history if it doesn't exist
	if history.Instances[instanceID] == nil {
		history.Instances[instanceID] = make([]ProcessMetrics, 0, c.maxHistory)
	}

	// Add new metrics to the instance history
	instanceMetrics := history.Instances[instanceID]
	if len(instanceMetrics) >= c.maxHistory {
		// Remove oldest entry
		instanceMetrics = instanceMetrics[1:]
	}
	instanceMetrics = append(instanceMetrics, metrics)
	history.Instances[instanceID] = instanceMetrics
}

// cleanupMetrics removes metrics for processes that no longer exist
func (c *ProcessMetricsCollector) cleanupMetrics(activeProcesses map[string]int32) {
	c.historyMu.RLock()
	var toDelete []string
	var toDeleteFromInstance []struct {
		processName string
		instanceID  string
	}

	for name := range c.history {
		if _, exists := activeProcesses[name]; !exists {
			toDelete = append(toDelete, name)
		}
	}

	// Also cleanup instance history
	for processName, history := range c.instanceHistory {
		history.mu.RLock()
		for instanceID := range history.Instances {
			fullName := processName + "-" + instanceID
			if instanceID == "0" {
				fullName = processName
			}
			if _, exists := activeProcesses[fullName]; !exists {
				toDeleteFromInstance = append(toDeleteFromInstance, struct {
					processName string
					instanceID  string
				}{processName, instanceID})
			}
		}
		history.mu.RUnlock()
	}
	c.historyMu.RUnlock()

	if len(toDelete) > 0 || len(toDeleteFromInstance) > 0 {
		c.historyMu.Lock()
		for _, name := range toDelete {
			delete(c.history, name)
			processName, instanceID := parseProcessName(name)
			// Remove from Prometheus metrics with new labels
			c.processCPUPercent.DeleteLabelValues(processName, instanceID)
			c.processMemoryMB.DeleteLabelValues(processName, instanceID)
			c.processNumThreads.DeleteLabelValues(processName, instanceID)
			if runtime.GOOS != "windows" {
				c.processNumFDs.DeleteLabelValues(processName, instanceID)
			}
		}

		// Cleanup instance history
		for _, item := range toDeleteFromInstance {
			if history, exists := c.instanceHistory[item.processName]; exists {
				history.mu.Lock()
				delete(history.Instances, item.instanceID)
				// If no instances left, remove the entire process history
				if len(history.Instances) == 0 {
					delete(c.instanceHistory, item.processName)
				}
				history.mu.Unlock()
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

// GetProcessMetrics returns aggregated metrics for a specific process
func (c *ProcessMetricsCollector) GetProcessMetrics(processName string) (ProcessAggregatedMetrics, bool) {
	if !c.enabled {
		return ProcessAggregatedMetrics{}, false
	}

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	history, exists := c.instanceHistory[processName]
	if !exists {
		return ProcessAggregatedMetrics{}, false
	}

	history.mu.RLock()
	defer history.mu.RUnlock()

	if len(history.Instances) == 0 {
		return ProcessAggregatedMetrics{}, false
	}

	var instances []InstanceMetrics
	var totalCPU, totalMemory float64
	var totalThreads, totalFDs int32
	timestamp := time.Now()

	for instanceID, metrics := range history.Instances {
		if len(metrics) == 0 {
			continue
		}

		// Get latest metrics for this instance
		latest := metrics[len(metrics)-1]
		instance := InstanceMetrics{
			ProcessName:    processName,
			InstanceID:     instanceID,
			ProcessMetrics: latest,
		}
		instances = append(instances, instance)

		totalCPU += latest.CPUPercent
		totalMemory += latest.MemoryMB
		totalThreads += latest.NumThreads
		totalFDs += latest.NumFDs

		if latest.Timestamp.After(timestamp) || timestamp.IsZero() {
			timestamp = latest.Timestamp
		}
	}

	if len(instances) == 0 {
		return ProcessAggregatedMetrics{}, false
	}

	return ProcessAggregatedMetrics{
		ProcessName:     processName,
		TotalInstances:  len(instances),
		AvgCPUPercent:   totalCPU / float64(len(instances)),
		TotalMemoryMB:   totalMemory,
		AvgMemoryMB:     totalMemory / float64(len(instances)),
		TotalNumThreads: totalThreads,
		TotalNumFDs:     totalFDs,
		Instances:       instances,
		Timestamp:       timestamp,
	}, true
}

// GetInstanceMetrics returns metrics for a specific process instance
func (c *ProcessMetricsCollector) GetInstanceMetrics(processName, instanceID string) (InstanceMetrics, bool) {
	if !c.enabled {
		return InstanceMetrics{}, false
	}

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	history, exists := c.instanceHistory[processName]
	if !exists {
		return InstanceMetrics{}, false
	}

	history.mu.RLock()
	defer history.mu.RUnlock()

	metrics, exists := history.Instances[instanceID]
	if !exists || len(metrics) == 0 {
		return InstanceMetrics{}, false
	}

	latest := metrics[len(metrics)-1]
	return InstanceMetrics{
		ProcessName:    processName,
		InstanceID:     instanceID,
		ProcessMetrics: latest,
	}, true
}

// GetAllProcessMetrics returns aggregated metrics for all processes
func (c *ProcessMetricsCollector) GetAllProcessMetrics() map[string]ProcessAggregatedMetrics {
	if !c.enabled {
		return make(map[string]ProcessAggregatedMetrics)
	}

	result := make(map[string]ProcessAggregatedMetrics)

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	for processName := range c.instanceHistory {
		if metrics, ok := c.GetProcessMetrics(processName); ok {
			result[processName] = metrics
		}
	}

	return result
}

// GetInstanceHistory returns historical metrics for a specific instance
func (c *ProcessMetricsCollector) GetInstanceHistory(processName, instanceID string) ([]ProcessMetrics, bool) {
	if !c.enabled {
		return nil, false
	}

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	history, exists := c.instanceHistory[processName]
	if !exists {
		return nil, false
	}

	history.mu.RLock()
	defer history.mu.RUnlock()

	metrics, exists := history.Instances[instanceID]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	result := make([]ProcessMetrics, len(metrics))
	copy(result, metrics)
	return result, true
}

// AddToHistoryForTesting adds metrics to history for testing purposes
func (c *ProcessMetricsCollector) AddToHistoryForTesting(name string, metrics ProcessMetrics) {
	c.addToHistory(name, metrics)
	processName, instanceID := parseProcessName(name)
	c.addToInstanceHistory(processName, instanceID, metrics)
}
