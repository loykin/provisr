package metrics

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewProcessMetricsCollector(t *testing.T) {
	tests := []struct {
		name     string
		config   ProcessMetricsConfig
		expected ProcessMetricsConfig
	}{
		{
			name: "default values",
			config: ProcessMetricsConfig{
				Enabled: true,
			},
			expected: ProcessMetricsConfig{
				Enabled:    true,
				Interval:   5 * time.Second,
				MaxHistory: 100,
			},
		},
		{
			name: "custom values",
			config: ProcessMetricsConfig{
				Enabled:    true,
				Interval:   10 * time.Second,
				MaxHistory: 50,
			},
			expected: ProcessMetricsConfig{
				Enabled:    true,
				Interval:   10 * time.Second,
				MaxHistory: 50,
			},
		},
		{
			name: "history_size alias",
			config: ProcessMetricsConfig{
				Enabled:     true,
				HistorySize: 200,
			},
			expected: ProcessMetricsConfig{
				Enabled:    true,
				Interval:   5 * time.Second,
				MaxHistory: 200,
			},
		},
		{
			name: "disabled collector",
			config: ProcessMetricsConfig{
				Enabled: false,
			},
			expected: ProcessMetricsConfig{
				Enabled:    false,
				Interval:   5 * time.Second,
				MaxHistory: 100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewProcessMetricsCollector(tt.config)
			assert.NotNil(t, collector)
			assert.Equal(t, tt.expected.Enabled, collector.enabled)
			assert.Equal(t, tt.expected.Interval, collector.interval)
			assert.Equal(t, tt.expected.MaxHistory, collector.maxHistory)
			assert.NotNil(t, collector.history)
			assert.NotNil(t, collector.stopCh)
		})
	}
}

func TestProcessMetricsCollectorRegisterMetrics(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expectOK bool
	}{
		{
			name:     "enabled collector",
			enabled:  true,
			expectOK: true,
		},
		{
			name:     "disabled collector",
			enabled:  false,
			expectOK: true, // Should not error even when disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ProcessMetricsConfig{
				Enabled:    tt.enabled,
				Interval:   time.Second,
				MaxHistory: 10,
			}
			collector := NewProcessMetricsCollector(config)
			registry := prometheus.NewRegistry()

			err := collector.RegisterMetrics(registry)
			if tt.expectOK {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			// Test idempotent registration
			err = collector.RegisterMetrics(registry)
			assert.NoError(t, err) // Should not error on second registration
		})
	}
}

func TestProcessMetricsCollectorStartStop(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   100 * time.Millisecond,
		MaxHistory: 10,
	}
	collector := NewProcessMetricsCollector(config)

	// Mock process provider
	processes := map[string]int32{
		"test-proc-1": 1234,
		"test-proc-2": 5678,
	}
	getProcesses := func() map[string]int32 {
		return processes
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start collection
	err := collector.Start(ctx, getProcesses)
	assert.NoError(t, err)

	// Wait a bit for collection to happen
	time.Sleep(200 * time.Millisecond)

	// Stop collection
	collector.Stop()

	// Verify it can be stopped multiple times
	collector.Stop()
}

func TestProcessMetricsCollectorDisabled(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled: false,
	}
	collector := NewProcessMetricsCollector(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should be no-op for disabled collector
	err := collector.Start(ctx, func() map[string]int32 { return nil })
	assert.NoError(t, err)

	// Stop should be no-op
	collector.Stop()

	// Methods should return empty/false for disabled collector
	assert.False(t, collector.IsEnabled())

	metrics, found := collector.GetMetrics("test")
	assert.False(t, found)
	assert.Equal(t, ProcessMetrics{}, metrics)

	history, found := collector.GetHistory("test")
	assert.False(t, found)
	assert.Nil(t, history)

	allMetrics := collector.GetAllMetrics()
	assert.Empty(t, allMetrics)
}

func TestProcessMetricsHistory(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		MaxHistory: 3, // Small history for testing
	}
	collector := NewProcessMetricsCollector(config)

	// Add metrics to history
	for i := 0; i < 5; i++ {
		metrics := ProcessMetrics{
			PID:        int32(1000 + i),
			Name:       "test-proc",
			CPUPercent: float64(i * 10),
			MemoryMB:   float64(i * 100),
			Timestamp:  time.Now().Add(time.Duration(i) * time.Second),
		}
		collector.addToHistory("test-proc", metrics)
	}

	// Should only keep the last 3 entries
	history, found := collector.GetHistory("test-proc")
	assert.True(t, found)
	assert.Len(t, history, 3)

	// Check that the oldest entries were removed (should have the last 3)
	assert.Equal(t, float64(20), history[0].CPUPercent) // index 2
	assert.Equal(t, float64(30), history[1].CPUPercent) // index 3
	assert.Equal(t, float64(40), history[2].CPUPercent) // index 4
}

