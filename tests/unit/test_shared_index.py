"""Tests for cross-tenant shared_index integration (Issue #49).

Covers:
- GlobalStore: add_shared, remove_shared, get_shared_with_me, cleanup_stale_shared
- Share creates shared_index entry
- Revoke removes shared_index entry
- Group expansion on share (group: actor creates entries for all members)
- Group membership change cascades to shared_index
- Stale cleanup on node delete
- shared_with_me returns cross-tenant results
- Applier delete_node cleans up shared_index
- Multiple tenants, multiple shares
"""

import time
import uuid

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.global_store import GlobalStore

TENANT_A = "tenant_alice"
TENANT_B = "tenant_bob"
ALICE = "user:alice"
BOB = "user:bob"
CAROL = "user:carol"
DAVE = "user:dave"
EVE = "user:eve"
GROUP_FRIENDS = "group:friends"


# ── Fixtures ──────────────────────────────────────────────────────────


@pytest.fixture
def global_store(tmp_path):
    gs = GlobalStore(data_dir=tmp_path / "global")
    yield gs
    gs.close()


@pytest.fixture
def store_a(tmp_path):
    s = CanonicalStore(data_dir=tmp_path / "tenants")
    with s._get_connection(TENANT_A, create=True) as conn:
        s._create_schema(conn)
    return s


@pytest.fixture
def store_b(tmp_path):
    s = CanonicalStore(data_dir=tmp_path / "tenants_b")
    with s._get_connection(TENANT_B, create=True) as conn:
        s._create_schema(conn)
    return s


# ── Helpers ───────────────────────────────────────────────────────────


def _create_node(store, owner=ALICE, type_id=1, tenant_id=TENANT_A, acl=None):
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


def _share(store, node_id, actor_id, permission="read", granted_by=ALICE, tenant_id=TENANT_A):
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


def _add_group_member(store, group_id, member_id, tenant_id=TENANT_A):
    now = int(time.time() * 1000)
    store._sync_add_group_member(tenant_id, group_id, member_id, "member", now)


def _remove_group_member(store, group_id, member_id, tenant_id=TENANT_A):
    return store._sync_remove_group_member(tenant_id, group_id, member_id)


# ── GlobalStore basic tests ──────────────────────────────────────────


@pytest.mark.asyncio
async def test_add_shared_creates_entry(global_store):
    """add_shared creates a shared_index entry."""
    await global_store.add_shared(BOB, TENANT_A, "node-1", "read")
    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 1
    assert entries[0]["source_tenant"] == TENANT_A
    assert entries[0]["node_id"] == "node-1"
    assert entries[0]["permission"] == "read"


@pytest.mark.asyncio
async def test_remove_shared_deletes_entry(global_store):
    """remove_shared removes a shared_index entry."""
    await global_store.add_shared(BOB, TENANT_A, "node-1", "read")
    removed = await global_store.remove_shared(BOB, TENANT_A, "node-1")
    assert removed is True
    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 0


@pytest.mark.asyncio
async def test_remove_shared_returns_false_when_not_found(global_store):
    """remove_shared returns False when entry doesn't exist."""
    removed = await global_store.remove_shared(BOB, TENANT_A, "nonexistent")
    assert removed is False


@pytest.mark.asyncio
async def test_get_shared_with_me_pagination(global_store):
    """get_shared_with_me supports limit and offset."""
    for i in range(5):
        await global_store.add_shared(BOB, TENANT_A, f"node-{i}", "read")

    page1 = await global_store.get_shared_with_me(BOB, limit=2, offset=0)
    page2 = await global_store.get_shared_with_me(BOB, limit=2, offset=2)
    page3 = await global_store.get_shared_with_me(BOB, limit=2, offset=4)
    assert len(page1) == 2
    assert len(page2) == 2
    assert len(page3) == 1


