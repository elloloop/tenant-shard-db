# SPDX-License-Identifier: AGPL-3.0-only
"""Dual-stack parity fixtures — Phase 4A (EPIC #407).

The parity suite boots the Python and Go gRPC servers side-by-side
(see ``docker-compose.parity.yml``) and drives the same RPC sequences
against each via two separate SDK clients. These fixtures handle:

* connecting both clients (with retry-on-boot mirroring the e2e
  conftest)
* a private, freshly-allocated tenant per test (so test-order
  independence holds and there's no cross-contamination from one
  scenario's writes into another's read-back)
* normalisation of nodes / receipts (UUIDs, timestamps, WAL offsets,
  list-order non-determinism) so the parity assertion focuses on
  semantic equivalence
* state-snapshot + deep-equality helpers
* a ``restart_servers`` callable for the WAL-replay parity test

This module is intentionally **not** a permanent CI fixture. Once
Phase 4D deletes ``server/python/`` the file becomes dead — track it
in the cleanup task for that phase.
"""

from __future__ import annotations

import asyncio
import os
import re
import subprocess
import sys
import time
from collections.abc import AsyncGenerator, Awaitable, Callable
from dataclasses import asdict, is_dataclass
from typing import Any

import pytest
import pytest_asyncio

# Make ``import e2e_schema_pb2`` resolve regardless of CWD.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import e2e_schema_pb2 as pb  # noqa: E402

from entdb_sdk import DbClient, register_proto_schema  # noqa: E402

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

PYTHON_HOST = os.environ.get("ENTDB_PYTHON_HOST", "server-python")
PYTHON_PORT = int(os.environ.get("ENTDB_PYTHON_PORT", "50051"))
GO_HOST = os.environ.get("ENTDB_GO_HOST", "server-go")
GO_PORT = int(os.environ.get("ENTDB_GO_PORT", "50051"))

# The seed tenant is created on the Go server via --seed-profile=e2e;
# the Python server lacks an analogous flag (its global_store is
# disabled in dev-mode), so the first write to a tenant id creates
# it. Each test allocates its own private tenant so cross-test state
# never leaks into the parity comparison.
DEFAULT_TENANT = os.environ.get("ENTDB_TENANT", "e2e-test")
ACTOR = "user:e2e-runner"

COMPOSE_FILE_IN_CONTAINER = "/compose/docker-compose.yml"


# ---------------------------------------------------------------------------
# Connection helpers
# ---------------------------------------------------------------------------


def _wait_for_tcp(host: str, port: int, timeout: int = 90) -> None:
    """Block until ``host:port`` accepts a TCP connection.

    The Go server starts almost immediately; the Python server can
    take ~30s in a cold-build container. 90s is a generous
    upper-bound that still fails fast when the container is missing.
    """
    import socket

    deadline = time.time() + timeout
    last_err: Exception | None = None
    while time.time() < deadline:
        try:
            with socket.create_connection((host, port), timeout=2):
                return
        except (TimeoutError, OSError) as exc:
            last_err = exc
            time.sleep(1)
    raise RuntimeError(
        f"gRPC server at {host}:{port} not reachable after {timeout}s (last error: {last_err!r})"
    )


_SCHEMA_REGISTERED = False


async def _connect(target: str) -> DbClient:
    """Build a connected DbClient with the e2e schema registered."""
    global _SCHEMA_REGISTERED
    if not _SCHEMA_REGISTERED:
        register_proto_schema(pb)
        _SCHEMA_REGISTERED = True

    client = DbClient(target)
    last_exc: Exception | None = None
    for _ in range(30):
        try:
            await client.connect()
            return client
        except Exception as exc:  # noqa: BLE001
            last_exc = exc
            await asyncio.sleep(2)
    raise RuntimeError(f"could not connect to {target}: {last_exc!r}")


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def python_target() -> str:
    return f"{PYTHON_HOST}:{PYTHON_PORT}"


@pytest.fixture(scope="session")
def go_target() -> str:
    return f"{GO_HOST}:{GO_PORT}"


@pytest.fixture(scope="session")
def actor() -> str:
    return ACTOR


