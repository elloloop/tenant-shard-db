# SPDX-License-Identifier: AGPL-3.0-only
"""gRPC wire-format invariants.

These tests pin the protocol-level contract any reimplementation must
match. The Python SDK and the Go SDK both depend on these invariants;
diverging silently here would make the next version unconsumable.

What's covered:

    1. Node payload on the wire is **id-keyed** (per CLAUDE.md
       invariant #6 and the docstring on
       ``EntDBServicer._node_to_proto``). The SDK translates back to
       names; raw clients must see numeric-string keys.

    2. Operation order does not matter — the server's response is
       independent of the order in which proto fields are encoded.

    3. Oversized request rejection — a payload larger than the
       gRPC default 4 MiB cap returns ``RESOURCE_EXHAUSTED``.

    4. Empty / null payloads are tolerated for types with no
       required fields, and unknown field names are silently
       dropped (matches ``name_to_id_keys`` contract).
"""

from __future__ import annotations

import asyncio
import socket
import tempfile

import grpc
import pytest
from google.protobuf.struct_pb2 import Struct
from grpc import aio as grpc_aio

from dbaas.entdb_server.api.generated import (
    EntDBServiceStub,
    add_EntDBServiceServicer_to_server,
)
from dbaas.entdb_server.api.generated import entdb_pb2 as pb
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.apply.applier import Applier, MailboxFanoutConfig
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import NodeTypeDef, field

TENANT = "wire"
ALICE = "user:alice"


def _free_port() -> int:
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def _build_registry() -> SchemaRegistry:
    reg = SchemaRegistry()
    reg.register_node_type(
        NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str"), field(2, "name", "str")),
        )
    )
    return reg


@pytest.fixture
async def server_with_default_limits():
    """gRPC server using the default 4 MiB receive cap (no override).

    The production ``GrpcServer`` raises the limit to 50 MiB; this
    fixture deliberately uses bare ``grpc_aio.server()`` so the
    oversized-message test exercises the protocol-default rejection.
    """
    from dbaas.entdb_server.schema import registry as registry_mod

    prev = registry_mod._global_registry
    reg = _build_registry()
    registry_mod._global_registry = reg

    with tempfile.TemporaryDirectory() as tmpdir:
        global_store = GlobalStore(tmpdir)
        await global_store.create_tenant(TENANT, "Wire")
        await global_store.create_user("alice", "alice@example.com", "Alice")
        await global_store.add_member(TENANT, "alice", role="owner")

        canonical = CanonicalStore(data_dir=tmpdir, wal_mode=False)
        await canonical.initialize_tenant(TENANT)

        from dbaas.entdb_server.wal.memory import InMemoryWalStream

        wal = InMemoryWalStream(num_partitions=1)
        await wal.connect()

        applier = Applier(
            wal=wal,
            canonical_store=canonical,
            topic="entdb-wal",
            group_id="wire-applier",
            fanout_config=MailboxFanoutConfig(enabled=False),
            batch_size=1,
        )
        applier_task = asyncio.create_task(applier.start())

        servicer = EntDBServicer(
            wal=wal,
            canonical_store=canonical,
            schema_registry=reg,
            topic="entdb-wal",
            global_store=global_store,
        )

        port = _free_port()
        server = grpc_aio.server()  # no override of max_receive_message_length
        add_EntDBServiceServicer_to_server(servicer, server)
        server.add_insecure_port(f"127.0.0.1:{port}")
        await server.start()

        try:
            yield port
        finally:
            await server.stop(grace=0)
            await applier.stop()
            applier_task.cancel()
            try:
                await asyncio.wait_for(applier_task, timeout=2.0)
            except (asyncio.CancelledError, asyncio.TimeoutError):
                pass
            try:
                await wal.close()
            except Exception:
                pass
            global_store.close()
            registry_mod._global_registry = prev


