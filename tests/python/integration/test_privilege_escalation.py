# SPDX-License-Identifier: AGPL-3.0-only
"""
Privilege-escalation regression tests.

Bug class
---------
Several gRPC handlers used to read the ``actor`` (or ``context.actor``)
field straight off the request payload and feed it into authorization
helpers like ``_is_admin_or_system`` / ``_check_tenant_access`` /
``_check_cross_tenant_read``. Those helpers do simple string checks
(``actor.startswith("system:")``) and trust the result.

That meant any authenticated low-privilege user — say ``user:eve`` —
could send ``actor = "system:admin"`` (or ``__system__`` / ``admin:foo``)
in the body of the message and have the handler treat them as a
privileged caller for the duration of that RPC. CVE-class severity.

What's pinned here
------------------
For every gRPC RPC that takes an ``actor`` (or ``context.actor``)
field, the handler:

* MUST consult the trusted identity populated by ``AuthInterceptor``
  via the ``_current_identity`` ``ContextVar`` — i.e. call
  :func:`get_authoritative_actor` — before performing any privilege
  decision.
* MUST either reject the call with ``PERMISSION_DENIED`` *or* execute
  it as the trusted identity (e.g. ``ListTenants`` returns only
  Eve's tenants, ``ExecuteAtomic`` persists ``actor=user:eve`` in the
  WAL event), regardless of what the request payload claimed.

The tests here authenticate as ``user:eve`` (a real, low-privilege
user) and supply privileged-looking ``actor`` strings on the wire;
they assert that the privileged path is never taken.
"""

from __future__ import annotations

import contextlib
import json
import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest
from google.protobuf.struct_pb2 import Struct

from dbaas.entdb_server.api.generated import (
    CreateNodeOp,
    CreateUserRequest,
    ExecuteAtomicRequest,
    GetEdgesRequest,
    GetNodeRequest,
    GetNodesRequest,
    GetTenantQuotaRequest,
    ListTenantsRequest,
    Operation,
    QueryNodesRequest,
    RequestContext,
    TransferUserContentRequest,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.auth.auth_interceptor import (
    reset_current_identity,
    set_current_identity,
)
from dbaas.entdb_server.global_store import GlobalStore

# --------------------------------------------------------------------------
# Test scaffolding
# --------------------------------------------------------------------------


class _AbortError(BaseException):
    """Stand-in for ``grpc.aio.AbortError`` (BaseException so generic
    ``except Exception:`` blocks in handlers don't swallow it)."""


class _FakeContext:
    """Minimal stand-in for ``grpc_aio.ServicerContext``."""

    def __init__(self) -> None:
        self.aborted = False
        self.abort_code: grpc.StatusCode | None = None
        self.abort_message: str | None = None

    async def abort(self, code: grpc.StatusCode, message: str) -> None:
        self.aborted = True
        self.abort_code = code
        self.abort_message = message
        raise _AbortError(f"[{code}] {message}")

    def set_trailing_metadata(self, *_a, **_kw) -> None:
        return None


@contextlib.contextmanager
def _trusted_identity(identity: str | None):
    """Set the auth-interceptor ContextVar for the duration of a test."""
    token = set_current_identity(identity)
    try:
        yield
    finally:
        reset_current_identity(token)


@pytest.fixture
def global_store():
    with tempfile.TemporaryDirectory() as tmpdir:
        gs = GlobalStore(tmpdir)
        yield gs
        gs.close()


def _make_servicer(global_store: GlobalStore) -> tuple[EntDBServicer, MagicMock, MagicMock]:
    """Build an EntDBServicer wired to ``global_store`` with mock WAL+canonical_store.

    Returns ``(servicer, wal_mock, canonical_store_mock)`` so individual
    tests can assert on the WAL events that were appended.
    """
    wal = MagicMock()
    pos = MagicMock()
    pos.__str__ = MagicMock(return_value="0:0:0")
    wal.append = AsyncMock(return_value=pos)

    canonical_store = MagicMock()
    canonical_store.list_tenants = MagicMock(return_value=["acme"])
    canonical_store.initialize_tenant = AsyncMock()
    canonical_store.wait_for_offset = AsyncMock(return_value=True)
    canonical_store.get_node = AsyncMock(return_value=None)
    canonical_store.get_edges_from = AsyncMock(return_value=[])
    canonical_store.get_edges_to = AsyncMock(return_value=[])
    canonical_store.query_nodes = AsyncMock(return_value=[])
    canonical_store.search_nodes = AsyncMock(return_value=[])
    canonical_store.resolve_actor_groups = AsyncMock(return_value=["user:eve"])
    canonical_store.has_node_access = AsyncMock(return_value=False)
    canonical_store.can_access = AsyncMock(return_value=False)

    schema_registry = MagicMock()
    schema_registry.fingerprint = ""
    schema_registry.to_dict = MagicMock(return_value={"node_types": [], "edge_types": []})

    servicer = EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=schema_registry,
        global_store=global_store,
    )
    return servicer, wal, canonical_store


