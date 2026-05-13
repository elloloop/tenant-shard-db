# SPDX-License-Identifier: AGPL-3.0-only
"""Integration tests for comparison operators on QueryNodes (issue #501).

The Python SDK's typed ``Filter``/``FilterOp`` surface is exercised
end-to-end against the live Go server. All filters AND-ed, no OR / no
nesting / no cursor — the v1 cut from the issue.

The contract-seed registry only declares the ``User`` type (id=1) with
two string fields ``email`` and ``name``. SQLite's TEXT comparison is
lexicographic, so range operators on ``email`` exercise the same code
path the integer sweeper use case will hit in production.
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


async def _seed_users(stub: EntDBServiceStub, emails: list[str]) -> None:
    """Create one User per email via ExecuteAtomic."""
    ops = []
    for email in emails:
        data = struct_pb2.Struct()
        json_format.ParseDict({"email": email, "name": email.split("@")[0]}, data)
        ops.append(
            pb.Operation(
                create_node=pb.CreateNodeOp(
                    type_id=1,
                    id=f"u-{email}",
                    data=data,
                )
            )
        )
    req = pb.ExecuteAtomicRequest(
        context=_ctx(),
        idempotency_key=f"range-seed-{uuid.uuid4().hex[:8]}",
        operations=ops,
        wait_applied=True,
        wait_timeout_ms=5000,
    )
    resp = await stub.ExecuteAtomic(req)
    assert resp.success, resp.error


def _value(v) -> struct_pb2.Value:
    out = struct_pb2.Value()
    out.string_value = v
    return out


def _email(n: pb.Node) -> str:
    # Payload wire format is id-keyed (CLAUDE.md invariant #6 +
    # tests/python/unit/test_payload_wire_format.py pin); field 1 is
    # "email" per the contract-seed User type.
    return n.payload.fields["1"].string_value


async def _query_emails(stub: EntDBServiceStub, op: pb.FilterOp, value: str) -> list[str]:
    resp = await stub.QueryNodes(
        pb.QueryNodesRequest(
            context=_ctx(),
            type_id=1,
            order_by="node_id",
            descending=False,
            limit=100,
            filters=[pb.FieldFilter(field="email", op=op, value=_value(value))],
        )
    )
    return sorted(_email(n) for n in resp.nodes if _email(n).startswith("rng-"))


@pytest.fixture
async def stub(grpc_endpoint):
    async with grpc_aio.insecure_channel(grpc_endpoint) as ch:
        s = EntDBServiceStub(ch)
        await _seed_users(s, ["rng-a@x", "rng-b@x", "rng-c@x"])
        yield s


async def test_lt(stub) -> None:
    assert await _query_emails(stub, pb.FilterOp.LT, "rng-b@x") == ["rng-a@x"]


async def test_lte(stub) -> None:
    assert await _query_emails(stub, pb.FilterOp.LTE, "rng-b@x") == ["rng-a@x", "rng-b@x"]


async def test_gt(stub) -> None:
    assert await _query_emails(stub, pb.FilterOp.GT, "rng-b@x") == ["rng-c@x"]


async def test_gte(stub) -> None:
    assert await _query_emails(stub, pb.FilterOp.GTE, "rng-b@x") == ["rng-b@x", "rng-c@x"]


async def test_eq(stub) -> None:
    assert await _query_emails(stub, pb.FilterOp.EQ, "rng-b@x") == ["rng-b@x"]


async def test_neq(stub) -> None:
    assert await _query_emails(stub, pb.FilterOp.NEQ, "rng-b@x") == ["rng-a@x", "rng-c@x"]


async def test_and_of_two_filters(stub) -> None:
    """Half-open range: email >= rng-a@x AND email < rng-c@x."""
    resp = await stub.QueryNodes(
        pb.QueryNodesRequest(
            context=_ctx(),
            type_id=1,
            order_by="node_id",
            descending=False,
            limit=100,
            filters=[
                pb.FieldFilter(field="email", op=pb.FilterOp.GTE, value=_value("rng-a@x")),
                pb.FieldFilter(field="email", op=pb.FilterOp.LT, value=_value("rng-c@x")),
            ],
        )
    )
    got = sorted(_email(n) for n in resp.nodes if _email(n).startswith("rng-"))
    assert got == ["rng-a@x", "rng-b@x"]


async def test_limit_bounded(stub) -> None:
    """Limit caps the result count even when more rows match."""
    resp = await stub.QueryNodes(
        pb.QueryNodesRequest(
            context=_ctx(),
            type_id=1,
            order_by="node_id",
            descending=False,
            limit=2,
            filters=[
                pb.FieldFilter(field="email", op=pb.FilterOp.GTE, value=_value("rng-")),
            ],
        )
    )
    assert len(resp.nodes) <= 2


async def test_contains_rejected(stub) -> None:
    """CONTAINS is still INVALID_ARGUMENT — issue #501 deferred it."""
    with pytest.raises(grpc_aio.AioRpcError) as exc:
        await stub.QueryNodes(
            pb.QueryNodesRequest(
                context=_ctx(),
                type_id=1,
                filters=[
                    pb.FieldFilter(
                        field="email",
                        op=pb.FilterOp.CONTAINS,
                        value=_value("alice"),
                    ),
                ],
            )
        )
    assert exc.value.code() == grpc.StatusCode.INVALID_ARGUMENT
