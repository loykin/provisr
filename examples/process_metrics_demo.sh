#!/bin/bash

# Process Metrics Monitoring Demo Script
# This script demonstrates the CPU and memory monitoring capabilities of provisr

set -e

echo "=== Provisr Process Metrics Monitoring Demo ==="
echo

# Build provisr if needed
if [ ! -f "./provisr" ]; then
    echo "Building provisr..."
    go build -o provisr ./cmd/provisr
fi

CONFIG_FILE="config/process_metrics_demo.toml"

echo "Starting provisr daemon with process metrics monitoring enabled..."
echo "Config file: $CONFIG_FILE"
echo

# Start the daemon in the background
./provisr serve "$CONFIG_FILE" &
DAEMON_PID=$!

# Function to cleanup
cleanup() {
    echo
    echo "Stopping daemon..."
    kill $DAEMON_PID 2>/dev/null || true
    wait $DAEMON_PID 2>/dev/null || true
    echo "Demo completed."
}

# Set up cleanup on exit
trap cleanup EXIT

# Wait for daemon to start
echo "Waiting for daemon to start..."
sleep 3

echo "=== Testing Process Metrics API Endpoints ==="
echo

# Test getting all process metrics
echo "1. Getting all process metrics:"
curl -s "http://localhost:8080/api/metrics" | jq '.' || echo "No metrics available yet or jq not installed"
echo

# Wait a bit for metrics to be collected
echo "Waiting 5 seconds for metrics collection..."
sleep 5

echo "2. Getting all process metrics (after collection):"
curl -s "http://localhost:8080/api/metrics" | jq '.' || curl -s "http://localhost:8080/api/metrics"
echo

# Test getting specific process metrics
echo "3. Getting metrics for demo-app process:"
curl -s "http://localhost:8080/api/metrics?name=demo-app" | jq '.' || curl -s "http://localhost:8080/api/metrics?name=demo-app"
echo

# Wait for more metrics collection
echo "Waiting 10 more seconds for metrics history..."
sleep 10

# Test getting process metrics history
echo "4. Getting metrics history for demo-app-1 process:"
curl -s "http://localhost:8080/api/metrics/history?name=demo-app-1" | jq '.' || curl -s "http://localhost:8080/api/metrics/history?name=demo-app-1"
echo

# Test group metrics (new feature)
echo "5. Getting group metrics for demo-app base (all instances):"
curl -s "http://localhost:8080/api/metrics/group?base=demo-app" | jq '.' || curl -s "http://localhost:8080/api/metrics/group?base=demo-app"
echo

# Test Prometheus metrics endpoint
echo "6. Checking Prometheus metrics endpoint:"
echo "Available process metrics from Prometheus:"
curl -s "http://localhost:9090/metrics" | grep "provisr_process_" | head -10
echo

echo "=== Demo completed! ==="
echo "Available endpoints:"
echo "- All process metrics: http://localhost:8080/api/metrics"
echo "- Specific process metrics: http://localhost:8080/api/metrics?name=demo-app-1"
echo "- Process history: http://localhost:8080/api/metrics/history?name=demo-app-1"
echo "- Group metrics (NEW): http://localhost:8080/api/metrics/group?base=demo-app"
echo "- Prometheus metrics: http://localhost:9090/metrics"
echo "- Process status: http://localhost:8080/api/status"
echo
echo "Press Ctrl+C to stop the demo."

# Keep the demo running until interrupted
wait $DAEMON_PID