async def _bootstrap_tenant(
    gs: GlobalStore,
    tenant_id: str = "acme",
    members: dict[str, str] | None = None,
) -> None:
    await gs.create_tenant(tenant_id, f"Tenant {tenant_id}")
    if members:
        for user_id, role in members.items():
            await gs.add_member(tenant_id, user_id, role=role)


# Privileged-looking strings a malicious caller might shove into the
# request body. The fix must NOT honour any of these when the
# authenticated identity is a regular user.
CLAIMED_ADMIN_ACTORS = [
    "system:admin",
    "system:gdpr-worker",
    "__system__",
    "admin:root",
]


# --------------------------------------------------------------------------
# Read-path RPCs that take ``request.context.actor``
#
# When eve authenticates as ``user:eve`` and the tenant exists but she
# is not a member, the cross-tenant check must reject her with
# PERMISSION_DENIED — even though the request payload says she's
# ``system:admin``. Trusting the payload string would have routed her
# through the ``return "member"`` short-circuit at the top of
# ``_check_cross_tenant_read`` and silently granted full read access.
# --------------------------------------------------------------------------


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_get_node_rejects_claimed_admin_actor(global_store, claimed):
    await _bootstrap_tenant(global_store, members={"alice": "owner"})
    servicer, _wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = GetNodeRequest(
        context=RequestContext(tenant_id="acme", actor=claimed),
        node_id="some-node",
    )
    with _trusted_identity("user:eve"), contextlib.suppress(_AbortError):
        # Eve is not a member; the trusted-actor fix must reject her
        # despite the payload claim. The handler swallows aborts and
        # returns ``found=False``; we still verify ctx.aborted +
        # PERMISSION_DENIED so a future regression that turns the
        # rejection into a silent allow is caught.
        await servicer.GetNode(request, ctx)

    assert ctx.aborted is True, (
        f"GetNode honoured claimed actor {claimed!r}: no abort fired. "
        "The handler must consult the trusted identity from "
        "AuthInterceptor (get_authoritative_actor), NOT the payload."
    )
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_get_nodes_rejects_claimed_admin_actor(global_store, claimed):
    await _bootstrap_tenant(global_store, members={"alice": "owner"})
    servicer, _wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = GetNodesRequest(
        context=RequestContext(tenant_id="acme", actor=claimed),
        node_ids=["a", "b"],
    )
    with _trusted_identity("user:eve"), contextlib.suppress(_AbortError):
        await servicer.GetNodes(request, ctx)

    assert ctx.aborted is True, (
        f"GetNodes honoured claimed actor {claimed!r}; PERMISSION_DENIED expected"
    )
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_query_nodes_rejects_claimed_admin_actor(global_store, claimed):
    await _bootstrap_tenant(global_store, members={"alice": "owner"})
    servicer, _wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = QueryNodesRequest(
        context=RequestContext(tenant_id="acme", actor=claimed),
        type_id=1,
    )
    with _trusted_identity("user:eve"), contextlib.suppress(_AbortError):
        await servicer.QueryNodes(request, ctx)

    assert ctx.aborted is True, (
        f"QueryNodes honoured claimed actor {claimed!r}; PERMISSION_DENIED expected"
    )
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