_BOOT_WAITED = False


@pytest_asyncio.fixture
async def python_client(python_target: str) -> AsyncGenerator[DbClient, None]:
    """Connected SDK client for the Python server.

    Function-scoped (mirrors e2e conftest): pytest-asyncio's default
    event loop is per-function and ``grpc.aio`` channels are bound to
    the loop they're built on, so reusing one across tests yields the
    well-known "attached to a different loop" error.
    """
    global _BOOT_WAITED
    if not _BOOT_WAITED:
        _wait_for_tcp(PYTHON_HOST, PYTHON_PORT)
        _wait_for_tcp(GO_HOST, GO_PORT)
        _BOOT_WAITED = True

    client = await _connect(python_target)
    try:
        yield client
    finally:
        await client.close()


@pytest_asyncio.fixture
async def go_client(go_target: str) -> AsyncGenerator[DbClient, None]:
    """Connected SDK client for the Go server."""
    global _BOOT_WAITED
    if not _BOOT_WAITED:
        _wait_for_tcp(PYTHON_HOST, PYTHON_PORT)
        _wait_for_tcp(GO_HOST, GO_PORT)
        _BOOT_WAITED = True

    client = await _connect(go_target)
    try:
        yield client
    finally:
        await client.close()


@pytest_asyncio.fixture
async def parity_tenant(python_client: DbClient, go_client: DbClient) -> str:
    """Allocate the same private tenant id on BOTH servers.

    Per-test tenant isolation is what lets us write `final state must
    be identical` (rather than `final state must be a superset of the
    pre-test state`). Side-by-side scenarios pollute each other's
    universal-tenant state otherwise.

    The Go server boots with its global_store enabled, so we must
    CreateTenant + AddTenantMember explicitly. The Python server runs
    with global_store disabled (dev mode) so any tenant id is auto-
    created on first write — simply minting the id is sufficient.
    """
    import uuid

    tid = f"parity-{uuid.uuid4().hex[:10]}"

    # Go server requires an explicit tenant.
    create = await go_client.create_tenant(
        tenant_id=tid,
        name=f"parity {tid}",
        actor="system:parity-admin",
    )
    if not create.get("success"):
        raise RuntimeError(f"go CreateTenant({tid}) failed: {create.get('error')}")
    await go_client.add_tenant_member(
        tenant_id=tid,
        user_id="e2e-runner",
        role="owner",
        actor="system:parity-admin",
    )

    return tid


# ---------------------------------------------------------------------------
# Restart helper (used by the WAL-replay parity scenario)
# ---------------------------------------------------------------------------


@pytest.fixture
def restart_servers() -> Callable[[], Awaitable[None]]:
    """Return an async callable that restarts BOTH servers via the
    bind-mounted compose file. Skips when the docker socket or
    compose file aren't available (e.g. ad-hoc host-side runs).
    """

    async def _restart() -> None:
        if not os.path.exists("/var/run/docker.sock"):
            pytest.skip("restart_servers requires /var/run/docker.sock")
        if not os.path.exists(COMPOSE_FILE_IN_CONTAINER):
            pytest.skip(f"restart_servers requires {COMPOSE_FILE_IN_CONTAINER}")

        def _cmd() -> None:
            subprocess.run(
                [
                    "docker",
                    "compose",
                    "-f",
                    COMPOSE_FILE_IN_CONTAINER,
                    "restart",
                    "server-python",
                    "server-go",
                ],
                check=True,
                capture_output=True,
            )

        await asyncio.to_thread(_cmd)

        # Wait for both listeners to come back. Python's Kafka
        # consumer has to rejoin its group + replay; the Go server
        # restarts in <1s. The applier-replay settle wait is the same
        # 5s the e2e conftest uses for the Python target.
        await asyncio.to_thread(_wait_for_tcp, PYTHON_HOST, PYTHON_PORT, 90)
        await asyncio.to_thread(_wait_for_tcp, GO_HOST, GO_PORT, 90)
        await asyncio.sleep(5)

    return _restart


# ---------------------------------------------------------------------------
# Normalisation helpers
# ---------------------------------------------------------------------------

