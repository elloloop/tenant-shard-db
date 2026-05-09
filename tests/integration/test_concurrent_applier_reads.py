# SPDX-License-Identifier: AGPL-3.0-only
"""Concurrent reader + applier safety.

Drives the canonical store with a burst of writes through the applier
while many reader tasks query nodes/edges in tight loops. Asserts:

    1. No reader observes a half-applied transaction
       (a Node referenced by an Edge but missing from ``get_node``).
    2. No deadlock — the test must complete well under timeout.
    3. Read-after-write consistency: every committed write is visible
       in a final post-test read.

Why: Python's GIL + asyncio cooperative scheduling hide many
ordering bugs that would surface under a Go applier with real
preemption. Forcing this test to pass under high concurrency makes
the contract explicit.
"""

from __future__ import annotations

import asyncio
import json
import tempfile
import time

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import EdgeTypeDef, NodeTypeDef, field
from dbaas.entdb_server.wal.memory import InMemoryWalStream

TENANT = "tenant_concurrent"
TOPIC = "concurrent-wal"


def _build_registry() -> SchemaRegistry:
    reg = SchemaRegistry()
    reg.register_node_type(
        NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str"), field(2, "name", "str")),
        )
    )
    reg.register_node_type(
        NodeTypeDef(
            type_id=2,
            name="Task",
            fields=(field(1, "title", "str"), field(2, "description", "str")),
        )
    )
    reg.register_edge_type(EdgeTypeDef(edge_id=100, name="AssignedTo", from_type=2, to_type=1))
    return reg


@pytest.fixture
def schema_registry_global():
    from dbaas.entdb_server.schema import registry as registry_mod

    prev = registry_mod._global_registry
    reg = _build_registry()
    registry_mod._global_registry = reg
    try:
        yield reg
    finally:
        registry_mod._global_registry = prev


async def _append_event(
    wal: InMemoryWalStream,
    topic: str,
    tenant_id: str,
    actor: str,
    idem_key: str,
    ops: list[dict],
) -> None:
    event = {
        "tenant_id": tenant_id,
        "actor": actor,
        "idempotency_key": idem_key,
        "schema_fingerprint": None,
        "ts_ms": int(time.time() * 1000),
        "ops": ops,
    }
    await wal.append(topic, tenant_id, json.dumps(event).encode("utf-8"))


async def _reader_loop(
    canonical: CanonicalStore,
    tenant_id: str,
    stop: asyncio.Event,
    observed_invariant_violations: list[str],
    max_iterations: int = 5000,
) -> int:
    """Runs until ``stop`` is set or it has done ``max_iterations`` cycles.

    For every iteration:
      - List type-2 (Task) nodes. For each task, list its outgoing
        edges. Every ``to_node_id`` must resolve to a real node, and
        every ``from_node_id`` must equal the task we asked about.
      - List type-1 (User) nodes; assert each carries a parseable
        payload (no half-written rows).
    """
    iters = 0
    while not stop.is_set() and iters < max_iterations:
        iters += 1
        try:
            tasks = await canonical.get_nodes_by_type(tenant_id, type_id=2, limit=10000)
            for t in tasks:
                # Sanity: payload is a dict (not a half-written str).
                assert isinstance(t.payload, dict)
                edges = await canonical.get_edges_from(tenant_id, t.node_id)
                for e in edges:
                    assert e.from_node_id == t.node_id
                    # The Edge points at a User; ``get_node`` must
                    # return it (the create_node committed in the
                    # SAME transaction as create_edge, so a half-
                    # applied state would surface here).
                    target = await canonical.get_node(tenant_id, e.to_node_id)
                    if target is None:
                        observed_invariant_violations.append(
                            f"Edge {t.node_id}->{e.to_node_id} references missing node"
                        )
            users = await canonical.get_nodes_by_type(tenant_id, type_id=1, limit=10000)
            for u in users:
                assert isinstance(u.payload, dict)
        except Exception as ex:
            observed_invariant_violations.append(f"reader exception: {ex!r}")
        # Yield generously so the applier and other readers progress.
        await asyncio.sleep(0)
    return iters


