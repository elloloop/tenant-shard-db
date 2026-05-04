"""
Round-trip integration test for the Python SDK over gRPC.

Verifies the wire-format invariant introduced in the PR-D fix:
``Node.payload`` is field-id-keyed on the wire, and the SDK
translates id → name on the client side using the local schema
registry. A user writing a proto message with set fields and
reading the node back must see the same field names in
``Node.payload``.

We spin up a real ``EntDBServicer`` + ``Applier`` + in-process gRPC
server on a localhost port, then connect a real ``DbClient`` and
exercise create + read + query.
"""

from __future__ import annotations

import asyncio
import socket
import tempfile

import pytest
from grpc import aio as grpc_aio

from dbaas.entdb_server.api.generated import add_EntDBServiceServicer_to_server
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.apply.applier import Applier, MailboxFanoutConfig
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore
from dbaas.entdb_server.schema.registry import SchemaRegistry as ServerSchemaRegistry
from dbaas.entdb_server.schema.types import FieldDef as ServerFieldDef
from dbaas.entdb_server.schema.types import FieldKind as ServerFieldKind
from dbaas.entdb_server.schema.types import NodeTypeDef as ServerNodeTypeDef
from dbaas.entdb_server.wal.memory import InMemoryWalStream
from sdk.entdb_sdk import register_proto_schema
from sdk.entdb_sdk.client import DbClient
from sdk.entdb_sdk.registry import get_registry, reset_registry
from tests._test_schemas import test_schema_pb2 as ts


def _free_port() -> int:
    """Grab an unused localhost port for the in-process server."""
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


@pytest.fixture
async def live_server():
    """Start a real gRPC server with applier + servicer, yield port + sdk_registry."""
    reset_registry()
    register_proto_schema(ts)
    sdk_registry = get_registry()

    with tempfile.TemporaryDirectory() as tmpdir:
        # Server-side schema mirrors the test_schema (Product type 9001).
        server_registry = ServerSchemaRegistry()
        server_registry.register_node_type(
            ServerNodeTypeDef(
                type_id=9001,
                name="Product",
                fields=(
                    ServerFieldDef(field_id=1, name="sku", kind=ServerFieldKind.STRING),
                    ServerFieldDef(field_id=2, name="name", kind=ServerFieldKind.STRING),
                    ServerFieldDef(field_id=3, name="price_cents", kind=ServerFieldKind.INTEGER),
                    ServerFieldDef(
                        field_id=4,
                        name="status",
                        kind=ServerFieldKind.ENUM,
                        enum_values=("draft", "active", "archived"),
                    ),
                ),
            )
        )

        global_store = GlobalStore(tmpdir)
        await global_store.create_tenant("acme", "Acme")
        await global_store.create_user("alice", "alice@example.com", "Alice")
        await global_store.add_member("acme", "alice", role="owner")

        canonical = CanonicalStore(data_dir=tmpdir)
        await canonical.initialize_tenant("acme")

        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic="test-wal",
            group_id="test",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier_task = asyncio.create_task(applier.start())

        servicer = EntDBServicer(
            wal=wal,
            canonical_store=canonical,
            schema_registry=server_registry,
            topic="test-wal",
            global_store=global_store,
        )

        port = _free_port()
        server = grpc_aio.server()
        add_EntDBServiceServicer_to_server(servicer, server)
        server.add_insecure_port(f"127.0.0.1:{port}")
        await server.start()

        try:
            yield port, sdk_registry
        finally:
            await server.stop(grace=0)
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
            reset_registry()


@pytest.mark.asyncio
async def test_sdk_roundtrip_preserves_named_fields(live_server):
    """SDK user writes a proto message and reads name-keyed payload back.

    The wire is id-keyed; the SDK translates on egress, so the
    user-facing ``node.payload`` must round-trip with the original
    proto field names ('sku', 'name', 'price_cents').
    """
    port, sdk_registry = live_server

    async with DbClient(f"127.0.0.1:{port}", registry=sdk_registry) as db:
        plan = db.atomic("acme", "user:alice")
        plan.create(
            ts.Product(sku="WIDGET-1", name="Widget", price_cents=1499),
            as_="prod1",
        )
        result = await plan.commit(wait_applied=True)
        assert result.success, f"commit failed: {result.error}"
        assert len(result.created_node_ids) == 1
        new_id = result.created_node_ids[0]

        product_type = sdk_registry.get_node_type(9001)
        assert product_type is not None
        node = await db.get(product_type, new_id, "acme", "user:alice")
        assert node is not None
        assert node.payload.get("sku") == "WIDGET-1"
        assert node.payload.get("name") == "Widget"
        assert node.payload.get("price_cents") == 1499
        # No id-keyed leakage (the wire was id-keyed but SDK translated).
        assert "1" not in node.payload
        assert "2" not in node.payload
        assert "3" not in node.payload


@pytest.mark.asyncio
async def test_sdk_roundtrip_query_nodes(live_server):
    """``DbClient.query`` returns name-keyed payloads to SDK consumers."""
    port, sdk_registry = live_server

    async with DbClient(f"127.0.0.1:{port}", registry=sdk_registry) as db:
        plan = db.atomic("acme", "user:alice")
        plan.create(ts.Product(sku="A", name="Apple", price_cents=100), as_="a")
        plan.create(ts.Product(sku="B", name="Banana", price_cents=200), as_="b")
        result = await plan.commit(wait_applied=True)
        assert result.success, f"commit failed: {result.error}"

        product_type = sdk_registry.get_node_type(9001)
        nodes = await db.query(product_type, "acme", "user:alice", limit=10)
        assert len(nodes) == 2
        for n in nodes:
            assert "sku" in n.payload, (
                f"expected name-keyed payload after SDK translation, got {n.payload}"
            )
            assert "name" in n.payload
            assert "price_cents" in n.payload
            # No id-keyed keys leaked through.
            assert not any(k.isdigit() for k in n.payload), (
                f"expected no id-keyed keys, got {n.payload}"
            )
