#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# Dual-backend benchmark runner (Phase 4B of EPIC #407).
#
# Runs ``bench_grpc_dual.py`` against both the Python (in-process) and
# Go (subprocess) EntDB servers, capturing pytest-benchmark JSON to
# ``.benchmarks/{python,go}/<timestamp>.json``.
#
# Two execution modes:
#
#   ./run_dual_backend.sh                     -> run on the host (no resource limits)
#   ./run_dual_backend.sh --constrained       -> run inside a Docker container
#                                                with ``--cpus=1 --memory=512m``
#                                                (a "small CI runner" emulation)
#
# The constrained mode is what the CI matrix uses on Linux; the host
# mode is the local-dev fallback (e.g. macOS, where Docker Desktop's
# virtualization layer distorts resource limits anyway).
#
# Env vars:
#   PYTEST_ARGS    extra args appended to the pytest invocation
#   BENCH_ROUNDS   --benchmark-min-rounds value (default 30)

set -euo pipefail

ROUNDS="${BENCH_ROUNDS:-30}"
CONSTRAINED=0
if [[ "${1:-}" == "--constrained" ]]; then
    CONSTRAINED=1
    shift
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_PY=".benchmarks/python/${TS}.json"
OUT_GO=".benchmarks/go/${TS}.json"
mkdir -p .benchmarks/python .benchmarks/go

PYTEST_BASE_ARGS=(
    tests/python/benchmarks/bench_grpc_dual.py
    --benchmark-only
    --benchmark-min-rounds="${ROUNDS}"
    --benchmark-disable-gc
    --timeout=120
    -q
    ${PYTEST_ARGS:-}
)

run_local() {
    local target="$1"
    local out="$2"
    echo "=== Running bench_grpc_dual.py with ENTDB_SERVER_TARGET=${target} ==="
    ENTDB_SERVER_TARGET="${target}" python -m pytest "${PYTEST_BASE_ARGS[@]}" \
        --benchmark-json="${out}"
}

run_docker() {
    # Constrained mode: spin up a Linux container with the repo mounted,
    # the right deps pre-installed, and the host's Docker socket
    # exposed (the Go target is in-container, no need for the socket).
    #
    # We use the existing python:3.11-slim base since the bench needs:
    #   - python + pytest-benchmark
    #   - the Go toolchain (to build entdb-server when ENTDB_SERVER_TARGET=go)
    local target="$1"
    local out_relative="$2"
    echo "=== Running bench_grpc_dual.py in Docker (--cpus=1 --memory=512m) with ENTDB_SERVER_TARGET=${target} ==="
    # The repo's .go-bin/ cache may contain a host-arch (e.g. darwin/arm64)
    # binary that won't exec in a linux container. Build a fresh one
    # inside the container and pin it via ENTDB_GO_BINARY so the
    # integration conftest doesn't try to reuse the cached file.
    docker run --rm \
        --cpus=1 \
        --memory=512m \
        -v "${REPO_ROOT}:/work" \
        -w /work \
        -e "ENTDB_SERVER_TARGET=${target}" \
        -e PYTHONPATH=/work/server/python:/work/sdk/python \
        -e ENTDB_GO_BINARY=/tmp/entdb-server-linux \
        golang:1.25-bookworm \
        bash -c "
            set -e
            apt-get update -qq && apt-get install -qq -y python3 python3-pip python3-venv >/dev/null
            python3 -m venv /tmp/venv
            . /tmp/venv/bin/activate
            pip install -q -e ./server/python -e ./sdk/python \
                pytest pytest-benchmark==5.2.3 pytest-asyncio pytest-timeout grpcio-tools
            if [ '${target}' = 'go' ]; then
                cd /work/server/go && go build -o /tmp/entdb-server-linux ./cmd/entdb-server && cd /work
            fi
            python -m pytest ${PYTEST_BASE_ARGS[*]} --benchmark-json='${out_relative}'
        "
}

if [[ "${CONSTRAINED}" -eq 1 ]]; then
    run_docker python "${OUT_PY}"
    run_docker go "${OUT_GO}"
else
    run_local python "${OUT_PY}"
    run_local go "${OUT_GO}"
fi

echo
echo "Wrote:"
echo "  ${OUT_PY}"
echo "  ${OUT_GO}"
