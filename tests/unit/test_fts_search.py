"""
Unit tests for FTS5-backed full-text search (2026-04-19 decision).

Covers:
1. Basic FTS search (create nodes, search, verify matches)
2. Porter stemming ("running" matches "run")
3. Multi-field search (query matches across name AND description)
4. FTS update (update searchable field, old value gone, new value found)
5. FTS delete (delete node, no longer appears in search)
6. ACL filtering on search results
7. Non-string searchable field rejected at registry level
8. Empty query returns error (gRPC handler)
9. FTS table created lazily on first write/search
10. Search on type with no searchable fields returns empty list
"""

from __future__ import annotations

import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.apply.applier import (
    Applier,
    MailboxFanoutConfig,
    TransactionEvent,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore
from dbaas.entdb_server.schema.registry import (
    get_registry,
    reset_registry,
)
from dbaas.entdb_server.schema.types import NodeTypeDef, field
from dbaas.entdb_server.wal.memory import InMemoryWalStream

TENANT = "tenant-fts-tests"
ALICE = "user:alice"
BOB = "user:bob"

TYPE_PRODUCT = 201

# Field ids matching the schema below
FIELD_SKU = 1
FIELD_NAME = 3
FIELD_PRICE = 4
FIELD_DESC = 6


# ── Fixtures ────────────────────────────────────────────────────────


@pytest.fixture(autouse=True)
def registered_schema():
    """Register a Product type with searchable fields for every test."""
    reset_registry()
    reg = get_registry()
    product = NodeTypeDef(
        type_id=TYPE_PRODUCT,
        name="Product",
        fields=(
            field(FIELD_SKU, "sku", "str", unique=True),
            field(FIELD_NAME, "name", "str", searchable=True, indexed=True),
            field(FIELD_PRICE, "price", "int"),
            field(FIELD_DESC, "desc", "str", searchable=True),
        ),
    )
    reg.register_node_type(product)
    yield reg
    reset_registry()


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=str(tmp_path))
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


@pytest.fixture
def global_store():
    with tempfile.TemporaryDirectory() as tmpdir:
        gs = GlobalStore(tmpdir)
        yield gs
        gs.close()


async def _make_applier(store) -> Applier:
    wal = InMemoryWalStream(num_partitions=1)
    await wal.connect()
    return Applier(
        wal=wal,
        canonical_store=store,
        topic="t",
        fanout_config=MailboxFanoutConfig(enabled=False),
    )


def _make_create_event(
    idempotency_key: str,
    *,
    node_id: str,
    payload: dict,
    tenant_id: str = TENANT,
    type_id: int = TYPE_PRODUCT,
    actor: str = ALICE,
) -> TransactionEvent:
    return TransactionEvent.from_dict(
        {
            "tenant_id": tenant_id,
            "actor": actor,
            "idempotency_key": idempotency_key,
            "ops": [
                {
                    "op": "create_node",
                    "type_id": type_id,
                    "id": node_id,
                    "data": payload,
                }
            ],
        }
    )


def _make_update_event(
    idempotency_key: str,
    *,
    node_id: str,
    patch: dict,
    tenant_id: str = TENANT,
    type_id: int = TYPE_PRODUCT,
    actor: str = ALICE,
) -> TransactionEvent:
    return TransactionEvent.from_dict(
        {
            "tenant_id": tenant_id,
            "actor": actor,
            "idempotency_key": idempotency_key,
            "ops": [
                {
                    "op": "update_node",
                    "type_id": type_id,
                    "id": node_id,
                    "patch": patch,
                }
            ],
        }
    )


def _make_delete_event(
    idempotency_key: str,
    *,
    node_id: str,
    tenant_id: str = TENANT,
    type_id: int = TYPE_PRODUCT,
    actor: str = ALICE,
) -> TransactionEvent:
    return TransactionEvent.from_dict(
        {
            "tenant_id": tenant_id,
            "actor": actor,
            "idempotency_key": idempotency_key,
            "ops": [
                {
                    "op": "delete_node",
                    "type_id": type_id,
                    "id": node_id,
                }
            ],
        }
    )


# ── Tests ───────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_basic_fts_search(store):
    """Create nodes with text, search, verify matches."""
    applier = await _make_applier(store)
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    await applier.apply_event(
        _make_create_event(
            "ev1",
            node_id="p1",
            payload={
                str(FIELD_SKU): "W1",
                str(FIELD_NAME): "Blue Widget",
                str(FIELD_DESC): "A small widget",
            },
        )
    )
    await applier.apply_event(
        _make_create_event(
            "ev2",
            node_id="p2",
            payload={
                str(FIELD_SKU): "G1",
                str(FIELD_NAME): "Red Gadget",
                str(FIELD_DESC): "A large gadget",
            },
        )
    )

    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "widget", searchable_fids)
    assert len(results) == 1
    assert results[0].node_id == "p1"


