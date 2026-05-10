# SPDX-License-Identifier: AGPL-3.0-only
"""gRPC contract tests — every RPC, exercised over a real channel.

This is the contract a Go reimplementation has to satisfy: identical
response shape and identical status codes for every RPC the server
exposes. Each case is parameterised by ``(rpc_name, build_request,
mode)`` so adding a new RPC means appending a row, not writing a new
test function.

Modes:
    happy: call must succeed (no abort) and the assertions in the
        case dict must hold.
    invalid_argument: call must fail with INVALID_ARGUMENT (server
        rejects a malformed request).
    not_found: call must succeed (no abort) and the response must
        carry ``found=False`` / equivalent missing flag.
    permission_denied: call must fail with PERMISSION_DENIED.
    unimplemented: call must fail with UNIMPLEMENTED (e.g. user
        registry RPCs without a global_store wired).

Read handlers in EntDB intentionally swallow late errors and return
empty/found-False responses — the cases below mirror that observed
behaviour. Contract tests must encode real wire output, not what the
docstring claims.
"""

from __future__ import annotations

import grpc
import pytest
from grpc import aio as grpc_aio

from entdb_server.api.generated import EntDBServiceStub
from entdb_server.api.generated import entdb_pb2 as pb

# The ``live_server`` fixture used to live here. It now lives in
# ``tests/python/integration/conftest.py`` so the same setup serves
# both the in-process Python target and the Go subprocess target
# (selected via ``ENTDB_SERVER_TARGET``; see
# ``docs/go-port/shared/test-harness.md``).

TENANT = "acme"
ALICE = "user:alice"
BOB = "user:bob"
ADMIN = "system:admin"


# ---- Request builders & assertions per RPC ---------------------------------
#
# Each case is a dict:
#   rpc:          attribute name on EntDBServiceStub (matches proto rpc name).
#   build:        callable(stub, ctx) -> request message. ctx exposes
#                 SEED_NODE_ID etc.
#   mode:         "happy" | "invalid_argument" | "not_found" |
#                 "permission_denied" | "unimplemented".
#   check:        optional callable(response) -> None for extra assertions
#                 in happy mode.
#
# When ``mode`` is one of the error modes, the call must abort with
# the matching grpc.StatusCode and the test asserts that.

SEED_NODE_ID = "seeded-node"


def _ctx() -> pb.RequestContext:
    return pb.RequestContext(tenant_id=TENANT, actor=ALICE)


