"""Tests for cross-tenant read path (Issue #63).

Covers:
- Bob reads node from Alice's tenant (has node_access) -> success
- Bob reads node without access -> PERMISSION_DENIED
- Bob queries Alice's tenant (has some access) -> returns only accessible nodes
- shared_with_me returns cross-tenant entries
- Cross-tenant with group -> group expanded, individual entries work
- GetNodes cross-tenant filtering
- System actor bypasses cross-tenant checks
- Expired node_access is rejected
- DENY overrides cross-tenant allow
- ListSharedWithMe aggregates per-tenant + cross-tenant results
"""

import asyncio
import time
import uuid
from unittest.mock import AsyncMock, MagicMock

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore

TENANT_ALICE = "tenant_alice"
TENANT_BOB = "tenant_bob"
ALICE = "user:alice"
BOB = "user:bob"
CAROL = "user:carol"
SYSTEM = "__system__"
GROUP_TEAM = "group:team"


# ── Fixtures ─────────────────────────────────────────────────────────


@pytest.fixture
def global_store(tmp_path):
    gs = GlobalStore(data_dir=tmp_path / "global")
    yield gs
    gs.close()


@pytest.fixture
def store(tmp_path):
    """Single CanonicalStore used for both tenants (they get separate DB files)."""
    s = CanonicalStore(data_dir=tmp_path / "tenants")
    with s._get_connection(TENANT_ALICE, create=True) as conn:
        s._create_schema(conn)
    with s._get_connection(TENANT_BOB, create=True) as conn:
        s._create_schema(conn)
    return s


# ── Helpers ──────────────────────────────────────────────────────────


def _create_node(store, owner=ALICE, type_id=1, tenant_id=TENANT_ALICE, acl=None):
    nid = str(uuid.uuid4())
    now = int(time.time() * 1000)
    node = store._sync_create_node(
        tenant_id,
        type_id,
        {"title": "test"},
        owner,
        nid,
        acl or [],
        now,
    )
    return node.node_id


def _share(store, node_id, actor_id, permission="read", granted_by=ALICE, tenant_id=TENANT_ALICE):
    now = int(time.time() * 1000)
    store._sync_share_node(
        tenant_id,
        node_id,
        actor_id,
        "user",
        permission,
        granted_by,
        now,
        None,
    )


def _share_expired(store, node_id, actor_id, tenant_id=TENANT_ALICE):
    """Share with an already-expired expiration."""
    past = int(time.time() * 1000) - 100_000
    store._sync_share_node(
        tenant_id,
        node_id,
        actor_id,
        "user",
        "read",
        ALICE,
        past - 1,
        past,
    )


def _share_deny(store, node_id, actor_id, tenant_id=TENANT_ALICE):
    """Grant a DENY entry for an actor."""
    now = int(time.time() * 1000)
    store._sync_share_node(
        tenant_id,
        node_id,
        actor_id,
        "user",
        "deny",
        ALICE,
        now,
        None,
    )


def _add_group_member(store, group_id, member_id, tenant_id=TENANT_ALICE):
    now = int(time.time() * 1000)
    store._sync_add_group_member(tenant_id, group_id, member_id, "member", now)


# ── CanonicalStore: has_node_access ──────────────────────────────────


@pytest.mark.asyncio
async def test_has_node_access_true(store):
    """has_node_access returns True when actor has at least one entry."""
    node_id = _create_node(store)
    _share(store, node_id, BOB)

    result = await store.has_node_access(TENANT_ALICE, [BOB])
    assert result is True


@pytest.mark.asyncio
async def test_has_node_access_false(store):
    """has_node_access returns False when actor has no entries."""
    _create_node(store)  # node exists but not shared with Bob

    result = await store.has_node_access(TENANT_ALICE, [BOB])
    assert result is False


@pytest.mark.asyncio
async def test_has_node_access_for_node_true(store):
    """has_node_access_for_node returns True for a specific shared node."""
    node_id = _create_node(store)
    _share(store, node_id, BOB)

    result = await store.has_node_access_for_node(TENANT_ALICE, node_id, [BOB])
    assert result is True


@pytest.mark.asyncio
async def test_has_node_access_for_node_false(store):
    """has_node_access_for_node returns False when actor has no access to specific node."""
    node_shared = _create_node(store)
    node_not_shared = _create_node(store)
    _share(store, node_shared, BOB)

    result = await store.has_node_access_for_node(TENANT_ALICE, node_not_shared, [BOB])
    assert result is False