@pytest.mark.asyncio
async def test_cleanup_stale_shared(global_store):
    """cleanup_stale_shared removes all entries for a deleted node."""
    await global_store.add_shared(BOB, TENANT_A, "node-del", "read")
    await global_store.add_shared(CAROL, TENANT_A, "node-del", "write")
    await global_store.add_shared(BOB, TENANT_A, "node-keep", "read")

    removed = await global_store.cleanup_stale_shared(TENANT_A, "node-del")
    assert removed == 2

    # node-del entries gone
    entries_bob = await global_store.get_shared_with_me(BOB)
    assert len(entries_bob) == 1
    assert entries_bob[0]["node_id"] == "node-keep"

    entries_carol = await global_store.get_shared_with_me(CAROL)
    assert len(entries_carol) == 0


@pytest.mark.asyncio
async def test_remove_all_shared_for_user(global_store):
    """remove_all_shared_for_user removes all entries for a user."""
    await global_store.add_shared(BOB, TENANT_A, "node-1", "read")
    await global_store.add_shared(BOB, TENANT_B, "node-2", "write")
    removed = await global_store.remove_all_shared_for_user(BOB)
    assert removed == 2
    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 0


@pytest.mark.asyncio
async def test_get_shared_entries_for_node(global_store):
    """get_shared_entries_for_node returns entries for a specific node."""
    await global_store.add_shared(BOB, TENANT_A, "node-x", "read")
    await global_store.add_shared(CAROL, TENANT_A, "node-x", "write")
    await global_store.add_shared(BOB, TENANT_A, "node-y", "read")

    entries = await global_store.get_shared_entries_for_node(TENANT_A, "node-x")
    assert len(entries) == 2
    user_ids = {e["user_id"] for e in entries}
    assert user_ids == {BOB, CAROL}


# ── Share creates shared_index entry ─────────────────────────────────


@pytest.mark.asyncio
async def test_share_node_creates_shared_index_entry(global_store, store_a):
    """When share_node is called, a shared_index entry is created."""
    node_id = _create_node(store_a)
    _share(store_a, node_id, BOB, "read")

    # Simulate what the gRPC handler does
    await global_store.add_shared(BOB, TENANT_A, node_id, "read")

    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 1
    assert entries[0]["node_id"] == node_id
    assert entries[0]["source_tenant"] == TENANT_A


@pytest.mark.asyncio
async def test_share_node_with_write_permission(global_store, store_a):
    """Share with write permission is tracked correctly."""
    node_id = _create_node(store_a)
    _share(store_a, node_id, BOB, "write")
    await global_store.add_shared(BOB, TENANT_A, node_id, "write")

    entries = await global_store.get_shared_with_me(BOB)
    assert entries[0]["permission"] == "write"


# ── Revoke removes shared_index entry ────────────────────────────────


@pytest.mark.asyncio
async def test_revoke_access_removes_shared_index_entry(global_store, store_a):
    """When revoke_access is called, the shared_index entry is removed."""
    node_id = _create_node(store_a)
    _share(store_a, node_id, BOB, "read")
    await global_store.add_shared(BOB, TENANT_A, node_id, "read")

    store_a._sync_revoke_access(TENANT_A, node_id, BOB)
    await global_store.remove_shared(BOB, TENANT_A, node_id)

    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 0


# ── Group expansion on share ─────────────────────────────────────────


@pytest.mark.asyncio
async def test_group_share_expands_to_members(global_store, store_a):
    """Sharing with a group creates individual shared_index entries for each member."""
    _add_group_member(store_a, GROUP_FRIENDS, BOB)
    _add_group_member(store_a, GROUP_FRIENDS, CAROL)
    _add_group_member(store_a, GROUP_FRIENDS, DAVE)

    node_id = _create_node(store_a)
    _share(store_a, node_id, GROUP_FRIENDS, "read")

    # Simulate group expansion (as gRPC handler / applier does)
    members = await store_a.get_group_members(TENANT_A, GROUP_FRIENDS)
    assert set(members) == {BOB, CAROL, DAVE}

    for member in members:
        await global_store.add_shared(member, TENANT_A, node_id, "read")

    # Each member should have an entry
    for member in [BOB, CAROL, DAVE]:
        entries = await global_store.get_shared_with_me(member)
        assert len(entries) == 1
        assert entries[0]["node_id"] == node_id