async def test_concurrent_reads_during_applier_burst(schema_registry_global):
    """10 readers + 100 writes; readers must never see torn state."""
    with tempfile.TemporaryDirectory() as tmpdir:
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        canonical = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical.initialize_tenant(TENANT)

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic=TOPIC,
            group_id="concurrent-applier",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier_task = asyncio.create_task(applier.start())

        stop_readers = asyncio.Event()
        violations: list[str] = []

        readers = [
            asyncio.create_task(_reader_loop(canonical, TENANT, stop_readers, violations))
            for _ in range(10)
        ]

        # Pre-create users that tasks will reference. This makes the
        # half-applied assertion sharper: when a multi-op event
        # commits ``create_task + create_edge`` together, the edge
        # must not be visible until the task is.
        for i in range(20):
            await _append_event(
                wal,
                TOPIC,
                TENANT,
                f"user:author{i}",
                f"user-create-{i}",
                [
                    {
                        "op": "create_node",
                        "id": f"user-{i}",
                        "type_id": 1,
                        "data": {
                            "1": f"u{i}@example.com",
                            "2": f"User {i}",
                        },
                    }
                ],
            )

        # Wait briefly for users to be applied (so their ids resolve).
        deadline = time.time() + 5.0
        while time.time() < deadline:
            users = await canonical.get_nodes_by_type(TENANT, type_id=1, limit=1000)
            if len(users) >= 20:
                break
            await asyncio.sleep(0.05)
        else:
            pytest.fail("Pre-create did not complete")

        # Now interleave: 100 multi-op transactions, each creates a
        # task and an edge to one of the existing users. The applier
        # must commit (task + edge) atomically.
        committed_idem_keys: list[str] = []
        for i in range(100):
            idem = f"task-{i}"
            committed_idem_keys.append(idem)
            user_idx = i % 20
            await _append_event(
                wal,
                TOPIC,
                TENANT,
                f"user:author{user_idx}",
                idem,
                [
                    {
                        "op": "create_node",
                        "id": f"task-{i}",
                        "type_id": 2,
                        "data": {
                            "1": f"Task {i}",
                            "2": f"Description for task {i}",
                        },
                    },
                    {
                        "op": "create_edge",
                        "edge_id": 100,
                        "from": {"id": f"task-{i}"},
                        "to": {"id": f"user-{user_idx}"},
                        "props": {"i": i},
                    },
                ],
            )
            # Tiny yield so readers run between writes.
            if i % 5 == 0:
                await asyncio.sleep(0)

        # Wait for every committed event to be applied. 30s is the
        # outer test timeout; we want to comfortably finish by 10s.
        wait_deadline = time.time() + 20.0
        while time.time() < wait_deadline:
            applied_all = True
            for k in committed_idem_keys:
                if not await canonical.check_idempotency(TENANT, k):
                    applied_all = False
                    break
            if applied_all:
                break
            await asyncio.sleep(0.05)
        else:
            pytest.fail(
                "Applier did not catch up to all 100 writes in 20s — "
                "possible deadlock or stalled reader interference"
            )

        # Stop readers and gather iteration counts.
        stop_readers.set()
        iter_counts = await asyncio.gather(*readers)

        assert violations == [], f"Concurrent readers observed invariant violations: {violations}"
        # Every reader must have run at least once; otherwise we
        # accidentally serialised reads behind writes and the test
        # is meaningless.
        assert all(c > 0 for c in iter_counts), f"At least one reader never iterated: {iter_counts}"
        # Sanity: total reader work was non-trivial. 10 readers
        # should jointly complete >= 50 iterations.
        assert sum(iter_counts) >= 50, (
            f"Total reader iterations too low: {iter_counts}; "
            "the test is not actually exercising concurrency"
        )

        # Read-after-write consistency: every task and edge must
        # be observable now.
        final_tasks = await canonical.get_nodes_by_type(TENANT, type_id=2, limit=10000)
        assert len(final_tasks) == 100, f"Expected 100 tasks, got {len(final_tasks)}"
        # Spot-check edge visibility for a sample.
        for i in (0, 50, 99):
            edges = await canonical.get_edges_from(TENANT, f"task-{i}")
            assert len(edges) == 1, f"task-{i}: expected 1 edge, got {len(edges)}"
            assert edges[0].to_node_id == f"user-{i % 20}"

        await applier.stop()
        applier_task.cancel()
        try:
            await asyncio.wait_for(applier_task, timeout=2.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            pass
        canonical.close_all()
        await wal.close()


async def test_concurrent_test_completes_under_30s(schema_registry_global):
    """Hard deadline: the test above must complete inside the budget.

    Lighter variant — 5 readers + 50 writes — purely as a deadlock
    canary. If this hangs, the heavier test above will too.
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        canonical = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical.initialize_tenant(TENANT)

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic=TOPIC,
            group_id="canary-applier",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier_task = asyncio.create_task(applier.start())

        stop = asyncio.Event()
        violations: list[str] = []
        readers = [
            asyncio.create_task(_reader_loop(canonical, TENANT, stop, violations)) for _ in range(5)
        ]

        keys = []
        start = time.time()
        for i in range(50):
            keys.append(f"canary-{i}")
            await _append_event(
                wal,
                TOPIC,
                TENANT,
                "user:writer",
                f"canary-{i}",
                [
                    {
                        "op": "create_node",
                        "id": f"node-{i}",
                        "type_id": 1,
                        "data": {"1": f"x{i}@e.com", "2": f"X{i}"},
                    }
                ],
            )

        deadline = start + 25.0
        applied_all = False
        while time.time() < deadline:
            applied_all = True
            for k in keys:
                if not await canonical.check_idempotency(TENANT, k):
                    applied_all = False
                    break
            if applied_all:
                break
            await asyncio.sleep(0.05)
        if not applied_all:
            pytest.fail("Canary deadlocked")

        elapsed = time.time() - start
        assert elapsed < 25.0, f"Canary took {elapsed:.2f}s — too slow"

        stop.set()
        await asyncio.gather(*readers)
        assert violations == [], violations

        await applier.stop()
        applier_task.cancel()
        try:
            await asyncio.wait_for(applier_task, timeout=2.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            pass
        canonical.close_all()
        await wal.close()