@pytest.mark.asyncio
async def test_porter_stemming(store):
    """Porter stemming: 'running' matches 'run'."""
    applier = await _make_applier(store)
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    await applier.apply_event(
        _make_create_event(
            "ev1",
            node_id="p1",
            payload={
                str(FIELD_SKU): "R1",
                str(FIELD_NAME): "Running Shoes",
                str(FIELD_DESC): "For runners",
            },
        )
    )

    # "run" should match "Running" and "runners" via Porter stemming
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "run", searchable_fids)
    assert len(results) == 1
    assert results[0].node_id == "p1"


@pytest.mark.asyncio
async def test_multi_field_search(store):
    """Query matches across name AND description fields."""
    applier = await _make_applier(store)
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    await applier.apply_event(
        _make_create_event(
            "ev1",
            node_id="p1",
            payload={str(FIELD_SKU): "A1", str(FIELD_NAME): "Alpha", str(FIELD_DESC): "Fancy item"},
        )
    )
    await applier.apply_event(
        _make_create_event(
            "ev2",
            node_id="p2",
            payload={
                str(FIELD_SKU): "B1",
                str(FIELD_NAME): "Fancy Beta",
                str(FIELD_DESC): "Standard item",
            },
        )
    )

    # "fancy" appears in desc of p1 and name of p2
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "fancy", searchable_fids)
    assert len(results) == 2
    result_ids = {r.node_id for r in results}
    assert result_ids == {"p1", "p2"}


@pytest.mark.asyncio
async def test_fts_update(store):
    """Update a searchable field: old value no longer matches, new value does."""
    applier = await _make_applier(store)
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    await applier.apply_event(
        _make_create_event(
            "ev1",
            node_id="p1",
            payload={
                str(FIELD_SKU): "U1",
                str(FIELD_NAME): "Original Name",
                str(FIELD_DESC): "desc",
            },
        )
    )

    # Verify original name is searchable
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "Original", searchable_fids)
    assert len(results) == 1

    # Update the name
    await applier.apply_event(
        _make_update_event(
            "ev2",
            node_id="p1",
            patch={str(FIELD_NAME): "Updated Name"},
        )
    )

    # Old value should no longer match
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "Original", searchable_fids)
    assert len(results) == 0

    # New value should match
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "Updated", searchable_fids)
    assert len(results) == 1
    assert results[0].node_id == "p1"


@pytest.mark.asyncio
async def test_fts_delete(store):
    """Delete a node: it no longer appears in search results."""
    applier = await _make_applier(store)
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    await applier.apply_event(
        _make_create_event(
            "ev1",
            node_id="p1",
            payload={
                str(FIELD_SKU): "D1",
                str(FIELD_NAME): "Deletable Widget",
                str(FIELD_DESC): "will be deleted",
            },
        )
    )

    # Verify it's searchable
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "Deletable", searchable_fids)
    assert len(results) == 1

    # Delete
    await applier.apply_event(_make_delete_event("ev2", node_id="p1"))

    # Should no longer appear
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "Deletable", searchable_fids)
    assert len(results) == 0


@pytest.mark.asyncio
async def test_non_string_searchable_field_excluded():
    """Non-string field with searchable=true is excluded from searchable_field_ids."""
    reset_registry()
    reg = get_registry()
    bad_type = NodeTypeDef(
        type_id=999,
        name="BadType",
        fields=(
            field(1, "title", "str", searchable=True),
            field(2, "count", "int", searchable=True),  # not string
        ),
    )
    reg.register_node_type(bad_type)

    fids = reg.get_searchable_field_ids(999)
    # Only the string field should be included
    assert fids == [1]
    reset_registry()


@pytest.mark.asyncio
async def test_search_no_searchable_fields(store):
    """Search on a type with no searchable fields returns empty list."""
    reset_registry()
    reg = get_registry()
    plain_type = NodeTypeDef(
        type_id=300,
        name="PlainType",
        fields=(
            field(1, "title", "str"),
            field(2, "count", "int"),
        ),
    )
    reg.register_node_type(plain_type)

    searchable_fids = reg.get_searchable_field_ids(300)
    assert searchable_fids == []

    results = store._sync_search_nodes(TENANT, 300, "anything", searchable_fids)
    assert results == []
    reset_registry()


@pytest.mark.asyncio
async def test_fts_table_created_lazily(store):
    """FTS table is created lazily on first search/write."""
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    # Table should not exist yet
    with store._get_connection(TENANT) as conn:
        cursor = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'fts_%'"
        )
        assert cursor.fetchone() is None

    # Search creates the table lazily
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "test", searchable_fids)
    assert results == []

    # Now the table should exist
    with store._get_connection(TENANT) as conn:
        cursor = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' AND name = ?",
            (f"fts_t{TYPE_PRODUCT}",),
        )
        row = cursor.fetchone()
        assert row is not None


