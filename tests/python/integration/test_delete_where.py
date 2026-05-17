# SPDX-License-Identifier: AGPL-3.0-only
"""Integration tests for the DeleteWhere sweeper op (issue #504).

DeleteWhere collapses the TTL-sweeper "QueryNodes for matching ids,
then ExecuteAtomic to delete them" loop into a single round-trip op
inside ExecuteAtomic. It reuses the issue-#501 FieldFilter predicate
shape verbatim, so the wire surface is exercised here end-to-end
against the live Go server (server handler -> WAL -> applier).

The contract-seed registry declares the ``User`` type (id=1) with
``email`` (field 1) and ``name`` (field 2). SQLite TEXT comparison is
lexicographic so range predicates on ``email`` cover the integer
expires-at sweeper use case's code path too.
"""

from __future__ import annotations

import uuid

import grpc
import pytest
from google.protobuf import json_format, struct_pb2
from grpc import aio as grpc_aio

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

TENANT = "acme"
ACTOR = "user:alice"


def _ctx() -> pb.RequestContext:
    return pb.RequestContext(tenant_id=TENANT, actor=ACTOR)


def _value(v: str) -> struct_pb2.Value:
    out = struct_pb2.Value()
    out.string_value = v
    return out


async def _seed(stub: EntDBServiceStub, prefix: str, emails: list[str]) -> None:
    ops = []
    for email in emails:
        data = struct_pb2.Struct()
        json_format.ParseDict({"email": email, "name": email.split("@")[0]}, data)
        ops.append(
            pb.Operation(create_node=pb.CreateNodeOp(type_id=1, id=f"{prefix}-{email}", data=data))
        )
    resp = await stub.ExecuteAtomic(
        pb.ExecuteAtomicRequest(
            context=_ctx(),
            idempotency_key=f"dw-seed-{uuid.uuid4().hex[:8]}",
            operations=ops,
            wait_applied=True,
            wait_timeout_ms=5000,
        )
    )
    assert resp.success, resp.error


async def _exists(stub: EntDBServiceStub, node_id: str) -> bool:
    resp = await stub.GetNode(pb.GetNodeRequest(context=_ctx(), type_id=1, node_id=node_id))
    return resp.found


@pytest.fixture
async def stub(grpc_endpoint):
    async with grpc_aio.insecure_channel(grpc_endpoint) as ch:
        yield EntDBServiceStub(ch)


async def test_delete_where_sweeps_only_matching(stub) -> None:
    """A predicate sweep removes exactly the matching nodes."""
    p = f"dw1-{uuid.uuid4().hex[:6]}"
    await _seed(stub, p, ["a@x", "b@x", "z@x"])

    # Sweep everything with email < "m@x" (=> a@x, b@x), keep z@x.
    resp = await stub.ExecuteAtomic(
        pb.ExecuteAtomicRequest(
            context=_ctx(),
            idempotency_key=f"dw-sweep-{uuid.uuid4().hex[:8]}",
            operations=[
                pb.Operation(
                    delete_where=pb.DeleteWhereOp(
                        type_id=1,
                        where=[
                            pb.FieldFilter(
                                field="email",
                                op=pb.FilterOp.LT,
                                value=_value("m@x"),
                            )
                        ],
                    )
                )
            ],
            wait_applied=True,
            wait_timeout_ms=5000,
        )
    )
    assert resp.success, resp.error

    assert not await _exists(stub, f"{p}-a@x")
    assert not await _exists(stub, f"{p}-b@x")
    assert await _exists(stub, f"{p}-z@x")


async def test_delete_where_empty_predicate_rejected(stub) -> None:
    """An unconditional bulk delete is INVALID_ARGUMENT."""
    with pytest.raises(grpc_aio.AioRpcError) as exc:
        await stub.ExecuteAtomic(
            pb.ExecuteAtomicRequest(
                context=_ctx(),
                idempotency_key=f"dw-bad-{uuid.uuid4().hex[:8]}",
                operations=[pb.Operation(delete_where=pb.DeleteWhereOp(type_id=1))],
            )
        )
    assert exc.value.code() == grpc.StatusCode.INVALID_ARGUMENT


async def _sweep_cd(stub: EntDBServiceStub, idem: str, limit: int) -> None:
    """Sweep every cd*@x.example User (a stable, prefix-free predicate)."""
    resp = await stub.ExecuteAtomic(
        pb.ExecuteAtomicRequest(
            context=_ctx(),
            idempotency_key=idem,
            operations=[
                pb.Operation(
                    delete_where=pb.DeleteWhereOp(
                        type_id=1,
                        limit=limit,
                        where=[
                            pb.FieldFilter(
                                field="email",
                                op=pb.FilterOp.GTE,
                                value=_value("cd0@x"),
                            ),
                            pb.FieldFilter(
                                field="email",
                                op=pb.FilterOp.LT,
                                value=_value("cd9@x"),
                            ),
                        ],
                    )
                )
            ],
            wait_applied=True,
            wait_timeout_ms=5000,
        )
    )
    assert resp.success, resp.error


async def test_delete_where_limit_is_best_effort(stub) -> None:
    """limit caps the per-op deletes; a second sweep drains the rest."""
    p = f"dw2-{uuid.uuid4().hex[:6]}"
    await _seed(stub, p, ["cd1@x", "cd2@x", "cd3@x"])

    # First sweep with limit=2 deletes at most 2 of the 3 matches.
    await _sweep_cd(stub, f"dw-lim-1-{uuid.uuid4().hex[:8]}", limit=2)
    remaining = 0
    for n in ("cd1@x", "cd2@x", "cd3@x"):
        if await _exists(stub, f"{p}-{n}"):
            remaining += 1
    assert remaining == 1, f"limit=2 should leave exactly 1, got {remaining}"

    # Second sweep (no limit) drains the survivor.
    await _sweep_cd(stub, f"dw-lim-2-{uuid.uuid4().hex[:8]}", limit=0)
    remaining = 0
    for n in ("cd1@x", "cd2@x", "cd3@x"):
        if await _exists(stub, f"{p}-{n}"):
            remaining += 1
    assert remaining == 0, f"second sweep should drain all, got {remaining}"
