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


# ---------------------------------------------------------------------------
# Issue #545 — DeleteWhere against a schema-less server
# ---------------------------------------------------------------------------
#
# A server started without a schema (``--seed-profile none``) cannot
# resolve a FieldFilter.field NAME to a payload field id. The
# documented contract (mirroring QueryNodes' schema-optional path) is:
# a digit-only numeric field id works without a schema; a field NAME
# returns a clear INVALID_ARGUMENT. These exercise that end-to-end
# against a live schema-less Go subprocess.

SL_TENANT = "sl-545"
SL_TYPE_ID = 525  # caller's own (server-unknown) node type
SL_ADMIN = "system:admin"


def _sl_ctx() -> pb.RequestContext:
    # system:/admin: actors bypass tenant-membership checks
    # (execute_atomic.go:826); this test pins field-id resolution,
    # not ACL, so drive it as the tenant's creating admin.
    return pb.RequestContext(tenant_id=SL_TENANT, actor=SL_ADMIN)


async def _sl_provision_tenant(stub: EntDBServiceStub) -> None:
    """Create SL_TENANT on the schema-less server (no --seed-tenant).

    CreateTenant flows through the global WAL + applier asynchronously,
    so poll GetTenant until it materialises before any write.
    """
    import asyncio

    resp = await stub.CreateTenant(
        pb.CreateTenantRequest(actor=SL_ADMIN, tenant_id=SL_TENANT, name="SL 545")
    )
    assert resp.success, resp
    for _ in range(100):
        g = await stub.GetTenant(pb.GetTenantRequest(actor=SL_ADMIN, tenant_id=SL_TENANT))
        if g.found:
            return
        await asyncio.sleep(0.05)
    raise AssertionError(f"tenant {SL_TENANT!r} never materialised")


def _int_value(v: int) -> struct_pb2.Value:
    out = struct_pb2.Value()
    out.number_value = v
    return out


@pytest.fixture
async def schemaless_stub(schemaless_grpc_endpoint):
    async with grpc_aio.insecure_channel(schemaless_grpc_endpoint) as ch:
        stub = EntDBServiceStub(ch)
        await _sl_provision_tenant(stub)
        yield stub


async def _sl_seed(stub: EntDBServiceStub, rows: dict[str, int]) -> None:
    """Create id-keyed nodes on the schema-less server (field 4 = expires_at)."""
    ops = []
    for node_id, expires_at in rows.items():
        data = struct_pb2.Struct()
        # Schema-less => the payload itself is id-keyed ("4", not "expires_at").
        json_format.ParseDict({"4": expires_at}, data)
        ops.append(
            pb.Operation(create_node=pb.CreateNodeOp(type_id=SL_TYPE_ID, id=node_id, data=data))
        )
    resp = await stub.ExecuteAtomic(
        pb.ExecuteAtomicRequest(
            context=_sl_ctx(),
            idempotency_key=f"sl-seed-{uuid.uuid4().hex[:8]}",
            operations=ops,
            wait_applied=True,
            wait_timeout_ms=5000,
        )
    )
    assert resp.success, resp.error


async def _sl_exists(stub: EntDBServiceStub, node_id: str) -> bool:
    resp = await stub.GetNode(
        pb.GetNodeRequest(context=_sl_ctx(), type_id=SL_TYPE_ID, node_id=node_id)
    )
    return resp.found


async def test_delete_where_schemaless_by_numeric_field_id(schemaless_stub) -> None:
    """Schema-less server: DeleteWhere by NUMERIC field id sweeps the matches.

    Proves the issue #545 escape hatch works end-to-end: no schema on
    the server, no client registry — the digit-only ``field="4"`` is
    resolved to a raw payload field id and exactly the matching nodes
    are swept.
    """
    p = f"sln-{uuid.uuid4().hex[:6]}"
    await _sl_seed(
        schemaless_stub,
        {f"{p}-stale-1": 10, f"{p}-stale-2": 20, f"{p}-keep": 9999},
    )

    resp = await schemaless_stub.ExecuteAtomic(
        pb.ExecuteAtomicRequest(
            context=_sl_ctx(),
            idempotency_key=f"sl-sweep-{uuid.uuid4().hex[:8]}",
            operations=[
                pb.Operation(
                    delete_where=pb.DeleteWhereOp(
                        type_id=SL_TYPE_ID,
                        where=[
                            # NUMERIC field id — no schema needed.
                            pb.FieldFilter(
                                field="4",
                                op=pb.FilterOp.LT,
                                value=_int_value(100),
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

    assert not await _sl_exists(schemaless_stub, f"{p}-stale-1")
    assert not await _sl_exists(schemaless_stub, f"{p}-stale-2")
    assert await _sl_exists(schemaless_stub, f"{p}-keep")


async def test_delete_where_schemaless_field_name_rejected(schemaless_stub) -> None:
    """Schema-less server: a field NAME is a clear INVALID_ARGUMENT.

    The numeric id is the only schema-less route; a non-digit name is
    genuinely unresolvable and must surface the shared
    "cannot translate filter key … without a schema" wording.
    """
    with pytest.raises(grpc_aio.AioRpcError) as exc:
        await schemaless_stub.ExecuteAtomic(
            pb.ExecuteAtomicRequest(
                context=_sl_ctx(),
                idempotency_key=f"sl-name-{uuid.uuid4().hex[:8]}",
                operations=[
                    pb.Operation(
                        delete_where=pb.DeleteWhereOp(
                            type_id=SL_TYPE_ID,
                            where=[
                                pb.FieldFilter(
                                    field="expires_at",
                                    op=pb.FilterOp.LT,
                                    value=_int_value(100),
                                )
                            ],
                        )
                    )
                ],
            )
        )
    assert exc.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    msg = exc.value.details() or ""
    assert "cannot translate filter key" in msg
    assert "without a schema" in msg
