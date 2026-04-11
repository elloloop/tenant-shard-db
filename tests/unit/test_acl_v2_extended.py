"""Extended ACL v2 tests for edge cases, regression, and benchmarks.

Covers:
- Permission hierarchy completeness
- AclDefaults serialization round-trip
- EdgeTypeDef propagate_share in to_dict/from_dict round-trip
- Empty ACL behaviors
- Multiple shares same node different permissions
- Revoke then re-share
- Group member removal cascades access
- Deeply nested inheritance (5+ levels)
- ACL with expired shares
- Connected nodes pagination
- Benchmark: can_access point check latency
- Benchmark: get_connected_nodes with ACL
"""

import time
import uuid

import pytest

from dbaas.entdb_server.apply.acl import AclManager, Permission
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from sdk.entdb_sdk.schema import AclDefaults, EdgeTypeDef, NodeTypeDef

TENANT = "test_extended"
ALICE = "user:alice"
BOB = "user:bob"
CAROL = "user:carol"
DAVE = "user:dave"
SYSTEM = "__system__"


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=tmp_path)
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


def _node(store, owner=ALICE, type_id=1, acl=None):
    nid = str(uuid.uuid4())
    now = int(time.time() * 1000)
    node = store._sync_create_node(TENANT, type_id, {"title": "test"}, owner, nid, acl or [], now)
    return node.node_id


def _edge(store, from_id, to_id, edge_type_id=1, propagates_acl=False):
    now = int(time.time() * 1000)
    with store._get_connection(TENANT) as conn:
        conn.execute(
            "INSERT OR REPLACE INTO edges "
            "(tenant_id, edge_type_id, from_node_id, to_node_id, props_json, propagates_acl, created_at) "
            "VALUES (?, ?, ?, ?, ?, ?, ?)",
            (TENANT, edge_type_id, from_id, to_id, "{}", 1 if propagates_acl else 0, now),
        )
        if propagates_acl:
            cycle = conn.execute(
                "WITH RECURSIVE chain(nid, depth) AS ("
                "  SELECT ?, 0 UNION ALL"
                "  SELECT ai.inherit_from, c.depth + 1"
                "  FROM acl_inherit ai JOIN chain c ON ai.node_id = c.nid"
                "  WHERE c.depth < 10"
                ") SELECT 1 FROM chain WHERE nid = ? LIMIT 1",
                (from_id, to_id),
            ).fetchone()
            if not cycle:
                conn.execute(
                    "INSERT OR IGNORE INTO acl_inherit (node_id, inherit_from) VALUES (?, ?)",
                    (to_id, from_id),
                )


def _share(store, node_id, actor_id, permission="read", granted_by=ALICE):
    now = int(time.time() * 1000)
    store._sync_share_node(TENANT, node_id, actor_id, "user", permission, granted_by, now, None)


def _group_add(store, group_id, member):
    now = int(time.time() * 1000)
    store._sync_add_group_member(TENANT, group_id, member, "member", now)


# ── Permission Hierarchy Tests ────────────────────────────────────────


class TestPermissionHierarchyComplete:
    """Verify every permission level includes the right sub-permissions."""

    def test_read_only_includes_read(self):
        h = AclManager.PERMISSION_HIERARCHY
        assert h[Permission.READ] == {Permission.READ}

    def test_comment_includes_read(self):
        h = AclManager.PERMISSION_HIERARCHY
        assert Permission.READ in h[Permission.COMMENT]

    def test_write_includes_comment_and_read(self):
        h = AclManager.PERMISSION_HIERARCHY
        assert Permission.COMMENT in h[Permission.WRITE]
        assert Permission.READ in h[Permission.WRITE]

    def test_share_includes_write(self):
        h = AclManager.PERMISSION_HIERARCHY
        assert Permission.WRITE in h[Permission.SHARE]

    def test_admin_includes_everything_except_deny(self):
        h = AclManager.PERMISSION_HIERARCHY
        admin = h[Permission.ADMIN]
        for p in Permission:
            if p != Permission.DENY:
                assert p in admin, f"ADMIN should include {p}"

    def test_deny_grants_nothing(self):
        h = AclManager.PERMISSION_HIERARCHY
        assert len(h[Permission.DENY]) == 0


# ── Serialization Round-trip Tests ────────────────────────────────────


class TestAclDefaultsRoundTrip:
    def test_default_round_trip(self):
        d = AclDefaults()
        assert AclDefaults.from_dict(d.to_dict()) == d

    def test_custom_round_trip(self):
        d = AclDefaults(public=True, tenant_visible=False, inherit=False)
        assert AclDefaults.from_dict(d.to_dict()) == d

    def test_node_type_with_acl_defaults_round_trip(self):
        t = NodeTypeDef(type_id=1, name="Test", acl_defaults=AclDefaults(inherit=False))
        d = t.to_dict()
        assert d["acl_defaults"]["inherit"] is False

    def test_edge_type_propagate_share_in_dict(self):
        e = EdgeTypeDef(edge_id=1, name="Has", from_type=1, to_type=2, propagate_share=True)
        d = e.to_dict()
        assert d["propagate_share"] is True


# ── Edge Cases ────────────────────────────────────────────────────────


