#!/bin/bash
# =============================================================================
# EntDB Dual-Stack Parity Test Runner — Phase 4A (EPIC #407)
# =============================================================================
#
# Boots the Python and Go gRPC servers side-by-side on independent
# Kafka clusters, runs the parity suite (tests/python/parity/), and
# tears the stack down.
#
# Transitional: this script + ../parity/ delete together with
# server/python/ in Phase 4D. There is intentionally no CI job
# pointing at it (one-off pre-deletion sanity, not a recurring gate).
#
# Options:
#   --no-cache   Rebuild containers without cache.
#   --keep       Don't tear the stack down at exit (debug).
#   --logs       Show server logs on test failure.
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.parity.yml"
export COMPOSE_PROJECT_NAME="entdb-parity"

NO_CACHE=""
KEEP_RUNNING=false
SHOW_LOGS=false

for arg in "$@"; do
    case $arg in
        --no-cache) NO_CACHE="--no-cache" ;;
        --keep) KEEP_RUNNING=true ;;
        --logs) SHOW_LOGS=true ;;
    esac
done

echo "=============================================="
echo "EntDB Dual-Stack Parity Suite (Phase 4A)"
echo "=============================================="
echo ""

cd "$PROJECT_ROOT"

echo "[1/4] Building containers..."
docker compose -f "$COMPOSE_FILE" build $NO_CACHE

echo ""
echo "[2/4] Starting infrastructure (Redpanda x2, MinIO)..."
docker compose -f "$COMPOSE_FILE" up -d --wait redpanda-py redpanda-go minio
docker compose -f "$COMPOSE_FILE" up -d minio-init

echo ""
echo "[3/4] Starting both EntDB servers..."
docker compose -f "$COMPOSE_FILE" up -d server-python server-go
echo "Waiting for Python server healthcheck..."
docker compose -f "$COMPOSE_FILE" up -d --wait server-python
echo "Giving the Go server a beat to bind its listener..."
sleep 3

echo ""
echo "[4/4] Running parity scenarios..."
set +e
docker compose -f "$COMPOSE_FILE" up --exit-code-from parity-tests parity-tests
EXIT_CODE=$?
set -e

if [ $EXIT_CODE -ne 0 ] && [ "$SHOW_LOGS" = true ]; then
    echo ""
    echo "[!] Parity tests failed. Showing server logs..."
    echo "--- python server ---"
    docker compose -f "$COMPOSE_FILE" logs server-python
    echo "--- go server ---"
    docker compose -f "$COMPOSE_FILE" logs server-go
fi

if [ "$KEEP_RUNNING" = false ]; then
    echo ""
    echo "Cleaning up..."
    docker compose -f "$COMPOSE_FILE" down -v
else
    echo ""
    echo "Containers left running (use 'docker compose -f $COMPOSE_FILE down -v' to clean up)"
fi

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    echo "=============================================="
    echo "Parity Suite PASSED"
    echo "=============================================="
else
    echo "=============================================="
    echo "Parity Suite FAILED (exit code: $EXIT_CODE)"
    echo "=============================================="
fi

exit $EXIT_CODE
