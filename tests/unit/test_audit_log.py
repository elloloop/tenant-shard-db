"""
Unit tests for hash-chained audit log.

Tests cover:
- Append single/multiple entries
- Hash chain integrity (prev_hash computation)
- Genesis entry (first in chain)
- Verify chain on valid log
- Tamper detection (modify, delete)
- Query with filters (actor, action, time range)
- Export (chronological, with time bounds)
- Concurrent appends (order preserved)
- Edge cases (empty log, metadata, ip/user-agent)
"""

import asyncio
import sqlite3
import tempfile
import time

import pytest

from dbaas.entdb_server.apply.canonical_store import (
    CanonicalStore,
    _compute_entry_hash,
)


# ── fixtures ──────────────────────────────────────────────────────────

@pytest.fixture()
def store(tmp_path):
    """Create a CanonicalStore in a temp directory."""
    s = CanonicalStore(str(tmp_path))
    yield s
    s.close_all()


@pytest.fixture()
async def tenant_store(store):
    """Store with a tenant already initialized."""
    await store.initialize_tenant("t1")
    return store


TENANT = "t1"


# ── helpers ───────────────────────────────────────────────────────────

async def _append(store, **kwargs):
    """Shorthand for append_audit with defaults."""
    defaults = dict(
        tenant_id=TENANT,
        actor_id="user:alice",
        action="create_node",
        target_type="node",
        target_id="n1",
    )
    defaults.update(kwargs)
    return await store.append_audit(**defaults)


# ── _compute_entry_hash unit tests ────────────────────────────────────

class TestComputeEntryHash:
    def test_deterministic(self):
        """Same inputs always produce the same hash."""
        h1 = _compute_entry_hash("e1", "genesis", "alice", "create", "node", "n1", 1000)
        h2 = _compute_entry_hash("e1", "genesis", "alice", "create", "node", "n1", 1000)
        assert h1 == h2

    def test_different_inputs(self):
        """Changing any field changes the hash."""
        base = _compute_entry_hash("e1", "genesis", "alice", "create", "node", "n1", 1000)
        assert base != _compute_entry_hash("e2", "genesis", "alice", "create", "node", "n1", 1000)
        assert base != _compute_entry_hash("e1", "other", "alice", "create", "node", "n1", 1000)
        assert base != _compute_entry_hash("e1", "genesis", "bob", "create", "node", "n1", 1000)
        assert base != _compute_entry_hash("e1", "genesis", "alice", "delete", "node", "n1", 1000)
        assert base != _compute_entry_hash("e1", "genesis", "alice", "create", "edge", "n1", 1000)
        assert base != _compute_entry_hash("e1", "genesis", "alice", "create", "node", "n2", 1000)
        assert base != _compute_entry_hash("e1", "genesis", "alice", "create", "node", "n1", 2000)

    def test_hash_is_hex_sha256(self):
        """Hash should be 64 hex characters (SHA-256)."""
        h = _compute_entry_hash("e1", "genesis", "a", "b", "c", "d", 0)
        assert len(h) == 64
        assert all(c in "0123456789abcdef" for c in h)


# ── append tests ──────────────────────────────────────────────────────

class TestAppendAudit:
    @pytest.mark.asyncio
    async def test_append_single_entry(self, tenant_store):
        """Appending one entry returns a UUID event_id."""
        eid = await _append(tenant_store)
        assert eid is not None
        assert len(eid) == 36  # UUID format

    @pytest.mark.asyncio
    async def test_genesis_entry_has_genesis_prev_hash(self, tenant_store):
        """First entry must have prev_hash = 'genesis'."""
        await _append(tenant_store)
        log = await tenant_store.get_audit_log(TENANT)
        assert len(log) == 1
        assert log[0]["prev_hash"] == "genesis"

    @pytest.mark.asyncio
    async def test_second_entry_has_correct_prev_hash(self, tenant_store):
        """Second entry's prev_hash == hash of the first entry."""
        await _append(tenant_store, target_id="n1")
        await _append(tenant_store, target_id="n2")

        log = await tenant_store.export_audit_log(TENANT)  # chronological order
        assert len(log) == 2

        first = log[0]
        expected = _compute_entry_hash(
            first["event_id"],
            first["prev_hash"],
            first["actor_id"],
            first["action"],
            first["target_type"],
            first["target_id"],
            first["created_at"],
        )
        assert log[1]["prev_hash"] == expected

    @pytest.mark.asyncio
    async def test_append_multiple_chain(self, tenant_store):
        """Appending N entries forms a valid chain of length N."""
        for i in range(5):
            await _append(tenant_store, target_id=f"n{i}")
        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is True
        assert msg == "ok"

    @pytest.mark.asyncio
    async def test_append_with_optional_fields(self, tenant_store):
        """ip_address, user_agent, metadata are stored correctly."""
        await _append(
            tenant_store,
            ip_address="192.168.1.1",
            user_agent="TestBot/1.0",
            metadata='{"key": "value"}',
        )
        log = await tenant_store.get_audit_log(TENANT)
        entry = log[0]
        assert entry["ip_address"] == "192.168.1.1"
        assert entry["user_agent"] == "TestBot/1.0"
        assert entry["metadata"] == '{"key": "value"}'

    @pytest.mark.asyncio
    async def test_append_optional_fields_default_none(self, tenant_store):
        """Optional fields default to None."""
        await _append(tenant_store)
        log = await tenant_store.get_audit_log(TENANT)
        entry = log[0]
        assert entry["ip_address"] is None
        assert entry["user_agent"] is None
        assert entry["metadata"] is None


