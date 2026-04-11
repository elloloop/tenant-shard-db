"""Tests for ACL v2: inheritance-based access control.

Covers:
- Schema: AclDefaults, propagate_share
- Permission extensions: SHARE, COMMENT, DENY
- CanonicalStore ACL engine: can_access, resolve_actor_groups
- Inheritance chain: comment → task → project
- Private override (inherit=False on AclDefaults)
- Multi-parent inheritance (OR logic)
- Group membership resolution (nested groups)
- New node after share inherits automatically
- Revoke parent → children blocked
- Deny overrides inherited allow
- get_connected_nodes with ACL
- share_node / revoke_access / list_shared_with_me
- transfer_ownership
- Cycle detection in acl_inherit
- Compat checker: propagate_share + acl_defaults changes
"""

import time

import pytest

from dbaas.entdb_server.apply.acl import AclManager, Permission
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.schema.compat import (
    ChangeKind,
    check_compatibility,
)
from dbaas.entdb_server.schema.registry import SchemaRegistry
from sdk.entdb_sdk.schema import (
    AclDefaults,
    EdgeTypeDef,
    NodeTypeDef,
)

TENANT = "test_tenant"
ALICE = "user:alice"
BOB = "user:bob"
CAROL = "user:carol"
SYSTEM = "__system__"


# ── Fixtures ──────────────────────────────────────────────────────────


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=tmp_path)
    # Initialize tenant DB synchronously
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


@pytest.fixture
def _setup_tenant():
    """No-op — store fixture already initializes the tenant."""
    pass


# Helper to create a node and return its ID
def _create_node(store, owner=ALICE, type_id=1, tenant_id=TENANT, acl=None):
    import uuid
    nid = str(uuid.uuid4())
    now = int(time.time() * 1000)
    node = store._sync_create_node(
        tenant_id, type_id, {"title": "test"}, owner, nid, acl or [], now,
    )
    return node.node_id


def _create_edge(store, from_id, to_id, edge_type_id=1, propagates_acl=False):
    now = int(time.time() * 1000)
    with store._get_connection(TENANT) as conn:
        conn.execute(
            "INSERT OR REPLACE INTO edges "
            "(tenant_id, edge_type_id, from_node_id, to_node_id, "
            "props_json, propagates_acl, created_at) "
            "VALUES (?, ?, ?, ?, ?, ?, ?)",
            (TENANT, edge_type_id, from_id, to_id, "{}", 1 if propagates_acl else 0, now),
        )
        if propagates_acl:
            # Cycle detection
            cycle = conn.execute(
                """
                WITH RECURSIVE chain(nid, depth) AS (
                    SELECT ?, 0
                    UNION ALL
                    SELECT ai.inherit_from, c.depth + 1
                    FROM acl_inherit ai JOIN chain c ON ai.node_id = c.nid
                    WHERE c.depth < 10
                )
                SELECT 1 FROM chain WHERE nid = ? LIMIT 1
                """,
                (from_id, to_id),
            ).fetchone()
            if not cycle:
                conn.execute(
                    "INSERT OR IGNORE INTO acl_inherit (node_id, inherit_from) VALUES (?, ?)",
                    (to_id, from_id),
                )


def _share(store, node_id, actor_id, permission="read", granted_by=ALICE):
    now = int(time.time() * 1000)
    store._sync_share_node(
        TENANT, node_id, actor_id, "user", permission, granted_by, now, None,
    )


def _add_group_member(store, group_id, member_actor_id, role="member"):
    now = int(time.time() * 1000)
    store._sync_add_group_member(TENANT, group_id, member_actor_id, role, now)


# ── Schema Tests ──────────────────────────────────────────────────────


