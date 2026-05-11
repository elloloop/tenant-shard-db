#!/bin/bash
# =============================================================================
# EntDB E2E Test Runner
# =============================================================================
#
# This script runs the end-to-end test suite in Docker.
#
# Usage:
#   ./tests/python/e2e/run-e2e.sh                            # Python server (default)
#   ENTDB_SERVER_TARGET=go ./tests/python/e2e/run-e2e.sh     # Go server (Wave-5)
#
# Options:
#   --no-cache    Rebuild containers without cache
#   --keep        Keep containers running after tests
#   --logs        Show server logs on failure
#
# Environment:
#   ENTDB_SERVER_TARGET   "python" (default) or "go". Selects which
#                         server compose file is used. The Go target
#                         uses Dockerfile.go (CGO-free distroless) and
#                         the in-memory WAL — no Redpanda/MinIO.
#
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

ENTDB_SERVER_TARGET="${ENTDB_SERVER_TARGET:-python}"

case "$ENTDB_SERVER_TARGET" in
    python)
        COMPOSE_FILE="$SCRIPT_DIR/docker-compose.e2e.yml"
        # Project name MUST match COMPOSE_PROJECT_NAME inside the
        # e2e-tests container so the crash-recovery test's
        # `docker compose restart server` finds the right container.
        export COMPOSE_PROJECT_NAME="e2e"
        ;;
    go)
        COMPOSE_FILE="$SCRIPT_DIR/docker-compose.e2e.go.yml"
        export COMPOSE_PROJECT_NAME="e2e-go"
        ;;
    *)
        echo "ERROR: ENTDB_SERVER_TARGET must be 'python' or 'go' (got '$ENTDB_SERVER_TARGET')"
        exit 2
        ;;
esac

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
echo "EntDB E2E Test Suite (target=$ENTDB_SERVER_TARGET)"
echo "=============================================="
echo ""

# Change to project root
cd "$PROJECT_ROOT"

# Build and run tests
echo "[1/3] Building containers..."
docker compose -f "$COMPOSE_FILE" build $NO_CACHE

echo ""
echo "[2/3] Starting infrastructure..."

if [ "$ENTDB_SERVER_TARGET" = "go" ]; then
    # Go server has no Kafka/S3 dependencies (in-memory WAL only) and
    # the distroless image carries no healthcheck binary; the test
    # runner retries its own connect, so plain `up -d` is enough.
    docker compose -f "$COMPOSE_FILE" up -d server
    echo "Waiting briefly for Go server to bind its listener..."
    sleep 3
else
    docker compose -f "$COMPOSE_FILE" up -d redpanda minio
    docker compose -f "$COMPOSE_FILE" up -d minio-init
    docker compose -f "$COMPOSE_FILE" up -d server

    echo "Waiting for server to be healthy..."
    docker compose -f "$COMPOSE_FILE" up -d --wait server
fi

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
