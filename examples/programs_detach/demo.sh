#!/bin/bash
set -euo pipefail

# Demo: programs_directory + detached process + store recovery

echo "=== Provisr Detached Programs Directory Demo ==="

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVISR_BIN="$SCRIPT_DIR/../../provisr"
CONFIG_PATH="$SCRIPT_DIR/config.toml"
API_URL="http://127.0.0.1:18080/api"
PID_FILE="./provisr-detached-worker.pid"
DB_FILE="./provisr-detach-demo.db"

cleanup_daemon() {
  echo "Cleaning up provisr daemon..."
  pkill -f "provisr serve $CONFIG_PATH" >/dev/null 2>&1 || true
}

cleanup_worker() {
  if [[ -f "$PID_FILE" ]]; then
    WORKER_PID=$(cat "$PID_FILE" || echo "")
    if [[ -n "${WORKER_PID}" ]]; then
      if kill -0 "$WORKER_PID" 2>/dev/null; then
        echo "Killing detached worker PID $WORKER_PID"
        kill -TERM -$WORKER_PID 2>/dev/null || true
        sleep 1
        kill -KILL -$WORKER_PID 2>/dev/null || true
      fi
    fi
    rm -f "$PID_FILE" || true
  fi
}

cleanup() {
  echo "=== Cleanup ==="
  # Try graceful stop via API if available
  curl -s -X POST "$API_URL/stop?name=detached-worker&wait=1s" >/dev/null 2>&1 || true
  cleanup_daemon
  cleanup_worker
  rm -f "$DB_FILE" || true
}

trap cleanup EXIT

if [[ ! -x "$PROVISR_BIN" ]]; then
  echo "Building provisr binary..."
  (cd "$SCRIPT_DIR/../.." && go build -o provisr)
fi

# Ensure fresh DB for the demo (so we can watch it get recreated)
rm -f "$DB_FILE" || true

# 1) Start provisr daemon which loads the programs directory and starts detached worker
echo "Starting provisr daemon..."
"$PROVISR_BIN" serve "$CONFIG_PATH" &
DAEMON_PID=$!

echo "Waiting for server to be ready..."
sleep 2

# 2) Check worker is running via API
status_json=$(curl -s "$API_URL/status?name=detached-worker" || true)
echo "Initial status: $status_json"
if ! echo "$status_json" | grep -q '"running":true'; then
  echo "ERROR: detached-worker not reported running"
  exit 1
fi

# 3) Kill provisr (manager) and verify the worker is still alive
set +e
kill -TERM "$DAEMON_PID" 2>/dev/null
sleep 1
kill -KILL "$DAEMON_PID" 2>/dev/null
set -e

echo "Provisr daemon killed. Checking detached worker still alive via PID file..."
if [[ ! -f "$PID_FILE" ]]; then
  echo "ERROR: expected PID file $PID_FILE to exist"
  exit 1
fi
WORKER_PID=$(cat "$PID_FILE")
if ! kill -0 "$WORKER_PID" 2>/dev/null; then
  echo "ERROR: detached worker does not seem alive (pid=$WORKER_PID)"
  exit 1
fi

echo "✓ Detached worker is still running after manager death (pid=$WORKER_PID)"

# 4) Restart provisr and verify it reattaches (store-assisted)
"$PROVISR_BIN" serve "$CONFIG_PATH" &
DAEMON2_PID=$!
sleep 2

reattach_json=$(curl -s "$API_URL/status?name=detached-worker" || true)
echo "Reattach status: $reattach_json"
if ! echo "$reattach_json" | grep -q '"running":true'; then
  echo "ERROR: manager did not reattach to detached worker"
  exit 1
fi

echo "✓ Manager reattached to running detached worker"

# 5) Stop the worker via API (this should deliver a stop and update store)
stop_json=$(curl -s -X POST "$API_URL/stop?name=detached-worker&wait=1s" || true)
echo "Stop response: $stop_json"
sleep 1

final_json=$(curl -s "$API_URL/status?name=detached-worker" || true)
echo "Final status: $final_json"
if ! echo "$final_json" | grep -q '"running":false'; then
  echo "ERROR: worker did not stop as expected"
  exit 1
fi

echo "🎉 Demo finished successfully"
echo "✓ Detached child survived manager restart"
echo "✓ Store-based recovery allowed reattachment and stop handling"
