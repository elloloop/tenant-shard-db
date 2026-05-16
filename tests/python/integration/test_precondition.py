# SPDX-License-Identifier: AGPL-3.0-only
"""Integration tests for conditional UpdateNode (CAS / issue #500).

The SDK's ``Plan.update(..., precondition=("field", expected))``
surface is exercised end-to-end against the live Go server. The
applier evaluates the predicate against the materialized SQLite
state inside the WAL apply path; on mismatch the WHOLE batch
aborts (no ops commit) and the failure is memoized in the
idempotency cache.
"""

from __future__ import annotations

import uuid

import pytest
from google.protobuf import struct_pb2
from grpc import aio as grpc_aio

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

TENANT = "acme"
ACTOR = "user:alice"


def _ctx() -> pb.RequestContext:
    return pb.RequestContext(tenant_id=TENANT, actor=ACTOR)


def _string(v: str) -> struct_pb2.Value:
    out = struct_pb2.Value()
    out.string_value = v
    return out


async def _seed_user(stub: EntDBServiceStub, node_id: str, email: str) -> None:
    data = struct_pb2.Struct()
    data.update({"email": email, "name": email.split("@")[0]})
    req = pb.ExecuteAtomicRequest(
        context=_ctx(),
        idempotency_key=f"seed-{node_id}-{uuid.uuid4().hex[:8]}",
        operations=[pb.Operation(create_node=pb.CreateNodeOp(type_id=1, id=node_id, data=data))],
        wait_applied=True,
        wait_timeout_ms=5000,
    )
    resp = await stub.ExecuteAtomic(req)
    assert resp.success, resp.error


async def _update_with_precondition(
    stub: EntDBServiceStub,
    node_id: str,
    new_email: str,
    pre_field: str,
    pre_expected: str,
    *,
    idem_key: str,
    pre_field_id: int | None = 1,
) -> pb.ExecuteAtomicResponse:
    patch = struct_pb2.Struct()
    patch.update({"email": new_email})
    req = pb.ExecuteAtomicRequest(
        context=_ctx(),
        idempotency_key=idem_key,
        operations=[
            pb.Operation(
                update_node=pb.UpdateNodeOp(
                    type_id=1,
                    id=node_id,
                    patch=patch,
                    precondition=pb.UpdateNodePrecondition(
                        field=pre_field,
                        field_id=pre_field_id or 0,
                        equals=_string(pre_expected),
                    ),
                )
            )
        ],
        wait_applied=True,
        wait_timeout_ms=5000,
    )
    return await stub.ExecuteAtomic(req)


async def _read_email(stub: EntDBServiceStub, node_id: str) -> str:
    resp = await stub.GetNode(pb.GetNodeRequest(context=_ctx(), type_id=1, node_id=node_id))
    assert resp.found
    return resp.node.payload.fields["1"].string_value


@pytest.fixture
async def stub(grpc_endpoint):
    async with grpc_aio.insecure_channel(grpc_endpoint) as ch:
        yield EntDBServiceStub(ch)


async def test_precondition_met_applies(stub) -> None:
    """When precondition matches, the patch commits."""
    nid = f"u-cas-{uuid.uuid4().hex[:8]}"
    await _seed_user(stub, nid, "old@x")
    resp = await _update_with_precondition(
        stub,
        nid,
        new_email="new@x",
        pre_field="email",
        pre_expected="old@x",
        idem_key=f"upd-{uuid.uuid4().hex[:8]}",
    )
    assert resp.success
    assert resp.applied_status == pb.ReceiptStatus.RECEIPT_STATUS_APPLIED
    assert await _read_email(stub, nid) == "new@x"


async def test_precondition_miss_aborts_batch(stub) -> None:
    """When precondition fails, no ops commit; response carries detail."""
    nid = f"u-cas-{uuid.uuid4().hex[:8]}"
    await _seed_user(stub, nid, "old@x")
    resp = await _update_with_precondition(
        stub,
        nid,
        new_email="new@x",
        pre_field="email",
        pre_expected="wrong@x",
        idem_key=f"upd-{uuid.uuid4().hex[:8]}",
    )
    assert not resp.success
    assert resp.applied_status == pb.ReceiptStatus.RECEIPT_STATUS_FAILED_PRECONDITION
    assert resp.error_code == "FAILED_PRECONDITION"
    assert resp.precondition_failure.field == "email"
    assert resp.precondition_failure.expected.string_value == "wrong@x"
    assert resp.precondition_failure.observed.string_value == "old@x"
    # Patch did NOT commit — original value still there.
    assert await _read_email(stub, nid) == "old@x"