# UUIDs (canonical 8-4-4-4-12 hex form, both lowercase and uppercase).
_UUID_RE = re.compile(
    r"\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-"
    r"[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b"
)

# RFC3339-ish timestamps.
_TS_RE = re.compile(r"\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b")


def _replace_uuids(s: str) -> str:
    return _UUID_RE.sub("<UUID>", s)


def _replace_ts(s: str) -> str:
    return _TS_RE.sub("<TS>", s)


def _normalise_value(v: Any) -> Any:
    """Recursively normalise a value: replace UUIDs / timestamps in
    strings, swallow timestamp-shaped ints, and pass dict/list through
    field-by-field. ``created_at`` / ``updated_at`` / ``applied_at`` /
    ``ts_ms`` / ``stream_position`` are emptied because their values
    legitimately differ between independent servers running on
    independent WALs."""
    if isinstance(v, str):
        return _replace_ts(_replace_uuids(v))
    if isinstance(v, (int, float, bool)) or v is None:
        return v
    if isinstance(v, dict):
        return {k: _normalise_value(val) for k, val in v.items()}
    if isinstance(v, list):
        return [_normalise_value(x) for x in v]
    if is_dataclass(v):
        return _normalise_value(asdict(v))
    # Fall through: unknown object — stringify and normalise.
    return _replace_ts(_replace_uuids(str(v)))


# Fields whose values legitimately differ between servers and MUST be
# stripped before comparison. These are wall-clock or WAL-position
# artefacts, not semantic state.
_STRIPPED_KEYS = frozenset(
    {
        "node_id",
        "id",  # receipt.id
        "created_at",
        "updated_at",
        "applied_at",
        "ts_ms",
        "stream_position",
        "offset",
        "wal_offset",
        "receipt_id",
        "idempotency_key_uuid",  # parity tests use deterministic
        # idempotency_key values for the
        # idempotent scenario; if a future
        # field added a per-call uuid it
        # would be elided here.
        "trace_id",
        "tenant_id",  # private per-test tenant ids differ by run.
    }
)


