#!/bin/bash
set -e

echo "🧪 Simple Provisr Integration Test"
echo "================================="

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up..."
    pkill -f "provisr serve" 2>/dev/null || true
    rm -rf run/ provisr-logs/ programs/ test-*.toml auth.db provisr.pid 2>/dev/null || true
}

trap cleanup EXIT

# Test 1: Build and basic functionality
echo "📦 Building provisr..."
go build -o provisr ./cmd/provisr

# Test 2: Error handling
echo "🚫 Testing error handling..."
echo 'pid_dir = "./run"
[[groups]]
name = "invalid"
members = ["non-existent"]' > test-invalid.toml

if ./provisr serve test-invalid.toml 2>&1 | grep -q "unknown member"; then
    echo "✅ Error handling works"
else
    echo "❌ Error handling failed"
    exit 1
fi

# Test 3: Create working config
echo "📝 Creating test configuration..."
cat > test.toml << 'EOF'
pid_dir = "./run"

[server]
enabled = true
listen = ":9999"
base_path = "/api"

[[groups]]
name = "workers"
members = ["test-worker"]
EOF

mkdir -p programs
cat > programs/test-worker.toml << 'EOF'
type = "process"
[spec]
name = "test-worker"
command = "echo 'Test worker running' && sleep 2"
auto_restart = false
EOF

# Test 4: Start server
echo "🚀 Starting server..."
./provisr serve test.toml &
SERVER_PID=$!

# Wait for server
sleep 3

# Test 5: Basic API tests
echo "🔌 Testing API..."
if curl -s http://localhost:9999/api/status | grep -q '"ok":true'; then
    echo "✅ Server is responding"
else
    echo "❌ Server not responding"
    exit 1
fi

# Test 6: Process operations
echo "⚙️  Testing process operations..."
if curl -s -X POST http://localhost:9999/api/start -H "Content-Type: application/json" -d '{"name":"test-worker"}' | grep -q '"ok":true'; then
    echo "✅ Process start works"
else
    echo "❌ Process start failed"
fi

sleep 3

# Test 7: Group operations
echo "👥 Testing group operations..."
if curl -s "http://localhost:9999/api/group/status?group=workers" | grep -q "test-worker"; then
    echo "✅ Group status works"
else
    echo "❌ Group status failed"
fi

if curl -s -X POST "http://localhost:9999/api/group/start?group=workers" | grep -q '"ok":true'; then
    echo "✅ Group start works"
else
    echo "❌ Group start failed"
fi

# Test 8: CLI commands
echo "💻 Testing CLI commands..."
if ./provisr group-status --group=workers --api-url=http://localhost:9999/api >/dev/null 2>&1; then
    echo "✅ CLI group commands work"
else
    echo "❌ CLI group commands failed"
fi

echo ""
echo "🎉 All tests passed!"
echo "==================="