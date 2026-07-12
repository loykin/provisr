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

	corestats "github.com/loykin/provisr/core/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v4/process"
)

// ProcessMetrics holds CPU and memory metrics for a single process
type ProcessMetrics = corestats.ProcessMetrics

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
	Enabled    bool          `mapstructure:"enabled"`
	Interval   time.Duration `mapstructure:"interval"`
	MaxHistory int           `mapstructure:"max_history"`
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
		maxHistory = 100 // default
	}

	interval := config.Interval
	if interval == 0 {
		interval = 5 * time.Second // default
	}

	return &ProcessMetricsCollector{
		enabled:         config.Enabled,
		interval:        interval,
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

// addToHistory maps a full process instance name to the canonical instance history.
func (c *ProcessMetricsCollector) addToHistory(name string, metrics ProcessMetrics) {
	processName, instanceID := parseProcessName(name)
	c.addToInstanceHistory(processName, instanceID, metrics)
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
	var toDeleteFromInstance []struct {
		processName string
		instanceID  string
	}

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

	if len(toDeleteFromInstance) > 0 {
		c.historyMu.Lock()
		for _, item := range toDeleteFromInstance {
			c.processCPUPercent.DeleteLabelValues(item.processName, item.instanceID)
			c.processMemoryMB.DeleteLabelValues(item.processName, item.instanceID)
			c.processNumThreads.DeleteLabelValues(item.processName, item.instanceID)
			if runtime.GOOS != "windows" {
				c.processNumFDs.DeleteLabelValues(item.processName, item.instanceID)
			}
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

	processName, instanceID := parseProcessName(name)
	instance, ok := c.GetInstanceMetrics(processName, instanceID)
	if !ok {
		return ProcessMetrics{}, false
	}
	return instance.ProcessMetrics, true
}

// GetHistory returns the historical metrics for a specific process
func (c *ProcessMetricsCollector) GetHistory(name string) ([]ProcessMetrics, bool) {
	if !c.enabled {
		return nil, false
	}

	processName, instanceID := parseProcessName(name)
	return c.GetInstanceHistory(processName, instanceID)
}

// GetAllMetrics returns the latest metrics for all processes
func (c *ProcessMetricsCollector) GetAllMetrics() map[string]ProcessMetrics {
	if !c.enabled {
		return make(map[string]ProcessMetrics)
	}

	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	result := make(map[string]ProcessMetrics)
	for processName, history := range c.instanceHistory {
		history.mu.RLock()
		for instanceID, metrics := range history.Instances {
			if len(metrics) > 0 {
				name := processName
				if instanceID != "0" {
					name += "-" + instanceID
				}
				result[name] = metrics[len(metrics)-1]
			}
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
}
