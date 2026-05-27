# SPDX-License-Identifier: AGPL-3.0-only
"""Unit tests for ``ActorScope.insert_if_not_exists`` (issue #599).

The helper closes the racy ``get_by_key → create`` idiom: under N
concurrent writers, exactly one wins and the rest learn the existing
canonical node id without a user-visible second round trip. v2.1.0
ships an SDK-only wrapper around the primitives already in v2.0.x
(Plan + WaitApplied + typed UniqueConstraintError + GetNodeByKey);
the single-round-trip server path is v2.2.

These tests pin:

  - HappyPath:        commit succeeds → ``(created, None)``;
                      ``wait_applied=True`` is forced (issue #606).
  - SingleField:      commit raises a single-field ``UCE`` →
                      GetNodeByKey lookup → ``(None, existing)``.
  - Composite:        commit raises a composite ``UCE`` → re-raised
                      (no GetByCompositeKey RPC in v2.x).
  - Unrelated:        non-UCE error bubbles up unchanged.
  - NilMsg:           precondition short-circuits before any RPC.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from entdb_sdk import register_proto_schema
from entdb_sdk._grpc_client import GrpcCommitResult, GrpcReceipt, Node
from entdb_sdk.client import DbClient
from entdb_sdk.errors import UniqueConstraintError
from entdb_sdk.registry import get_registry, reset_registry
from tests.python._test_schemas import test_schema_pb2 as ts


@pytest.fixture
def proto_client():
    """A ``DbClient`` with the v0.3 test schema registered."""
    reset_registry()
    register_proto_schema(ts)
    c = DbClient("localhost:50051")
    c._connected = True
    c.registry = get_registry()
    c._grpc = AsyncMock()
    yield c
    reset_registry()


def _product():
    return ts.Product(sku="SKU-1", name="Widget", price_cents=100)


def _commit_ok() -> GrpcCommitResult:
    return GrpcCommitResult(
        success=True,
        receipt=GrpcReceipt(
            tenant_id="t1",
            idempotency_key="k1",
            stream_position="pos_1",
        ),
        created_node_ids=["new-id-1"],
        applied=True,
        error=None,
    )


def _existing_node(node_id: str = "existing-id-7") -> Node:
    return Node(
        tenant_id="t1",
        node_id=node_id,
        type_id=9001,
        payload={"sku": "SKU-1"},
        created_at=1000,
        updated_at=1000,
        owner_actor="user:alice",
        acl=[],
    )


async def test_happy_path_returns_created_id_and_forces_wait_applied(proto_client):
    proto_client._grpc.execute_atomic = AsyncMock(return_value=_commit_ok())
    scope = proto_client.tenant("t1").actor("user:alice")

    created, existed = await scope.insert_if_not_exists(_product(), idempotency_key="k1")

    assert created == "new-id-1"
    assert existed is None
    # WaitApplied must be forced on so the applier's outcome
    # surfaces synchronously (issue #606).
    call = proto_client._grpc.execute_atomic.call_args
    assert call.kwargs.get("wait_applied") is True


async def test_single_field_conflict_resolves_to_existing(proto_client):
    uce = UniqueConstraintError(
        message="Unique constraint violation",
        tenant_id="t1",
        type_id=9001,
        field_id=1,
        value="SKU-1",
    )
    proto_client._grpc.execute_atomic = AsyncMock(side_effect=uce)
    proto_client._grpc.get_node_by_key = AsyncMock(return_value=_existing_node())
    scope = proto_client.tenant("t1").actor("user:alice")

    created, existed = await scope.insert_if_not_exists(_product(), idempotency_key="k1")

    assert created is None
    assert existed == "existing-id-7"
    proto_client._grpc.get_node_by_key.assert_awaited_once()
    call = proto_client._grpc.get_node_by_key.call_args
    # The lookup keys come from the UCE, not from caller-supplied state —
    # this is the whole point of the typed error.
    assert call.kwargs["type_id"] == 9001
    assert call.kwargs["field_id"] == 1
    assert call.kwargs["value"] == "SKU-1"


async def test_composite_conflict_reraises_without_lookup(proto_client):
    uce = UniqueConstraintError(
        message="Composite unique constraint violation",
        tenant_id="t1",
        type_id=9001,
        constraint_name="(1,2)",
        field_ids=(1, 2),
        values=("google", "uid-1"),
    )
    proto_client._grpc.execute_atomic = AsyncMock(side_effect=uce)
    proto_client._grpc.get_node_by_key = AsyncMock()
    scope = proto_client.tenant("t1").actor("user:alice")

    with pytest.raises(UniqueConstraintError) as ei:
        await scope.insert_if_not_exists(_product(), idempotency_key="k1")
    assert ei.value.is_composite
    proto_client._grpc.get_node_by_key.assert_not_awaited()


async def test_unrelated_error_propagates(proto_client):
    boom = RuntimeError("transport: deadline exceeded")
    proto_client._grpc.execute_atomic = AsyncMock(side_effect=boom)
    proto_client._grpc.get_node_by_key = AsyncMock()
    scope = proto_client.tenant("t1").actor("user:alice")

    with pytest.raises(RuntimeError, match="deadline exceeded"):
        await scope.insert_if_not_exists(_product(), idempotency_key="k1")
    proto_client._grpc.get_node_by_key.assert_not_awaited()


async def test_nil_msg_raises_before_any_rpc(proto_client):
    proto_client._grpc.execute_atomic = AsyncMock()
    scope = proto_client.tenant("t1").actor("user:alice")
    with pytest.raises(TypeError):
        await scope.insert_if_not_exists(None, idempotency_key="k1")
    proto_client._grpc.execute_atomic.assert_not_awaited()