class TestAclDefaults:
    def test_default_values(self):
        d = AclDefaults()
        assert d.public is False
        assert d.tenant_visible is True
        assert d.inherit is True

    def test_private_defaults(self):
        d = AclDefaults(public=False, tenant_visible=False, inherit=False)
        assert d.inherit is False

    def test_to_dict(self):
        d = AclDefaults(public=True, tenant_visible=False, inherit=True)
        assert d.to_dict() == {"public": True, "tenant_visible": False, "inherit": True}

    def test_from_dict(self):
        d = AclDefaults.from_dict({"public": True, "tenant_visible": False, "inherit": False})
        assert d.public is True
        assert d.tenant_visible is False
        assert d.inherit is False

    def test_from_dict_defaults(self):
        d = AclDefaults.from_dict({})
        assert d.public is False
        assert d.tenant_visible is True
        assert d.inherit is True


class TestNodeTypeDefAclDefaults:
    def test_default_acl_defaults(self):
        Task = NodeTypeDef(type_id=1, name="Task")
        assert Task.acl_defaults.inherit is True
        assert Task.acl_defaults.tenant_visible is True

    def test_custom_acl_defaults(self):
        Grade = NodeTypeDef(
            type_id=2,
            name="Grade",
            acl_defaults=AclDefaults(inherit=False, tenant_visible=False),
        )
        assert Grade.acl_defaults.inherit is False
        assert Grade.acl_defaults.tenant_visible is False

    def test_to_dict_includes_acl_defaults(self):
        Task = NodeTypeDef(type_id=1, name="Task")
        d = Task.to_dict()
        assert "acl_defaults" in d
        assert d["acl_defaults"]["inherit"] is True


class TestEdgeTypeDefPropagateShare:
    def test_default_no_propagation(self):
        AssignedTo = EdgeTypeDef(edge_id=1, name="AssignedTo", from_type=1, to_type=2)
        assert AssignedTo.propagate_share is False

    def test_propagation_enabled(self):
        HasComment = EdgeTypeDef(
            edge_id=2, name="HasComment", from_type=1, to_type=2,
            propagate_share=True,
        )
        assert HasComment.propagate_share is True

    def test_to_dict_includes_propagate_share(self):
        e = EdgeTypeDef(edge_id=1, name="HasComment", from_type=1, to_type=2, propagate_share=True)
        d = e.to_dict()
        assert d["propagate_share"] is True


# ── Permission Extension Tests ────────────────────────────────────────


class TestPermissionExtensions:
    def test_new_permissions_exist(self):
        assert Permission.SHARE.value == "share"
        assert Permission.COMMENT.value == "comment"
        assert Permission.DENY.value == "deny"

    def test_admin_includes_share(self):
        mgr = AclManager()
        acl = [{"principal": ALICE, "permission": "admin"}]
        assert mgr.check_permission(ALICE, acl, Permission.SHARE, "other")

    def test_admin_includes_comment(self):
        mgr = AclManager()
        acl = [{"principal": ALICE, "permission": "admin"}]
        assert mgr.check_permission(ALICE, acl, Permission.COMMENT, "other")

    def test_write_includes_comment(self):
        mgr = AclManager()
        acl = [{"principal": ALICE, "permission": "write"}]
        assert mgr.check_permission(ALICE, acl, Permission.COMMENT, "other")

    def test_comment_does_not_include_write(self):
        mgr = AclManager()
        acl = [{"principal": ALICE, "permission": "comment"}]
        assert not mgr.check_permission(ALICE, acl, Permission.WRITE, "other")

    def test_deny_blocks_access(self):
        mgr = AclManager()
        acl = [
            {"principal": ALICE, "permission": "read"},
            {"principal": ALICE, "permission": "deny"},
        ]
        assert not mgr.check_permission(ALICE, acl, Permission.READ, "other")

    def test_deny_does_not_block_owner(self):
        mgr = AclManager()
        acl = [{"principal": ALICE, "permission": "deny"}]
        # Owner always has full access
        assert mgr.check_permission(ALICE, acl, Permission.READ, ALICE)

    def test_deny_grants_nothing(self):
        hierarchy = AclManager.PERMISSION_HIERARCHY
        assert hierarchy[Permission.DENY] == set()


# ── CanonicalStore ACL Engine Tests ───────────────────────────────────


