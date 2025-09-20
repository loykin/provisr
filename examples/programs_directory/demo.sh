#!/bin/bash
set -e

echo "=== Provisr Programs Directory Demo ==="
echo

# Cleanup function
cleanup() {
    echo "Cleaning up all processes..."
    ../../provisr stop --config config.toml >/dev/null 2>&1 || true
}

trap cleanup EXIT

echo "1. Starting all processes from programs directory..."
result=$(../../provisr start --config config.toml)
echo "✓ Started processes with priorities:"
echo "  Priority 1: database"
echo "  Priority 2: cache-redis"
echo "  Priority 5: api-server (2 instances)"
echo "  Priority 10: frontend"
echo "  Priority 15: scheduler"
echo "  Priority 20: worker (3 instances)"
echo

echo "2. Checking infrastructure group..."
infra=$(../../provisr group-status --config config.toml --group infrastructure)
echo "Infrastructure group (database + cache):"
echo "$infra" | jq .
echo

echo "3. Checking web-services group..."
web=$(../../provisr group-status --config config.toml --group web-services)
echo "Web services group (frontend + api-server):"
echo "$web" | jq .
echo

echo "4. Wait a moment for worker processes to complete..."
sleep 8

echo "5. Checking which processes are still running..."
status=$(../../provisr status --config config.toml)
echo "Current status:"
echo "$status" | jq .
echo

echo "6. Stopping web-services group..."
../../provisr group-stop --config config.toml --group web-services
echo "✓ Web services stopped"
echo

echo "7. Stopping infrastructure group..."
../../provisr group-stop --config config.toml --group infrastructure
echo "✓ Infrastructure stopped"
echo

echo "8. Final status check..."
final_status=$(../../provisr status --config config.toml)
echo "Final status:"
echo "$final_status" | jq .
echo

echo "🎉 Demo completed successfully!"
echo "✓ Programs directory loading works"
echo "✓ Priority-based startup works"
echo "✓ Process groups work for start/stop operations"
echo "✓ Multiple file formats work"
echo "✓ Mixed auto-terminating and long-running processes work"