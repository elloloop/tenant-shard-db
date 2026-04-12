"""
Unit tests for the tamper-evident audit log subsystem.

Tests cover:
- AuditStore write + read entries
- Hash chain verification (valid chain)
- Tamper detection (modify a row, verify_chain returns False)
- Filtering by actor, action, time range
- Export format (JSON)
- Empty log edge case
- Concurrent writes preserve chain ordering
- Middleware best-effort semantics
- Export with unsupported format raises ValueError
- Details / metadata round-trip
- Normalised entry schema
- Chain verification detail messages
- Multiple tenants are isolated
- Middleware logs unknown actions without crashing
- Large chain verification
"""

from __future__ import annotations

import asyncio
import json
import sqlite3
import time

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.audit.audit_middleware import (
    KNOWN_ACTIONS,
    log_audit_event,
)
from dbaas.entdb_server.audit.audit_store import AuditStore

# ── fixtures ─────────────────────────────────────────────────────────────

TENANT = "t1"


@pytest.fixture()
def canonical_store(tmp_path):
    """Create a CanonicalStore in a temp directory."""
    s = CanonicalStore(str(tmp_path))
    yield s
    s.close_all()


@pytest.fixture()
async def audit_store(canonical_store):
    """AuditStore with a pre-initialised tenant."""
    await canonical_store.initialize_tenant(TENANT)
    return AuditStore(canonical_store)


@pytest.fixture()
def raw_store(canonical_store):
    """Expose the underlying CanonicalStore (for tamper tests)."""
    return canonical_store


# ── helpers ──────────────────────────────────────────────────────────────


async def _append(store: AuditStore, **kwargs):
    """Shorthand for appending an entry with sensible defaults."""
    defaults = {
        "tenant_id": TENANT,
        "actor": "user:alice",
        "action": "node.create",
        "resource_type": "node",
        "resource_id": "n1",
    }
    defaults.update(kwargs)
    return await store.append(**defaults)


# ── 1. Write + read entries ──────────────────────────────────────────────


class TestWriteAndRead:
    @pytest.mark.asyncio
    async def test_append_returns_event_id(self, audit_store):
        """Appending an entry returns a UUID event_id."""
        eid = await _append(audit_store)
        assert eid is not None
        assert len(eid) == 36  # UUID format

    @pytest.mark.asyncio
    async def test_read_back_entry(self, audit_store):
        """An appended entry can be read back with correct fields."""
        await _append(audit_store, actor="user:bob", action="node.update")
        log = await audit_store.get_audit_log(TENANT)
        assert len(log) == 1
        entry = log[0]
        assert entry["actor"] == "user:bob"
        assert entry["action"] == "node.update"
        assert entry["resource_type"] == "node"
        assert entry["resource_id"] == "n1"

    @pytest.mark.asyncio
    async def test_details_round_trip(self, audit_store):
        """Details dict is stored as JSON and returned in details_json."""
        details = {"type_id": 42, "fields": ["name", "email"]}
        await _append(audit_store, details=details)
        log = await audit_store.get_audit_log(TENANT)
        assert json.loads(log[0]["details_json"]) == details


# ── 2. Hash chain verification (valid) ──────────────────────────────────


class TestValidChain:
    @pytest.mark.asyncio
    async def test_single_entry_chain_valid(self, audit_store):
        """A single entry forms a valid chain."""
        await _append(audit_store)
        assert await audit_store.verify_chain(TENANT) is True

    @pytest.mark.asyncio
    async def test_multi_entry_chain_valid(self, audit_store):
        """Multiple sequential entries form a valid chain."""
        for i in range(8):
            await _append(audit_store, resource_id=f"n{i}")
        assert await audit_store.verify_chain(TENANT) is True

    @pytest.mark.asyncio
    async def test_genesis_prev_hash(self, audit_store):
        """First entry has prev_hash = 'genesis'."""
        await _append(audit_store)
        log = await audit_store.get_audit_log(TENANT)
        assert log[0]["prev_hash"] == "genesis"


# ── 3. Tamper detection ─────────────────────────────────────────────────