@pytest.mark.asyncio
async def test_group_share_empty_group(global_store, store_a):
    """Sharing with an empty group creates no shared_index entries."""
    node_id = _create_node(store_a)
    _share(store_a, node_id, GROUP_FRIENDS, "read")

    members = await store_a.get_group_members(TENANT_A, GROUP_FRIENDS)
    assert members == []

    for member in members:
        await global_store.add_shared(member, TENANT_A, node_id, "read")

    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 0


# ── Group membership change cascades to shared_index ─────────────────


@pytest.mark.asyncio
async def test_add_group_member_cascades_shared_index(global_store, store_a):
    """Adding a member to a group adds shared_index entries for all group-shared nodes."""
    _add_group_member(store_a, GROUP_FRIENDS, BOB)

    node1 = _create_node(store_a)
    node2 = _create_node(store_a)
    _share(store_a, node1, GROUP_FRIENDS, "read")
    _share(store_a, node2, GROUP_FRIENDS, "write")

    # Initially, Bob gets entries via group expansion
    for nid, perm in [(node1, "read"), (node2, "write")]:
        await global_store.add_shared(BOB, TENANT_A, nid, perm)

    # Now add Eve to the group
    _add_group_member(store_a, GROUP_FRIENDS, EVE)

    # Cascade: find all node_access for the group, add entries for Eve
    access_entries = await store_a.list_node_access_for_group(TENANT_A, GROUP_FRIENDS)
    assert len(access_entries) == 2

    for entry in access_entries:
        await global_store.add_shared(EVE, TENANT_A, entry["node_id"], entry["permission"])

    eve_entries = await global_store.get_shared_with_me(EVE)
    assert len(eve_entries) == 2
    node_ids = {e["node_id"] for e in eve_entries}
    assert node_ids == {node1, node2}


@pytest.mark.asyncio
async def test_remove_group_member_cascades_shared_index(global_store, store_a):
    """Removing a member from a group removes their shared_index entries."""
    _add_group_member(store_a, GROUP_FRIENDS, BOB)
    _add_group_member(store_a, GROUP_FRIENDS, CAROL)

    node_id = _create_node(store_a)
    _share(store_a, node_id, GROUP_FRIENDS, "read")

    await global_store.add_shared(BOB, TENANT_A, node_id, "read")
    await global_store.add_shared(CAROL, TENANT_A, node_id, "read")

    # Get group access entries BEFORE removing the member
    access_entries = await store_a.list_node_access_for_group(TENANT_A, GROUP_FRIENDS)

    # Remove Bob from the group
    _remove_group_member(store_a, GROUP_FRIENDS, BOB)

    # Cascade: remove Bob's shared_index entries for group-shared nodes
    for entry in access_entries:
        await global_store.remove_shared(BOB, TENANT_A, entry["node_id"])

    bob_entries = await global_store.get_shared_with_me(BOB)
    assert len(bob_entries) == 0

    # Carol should still have her entry
    carol_entries = await global_store.get_shared_with_me(CAROL)
    assert len(carol_entries) == 1


# ── Stale cleanup on node delete ─────────────────────────────────────


@pytest.mark.asyncio
async def test_delete_node_cleans_shared_index(global_store, store_a):
    """Deleting a node removes all shared_index entries for that node."""
    node_id = _create_node(store_a)
    _share(store_a, node_id, BOB, "read")
    _share(store_a, node_id, CAROL, "write")

    await global_store.add_shared(BOB, TENANT_A, node_id, "read")
    await global_store.add_shared(CAROL, TENANT_A, node_id, "write")

    # Simulate node deletion + cleanup
    removed = await global_store.cleanup_stale_shared(TENANT_A, node_id)
    assert removed == 2

    assert await global_store.get_shared_with_me(BOB) == []
    assert await global_store.get_shared_with_me(CAROL) == []