# ── verify chain tests ────────────────────────────────────────────────

class TestVerifyAuditChain:
    @pytest.mark.asyncio
    async def test_empty_log_is_valid(self, tenant_store):
        """Empty audit log is considered valid."""
        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is True
        assert msg == "ok"

    @pytest.mark.asyncio
    async def test_valid_chain(self, tenant_store):
        """A properly built chain verifies successfully."""
        for i in range(10):
            await _append(tenant_store, target_id=f"n{i}", actor_id=f"user:{i}")
        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is True

    @pytest.mark.asyncio
    async def test_tamper_prev_hash_detected(self, tenant_store):
        """Modifying prev_hash of an entry breaks the chain."""
        for i in range(3):
            await _append(tenant_store, target_id=f"n{i}")

        # Tamper with the second entry's prev_hash
        log = await tenant_store.export_audit_log(TENANT)
        tampered_id = log[1]["event_id"]

        # Direct DB manipulation to simulate tampering
        db_path = tenant_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute(
            "UPDATE audit_log SET prev_hash = 'tampered' WHERE event_id = ?",
            (tampered_id,),
        )
        conn.commit()
        conn.close()

        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is False
        assert tampered_id in msg

    @pytest.mark.asyncio
    async def test_tamper_actor_detected(self, tenant_store):
        """Modifying actor_id of a prior entry breaks the chain."""
        for i in range(3):
            await _append(tenant_store, target_id=f"n{i}")

        log = await tenant_store.export_audit_log(TENANT)
        first_id = log[0]["event_id"]

        db_path = tenant_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute(
            "UPDATE audit_log SET actor_id = 'hacker' WHERE event_id = ?",
            (first_id,),
        )
        conn.commit()
        conn.close()

        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is False
        # The break is detected at the second entry (whose prev_hash no longer matches)
        assert log[1]["event_id"] in msg

    @pytest.mark.asyncio
    async def test_tamper_action_detected(self, tenant_store):
        """Modifying the action field of a prior entry breaks the chain."""
        for i in range(3):
            await _append(tenant_store, target_id=f"n{i}")

        log = await tenant_store.export_audit_log(TENANT)
        first_id = log[0]["event_id"]

        db_path = tenant_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute(
            "UPDATE audit_log SET action = 'evil_action' WHERE event_id = ?",
            (first_id,),
        )
        conn.commit()
        conn.close()

        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is False

    @pytest.mark.asyncio
    async def test_delete_entry_detected(self, tenant_store):
        """Deleting an entry from the middle breaks the chain."""
        for i in range(4):
            await _append(tenant_store, target_id=f"n{i}")

        log = await tenant_store.export_audit_log(TENANT)
        middle_id = log[1]["event_id"]

        db_path = tenant_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute("DELETE FROM audit_log WHERE event_id = ?", (middle_id,))
        conn.commit()
        conn.close()

        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is False

    @pytest.mark.asyncio
    async def test_tamper_genesis_detected(self, tenant_store):
        """Changing the genesis entry's prev_hash is detected."""
        await _append(tenant_store, target_id="n1")
        await _append(tenant_store, target_id="n2")

        log = await tenant_store.export_audit_log(TENANT)
        genesis_id = log[0]["event_id"]

        db_path = tenant_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute(
            "UPDATE audit_log SET prev_hash = 'not_genesis' WHERE event_id = ?",
            (genesis_id,),
        )
        conn.commit()
        conn.close()

        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is False
        assert "genesis" in msg.lower() or genesis_id in msg


# ── query / filter tests ─────────────────────────────────────────────

