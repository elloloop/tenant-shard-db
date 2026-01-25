"""
Unit tests for ACL (Access Control List) system.

Tests cover:
- Principal parsing
- Permission checking
- Permission hierarchy
- Wildcard matching
"""

import pytest

from dbaas.entdb_server.apply.acl import (
    AclManager,
    Permission,
    PrincipalType,
    parse_principal,
)


class TestPrincipalParsing:
    """Tests for principal parsing."""

    def test_parse_user_principal(self):
        """Parse user:id format."""
        ptype, pid = parse_principal("user:alice")
        assert ptype == PrincipalType.USER
        assert pid == "alice"

    def test_parse_role_principal(self):
        """Parse role:name format."""
        ptype, pid = parse_principal("role:admin")
        assert ptype == PrincipalType.ROLE
        assert pid == "admin"

    def test_parse_tenant_wildcard(self):
        """Parse tenant:* format."""
        ptype, pid = parse_principal("tenant:*")
        assert ptype == PrincipalType.TENANT
        assert pid == "*"

    def test_parse_invalid_format(self):
        """Invalid format raises error."""
        with pytest.raises(ValueError):
            parse_principal("invalid")

    def test_parse_empty_id(self):
        """Empty ID raises error."""
        with pytest.raises(ValueError):
            parse_principal("user:")


class TestPermissionHierarchy:
    """Tests for permission hierarchy."""

    def test_read_is_lowest(self):
        """READ is lowest permission."""
        assert Permission.READ < Permission.WRITE
        assert Permission.READ < Permission.DELETE
        assert Permission.READ < Permission.ADMIN

    def test_write_implies_read(self):
        """WRITE is higher than READ."""
        assert Permission.WRITE > Permission.READ
        assert Permission.WRITE < Permission.DELETE

    def test_delete_implies_write(self):
        """DELETE is higher than WRITE."""
        assert Permission.DELETE > Permission.WRITE
        assert Permission.DELETE < Permission.ADMIN

    def test_admin_is_highest(self):
        """ADMIN is highest permission."""
        assert Permission.ADMIN > Permission.READ
        assert Permission.ADMIN > Permission.WRITE
        assert Permission.ADMIN > Permission.DELETE


class TestAclManager:
    """Tests for AclManager."""

    @pytest.fixture
    def manager(self):
        """Create ACL manager."""
        return AclManager()

    def test_owner_has_admin(self, manager):
        """Owner always has ADMIN permission."""
        result = manager.check_permission(
            actor="user:alice",
            owner="user:alice",
            principals=["user:bob"],
            required=Permission.ADMIN,
        )
        assert result is True

    def test_user_in_principals_has_permission(self, manager):
        """User in principals list has permission."""
        result = manager.check_permission(
            actor="user:bob",
            owner="user:alice",
            principals=["user:bob"],
            required=Permission.READ,
        )
        assert result is True

    def test_user_not_in_principals_denied(self, manager):
        """User not in principals is denied."""
        result = manager.check_permission(
            actor="user:charlie",
            owner="user:alice",
            principals=["user:bob"],
            required=Permission.READ,
        )
        assert result is False

    def test_tenant_wildcard_grants_access(self, manager):
        """tenant:* grants access to all tenant users."""
        result = manager.check_permission(
            actor="user:anyone",
            owner="user:alice",
            principals=["tenant:*"],
            required=Permission.READ,
        )
        assert result is True

    def test_role_principal(self, manager):
        """Role principal grants access to role members."""
        # Actor has admin role
        result = manager.check_permission(
            actor="user:bob",
            owner="user:alice",
            principals=["role:admin"],
            required=Permission.READ,
            actor_roles=["admin"],
        )
        assert result is True

    def test_role_not_matched_denied(self, manager):
        """User without role is denied."""
        result = manager.check_permission(
            actor="user:bob",
            owner="user:alice",
            principals=["role:admin"],
            required=Permission.READ,
            actor_roles=["member"],
        )
        assert result is False

    def test_permission_level_checked(self, manager):
        """Higher permission required fails with lower grant."""
        # Principal has READ, but WRITE is required
        result = manager.check_permission(
            actor="user:bob",
            owner="user:alice",
            principals=[("user:bob", Permission.READ)],
            required=Permission.WRITE,
        )
        assert result is False

    def test_higher_permission_grants_lower(self, manager):
        """Higher permission grants lower levels."""
        # Principal has ADMIN, READ is required
        result = manager.check_permission(
            actor="user:bob",
            owner="user:alice",
            principals=[("user:bob", Permission.ADMIN)],
            required=Permission.READ,
        )
        assert result is True

    def test_multiple_principals(self, manager):
        """Any matching principal grants access."""
        result = manager.check_permission(
            actor="user:charlie",
            owner="user:alice",
            principals=["user:bob", "user:charlie", "user:dave"],
            required=Permission.READ,
        )
        assert result is True

    def test_empty_principals_only_owner(self, manager):
        """Empty principals means only owner has access."""
        result = manager.check_permission(
            actor="user:bob",
            owner="user:alice",
            principals=[],
            required=Permission.READ,
        )
        assert result is False

        result = manager.check_permission(
            actor="user:alice",
            owner="user:alice",
            principals=[],
            required=Permission.ADMIN,
        )
        assert result is True


class TestAclManagerFiltering:
    """Tests for ACL filtering queries."""

    @pytest.fixture
    def manager(self):
        """Create ACL manager."""
        return AclManager()

    def test_filter_visible_nodes(self, manager):
        """Filter returns only visible nodes."""
        nodes = [
            {"id": "1", "owner": "user:alice", "principals": ["user:alice"]},
            {"id": "2", "owner": "user:alice", "principals": ["user:bob"]},
            {"id": "3", "owner": "user:alice", "principals": ["tenant:*"]},
        ]

        visible = manager.filter_visible(
            nodes=nodes,
            actor="user:bob",
            owner_key="owner",
            principals_key="principals",
        )

        visible_ids = {n["id"] for n in visible}
        assert visible_ids == {"2", "3"}

    def test_filter_includes_owned(self, manager):
        """Filter includes nodes owned by actor."""
        nodes = [
            {"id": "1", "owner": "user:alice", "principals": []},
            {"id": "2", "owner": "user:bob", "principals": []},
        ]

        visible = manager.filter_visible(
            nodes=nodes,
            actor="user:alice",
            owner_key="owner",
            principals_key="principals",
        )

        visible_ids = {n["id"] for n in visible}
        assert visible_ids == {"1"}
