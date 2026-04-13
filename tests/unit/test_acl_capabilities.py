"""Integration tests for typed ACL grants persisted in ``node_access``.

These exercise the end-to-end path: share a node with typed caps,
read the grant back via ``get_acl_grants``, and run it through
``CapabilityRegistry.check_grant``. Also covers the legacy
``permission`` → ``core_caps_json`` migration and cross-tenant
``tenant:<id>`` grantees.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.auth.capability_registry import (
    CapabilityRegistry,
    CoreCapability,
)

# ── Helpers ─────────────────────────────────────────────────────────


@pytest.fixture
def store(tmp_path: Path) -> CanonicalStore:
    s = CanonicalStore(data_dir=str(tmp_path / "tenants"))
    return s


@pytest.fixture
def tenant_id() -> str:
    return "tenant_acme"


@pytest.fixture
async def initialized_store(store: CanonicalStore, tenant_id: str) -> CanonicalStore:
    await store.initialize_tenant(tenant_id)

    # Create a dummy node so foreign-key / visibility checks line up.
    async def _create() -> str:
        return (
            await store.create_node(
                tenant_id=tenant_id,
                type_id=101,
                payload={"title": "hello"},
                owner_actor="user:owner",
                node_id="n-1",
            )
        ).node_id

    await _create()
    return store


# ── share_node + get_acl_grants typed fields ────────────────────────


async def test_share_node_persists_typed_core_caps(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:alice",
        permission="",
        granted_by="user:owner",
        type_id=101,
        core_caps=[int(CoreCapability.READ), int(CoreCapability.EDIT)],
        ext_cap_ids=[],
    )
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:alice")
    assert len(grants) == 1
    g = grants[0]
    assert g["type_id"] == 101
    assert sorted(g["core_cap_ids"]) == [
        int(CoreCapability.READ),
        int(CoreCapability.EDIT),
    ]
    assert g["ext_cap_ids"] == []


async def test_share_node_persists_typed_ext_caps(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:alice",
        permission="",
        granted_by="user:owner",
        type_id=101,
        core_caps=[],
        ext_cap_ids=[1, 2],
    )
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:alice")
    assert sorted(grants[0]["ext_cap_ids"]) == [1, 2]


async def test_legacy_permission_string_derives_core_caps(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:bob",
        permission="write",
        granted_by="user:owner",
    )
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:bob")
    assert grants, "Bob should have a grant row"
    g = grants[0]
    # "write" → READ + COMMENT + EDIT.
    assert int(CoreCapability.READ) in g["core_cap_ids"]
    assert int(CoreCapability.COMMENT) in g["core_cap_ids"]
    assert int(CoreCapability.EDIT) in g["core_cap_ids"]


async def test_migrate_permissions_to_capabilities_backfills_rows(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    # Simulate an old row with only the string permission: go
    # straight through the sync path and wipe out the core_caps_json
    # that ``share_node`` would otherwise have populated.
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:old",
        permission="admin",
        granted_by="user:owner",
    )

    def _reset(store: CanonicalStore) -> None:
        with store._get_connection(tenant_id) as conn:
            conn.execute(
                "UPDATE node_access SET core_caps_json = '[]' WHERE actor_id = ?",
                ("user:old",),
            )
            conn.commit()

    _reset(initialized_store)

    # Migration should back-fill the row.
    migrated = await initialized_store.migrate_permissions_to_capabilities(tenant_id)
    assert migrated >= 1
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:old")
    assert int(CoreCapability.ADMIN) in grants[0]["core_cap_ids"]


async def test_expired_grant_is_filtered(initialized_store: CanonicalStore, tenant_id: str) -> None:
    # expires_at in the past.
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:ephemeral",
        permission="read",
        granted_by="user:owner",
        expires_at=1,
    )
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:ephemeral")
    assert grants == []


async def test_revoke_removes_grant(initialized_store: CanonicalStore, tenant_id: str) -> None:
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:carol",
        permission="read",
        granted_by="user:owner",
    )
    assert await initialized_store.get_acl_grants(tenant_id, "n-1", "user:carol")
    await initialized_store.revoke_access(tenant_id, "n-1", "user:carol")
    assert await initialized_store.get_acl_grants(tenant_id, "n-1", "user:carol") == []


async def test_group_expansion_returns_group_grants(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    # Grant to a group; Alice should match via group membership.
    await initialized_store.add_group_member(tenant_id, "group:engineering", "user:alice", "member")
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="group:engineering",
        permission="write",
        granted_by="user:owner",
        type_id=101,
        actor_type="group",
    )
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:alice")
    assert grants, "Alice should inherit the group grant"
    g = grants[0]
    assert g["grantee"] == "group:engineering"
    assert int(CoreCapability.EDIT) in g["core_cap_ids"]


class _FakeActor:
    def __init__(self, user_id: str, tenant_id: str) -> None:
        self.user_id = user_id
        self.tenant_id = tenant_id


async def test_cross_tenant_tenant_principal_matches_actor_tenant(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    # Grant to tenant:globex — only actors from tenant globex can
    # resolve this grant.
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="tenant:globex",
        permission="read",
        granted_by="user:owner",
        type_id=101,
        actor_type="tenant",
    )
    # Actor from globex sees the grant.
    grants_globex = await initialized_store.get_acl_grants(
        tenant_id, "n-1", _FakeActor("dave", "globex")
    )
    assert any(g["grantee"] == "tenant:globex" for g in grants_globex)

    # Actor from a different tenant does NOT see it.
    grants_other = await initialized_store.get_acl_grants(
        tenant_id, "n-1", _FakeActor("dave", "initech")
    )
    assert not any(g["grantee"] == "tenant:globex" for g in grants_other)


async def test_check_grant_integration_read_satisfied(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:eve",
        permission="read",
        granted_by="user:owner",
        type_id=101,
    )
    reg = CapabilityRegistry()
    reg.register_type(type_id=101)
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:eve")
    assert grants
    required_core, required_ext = reg.required_for_op(101, "GetNode")
    satisfied = any(
        reg.check_grant(
            g["core_cap_ids"],
            g["ext_cap_ids"],
            required_core,
            required_ext,
            g["type_id"] or 101,
        )
        for g in grants
    )
    assert satisfied


async def test_check_grant_integration_edit_denied_for_read_only_grant(
    initialized_store: CanonicalStore, tenant_id: str
) -> None:
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:eve",
        permission="read",
        granted_by="user:owner",
        type_id=101,
    )
    reg = CapabilityRegistry()
    reg.register_type(type_id=101)
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:eve")
    required_core, _ = reg.required_for_op(101, "UpdateNode")
    satisfied = any(
        reg.check_grant(g["core_cap_ids"], g["ext_cap_ids"], required_core, None, 101)
        for g in grants
    )
    assert not satisfied


async def test_deny_grant_recognised(initialized_store: CanonicalStore, tenant_id: str) -> None:
    await initialized_store.share_node(
        tenant_id=tenant_id,
        node_id="n-1",
        actor_id="user:frank",
        permission="deny",
        granted_by="user:owner",
    )
    grants = await initialized_store.get_acl_grants(tenant_id, "n-1", "user:frank")
    # Deny rows carry empty core_cap_ids.
    assert grants
    assert grants[0]["permission"] == "deny"
    assert grants[0]["core_cap_ids"] == []