class TestGetAuditLog:
    @pytest.mark.asyncio
    async def test_default_order_newest_first(self, tenant_store):
        """get_audit_log returns entries newest-first."""
        e1 = await _append(tenant_store, target_id="n1")
        await asyncio.sleep(0.01)  # ensure distinct timestamps
        e2 = await _append(tenant_store, target_id="n2")

        log = await tenant_store.get_audit_log(TENANT)
        assert log[0]["event_id"] == e2
        assert log[1]["event_id"] == e1

    @pytest.mark.asyncio
    async def test_filter_by_actor(self, tenant_store):
        """Filtering by actor_id returns only matching entries."""
        await _append(tenant_store, actor_id="user:alice")
        await _append(tenant_store, actor_id="user:bob")
        await _append(tenant_store, actor_id="user:alice")

        log = await tenant_store.get_audit_log(TENANT, actor_id="user:bob")
        assert len(log) == 1
        assert log[0]["actor_id"] == "user:bob"

    @pytest.mark.asyncio
    async def test_filter_by_action(self, tenant_store):
        """Filtering by action returns only matching entries."""
        await _append(tenant_store, action="create_node")
        await _append(tenant_store, action="delete_node")
        await _append(tenant_store, action="create_node")

        log = await tenant_store.get_audit_log(TENANT, action="delete_node")
        assert len(log) == 1
        assert log[0]["action"] == "delete_node"

    @pytest.mark.asyncio
    async def test_filter_by_time_range(self, tenant_store):
        """Filtering by after/before returns entries in time range."""
        await _append(tenant_store, target_id="n1")
        mid_ts = int(time.time() * 1000)
        # Small sleep to ensure distinct timestamps
        await asyncio.sleep(0.01)
        await _append(tenant_store, target_id="n2")

        # Entries after mid_ts
        log = await tenant_store.get_audit_log(TENANT, after=mid_ts)
        assert len(log) >= 1
        for entry in log:
            assert entry["created_at"] > mid_ts

    @pytest.mark.asyncio
    async def test_limit_and_offset(self, tenant_store):
        """Limit and offset pagination works."""
        for i in range(5):
            await _append(tenant_store, target_id=f"n{i}")

        page1 = await tenant_store.get_audit_log(TENANT, limit=2, offset=0)
        page2 = await tenant_store.get_audit_log(TENANT, limit=2, offset=2)

        assert len(page1) == 2
        assert len(page2) == 2
        # No overlap
        ids1 = {e["event_id"] for e in page1}
        ids2 = {e["event_id"] for e in page2}
        assert ids1.isdisjoint(ids2)


# ── export tests ──────────────────────────────────────────────────────

class TestExportAuditLog:
    @pytest.mark.asyncio
    async def test_export_chronological_order(self, tenant_store):
        """export_audit_log returns entries oldest-first."""
        e1 = await _append(tenant_store, target_id="n1")
        e2 = await _append(tenant_store, target_id="n2")

        exported = await tenant_store.export_audit_log(TENANT)
        assert exported[0]["event_id"] == e1
        assert exported[1]["event_id"] == e2

    @pytest.mark.asyncio
    async def test_export_with_time_filter(self, tenant_store):
        """Export respects after/before filters."""
        await _append(tenant_store, target_id="n1")
        mid_ts = int(time.time() * 1000)
        await asyncio.sleep(0.01)
        await _append(tenant_store, target_id="n2")

        exported = await tenant_store.export_audit_log(TENANT, after=mid_ts)
        assert len(exported) >= 1
        for entry in exported:
            assert entry["created_at"] > mid_ts

    @pytest.mark.asyncio
    async def test_export_empty_log(self, tenant_store):
        """Export of empty log returns empty list."""
        exported = await tenant_store.export_audit_log(TENANT)
        assert exported == []

    @pytest.mark.asyncio
    async def test_export_includes_all_fields(self, tenant_store):
        """Exported entries contain all expected fields."""
        await _append(
            tenant_store,
            ip_address="10.0.0.1",
            user_agent="Agent/2.0",
            metadata='{"extra": true}',
        )
        exported = await tenant_store.export_audit_log(TENANT)
        entry = exported[0]
        expected_keys = {
            "event_id", "prev_hash", "actor_id", "action",
            "target_type", "target_id", "ip_address", "user_agent",
            "metadata", "created_at",
        }
        assert set(entry.keys()) == expected_keys


# ── concurrent appends ────────────────────────────────────────────────

class TestConcurrentAppends:
    @pytest.mark.asyncio
    async def test_concurrent_appends_preserve_chain(self, tenant_store):
        """Multiple concurrent appends still produce a valid chain.

        Because the store uses BEGIN IMMEDIATE, concurrent appends are
        serialized at the SQLite level, ensuring the chain remains intact.
        """
        tasks = [
            _append(tenant_store, target_id=f"n{i}", actor_id=f"user:{i}")
            for i in range(10)
        ]
        event_ids = await asyncio.gather(*tasks)

        # All appends succeeded
        assert len(event_ids) == 10
        assert len(set(event_ids)) == 10  # unique IDs

        # Chain is still valid
        valid, msg = await tenant_store.verify_audit_chain(TENANT)
        assert valid is True
        assert msg == "ok"
