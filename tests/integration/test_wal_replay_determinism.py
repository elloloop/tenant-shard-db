# SPDX-License-Identifier: AGPL-3.0-only
"""Determinism of WAL → SQLite replay.

EntDB is event-sourced: the WAL is the source of truth and SQLite is a
materialised view rebuilt by replaying it. These tests are the bedrock
contract for any reimplementation of the applier (Python or Go): the
state produced by replaying a WAL from offset 0 must be byte-for-byte
identical to the state produced by applying the same events live.

Coverage:
    1. Round-trip determinism — drive a mix of node/edge/admin events
       through the applier, snapshot the canonical store, delete the
       SQLite files, replay from offset 0 with a fresh applier, and
       assert the canonical dump matches.
    2. Idempotency on replay — replaying an event a second time
       (same idempotency_key) must not produce duplicates.
    3. Halt on a poisoned event — a malformed event mid-WAL must stop
       the applier at that offset; the WAL must NOT advance past it
       (matches the halt_on_error invariant).
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
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import EdgeTypeDef, NodeTypeDef, field
from dbaas.entdb_server.wal.memory import InMemoryWalStream


def _build_registry() -> SchemaRegistry:
    """Build a small registry: User (1), Task (2), AssignedTo edge (100)."""
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


def _make_event(
    tenant_id: str,
    actor: str,
    idempotency_key: str,
    ops: list,
) -> TransactionEvent:
    return TransactionEvent(
        tenant_id=tenant_id,
        actor=actor,
        idempotency_key=idempotency_key,
        schema_fingerprint=None,
        ts_ms=int(time.time() * 1000),
        ops=ops,
    )


async def _wal_append_event(wal: InMemoryWalStream, topic: str, event_dict: dict) -> None:
    """Append a TransactionEvent dict to the WAL using tenant_id as key."""
    await wal.append(
        topic,
        event_dict["tenant_id"],
        json.dumps(event_dict).encode("utf-8"),
    )


async def _drive_applier_until_idle(
    applier: Applier,
    wal: InMemoryWalStream,
    canonical_store: CanonicalStore,
    tenant_id: str,
    expected_idempotency_keys: list[str],
    timeout: float = 5.0,
) -> None:
    """Wait until every expected idempotency_key is recorded as applied."""
    deadline = asyncio.get_event_loop().time() + timeout
    while asyncio.get_event_loop().time() < deadline:
        if await canonical_store.tenant_exists(tenant_id):
            applied_all = True
            for key in expected_idempotency_keys:
                if not await canonical_store.check_idempotency(tenant_id, key):
                    applied_all = False
                    break
            if applied_all:
                return
        await asyncio.sleep(0.05)
    raise AssertionError(
        f"Applier never caught up: expected keys {expected_idempotency_keys} "
        f"to be applied within {timeout}s"
    )


async def _canonical_dump(
    store: CanonicalStore,
    tenant_id: str,
) -> dict:
    """Deterministic snapshot of every observable state for one tenant.

    A Go applier replaying the same WAL must produce exactly this
    structure for the test to pass — that's the whole point of the
    determinism contract.
    """
    nodes_t1 = await store.get_nodes_by_type(tenant_id, type_id=1, limit=10000)
    nodes_t2 = await store.get_nodes_by_type(tenant_id, type_id=2, limit=10000)

    def _node_repr(n) -> dict:
        return {
            "node_id": n.node_id,
            "type_id": n.type_id,
            "owner_actor": n.owner_actor,
            # payload_json is the raw on-disk form; the determinism
            # contract is that this string round-trips byte-for-byte.
            "payload_json": n.payload_json,
            "acl_json": n.acl_json,
        }

    nodes = sorted(
        [_node_repr(n) for n in (*nodes_t1, *nodes_t2)],
        key=lambda r: r["node_id"],
    )

    # Edges: gather via get_edges_from on every node we know about.
    edges: list[dict] = []
    for n in (*nodes_t1, *nodes_t2):
        out = await store.get_edges_from(tenant_id, n.node_id)
        for e in out:
            edges.append(
                {
                    "edge_type_id": e.edge_type_id,
                    "from_node_id": e.from_node_id,
                    "to_node_id": e.to_node_id,
                    "props": dict(sorted(e.props.items())),
                }
            )
    edges.sort(key=lambda e: (e["edge_type_id"], e["from_node_id"], e["to_node_id"]))

    return {"nodes": nodes, "edges": edges}


@pytest.fixture
def registry():
    return _build_registry()


@pytest.fixture
def schema_registry_global(registry):
    """Install the registry as the global one so the applier can find it."""
    from dbaas.entdb_server.schema import registry as registry_mod

    prev = registry_mod._global_registry
    registry_mod._global_registry = registry
    try:
        yield registry
    finally:
        registry_mod._global_registry = prev


async def test_wal_replay_produces_identical_state(schema_registry_global):
    """Replay from offset 0 reconstructs identical canonical state."""
    tenant_id = "tenant_replay"
    topic = "entdb-wal-replay"

    with tempfile.TemporaryDirectory() as tmpdir:
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        canonical = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical.initialize_tenant(tenant_id)

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic=topic,
            group_id="applier-1",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier_task = asyncio.create_task(applier.start())

        idempotency_keys: list[str] = []
        # Build a deterministic stream of mixed ops. Use stable
        # node_ids so the dump is comparable across runs.
        events_to_drive = [
            {
                "tenant_id": tenant_id,
                "actor": "user:alice",
                "idempotency_key": "k1-create-user-1",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_000_000,
                "ops": [
                    {
                        "op": "create_node",
                        "id": "u1",
                        "type_id": 1,
                        "data": {"1": "alice@example.com", "2": "Alice"},
                    }
                ],
            },
            {
                "tenant_id": tenant_id,
                "actor": "user:alice",
                "idempotency_key": "k2-create-user-2",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_001_000,
                "ops": [
                    {
                        "op": "create_node",
                        "id": "u2",
                        "type_id": 1,
                        "data": {"1": "bob@example.com", "2": "Bob"},
                    }
                ],
            },
            {
                "tenant_id": tenant_id,
                "actor": "user:alice",
                "idempotency_key": "k3-create-task",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_002_000,
                "ops": [
                    {
                        "op": "create_node",
                        "id": "t1",
                        "type_id": 2,
                        "data": {"1": "Fix bug", "2": "Important"},
                    }
                ],
            },
            {
                "tenant_id": tenant_id,
                "actor": "user:alice",
                "idempotency_key": "k4-update-task",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_003_000,
                "ops": [
                    {
                        "op": "update_node",
                        "id": "t1",
                        "type_id": 2,
                        "patch": {"2": "Critical"},
                    }
                ],
            },
            {
                "tenant_id": tenant_id,
                "actor": "user:alice",
                "idempotency_key": "k5-create-edge",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_004_000,
                "ops": [
                    {
                        "op": "create_edge",
                        "edge_id": 100,
                        "from": {"id": "t1"},
                        "to": {"id": "u1"},
                        "props": {"weight": 1},
                    }
                ],
            },
            {
                "tenant_id": tenant_id,
                "actor": "user:alice",
                "idempotency_key": "k6-delete-edge",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_005_000,
                "ops": [
                    {
                        "op": "delete_edge",
                        "edge_id": 100,
                        "from": {"id": "t1"},
                        "to": {"id": "u1"},
                    }
                ],
            },
            {
                "tenant_id": tenant_id,
                "actor": "system:admin",
                "idempotency_key": "k7-admin-transfer",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_006_000,
                "ops": [
                    {
                        "op": "admin_transfer_content",
                        "from_user": "user:alice",
                        "to_user": "user:bob",
                    }
                ],
            },
            {
                "tenant_id": tenant_id,
                "actor": "user:bob",
                "idempotency_key": "k8-create-edge-after-transfer",
                "schema_fingerprint": None,
                "ts_ms": 1_700_000_007_000,
                "ops": [
                    {
                        "op": "create_edge",
                        "edge_id": 100,
                        "from": {"id": "t1"},
                        "to": {"id": "u2"},
                        "props": {"weight": 5},
                    }
                ],
            },
        ]

        for ev in events_to_drive:
            idempotency_keys.append(ev["idempotency_key"])
            await _wal_append_event(wal, topic, ev)

        await _drive_applier_until_idle(applier, wal, canonical, tenant_id, idempotency_keys)

        before = await _canonical_dump(canonical, tenant_id)

        # Stop the applier and the canonical store; drop the SQLite
        # data on disk. The WAL remains intact in memory.
        await applier.stop()
        applier_task.cancel()
        try:
            await asyncio.wait_for(applier_task, timeout=2.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            pass
        canonical.close_all()

        # Wipe the per-tenant SQLite files (and any -journal/-wal
        # sidecars) so the rebuild starts truly empty.
        import os

        for entry in os.listdir(tmpdir):
            os.remove(os.path.join(tmpdir, entry))

        # Fresh canonical store + fresh applier with a new group_id
        # so the InMemoryWalStream replays from offset 0.
        canonical2 = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical2.initialize_tenant(tenant_id)
        applier2 = Applier(
            wal=wal,
            canonical_store=canonical2,
            topic=topic,
            group_id="applier-2-replay",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier2_task = asyncio.create_task(applier2.start())

        await _drive_applier_until_idle(applier2, wal, canonical2, tenant_id, idempotency_keys)

        after = await _canonical_dump(canonical2, tenant_id)

        assert after == before, (
            f"WAL replay did not reproduce identical state.\nBEFORE: {before}\nAFTER: {after}"
        )

        await applier2.stop()
        applier2_task.cancel()
        try:
            await asyncio.wait_for(applier2_task, timeout=2.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            pass
        canonical2.close_all()
        await wal.close()


async def test_replay_is_idempotent_on_duplicate_event(schema_registry_global):
    """The same idempotency_key applied twice produces one node, not two."""
    tenant_id = "tenant_idem"
    topic = "entdb-idem"

    with tempfile.TemporaryDirectory() as tmpdir:
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        canonical = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical.initialize_tenant(tenant_id)

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic=topic,
            group_id="applier-idem",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier_task = asyncio.create_task(applier.start())

        ev = {
            "tenant_id": tenant_id,
            "actor": "user:alice",
            "idempotency_key": "DUPLICATE_KEY",
            "schema_fingerprint": None,
            "ts_ms": 1_700_000_000_000,
            "ops": [
                {
                    "op": "create_node",
                    "id": "u-dup",
                    "type_id": 1,
                    "data": {"1": "dup@example.com", "2": "Dup"},
                }
            ],
        }
        # Append the SAME event twice.
        await _wal_append_event(wal, topic, ev)
        await _wal_append_event(wal, topic, ev)

        await _drive_applier_until_idle(applier, wal, canonical, tenant_id, ["DUPLICATE_KEY"])
        # Give the second copy a chance to process.
        await asyncio.sleep(0.2)

        nodes = await canonical.get_nodes_by_type(tenant_id, type_id=1)
        assert len(nodes) == 1, f"Expected exactly one node after duplicate event; got {len(nodes)}"

        await applier.stop()
        applier_task.cancel()
        try:
            await asyncio.wait_for(applier_task, timeout=2.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            pass
        canonical.close_all()
        await wal.close()


async def test_applier_halts_on_poisoned_event(schema_registry_global):
    """A poisoned event mid-WAL halts the applier; later events are NOT applied.

    Mirrors finding #2 of the recent security audit: the WAL offset
    must NEVER advance past a failed event. The applier sets
    halt_on_error=True by default — the second create_node here is
    invalid (mailbox-mode mixing) and ``_event_storage_mode`` raises
    a ``ValidationError``.
    """
    tenant_id = "tenant_halt"
    topic = "entdb-halt"

    with tempfile.TemporaryDirectory() as tmpdir:
        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        canonical = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical.initialize_tenant(tenant_id)

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic=topic,
            group_id="applier-halt",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
            halt_on_error=True,
        )
        applier_task = asyncio.create_task(applier.start())

        good_event = {
            "tenant_id": tenant_id,
            "actor": "user:alice",
            "idempotency_key": "good-1",
            "schema_fingerprint": None,
            "ts_ms": 1_700_000_000_000,
            "ops": [
                {
                    "op": "create_node",
                    "id": "good-node",
                    "type_id": 1,
                    "data": {"1": "ok@example.com", "2": "OK"},
                }
            ],
        }
        # Poisoned: USER_MAILBOX without target_user_id triggers
        # ValidationError in ``_event_storage_mode``.
        poisoned_event = {
            "tenant_id": tenant_id,
            "actor": "user:alice",
            "idempotency_key": "poisoned-1",
            "schema_fingerprint": None,
            "ts_ms": 1_700_000_001_000,
            "ops": [
                {
                    "op": "create_node",
                    "id": "bad-node",
                    "type_id": 1,
                    "data": {"1": "bad@example.com", "2": "Bad"},
                    "storage_mode": "USER_MAILBOX",
                    # target_user_id deliberately omitted.
                }
            ],
        }
        after_event = {
            "tenant_id": tenant_id,
            "actor": "user:alice",
            "idempotency_key": "after-1",
            "schema_fingerprint": None,
            "ts_ms": 1_700_000_002_000,
            "ops": [
                {
                    "op": "create_node",
                    "id": "after-node",
                    "type_id": 1,
                    "data": {"1": "after@example.com", "2": "After"},
                }
            ],
        }

        await _wal_append_event(wal, topic, good_event)
        await _wal_append_event(wal, topic, poisoned_event)
        await _wal_append_event(wal, topic, after_event)

        # Wait for the good event to be applied.
        await _drive_applier_until_idle(applier, wal, canonical, tenant_id, ["good-1"])

        # Give the applier time to halt on the poisoned event.
        await asyncio.sleep(0.5)

        # The good event was applied; the after event must NOT have
        # been (offset frozen at the poisoned record).
        assert await canonical.check_idempotency(tenant_id, "good-1")
        assert not await canonical.check_idempotency(tenant_id, "poisoned-1")
        assert not await canonical.check_idempotency(tenant_id, "after-1"), (
            "Applier must NOT advance past a poisoned event — finding #2"
        )

        # Cleanup. The applier task should already be done because
        # halt_on_error=True ends the loop on a failed apply.
        await applier.stop()
        applier_task.cancel()
        try:
            await asyncio.wait_for(applier_task, timeout=2.0)
        except (asyncio.CancelledError, asyncio.TimeoutError):
            pass
        canonical.close_all()
        await wal.close()
