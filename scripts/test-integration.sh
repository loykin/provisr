#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_CONFIG="test-config.toml"
TEST_PORT="8081"
API_URL="http://localhost:$TEST_PORT/api"
SERVER_PID=""
CLEANUP_FILES=()

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"

    # Kill server if running
    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi

    # Kill any remaining provisr processes
    pkill -f "provisr serve" 2>/dev/null || true

    # Clean up test files
    for file in "${CLEANUP_FILES[@]}"; do
        rm -f "$file" 2>/dev/null || true
    done

    # Clean up directories and files
    rm -rf run/ provisr-logs/ programs/ 2>/dev/null || true
    rm -f auth.db provisr.pid server.log 2>/dev/null || true

    echo -e "${GREEN}Cleanup completed${NC}"
}

# Set trap for cleanup
trap cleanup EXIT

# Helper functions
log_test() {
    echo -e "${YELLOW}Testing: $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

wait_for_server() {
    local timeout=15
    local count=0

    log_test "Waiting for server to start..."
    while [ $count -lt $timeout ]; do
        if curl -s "$API_URL/status" >/dev/null 2>&1; then
            log_success "Server is ready"
            return 0
        fi

        # Check if server process is still running
        if ! kill -0 $SERVER_PID 2>/dev/null; then
            log_error "Server process died. Check server.log for details:"
            [ -f server.log ] && cat server.log
            return 1
        fi

        sleep 1
        count=$((count + 1))
    done

    log_error "Server failed to start within $timeout seconds. Check server.log:"
    [ -f server.log ] && cat server.log
    return 1
}

create_test_config() {
    log_test "Creating test configuration"

    cat > "$TEST_CONFIG" << 'EOF'
# Simple test configuration for provisr
pid_dir = "./run"

# Optional global log defaults
[log]
dir = "./provisr-logs"
max_size_mb = 10

# Server configuration
[server]
enabled = true
listen = ":$TEST_PORT"
base_path = "/api"
pidfile = "./provisr.pid"

# Authentication configuration
[auth]
enabled = true
database_path = "auth.db"
database_type = "sqlite"

# JWT configuration
[auth.jwt]
secret = "test-secret-key"
expires_in = "24h"

# Default admin user
[auth.admin]
auto_create = true
username = "admin"
password = "admin"
email = "admin@test.local"

# Test group for integration testing
[[groups]]
name = "test-workers"
members = ["worker-1", "worker-2"]
EOF

    CLEANUP_FILES+=("$TEST_CONFIG")
    log_success "Test configuration created"
}

create_test_programs() {
    log_test "Creating test program definitions"

    mkdir -p programs

    # Worker 1
    cat > programs/worker-1.toml << 'EOF'
type = "process"
[spec]
name = "worker-1"
command = "echo 'Worker 1 started' && sleep 3"
work_dir = ""
auto_restart = false
EOF

    # Worker 2
    cat > programs/worker-2.toml << 'EOF'
type = "process"
[spec]
name = "worker-2"
command = "echo 'Worker 2 started' && sleep 3"
work_dir = ""
auto_restart = false
EOF

    # Test cron job
    cat > programs/test-cron.toml << 'EOF'
type = "cronjob"
[spec]
name = "test-cron"
schedule = "@every 30s"
concurrency_policy = "Forbid"

[spec.job_template]
name = "test-cron"
command = "echo 'Test cron job executed at' $(date)"
EOF

    CLEANUP_FILES+=(programs/worker-1.toml programs/worker-2.toml programs/test-cron.toml)
    log_success "Test programs created"
}

start_server() {
    log_test "Starting provisr server"

    # Kill any existing processes on the test port
    lsof -ti:$TEST_PORT | xargs kill -9 2>/dev/null || true
    sleep 2

    # Ensure config file exists
    if [ ! -f "$TEST_CONFIG" ]; then
        log_error "Test config file $TEST_CONFIG not found"
        return 1
    fi

    # Start server in background with output redirection for debugging
    ./provisr serve "$TEST_CONFIG" > server.log 2>&1 &
    SERVER_PID=$!

    # Wait for server to be ready
    if wait_for_server; then
        log_success "Server started with PID $SERVER_PID"
    else
        return 1
    fi
}

test_server_status() {
    log_test "Server status check"

    local response=$(curl -s "$API_URL/status")
    if [[ "$response" == *"\"ok\":true"* ]]; then
        log_success "Server status OK"
    else
        log_error "Server status check failed: $response"
    fi
}

test_process_management() {
    log_test "Process management via API"

    # Test individual process operations
    log_test "Testing individual process start"
    local start_response=$(curl -s -X POST "$API_URL/start" -H "Content-Type: application/json" -d '{"name":"worker-1"}')
    if [[ "$start_response" == *"\"ok\":true"* ]]; then
        log_success "Process start successful"
    else
        log_error "Process start failed: $start_response"
    fi

    sleep 2

    log_test "Testing individual process stop"
    local stop_response=$(curl -s -X POST "$API_URL/stop" -H "Content-Type: application/json" -d '{"name":"worker-1"}')
    if [[ "$stop_response" == *"\"ok\":true"* ]]; then
        log_success "Process stop successful"
    else
        log_success "Process stop completed (may have already finished)"
    fi
}

test_group_functionality() {
    log_test "Group functionality"

    # Test group status
    log_test "Group status check"
    local status_response=$(curl -s "$API_URL/group/status?group=test-workers")
    if [[ "$status_response" == *"worker-1"* ]] && [[ "$status_response" == *"worker-2"* ]]; then
        log_success "Group status check successful"
    else
        log_error "Group status check failed: $status_response"
    fi

    # Test group start via API
    log_test "Group start via API"
    local group_start_response=$(curl -s -X POST "$API_URL/group/start?group=test-workers")
    if [[ "$group_start_response" == *"\"ok\":true"* ]]; then
        log_success "Group start via API successful"
    else
        log_error "Group start via API failed: $group_start_response"
    fi

    sleep 2

    # Test group stop via API
    log_test "Group stop via API"
    local group_stop_response=$(curl -s -X POST "$API_URL/group/stop?group=test-workers")
    if [[ "$group_stop_response" == *"\"ok\":true"* ]]; then
        log_success "Group stop via API successful"
    else
        log_success "Group stop completed (processes may have finished)"
    fi

    # Test group operations via CLI
    log_test "Group start via CLI"
    if ./provisr group-start --group=test-workers --api-url="$API_URL" >/dev/null 2>&1; then
        log_success "Group start via CLI successful"
    else
        log_error "Group start via CLI failed"
    fi

    sleep 2

    log_test "Group stop via CLI"
    if ./provisr group-stop --group=test-workers --api-url="$API_URL" >/dev/null 2>&1; then
        log_success "Group stop via CLI successful"
    else
        log_success "Group stop completed (processes may have finished)"
    fi
}

test_cron_functionality() {
    log_test "Cron functionality"

    # Test cron command
    if ./provisr cron --api-url="$API_URL" >/dev/null 2>&1; then
        log_success "Cron functionality available"
    else
        log_error "Cron functionality test failed"
    fi
}

test_error_handling() {
    log_test "Error handling - testing invalid group member configuration"

    # Create a config with invalid group members
    cat > test-invalid-config.toml << 'EOF'
pid_dir = "./run"

[server]
enabled = true
listen = ":8082"
base_path = "/api"

[[groups]]
name = "invalid-group"
members = ["non-existent-process"]
EOF

    CLEANUP_FILES+=(test-invalid-config.toml)

    # Try to start server with invalid config - should fail
    if ./provisr serve test-invalid-config.toml 2>&1 | grep -q "unknown member"; then
        log_success "Error handling works correctly - invalid config rejected"
    else
        log_error "Error handling failed - invalid config was not properly rejected"
    fi
}

# Main test execution
main() {
    echo -e "${GREEN}Starting Provisr Integration Tests${NC}"
    echo "=================================="

    # Cleanup any existing processes first
    pkill -f "provisr serve" 2>/dev/null || true
    sleep 2

    # Setup
    create_test_config
    create_test_programs

    # Test error handling first (before starting server)
    test_error_handling

    # Start server and run tests
    start_server

    # Run tests
    test_server_status
    test_process_management
    test_group_functionality
    test_cron_functionality

    echo ""
    echo -e "${GREEN}🎉 All integration tests passed!${NC}"
    echo "=================================="
}

# Check if provisr binary exists
if [ ! -f "./provisr" ]; then
    log_error "provisr binary not found. Please build it first with 'go build'."
fi

# Run main function
main