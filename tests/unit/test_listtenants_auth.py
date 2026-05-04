"""
Regression tests for [SEC-3] — ListTenants must not enumerate every
tenant on the server to any authenticated caller.

Visibility contract pinned by these tests:

- ``system:*`` / ``__system__`` / ``admin:*`` callers see every tenant
  on the node (sharding still applies).
- A regular ``user:<id>`` caller sees only tenants they are a member
  of (via ``global_store.get_user_tenants``).
- A request with no trusted identity is rejected with
  ``PERMISSION_DENIED`` rather than falling open.

These properties are the visible-surface fix for the cross-tenant
enumeration vector reported in #134. If a future refactor moves the
identity check or removes the membership filter, these tests should
fail loudly.
"""

from __future__ import annotations

import contextlib
import tempfile
from unittest.mock import MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.generated import ListTenantsRequest
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.auth.auth_interceptor import (
    reset_current_identity,
    set_current_identity,
)
from dbaas.entdb_server.global_store import GlobalStore


class _AbortError(BaseException):
    """Mimics ``grpc.aio.AbortError`` so it propagates through ``except Exception``."""


class _FakeContext:
    def __init__(self) -> None:
        self.aborted = False
        self.abort_code: grpc.StatusCode | None = None
        self.abort_message: str | None = None

    async def abort(self, code: grpc.StatusCode, message: str) -> None:
        self.aborted = True
        self.abort_code = code
        self.abort_message = message
        raise _AbortError(f"[{code}] {message}")


@contextlib.contextmanager
def _trusted_identity(identity: str | None):
    """Set the auth-interceptor ContextVar for the duration of a test."""
    token = set_current_identity(identity)
    try:
        yield
    finally:
        reset_current_identity(token)


def _make_servicer(global_store: GlobalStore, present_tenants: list[str]) -> EntDBServicer:
    """Build a Servicer whose canonical_store reports the given tenants."""
    wal = MagicMock()
    canonical_store = MagicMock()
    canonical_store.list_tenants = MagicMock(return_value=list(present_tenants))
    schema_registry = MagicMock()
    schema_registry.fingerprint = ""

    return EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=schema_registry,
        global_store=global_store,
    )


@pytest.fixture
def global_store():
    with tempfile.TemporaryDirectory() as tmpdir:
        gs = GlobalStore(tmpdir)
        yield gs
        gs.close()


# ══════════════════════════════════════════════════════════════════════
# Admin / system identities see every tenant
# ══════════════════════════════════════════════════════════════════════


@pytest.mark.parametrize(
    "trusted",
    ["__system__", "system:admin", "system:gdpr-worker", "admin:root"],
)
async def test_admin_identities_see_all_tenants(global_store, trusted):
    await global_store.create_tenant("acme", "Acme")
    await global_store.create_tenant("globex", "Globex")
    await global_store.create_tenant("initech", "Initech")

    servicer = _make_servicer(global_store, ["acme", "globex", "initech"])
    ctx = _FakeContext()

    with _trusted_identity(trusted):
        resp = await servicer.ListTenants(ListTenantsRequest(), ctx)

    assert sorted(t.tenant_id for t in resp.tenants) == ["acme", "globex", "initech"]
    assert ctx.aborted is False


# ══════════════════════════════════════════════════════════════════════
# Regular users see only tenants they are members of
# ══════════════════════════════════════════════════════════════════════


async def test_regular_user_sees_only_membership_tenants(global_store):
    # Server hosts three tenants; alice is a member of two.
    await global_store.create_tenant("acme", "Acme")
    await global_store.create_tenant("globex", "Globex")
    await global_store.create_tenant("initech", "Initech")
    await global_store.add_member("acme", "alice", role="member")
    await global_store.add_member("globex", "alice", role="admin")
    # alice is NOT in initech — must not appear in her result.

    servicer = _make_servicer(global_store, ["acme", "globex", "initech"])
    ctx = _FakeContext()

    with _trusted_identity("user:alice"):
        resp = await servicer.ListTenants(ListTenantsRequest(), ctx)

    visible = sorted(t.tenant_id for t in resp.tenants)
    assert visible == ["acme", "globex"]
    assert "initech" not in visible
    assert ctx.aborted is False


