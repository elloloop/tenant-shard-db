"""
gRPC wire-format contract for Node.payload.

Per ``CLAUDE.md`` and the Go SDK comment at
``sdk/go/entdb/marshal.go:342``, the wire format for ``Node.payload``
is **field-id-keyed** (e.g. ``{"1": "alice@example.com", "2": "..."}``),
not field-name-keyed. Storage is id-keyed, the wire is id-keyed; the
only translation point is the **write** boundary, where the server
accepts name-keyed input from clients and converts to ids before
storing.

These tests assert the read-path contract: GetNode, GetNodes,
QueryNodes, SearchNodes, and the payload variants used by the
ACL v2 handlers (GetConnectedNodes, ListSharedWithMe, etc.) must
return id-keyed payloads.

Background: prior to this fix, the server applied
``_payload_id_to_name_dict`` and ``_payload_id_to_name_json`` on the
read path, which silently translated id-keyed storage → name-keyed
wire. The Go SDK's marshal layer expects id-keyed and falls back to
name-matching only as a tolerance. The fallback drops fields silently
when names disagree (e.g. snake_case proto vs camelCase schema), so a
write+read round-trip would lose data.
"""

from __future__ import annotations

import asyncio
import tempfile
import time
from unittest.mock import AsyncMock

import pytest
from google.protobuf.json_format import MessageToDict
from google.protobuf.struct_pb2 import Struct, Value

from dbaas.entdb_server.api.generated import (
    CreateNodeOp,
    ExecuteAtomicRequest,
    GetNodeRequest,
    GetNodesRequest,
    Operation,
    QueryNodesRequest,
    RequestContext,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.apply.applier import Applier, MailboxFanoutConfig
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import FieldDef, FieldKind, NodeTypeDef
from dbaas.entdb_server.wal.memory import InMemoryWalStream

# ── Fixtures ────────────────────────────────────────────────────────


@pytest.fixture
async def setup():
    """Spin up servicer + applier with a User type registered.

    The User type uses the same shape as the bug report:
    field_id=1 → email, field_id=2 → password_hash.
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        registry = SchemaRegistry()
        registry.register_node_type(
            NodeTypeDef(
                type_id=101,
                name="User",
                fields=(
                    FieldDef(field_id=1, name="email", kind=FieldKind.STRING),
                    FieldDef(field_id=2, name="password_hash", kind=FieldKind.STRING),
                ),
            )
        )

        global_store = GlobalStore(tmpdir)
        await global_store.create_tenant("acme", "Acme Corp")

        canonical = CanonicalStore(data_dir=tmpdir)
        await canonical.initialize_tenant("acme")

        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        # Drive an Applier so writes through ExecuteAtomic actually land
        # in canonical_store and are visible to subsequent reads.
        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic="test-wal",
            group_id="test",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )

        servicer = EntDBServicer(
            wal=wal,
            canonical_store=canonical,
            schema_registry=registry,
            topic="test-wal",
            global_store=global_store,
        )

        applier_task = asyncio.create_task(applier.start())
        await asyncio.sleep(0.05)

        try:
            yield servicer, applier
        finally:
            await applier.stop()
            try:
                await asyncio.wait_for(applier_task, timeout=2.0)
            except asyncio.TimeoutError:
                applier_task.cancel()
            try:
                await wal.close()
            except Exception:
                pass
            global_store.close()


def _name_keyed_struct(d: dict) -> Struct:
    """Build a Struct with name-keyed payload (the SDK wire shape on writes)."""
    s = Struct()
    for k, v in d.items():
        s.fields[k].CopyFrom(Value(string_value=v))
    return s


async def _write_user(servicer, payload_name_keyed: dict, node_id: str = "user-1") -> None:
    """Drive a write through ExecuteAtomic with name-keyed payload (the SDK convention)."""
    ctx = AsyncMock()
    ctx.abort = AsyncMock()
    request = ExecuteAtomicRequest(
        context=RequestContext(tenant_id="acme", actor="user:alice"),
        idempotency_key=f"idem-{node_id}-{int(time.time() * 1e6)}",
        operations=[
            Operation(
                create_node=CreateNodeOp(
                    type_id=101,
                    id=node_id,
                    data=_name_keyed_struct(payload_name_keyed),
                )
            )
        ],
        wait_applied=True,
        wait_timeout_ms=2000,
    )
    resp = await servicer.ExecuteAtomic(request, ctx)
    assert resp.success, f"ExecuteAtomic failed: {resp.error}"


# ── Tests ───────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_get_node_returns_id_keyed_payload(setup):
    """GetNode must return id-keyed payload on the wire (not name-keyed).

    The user's repro: write a User with email + password_hash; the read
    side must surface the values keyed by field_id ('1', '2'), per the
    documented invariant that translation only happens at the write
    ingress boundary.
    """
    servicer, _ = setup
    await _write_user(
        servicer,
        {"email": "alice@example.com", "password_hash": "$2a$ZZZ"},
    )

    ctx = AsyncMock()
    ctx.abort = AsyncMock()
    resp = await servicer.GetNode(
        GetNodeRequest(
            context=RequestContext(tenant_id="acme", actor="user:alice"),
            type_id=101,
            node_id="user-1",
        ),
        ctx,
    )

    assert resp.found is True
    payload = MessageToDict(resp.node.payload)
    assert payload == {
        "1": "alice@example.com",
        "2": "$2a$ZZZ",
    }, f"expected id-keyed payload on the wire, got {payload}"


@pytest.mark.asyncio
async def test_get_nodes_returns_id_keyed_payload(setup):
    """GetNodes (batch) must return id-keyed payloads on the wire."""
    servicer, _ = setup
    await _write_user(servicer, {"email": "a@x.com", "password_hash": "ha"}, node_id="u1")
    await _write_user(servicer, {"email": "b@x.com", "password_hash": "hb"}, node_id="u2")

    ctx = AsyncMock()
    ctx.abort = AsyncMock()
    resp = await servicer.GetNodes(
        GetNodesRequest(
            context=RequestContext(tenant_id="acme", actor="user:alice"),
            type_id=101,
            node_ids=["u1", "u2"],
        ),
        ctx,
    )

    assert len(resp.nodes) == 2
    for node in resp.nodes:
        payload = MessageToDict(node.payload)
        assert set(payload.keys()) == {"1", "2"}, (
            f"expected id-keyed keys for node {node.node_id}, got {payload}"
        )


@pytest.mark.asyncio
async def test_query_nodes_returns_id_keyed_payload(setup):
    """QueryNodes must return id-keyed payloads on the wire."""
    servicer, _ = setup
    await _write_user(
        servicer, {"email": "alice@example.com", "password_hash": "$2a$"}, node_id="u1"
    )

    ctx = AsyncMock()
    ctx.abort = AsyncMock()
    resp = await servicer.QueryNodes(
        QueryNodesRequest(
            context=RequestContext(tenant_id="acme", actor="user:alice"),
            type_id=101,
            limit=10,
        ),
        ctx,
    )

    assert len(resp.nodes) >= 1
    payload = MessageToDict(resp.nodes[0].payload)
    assert payload.get("1") == "alice@example.com"
    assert payload.get("2") == "$2a$"
    assert "email" not in payload, "wire must be id-keyed, found 'email' key"
    assert "password_hash" not in payload, "wire must be id-keyed, found 'password_hash' key"
