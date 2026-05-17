#!/bin/bash
# =============================================================================
# EntDB Archive E2E Runner — WAL -> S3 Object Lock round-trip (EPIC #511)
# =============================================================================
#
# Boots Redpanda + MinIO (Object-Lock COMPLIANCE bucket) + entdb-server
# with -archive-enabled, then runs test_archive_object_lock.py which
# drives the SDK and probes S3 + the Prometheus /metrics endpoint.
#
# Kept separate from run-e2e.sh so the fast 22-case suite doesn't pay
# the MinIO + archive-sidecar startup cost.
#
# Usage:
#   ./tests/python/e2e/run-archive-e2e.sh [--no-cache] [--keep] [--logs]
#
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

COMPOSE_FILE="$SCRIPT_DIR/docker-compose.archive.yml"
export COMPOSE_PROJECT_NAME="e2e-archive"

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
echo "EntDB Archive E2E (WAL -> S3 Object Lock)"
echo "=============================================="
echo ""

cd "$PROJECT_ROOT"

echo "[1/4] Resetting existing archive-e2e containers..."
docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true

echo ""
echo "[2/4] Building containers..."
docker compose -f "$COMPOSE_FILE" build $NO_CACHE

echo ""
echo "[3/4] Starting infrastructure (redpanda + minio + object-lock bucket + server)..."
docker compose -f "$COMPOSE_FILE" up -d --wait redpanda minio
docker compose -f "$COMPOSE_FILE" up --no-log-prefix minio-init
docker compose -f "$COMPOSE_FILE" up -d server
echo "Waiting briefly for the Go server to bind its listener..."
sleep 5

echo ""
echo "[4/4] Running archive e2e tests..."
set +e
docker compose -f "$COMPOSE_FILE" up --exit-code-from archive-tests archive-tests
EXIT_CODE=$?
set -e

if [ $EXIT_CODE -ne 0 ] && [ "$SHOW_LOGS" = true ]; then
    echo ""
    echo "[!] Tests failed. Showing server logs..."
    docker compose -f "$COMPOSE_FILE" logs server
    echo ""
    echo "[!] minio-init logs..."
    docker compose -f "$COMPOSE_FILE" logs minio-init
fi

if [ "$KEEP_RUNNING" = false ]; then
    echo ""
    echo "[cleanup] Cleaning up..."
    docker compose -f "$COMPOSE_FILE" down -v
else
    echo ""
    echo "[cleanup] Keeping containers running (docker compose -f $COMPOSE_FILE down -v to clean up)"
fi

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    echo "=============================================="
    echo "Archive E2E PASSED"
    echo "=============================================="
else
    echo "=============================================="
    echo "Archive E2E FAILED (exit code: $EXIT_CODE)"
    echo "=============================================="
fi

exit $EXIT_CODE