class TestTamperDetection:
    @pytest.mark.asyncio
    async def test_modify_actor_breaks_chain(self, audit_store, raw_store):
        """Modifying a row's actor_id is detected."""
        for i in range(3):
            await _append(audit_store, resource_id=f"n{i}")

        # Tamper directly in SQLite
        db_path = raw_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute("UPDATE audit_log SET actor_id = 'hacker' WHERE rowid = 1")
        conn.commit()
        conn.close()

        assert await audit_store.verify_chain(TENANT) is False

    @pytest.mark.asyncio
    async def test_modify_action_breaks_chain(self, audit_store, raw_store):
        """Modifying a row's action field is detected."""
        for i in range(3):
            await _append(audit_store, resource_id=f"n{i}")

        db_path = raw_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute("UPDATE audit_log SET action = 'evil' WHERE rowid = 1")
        conn.commit()
        conn.close()

        assert await audit_store.verify_chain(TENANT) is False

    @pytest.mark.asyncio
    async def test_delete_row_breaks_chain(self, audit_store, raw_store):
        """Deleting a row from the middle breaks the chain."""
        for i in range(4):
            await _append(audit_store, resource_id=f"n{i}")

        db_path = raw_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute("DELETE FROM audit_log WHERE rowid = 2")
        conn.commit()
        conn.close()

        assert await audit_store.verify_chain(TENANT) is False

    @pytest.mark.asyncio
    async def test_verify_chain_detail_message(self, audit_store, raw_store):
        """verify_chain_detail returns a descriptive message on tampering."""
        for i in range(3):
            await _append(audit_store, resource_id=f"n{i}")

        db_path = raw_store.get_db_path(TENANT)
        conn = sqlite3.connect(str(db_path))
        conn.execute("UPDATE audit_log SET prev_hash = 'tampered' WHERE rowid = 2")
        conn.commit()
        conn.close()

        valid, msg = await audit_store.verify_chain_detail(TENANT)
        assert valid is False
        assert "Break" in msg or "event_id" in msg or "tampered" in msg.lower()


# ── 4. Filtering ────────────────────────────────────────────────────────


class TestFiltering:
    @pytest.mark.asyncio
    async def test_filter_by_actor(self, audit_store):
        """Only entries by the given actor are returned."""
        await _append(audit_store, actor="user:alice", resource_id="n1")
        await _append(audit_store, actor="user:bob", resource_id="n2")
        await _append(audit_store, actor="user:alice", resource_id="n3")

        log = await audit_store.get_audit_log(TENANT, actor="user:bob")
        assert len(log) == 1
        assert log[0]["actor"] == "user:bob"

    @pytest.mark.asyncio
    async def test_filter_by_action(self, audit_store):
        """Only entries with the given action are returned."""
        await _append(audit_store, action="node.create", resource_id="n1")
        await _append(audit_store, action="node.delete", resource_id="n2")

        log = await audit_store.get_audit_log(TENANT, action="node.delete")
        assert len(log) == 1
        assert log[0]["action"] == "node.delete"

    @pytest.mark.asyncio
    async def test_filter_by_since(self, audit_store):
        """Only entries after the given timestamp are returned."""
        await _append(audit_store, resource_id="n1")
        cutoff = int(time.time() * 1000)
        await asyncio.sleep(0.01)
        await _append(audit_store, resource_id="n2")

        log = await audit_store.get_audit_log(TENANT, since=cutoff)
        assert len(log) >= 1
        for entry in log:
            assert entry["timestamp"] > cutoff

    @pytest.mark.asyncio
    async def test_limit(self, audit_store):
        """Limit caps the number of returned entries."""
        for i in range(5):
            await _append(audit_store, resource_id=f"n{i}")
        log = await audit_store.get_audit_log(TENANT, limit=2)
        assert len(log) == 2


# ── 5. Export format ────────────────────────────────────────────────────


class TestExport:
    @pytest.mark.asyncio
    async def test_export_json_format(self, audit_store):
        """Export produces valid JSON with expected structure."""
        await _append(audit_store, resource_id="n1")
        await _append(audit_store, resource_id="n2")

        exported = await audit_store.export_audit_log(TENANT, format="json")
        entries = json.loads(exported)
        assert isinstance(entries, list)
        assert len(entries) == 2
        # Chronological order (oldest first)
        assert entries[0]["timestamp"] <= entries[1]["timestamp"]

    @pytest.mark.asyncio
    async def test_export_entry_schema(self, audit_store):
        """Exported entries contain all required keys."""
        await _append(audit_store, details={"x": 1})
        exported = await audit_store.export_audit_log(TENANT)
        entries = json.loads(exported)
        expected_keys = {
            "seq",
            "timestamp",
            "actor",
            "action",
            "resource_type",
            "resource_id",
            "details_json",
            "prev_hash",
            "entry_hash",
        }
        assert set(entries[0].keys()) == expected_keys

    @pytest.mark.asyncio
    async def test_export_unsupported_format(self, audit_store):
        """Requesting an unsupported format raises ValueError."""
        with pytest.raises(ValueError, match="Unsupported export format"):
            await audit_store.export_audit_log(TENANT, format="csv")


# ── 6. Empty log edge case ──────────────────────────────────────────────


