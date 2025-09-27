package metrics

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// BenchmarkProcessMetricsCollector tests performance with various process counts
func BenchmarkProcessMetricsCollector(b *testing.B) {
	benchmarks := []struct {
		name         string
		processCount int
		interval     time.Duration
		maxHistory   int
	}{
		{"10processes_1s_100history", 10, time.Second, 100},
		{"50processes_1s_100history", 50, time.Second, 100},
		{"100processes_1s_100history", 100, time.Second, 100},
		{"10processes_100ms_100history", 10, 100 * time.Millisecond, 100},
		{"10processes_1s_1000history", 10, time.Second, 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			config := ProcessMetricsConfig{
				Enabled:    true,
				Interval:   bm.interval,
				MaxHistory: bm.maxHistory,
			}
			collector := NewProcessMetricsCollector(config)

			// Create mock processes map
			processes := make(map[string]int32)
			for i := 0; i < bm.processCount; i++ {
				processes[fmt.Sprintf("proc-%d", i)] = int32(os.Getpid()) // Use current process PID
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				collector.collectMetrics(processes)
			}
		})
	}
}

// BenchmarkHistorySliceOperations tests the efficiency of history management
func BenchmarkHistorySliceOperations(b *testing.B) {
	tests := []struct {
		name       string
		maxHistory int
		entries    int
	}{
		{"small_history_100_200entries", 100, 200},
		{"medium_history_1000_2000entries", 1000, 2000},
		{"large_history_10000_20000entries", 10000, 20000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			history := &ProcessMetricsHistory{
				ProcessName: "test",
				Metrics:     make([]ProcessMetrics, 0, tt.maxHistory),
				MaxSize:     tt.maxHistory,
			}

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
					// Simulate adding entries to history
					history.mu.Lock()
					history.Metrics = append(history.Metrics, metric)
					if len(history.Metrics) > history.MaxSize {
						// This is the current inefficient approach
						copy(history.Metrics, history.Metrics[len(history.Metrics)-history.MaxSize:])
						history.Metrics = history.Metrics[:history.MaxSize]
					}
					history.mu.Unlock()
				}
			}
		})
	}
}

// BenchmarkConcurrentAccess tests performance under concurrent load
func BenchmarkConcurrentAccess(b *testing.B) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   100 * time.Millisecond,
		MaxHistory: 100,
	}
	collector := NewProcessMetricsCollector(config)

	// Add some initial data
	for i := 0; i < 10; i++ {
		metric := ProcessMetrics{
			PID:        int32(1000 + i),
			Name:       fmt.Sprintf("proc-%d", i),
			CPUPercent: float64(i * 10),
			MemoryMB:   float64(i * 50),
			Timestamp:  time.Now(),
		}
		collector.AddToHistoryForTesting(fmt.Sprintf("proc-%d", i), metric)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Mix of read and write operations
			switch b.N % 4 {
			case 0:
				collector.GetAllMetrics()
			case 1:
				collector.GetMetrics("proc-5")
			case 2:
				collector.GetHistory("proc-3")
			case 3:
				metric := ProcessMetrics{
					PID:        1234,
					Name:       "new-proc",
					CPUPercent: 25.0,
					MemoryMB:   64.0,
					Timestamp:  time.Now(),
				}
				collector.AddToHistoryForTesting("new-proc", metric)
			}
		}
	})
}

// BenchmarkMemoryUsage measures memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 1000,
	}

	b.Run("collector_creation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewProcessMetricsCollector(config)
		}
	})

	b.Run("metric_struct_creation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ProcessMetrics{
				PID:        1234,
				Name:       "test",
				CPUPercent: 50.0,
				MemoryMB:   128.0,
				Timestamp:  time.Now(),
			}
		}
	})

	b.Run("history_operations", func(b *testing.B) {
		collector := NewProcessMetricsCollector(config)
		metric := ProcessMetrics{
			PID:        1234,
			Name:       "test",
			CPUPercent: 50.0,
			MemoryMB:   128.0,
			Timestamp:  time.Now(),
		}

		for i := 0; i < b.N; i++ {
			collector.AddToHistoryForTesting("test-proc", metric)
		}
	})
}

// BenchmarkLockContention measures lock contention under high load
func BenchmarkLockContention(b *testing.B) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 100,
	}
	collector := NewProcessMetricsCollector(config)

	numGoroutines := []int{1, 2, 5, 10, 20}

	for _, numRoutines := range numGoroutines {
		b.Run(fmt.Sprintf("goroutines_%d", numRoutines), func(b *testing.B) {
			var wg sync.WaitGroup

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				wg.Add(numRoutines)
				for j := 0; j < numRoutines; j++ {
					go func(id int) {
						defer wg.Done()

						metric := ProcessMetrics{
							PID:        int32(1000 + id),
							Name:       fmt.Sprintf("proc-%d", id),
							CPUPercent: float64(id * 10),
							MemoryMB:   float64(id * 50),
							Timestamp:  time.Now(),
						}

						// Mix of operations
						collector.AddToHistoryForTesting(fmt.Sprintf("proc-%d", id), metric)
						collector.GetMetrics(fmt.Sprintf("proc-%d", id))
						collector.GetAllMetrics()
					}(j)
				}
				wg.Wait()
			}
		})
	}
}

// BenchmarkGopsutilCalls measures the overhead of gopsutil calls
func BenchmarkGopsutilCalls(b *testing.B) {
	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 100,
	}
	collector := NewProcessMetricsCollector(config)

	pid := int32(os.Getpid())

	b.Run("single_process_metrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = collector.getProcessMetrics("test", pid, time.Now())
		}
	})

	b.Run("multiple_process_metrics", func(b *testing.B) {
		processes := make(map[string]int32)
		for i := 0; i < 10; i++ {
			processes[fmt.Sprintf("proc-%d", i)] = pid
		}

		for i := 0; i < b.N; i++ {
			collector.collectMetrics(processes)
		}
	})
}

// TestMemoryLeakDetection runs a stress test to check for memory leaks
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	config := ProcessMetricsConfig{
		Enabled:    true,
		Interval:   10 * time.Millisecond,
		MaxHistory: 100,
	}
	collector := NewProcessMetricsCollector(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start metrics collection
	processes := func() map[string]int32 {
		result := make(map[string]int32)
		result["test-proc"] = int32(os.Getpid())
		return result
	}

	err := collector.Start(ctx, processes)
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}

	// Let it run for a while
	time.Sleep(2 * time.Second)

	// Stop collector
	collector.Stop()

	// Check if history is properly bounded
	allMetrics := collector.GetAllMetrics()
	if len(allMetrics) > 1 {
		t.Errorf("Expected at most 1 process, got %d", len(allMetrics))
	}

	history, found := collector.GetHistory("test-proc")
	if found && len(history) > config.MaxHistory {
		t.Errorf("History exceeded max size: got %d, max %d", len(history), config.MaxHistory)
	}
}