async def test_precondition_failure_is_idempotent(stub) -> None:
    """Retry with same idempotency key after a CAS miss returns the same cached
    failure WITHOUT re-evaluating, even if state has since changed."""
    nid = f"u-cas-{uuid.uuid4().hex[:8]}"
    await _seed_user(stub, nid, "old@x")
    idem = f"upd-{uuid.uuid4().hex[:8]}"

    # First attempt fails (expected wrong@x, observed old@x).
    r1 = await _update_with_precondition(stub, nid, "new@x", "email", "wrong@x", idem_key=idem)
    assert r1.applied_status == pb.ReceiptStatus.RECEIPT_STATUS_FAILED_PRECONDITION

    # Now mutate the node so that "wrong@x" would actually match.
    patch = struct_pb2.Struct()
    patch.update({"email": "wrong@x"})
    await stub.ExecuteAtomic(
        pb.ExecuteAtomicRequest(
            context=_ctx(),
            idempotency_key=f"mut-{uuid.uuid4().hex[:8]}",
            operations=[pb.Operation(update_node=pb.UpdateNodeOp(type_id=1, id=nid, patch=patch))],
            wait_applied=True,
            wait_timeout_ms=5000,
        )
    )
    assert await _read_email(stub, nid) == "wrong@x"

    # Retry with the SAME idempotency key. Must return the cached
    # failure — NOT re-evaluate. Caller must mint a fresh idem key
    # to re-evaluate.
    r2 = await _update_with_precondition(stub, nid, "new@x", "email", "wrong@x", idem_key=idem)
    assert r2.applied_status == pb.ReceiptStatus.RECEIPT_STATUS_FAILED_PRECONDITION
    # State is unchanged by the replay.
    assert await _read_email(stub, nid) == "wrong@x"


async def test_unknown_precondition_field_is_invalid_argument(stub) -> None:
    """Unknown precondition field is rejected at the handler — WAL never
    sees a precondition that can't be evaluated."""
    nid = f"u-cas-{uuid.uuid4().hex[:8]}"
    await _seed_user(stub, nid, "old@x")
    with pytest.raises(grpc_aio.AioRpcError) as exc:
        await _update_with_precondition(
            stub,
            nid,
            new_email="new@x",
            pre_field="does_not_exist",
            pre_expected="old@x",
            idem_key=f"upd-{uuid.uuid4().hex[:8]}",
            pre_field_id=None,
        )
    import grpc

    assert exc.value.code() == grpc.StatusCode.INVALID_ARGUMENT


async def test_sdk_plan_update_with_precondition_raises_typed_error(
    grpc_endpoint,
) -> None:
    """End-to-end through the high-level SDK gRPC client: a CAS miss
    raises ``PreconditionFailedError`` with structured detail."""
    from entdb_sdk import DbClient, PreconditionFailedError

    nid = f"u-cas-{uuid.uuid4().hex[:8]}"
    async with grpc_aio.insecure_channel(grpc_endpoint) as ch:
        await _seed_user(EntDBServiceStub(ch), nid, "old@x")

    client = DbClient(address=grpc_endpoint)
    await client.connect()
    try:
        idem = f"sdk-{uuid.uuid4().hex[:8]}"
        with pytest.raises(PreconditionFailedError) as excinfo:
            await client._grpc.execute_atomic(
                tenant_id=TENANT,
                actor=ACTOR,
                operations=[
                    {
                        "update_node": {
                            "type_id": 1,
                            "id": nid,
                            "patch": {"1": "new@x"},
                            "precondition": {
                                "field": "email",
                                "field_id": 1,
                                "equals": "wrong@x",
                            },
                        }
                    }
                ],
                idempotency_key=idem,
            )
        err = excinfo.value
        assert err.field == "email"
        assert err.expected == "wrong@x"
        assert err.observed == "old@x"
        assert err.op_index == 0
    finally:
        await client.close()