@pytest.mark.asyncio
async def test_cleanup_stale_only_affects_target_node(global_store, store_a):
    """Cleanup only removes entries for the specified node, not others."""
    node1 = _create_node(store_a)
    node2 = _create_node(store_a)

    await global_store.add_shared(BOB, TENANT_A, node1, "read")
    await global_store.add_shared(BOB, TENANT_A, node2, "read")

    await global_store.cleanup_stale_shared(TENANT_A, node1)

    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 1
    assert entries[0]["node_id"] == node2


# ── Cross-tenant shared_with_me ──────────────────────────────────────


@pytest.mark.asyncio
async def test_shared_with_me_cross_tenant(global_store, store_a, store_b):
    """shared_with_me returns results from multiple source tenants."""
    node_a = _create_node(store_a, owner=ALICE, tenant_id=TENANT_A)
    node_b = _create_node(store_b, owner=CAROL, tenant_id=TENANT_B)

    await global_store.add_shared(BOB, TENANT_A, node_a, "read")
    await global_store.add_shared(BOB, TENANT_B, node_b, "write")

    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 2
    tenants = {e["source_tenant"] for e in entries}
    assert tenants == {TENANT_A, TENANT_B}


@pytest.mark.asyncio
async def test_shared_with_me_different_permissions(global_store):
    """shared_with_me correctly tracks different permissions per node."""
    await global_store.add_shared(BOB, TENANT_A, "node-1", "read")
    await global_store.add_shared(BOB, TENANT_A, "node-2", "write")
    await global_store.add_shared(BOB, TENANT_B, "node-3", "admin")

    entries = await global_store.get_shared_with_me(BOB)
    perms = {e["node_id"]: e["permission"] for e in entries}
    assert perms["node-1"] == "read"
    assert perms["node-2"] == "write"
    assert perms["node-3"] == "admin"


# ── CanonicalStore new methods ───────────────────────────────────────


@pytest.mark.asyncio
async def test_get_group_members(store_a):
    """get_group_members returns all members of a group."""
    _add_group_member(store_a, GROUP_FRIENDS, BOB)
    _add_group_member(store_a, GROUP_FRIENDS, CAROL)

    members = await store_a.get_group_members(TENANT_A, GROUP_FRIENDS)
    assert set(members) == {BOB, CAROL}


@pytest.mark.asyncio
async def test_get_group_members_empty(store_a):
    """get_group_members returns empty list for nonexistent group."""
    members = await store_a.get_group_members(TENANT_A, "group:nonexistent")
    assert members == []


@pytest.mark.asyncio
async def test_list_node_access_for_group(store_a):
    """list_node_access_for_group returns all node_access entries for a group."""
    node1 = _create_node(store_a)
    node2 = _create_node(store_a)

    _share(store_a, node1, GROUP_FRIENDS, "read")
    _share(store_a, node2, GROUP_FRIENDS, "write")

    entries = await store_a.list_node_access_for_group(TENANT_A, GROUP_FRIENDS)
    assert len(entries) == 2
    node_perms = {e["node_id"]: e["permission"] for e in entries}
    assert node_perms[node1] == "read"
    assert node_perms[node2] == "write"


@pytest.mark.asyncio
async def test_list_node_access_for_group_empty(store_a):
    """list_node_access_for_group returns empty for group with no access."""
    entries = await store_a.list_node_access_for_group(TENANT_A, GROUP_FRIENDS)
    assert entries == []


# ── Applier integration (delete_node triggers cleanup) ───────────────