@pytest.mark.asyncio
async def test_has_node_access_ignores_deny(store):
    """has_node_access returns False when actor only has DENY entries."""
    node_id = _create_node(store)
    _share_deny(store, node_id, BOB)

    result = await store.has_node_access(TENANT_ALICE, [BOB])
    assert result is False


@pytest.mark.asyncio
async def test_has_node_access_ignores_expired(store):
    """has_node_access returns False when all entries are expired."""
    node_id = _create_node(store)
    _share_expired(store, node_id, BOB)

    result = await store.has_node_access(TENANT_ALICE, [BOB])
    assert result is False


# ── GlobalStore: is_member ───────────────────────────────────────────


@pytest.mark.asyncio
async def test_is_member_true(global_store):
    """is_member returns True for a tenant member."""
    await global_store.create_tenant(TENANT_ALICE, "Alice Corp")
    await global_store.create_user("alice", "alice@example.com", "Alice")
    await global_store.add_member(TENANT_ALICE, "alice", role="owner")

    result = await global_store.is_member(TENANT_ALICE, "alice")
    assert result is True


@pytest.mark.asyncio
async def test_is_member_false(global_store):
    """is_member returns False for a non-member."""
    await global_store.create_tenant(TENANT_ALICE, "Alice Corp")

    result = await global_store.is_member(TENANT_ALICE, "bob")
    assert result is False


# ── Cross-tenant read: Bob reads node from Alice's tenant ────────────


@pytest.mark.asyncio
async def test_bob_reads_node_with_access(store):
    """Bob can read a node in Alice's tenant when he has node_access."""
    node_id = _create_node(store, owner=ALICE)
    _share(store, node_id, BOB, "read")

    # Verify Bob can access the node
    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    can = await store.can_access(TENANT_ALICE, node_id, actor_ids)
    assert can is True

    # Verify Bob can fetch the node
    node = await store.get_node(TENANT_ALICE, node_id)
    assert node is not None
    assert node.node_id == node_id


@pytest.mark.asyncio
async def test_bob_cannot_read_node_without_access(store):
    """Bob cannot access a node in Alice's tenant when he has no node_access."""
    node_id = _create_node(store, owner=ALICE)

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    can = await store.can_access(TENANT_ALICE, node_id, actor_ids)
    assert can is False


@pytest.mark.asyncio
async def test_bob_reads_shared_not_unshared(store):
    """Bob can access shared nodes but not unshared ones in Alice's tenant."""
    shared_id = _create_node(store, owner=ALICE)
    private_id = _create_node(store, owner=ALICE)
    _share(store, shared_id, BOB, "read")

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    assert await store.can_access(TENANT_ALICE, shared_id, actor_ids) is True
    assert await store.can_access(TENANT_ALICE, private_id, actor_ids) is False


# ── Cross-tenant QueryNodes: returns only accessible nodes ───────────


@pytest.mark.asyncio
async def test_cross_tenant_query_only_accessible(store):
    """QueryNodes-style filtering: Bob only sees nodes he has access to."""
    node1 = _create_node(store, owner=ALICE, type_id=10)
    node2 = _create_node(store, owner=ALICE, type_id=10)
    node3 = _create_node(store, owner=ALICE, type_id=10)

    # Share only node1 and node3 with Bob
    _share(store, node1, BOB, "read")
    _share(store, node3, BOB, "read")

    # Query all type_id=10 nodes
    all_nodes = await store.query_nodes(
        tenant_id=TENANT_ALICE, type_id=10, limit=100,
    )
    assert len(all_nodes) == 3

    # Filter to Bob's accessible nodes (simulates what gRPC server does)
    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    accessible = []
    for n in all_nodes:
        if await store.can_access(TENANT_ALICE, n.node_id, actor_ids):
            accessible.append(n)

    assert len(accessible) == 2
    accessible_ids = {n.node_id for n in accessible}
    assert node1 in accessible_ids
    assert node3 in accessible_ids
    assert node2 not in accessible_ids


@pytest.mark.asyncio
async def test_cross_tenant_query_no_access_empty(store):
    """QueryNodes returns empty for cross-tenant actor with no access."""
    _create_node(store, owner=ALICE, type_id=10)
    _create_node(store, owner=ALICE, type_id=10)

    all_nodes = await store.query_nodes(
        tenant_id=TENANT_ALICE, type_id=10, limit=100,
    )
    assert len(all_nodes) == 2

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    accessible = [
        n for n in all_nodes
        if await store.can_access(TENANT_ALICE, n.node_id, actor_ids)
    ]
    assert len(accessible) == 0


# ── shared_with_me returns cross-tenant entries ──────────────────────


