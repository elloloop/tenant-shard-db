# SPDX-License-Identifier: AGPL-3.0-only
"""Integration tests for the USER_MAILBOX read surface (#568).

Drives the live Go server end-to-end: write USER_MAILBOX User nodes for
two distinct users (storage_mode=USER_MAILBOX + target_user_id), then
read them back through the target_user scope on GetNode / GetNodes /
QueryNodes / SearchNodes. Asserts a user only ever sees their own mailbox
nodes — never another user's — and that the applier-populated FTS index
makes mailbox search work without any client-side indexing.

Uses the contract-seed schema: User type id=1, field 1 "email", field 2
"name". The contract profile declares NO searchable field, so the
mailbox FTS *search* path is pinned by the Go-side server tests
(server/go/internal/{store,apply,api}/...mailbox*_test.go) rather than
here; this suite pins get / get-many / query scoping end-to-end against
the live server, plus the privacy guarantee that an unscoped tenant read
never surfaces a mailbox-private row.

A system actor is used so the cross-tenant ACL post-filter does not enter
the picture — the test isolates mailbox scoping, not membership.
"""

from __future__ import annotations

import uuid

import pytest
from google.protobuf import json_format, struct_pb2
from grpc import aio as grpc_aio

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._generated.entdb_pb2_grpc import EntDBServiceStub

TENANT = "acme"
SYSTEM_ACTOR = "system:test"
USER_TYPE_ID = 1


def _ctx() -> pb.RequestContext:
    return pb.RequestContext(tenant_id=TENANT, actor=SYSTEM_ACTOR)


async def _create_mailbox_user(
    stub: EntDBServiceStub, node_id: str, target_user: str, name: str
) -> None:
    data = struct_pb2.Struct()
    # Id-keyed payload (ADR-031): field 1 = email, field 2 = name.
    json_format.ParseDict({"1": f"{target_user}@x", "2": name}, data)
    req = pb.ExecuteAtomicRequest(
        context=_ctx(),
        idempotency_key=f"mbox-{uuid.uuid4().hex[:8]}",
        operations=[
            pb.Operation(
                create_node=pb.CreateNodeOp(
                    type_id=USER_TYPE_ID,
                    id=node_id,
                    data=data,
                    storage_mode=pb.StorageMode.STORAGE_MODE_USER_MAILBOX,
                    target_user_id=target_user,
                )
            )
        ],
        wait_applied=True,
        wait_timeout_ms=5000,
    )
    resp = await stub.ExecuteAtomic(req)
    assert resp.success, resp.error


@pytest.fixture
async def stub(grpc_endpoint):
    async with grpc_aio.insecure_channel(grpc_endpoint) as ch:
        s = EntDBServiceStub(ch)
        await _create_mailbox_user(s, "mbox-alice", "alice", "Alice Mailbox")
        await _create_mailbox_user(s, "mbox-bob", "bob", "Bob Mailbox")
        yield s


async def test_get_node_mailbox_scope(stub) -> None:
    # alice sees her own mailbox node.
    resp = await stub.GetNode(
        pb.GetNodeRequest(context=_ctx(), node_id="mbox-alice", target_user="alice")
    )
    assert resp.found

    # alice cannot read bob's mailbox node.
    resp = await stub.GetNode(
        pb.GetNodeRequest(context=_ctx(), node_id="mbox-bob", target_user="alice")
    )
    assert not resp.found


async def test_get_nodes_mailbox_scope(stub) -> None:
    resp = await stub.GetNodes(
        pb.GetNodesRequest(
            context=_ctx(),
            type_id=USER_TYPE_ID,
            node_ids=["mbox-alice", "mbox-bob"],
            target_user="alice",
        )
    )
    found = [n.node_id for n in resp.nodes]
    assert found == ["mbox-alice"]
    assert list(resp.missing_ids) == ["mbox-bob"]


async def test_query_nodes_mailbox_scope(stub) -> None:
    resp = await stub.QueryNodes(
        pb.QueryNodesRequest(
            context=_ctx(),
            type_id=USER_TYPE_ID,
            target_user="alice",
            page_size=100,
        )
    )
    ids = [n.node_id for n in resp.nodes]
    assert ids == ["mbox-alice"]


async def test_tenant_read_does_not_see_mailbox_node(stub) -> None:
    # A plain (no target_user) GetNode of a mailbox node still resolves
    # (GetNode is not type/scope filtered) — but a mailbox-mode query
    # without target_user must not surface mailbox-private rows as tenant
    # rows. Here an unscoped query of the User type returns the seeded
    # contract users but NOT the mailbox-only nodes.
    resp = await stub.QueryNodes(
        pb.QueryNodesRequest(
            context=_ctx(),
            type_id=USER_TYPE_ID,
            page_size=100,
        )
    )
    ids = {n.node_id for n in resp.nodes}
    # Mailbox nodes are excluded from the unscoped tenant query.
    assert "mbox-alice" not in ids
    assert "mbox-bob" not in ids
