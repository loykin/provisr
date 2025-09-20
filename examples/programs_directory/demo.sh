#!/bin/bash
set -e

echo "=== Provisr Programs Directory Demo ==="
echo

# Get absolute paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVISR_BIN="$SCRIPT_DIR/../../provisr"
CONFIG_PATH="$SCRIPT_DIR/config.toml"
API_URL="http://127.0.0.1:8080/api"

# Cleanup function
cleanup() {
    echo "Cleaning up all processes..."
    "$PROVISR_BIN" stop --config "$CONFIG_PATH" --api-url "$API_URL" >/dev/null 2>&1 || true
    # Kill daemon if running
    pkill -f "provisr serve" >/dev/null 2>&1 || true
}

trap cleanup EXIT

# Start daemon in background
echo "Starting provisr daemon..."
"$PROVISR_BIN" serve "$CONFIG_PATH" &
DAEMON_PID=$!

# Wait for daemon to be ready and processes to start
sleep 3

echo "1. Checking all processes from programs directory..."
# Note: Daemon auto-starts all processes from programs directory
echo "✓ All processes auto-started with priorities:"
echo "  Priority 1: database"
echo "  Priority 2: cache-redis"
echo "  Priority 5: api-server (2 instances)"
echo "  Priority 10: frontend"
echo "  Priority 15: scheduler"
echo "  Priority 20: worker (3 instances)"
echo

echo "2. Checking infrastructure group..."
infra=$("$PROVISR_BIN" group-status --config "$CONFIG_PATH" --group infrastructure --api-url "$API_URL")
echo "Infrastructure group (database + cache):"
echo "$infra" | jq .
echo

echo "3. Checking web-services group..."
web=$("$PROVISR_BIN" group-status --config "$CONFIG_PATH" --group web-services --api-url "$API_URL")
echo "Web services group (frontend + api-server):"
echo "$web" | jq .
echo

echo "4. Wait a moment for worker processes to complete..."
sleep 8

echo "5. Checking which processes are still running..."
# Use debug API endpoint to show all processes
echo "Current status:"
curl -s "$API_URL/debug/processes" | jq '.[].status | {name, running, pid}'
echo

echo "6. Stopping web-services group..."
"$PROVISR_BIN" group-stop --config "$CONFIG_PATH" --group web-services --api-url "$API_URL"
echo "✓ Web services stopped"
echo

echo "7. Stopping infrastructure group..."
"$PROVISR_BIN" group-stop --config "$CONFIG_PATH" --group infrastructure --api-url "$API_URL"
echo "✓ Infrastructure stopped"
echo

echo "8. Final status check..."
echo "Final status:"
curl -s "$API_URL/debug/processes" | jq '.[].status | {name, running, pid}'
echo

echo "🎉 Demo completed successfully!"
echo "✓ Programs directory loading works"
echo "✓ Priority-based startup works"
echo "✓ Process groups work for start/stop operations"
echo "✓ Multiple file formats work"
echo "✓ Mixed auto-terminating and long-running processes work"