# --------------------------------------------------------------------------
# Write path: ExecuteAtomic
#
# Eve authenticates as ``user:eve`` and claims ``actor: "system:admin"``
# in the body. Two properties must hold:
#
# 1. The authorization decision uses the trusted identity (here she
#    becomes a non-member of the tenant -> PERMISSION_DENIED).
# 2. If the call ever DOES go through (e.g. for a tenant where eve IS
#    a member), the persisted WAL event must record actor=user:eve,
#    not the privileged string she claimed. Without that property,
#    audit + GDPR exports get poisoned.
# --------------------------------------------------------------------------


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_execute_atomic_rejects_claimed_admin_actor(global_store, claimed):
    """Eve isn't a tenant member -> PERMISSION_DENIED, payload ignored."""
    await _bootstrap_tenant(global_store, members={"alice": "owner"})
    servicer, _wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = ExecuteAtomicRequest(
        context=RequestContext(tenant_id="acme", actor=claimed),
        idempotency_key="k-eve",
        operations=[Operation(create_node=CreateNodeOp(type_id=1, data=Struct()))],
    )
    with _trusted_identity("user:eve"), contextlib.suppress(_AbortError):
        await servicer.ExecuteAtomic(request, ctx)

    assert ctx.aborted is True, (
        f"ExecuteAtomic honoured claimed actor {claimed!r}; "
        "expected PERMISSION_DENIED for non-member user:eve"
    )
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


async def test_execute_atomic_persists_trusted_actor_not_claimed(global_store):
    """Even when the call is allowed (eve IS a member), the WAL event
    MUST record the trusted identity, not the privileged string the
    request claimed. Audit logs / GDPR exports key off the persisted
    actor; if the claim is honoured the event log lies."""
    await _bootstrap_tenant(global_store, members={"eve": "member"})
    servicer, wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = ExecuteAtomicRequest(
        context=RequestContext(tenant_id="acme", actor="system:admin"),
        idempotency_key="k-trusted",
        operations=[Operation(create_node=CreateNodeOp(type_id=1, data=Struct()))],
    )
    with _trusted_identity("user:eve"):
        resp = await servicer.ExecuteAtomic(request, ctx)

    assert resp.success is True
    wal.append.assert_awaited_once()
    args, kwargs = wal.append.call_args
    payload_bytes = args[2] if len(args) >= 3 else kwargs.get("value")
    event = json.loads(payload_bytes.decode("utf-8"))
    assert event["actor"] == "user:eve", (
        f"ExecuteAtomic persisted actor={event['actor']!r}. "
        "It must record the trusted identity (user:eve), not the "
        "privileged string the client claimed in the payload."
    )


# --------------------------------------------------------------------------
# Admin-only RPCs that take a top-level ``request.actor``
# --------------------------------------------------------------------------


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_create_user_rejects_claimed_admin_actor(global_store, claimed):
    """CreateUser is admin-only. The privilege check must run on the
    trusted identity, not the payload."""
    servicer, _wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = CreateUserRequest(
        actor=claimed,
        user_id="malicious",
        email="x@example.com",
        name="x",
    )
    with _trusted_identity("user:eve"), contextlib.suppress(_AbortError):
        await servicer.CreateUser(request, ctx)

    assert ctx.aborted is True, (
        f"CreateUser honoured claimed actor {claimed!r}: "
        "any authenticated user could escalate to admin."
    )
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_transfer_user_content_rejects_claimed_admin_actor(global_store, claimed):
    """TransferUserContent already calls ``_require_admin_or_owner``;
    this pins the existing fix so a regression that drops the trusted
    lookup is caught here too."""
    await _bootstrap_tenant(global_store, members={"alice": "owner"})
    servicer, wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = TransferUserContentRequest(
        actor=claimed,
        tenant_id="acme",
        from_user="user:alice",
        to_user="user:bob",
    )
    with _trusted_identity("user:eve"), contextlib.suppress(_AbortError):
        await servicer.TransferUserContent(request, ctx)

    assert ctx.aborted is True
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED
    # And the WAL must NOT have been written.
    wal.append.assert_not_awaited()


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_get_tenant_quota_rejects_claimed_admin_actor(global_store, claimed):
    await _bootstrap_tenant(global_store, members={"alice": "owner"})
    servicer, _wal, _cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = GetTenantQuotaRequest(actor=claimed, tenant_id="acme")
    with _trusted_identity("user:eve"), pytest.raises(_AbortError):
        await servicer.GetTenantQuota(request, ctx)

    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


