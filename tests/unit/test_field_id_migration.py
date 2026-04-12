"""
Unit tests for ``CanonicalStore.migrate_payloads_to_field_ids``.

The migration helper rewrites legacy name-keyed payload_json rows in the
nodes table to the id-keyed format expected by EntDB v2 (issue #104).

Tests cover:
    - Migration of legacy name-keyed rows
    - Idempotency (second pass migrates nothing)
    - Unknown field names dropped during migration
    - Rows whose type_id is not in the registry are left unchanged
    - Mixed tenant (legacy + already-migrated rows)
    - Empty tenant
    - Registry None (graceful no-op via unknown_type counter)
    - Returned summary counts match the actual DB state
"""

import json
import tempfile

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.schema import NodeTypeDef, SchemaRegistry, field


def _make_registry() -> SchemaRegistry:
    """Build a small registry with two node types used by the tests."""
    registry = SchemaRegistry()
    registry.register_node_type(
        NodeTypeDef(
            type_id=101,
            name="Task",
            fields=(
                field(1, "title", "str"),
                field(2, "status", "str"),
            ),
        )
    )
    registry.register_node_type(
        NodeTypeDef(
            type_id=202,
            name="Note",
            fields=(field(1, "body", "str"),),
        )
    )
    return registry


def _insert_row(
    store: CanonicalStore,
    tenant_id: str,
    node_id: str,
    type_id: int,
    payload: dict,
) -> None:
    """Insert a raw nodes row, bypassing CanonicalStore's serializer.

    This lets the test seed name-keyed (legacy) payloads directly so the
    migration helper has something to rewrite.
    """
    with store._get_connection(tenant_id) as conn:
        conn.execute(
            """
            INSERT INTO nodes (
                tenant_id, node_id, type_id, payload_json,
                created_at, updated_at, owner_actor, acl_blob
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                tenant_id,
                node_id,
                type_id,
                json.dumps(payload),
                1000,
                1000,
                "user:tester",
                "[]",
            ),
        )


def _read_payload(store: CanonicalStore, tenant_id: str, node_id: str) -> dict:
    with store._get_connection(tenant_id) as conn:
        row = conn.execute(
            "SELECT payload_json FROM nodes WHERE tenant_id = ? AND node_id = ?",
            (tenant_id, node_id),
        ).fetchone()
    return json.loads(row["payload_json"])


class TestMigratePayloadsToFieldIds:
    """Tests for ``CanonicalStore.migrate_payloads_to_field_ids``."""

    @pytest.fixture
    def data_dir(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            yield tmpdir

    @pytest.fixture
    def store(self, data_dir):
        return CanonicalStore(data_dir, wal_mode=False)

    @pytest.fixture
    def registry(self):
        return _make_registry()

    @pytest.mark.asyncio
    async def test_migrates_legacy_name_keyed_rows(self, store, registry):
        """A legacy name-keyed row is rewritten to id-keyed form."""
        tenant_id = "tenant_legacy"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "n1", 101, {"title": "buy milk", "status": "open"})

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result == {"scanned": 1, "migrated": 1, "skipped": 0, "unknown_type": 0}
        assert _read_payload(store, tenant_id, "n1") == {"1": "buy milk", "2": "open"}

    @pytest.mark.asyncio
    async def test_idempotent_second_pass_is_noop(self, store, registry):
        """Running the migration twice migrates zero rows on the second pass."""
        tenant_id = "tenant_idem"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "n1", 101, {"title": "x"})
        _insert_row(store, tenant_id, "n2", 202, {"body": "hello"})

        first = await store.migrate_payloads_to_field_ids(tenant_id, registry)
        assert first["migrated"] == 2

        second = await store.migrate_payloads_to_field_ids(tenant_id, registry)
        assert second == {"scanned": 2, "migrated": 0, "skipped": 2, "unknown_type": 0}

        # Payloads remain in id-keyed form after the second pass.
        assert _read_payload(store, tenant_id, "n1") == {"1": "x"}
        assert _read_payload(store, tenant_id, "n2") == {"1": "hello"}

    @pytest.mark.asyncio
    async def test_unknown_field_names_dropped(self, store, registry):
        """Unknown field names are silently dropped during migration."""
        tenant_id = "tenant_drop"
        await store.initialize_tenant(tenant_id)

        _insert_row(
            store,
            tenant_id,
            "n1",
            101,
            {"title": "keep me", "bogus": "drop me", "status": "open"},
        )

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result["migrated"] == 1
        assert _read_payload(store, tenant_id, "n1") == {"1": "keep me", "2": "open"}

    @pytest.mark.asyncio
    async def test_unknown_type_id_left_unchanged(self, store, registry):
        """Rows whose type_id is not registered are not modified."""
        tenant_id = "tenant_unknown_type"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "n1", 999, {"foo": "bar"})

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result == {
            "scanned": 1,
            "migrated": 0,
            "skipped": 0,
            "unknown_type": 1,
        }
        # Payload untouched.
        assert _read_payload(store, tenant_id, "n1") == {"foo": "bar"}

    @pytest.mark.asyncio
    async def test_mixed_legacy_and_migrated_rows(self, store, registry):
        """A tenant containing both legacy and id-keyed rows migrates only the legacy ones."""
        tenant_id = "tenant_mixed"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "legacy1", 101, {"title": "old", "status": "open"})
        _insert_row(store, tenant_id, "legacy2", 202, {"body": "old note"})
        _insert_row(store, tenant_id, "modern1", 101, {"1": "new", "2": "done"})
        _insert_row(store, tenant_id, "modern2", 202, {"1": "new note"})

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result == {
            "scanned": 4,
            "migrated": 2,
            "skipped": 2,
            "unknown_type": 0,
        }
        assert _read_payload(store, tenant_id, "legacy1") == {"1": "old", "2": "open"}
        assert _read_payload(store, tenant_id, "legacy2") == {"1": "old note"}
        assert _read_payload(store, tenant_id, "modern1") == {"1": "new", "2": "done"}
        assert _read_payload(store, tenant_id, "modern2") == {"1": "new note"}

    @pytest.mark.asyncio
    async def test_empty_tenant_zero_counts(self, store, registry):
        """A tenant with no nodes returns all-zero counts."""
        tenant_id = "tenant_empty"
        await store.initialize_tenant(tenant_id)

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result == {"scanned": 0, "migrated": 0, "skipped": 0, "unknown_type": 0}

    @pytest.mark.asyncio
    async def test_registry_none_marks_unknown_type(self, store):
        """A None registry marks every legacy row as unknown_type (graceful no-op)."""
        tenant_id = "tenant_no_registry"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "n1", 101, {"title": "x"})
        _insert_row(store, tenant_id, "n2", 202, {"body": "y"})

        result = await store.migrate_payloads_to_field_ids(tenant_id, None)

        assert result == {
            "scanned": 2,
            "migrated": 0,
            "skipped": 0,
            "unknown_type": 2,
        }
        # Rows untouched.
        assert _read_payload(store, tenant_id, "n1") == {"title": "x"}
        assert _read_payload(store, tenant_id, "n2") == {"body": "y"}

    @pytest.mark.asyncio
    async def test_summary_counts_match_db_state(self, store, registry):
        """The returned summary numbers match what's actually stored in the DB."""
        tenant_id = "tenant_summary"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "legacy", 101, {"title": "a"})
        _insert_row(store, tenant_id, "already", 101, {"1": "b"})
        _insert_row(store, tenant_id, "unk", 999, {"foo": "bar"})

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result["scanned"] == 3
        assert result["migrated"] == 1
        assert result["skipped"] == 1
        assert result["unknown_type"] == 1

        # Confirm DB matches that breakdown.
        with store._get_connection(tenant_id) as conn:
            rows = conn.execute(
                "SELECT node_id, payload_json FROM nodes WHERE tenant_id = ?",
                (tenant_id,),
            ).fetchall()
        payloads = {r["node_id"]: json.loads(r["payload_json"]) for r in rows}
        assert payloads["legacy"] == {"1": "a"}
        assert payloads["already"] == {"1": "b"}
        assert payloads["unk"] == {"foo": "bar"}

    @pytest.mark.asyncio
    async def test_empty_payload_treated_as_id_keyed(self, store, registry):
        """An empty payload counts as already id-keyed and is skipped."""
        tenant_id = "tenant_empty_payload"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "n1", 101, {})

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result == {"scanned": 1, "migrated": 0, "skipped": 1, "unknown_type": 0}
        assert _read_payload(store, tenant_id, "n1") == {}

    @pytest.mark.asyncio
    async def test_multiple_node_types_in_one_pass(self, store, registry):
        """The migration handles a heterogeneous mix of node types in one pass."""
        tenant_id = "tenant_multi_type"
        await store.initialize_tenant(tenant_id)

        _insert_row(store, tenant_id, "t1", 101, {"title": "alpha", "status": "open"})
        _insert_row(store, tenant_id, "t2", 101, {"title": "beta"})
        _insert_row(store, tenant_id, "t3", 202, {"body": "gamma"})

        result = await store.migrate_payloads_to_field_ids(tenant_id, registry)

        assert result == {"scanned": 3, "migrated": 3, "skipped": 0, "unknown_type": 0}
        assert _read_payload(store, tenant_id, "t1") == {"1": "alpha", "2": "open"}
        assert _read_payload(store, tenant_id, "t2") == {"1": "beta"}
        assert _read_payload(store, tenant_id, "t3") == {"1": "gamma"}