class TestResolveActorGroups:
    def test_no_groups(self, store, _setup_tenant):
        result = store._sync_resolve_actor_groups(TENANT, ALICE)
        assert result == [ALICE]

    def test_single_group(self, store, _setup_tenant):
        _add_group_member(store, "group:eng", ALICE)
        result = store._sync_resolve_actor_groups(TENANT, ALICE)
        assert ALICE in result
        assert "group:eng" in result

    def test_nested_groups(self, store, _setup_tenant):
        _add_group_member(store, "group:eng", ALICE)
        _add_group_member(store, "group:all-staff", "group:eng")
        result = store._sync_resolve_actor_groups(TENANT, ALICE)
        assert ALICE in result
        assert "group:eng" in result
        assert "group:all-staff" in result

    def test_multiple_groups(self, store, _setup_tenant):
        _add_group_member(store, "group:eng", ALICE)
        _add_group_member(store, "group:design", ALICE)
        result = store._sync_resolve_actor_groups(TENANT, ALICE)
        assert len(result) == 3  # alice + 2 groups


class TestCanAccess:
    def test_owner_can_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        assert store._sync_can_access(TENANT, node_id, [ALICE])

    def test_non_owner_no_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        assert not store._sync_can_access(TENANT, node_id, [BOB])

    def test_system_actor_always_has_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        assert store._sync_can_access(TENANT, node_id, [SYSTEM])

    def test_direct_share_grants_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        _share(store, node_id, BOB, "read")
        assert store._sync_can_access(TENANT, node_id, [BOB])

    def test_tenant_wildcard_grants_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE, acl=[
            {"principal": "tenant:*", "permission": "read"},
        ])
        assert store._sync_can_access(TENANT, node_id, [BOB])

    def test_expired_share_no_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        past = int(time.time() * 1000) - 60000  # 1 minute ago
        store._sync_share_node(
            TENANT, node_id, BOB, "user", "read", ALICE, past - 120000, past,
        )
        assert not store._sync_can_access(TENANT, node_id, [BOB])


class TestAclInheritance:
    def test_child_inherits_from_parent(self, store, _setup_tenant):
        """Comment inherits access from task."""
        task_id = _create_node(store, owner=ALICE)
        comment_id = _create_node(store, owner=CAROL, type_id=2)
        _create_edge(store, task_id, comment_id, propagates_acl=True)
        _share(store, task_id, BOB, "read")
        # Bob can access comment via task inheritance
        assert store._sync_can_access(TENANT, comment_id, [BOB])

    def test_deep_inheritance(self, store, _setup_tenant):
        """project → task → comment chain."""
        project_id = _create_node(store, owner=ALICE)
        task_id = _create_node(store, owner=ALICE, type_id=2)
        comment_id = _create_node(store, owner=ALICE, type_id=3)
        _create_edge(store, project_id, task_id, propagates_acl=True)
        _create_edge(store, task_id, comment_id, propagates_acl=True)
        _share(store, project_id, BOB, "read")
        assert store._sync_can_access(TENANT, comment_id, [BOB])

    def test_no_inheritance_without_propagate(self, store, _setup_tenant):
        """Non-structural edge does not grant inheritance."""
        task_id = _create_node(store, owner=ALICE)
        related_id = _create_node(store, owner=CAROL)
        _create_edge(store, task_id, related_id, propagates_acl=False)
        _share(store, task_id, BOB, "read")
        # Bob cannot access related node
        assert not store._sync_can_access(TENANT, related_id, [BOB])

    def test_new_child_after_share_inherits(self, store, _setup_tenant):
        """Node created after parent was shared still inherits access."""
        task_id = _create_node(store, owner=ALICE)
        _share(store, task_id, BOB, "read")
        # Comment created AFTER sharing — still inherits
        comment_id = _create_node(store, owner=CAROL, type_id=2)
        _create_edge(store, task_id, comment_id, propagates_acl=True)
        assert store._sync_can_access(TENANT, comment_id, [BOB])

    def test_revoke_parent_blocks_children(self, store, _setup_tenant):
        """Revoking access to parent automatically blocks children."""
        task_id = _create_node(store, owner=ALICE)
        comment_id = _create_node(store, owner=CAROL, type_id=2)
        _create_edge(store, task_id, comment_id, propagates_acl=True)
        _share(store, task_id, BOB, "read")
        assert store._sync_can_access(TENANT, comment_id, [BOB])
        # Revoke Bob from task
        store._sync_revoke_access(TENANT, task_id, BOB)
        assert not store._sync_can_access(TENANT, comment_id, [BOB])

    def test_multi_parent_or_logic(self, store, _setup_tenant):
        """Node with two structural parents: access to either grants access."""
        task1_id = _create_node(store, owner=ALICE)
        task2_id = _create_node(store, owner=CAROL)
        subtask_id = _create_node(store, owner=ALICE, type_id=2)
        _create_edge(store, task1_id, subtask_id, propagates_acl=True)
        _create_edge(store, task2_id, subtask_id, propagates_acl=True)
        _share(store, task1_id, BOB, "read")
        # Bob has access via task1, even though no access to task2
        assert store._sync_can_access(TENANT, subtask_id, [BOB])