@pytest.mark.asyncio
async def test_applier_delete_triggers_shared_cleanup(global_store, store_a):
    """Applier's delete_node path cleans up shared_index entries."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier, TransactionEvent

    node_id = _create_node(store_a)
    await global_store.add_shared(BOB, TENANT_A, node_id, "read")
    await global_store.add_shared(CAROL, TENANT_A, node_id, "write")

    # Verify entries exist
    assert len(await global_store.get_shared_with_me(BOB)) == 1
    assert len(await global_store.get_shared_with_me(CAROL)) == 1

    # Create a minimal Applier with global_store
    mock_wal = AsyncMock()
    mock_mailbox = AsyncMock()

    applier = Applier(
        wal=mock_wal,
        canonical_store=store_a,
        mailbox_store=mock_mailbox,
        global_store=global_store,
    )

    event = TransactionEvent(
        tenant_id=TENANT_A,
        actor=ALICE,
        idempotency_key=str(uuid.uuid4()),
        schema_fingerprint=None,
        ts_ms=int(time.time() * 1000),
        ops=[{"op": "delete_node", "id": node_id}],
    )

    result = await applier.apply_event(event)
    assert result.success

    # shared_index entries should be cleaned up
    assert await global_store.get_shared_with_me(BOB) == []
    assert await global_store.get_shared_with_me(CAROL) == []


@pytest.mark.asyncio
async def test_applier_delete_no_global_store_no_crash(store_a):
    """Applier without global_store doesn't crash on delete_node."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier, TransactionEvent

    node_id = _create_node(store_a)
    mock_wal = AsyncMock()
    mock_mailbox = AsyncMock()

    applier = Applier(
        wal=mock_wal,
        canonical_store=store_a,
        mailbox_store=mock_mailbox,
        # No global_store
    )

    event = TransactionEvent(
        tenant_id=TENANT_A,
        actor=ALICE,
        idempotency_key=str(uuid.uuid4()),
        schema_fingerprint=None,
        ts_ms=int(time.time() * 1000),
        ops=[{"op": "delete_node", "id": node_id}],
    )

    result = await applier.apply_event(event)
    assert result.success


# ── Applier shared_index helper methods ──────────────────────────────


@pytest.mark.asyncio
async def test_applier_shared_index_on_share_direct_user(global_store, store_a):
    """_update_shared_index_on_share creates entry for direct user."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier

    applier = Applier(
        wal=AsyncMock(),
        canonical_store=store_a,
        mailbox_store=AsyncMock(),
        global_store=global_store,
    )

    node_id = _create_node(store_a)
    await applier._update_shared_index_on_share(TENANT_A, node_id, BOB, "read")

    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 1
    assert entries[0]["node_id"] == node_id


@pytest.mark.asyncio
async def test_applier_shared_index_on_share_group(global_store, store_a):
    """_update_shared_index_on_share expands group to individual entries."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier

    _add_group_member(store_a, GROUP_FRIENDS, BOB)
    _add_group_member(store_a, GROUP_FRIENDS, CAROL)

    applier = Applier(
        wal=AsyncMock(),
        canonical_store=store_a,
        mailbox_store=AsyncMock(),
        global_store=global_store,
    )

    node_id = _create_node(store_a)
    await applier._update_shared_index_on_share(TENANT_A, node_id, GROUP_FRIENDS, "read")

    bob_entries = await global_store.get_shared_with_me(BOB)
    carol_entries = await global_store.get_shared_with_me(CAROL)
    assert len(bob_entries) == 1
    assert len(carol_entries) == 1