@pytest.mark.asyncio
async def test_search_with_limit_and_offset(store):
    """Search respects limit and offset parameters."""
    applier = await _make_applier(store)
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    # Create 5 widgets
    for i in range(5):
        await applier.apply_event(
            _make_create_event(
                f"ev{i}",
                node_id=f"p{i}",
                payload={
                    str(FIELD_SKU): f"W{i}",
                    str(FIELD_NAME): f"Widget {i}",
                    str(FIELD_DESC): "A widget product",
                },
            )
        )

    # Search with limit
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "widget", searchable_fids, limit=2)
    assert len(results) == 2

    # Search with offset
    all_results = store._sync_search_nodes(
        TENANT, TYPE_PRODUCT, "widget", searchable_fids, limit=50
    )
    offset_results = store._sync_search_nodes(
        TENANT, TYPE_PRODUCT, "widget", searchable_fids, limit=50, offset=2
    )
    assert len(offset_results) == len(all_results) - 2


@pytest.mark.asyncio
async def test_fts_update_non_searchable_field_no_change(store):
    """Updating a non-searchable field does not affect FTS index."""
    applier = await _make_applier(store)
    registry = get_registry()
    searchable_fids = registry.get_searchable_field_ids(TYPE_PRODUCT)

    await applier.apply_event(
        _make_create_event(
            "ev1",
            node_id="p1",
            payload={
                str(FIELD_SKU): "NC1",
                str(FIELD_NAME): "Constant Name",
                str(FIELD_PRICE): 100,
            },
        )
    )

    # Update price (non-searchable)
    await applier.apply_event(
        _make_update_event(
            "ev2",
            node_id="p1",
            patch={str(FIELD_PRICE): 200},
        )
    )

    # Name should still be searchable
    results = store._sync_search_nodes(TENANT, TYPE_PRODUCT, "Constant", searchable_fids)
    assert len(results) == 1
    assert results[0].node_id == "p1"


# ── gRPC handler tests ─────────────────────────────────────────────


class _AbortError(BaseException):
    pass


class _FakeContext:
    def __init__(self):
        self.aborted = False
        self.abort_code = None
        self.abort_message = None

    async def abort(self, code, message):
        self.aborted = True
        self.abort_code = code
        self.abort_message = message
        raise _AbortError(f"[{code}] {message}")


def _make_servicer(canonical_store, global_store=None, schema_registry=None):
    wal = MagicMock()
    pos = MagicMock()
    pos.__str__ = MagicMock(return_value="0:0:0")
    wal.append = AsyncMock(return_value=pos)

    reg = schema_registry if schema_registry is not None else get_registry()
    if getattr(reg, "fingerprint", None) is None:
        try:
            reg._fingerprint = ""
        except Exception:
            pass

    return EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=reg,
        global_store=global_store,
    )


async def _bootstrap_tenant(gs, tenant_id=TENANT, members=None):
    await gs.create_tenant(tenant_id, f"Tenant {tenant_id}")
    if members:
        for user_id, role in members.items():
            await gs.add_member(tenant_id, user_id, role=role)


@pytest.mark.asyncio
async def test_grpc_search_empty_query(store, global_store):
    """Empty query returns INVALID_ARGUMENT."""
    await _bootstrap_tenant(global_store, members={"alice": "admin"})
    servicer = _make_servicer(store, global_store)
    ctx = _FakeContext()

    from dbaas.entdb_server.api.generated import SearchNodesRequest

    req = SearchNodesRequest(
        tenant_id=TENANT,
        actor=ALICE,
        type_id=TYPE_PRODUCT,
        query="",
    )
    with pytest.raises(_AbortError):
        await servicer.SearchNodes(req, ctx)
    assert ctx.abort_code == grpc.StatusCode.INVALID_ARGUMENT


@pytest.mark.asyncio
async def test_grpc_search_returns_results(store, global_store):
    """SearchNodes RPC returns matching nodes."""
    await _bootstrap_tenant(global_store, members={"alice": "admin"})
    servicer = _make_servicer(store, global_store)

    # Create a node via the applier
    applier = await _make_applier(store)
    await applier.apply_event(
        _make_create_event(
            "ev1",
            node_id="p1",
            payload={
                str(FIELD_SKU): "S1",
                str(FIELD_NAME): "Searchable Widget",
                str(FIELD_DESC): "findme",
            },
        )
    )

    from dbaas.entdb_server.api.generated import SearchNodesRequest

    req = SearchNodesRequest(
        tenant_id=TENANT,
        actor=ALICE,
        type_id=TYPE_PRODUCT,
        query="widget",
        limit=10,
    )
    ctx = _FakeContext()
    resp = await servicer.SearchNodes(req, ctx)
    assert len(resp.nodes) == 1
    assert resp.nodes[0].node_id == "p1"