class TestDenyPrecedence:
    def test_explicit_deny_overrides_inherited_allow(self, store, _setup_tenant):
        """Explicit DENY on child blocks inherited access from parent."""
        task_id = _create_node(store, owner=ALICE)
        comment_id = _create_node(store, owner=CAROL, type_id=2)
        _create_edge(store, task_id, comment_id, propagates_acl=True)
        _share(store, task_id, BOB, "read")
        _share(store, comment_id, BOB, "deny")
        assert not store._sync_can_access(TENANT, comment_id, [BOB])

    def test_deny_does_not_affect_other_users(self, store, _setup_tenant):
        """Deny for Bob doesn't block Carol."""
        node_id = _create_node(store, owner=ALICE)
        _share(store, node_id, BOB, "deny")
        _share(store, node_id, CAROL, "read")
        assert not store._sync_can_access(TENANT, node_id, [BOB])
        assert store._sync_can_access(TENANT, node_id, [CAROL])


class TestGroupAccess:
    def test_group_share(self, store, _setup_tenant):
        """Access via group membership."""
        node_id = _create_node(store, owner=ALICE)
        _add_group_member(store, "group:eng", BOB)
        _share(store, node_id, "group:eng", "read")
        actor_ids = store._sync_resolve_actor_groups(TENANT, BOB)
        assert store._sync_can_access(TENANT, node_id, actor_ids)

    def test_nested_group_share(self, store, _setup_tenant):
        """Access via nested group: eng → all-staff."""
        node_id = _create_node(store, owner=ALICE)
        _add_group_member(store, "group:eng", BOB)
        _add_group_member(store, "group:all-staff", "group:eng")
        _share(store, node_id, "group:all-staff", "read")
        actor_ids = store._sync_resolve_actor_groups(TENANT, BOB)
        assert store._sync_can_access(TENANT, node_id, actor_ids)

    def test_group_inheritance(self, store, _setup_tenant):
        """Group access via parent inheritance chain."""
        task_id = _create_node(store, owner=ALICE)
        comment_id = _create_node(store, owner=CAROL, type_id=2)
        _create_edge(store, task_id, comment_id, propagates_acl=True)
        _add_group_member(store, "group:eng", BOB)
        _share(store, task_id, "group:eng", "read")
        actor_ids = store._sync_resolve_actor_groups(TENANT, BOB)
        assert store._sync_can_access(TENANT, comment_id, actor_ids)


