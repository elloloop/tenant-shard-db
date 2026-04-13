"""
Unit tests for storage routing (2026-04-13 storage decision).

Covers the mailbox + public storage modes, the edge direction
invariant, the immutable ``storage_mode`` rule, and the GDPR mailbox
delete pipeline.

See ``docs/decisions/storage.md`` for the decision document.
"""

from __future__ import annotations

from typing import Any

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.gdpr_worker import GdprDeletionWorker
from dbaas.entdb_server.wal.memory import InMemoryWalStream

# ────────────────────────────────────────────────────────────────────
# Helpers
# ────────────────────────────────────────────────────────────────────


async def _make_applier(tmp_path, tenant_id: str = "t1") -> Applier:
    wal = InMemoryWalStream(num_partitions=1)
    store = CanonicalStore(str(tmp_path / "canonical"))
    await store.initialize_tenant(tenant_id)
    applier = Applier(
        wal=wal,
        canonical_store=store,
        topic="t",
        fanout_config=MailboxFanoutConfig(enabled=False),
        halt_on_error=False,
    )
    return applier


def _make_event(
    tenant_id: str,
    idempotency_key: str,
    ops: list[dict[str, Any]],
    actor: str = "user:alice",
) -> TransactionEvent:
    return TransactionEvent(
        tenant_id=tenant_id,
        actor=actor,
        idempotency_key=idempotency_key,
        schema_fingerprint=None,
        ts_ms=1000,
        ops=ops,
    )


def _create_op(
    node_id: str,
    *,
    type_id: int = 1,
    storage_mode: str | None = None,
    target_user_id: str | None = None,
    data: dict | None = None,
) -> dict[str, Any]:
    op: dict[str, Any] = {
        "op": "create_node",
        "type_id": type_id,
        "id": node_id,
        "data": data or {"title": node_id},
    }
    if storage_mode is not None:
        op["storage_mode"] = storage_mode
    if target_user_id is not None:
        op["target_user_id"] = target_user_id
    return op


def _edge_op(from_id: str, to_id: str) -> dict[str, Any]:
    return {
        "op": "create_edge",
        "edge_id": 1,
        "from": from_id,
        "to": to_id,
    }


# ────────────────────────────────────────────────────────────────────
# 1-4: Basic routing to the right physical file
# ────────────────────────────────────────────────────────────────────


@pytest.mark.unit
class TestBasicRouting:
    @pytest.mark.asyncio
    async def test_default_create_lands_in_tenant_db(self, tmp_path):
        applier = await _make_applier(tmp_path)
        ev = _make_event("t1", "ev-1", [_create_op("n-1")])
        result = await applier.apply_event(ev)
        assert result.success

        tenant_db = applier.canonical_store._get_db_path("t1")
        assert tenant_db.exists()
        mailbox_db = applier.canonical_store._get_mailbox_db_path("t1", "alice")
        assert not mailbox_db.exists()
        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_explicit_tenant_lands_in_tenant_db(self, tmp_path):
        applier = await _make_applier(tmp_path)
        ev = _make_event(
            "t1",
            "ev-2",
            [_create_op("n-1", storage_mode="TENANT")],
        )
        result = await applier.apply_event(ev)
        assert result.success

        tenant_db = applier.canonical_store._get_db_path("t1")
        assert tenant_db.exists()
        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_user_mailbox_routes_to_per_user_file(self, tmp_path):
        applier = await _make_applier(tmp_path)
        ev = _make_event(
            "t1",
            "ev-3",
            [
                _create_op(
                    "n-1",
                    storage_mode="USER_MAILBOX",
                    target_user_id="alice",
                )
            ],
        )
        result = await applier.apply_event(ev)
        assert result.success, result.error

        mailbox_db = applier.canonical_store._get_mailbox_db_path("t1", "alice")
        assert mailbox_db.exists()
        # Row must actually be in the mailbox file.
        with applier.canonical_store._get_mailbox_connection("t1", "alice", create=False) as conn:
            row = conn.execute("SELECT node_id FROM nodes WHERE node_id = ?", ("n-1",)).fetchone()
            assert row is not None
        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_public_routes_to_public_db(self, tmp_path):
        applier = await _make_applier(tmp_path)
        ev = _make_event(
            "t1",
            "ev-4",
            [_create_op("pub-1", storage_mode="PUBLIC")],
        )
        result = await applier.apply_event(ev)
        assert result.success, result.error

        public_db = applier.canonical_store._get_public_db_path()
        assert public_db.exists()
        with applier.canonical_store._get_public_connection(create=False) as conn:
            row = conn.execute("SELECT node_id FROM nodes WHERE node_id = ?", ("pub-1",)).fetchone()
            assert row is not None
        applier.canonical_store.close_all()


# ────────────────────────────────────────────────────────────────────
# 5-6: Validation of required fields / immutability
# ────────────────────────────────────────────────────────────────────


@pytest.mark.unit
class TestValidation:
    @pytest.mark.asyncio
    async def test_user_mailbox_without_target_user_rejected(self, tmp_path):
        applier = await _make_applier(tmp_path)
        ev = _make_event(
            "t1",
            "ev-5",
            [_create_op("n-1", storage_mode="USER_MAILBOX")],
        )
        result = await applier.apply_event(ev)
        assert not result.success
        assert "target_user_id" in (result.error or "")
        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_update_node_with_storage_mode_rejected(self, tmp_path):
        applier = await _make_applier(tmp_path)
        # Create a node first
        ev1 = _make_event("t1", "ev-6a", [_create_op("n-1")])
        r1 = await applier.apply_event(ev1)
        assert r1.success

        # Attempt to flip storage_mode via update_node
        ev2 = _make_event(
            "t1",
            "ev-6b",
            [
                {
                    "op": "update_node",
                    "type_id": 1,
                    "id": "n-1",
                    "patch": {"storage_mode": "USER_MAILBOX"},
                }
            ],
        )
        r2 = await applier.apply_event(ev2)
        assert not r2.success
        assert "immutable" in (r2.error or "")
        applier.canonical_store.close_all()


