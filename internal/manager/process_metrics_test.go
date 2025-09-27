package manager

import (
	"fmt"
	"testing"
	"time"

	"github.com/loykin/provisr/internal/metrics"
	"github.com/loykin/provisr/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerProcessMetrics(t *testing.T) {
	mgr := NewManager()

	t.Run("SetProcessMetricsCollector with enabled collector", func(t *testing.T) {
		config := metrics.ProcessMetricsConfig{
			Enabled:    true,
			Interval:   100 * time.Millisecond,
			MaxHistory: 10,
		}
		collector := metrics.NewProcessMetricsCollector(config)

		err := mgr.SetProcessMetricsCollector(collector)
		assert.NoError(t, err)

		assert.True(t, mgr.IsProcessMetricsEnabled())
	})

	t.Run("SetProcessMetricsCollector with disabled collector", func(t *testing.T) {
		config := metrics.ProcessMetricsConfig{
			Enabled:    false,
			Interval:   time.Second,
			MaxHistory: 10,
		}
		collector := metrics.NewProcessMetricsCollector(config)

		err := mgr.SetProcessMetricsCollector(collector)
		assert.NoError(t, err)

		assert.False(t, mgr.IsProcessMetricsEnabled())
	})

	t.Run("SetProcessMetricsCollector with nil", func(t *testing.T) {
		err := mgr.SetProcessMetricsCollector(nil)
		assert.NoError(t, err)

		assert.False(t, mgr.IsProcessMetricsEnabled())
	})
}

func TestManagerProcessMetricsGetters(t *testing.T) {
	mgr := NewManager()

	t.Run("no collector set", func(t *testing.T) {
		processMetrics, found := mgr.GetProcessMetrics("test")
		assert.False(t, found)
		assert.Equal(t, metrics.ProcessMetrics{}, processMetrics)

		history, found := mgr.GetProcessMetricsHistory("test")
		assert.False(t, found)
		assert.Nil(t, history)

		allMetrics := mgr.GetAllProcessMetrics()
		assert.Empty(t, allMetrics)

		assert.False(t, mgr.IsProcessMetricsEnabled())
	})

	t.Run("with collector set", func(t *testing.T) {
		config := metrics.ProcessMetricsConfig{
			Enabled:    true,
			Interval:   time.Second,
			MaxHistory: 10,
		}
		collector := metrics.NewProcessMetricsCollector(config)
		err := mgr.SetProcessMetricsCollector(collector)
		require.NoError(t, err)

		// Initially empty
		_, found := mgr.GetProcessMetrics("test")
		assert.False(t, found)

		_, found = mgr.GetProcessMetricsHistory("test")
		assert.False(t, found)

		allMetrics := mgr.GetAllProcessMetrics()
		assert.Empty(t, allMetrics)

		assert.True(t, mgr.IsProcessMetricsEnabled())
	})
}

func TestManagerGetProcessPIDs(t *testing.T) {
	mgr := NewManager()

	t.Run("empty manager", func(t *testing.T) {
		pids := mgr.getProcessPIDs()
		assert.Empty(t, pids)
	})

	t.Run("with registered processes", func(t *testing.T) {
		// Register some test processes
		spec1 := process.Spec{
			Name:    "test-proc-1",
			Command: "echo test1",
		}
		spec2 := process.Spec{
			Name:    "test-proc-2",
			Command: "echo test2",
		}

		err := mgr.RegisterN(spec1)
		require.NoError(t, err)
		err = mgr.RegisterN(spec2)
		require.NoError(t, err)

		// At this point processes may or may not be running
		// depending on the command execution, so we just test
		// that the method doesn't panic and returns a map
		pids := mgr.getProcessPIDs()
		assert.NotNil(t, pids)
		// The actual content depends on whether the echo commands
		// are still running, which is unpredictable in tests
	})
}