func TestProcessMetricsCleanup(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		MaxHistory: 10,
	}
	collector := NewProcessMetricsCollector(config)

	// Add some metrics
	metrics1 := ProcessMetrics{
		PID:  1234,
		Name: "proc1",
	}
	metrics2 := ProcessMetrics{
		PID:  5678,
		Name: "proc2",
	}
	collector.addToHistory("proc1", metrics1)
	collector.addToHistory("proc2", metrics2)

	// Verify both exist
	_, found1 := collector.GetMetrics("proc1")
	_, found2 := collector.GetMetrics("proc2")
	assert.True(t, found1)
	assert.True(t, found2)

	// Cleanup with only proc1 active
	activeProcesses := map[string]int32{
		"proc1": 1234,
	}
	collector.cleanupMetrics(activeProcesses)

	// proc1 should still exist, proc2 should be removed
	_, found1 = collector.GetMetrics("proc1")
	_, found2 = collector.GetMetrics("proc2")
	assert.True(t, found1)
	assert.False(t, found2)
}

func TestProcessMetricsSetEnabled(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled: true,
	}
	collector := NewProcessMetricsCollector(config)

	assert.True(t, collector.IsEnabled())

	collector.SetEnabled(false)
	assert.False(t, collector.IsEnabled())

	collector.SetEnabled(true)
	assert.True(t, collector.IsEnabled())
}

func TestProcessMetricsConcurrentAccess(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		MaxHistory: 100,
	}
	collector := NewProcessMetricsCollector(config)

	// Test concurrent access to metrics
	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 20

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				metrics := ProcessMetrics{
					PID:        int32(1000 + id),
					Name:       fmt.Sprintf("proc-%d", id),
					CPUPercent: float64(j),
					MemoryMB:   float64(j * 10),
					Timestamp:  time.Now(),
				}
				collector.addToHistory(fmt.Sprintf("proc-%d", id), metrics)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				procName := fmt.Sprintf("proc-%d", id%10) // Read from various processes
				collector.GetMetrics(procName)
				collector.GetHistory(procName)
				collector.GetAllMetrics()
			}
		}(i)
	}

	// Concurrent cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			activeProcesses := make(map[string]int32)
			for j := 0; j < 25; j++ { // Keep half the processes active
				activeProcesses[fmt.Sprintf("proc-%d", j)] = int32(1000 + j)
			}
			collector.cleanupMetrics(activeProcesses)
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestProcessMetricsHistoryConcurrency(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		MaxHistory: 10,
	}
	collector := NewProcessMetricsCollector(config)

	var wg sync.WaitGroup
	processName := "test-proc"

	// Concurrent writes to the same process history
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			metrics := ProcessMetrics{
				PID:        int32(1000 + id),
				Name:       processName,
				CPUPercent: float64(id),
				MemoryMB:   float64(id * 10),
				Timestamp:  time.Now(),
			}
			collector.addToHistory(processName, metrics)
		}(i)
	}

	// Concurrent reads from the same process history
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				collector.GetHistory(processName)
				collector.GetMetrics(processName)
			}
		}()
	}

	wg.Wait()

	// Verify final state
	history, found := collector.GetHistory(processName)
	assert.True(t, found)
	assert.LessOrEqual(t, len(history), 10) // Should not exceed max history
}

func TestProcessMetricsGetters(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		MaxHistory: 5,
	}
	collector := NewProcessMetricsCollector(config)

	// Add some test data
	metrics1 := ProcessMetrics{
		PID:        1234,
		Name:       "proc1",
		CPUPercent: 10.5,
		MemoryMB:   128.0,
		Timestamp:  time.Now(),
	}
	metrics2 := ProcessMetrics{
		PID:        5678,
		Name:       "proc2",
		CPUPercent: 20.0,
		MemoryMB:   256.0,
		Timestamp:  time.Now(),
	}

	collector.addToHistory("proc1", metrics1)
	collector.addToHistory("proc2", metrics2)

	// Test GetMetrics
	latest, found := collector.GetMetrics("proc1")
	assert.True(t, found)
	assert.Equal(t, metrics1.PID, latest.PID)
	assert.Equal(t, metrics1.CPUPercent, latest.CPUPercent)

	// Test GetMetrics for non-existent process
	_, found = collector.GetMetrics("nonexistent")
	assert.False(t, found)

	// Test GetAllMetrics
	allMetrics := collector.GetAllMetrics()
	assert.Len(t, allMetrics, 2)
	assert.Contains(t, allMetrics, "proc1")
	assert.Contains(t, allMetrics, "proc2")
	assert.Equal(t, metrics1.CPUPercent, allMetrics["proc1"].CPUPercent)
	assert.Equal(t, metrics2.CPUPercent, allMetrics["proc2"].CPUPercent)

	// Test GetHistory
	history, found := collector.GetHistory("proc1")
	assert.True(t, found)
	assert.Len(t, history, 1)
	assert.Equal(t, metrics1.PID, history[0].PID)

	// Test GetHistory for non-existent process
	_, found = collector.GetHistory("nonexistent")
	assert.False(t, found)
}

func TestRegisterWithProcessMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()

	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 10,
	}

	err := RegisterWithProcessMetrics(registry, config)
	assert.NoError(t, err)

	// Test idempotent registration
	err = RegisterWithProcessMetrics(registry, config)
	assert.NoError(t, err)

	// Verify that GetProcessMetricsCollector returns non-nil
	collector := GetProcessMetricsCollector()
	assert.NotNil(t, collector)
	assert.True(t, collector.IsEnabled())
}

func TestProcessMetricsCollectorEdgeCases(t *testing.T) {
	t.Run("empty process list", func(t *testing.T) {
		config := ProcessMetricsConfig{
			Enabled:    true,
			Interval:   10 * time.Millisecond,
			MaxHistory: 10,
		}
		collector := NewProcessMetricsCollector(config)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := collector.Start(ctx, func() map[string]int32 {
			return map[string]int32{} // Empty process list
		})
		assert.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
		collector.Stop()

		allMetrics := collector.GetAllMetrics()
		assert.Empty(t, allMetrics)
	})

	t.Run("process with zero PID", func(t *testing.T) {
		config := ProcessMetricsConfig{
			Enabled:    true,
			Interval:   10 * time.Millisecond,
			MaxHistory: 10,
		}
		collector := NewProcessMetricsCollector(config)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := collector.Start(ctx, func() map[string]int32 {
			return map[string]int32{
				"invalid-proc": 0, // Zero PID should be ignored
				"valid-proc":   1234,
			}
		})
		assert.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
		collector.Stop()

		// Should not have metrics for invalid-proc
		_, found := collector.GetMetrics("invalid-proc")
		assert.False(t, found)
	})

	t.Run("context cancellation", func(t *testing.T) {
		config := ProcessMetricsConfig{
			Enabled:    true,
			Interval:   10 * time.Millisecond,
			MaxHistory: 10,
		}
		collector := NewProcessMetricsCollector(config)

		ctx, cancel := context.WithCancel(context.Background())

		err := collector.Start(ctx, func() map[string]int32 {
			return map[string]int32{"test-proc": 1234}
		})
		assert.NoError(t, err)

		// Cancel context to stop collection
		cancel()

		// Give it time to process the cancellation
		time.Sleep(50 * time.Millisecond)

		// Collection should have stopped gracefully
	})
}

func TestProcessMetricsCollectorMaxHistoryZero(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		MaxHistory: 0, // Should default to HistorySize or 100
	}
	collector := NewProcessMetricsCollector(config)

	assert.Equal(t, 100, collector.maxHistory) // Should use default
}

func TestProcessMetricsHistoryMaxSize(t *testing.T) {
	history := &ProcessMetricsHistory{
		ProcessName: "test",
		Metrics:     make([]ProcessMetrics, 0),
		MaxSize:     2,
	}

	// Add metrics beyond max size
	for i := 0; i < 5; i++ {
		metrics := ProcessMetrics{
			PID:        int32(1000 + i),
			Name:       "test",
			CPUPercent: float64(i * 10),
			Timestamp:  time.Now(),
		}

		history.mu.Lock()
		history.Metrics = append(history.Metrics, metrics)
		if len(history.Metrics) > history.MaxSize {
			copy(history.Metrics, history.Metrics[len(history.Metrics)-history.MaxSize:])
			history.Metrics = history.Metrics[:history.MaxSize]
		}
		history.mu.Unlock()
	}

	history.mu.RLock()
	assert.Len(t, history.Metrics, 2)
	// Should contain the last 2 entries (index 3 and 4)
	assert.Equal(t, float64(30), history.Metrics[0].CPUPercent)
	assert.Equal(t, float64(40), history.Metrics[1].CPUPercent)
	history.mu.RUnlock()
}