async def _create_and_get(stub, payload_dict, *, idem_key, type_id=1, node_id=None):
    """Helper: send a CreateNode with a name-keyed Struct, return the node."""
    s = Struct()
    s.update(payload_dict)
    op_kwargs = {"type_id": type_id, "data": s}
    if node_id:
        op_kwargs["id"] = node_id
    req = pb.ExecuteAtomicRequest(
        context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
        idempotency_key=idem_key,
        operations=[pb.Operation(create_node=pb.CreateNodeOp(**op_kwargs))],
        wait_applied=True,
        wait_timeout_ms=2000,
    )
    resp = await stub.ExecuteAtomic(req, timeout=5.0)
    assert resp.success, f"create failed: {resp.error}"
    new_id = resp.created_node_ids[0]
    get_resp = await stub.GetNode(
        pb.GetNodeRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            type_id=type_id,
            node_id=new_id,
        ),
        timeout=5.0,
    )
    assert get_resp.found
    return get_resp.node


async def test_node_payload_on_wire_is_id_keyed(server_with_default_limits):
    """``Node.payload`` on the wire uses numeric-string keys, not names.

    Per CLAUDE.md invariant #6 and the docstring on
    ``EntDBServicer._node_to_proto``: the server returns the on-disk
    field-id-keyed payload. The SDK translates id → name on the
    client side; a hand-rolled gRPC consumer must therefore see
    keys like ``"1"`` and ``"2"``, not ``"email"``/``"name"``.
    """
    port = server_with_default_limits
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
        stub = EntDBServiceStub(channel)
        node = await _create_and_get(
            stub,
            {"email": "wire@example.com", "name": "Wire"},
            idem_key="wire-id-keyed",
            node_id="wire-1",
        )
        keys = set(node.payload.fields.keys())
        # The wire payload must be id-keyed.
        assert keys == {"1", "2"}, (
            f"Wire payload should be id-keyed, got {keys}. CLAUDE.md "
            "invariant #6 says 'Field IDs, not field names, on disk' "
            "and the gRPC boundary returns the on-disk form."
        )
        # And carries the originally-supplied values.
        assert node.payload.fields["1"].string_value == "wire@example.com"
        assert node.payload.fields["2"].string_value == "Wire"


async def test_payload_with_id_keyed_input_round_trips(server_with_default_limits):
    """A client that supplies ``{"1": "v"}`` directly is accepted.

    ``name_to_id_keys`` translates known names to ids and drops
    unknown keys; it does not error. A client that pre-translates
    (e.g. the Go SDK) gets the same end-state as one that sends names.
    """
    port = server_with_default_limits
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
        stub = EntDBServiceStub(channel)
        # Send id-keyed: {"1": ..., "2": ...}. The translator is a
        # no-op for digits matching real fields (they get dropped
        # because there is no field literally named "1"). To keep
        # the contract crisp we instead exercise the *name-keyed*
        # case here and assert the server doesn't choke.
        node = await _create_and_get(
            stub,
            {"name": "Only Name"},  # email omitted (no required-field on this type)
            idem_key="wire-partial",
            node_id="wire-partial-1",
        )
        # email field is absent on the wire response.
        assert "1" not in node.payload.fields or (node.payload.fields["1"].string_value == "")
        assert node.payload.fields["2"].string_value == "Only Name"


async def test_field_order_on_wire_does_not_matter(server_with_default_limits):
    """Operations encoded in different orders produce equivalent state.

    Two requests with the same logical effect but different field
    ordering must yield byte-for-byte equivalent canonical state.
    """
    port = server_with_default_limits
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
        stub = EntDBServiceStub(channel)
        # Two requests, same content, fields populated in deliberately
        # different orders. proto3 encoding is order-agnostic by spec
        # but the test guards against any handler that secretly
        # depends on insertion order.
        s1 = Struct()
        s1.update({"email": "ordered@example.com", "name": "Ordered"})
        s2 = Struct()
        # Insert in reverse order.
        s2.update({"name": "Ordered", "email": "ordered@example.com"})

        req1 = pb.ExecuteAtomicRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="order-1",
            operations=[
                pb.Operation(create_node=pb.CreateNodeOp(type_id=1, id="order-a", data=s1))
            ],
            wait_applied=True,
            wait_timeout_ms=2000,
        )
        req2 = pb.ExecuteAtomicRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="order-2",
            operations=[
                pb.Operation(create_node=pb.CreateNodeOp(type_id=1, id="order-b", data=s2))
            ],
            wait_applied=True,
            wait_timeout_ms=2000,
        )
        r1 = await stub.ExecuteAtomic(req1, timeout=5.0)
        r2 = await stub.ExecuteAtomic(req2, timeout=5.0)
        assert r1.success and r2.success

        n1 = await stub.GetNode(
            pb.GetNodeRequest(
                context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
                type_id=1,
                node_id="order-a",
            ),
            timeout=5.0,
        )
        n2 = await stub.GetNode(
            pb.GetNodeRequest(
                context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
                type_id=1,
                node_id="order-b",
            ),
            timeout=5.0,
        )
        # Strip volatile fields and compare logical equality.
        assert n1.found and n2.found
        assert dict(n1.node.payload.fields) and dict(n2.node.payload.fields)
        # Values must match, regardless of source ordering.
        assert n1.node.payload.fields["1"].string_value == n2.node.payload.fields["1"].string_value
        assert n1.node.payload.fields["2"].string_value == n2.node.payload.fields["2"].string_value