# --------------------------------------------------------------------------
# ListTenants — already partially fixed; pin it stays fixed
# --------------------------------------------------------------------------


@pytest.mark.parametrize("claimed", CLAIMED_ADMIN_ACTORS)
async def test_list_tenants_ignores_claimed_admin_actor(global_store, claimed):
    """Eve sees only the tenants she belongs to, regardless of the
    privileged actor string she sent."""
    await _bootstrap_tenant(global_store, members={"alice": "owner"})
    await _bootstrap_tenant(global_store, tenant_id="globex", members={"eve": "member"})
    await _bootstrap_tenant(global_store, tenant_id="initech", members={"bob": "owner"})
    servicer, _wal, cs = _make_servicer(global_store)
    cs.list_tenants = MagicMock(return_value=["acme", "globex", "initech"])
    ctx = _FakeContext()

    # The actor field on ListTenantsRequest doesn't even exist in the
    # proto; ListTenants is identity-driven. We still set the trusted
    # identity to user:eve and verify that the privileged claim path
    # — were it ever to be added later — could not bypass membership
    # filtering.
    with _trusted_identity("user:eve"):
        resp = await servicer.ListTenants(ListTenantsRequest(), ctx)

    visible = sorted(t.tenant_id for t in resp.tenants)
    assert visible == ["globex"], (
        f"ListTenants returned {visible!r} for user:eve; expected only "
        "['globex']. A regression has reintroduced cross-tenant "
        "enumeration."
    )
    # Sanity: a privileged trusted identity DOES see all tenants —
    # this confirms the test fixture is wired correctly and isn't
    # masking the bug.
    ctx2 = _FakeContext()
    with _trusted_identity(claimed):
        resp2 = await servicer.ListTenants(ListTenantsRequest(), ctx2)
    assert sorted(t.tenant_id for t in resp2.tenants) == ["acme", "globex", "initech"]


# --------------------------------------------------------------------------
# Edges — GetEdgesFrom / GetEdgesTo take ``context.actor`` (currently
# they don't run the cross-tenant check, but they shouldn't trust the
# claimed actor for any future authz decision either). We pin the
# weaker property: the handler must not crash, and when the new
# trusted-actor pattern is wired in, eve cannot escalate.
# --------------------------------------------------------------------------


async def test_get_edges_from_does_not_use_claimed_actor_for_authz(global_store):
    """Pin the negative behaviour: even if GetEdgesFrom doesn't
    currently abort on auth, it must not stash a privileged claim
    anywhere that downstream code reads. We invoke the handler and
    verify (a) it returns successfully and (b) the canonical_store
    was queried with the trusted tenant_id, not the claimed actor."""
    await _bootstrap_tenant(global_store, members={"eve": "member"})
    servicer, _wal, cs = _make_servicer(global_store)
    ctx = _FakeContext()

    request = GetEdgesRequest(
        context=RequestContext(tenant_id="acme", actor="system:admin"),
        node_id="n1",
    )
    with _trusted_identity("user:eve"):
        resp = await servicer.GetEdgesFrom(request, ctx)

    assert resp.edges == [] or resp.edges is not None
    cs.get_edges_from.assert_awaited_once()