class TestEmptyLog:
    @pytest.mark.asyncio
    async def test_empty_log_verify_returns_true(self, audit_store):
        """An empty audit log verifies successfully."""
        assert await audit_store.verify_chain(TENANT) is True

    @pytest.mark.asyncio
    async def test_empty_log_get_returns_empty(self, audit_store):
        """Querying an empty audit log returns an empty list."""
        log = await audit_store.get_audit_log(TENANT)
        assert log == []

    @pytest.mark.asyncio
    async def test_empty_log_export_returns_empty_array(self, audit_store):
        """Exporting an empty log returns '[]'."""
        exported = await audit_store.export_audit_log(TENANT)
        assert json.loads(exported) == []


# ── 7. Concurrent writes ────────────────────────────────────────────────


class TestConcurrentWrites:
    @pytest.mark.asyncio
    async def test_concurrent_appends_preserve_chain(self, audit_store):
        """Concurrent appends still produce a valid chain.

        SQLite's BEGIN IMMEDIATE serialises writers, so the chain
        remains intact even under concurrent access.
        """
        tasks = [_append(audit_store, resource_id=f"n{i}", actor=f"user:{i}") for i in range(10)]
        event_ids = await asyncio.gather(*tasks)

        assert len(event_ids) == 10
        assert len(set(event_ids)) == 10  # all unique

        assert await audit_store.verify_chain(TENANT) is True

    @pytest.mark.asyncio
    async def test_large_chain_verification(self, audit_store):
        """A chain with many entries still verifies correctly."""
        for i in range(25):
            await _append(audit_store, resource_id=f"n{i}")
        assert await audit_store.verify_chain(TENANT) is True


# ── 8. Middleware ────────────────────────────────────────────────────────


class TestMiddleware:
    @pytest.mark.asyncio
    async def test_log_audit_event_writes_entry(self, audit_store):
        """log_audit_event writes an entry via AuditStore."""
        eid = await log_audit_event(
            audit_store,
            tenant_id=TENANT,
            actor="user:alice",
            action="node.create",
            resource_type="node",
            resource_id="n42",
            details={"type_id": 7},
        )
        assert eid is not None

        log = await audit_store.get_audit_log(TENANT)
        assert len(log) == 1
        assert log[0]["resource_id"] == "n42"

    @pytest.mark.asyncio
    async def test_middleware_unknown_action_still_logs(self, audit_store):
        """An unknown action is logged with a warning, not rejected."""
        eid = await log_audit_event(
            audit_store,
            tenant_id=TENANT,
            actor="user:alice",
            action="custom.unknown",
            resource_type="widget",
            resource_id="w1",
        )
        assert eid is not None

    @pytest.mark.asyncio
    async def test_known_actions_set(self):
        """All expected actions are present in KNOWN_ACTIONS."""
        expected = {
            "node.create",
            "node.update",
            "node.delete",
            "edge.create",
            "edge.delete",
            "acl.grant",
            "acl.revoke",
            "user.delete",
            "user.freeze",
            "user.export",
            "tenant.create",
            "tenant.delete",
        }
        assert expected == KNOWN_ACTIONS


# ── 9. Tenant isolation ─────────────────────────────────────────────────


class TestTenantIsolation:
    @pytest.mark.asyncio
    async def test_different_tenants_isolated(self, audit_store, raw_store):
        """Entries in one tenant do not appear in another."""
        await raw_store.initialize_tenant("t2")
        store2 = AuditStore(raw_store)

        await _append(audit_store, tenant_id=TENANT, resource_id="n1")
        await _append(store2, tenant_id="t2", resource_id="n2")

        log1 = await audit_store.get_audit_log(TENANT)
        log2 = await store2.get_audit_log("t2")

        assert len(log1) == 1
        assert len(log2) == 1
        assert log1[0]["resource_id"] == "n1"
        assert log2[0]["resource_id"] == "n2"


# ── 10. Normalised entry schema ─────────────────────────────────────────


class TestNormalisedEntry:
    @pytest.mark.asyncio
    async def test_entry_hash_is_hex_sha256(self, audit_store):
        """The computed entry_hash is a 64-char hex string (SHA-256)."""
        await _append(audit_store)
        log = await audit_store.get_audit_log(TENANT)
        h = log[0]["entry_hash"]
        assert len(h) == 64
        assert all(c in "0123456789abcdef" for c in h)

    @pytest.mark.asyncio
    async def test_entry_has_all_fields(self, audit_store):
        """Every normalised entry has the full set of public keys."""
        await _append(audit_store)
        entry = (await audit_store.get_audit_log(TENANT))[0]
        required = {
            "seq",
            "timestamp",
            "actor",
            "action",
            "resource_type",
            "resource_id",
            "details_json",
            "prev_hash",
            "entry_hash",
        }
        assert set(entry.keys()) == required