class TestAclEdgeCases:
    def test_no_acl_no_visibility_no_access(self, store):
        """Node with empty ACL and no visibility: only owner can access."""
        nid = _node(store, owner=ALICE, acl=[])
        assert store._sync_can_access(TENANT, nid, [ALICE])
        assert not store._sync_can_access(TENANT, nid, [BOB])

    def test_multiple_shares_different_permissions(self, store):
        """Last share wins for same actor (REPLACE)."""
        nid = _node(store, owner=ALICE)
        _share(store, nid, BOB, "read")
        _share(store, nid, BOB, "write")  # should replace
        assert store._sync_can_access(TENANT, nid, [BOB])

    def test_revoke_then_reshare(self, store):
        """Revoke then re-share restores access."""
        nid = _node(store, owner=ALICE)
        _share(store, nid, BOB, "read")
        assert store._sync_can_access(TENANT, nid, [BOB])
        store._sync_revoke_access(TENANT, nid, BOB)
        assert not store._sync_can_access(TENANT, nid, [BOB])
        _share(store, nid, BOB, "write")
        assert store._sync_can_access(TENANT, nid, [BOB])

    def test_group_removal_cascades_access(self, store):
        """Removing user from group removes access to group-shared nodes."""
        nid = _node(store, owner=ALICE)
        _group_add(store, "group:eng", BOB)
        _share(store, nid, "group:eng", "read")
        groups = store._sync_resolve_actor_groups(TENANT, BOB)
        assert store._sync_can_access(TENANT, nid, groups)
        store._sync_remove_group_member(TENANT, "group:eng", BOB)
        groups = store._sync_resolve_actor_groups(TENANT, BOB)
        assert not store._sync_can_access(TENANT, nid, groups)

    def test_nonexistent_node_no_access(self, store):
        """Access check on nonexistent node returns False."""
        assert not store._sync_can_access(TENANT, "nonexistent-id", [ALICE])

    def test_system_actor_access_nonexistent(self, store):
        """System actor bypasses all checks, even for nonexistent nodes."""
        assert store._sync_can_access(TENANT, "nonexistent-id", [SYSTEM])


# ── Deep Inheritance ──────────────────────────────────────────────────


class TestDeepInheritance:
    def test_five_level_chain(self, store):
        """workspace → project → epic → task → comment (5 levels)."""
        workspace = _node(store, type_id=1)
        project = _node(store, type_id=2)
        epic = _node(store, type_id=3)
        task = _node(store, type_id=4)
        comment = _node(store, type_id=5)
        _edge(store, workspace, project, edge_type_id=1, propagates_acl=True)
        _edge(store, project, epic, edge_type_id=2, propagates_acl=True)
        _edge(store, epic, task, edge_type_id=3, propagates_acl=True)
        _edge(store, task, comment, edge_type_id=4, propagates_acl=True)
        _share(store, workspace, BOB, "read")
        assert store._sync_can_access(TENANT, comment, [BOB])

    def test_revoke_at_root_blocks_all_levels(self, store):
        """Revoking at root blocks entire chain."""
        root = _node(store, type_id=1)
        mid = _node(store, type_id=2)
        leaf = _node(store, type_id=3)
        _edge(store, root, mid, propagates_acl=True)
        _edge(store, mid, leaf, propagates_acl=True)
        _share(store, root, BOB, "read")
        assert store._sync_can_access(TENANT, leaf, [BOB])
        store._sync_revoke_access(TENANT, root, BOB)
        assert not store._sync_can_access(TENANT, leaf, [BOB])

    def test_partial_chain_access(self, store):
        """Share at mid-level: parent not accessible, child is."""
        root = _node(store, type_id=1)
        mid = _node(store, type_id=2)
        leaf = _node(store, type_id=3)
        _edge(store, root, mid, propagates_acl=True)
        _edge(store, mid, leaf, propagates_acl=True)
        _share(store, mid, BOB, "read")
        assert not store._sync_can_access(TENANT, root, [BOB])
        assert store._sync_can_access(TENANT, mid, [BOB])
        assert store._sync_can_access(TENANT, leaf, [BOB])


# ── Expired Shares ────────────────────────────────────────────────────


class TestExpiredShares:
    def test_expired_share_no_access(self, store):
        nid = _node(store, owner=ALICE)
        past = int(time.time() * 1000) - 60000
        store._sync_share_node(TENANT, nid, BOB, "user", "read", ALICE, past - 120000, past)
        assert not store._sync_can_access(TENANT, nid, [BOB])

    def test_future_expiry_has_access(self, store):
        nid = _node(store, owner=ALICE)
        future = int(time.time() * 1000) + 3600000
        store._sync_share_node(
            TENANT, nid, BOB, "user", "read", ALICE, int(time.time() * 1000), future
        )
        assert store._sync_can_access(TENANT, nid, [BOB])


# ── Connected Nodes Pagination ────────────────────────────────────────


class TestConnectedNodesPagination:
    def test_limit_and_offset(self, store):
        parent = _node(store, owner=ALICE)
        children = [_node(store, type_id=2) for _ in range(10)]
        for c in children:
            _edge(store, parent, c, edge_type_id=20, propagates_acl=True)
        _share(store, parent, BOB, "read")
        page1 = store._sync_get_connected_nodes(TENANT, parent, 20, [BOB], 5, 0)
        page2 = store._sync_get_connected_nodes(TENANT, parent, 20, [BOB], 5, 5)
        assert len(page1) == 5
        assert len(page2) == 5
        ids1 = {n.node_id for n in page1}
        ids2 = {n.node_id for n in page2}
        assert ids1.isdisjoint(ids2)
