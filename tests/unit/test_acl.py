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
    Principal,
)


class TestPrincipalParsing:
    """Tests for principal parsing."""

    def test_parse_user_principal(self):
        """Parse user:id format."""
        principal = Principal.parse("user:alice")
        assert principal.type == "user"
        assert principal.id == "alice"

    def test_parse_role_principal(self):
        """Parse role:name format."""
        principal = Principal.parse("role:admin")
        assert principal.type == "role"
        assert principal.id == "admin"

    def test_parse_tenant_wildcard(self):
        """Parse tenant:* format."""
        principal = Principal.parse("tenant:*")
        assert principal.type == "tenant"
        assert principal.id == "*"

    def test_parse_invalid_format(self):
        """Invalid format raises error."""
        with pytest.raises(ValueError):
            Principal.parse("invalid")

    def test_parse_invalid_type(self):
        """Invalid principal type raises error."""
        with pytest.raises(ValueError):
            Principal.parse("badtype:id")

    def test_principal_str(self):
        """Principal converts to string."""
        principal = Principal(type="user", id="alice")
        assert str(principal) == "user:alice"


class TestPrincipalMatching:
    """Tests for principal matching."""

    def test_exact_match(self):
        """Principal matches exact actor string."""
        principal = Principal(type="user", id="alice")
        assert principal.matches("user:alice")

    def test_no_match(self):
        """Principal doesn't match different actor."""
        principal = Principal(type="user", id="alice")
        assert not principal.matches("user:bob")

    def test_tenant_wildcard_matches_all(self):
        """tenant:* matches any actor."""
        principal = Principal(type="tenant", id="*")
        assert principal.matches("user:anyone")


class TestPermissionHierarchy:
    """Tests for permission hierarchy in AclManager."""

    def test_read_only_grants_read(self):
        """READ only covers READ."""
        hierarchy = AclManager.PERMISSION_HIERARCHY
        assert Permission.READ in hierarchy[Permission.READ]
        assert Permission.WRITE not in hierarchy[Permission.READ]

    def test_write_includes_read(self):
        """WRITE grants READ and WRITE."""
        hierarchy = AclManager.PERMISSION_HIERARCHY
        assert Permission.READ in hierarchy[Permission.WRITE]
        assert Permission.WRITE in hierarchy[Permission.WRITE]
        assert Permission.DELETE not in hierarchy[Permission.WRITE]

    def test_admin_includes_all(self):
        """ADMIN grants all permissions."""
        hierarchy = AclManager.PERMISSION_HIERARCHY
        assert Permission.READ in hierarchy[Permission.ADMIN]
        assert Permission.WRITE in hierarchy[Permission.ADMIN]
        assert Permission.DELETE in hierarchy[Permission.ADMIN]
        assert Permission.ADMIN in hierarchy[Permission.ADMIN]


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
            acl=[],
            required=Permission.ADMIN,
            owner_actor="user:alice",
        )
        assert result is True

    def test_user_in_acl_has_permission(self, manager):
        """User in ACL has permission."""
        result = manager.check_permission(
            actor="user:bob",
            acl=[{"principal": "user:bob", "permission": "read"}],
            required=Permission.READ,
            owner_actor="user:alice",
        )
        assert result is True

    def test_user_not_in_acl_denied(self, manager):
        """User not in ACL is denied."""
        result = manager.check_permission(
            actor="user:charlie",
            acl=[{"principal": "user:bob", "permission": "read"}],
            required=Permission.READ,
            owner_actor="user:alice",
        )
        assert result is False

    def test_tenant_wildcard_grants_access(self, manager):
        """tenant:* grants access to all tenant users."""
        result = manager.check_permission(
            actor="user:anyone",
            acl=[{"principal": "tenant:*", "permission": "read"}],
            required=Permission.READ,
            owner_actor="user:alice",
        )
        assert result is True

    def test_higher_permission_grants_lower(self, manager):
        """Higher permission grants lower levels."""
        result = manager.check_permission(
            actor="user:bob",
            acl=[{"principal": "user:bob", "permission": "admin"}],
            required=Permission.READ,
            owner_actor="user:alice",
        )
        assert result is True

    def test_lower_permission_denies_higher(self, manager):
        """Lower permission denies higher levels."""
        result = manager.check_permission(
            actor="user:bob",
            acl=[{"principal": "user:bob", "permission": "read"}],
            required=Permission.WRITE,
            owner_actor="user:alice",
        )
        assert result is False

    def test_empty_acl_only_owner(self, manager):
        """Empty ACL means only owner has access."""
        assert not manager.check_permission(
            actor="user:bob",
            acl=[],
            required=Permission.READ,
            owner_actor="user:alice",
        )
        assert manager.check_permission(
            actor="user:alice",
            acl=[],
            required=Permission.ADMIN,
            owner_actor="user:alice",
        )

    def test_default_acl_contains_owner(self, manager):
        """Default ACL gives owner admin access."""
        acl = manager.create_default_acl("user:alice")
        assert any(e["principal"] == "user:alice" and e["permission"] == "admin" for e in acl)

    def test_default_acl_tenant_readable(self, manager):
        """Default ACL can include tenant:* read."""
        acl = manager.create_default_acl("user:alice", tenant_readable=True)
        assert any(e["principal"] == "tenant:*" and e["permission"] == "read" for e in acl)

    def test_validate_acl_valid(self, manager):
        """Valid ACL passes validation."""
        errors = manager.validate_acl(
            [
                {"principal": "user:alice", "permission": "admin"},
                {"principal": "tenant:*", "permission": "read"},
            ]
        )
        assert len(errors) == 0

    def test_validate_acl_invalid(self, manager):
        """Invalid ACL returns errors."""
        errors = manager.validate_acl(
            [
                {"principal": "invalid", "permission": "read"},
            ]
        )
        assert len(errors) > 0