async def test_regular_user_with_zero_memberships_sees_empty_list(global_store):
    """Even a perfectly authenticated user with no memberships
    sees an empty list — they have no business knowing other
    tenants exist."""
    await global_store.create_tenant("acme", "Acme")
    await global_store.create_tenant("globex", "Globex")

    servicer = _make_servicer(global_store, ["acme", "globex"])
    ctx = _FakeContext()

    with _trusted_identity("user:nobody"):
        resp = await servicer.ListTenants(ListTenantsRequest(), ctx)

    assert resp.tenants == []
    assert ctx.aborted is False


async def test_user_id_is_stripped_of_actor_prefix_for_membership_lookup(global_store):
    """The membership lookup expects bare ``user_id`` ('alice'), not
    a prefixed actor ('user:alice'). The handler must strip the
    prefix before querying ``global_store.get_user_tenants``.
    """
    await global_store.create_tenant("acme", "Acme")
    await global_store.add_member("acme", "alice", role="member")

    servicer = _make_servicer(global_store, ["acme"])
    ctx = _FakeContext()

    with _trusted_identity("user:alice"):
        resp = await servicer.ListTenants(ListTenantsRequest(), ctx)

    assert [t.tenant_id for t in resp.tenants] == ["acme"]


# ══════════════════════════════════════════════════════════════════════
# Anonymous (no trusted identity) is rejected
# ══════════════════════════════════════════════════════════════════════


async def test_no_trusted_identity_rejected_with_permission_denied(global_store):
    """The pre-fix bug: anyone past the AuthInterceptor — including
    deployments where the interceptor is misconfigured — could
    enumerate every tenant. The handler must NOT fall open."""
    await global_store.create_tenant("acme", "Acme")

    servicer = _make_servicer(global_store, ["acme"])
    ctx = _FakeContext()

    with _trusted_identity(None), pytest.raises(_AbortError):
        await servicer.ListTenants(ListTenantsRequest(), ctx)

    assert ctx.aborted is True
    assert ctx.abort_code == grpc.StatusCode.PERMISSION_DENIED


# ══════════════════════════════════════════════════════════════════════
# Sharding still applies to admin callers
# ══════════════════════════════════════════════════════════════════════


async def test_admin_only_sees_tenants_owned_by_this_node(global_store):
    """The pre-fix sharding filter is preserved — admin sees every
    tenant ON THIS NODE, not every tenant in the cluster."""
    await global_store.create_tenant("acme", "Acme")
    await global_store.create_tenant("globex", "Globex")

    sharding = MagicMock()
    sharding.is_multi_node = True
    # acme owned by us, globex owned by another node.
    sharding.is_mine = MagicMock(side_effect=lambda tid: tid == "acme")

    wal = MagicMock()
    canonical_store = MagicMock()
    canonical_store.list_tenants = MagicMock(return_value=["acme", "globex"])
    schema_registry = MagicMock()
    schema_registry.fingerprint = ""
    servicer = EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=schema_registry,
        global_store=global_store,
        sharding=sharding,
    )
    ctx = _FakeContext()

    with _trusted_identity("__system__"):
        resp = await servicer.ListTenants(ListTenantsRequest(), ctx)

    visible = [t.tenant_id for t in resp.tenants]
    assert visible == ["acme"]
    assert "globex" not in visible


# ══════════════════════════════════════════════════════════════════════
# Defensive: no global_store attached → empty (don't fall open)
# ══════════════════════════════════════════════════════════════════════


async def test_no_global_store_returns_empty_for_regular_user():
    """In test/embedded harnesses that don't wire a GlobalStore,
    a regular user gets an empty list rather than the unfiltered
    raw tenant list."""
    wal = MagicMock()
    canonical_store = MagicMock()
    canonical_store.list_tenants = MagicMock(return_value=["acme", "globex"])
    schema_registry = MagicMock()
    schema_registry.fingerprint = ""
    servicer = EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=schema_registry,
        global_store=None,
    )
    ctx = _FakeContext()

    with _trusted_identity("user:alice"):
        resp = await servicer.ListTenants(ListTenantsRequest(), ctx)

    assert resp.tenants == []
