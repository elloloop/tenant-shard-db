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


async def test_server_prefers_typed_value_over_legacy(stub) -> None:
    """#572 / ADR-028: when a filter carries both the legacy struct
    ``value`` and the typed ``typed_value``, the server resolves the
    predicate from ``typed_value``. Pinned with a discriminating pair —
    the legacy value points at a row that does not exist, the typed
    value at one that does. A pass proves the server reads typed; a
    regression to the legacy path would return nothing.

    This is the production guarantee behind the SDK dual-write: big-int
    keys (>2^53) round-trip losslessly only because the typed branch
    wins, and the contract-seed schema has no int64 field to exercise
    that directly, so the preference itself is what we pin here."""
    from entdb_sdk._generated import EntValue

    resp = await stub.QueryNodes(
        pb.QueryNodesRequest(
            context=_ctx(),
            type_id=1,
            order_by="node_id",
            descending=False,
            limit=100,
            filters=[
                pb.FieldFilter(
                    field="email",
                    op=pb.FilterOp.EQ,
                    value=_value("rng-DOES-NOT-EXIST@x"),
                    typed_value=EntValue(string_value="rng-b@x"),
                )
            ],
        )
    )
    got = sorted(_email(n) for n in resp.nodes if _email(n).startswith("rng-"))
    assert got == ["rng-b@x"], "server must prefer typed_value over legacy value"


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


async def test_query_does_not_silently_truncate(grpc_endpoint) -> None:
    """A read must return ALL rows the caller wrote, not silently cap at 100.

    Characterizes the customer-reported symptom ("List returned 100 of 250")
    and pins the ADR-029 fix. Seeds 150 rows isolated by a unique half-open
    email range so the server-side filtered set is exactly 150, then reads
    with no explicit page_size — so the server falls back to the 100-row
    default — and follows next_page_token to exhaustion.

    The contract (ADR-029 invariant 3): the first page is capped at 100 but
    MUST carry a non-empty next_page_token, and following it retrieves the
    remainder. A read of N rows yields N rows, never a silent prefix.
    """
    n = 150
    prefix = f"pag-{uuid.uuid4().hex[:6]}-"
    upper = prefix + "~"  # 0x7E > any digit/'@'/'x' in the seeded suffixes
    async with grpc_aio.insecure_channel(grpc_endpoint) as ch:
        s = EntDBServiceStub(ch)
        await _seed_users(s, [f"{prefix}{i:04d}@x" for i in range(n)])

        def _req(token: str) -> pb.QueryNodesRequest:
            return pb.QueryNodesRequest(
                context=_ctx(),
                type_id=1,
                order_by="node_id",
                descending=False,
                # No explicit page_size — mirrors the SDK list/query helpers,
                # so the server applies the 100-row default.
                page_token=token,
                filters=[
                    pb.FieldFilter(field="email", op=pb.FilterOp.GTE, value=_value(prefix)),
                    pb.FieldFilter(field="email", op=pb.FilterOp.LT, value=_value(upper)),
                ],
            )

        # First page: capped at the 100-row default, but the cap is now
        # *visible* — a non-empty cursor means "there is more".
        first = await s.QueryNodes(_req(""))
        assert len(first.nodes) == 100, f"default page = {len(first.nodes)}, want 100"
        assert first.next_page_token, "truncated page carried no cursor (silent truncation, Bug A)"

        # Follow the cursor to exhaustion, accumulating distinct node_ids.
        seen: set[str] = set()
        token = ""
        pages = 0
        while True:
            pages += 1
            assert pages <= 1000, "cursor did not terminate"
            resp = await s.QueryNodes(_req(token))
            for node in resp.nodes:
                if _email(node).startswith(prefix):
                    assert node.node_id not in seen, f"duplicate row {node.node_id} across pages"
                    seen.add(node.node_id)
            token = resp.next_page_token
            if not token:
                break

        assert len(seen) == n, f"wrote {n} rows, read back {len(seen)} — silent truncation (Bug A)"
