#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# EntDB bench runner — Tier 1 (EntDB only) and Tier 2 (EntDB vs Postgres).
#
# Replaces the old ``run_dual_backend.sh`` (Python-vs-Go), which has
# been dead since the Python server was deleted in Phase 4D of
# EPIC #407 (commit 8d07f5f).
#
# Tier 1 — EntDB at its smallest container.
#
#   ./run_bench.sh --tier 1
#
#   Runs ``bench_entdb.py`` against a Go ``entdb-server`` subprocess on
#   the host. No Postgres, no Docker compose. This is the "is EntDB
#   regressing against itself" gate; it runs on every PR.
#
# Tier 2 — Parity (EntDB vs Postgres at the Postgres floor).
#
#   ./run_bench.sh --tier 2
#   ./run_bench.sh --with-postgres        # equivalent
#
#   Brings up ``docker-compose.bench.yml`` (Go server + postgres:17-alpine,
#   both pinned to ``--cpus=1.0`` / ``--memory=512m`` — see the doc for
#   the empirical floor measurement), runs both bench modules, writes
#   per-backend JSON under ``.benchmarks/{entdb-go,postgres}/``.
#
# Env vars:
#   PYTEST_ARGS    extra args appended to the pytest invocation. Parsed
#                  as a space-separated list via ``read -ra`` — quoting
#                  nested args (e.g. ``-k "edge or write"``) is not
#                  supported; use ``-k edge`` or ``-k 'edge or write'``
#                  on the command line directly if you need a complex
#                  selector.
#   BENCH_ROUNDS   --benchmark-min-rounds value (default 5)
#   BENCH_MEM      override the per-container memory budget for Tier 2
#                  (Tier 2 ONLY — Tier 1 runs as a host subprocess
#                  without cgroup limits and ignores this).

set -euo pipefail

TIER=1
WITH_POSTGRES=0
ROUNDS="${BENCH_ROUNDS:-5}"
MEM_OVERRIDE="${BENCH_MEM:-}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --tier)
            TIER="$2"
            shift 2
            ;;
        --tier=*)
            TIER="${1#*=}"
            shift
            ;;
        --with-postgres)
            WITH_POSTGRES=1
            TIER=2
            shift
            ;;
        --memory)
            MEM_OVERRIDE="$2"
            shift 2
            ;;
        --memory=*)
            MEM_OVERRIDE="${1#*=}"
            shift
            ;;
        --rounds)
            ROUNDS="$2"
            shift 2
            ;;
        --rounds=*)
            ROUNDS="${1#*=}"
            shift
            ;;
        --help|-h)
            sed -n '2,38p' "$0"
            exit 0
            ;;
        *)
            echo "unknown arg: $1" >&2
            exit 2
            ;;
    esac
done

if [[ "$TIER" != "1" && "$TIER" != "2" ]]; then
    echo "--tier must be 1 or 2" >&2
    exit 2
fi
if [[ "$TIER" == "2" ]]; then
    WITH_POSTGRES=1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
TIER_TAG="tier${TIER}"
if [[ "$TIER" == "1" ]]; then
    TIER_TAG="tier1_constrained"
else
    TIER_TAG="tier2_parity"
fi

OUT_ENTDB=".benchmarks/entdb-go/${TS}_${TIER_TAG}.json"
mkdir -p ".benchmarks/entdb-go"

# Parse PYTEST_ARGS into an array preserving word boundaries. The
# previous form ``${PYTEST_ARGS:-}`` inside an array splat lost
# whitespace structure — multi-token args still split on IFS but a
# quoted segment like ``-k "edge or write"`` could not survive the
# round-trip through an exported env var anyway. ``read -ra`` is the
# bash-idiomatic split; see ``--help`` for the contract (issue #491
# item 8).
PYTEST_EXTRA_ARGS=()
if [[ -n "${PYTEST_ARGS:-}" ]]; then
    # shellcheck disable=SC2206  # intentional word-splitting on $PYTEST_ARGS
    read -ra PYTEST_EXTRA_ARGS <<< "${PYTEST_ARGS}"
fi

PYTEST_BASE_ARGS=(
    --benchmark-only
    --benchmark-min-rounds="${ROUNDS}"
    --benchmark-disable-gc
    --timeout=120
    -q
    "${PYTEST_EXTRA_ARGS[@]}"
)

# ---------------------------------------------------------------------------
# Tier 1 — EntDB only, host subprocess.
# ---------------------------------------------------------------------------

if [[ "$TIER" == "1" ]]; then
    echo "=== Tier 1 — EntDB only ==="
    python -m pytest tests/python/benchmarks/bench_entdb.py \
        "${PYTEST_BASE_ARGS[@]}" \
        --benchmark-json="${OUT_ENTDB}"
    echo
    echo "Wrote:"
    echo "  ${OUT_ENTDB}"
    exit 0
fi

# ---------------------------------------------------------------------------
# Tier 2 — EntDB + Postgres at parity.
# ---------------------------------------------------------------------------

OUT_PG=".benchmarks/postgres/${TS}_${TIER_TAG}.json"
mkdir -p ".benchmarks/postgres"

COMPOSE_FILE="${REPO_ROOT}/tests/python/benchmarks/docker-compose.bench.yml"

cleanup() {
    docker compose -f "$COMPOSE_FILE" down --remove-orphans -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Allow runtime memory override.
if [[ -n "$MEM_OVERRIDE" ]]; then
    export COMPOSE_BENCH_MEM="$MEM_OVERRIDE"
    # docker-compose v2 doesn't interpolate ``mem_limit`` from env, so
    # we have to override per-service with a one-shot YAML patch.
    PATCH_FILE="$(mktemp)"
    cat > "$PATCH_FILE" <<YAML
services:
  entdb-server:
    mem_limit: ${MEM_OVERRIDE}
  postgres:
    mem_limit: ${MEM_OVERRIDE}
YAML
    COMPOSE_ARGS=(-f "$COMPOSE_FILE" -f "$PATCH_FILE")
else
    COMPOSE_ARGS=(-f "$COMPOSE_FILE")
fi

echo "=== Tier 2 — bringing up compose stack (EntDB + Postgres) ==="
docker compose "${COMPOSE_ARGS[@]}" up -d --build --wait
PG_PORT=55432
ENTDB_PORT=50051

export ENTDB_BENCH_PG_DSN="postgresql://bench:bench@127.0.0.1:${PG_PORT}/bench"
export ENTDB_BENCH_ENDPOINT="127.0.0.1:${ENTDB_PORT}"

echo
echo "=== Tier 2 — EntDB bench ==="
python -m pytest tests/python/benchmarks/bench_entdb.py \
    "${PYTEST_BASE_ARGS[@]}" \
    --benchmark-json="${OUT_ENTDB}"

echo
echo "=== Tier 2 — Postgres bench ==="
python -m pytest tests/python/benchmarks/bench_postgres.py \
    "${PYTEST_BASE_ARGS[@]}" \
    --benchmark-json="${OUT_PG}"

# Dump EXPLAIN ANALYZE for each bench query (issue #487 open question #10).
EXPLAIN_OUT=".benchmarks/postgres/${TS}_${TIER_TAG}_explain.txt"
python tests/python/benchmarks/dump_pg_explain.py \
    "${ENTDB_BENCH_PG_DSN}" > "${EXPLAIN_OUT}" 2>&1 || true

echo
echo "Wrote:"
echo "  ${OUT_ENTDB}"
echo "  ${OUT_PG}"
echo "  ${EXPLAIN_OUT}"