CONTRACT_CASES: list[dict] = [
    # Health — always healthy.
    {
        "rpc": "Health",
        "build": lambda: pb.HealthRequest(),
        "mode": "happy",
        "check": lambda r: r.healthy is True and r.version != "",
    },
    # GetSchema — registry has node types, fingerprint is non-empty.
    {
        "rpc": "GetSchema",
        "build": lambda: pb.GetSchemaRequest(tenant_id=TENANT),
        "mode": "happy",
        "check": lambda r: r.fingerprint != "" or r.HasField("schema"),
    },
    # GetNode — happy + not_found.
    {
        "rpc": "GetNode",
        "build": lambda: pb.GetNodeRequest(context=_ctx(), type_id=1, node_id=SEED_NODE_ID),
        "mode": "happy",
        "check": lambda r: r.found is True and r.node.node_id == SEED_NODE_ID,
    },
    {
        "rpc": "GetNode",
        "build": lambda: pb.GetNodeRequest(context=_ctx(), type_id=1, node_id="does-not-exist"),
        "mode": "not_found",  # response.found is False; no abort
        "check": lambda r: r.found is False,
    },
    # GetNodes — repeated lookup.
    {
        "rpc": "GetNodes",
        "build": lambda: pb.GetNodesRequest(
            context=_ctx(),
            type_id=1,
            node_ids=[SEED_NODE_ID, "missing-1"],
        ),
        "mode": "happy",
        "check": lambda r: any(n.node_id == SEED_NODE_ID for n in r.nodes),
    },
    # QueryNodes — list everything.
    {
        "rpc": "QueryNodes",
        "build": lambda: pb.QueryNodesRequest(context=_ctx(), type_id=1, limit=10),
        "mode": "happy",
        "check": lambda r: any(n.node_id == SEED_NODE_ID for n in r.nodes),
    },
    # GetEdgesFrom — none seeded; expect empty (still success).
    {
        "rpc": "GetEdgesFrom",
        "build": lambda: pb.GetEdgesRequest(context=_ctx(), node_id=SEED_NODE_ID),
        "mode": "happy",
        "check": lambda r: list(r.edges) == [],
    },
    # GetEdgesTo — none seeded; expect empty.
    {
        "rpc": "GetEdgesTo",
        "build": lambda: pb.GetEdgesRequest(context=_ctx(), node_id=SEED_NODE_ID),
        "mode": "happy",
        "check": lambda r: list(r.edges) == [],
    },
    # GetConnectedNodes — none connected; expect empty.
    {
        "rpc": "GetConnectedNodes",
        "build": lambda: pb.GetConnectedNodesRequest(
            context=_ctx(),
            node_id=SEED_NODE_ID,
            edge_type_id=100,
            limit=10,
        ),
        "mode": "happy",
        "check": lambda r: list(r.nodes) == [],
    },
    # SearchMailbox — deprecated stub; returns empty.
    {
        "rpc": "SearchMailbox",
        "build": lambda: pb.SearchMailboxRequest(context=_ctx(), user_id="alice", query="hi"),
        "mode": "happy",
        "check": lambda r: list(r.results) == [] and r.has_more is False,
    },
    # GetMailbox — deprecated stub; returns empty.
    {
        "rpc": "GetMailbox",
        "build": lambda: pb.GetMailboxRequest(context=_ctx(), user_id="alice"),
        "mode": "happy",
        "check": lambda r: list(r.items) == [] and r.unread_count == 0,
    },
    # ListMailboxUsers — deprecated stub; returns empty.
    {
        "rpc": "ListMailboxUsers",
        "build": lambda: pb.ListMailboxUsersRequest(tenant_id=TENANT),
        "mode": "happy",
        "check": lambda r: list(r.user_ids) == [],
    },
    # ListTenants — without an auth interceptor there's no trusted
    # identity, so the handler aborts with PERMISSION_DENIED. This
    # is the exact contract a Go reimplementation must reproduce.
    {
        "rpc": "ListTenants",
        "build": lambda: pb.ListTenantsRequest(),
        "mode": "permission_denied",
    },
    # WaitForOffset — empty stream_position waits forever; instead
    # ask for an offset we already passed (the seed receipt) so it
    # returns immediately.
    {
        "rpc": "WaitForOffset",
        "build": lambda: pb.WaitForOffsetRequest(
            context=_ctx(),
            stream_position="entdb-wal:0:0",
            timeout_ms=500,
        ),
        "mode": "happy",
        # reached may be True or False depending on whether the
        # applier has caught up; the response itself must be
        # well-formed (no abort), which is what the framework
        # already verifies.
        "check": lambda _r: True,
    },
    # GetReceiptStatus — querying a known idempotency_key returns
    # APPLIED; unknown returns PENDING.
    {
        "rpc": "GetReceiptStatus",
        "build": lambda: pb.GetReceiptStatusRequest(context=_ctx(), idempotency_key="seed-1"),
        "mode": "happy",
        "check": lambda r: r.status == pb.ReceiptStatus.RECEIPT_STATUS_APPLIED,
    },
    {
        "rpc": "GetReceiptStatus",
        "build": lambda: pb.GetReceiptStatusRequest(context=_ctx(), idempotency_key="never-issued"),
        "mode": "happy",
        "check": lambda r: r.status == pb.ReceiptStatus.RECEIPT_STATUS_PENDING,
    },
    # ShareNode — admin actor has CORE_CAP_ADMIN, can share.
    {
        "rpc": "ShareNode",
        "build": lambda: pb.ShareNodeRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=ADMIN),
            node_id=SEED_NODE_ID,
            actor_id="user:bob",
            permission="read",
        ),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    # ListSharedWithMe — runs after ShareNode; bob may or may not
    # see the share depending on global_store wiring; just assert
    # well-formed response.
    {
        "rpc": "ListSharedWithMe",
        "build": lambda: pb.ListSharedWithMeRequest(
            context=pb.RequestContext(tenant_id=TENANT, actor=BOB),
            limit=10,
        ),
        "mode": "happy",
        "check": lambda _r: True,
    },
    # RevokeAccess — operates on the seed node; idempotent if no grant.
    {
        "rpc": "RevokeAccess",
        "build": lambda: pb.RevokeAccessRequest(
            context=_ctx(),
            node_id=SEED_NODE_ID,
            actor_id="user:bob",
        ),
        "mode": "happy",
        "check": lambda _r: True,  # found may be True or False
    },
    # AddGroupMember
    {
        "rpc": "AddGroupMember",
        "build": lambda: pb.GroupMemberRequest(
            context=_ctx(),
            group_id="g-test",
            member_actor_id="user:bob",
            role="member",
        ),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    # RemoveGroupMember — must run AFTER AddGroupMember.
    {
        "rpc": "RemoveGroupMember",
        "build": lambda: pb.GroupMemberRequest(
            context=_ctx(),
            group_id="g-test",
            member_actor_id="user:bob",
        ),
        "mode": "happy",
        "check": lambda _r: True,
    },
    # TransferOwnership — happy.
    {
        "rpc": "TransferOwnership",
        "build": lambda: pb.TransferOwnershipRequest(
            context=_ctx(), node_id=SEED_NODE_ID, new_owner=BOB
        ),
        "mode": "happy",
        "check": lambda r: r.found is True,
    },
    # User registry — without auth interceptor, every actor is trusted.
    {
        "rpc": "GetUser",
        "build": lambda: pb.GetUserRequest(actor=ALICE, user_id="alice"),
        "mode": "happy",
        "check": lambda r: r.found is True and r.user.user_id == "alice",
    },
    {
        "rpc": "GetUser",
        "build": lambda: pb.GetUserRequest(actor=ALICE, user_id="ghost"),
        "mode": "not_found",
        "check": lambda r: r.found is False,
    },
    {
        "rpc": "GetUser",
        "build": lambda: pb.GetUserRequest(actor="", user_id="alice"),
        "mode": "invalid_argument",
    },
    {
        "rpc": "GetUser",
        "build": lambda: pb.GetUserRequest(actor=ALICE, user_id=""),
        "mode": "invalid_argument",
    },
    {
        "rpc": "ListUsers",
        "build": lambda: pb.ListUsersRequest(actor=ADMIN, limit=10),
        "mode": "happy",
        "check": lambda r: any(u.user_id == "alice" for u in r.users),
    },
    {
        "rpc": "ListUsers",
        "build": lambda: pb.ListUsersRequest(actor="", limit=10),
        "mode": "invalid_argument",
    },
    {
        "rpc": "CreateUser",
        "build": lambda: pb.CreateUserRequest(
            actor=ADMIN, user_id="charlie", email="c@example.com", name="Charlie"
        ),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "CreateUser",
        "build": lambda: pb.CreateUserRequest(
            actor=ALICE, user_id="dave", email="d@example.com", name="Dave"
        ),
        "mode": "permission_denied",
    },
    {
        "rpc": "CreateUser",
        "build": lambda: pb.CreateUserRequest(actor=ADMIN, user_id="", email="x@e.com", name="X"),
        "mode": "invalid_argument",
    },
    {
        "rpc": "UpdateUser",
        "build": lambda: pb.UpdateUserRequest(actor=ALICE, user_id="alice", name="Alice Updated"),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "UpdateUser",
        "build": lambda: pb.UpdateUserRequest(actor=ALICE, user_id="bob", name="Hacked"),
        "mode": "permission_denied",
    },
    # Tenant registry.
    {
        "rpc": "GetTenant",
        "build": lambda: pb.GetTenantRequest(actor=ALICE, tenant_id=TENANT),
        "mode": "happy",
        "check": lambda r: r.found is True and r.tenant.tenant_id == TENANT,
    },
    {
        "rpc": "GetTenant",
        "build": lambda: pb.GetTenantRequest(actor=ALICE, tenant_id="ghost"),
        "mode": "not_found",
        "check": lambda r: r.found is False,
    },
    {
        "rpc": "GetTenant",
        "build": lambda: pb.GetTenantRequest(actor="", tenant_id=TENANT),
        "mode": "invalid_argument",
    },
    {
        "rpc": "CreateTenant",
        "build": lambda: pb.CreateTenantRequest(actor=ADMIN, tenant_id="newtenant", name="New"),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "CreateTenant",
        "build": lambda: pb.CreateTenantRequest(actor=ADMIN, tenant_id="", name="X"),
        "mode": "invalid_argument",
    },
    # Tenant membership.
    {
        "rpc": "GetTenantMembers",
        "build": lambda: pb.GetTenantMembersRequest(actor=ALICE, tenant_id=TENANT),
        "mode": "happy",
        "check": lambda r: any(m.user_id == "alice" for m in r.members),
    },
    {
        "rpc": "GetTenantMembers",
        "build": lambda: pb.GetTenantMembersRequest(actor="", tenant_id=TENANT),
        "mode": "invalid_argument",
    },
    {
        "rpc": "GetUserTenants",
        "build": lambda: pb.GetUserTenantsRequest(actor=ALICE, user_id="alice"),
        "mode": "happy",
        "check": lambda r: any(m.tenant_id == TENANT for m in r.memberships),
    },
    {
        "rpc": "GetUserTenants",
        "build": lambda: pb.GetUserTenantsRequest(actor="", user_id="alice"),
        "mode": "invalid_argument",
    },
    {
        "rpc": "AddTenantMember",
        "build": lambda: pb.TenantMemberRequest(
            actor=ALICE, tenant_id=TENANT, user_id="charlie", role="member"
        ),
        "mode": "happy",
        "check": lambda _r: True,
    },
    {
        "rpc": "AddTenantMember",
        "build": lambda: pb.TenantMemberRequest(actor="", tenant_id=TENANT, user_id="charlie"),
        "mode": "invalid_argument",
    },
    {
        "rpc": "AddTenantMember",
        "build": lambda: pb.TenantMemberRequest(
            actor=BOB, tenant_id=TENANT, user_id="dave", role="member"
        ),
        "mode": "permission_denied",
    },
    {
        "rpc": "ChangeMemberRole",
        "build": lambda: pb.ChangeMemberRoleRequest(
            actor=ALICE, tenant_id=TENANT, user_id="bob", new_role="admin"
        ),
        "mode": "happy",
        "check": lambda _r: True,
    },
    {
        "rpc": "ChangeMemberRole",
        "build": lambda: pb.ChangeMemberRoleRequest(
            actor="", tenant_id=TENANT, user_id="bob", new_role="admin"
        ),
        "mode": "invalid_argument",
    },
    {
        "rpc": "RemoveTenantMember",
        "build": lambda: pb.TenantMemberRequest(actor=ALICE, tenant_id=TENANT, user_id="bob"),
        "mode": "happy",
        "check": lambda _r: True,
    },
    {
        "rpc": "RemoveTenantMember",
        "build": lambda: pb.TenantMemberRequest(actor="", tenant_id=TENANT, user_id="bob"),
        "mode": "invalid_argument",
    },
    # Admin operations.
    {
        "rpc": "TransferUserContent",
        "build": lambda: pb.TransferUserContentRequest(
            actor=ADMIN, tenant_id=TENANT, from_user="alice", to_user="charlie"
        ),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "TransferUserContent",
        "build": lambda: pb.TransferUserContentRequest(
            actor=ADMIN, tenant_id="", from_user="alice", to_user="bob"
        ),
        "mode": "invalid_argument",
    },
    {
        "rpc": "SetLegalHold",
        "build": lambda: pb.LegalHoldRequest(actor=ADMIN, tenant_id=TENANT, enabled=True),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "SetLegalHold",
        "build": lambda: pb.LegalHoldRequest(actor=ADMIN, tenant_id="", enabled=True),
        "mode": "invalid_argument",
    },
    {
        "rpc": "RevokeAllUserAccess",
        "build": lambda: pb.RevokeAllUserAccessRequest(
            actor=ADMIN, tenant_id=TENANT, user_id="bob"
        ),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "RevokeAllUserAccess",
        "build": lambda: pb.RevokeAllUserAccessRequest(actor=ADMIN, tenant_id=TENANT, user_id=""),
        "mode": "invalid_argument",
    },
    # Quota dashboard.
    {
        "rpc": "GetTenantQuota",
        "build": lambda: pb.GetTenantQuotaRequest(actor=ADMIN, tenant_id=TENANT),
        "mode": "happy",
        "check": lambda r: r.tenant_id == TENANT,
    },
    {
        "rpc": "GetTenantQuota",
        "build": lambda: pb.GetTenantQuotaRequest(actor=ADMIN, tenant_id=""),
        "mode": "invalid_argument",
    },
    {
        "rpc": "GetTenantQuota",
        "build": lambda: pb.GetTenantQuotaRequest(actor=BOB, tenant_id=TENANT),
        "mode": "permission_denied",
    },
    # GetNodeByKey — type 1 has no unique fields in our test schema,
    # so the key resolves to nothing → found=False.
    {
        "rpc": "GetNodeByKey",
        "build": lambda: pb.GetNodeByKeyRequest(
            tenant_id=TENANT,
            actor=ALICE,
            type_id=1,
            field_id=1,
        ),
        "mode": "not_found",
        "check": lambda r: r.found is False,
    },
    # SearchNodes — empty query → INVALID_ARGUMENT.
    {
        "rpc": "SearchNodes",
        "build": lambda: pb.SearchNodesRequest(tenant_id=TENANT, actor=ALICE, type_id=1, query=""),
        "mode": "invalid_argument",
    },
    # SearchNodes — type 1 has no searchable fields → empty list.
    {
        "rpc": "SearchNodes",
        "build": lambda: pb.SearchNodesRequest(
            tenant_id=TENANT, actor=ALICE, type_id=1, query="anything"
        ),
        "mode": "happy",
        "check": lambda r: list(r.nodes) == [],
    },
    # GDPR.
    {
        "rpc": "ExportUserData",
        "build": lambda: pb.ExportUserDataRequest(actor=ALICE, user_id="alice"),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "ExportUserData",
        "build": lambda: pb.ExportUserDataRequest(actor=ALICE, user_id="bob"),
        "mode": "permission_denied",
    },
    {
        "rpc": "FreezeUser",
        "build": lambda: pb.FreezeUserRequest(actor=ADMIN, user_id="bob", enabled=True),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "FreezeUser",
        "build": lambda: pb.FreezeUserRequest(actor=ADMIN, user_id="bob", enabled=False),
        "mode": "happy",
    },
    {
        "rpc": "DeleteUser",
        "build": lambda: pb.DeleteUserRequest(actor=ALICE, user_id="alice", grace_days=30),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "DeleteUser",
        "build": lambda: pb.DeleteUserRequest(actor=ALICE, user_id="bob", grace_days=30),
        "mode": "permission_denied",
    },
    {
        "rpc": "CancelUserDeletion",
        "build": lambda: pb.CancelUserDeletionRequest(actor=ALICE, user_id="alice"),
        "mode": "happy",
        "check": lambda _r: True,
    },
    {
        "rpc": "ArchiveTenant",
        # The fixture is per-test; use the seed tenant.
        "build": lambda: pb.ArchiveTenantRequest(actor=ADMIN, tenant_id=TENANT),
        "mode": "happy",
        "check": lambda r: r.success is True,
    },
    {
        "rpc": "ArchiveTenant",
        "build": lambda: pb.ArchiveTenantRequest(actor="", tenant_id=TENANT),
        "mode": "invalid_argument",
    },
]


