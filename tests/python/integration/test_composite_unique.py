# SPDX-License-Identifier: AGPL-3.0-only
"""Integration tests for composite (multi-field) unique constraints.

Issue #566. The contract schema (registered by the harness through a
self-describing write, ADR-031) carries an ``OAuthIdentity`` (type_id=201)
node type with a ``(provider, provider_user_id)`` composite unique
constraint (field-id tuple (1,2)) plus a single-field unique ``email``
(field 3). These tests drive the full
WAL -> applier -> SQLite composite-index path against the live Go
server and assert the violation surfaces as a gRPC ``ALREADY_EXISTS``
carrying the structured detail the SDK parsers consume.
"""

from __future__ import annotations

import uuid

import grpc
import pytest
from grpc import aio as grpc_aio

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

TENANT = "acme"
ACTOR = "user:alice"
OAUTH_TYPE_ID = 201


def _ctx() -> pb.RequestContext:
    return pb.RequestContext(tenant_id=TENANT, actor=ACTOR)


async def _create_oauth(
    stub: EntDBServiceStub,
    *,
    provider: str,
    provider_user_id: str,
    email: str,
    idem: str,
):
    op = pb.CreateNodeOp(
        type_id=OAUTH_TYPE_ID,
        id=f"oauth-{uuid.uuid4().hex[:8]}",
        typed_data={
            1: pb.EntValue(string_value=provider),
            2: pb.EntValue(string_value=provider_user_id),
            3: pb.EntValue(string_value=email),
        },
    )
    req = pb.ExecuteAtomicRequest(
        context=_ctx(),
        idempotency_key=idem,
        operations=[pb.Operation(create_node=op)],
        wait_applied=True,
        wait_timeout_ms=5000,
    )
    return await stub.ExecuteAtomic(req)


@pytest.fixture
async def stub(grpc_endpoint):
    async with grpc_aio.insecure_channel(grpc_endpoint) as ch:
        yield EntDBServiceStub(ch)


async def test_composite_unique_violation_already_exists(stub) -> None:
    """A duplicate (provider, provider_user_id) tuple surfaces as a
    gRPC ALREADY_EXISTS with the structured composite detail."""
    uid = uuid.uuid4().hex[:8]
    r1 = await _create_oauth(
        stub,
        provider="google",
        provider_user_id=f"uid-{uid}",
        email=f"a-{uid}@x.com",
        idem=f"oa-{uid}-1",
    )
    assert r1.success, r1.error

    with pytest.raises(grpc.aio.AioRpcError) as exc_info:
        await _create_oauth(
            stub,
            provider="google",
            provider_user_id=f"uid-{uid}",  # same composite tuple
            email=f"b-{uid}@x.com",  # distinct email -> only composite trips
            idem=f"oa-{uid}-2",
        )
    err = exc_info.value
    assert err.code() == grpc.StatusCode.ALREADY_EXISTS
    detail = err.details()
    assert "Composite unique constraint violation" in detail
    # NAME-FREE (ADR-031): the constraint identity is the field-id TUPLE
    # SIGNATURE, not a constraint name.
    assert "constraint='(1,2)'" in detail
    assert "fields=[1, 2]" in detail
    assert "type_id=201" in detail
    assert f"tenant={TENANT}" in detail
    assert "already exists" in detail


async def test_single_field_unique_violation_already_exists(stub) -> None:
    """A duplicate single-field unique email surfaces as a gRPC
    ALREADY_EXISTS with the single-field detail (distinct composite
    tuple, so only the email constraint trips)."""
    uid = uuid.uuid4().hex[:8]
    email = f"dup-{uid}@x.com"
    r1 = await _create_oauth(
        stub,
        provider="google",
        provider_user_id=f"g-{uid}",
        email=email,
        idem=f"sf-{uid}-1",
    )
    assert r1.success, r1.error

    with pytest.raises(grpc.aio.AioRpcError) as exc_info:
        await _create_oauth(
            stub,
            provider="github",  # distinct composite tuple
            provider_user_id=f"h-{uid}",
            email=email,  # duplicate single-field unique
            idem=f"sf-{uid}-2",
        )
    err = exc_info.value
    assert err.code() == grpc.StatusCode.ALREADY_EXISTS
    detail = err.details()
    assert "Unique constraint violation" in detail
    assert "field_id=3" in detail
    assert email in detail


async def test_composite_unique_distinct_tuples_both_apply(stub) -> None:
    """Distinct composite tuples both apply — the constraint only
    rejects exact-tuple duplicates."""
    uid = uuid.uuid4().hex[:8]
    r1 = await _create_oauth(
        stub,
        provider="google",
        provider_user_id=f"x-{uid}",
        email=f"x1-{uid}@x.com",
        idem=f"d-{uid}-1",
    )
    assert r1.success, r1.error
    # Same provider, different provider_user_id -> distinct tuple.
    r2 = await _create_oauth(
        stub,
        provider="google",
        provider_user_id=f"y-{uid}",
        email=f"x2-{uid}@x.com",
        idem=f"d-{uid}-2",
    )
    assert r2.success, r2.error


# ---------------------------------------------------------------------------
# v2.2 / issue #599 — single-RTT InsertIfNotExists for composite unique
# ---------------------------------------------------------------------------


async def test_composite_unique_insert_if_not_exists_skip(stub) -> None:
    """A second insert with the same (provider, provider_user_id)
    tuple, opting into ``on_conflict=skip``, resolves to the first
    writer's id in ONE round trip — no follow-up GetByCompositeKey
    needed (which doesn't exist in any released wire). v2.1.x
    couldn't resolve composite collisions; v2.2's server-side SKIP
    does because the lookup is keyed off the violated index, not an
    SDK-visible key token.
    """
    uid = uuid.uuid4().hex[:8]
    r1 = await _create_oauth(
        stub,
        provider="google",
        provider_user_id=f"skp-{uid}",
        email=f"skp1-{uid}@x.com",
        idem=f"skp-{uid}-1",
    )
    assert r1.success, r1.error
    first_id = r1.created_node_ids[0]
    assert first_id, "first create must mint a node id"

    # Second writer with the same composite tuple, on_conflict=skip.
    # The applier swallows the violation and returns the existing id
    # in existing_node_ids[0]. created_node_ids[0] is "" at the same
    # slot (index-aligned).
    op = pb.CreateNodeOp(
        type_id=OAUTH_TYPE_ID,
        id=f"oauth-{uuid.uuid4().hex[:8]}",
        typed_data={
            1: pb.EntValue(string_value="google"),
            2: pb.EntValue(string_value=f"skp-{uid}"),
            3: pb.EntValue(string_value=f"skp2-{uid}@x.com"),
        },
        on_conflict=pb.NodeConflictPolicy.NODE_CONFLICT_POLICY_SKIP,
    )
    req = pb.ExecuteAtomicRequest(
        context=_ctx(),
        idempotency_key=f"skp-{uid}-2",
        operations=[pb.Operation(create_node=op)],
        wait_applied=True,
        wait_timeout_ms=5000,
    )
    r2 = await stub.ExecuteAtomic(req)
    assert r2.success, r2.error
    assert list(r2.existing_node_ids) == [first_id], (
        f"expected existing_node_ids=[{first_id!r}], got {list(r2.existing_node_ids)!r}"
    )
    # The created-side slot is empty at the same index — exactly one
    # of the two parallel arrays carries the id.
    assert list(r2.created_node_ids) == [""], (
        f"expected created_node_ids=[''], got {list(r2.created_node_ids)!r}"
    )
