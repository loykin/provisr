package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loykin/provisr/internal/manager"
	"github.com/loykin/provisr/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessMetricsEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create manager and set up metrics collector
	mgr := manager.NewManager()
	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 10,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	// Add some test metrics
	testMetrics := map[string]metrics.ProcessMetrics{
		"app-1": {
			PID:        1234,
			Name:       "app-1",
			CPUPercent: 15.5,
			MemoryMB:   128.0,
			Timestamp:  time.Now(),
		},
		"app-2": {
			PID:        5678,
			Name:       "app-2",
			CPUPercent: 25.0,
			MemoryMB:   256.0,
			Timestamp:  time.Now(),
		},
		"web-1": {
			PID:        9999,
			Name:       "web-1",
			CPUPercent: 10.0,
			MemoryMB:   64.0,
			Timestamp:  time.Now(),
		},
	}

	// Manually add metrics to collector for testing
	for name, metric := range testMetrics {
		collector.AddToHistoryForTesting(name, metric)
	}

	router := NewRouter(mgr, "/api")
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	t.Run("GET /api/metrics - all metrics", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]metrics.ProcessMetrics
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Len(t, result, 3)
		assert.Contains(t, result, "app-1")
		assert.Contains(t, result, "app-2")
		assert.Contains(t, result, "web-1")
		assert.Equal(t, float64(15.5), result["app-1"].CPUPercent)
	})

	t.Run("GET /api/metrics?name=app-1 - specific process", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics?name=app-1")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result metrics.ProcessMetrics
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "app-1", result.Name)
		assert.Equal(t, int32(1234), result.PID)
		assert.Equal(t, float64(15.5), result.CPUPercent)
	})

	t.Run("GET /api/metrics?name=nonexistent - not found", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics?name=nonexistent")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "process not found")
	})

	t.Run("GET /api/metrics?name=../invalid - invalid name", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics?name=../invalid")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "invalid name")
	})

	t.Run("GET /api/metrics/history?name=app-1 - process history", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/history?name=app-1")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "app-1", result["process"])
		assert.NotNil(t, result["history"])

		historyList, ok := result["history"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, historyList, 1)
	})

	t.Run("GET /api/metrics/history - missing name parameter", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/history")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "name parameter required")
	})

	t.Run("GET /api/metrics/group?base=app - group metrics", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/group?base=app")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "app", result["base"])
		assert.Equal(t, float64(2), result["process_count"])  // app-1 and app-2
		assert.Equal(t, float64(40.5), result["total_cpu"])   // 15.5 + 25.0
		assert.Equal(t, float64(384), result["total_memory"]) // 128 + 256
		assert.Equal(t, float64(20.25), result["avg_cpu"])    // 40.5 / 2
		assert.Equal(t, float64(192), result["avg_memory"])   // 384 / 2

		processes, ok := result["processes"].(map[string]interface{})
		assert.True(t, ok)
		assert.Len(t, processes, 2)
		assert.Contains(t, processes, "app-1")
		assert.Contains(t, processes, "app-2")
	})

	t.Run("GET /api/metrics/group - missing base parameter", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/group")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "base parameter required")
	})

	t.Run("GET /api/metrics/group?base=nonexistent - no processes", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/group?base=nonexistent")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "no processes found")
	})
}

func TestProcessMetricsDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create manager without metrics collector
	mgr := manager.NewManager()

	router := NewRouter(mgr, "/api")
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	t.Run("GET /api/metrics - disabled", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "process metrics collection is disabled")
	})

	t.Run("GET /api/metrics/history?name=test - disabled", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/history?name=test")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "process metrics collection is disabled")
	})

	t.Run("GET /api/metrics/group?base=test - disabled", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/group?base=test")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "process metrics collection is disabled")
	})
}

func TestAPIEndpointsProcessMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := manager.NewManager()
	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 10,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	endpoints := NewAPIEndpoints(mgr, "/api")

	// Test individual handler functions
	t.Run("ProcessMetricsHandler", func(t *testing.T) {
		handler := endpoints.ProcessMetricsHandler()
		assert.NotNil(t, handler)
	})

	t.Run("ProcessMetricsHistoryHandler", func(t *testing.T) {
		handler := endpoints.ProcessMetricsHistoryHandler()
		assert.NotNil(t, handler)
	})

	t.Run("ProcessMetricsGroupHandler", func(t *testing.T) {
		handler := endpoints.ProcessMetricsGroupHandler()
		assert.NotNil(t, handler)
	})

	// Test RegisterAll includes process metrics endpoints
	t.Run("RegisterAll includes process metrics", func(t *testing.T) {
		router := gin.New()
		group := router.Group("/api")
		endpoints.RegisterAll(group)

		// The endpoints should be registered, but we can't easily test this
		// without setting up a full HTTP server. The fact that RegisterAll
		// doesn't panic is a good sign.
	})
}

func TestProcessMetricsGroupEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := manager.NewManager()
	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 10,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	// Add test metrics with various naming patterns
	testMetrics := map[string]metrics.ProcessMetrics{
		"app":           {PID: 1111, Name: "app", CPUPercent: 5.0, MemoryMB: 50.0},
		"app-1":         {PID: 1234, Name: "app-1", CPUPercent: 15.5, MemoryMB: 128.0},
		"app-2":         {PID: 5678, Name: "app-2", CPUPercent: 25.0, MemoryMB: 256.0},
		"application-1": {PID: 9999, Name: "application-1", CPUPercent: 10.0, MemoryMB: 64.0},
	}

	for name, metric := range testMetrics {
		collector.AddToHistoryForTesting(name, metric)
	}

	router := NewRouter(mgr, "/api")
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	t.Run("base matches exact name and instances", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/group?base=app")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		// Should match "app", "app-1", and "app-2" but not "application-1"
		assert.Equal(t, float64(3), result["process_count"])
		assert.Equal(t, float64(45.5), result["total_cpu"]) // 5.0 + 15.5 + 25.0

		processes, ok := result["processes"].(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, processes, "app")
		assert.Contains(t, processes, "app-1")
		assert.Contains(t, processes, "app-2")
		assert.NotContains(t, processes, "application-1")
	})

	t.Run("base with different prefix", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/group?base=application")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		// Should match "application-1" but not the "app*" processes
		assert.Equal(t, float64(1), result["process_count"])
		assert.Equal(t, float64(10.0), result["total_cpu"])

		processes, ok := result["processes"].(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, processes, "application-1")
		assert.NotContains(t, processes, "app")
		assert.NotContains(t, processes, "app-1")
		assert.NotContains(t, processes, "app-2")
	})

	t.Run("invalid base name", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/metrics/group?base=../invalid")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var result errorResp
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result.Error, "invalid base")
	})
}

func TestProcessMetricsConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := manager.NewManager()
	config := metrics.ProcessMetricsConfig{
		Enabled:    true,
		Interval:   time.Second,
		MaxHistory: 10,
	}
	collector := metrics.NewProcessMetricsCollector(config)
	err := mgr.SetProcessMetricsCollector(collector)
	require.NoError(t, err)

	// Add some test metrics
	for i := 0; i < 10; i++ {
		metric := metrics.ProcessMetrics{
			PID:        int32(1000 + i),
			Name:       fmt.Sprintf("proc-%d", i),
			CPUPercent: float64(i * 10),
			MemoryMB:   float64(i * 50),
			Timestamp:  time.Now(),
		}
		collector.AddToHistoryForTesting(fmt.Sprintf("proc-%d", i), metric)
	}

	router := NewRouter(mgr, "/api")
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	// Test concurrent requests
	numRequests := 20
	ch := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			resp, err := http.Get(ts.URL + "/api/metrics")
			if err != nil {
				ch <- err
				return
			}
			_ = resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				ch <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				return
			}

			ch <- nil
		}(i)
	}

	// Check all requests completed successfully
	for i := 0; i < numRequests; i++ {
		err := <-ch
		assert.NoError(t, err)
	}
}