@pytest.mark.asyncio
async def test_shared_with_me_cross_tenant_via_global_store(global_store, store):
    """ListSharedWithMe should include cross-tenant nodes from global shared_index."""
    # Create a node in Alice's tenant
    node_alice = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    # Share it with Bob and record in global shared_index
    _share(store, node_alice, BOB, "read", tenant_id=TENANT_ALICE)
    await global_store.add_shared(BOB, TENANT_ALICE, node_alice, "read")

    # Create a node in Bob's own tenant
    node_bob = _create_node(store, owner=BOB, tenant_id=TENANT_BOB)
    _share(store, node_bob, BOB, "read", granted_by=BOB, tenant_id=TENANT_BOB)

    # Bob's per-tenant shares in his own tenant
    actor_ids_bob = await store.resolve_actor_groups(TENANT_BOB, BOB)
    per_tenant = await store.list_shared_with_me(
        tenant_id=TENANT_BOB, actor_ids=actor_ids_bob, limit=100,
    )
    per_tenant_ids = {n.node_id for n in per_tenant}
    assert node_bob in per_tenant_ids

    # Cross-tenant entries from global store
    cross_entries = await global_store.get_shared_with_me(BOB)
    assert len(cross_entries) == 1
    assert cross_entries[0]["source_tenant"] == TENANT_ALICE
    assert cross_entries[0]["node_id"] == node_alice

    # Fetch the cross-tenant node
    cross_node = await store.get_node(TENANT_ALICE, node_alice)
    assert cross_node is not None
    assert cross_node.node_id == node_alice


@pytest.mark.asyncio
async def test_shared_with_me_multi_tenant(global_store, store):
    """shared_with_me aggregates entries from multiple source tenants."""
    node_a = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    node_b = _create_node(store, owner=BOB, tenant_id=TENANT_BOB)

    await global_store.add_shared(CAROL, TENANT_ALICE, node_a, "read")
    await global_store.add_shared(CAROL, TENANT_BOB, node_b, "write")

    entries = await global_store.get_shared_with_me(CAROL)
    assert len(entries) == 2
    tenants = {e["source_tenant"] for e in entries}
    assert tenants == {TENANT_ALICE, TENANT_BOB}


# ── Cross-tenant with groups ─────────────────────────────────────────


@pytest.mark.asyncio
async def test_cross_tenant_group_access(store):
    """Group member can access node shared with their group cross-tenant."""
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    _add_group_member(store, GROUP_TEAM, BOB, tenant_id=TENANT_ALICE)
    _share(store, node_id, GROUP_TEAM, "read", tenant_id=TENANT_ALICE)

    # Resolve Bob's groups in Alice's tenant
    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    assert GROUP_TEAM in actor_ids

    can = await store.can_access(TENANT_ALICE, node_id, actor_ids)
    assert can is True


@pytest.mark.asyncio
async def test_cross_tenant_group_shared_index_expansion(global_store, store):
    """Group share creates shared_index entries for each member."""
    _add_group_member(store, GROUP_TEAM, BOB, tenant_id=TENANT_ALICE)
    _add_group_member(store, GROUP_TEAM, CAROL, tenant_id=TENANT_ALICE)

    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    _share(store, node_id, GROUP_TEAM, "read", tenant_id=TENANT_ALICE)

    # Expand group for shared_index (simulates what gRPC handler does)
    members = await store.get_group_members(TENANT_ALICE, GROUP_TEAM)
    for member in members:
        await global_store.add_shared(member, TENANT_ALICE, node_id, "read")

    # Each member should have a cross-tenant entry
    bob_entries = await global_store.get_shared_with_me(BOB)
    carol_entries = await global_store.get_shared_with_me(CAROL)
    assert len(bob_entries) == 1
    assert len(carol_entries) == 1
    assert bob_entries[0]["node_id"] == node_id
    assert carol_entries[0]["node_id"] == node_id


@pytest.mark.asyncio
async def test_cross_tenant_group_has_node_access(store):
    """has_node_access returns True when actor's group has access."""
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    _add_group_member(store, GROUP_TEAM, BOB, tenant_id=TENANT_ALICE)
    _share(store, node_id, GROUP_TEAM, "read", tenant_id=TENANT_ALICE)

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    result = await store.has_node_access(TENANT_ALICE, actor_ids)
    assert result is True


