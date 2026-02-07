#!/bin/bash
# =============================================================================
# EntDB E2E Test Runner
# =============================================================================
#
# This script runs the end-to-end test suite in Docker.
#
# Usage:
#   ./tests/e2e/run-e2e.sh
#
# Options:
#   --no-cache    Rebuild containers without cache
#   --keep        Keep containers running after tests
#   --logs        Show server logs on failure
#
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.e2e.yml"

# Parse arguments
NO_CACHE=""
KEEP_RUNNING=false
SHOW_LOGS=false

for arg in "$@"; do
    case $arg in
        --no-cache)
            NO_CACHE="--no-cache"
            ;;
        --keep)
            KEEP_RUNNING=true
            ;;
        --logs)
            SHOW_LOGS=true
            ;;
    esac
done

echo "=============================================="
echo "EntDB E2E Test Suite"
echo "=============================================="
echo ""

# Change to project root
cd "$PROJECT_ROOT"

# Build and run tests
echo "[1/3] Building containers..."
docker compose -f "$COMPOSE_FILE" build $NO_CACHE

echo ""
echo "[2/3] Starting infrastructure..."
docker compose -f "$COMPOSE_FILE" up -d redpanda minio
docker compose -f "$COMPOSE_FILE" up -d minio-init
docker compose -f "$COMPOSE_FILE" up -d server

echo "Waiting for server to be healthy..."
docker compose -f "$COMPOSE_FILE" up -d --wait server

echo ""
echo "[3/4] Running e2e tests..."
set +e  # Don't exit on error, we want to capture the exit code

docker compose -f "$COMPOSE_FILE" up --exit-code-from e2e-tests e2e-tests
EXIT_CODE=$?

set -e

# Show logs on failure
if [ $EXIT_CODE -ne 0 ] && [ "$SHOW_LOGS" = true ]; then
    echo ""
    echo "[!] Tests failed. Showing server logs..."
    docker compose -f "$COMPOSE_FILE" logs server
fi

# Cleanup
if [ "$KEEP_RUNNING" = false ]; then
    echo ""
    echo "[4/4] Cleaning up..."
    docker compose -f "$COMPOSE_FILE" down -v
else
    echo ""
    echo "[4/4] Keeping containers running (use 'docker compose -f $COMPOSE_FILE down -v' to clean up)"
fi

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    echo "=============================================="
    echo "E2E Tests PASSED"
    echo "=============================================="
else
    echo "=============================================="
    echo "E2E Tests FAILED (exit code: $EXIT_CODE)"
    echo "=============================================="
fi

exit $EXIT_CODE