# ────────────────────────────────────────────────────────────────────
# 7-13: Edge direction invariant
# ────────────────────────────────────────────────────────────────────


@pytest.mark.unit
class TestEdgeDirectionInvariant:
    def _storage_map(
        self, entries: list[tuple[str, str, str | None]]
    ) -> dict[str, tuple[str, str | None]]:
        return {nid: (mode, user) for nid, mode, user in entries}

    def test_tenant_to_tenant_allowed(self):
        m = self._storage_map([("a", "TENANT", None), ("b", "TENANT", None)])
        CanonicalStore._validate_edge_direction("a", "b", "t1", m)

    def test_tenant_to_public_allowed(self):
        m = self._storage_map([("a", "TENANT", None), ("b", "PUBLIC", None)])
        CanonicalStore._validate_edge_direction("a", "b", "t1", m)

    def test_mailbox_to_tenant_allowed(self):
        m = self._storage_map([("a", "USER_MAILBOX", "alice"), ("b", "TENANT", None)])
        CanonicalStore._validate_edge_direction("a", "b", "t1", m)

    def test_mailbox_to_public_allowed(self):
        m = self._storage_map([("a", "USER_MAILBOX", "alice"), ("b", "PUBLIC", None)])
        CanonicalStore._validate_edge_direction("a", "b", "t1", m)

    def test_tenant_to_mailbox_rejected(self):
        m = self._storage_map([("a", "TENANT", None), ("b", "USER_MAILBOX", "alice")])
        with pytest.raises(ValueError, match="hierarchy"):
            CanonicalStore._validate_edge_direction("a", "b", "t1", m)

    def test_public_to_tenant_rejected(self):
        m = self._storage_map([("a", "PUBLIC", None), ("b", "TENANT", None)])
        with pytest.raises(ValueError, match="hierarchy"):
            CanonicalStore._validate_edge_direction("a", "b", "t1", m)

    def test_cross_user_mailbox_rejected(self):
        m = self._storage_map(
            [
                ("a", "USER_MAILBOX", "alice"),
                ("b", "USER_MAILBOX", "bob"),
            ]
        )
        with pytest.raises(ValueError, match="different user mailboxes"):
            CanonicalStore._validate_edge_direction("a", "b", "t1", m)


# ────────────────────────────────────────────────────────────────────
# 14-15: GDPR delete + delete_user_drafts
# ────────────────────────────────────────────────────────────────────


@pytest.mark.unit
class TestGdprMailboxDelete:
    @pytest.mark.asyncio
    async def test_delete_user_mailbox_removes_file(self, tmp_path):
        applier = await _make_applier(tmp_path)

        # Put a node into alice's mailbox
        ev = _make_event(
            "t1",
            "ev-14",
            [
                _create_op(
                    "n-mail",
                    storage_mode="USER_MAILBOX",
                    target_user_id="alice",
                )
            ],
        )
        result = await applier.apply_event(ev)
        assert result.success

        mailbox_db = applier.canonical_store._get_mailbox_db_path("t1", "alice")
        assert mailbox_db.exists()

        class _NullGlobal:
            pass

        worker = GdprDeletionWorker(
            _NullGlobal(),
            applier.canonical_store,
            schema_registry=None,
        )
        removed = await worker._delete_user_mailbox("t1", "alice")
        assert removed is True
        assert not mailbox_db.exists()
        applier.canonical_store.close_all()

    @pytest.mark.asyncio
    async def test_delete_user_drafts_only_removes_drafts(self, tmp_path):
        applier = await _make_applier(tmp_path)
        # Draft node (any field value of "draft" is treated as draft)
        ev1 = _make_event(
            "t1",
            "ev-15a",
            [_create_op("n-draft", data={"title": "D", "1": "draft"})],
            actor="user:alice",
        )
        # Non-draft node
        ev2 = _make_event(
            "t1",
            "ev-15b",
            [_create_op("n-pub", data={"title": "P", "1": "published"})],
            actor="user:alice",
        )
        r1 = await applier.apply_event(ev1)
        r2 = await applier.apply_event(ev2)
        assert r1.success and r2.success

        deleted = await applier.canonical_store.delete_user_drafts("t1", "alice")
        assert deleted == 1

        with applier.canonical_store._get_connection("t1") as conn:
            rows = conn.execute("SELECT node_id FROM nodes WHERE tenant_id = ?", ("t1",)).fetchall()
        remaining = {r[0] for r in rows}
        assert "n-pub" in remaining
        assert "n-draft" not in remaining
        applier.canonical_store.close_all()


# ────────────────────────────────────────────────────────────────────
# 16: Mixed storage mode in one event
# ────────────────────────────────────────────────────────────────────


@pytest.mark.unit
class TestMixedStorageMode:
    @pytest.mark.asyncio
    async def test_mixed_modes_in_one_event_rejected(self, tmp_path):
        applier = await _make_applier(tmp_path)
        ev = _make_event(
            "t1",
            "ev-16",
            [
                _create_op("n-t", storage_mode="TENANT"),
                _create_op("n-p", storage_mode="PUBLIC"),
            ],
        )
        result = await applier.apply_event(ev)
        assert not result.success
        assert "mixes storage modes" in (result.error or "")
        applier.canonical_store.close_all()