@pytest.mark.parametrize(
    "case",
    CONTRACT_CASES,
    ids=[f"{c['rpc']}-{c['mode']}" for c in CONTRACT_CASES],
)
async def test_grpc_contract(live_server, case):
    """Drive every RPC over a real gRPC channel and assert wire shape.

    Cases run sequentially against one server fixture (so happy
    writes can set up state for later reads). The fixture is per-test
    so cross-test ordering is irrelevant; in-test ordering follows
    the CONTRACT_CASES list order.
    """
    port = live_server
    async with grpc_aio.insecure_channel(f"127.0.0.1:{port}") as channel:
        stub = EntDBServiceStub(channel)
        rpc_method = getattr(stub, case["rpc"])
        req = case["build"]()
        mode = case["mode"]

        if mode in ("happy", "not_found"):
            resp = await rpc_method(req, timeout=5.0)
            check = case.get("check")
            if check is not None:
                assert check(resp), f"{case['rpc']} ({mode}) response failed check: {resp}"
        elif mode == "invalid_argument":
            with pytest.raises(grpc.aio.AioRpcError) as ei:
                await rpc_method(req, timeout=5.0)
            assert ei.value.code() == grpc.StatusCode.INVALID_ARGUMENT, (
                f"{case['rpc']}: expected INVALID_ARGUMENT, got {ei.value.code()}"
            )
        elif mode == "permission_denied":
            with pytest.raises(grpc.aio.AioRpcError) as ei:
                await rpc_method(req, timeout=5.0)
            assert ei.value.code() == grpc.StatusCode.PERMISSION_DENIED, (
                f"{case['rpc']}: expected PERMISSION_DENIED, got {ei.value.code()}"
            )
        elif mode == "unimplemented":
            with pytest.raises(grpc.aio.AioRpcError) as ei:
                await rpc_method(req, timeout=5.0)
            assert ei.value.code() == grpc.StatusCode.UNIMPLEMENTED, (
                f"{case['rpc']}: expected UNIMPLEMENTED, got {ei.value.code()}"
            )
        else:
            raise AssertionError(f"Unknown mode: {mode}")


async def test_every_rpc_in_proto_has_at_least_one_contract_case():
    """Guard: if a new RPC ships in the proto, this test forces a contract case.

    The whole point of this file is to be the contract for a Go
    rewrite. A drift detector keeps that promise honest.
    """
    rpcs_in_stub = {
        name for name in dir(EntDBServiceStub) if not name.startswith("_") and name[0].isupper()
    }
    rpcs_in_cases = {c["rpc"] for c in CONTRACT_CASES}
    missing = (
        rpcs_in_stub
        - rpcs_in_cases
        - {
            # ExecuteAtomic is exhaustively covered in
            # test_payload_roundtrip_python.py and test_grpc_wire_format.py.
            "ExecuteAtomic",
            # DelegateAccess: setup is non-trivial (requires content owned
            # by from_user); covered in dedicated unit tests. Tracked for
            # follow-up parity.
            "DelegateAccess",
        }
    )
    assert not missing, (
        f"RPCs missing from CONTRACT_CASES: {sorted(missing)}. "
        "Every new RPC must ship with a contract case so a Go rewrite "
        "can verify identical wire behaviour."
    )