func TestManagerShutdownWithMetrics(t *testing.T) {
	mgr := NewManager()

	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   50 * time.Millisecond,
		MaxHistory: 10,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	// Start metrics collection
	err = collector.Start(mgr.metricsCtx, mgr.getProcessPIDs)
	require.NoError(t, err)

	// Wait a bit for metrics collection to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown should stop metrics collection gracefully
	err = mgr.Shutdown()
	assert.NoError(t, err)

	// Verify shutdown is idempotent
	err = mgr.Shutdown()
	assert.NoError(t, err)
}

func TestManagerProcessMetricsConcurrency(t *testing.T) {
	mgr := NewManager()

	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   10 * time.Millisecond,
		MaxHistory: 5,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	// Test concurrent access to metrics methods
	done := make(chan bool)

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				mgr.GetProcessMetrics("test")
				mgr.GetProcessMetricsHistory("test")
				mgr.GetAllProcessMetrics()
				mgr.IsProcessMetricsEnabled()
				mgr.getProcessPIDs()
			}
			done <- true
		}()
	}

	// Concurrent process operations
	go func() {
		for i := 0; i < 5; i++ {
			spec := process.Spec{
				Name:    fmt.Sprintf("concurrent-test-%d", i),
				Command: "echo concurrent",
			}
			_ = mgr.RegisterN(spec)
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 6; i++ {
		<-done
	}

	// Cleanup
	_ = mgr.Shutdown()
}

func TestManagerProcessMetricsIntegration(t *testing.T) {
	mgr := NewManager()

	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   50 * time.Millisecond,
		MaxHistory: 10,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	// Start metrics collection
	err = collector.Start(mgr.metricsCtx, mgr.getProcessPIDs)
	require.NoError(t, err)

	// Register a simple process
	spec := process.Spec{
		Name:    "metrics-test",
		Command: "sleep 0.5", // Short-lived process
	}
	err = mgr.RegisterN(spec)
	require.NoError(t, err)

	// Wait for the process to start and for metrics to be collected
	time.Sleep(150 * time.Millisecond)

	// Check if we can get process PIDs (process might have finished)
	pids := mgr.getProcessPIDs()
	assert.NotNil(t, pids)

	// Test metrics retrieval
	allMetrics := mgr.GetAllProcessMetrics()
	assert.NotNil(t, allMetrics)

	// Cleanup
	_ = mgr.Shutdown()
}

func TestManagerProcessMetricsMultipleInstances(t *testing.T) {
	mgr := NewManager()

	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   50 * time.Millisecond,
		MaxHistory: 10,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	// Register a process with multiple instances
	spec := process.Spec{
		Name:      "multi-test",
		Command:   "sleep 0.5",
		Instances: 3,
	}
	err = mgr.RegisterN(spec)
	require.NoError(t, err)

	// Wait for processes to start
	time.Sleep(100 * time.Millisecond)

	// Get PIDs - should have entries for multi-test-1, multi-test-2, multi-test-3
	pids := mgr.getProcessPIDs()
	assert.NotNil(t, pids)

	// The processes might have finished already due to short sleep,
	// but the manager should handle this gracefully
	allMetrics := mgr.GetAllProcessMetrics()
	assert.NotNil(t, allMetrics)

	// Cleanup
	_ = mgr.Shutdown()
}

func TestManagerProcessMetricsCollectorNil(t *testing.T) {
	mgr := NewManager()

	// Test methods when no collector is set
	assert.False(t, mgr.IsProcessMetricsEnabled())

	processMetrics, found := mgr.GetProcessMetrics("test")
	assert.False(t, found)
	assert.Equal(t, metrics.ProcessMetrics{}, processMetrics)

	processHistory, found := mgr.GetProcessMetricsHistory("test")
	assert.False(t, found)
	assert.Nil(t, processHistory)

	allMetrics := mgr.GetAllProcessMetrics()
	assert.Empty(t, allMetrics)

	// These should not panic
	_ = mgr.Shutdown()
}