def _strip_volatile(d: dict[str, Any]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for k, v in d.items():
        if k in _STRIPPED_KEYS:
            continue
        if isinstance(v, dict):
            out[k] = _strip_volatile(v)
        elif isinstance(v, list):
            out[k] = [_strip_volatile(x) if isinstance(x, dict) else _normalise_value(x) for x in v]
        else:
            out[k] = _normalise_value(v)
    return out


def normalise_node(node: Any) -> dict[str, Any]:
    """Reduce a Node to its parity-comparable form: type_id + payload
    + owner_actor + sorted-acl. Everything else (id, timestamps,
    tenant_id) is volatile or per-server.
    """
    d = asdict(node) if is_dataclass(node) else dict(node)
    return _strip_volatile(d)


def normalise_edge(edge: Any) -> dict[str, Any]:
    d = asdict(edge) if is_dataclass(edge) else dict(edge)
    return _strip_volatile(d)


def _sort_key(d: dict[str, Any]) -> str:
    """Stable sort key for normalised nodes/edges. We sort by the
    canonical payload because node_id is stripped and payloads carry
    the semantic identity (email / sku / order_number)."""
    import json

    return json.dumps(d, sort_keys=True, default=str)


def normalise_node_list(nodes: list[Any]) -> list[dict[str, Any]]:
    return sorted((normalise_node(n) for n in nodes), key=_sort_key)


def normalise_edge_list(edges: list[Any]) -> list[dict[str, Any]]:
    return sorted((normalise_edge(e) for e in edges), key=_sort_key)


# ---------------------------------------------------------------------------
# State snapshot + equivalence assertion
# ---------------------------------------------------------------------------


async def _snapshot(client: DbClient, tenant_id: str) -> dict[str, Any]:
    """Snapshot the parity-relevant slice of a server's state for one
    tenant: every User / Product / Order node + every outgoing edge
    from every node. Returns a normalised, sort-stable structure.
    """
    scope = client.tenant(tenant_id).actor(ACTOR)

    users = await scope.query(pb.User, limit=500, order_by="created_at", descending=False)
    products = await scope.query(pb.Product, limit=500, order_by="created_at", descending=False)
    orders = await scope.query(pb.Order, limit=500, order_by="created_at", descending=False)

    # Edges: walk every source node and collect outgoing.
    edges: list[Any] = []
    for src in [*users, *products, *orders]:
        for edge_type in (pb.Purchased, pb.PlacedOrder, pb.OrderContains):
            try:
                edges.extend(await scope.edges_out(src.node_id, edge_type=edge_type))
            except Exception:
                # An edge type may legitimately not exist for a given
                # source type — skip rather than fail the parity
                # comparison, both servers should respond identically
                # (either succeed-empty or 404).
                pass

    return {
        "users": normalise_node_list(users),
        "products": normalise_node_list(products),
        "orders": normalise_node_list(orders),
        "edges": normalise_edge_list(edges),
    }


async def assert_state_equivalent(
    python_client: DbClient,
    go_client: DbClient,
    tenant_id: str,
) -> None:
    """Snapshot both servers and diff their normalised state.

    Raises ``AssertionError`` with a human-readable diff on mismatch.
    """
    py_state = await _snapshot(python_client, tenant_id)
    go_state = await _snapshot(go_client, tenant_id)

    if py_state == go_state:
        return

    import json

    py_dump = json.dumps(py_state, indent=2, sort_keys=True, default=str)
    go_dump = json.dumps(go_state, indent=2, sort_keys=True, default=str)
    diff_lines: list[str] = []
    for key in sorted(set(py_state.keys()) | set(go_state.keys())):
        if py_state.get(key) != go_state.get(key):
            diff_lines.append(f"  - section {key!r} differs:")
            diff_lines.append(f"    python: {py_state.get(key)!r}")
            diff_lines.append(f"    go:     {go_state.get(key)!r}")
    diff = "\n".join(diff_lines)
    raise AssertionError(
        "Python and Go servers disagree on tenant state:\n"
        + diff
        + "\n\n--- python full state ---\n"
        + py_dump
        + "\n--- go full state ---\n"
        + go_dump
    )


def normalise_commit_result(result: Any) -> dict[str, Any]:
    """Reduce a CommitResult to its parity-comparable form."""
    d = asdict(result) if is_dataclass(result) else dict(result)
    # `created_node_ids` is a list of generated UUIDs — collapse to
    # `["<UUID>"] * n` so the cardinality is preserved but the
    # specific ids (which legitimately differ) don't break parity.
    created = d.get("created_node_ids") or []
    d["created_node_ids"] = ["<UUID>"] * len(created)
    return _strip_volatile(d)


def assert_receipts_equivalent(
    python_results: list[Any],
    go_results: list[Any],
) -> None:
    """Diff a list of commit results from each server. Position-
    sensitive: we compare result[i] against result[i] because the
    parity scenarios fire the same ordered sequence at each server.
    """
    assert len(python_results) == len(go_results), (
        f"receipt count mismatch: python={len(python_results)} go={len(go_results)}"
    )
    for i, (py, go) in enumerate(zip(python_results, go_results, strict=True)):
        py_n = normalise_commit_result(py)
        go_n = normalise_commit_result(go)
        assert py_n == go_n, f"receipt {i} differs:\n  python={py_n!r}\n  go={go_n!r}"


# ---------------------------------------------------------------------------
# Replay helper
# ---------------------------------------------------------------------------


async def replay_against_both(
    python_client: DbClient,
    go_client: DbClient,
    tenant_id: str,
    sequence: Callable[[DbClient, str], Awaitable[list[Any]]],
) -> tuple[list[Any], list[Any]]:
    """Run the same async-callable sequence against each server and
    return ``(python_results, go_results)``.

    The sequence is invoked with ``(client, tenant_id)`` so it can use
    any combination of plan-commit, share/revoke, query, etc. The
    parity test asserts on the returned results + on
    ``assert_state_equivalent``.
    """
    py = await sequence(python_client, tenant_id)
    go = await sequence(go_client, tenant_id)
    return py, go
