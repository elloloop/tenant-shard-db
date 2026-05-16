#!/bin/bash
# =============================================================================
# EntDB E2E Test Runner
# =============================================================================
#
# Runs the end-to-end test suite in Docker against the Go server.
#
# (Historical: this harness used to support an ENTDB_SERVER_TARGET=python|go
#  switch during the Go-rewrite. The Python server was retired in Phase 4D
#  of EPIC #407, so only the Go-target compose file remains.)
#
# Usage:
#   ./tests/python/e2e/run-e2e.sh
#
# Options:
#   --no-cache    Rebuild containers without cache
#   --keep        Keep containers running after tests
#   --logs        Show server logs on failure
#
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

COMPOSE_FILE="$SCRIPT_DIR/docker-compose.e2e.yml"
# Project name MUST match COMPOSE_PROJECT_NAME inside the e2e-tests
# container so the crash-recovery test's `docker compose restart server`
# finds the right container.
export COMPOSE_PROJECT_NAME="e2e-go"

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
echo "EntDB E2E Test Suite (target=go)"
echo "=============================================="
echo ""

# Change to project root
cd "$PROJECT_ROOT"

# Build and run tests
echo "[1/4] Resetting existing e2e containers..."
docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true

echo ""
echo "[2/4] Building containers..."
docker compose -f "$COMPOSE_FILE" build $NO_CACHE

echo ""
echo "[3/4] Starting infrastructure..."

# Go server uses Kafka/Redpanda as its WAL. The distroless image carries
# no healthcheck binary; the test runner retries its own connect, so plain
# `up -d` is enough.
docker compose -f "$COMPOSE_FILE" up -d --wait redpanda
docker compose -f "$COMPOSE_FILE" up -d server
echo "Waiting briefly for Go server to bind its listener..."
sleep 3

echo ""
echo "[4/4] Running e2e tests..."
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
    echo "[cleanup] Cleaning up..."
    docker compose -f "$COMPOSE_FILE" down -v
else
    echo ""
    echo "[cleanup] Keeping containers running (use 'docker compose -f $COMPOSE_FILE down -v' to clean up)"
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