# ── Edge cases ───────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_cross_tenant_deny_overrides_allow(store):
    """DENY entry blocks cross-tenant access even when node_access exists."""
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    _share(store, node_id, BOB, "read", tenant_id=TENANT_ALICE)
    _share_deny(store, node_id, BOB, tenant_id=TENANT_ALICE)

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    can = await store.can_access(TENANT_ALICE, node_id, actor_ids)
    assert can is False


@pytest.mark.asyncio
async def test_cross_tenant_expired_access_denied(store):
    """Expired node_access entry does not grant cross-tenant access."""
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    _share_expired(store, node_id, BOB, tenant_id=TENANT_ALICE)

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    can = await store.can_access(TENANT_ALICE, node_id, actor_ids)
    assert can is False


@pytest.mark.asyncio
async def test_system_actor_bypasses_cross_tenant(store):
    """System actor can access any node regardless of tenant membership."""
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)

    can = await store.can_access(TENANT_ALICE, node_id, [SYSTEM])
    assert can is True


@pytest.mark.asyncio
async def test_cross_tenant_write_permission(store):
    """Cross-tenant actor with write permission can access node."""
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    _share(store, node_id, BOB, "write", tenant_id=TENANT_ALICE)

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    can = await store.can_access(TENANT_ALICE, node_id, actor_ids)
    assert can is True

    # has_node_access should also be True
    result = await store.has_node_access(TENANT_ALICE, actor_ids)
    assert result is True


@pytest.mark.asyncio
async def test_cross_tenant_revoke_removes_access(store, global_store):
    """Revoking node_access removes cross-tenant access."""
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)
    _share(store, node_id, BOB, "read", tenant_id=TENANT_ALICE)
    await global_store.add_shared(BOB, TENANT_ALICE, node_id, "read")

    # Verify access
    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    assert await store.can_access(TENANT_ALICE, node_id, actor_ids) is True

    # Revoke
    await store.revoke_access(TENANT_ALICE, node_id, BOB)
    await global_store.remove_shared(BOB, TENANT_ALICE, node_id)

    # Verify access revoked
    assert await store.can_access(TENANT_ALICE, node_id, actor_ids) is False
    assert await global_store.get_shared_with_me(BOB) == []


@pytest.mark.asyncio
async def test_cross_tenant_multiple_nodes_mixed_access(store):
    """Bob has access to some nodes in Alice's tenant, not others."""
    nodes = [_create_node(store, owner=ALICE, type_id=5) for _ in range(5)]

    # Share only first two with Bob
    _share(store, nodes[0], BOB, "read")
    _share(store, nodes[1], BOB, "write")

    actor_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)

    # has_node_access is True (at least one entry exists)
    assert await store.has_node_access(TENANT_ALICE, actor_ids) is True

    # But Bob can only access the shared ones
    access_results = {
        nid: await store.can_access(TENANT_ALICE, nid, actor_ids)
        for nid in nodes
    }
    assert access_results[nodes[0]] is True
    assert access_results[nodes[1]] is True
    assert access_results[nodes[2]] is False
    assert access_results[nodes[3]] is False
    assert access_results[nodes[4]] is False


# ── End-to-end: cross-tenant ACL check ───────────────────────────────


@pytest.mark.asyncio
async def test_end_to_end_cross_tenant_acl(store, global_store):
    """Full end-to-end: Alice shares, Bob reads, Carol cannot read."""
    # Alice creates a node
    node_id = _create_node(store, owner=ALICE, tenant_id=TENANT_ALICE)

    # Alice shares with Bob
    _share(store, node_id, BOB, "read", tenant_id=TENANT_ALICE)
    await global_store.add_shared(BOB, TENANT_ALICE, node_id, "read")

    # Bob can see it
    bob_ids = await store.resolve_actor_groups(TENANT_ALICE, BOB)
    assert await store.can_access(TENANT_ALICE, node_id, bob_ids) is True
    assert await store.has_node_access(TENANT_ALICE, bob_ids) is True
    assert await store.has_node_access_for_node(TENANT_ALICE, node_id, bob_ids) is True

    bob_shared = await global_store.get_shared_with_me(BOB)
    assert len(bob_shared) == 1

    node = await store.get_node(TENANT_ALICE, node_id)
    assert node is not None

    # Carol cannot see it
    carol_ids = await store.resolve_actor_groups(TENANT_ALICE, CAROL)
    assert await store.can_access(TENANT_ALICE, node_id, carol_ids) is False
    assert await store.has_node_access(TENANT_ALICE, carol_ids) is False
    assert await store.has_node_access_for_node(TENANT_ALICE, node_id, carol_ids) is False

    carol_shared = await global_store.get_shared_with_me(CAROL)
    assert len(carol_shared) == 0