class TestShareRevoke:
    def test_share_and_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        _share(store, node_id, BOB, "write")
        assert store._sync_can_access(TENANT, node_id, [BOB])

    def test_revoke_removes_access(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        _share(store, node_id, BOB, "read")
        assert store._sync_can_access(TENANT, node_id, [BOB])
        store._sync_revoke_access(TENANT, node_id, BOB)
        assert not store._sync_can_access(TENANT, node_id, [BOB])

    def test_revoke_nonexistent_returns_false(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        assert not store._sync_revoke_access(TENANT, node_id, BOB)


class TestListSharedWithMe:
    def test_lists_shared_nodes(self, store, _setup_tenant):
        n1 = _create_node(store, owner=ALICE)
        n2 = _create_node(store, owner=ALICE, type_id=2)
        _create_node(store, owner=ALICE, type_id=3)  # not shared
        _share(store, n1, BOB, "read")
        _share(store, n2, BOB, "write")
        nodes = store._sync_list_shared_with_me(TENANT, [BOB], 100, 0)
        node_ids = {n.node_id for n in nodes}
        assert n1 in node_ids
        assert n2 in node_ids
        assert len(nodes) == 2

    def test_excludes_denied_nodes(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        _share(store, node_id, BOB, "deny")
        nodes = store._sync_list_shared_with_me(TENANT, [BOB], 100, 0)
        assert len(nodes) == 0

    def test_includes_group_shares(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        _add_group_member(store, "group:eng", BOB)
        _share(store, node_id, "group:eng", "read")
        actor_ids = store._sync_resolve_actor_groups(TENANT, BOB)
        nodes = store._sync_list_shared_with_me(TENANT, actor_ids, 100, 0)
        assert len(nodes) == 1


class TestGetConnectedNodes:
    def test_returns_connected_with_access(self, store, _setup_tenant):
        task_id = _create_node(store, owner=ALICE)
        c1 = _create_node(store, owner=ALICE, type_id=2)
        c2 = _create_node(store, owner=ALICE, type_id=2)
        _create_edge(store, task_id, c1, edge_type_id=10, propagates_acl=True)
        _create_edge(store, task_id, c2, edge_type_id=10, propagates_acl=True)
        _share(store, task_id, BOB, "read")
        nodes = store._sync_get_connected_nodes(TENANT, task_id, 10, [BOB], 100, 0)
        ids = {n.node_id for n in nodes}
        assert c1 in ids
        assert c2 in ids

    def test_filters_denied_children(self, store, _setup_tenant):
        task_id = _create_node(store, owner=ALICE)
        c1 = _create_node(store, owner=ALICE, type_id=2)
        c2 = _create_node(store, owner=ALICE, type_id=2)
        _create_edge(store, task_id, c1, edge_type_id=10, propagates_acl=True)
        _create_edge(store, task_id, c2, edge_type_id=10, propagates_acl=True)
        _share(store, task_id, BOB, "read")
        _share(store, c2, BOB, "deny")
        nodes = store._sync_get_connected_nodes(TENANT, task_id, 10, [BOB], 100, 0)
        ids = {n.node_id for n in nodes}
        assert c1 in ids
        assert c2 not in ids

    def test_no_access_to_source_returns_empty(self, store, _setup_tenant):
        task_id = _create_node(store, owner=ALICE)
        c1 = _create_node(store, owner=ALICE, type_id=2)
        _create_edge(store, task_id, c1, edge_type_id=10, propagates_acl=True)
        # Bob has no access to task
        nodes = store._sync_get_connected_nodes(TENANT, task_id, 10, [BOB], 100, 0)
        assert len(nodes) == 0

    def test_system_actor_bypasses_acl(self, store, _setup_tenant):
        task_id = _create_node(store, owner=ALICE)
        c1 = _create_node(store, owner=ALICE, type_id=2)
        _create_edge(store, task_id, c1, edge_type_id=10, propagates_acl=True)
        nodes = store._sync_get_connected_nodes(TENANT, task_id, 10, [SYSTEM], 100, 0)
        assert len(nodes) == 1


class TestTransferOwnership:
    def test_transfer(self, store, _setup_tenant):
        node_id = _create_node(store, owner=ALICE)
        assert store._sync_can_access(TENANT, node_id, [ALICE])
        assert not store._sync_can_access(TENANT, node_id, [BOB])
        result = store._sync_transfer_ownership(TENANT, node_id, BOB)
        assert result is True
        assert store._sync_can_access(TENANT, node_id, [BOB])

    def test_transfer_nonexistent_returns_false(self, store, _setup_tenant):
        result = store._sync_transfer_ownership(TENANT, "nonexistent", BOB)
        assert result is False


class TestCycleDetection:
    def test_self_cycle_rejected(self, store, _setup_tenant):
        """A → A cycle should not create acl_inherit row."""
        node_id = _create_node(store, owner=ALICE)
        _create_edge(store, node_id, node_id, propagates_acl=True)
        with store._get_connection(TENANT) as conn:
            row = conn.execute(
                "SELECT COUNT(*) FROM acl_inherit WHERE node_id = ? AND inherit_from = ?",
                (node_id, node_id),
            ).fetchone()
            assert row[0] == 0

    def test_two_node_cycle_rejected(self, store, _setup_tenant):
        """A → B → A cycle: second edge should not create acl_inherit."""
        a = _create_node(store, owner=ALICE)
        b = _create_node(store, owner=ALICE, type_id=2)
        _create_edge(store, a, b, propagates_acl=True)
        _create_edge(store, b, a, propagates_acl=True)
        with store._get_connection(TENANT) as conn:
            rows = conn.execute("SELECT * FROM acl_inherit").fetchall()
            # Only A → B should exist, not B → A
            assert len(rows) == 1
            assert rows[0]["node_id"] == b
            assert rows[0]["inherit_from"] == a

    def test_three_node_cycle_rejected(self, store, _setup_tenant):
        """A → B → C → A cycle: C → A edge should not create acl_inherit."""
        a = _create_node(store, owner=ALICE)
        b = _create_node(store, owner=ALICE, type_id=2)
        c = _create_node(store, owner=ALICE, type_id=3)
        _create_edge(store, a, b, propagates_acl=True)
        _create_edge(store, b, c, propagates_acl=True)
        _create_edge(store, c, a, propagates_acl=True)
        with store._get_connection(TENANT) as conn:
            rows = conn.execute("SELECT * FROM acl_inherit").fetchall()
            # A→B and B→C should exist, but not C→A
            assert len(rows) == 2


class TestGroupMembership:
    def test_add_and_resolve(self, store, _setup_tenant):
        _add_group_member(store, "group:eng", ALICE)
        result = store._sync_resolve_actor_groups(TENANT, ALICE)
        assert "group:eng" in result

    def test_remove_member(self, store, _setup_tenant):
        _add_group_member(store, "group:eng", ALICE)
        store._sync_remove_group_member(TENANT, "group:eng", ALICE)
        result = store._sync_resolve_actor_groups(TENANT, ALICE)
        assert "group:eng" not in result

    def test_remove_nonexistent_returns_false(self, store, _setup_tenant):
        assert not store._sync_remove_group_member(TENANT, "group:eng", ALICE)


# ── Schema Compat Tests ──────────────────────────────────────────────


class TestSchemaCompat:
    def _make_registry(self, node_types=None, edge_types=None):
        reg = SchemaRegistry()
        for nt in (node_types or []):
            reg.register_node_type(nt)
        for et in (edge_types or []):
            reg.register_edge_type(et)
        return reg

    def test_propagate_share_change_is_breaking(self):
        Task = NodeTypeDef(type_id=1, name="Task")
        old = self._make_registry(
            node_types=[Task],
            edge_types=[EdgeTypeDef(edge_id=1, name="Has", from_type=1, to_type=1, propagate_share=False)],
        )
        new = self._make_registry(
            node_types=[Task],
            edge_types=[EdgeTypeDef(edge_id=1, name="Has", from_type=1, to_type=1, propagate_share=True)],
        )
        changes = check_compatibility(old, new)
        kinds = [c.kind for c in changes]
        assert ChangeKind.PROPAGATE_SHARE_CHANGED in kinds
        breaking = [c for c in changes if c.is_breaking]
        assert len(breaking) >= 1

    def test_acl_defaults_change_is_non_breaking(self):
        old = self._make_registry(
            node_types=[NodeTypeDef(type_id=1, name="Task")],
        )
        new = self._make_registry(
            node_types=[NodeTypeDef(
                type_id=1, name="Task",
                acl_defaults=AclDefaults(public=True, tenant_visible=False),
            )],
        )
        changes = check_compatibility(old, new)
        kinds = [c.kind for c in changes]
        assert ChangeKind.ACL_DEFAULTS_CHANGED in kinds
        breaking = [c for c in changes if c.is_breaking]
        assert len(breaking) == 0