@pytest.mark.asyncio
async def test_applier_shared_index_on_revoke_direct(global_store, store_a):
    """_update_shared_index_on_revoke removes entry for direct user."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier

    applier = Applier(
        wal=AsyncMock(),
        canonical_store=store_a,
        mailbox_store=AsyncMock(),
        global_store=global_store,
    )

    node_id = _create_node(store_a)
    await global_store.add_shared(BOB, TENANT_A, node_id, "read")
    assert len(await global_store.get_shared_with_me(BOB)) == 1

    await applier._update_shared_index_on_revoke(TENANT_A, node_id, BOB)
    assert len(await global_store.get_shared_with_me(BOB)) == 0


@pytest.mark.asyncio
async def test_applier_shared_index_on_revoke_group(global_store, store_a):
    """_update_shared_index_on_revoke removes entries for group members."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier

    _add_group_member(store_a, GROUP_FRIENDS, BOB)
    _add_group_member(store_a, GROUP_FRIENDS, CAROL)

    applier = Applier(
        wal=AsyncMock(),
        canonical_store=store_a,
        mailbox_store=AsyncMock(),
        global_store=global_store,
    )

    node_id = _create_node(store_a)
    await global_store.add_shared(BOB, TENANT_A, node_id, "read")
    await global_store.add_shared(CAROL, TENANT_A, node_id, "read")

    await applier._update_shared_index_on_revoke(TENANT_A, node_id, GROUP_FRIENDS)

    assert await global_store.get_shared_with_me(BOB) == []
    assert await global_store.get_shared_with_me(CAROL) == []


@pytest.mark.asyncio
async def test_applier_group_member_add_cascades(global_store, store_a):
    """_update_shared_index_on_group_member_add cascades entries."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier

    node1 = _create_node(store_a)
    node2 = _create_node(store_a)
    _share(store_a, node1, GROUP_FRIENDS, "read")
    _share(store_a, node2, GROUP_FRIENDS, "write")

    applier = Applier(
        wal=AsyncMock(),
        canonical_store=store_a,
        mailbox_store=AsyncMock(),
        global_store=global_store,
    )

    await applier._update_shared_index_on_group_member_add(TENANT_A, GROUP_FRIENDS, EVE)

    eve_entries = await global_store.get_shared_with_me(EVE)
    assert len(eve_entries) == 2
    node_ids = {e["node_id"] for e in eve_entries}
    assert node_ids == {node1, node2}


@pytest.mark.asyncio
async def test_applier_group_member_remove_cascades(global_store, store_a):
    """_update_shared_index_on_group_member_remove removes entries."""
    from unittest.mock import AsyncMock

    from dbaas.entdb_server.apply.applier import Applier

    node1 = _create_node(store_a)
    _share(store_a, node1, GROUP_FRIENDS, "read")

    await global_store.add_shared(BOB, TENANT_A, node1, "read")

    applier = Applier(
        wal=AsyncMock(),
        canonical_store=store_a,
        mailbox_store=AsyncMock(),
        global_store=global_store,
    )

    await applier._update_shared_index_on_group_member_remove(TENANT_A, GROUP_FRIENDS, BOB)

    assert await global_store.get_shared_with_me(BOB) == []


# ── Edge cases ───────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_upsert_shared_updates_permission(global_store):
    """Re-sharing with different permission updates the entry."""
    await global_store.add_shared(BOB, TENANT_A, "node-1", "read")
    await global_store.add_shared(BOB, TENANT_A, "node-1", "write")

    entries = await global_store.get_shared_with_me(BOB)
    assert len(entries) == 1
    assert entries[0]["permission"] == "write"


@pytest.mark.asyncio
async def test_shared_index_isolation_between_users(global_store):
    """Shares for one user don't appear for another."""
    await global_store.add_shared(BOB, TENANT_A, "node-1", "read")
    await global_store.add_shared(CAROL, TENANT_A, "node-2", "write")

    bob_entries = await global_store.get_shared_with_me(BOB)
    carol_entries = await global_store.get_shared_with_me(CAROL)

    assert len(bob_entries) == 1
    assert bob_entries[0]["node_id"] == "node-1"
    assert len(carol_entries) == 1
    assert carol_entries[0]["node_id"] == "node-2"
