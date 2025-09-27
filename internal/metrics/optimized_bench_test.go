package metrics

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// BenchmarkOptimizedHistoryOperations tests the new circular buffer approach
func BenchmarkOptimizedHistoryOperations(b *testing.B) {
	tests := []struct {
		name       string
		maxHistory int
		entries    int
	}{
		{"optimized_small_history_100_200entries", 100, 200},
		{"optimized_medium_history_1000_2000entries", 1000, 2000},
		{"optimized_large_history_10000_20000entries", 10000, 20000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			config := ProcessMetricsConfig{
				Enabled:    true,
				Interval:   time.Second,
				MaxHistory: tt.maxHistory,
			}
			collector := NewProcessMetricsCollector(config)

			metric := ProcessMetrics{
				PID:        1234,
				Name:       "test",
				CPUPercent: 50.0,
				MemoryMB:   128.0,
				Timestamp:  time.Now(),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := 0; j < tt.entries; j++ {
					collector.AddToHistoryForTesting("test-proc", metric)
				}
			}
		})
	}
}

// BenchmarkCircularBufferVsSliceCopy compares performance
func BenchmarkCircularBufferVsSliceCopy(b *testing.B) {
	const maxHistory = 1000
	const entries = 2000

	metric := ProcessMetrics{
		PID:        1234,
		Name:       "test",
		CPUPercent: 50.0,
		MemoryMB:   128.0,
		Timestamp:  time.Now(),
	}

	b.Run("circular_buffer_approach", func(b *testing.B) {
		config := ProcessMetricsConfig{
			Enabled:    true,
			Interval:   time.Second,
			MaxHistory: maxHistory,
		}
		collector := NewProcessMetricsCollector(config)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < entries; j++ {
				collector.AddToHistoryForTesting("test-proc", metric)
			}
		}
	})

	b.Run("old_slice_copy_approach", func(b *testing.B) {
		// Simulate the old approach
		history := &ProcessMetricsHistory{
			ProcessName: "test",
			Metrics:     make([]ProcessMetrics, 0, maxHistory),
			MaxSize:     maxHistory,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < entries; j++ {
				history.mu.Lock()
				history.Metrics = append(history.Metrics, metric)
				if len(history.Metrics) > history.MaxSize {
					// Old inefficient approach
					copy(history.Metrics, history.Metrics[len(history.Metrics)-history.MaxSize:])
					history.Metrics = history.Metrics[:history.MaxSize]
				}
				history.mu.Unlock()
			}
		}
	})
}

// BenchmarkOptimizedBatchCollection tests the batched metrics collection
func BenchmarkOptimizedBatchCollection(b *testing.B) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 100,
	}
	collector := NewProcessMetricsCollector(config)

	// Create test processes
	processes := make(map[string]int32)
	for i := 0; i < 50; i++ {
		processes[fmt.Sprintf("proc-%d", i)] = int32(os.Getpid())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.collectMetrics(processes)
	}
}

// BenchmarkGetOperationsOptimized tests read performance with circular buffer
func BenchmarkGetOperationsOptimized(b *testing.B) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 1000,
	}
	collector := NewProcessMetricsCollector(config)

	// Fill with test data
	metric := ProcessMetrics{
		PID:        1234,
		Name:       "test",
		CPUPercent: 50.0,
		MemoryMB:   128.0,
		Timestamp:  time.Now(),
	}

	for i := 0; i < 10; i++ {
		for j := 0; j < 1500; j++ { // Fill beyond max to test circular buffer
			collector.AddToHistoryForTesting(fmt.Sprintf("proc-%d", i), metric)
		}
	}

	b.Run("GetMetrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.GetMetrics("proc-5")
		}
	})

	b.Run("GetHistory", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.GetHistory("proc-5")
		}
	})

	b.Run("GetAllMetrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.GetAllMetrics()
		}
	})
}

// TestCircularBufferCorrectness verifies that the circular buffer works correctly
func TestCircularBufferCorrectness(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 3, // Small buffer for testing
	}
	collector := NewProcessMetricsCollector(config)

	// Add metrics one by one and verify
	timestamps := []time.Time{
		time.Unix(1000, 0),
		time.Unix(2000, 0),
		time.Unix(3000, 0),
		time.Unix(4000, 0), // This should overwrite the first one
		time.Unix(5000, 0), // This should overwrite the second one
	}

	for i, ts := range timestamps {
		metric := ProcessMetrics{
			PID:        int32(1000 + i),
			Name:       "test",
			CPUPercent: float64(i * 10),
			MemoryMB:   float64(i * 20),
			Timestamp:  ts,
		}
		collector.AddToHistoryForTesting("test-proc", metric)
	}

	// Get latest metrics
	latest, found := collector.GetMetrics("test-proc")
	if !found {
		t.Fatal("Expected to find metrics")
	}
	if latest.Timestamp != timestamps[4] {
		t.Errorf("Expected latest timestamp %v, got %v", timestamps[4], latest.Timestamp)
	}

	// Get history - should contain last 3 entries in chronological order
	history, found := collector.GetHistory("test-proc")
	if !found {
		t.Fatal("Expected to find history")
	}
	if len(history) != 3 {
		t.Errorf("Expected history length 3, got %d", len(history))
	}

	// Verify chronological order (should be timestamps[2], timestamps[3], timestamps[4])
	expectedTimestamps := []time.Time{timestamps[2], timestamps[3], timestamps[4]}
	for i, expected := range expectedTimestamps {
		if history[i].Timestamp != expected {
			t.Errorf("History[%d]: expected timestamp %v, got %v", i, expected, history[i].Timestamp)
		}
	}
}

// TestCircularBufferEdgeCases tests edge cases for the circular buffer
func TestCircularBufferEdgeCases(t *testing.T) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 5,
	}
	collector := NewProcessMetricsCollector(config)

	// Test empty buffer
	_, found := collector.GetMetrics("non-existent")
	if found {
		t.Error("Expected not to find metrics for non-existent process")
	}

	_, found = collector.GetHistory("non-existent")
	if found {
		t.Error("Expected not to find history for non-existent process")
	}

	// Test single entry
	metric := ProcessMetrics{
		PID:        1234,
		Name:       "test",
		CPUPercent: 50.0,
		MemoryMB:   128.0,
		Timestamp:  time.Unix(1000, 0),
	}
	collector.AddToHistoryForTesting("test-proc", metric)

	latest, found := collector.GetMetrics("test-proc")
	if !found {
		t.Fatal("Expected to find metrics")
	}
	if latest.CPUPercent != 50.0 {
		t.Errorf("Expected CPU 50.0, got %f", latest.CPUPercent)
	}

	history, found := collector.GetHistory("test-proc")
	if !found {
		t.Fatal("Expected to find history")
	}
	if len(history) != 1 {
		t.Errorf("Expected history length 1, got %d", len(history))
	}
}