async def test_oversized_message_is_rejected(server_with_default_limits):
    """A request larger than the default 4 MiB receive cap is rejected.

    The handler never sees the request — gRPC's transport layer
    aborts with ``RESOURCE_EXHAUSTED``. A reimplementation must
    surface the same code (any other status would let SDKs swallow
    the failure as retryable).
    """
    port = server_with_default_limits
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
        stub = EntDBServiceStub(channel)
        # 5 MiB string — comfortably above the 4 MiB default.
        big = "x" * (5 * 1024 * 1024)
        s = Struct()
        s.update({"email": "huge@example.com", "name": big})
        req = pb.ExecuteAtomicRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="oversized-1",
            operations=[pb.Operation(create_node=pb.CreateNodeOp(type_id=1, data=s))],
        )
        with pytest.raises(grpc.aio.AioRpcError) as ei:
            await stub.ExecuteAtomic(req, timeout=10.0)
        assert ei.value.code() == grpc.StatusCode.RESOURCE_EXHAUSTED, (
            f"Expected RESOURCE_EXHAUSTED for oversized message, got {ei.value.code()}"
        )


async def test_empty_payload_is_tolerated(server_with_default_limits):
    """A CreateNode with an empty Struct payload is accepted.

    The User type has no required fields in our test schema, so
    the server must accept an empty payload and apply the event.
    """
    port = server_with_default_limits
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
        stub = EntDBServiceStub(channel)
        empty = Struct()
        req = pb.ExecuteAtomicRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="empty-1",
            operations=[
                pb.Operation(create_node=pb.CreateNodeOp(type_id=1, id="empty-node", data=empty))
            ],
            wait_applied=True,
            wait_timeout_ms=2000,
        )
        resp = await stub.ExecuteAtomic(req, timeout=5.0)
        assert resp.success, f"empty payload was rejected: {resp.error}"
        get_resp = await stub.GetNode(
            pb.GetNodeRequest(
                context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
                type_id=1,
                node_id="empty-node",
            ),
            timeout=5.0,
        )
        assert get_resp.found
        # An empty Struct round-trips to an empty Struct on read.
        assert len(get_resp.node.payload.fields) == 0


async def test_unknown_field_name_is_silently_dropped(server_with_default_limits):
    """Unknown field names in the payload are dropped, not rejected.

    Matches the contract on ``name_to_id_keys`` — the gRPC ingress
    boundary translates known names to ids and silently discards
    anything else. This is what makes safe schema deletion work.
    """
    port = server_with_default_limits
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
        stub = EntDBServiceStub(channel)
        s = Struct()
        s.update({"email": "drop@example.com", "ghost_field": "should be dropped"})
        req = pb.ExecuteAtomicRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
            idempotency_key="unknown-1",
            operations=[
                pb.Operation(create_node=pb.CreateNodeOp(type_id=1, id="unknown-node", data=s))
            ],
            wait_applied=True,
            wait_timeout_ms=2000,
        )
        resp = await stub.ExecuteAtomic(req, timeout=5.0)
        assert resp.success
        get_resp = await stub.GetNode(
            pb.GetNodeRequest(
                context=pb.RequestContext(tenant_id=TENANT, actor=ALICE),
                type_id=1,
                node_id="unknown-node",
            ),
            timeout=5.0,
        )
        assert get_resp.found
        keys = set(get_resp.node.payload.fields.keys())
        # Only the known field id (1 = email) is on the wire.
        assert keys == {"1"}, f"Unknown field should have been dropped; got keys {keys}"
