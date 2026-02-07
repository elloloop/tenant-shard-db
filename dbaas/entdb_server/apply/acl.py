"""
ACL and visibility management for EntDB.

This module handles access control for nodes:
- Principal parsing (user:X, role:X, tenant:*)
- Permission checking (read, write, delete, admin)
- Visibility filtering for queries

Invariants:
    - Owner always has full access
    - tenant:* grants access to all users in tenant
    - ACL checks are performed before data access
    - Visibility index is derived from ACLs

How to change safely:
    - New principal types must be backward compatible
    - New permission levels must be additive
    - Test permission checks thoroughly before deployment
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from enum import Enum
from typing import Any

logger = logging.getLogger(__name__)


class Permission(Enum):
    """Permission levels for access control."""

    READ = "read"
    WRITE = "write"
    DELETE = "delete"
    ADMIN = "admin"


class AccessDeniedError(Exception):
    """Access denied due to insufficient permissions."""

    def __init__(
        self,
        actor: str,
        node_id: str,
        permission: Permission,
        message: str | None = None,
    ):
        self.actor = actor
        self.node_id = node_id
        self.permission = permission
        msg = message or f"Access denied: {actor} lacks {permission.value} on {node_id}"
        super().__init__(msg)


@dataclass(frozen=True)
class Principal:
    """Represents an actor or group for access control.

    Principal types:
        - user:ID - Specific user
        - role:NAME - Role-based access
        - group:ID - Group membership
        - tenant:* - All users in tenant
        - system:NAME - System/service accounts

    Attributes:
        type: Principal type (user, role, group, tenant, system)
        id: Principal identifier
    """

    type: str
    id: str

    @classmethod
    def parse(cls, principal_str: str) -> Principal:
        """Parse a principal string.

        Args:
            principal_str: String like "user:42" or "role:admin"

        Returns:
            Parsed Principal

        Raises:
            ValueError: If format is invalid
        """
        if ":" not in principal_str:
            raise ValueError(f"Invalid principal format: {principal_str}")

        parts = principal_str.split(":", 1)
        if len(parts) != 2:
            raise ValueError(f"Invalid principal format: {principal_str}")

        type_str, id_str = parts
        valid_types = {"user", "role", "group", "tenant", "system"}

        if type_str not in valid_types:
            raise ValueError(f"Invalid principal type: {type_str}")

        return cls(type=type_str, id=id_str)

    def __str__(self) -> str:
        return f"{self.type}:{self.id}"

    def matches(self, actor: str) -> bool:
        """Check if this principal matches an actor.

        Supports wildcard matching for tenant:* and role patterns.

        Args:
            actor: Actor string to check

        Returns:
            True if this principal grants access to the actor
        """
        # Exact match
        if str(self) == actor:
            return True

        # Wildcard matching
        if self.type == "tenant" and self.id == "*":
            # tenant:* matches all users in the tenant
            return True

        # Role matching (would need role lookup in production)
        if self.type == "role":
            # In production, this would check actor's roles
            # For now, just check exact match
            return False

        return False


@dataclass
class AclEntry:
    """ACL entry granting permission to a principal.

    Attributes:
        principal: Who gets access
        permission: What level of access
    """

    principal: Principal
    permission: Permission

    def to_dict(self) -> dict[str, str]:
        """Convert to dictionary for storage."""
        return {
            "principal": str(self.principal),
            "permission": self.permission.value,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> AclEntry:
        """Create from dictionary."""
        return cls(
            principal=Principal.parse(data["principal"]),
            permission=Permission(data["permission"]),
        )


class AclManager:
    """Manages access control for nodes.

    This class provides:
    - Permission checking for operations
    - ACL construction and validation
    - Visibility principal extraction

    Thread safety:
        This class is stateless and thread-safe.

    Example:
        >>> acl_manager = AclManager()
        >>> acl = [
        ...     {"principal": "user:42", "permission": "read"},
        ...     {"principal": "role:admin", "permission": "admin"},
        ... ]
        >>> acl_manager.check_permission("user:42", acl, Permission.READ, "owner:1")
        True
    """

    # Permission hierarchy (higher includes lower)
    PERMISSION_HIERARCHY = {
        Permission.READ: {Permission.READ},
        Permission.WRITE: {Permission.READ, Permission.WRITE},
        Permission.DELETE: {Permission.READ, Permission.WRITE, Permission.DELETE},
        Permission.ADMIN: {Permission.READ, Permission.WRITE, Permission.DELETE, Permission.ADMIN},
    }

    def check_permission(
        self,
        actor: str,
        acl: list[dict[str, str]],
        required: Permission,
        owner_actor: str,
    ) -> bool:
        """Check if an actor has required permission.

        Args:
            actor: Actor requesting access
            acl: Node's access control list
            required: Required permission level
            owner_actor: Node owner (always has full access)

        Returns:
            True if access is granted

        Example:
            >>> acl_manager.check_permission(
            ...     "user:42",
            ...     [{"principal": "user:42", "permission": "read"}],
            ...     Permission.READ,
            ...     "user:1",
            ... )
            True
        """
        # Owner always has full access
        if actor == owner_actor:
            return True

        # Check each ACL entry
        for entry_dict in acl:
            try:
                entry = AclEntry.from_dict(entry_dict)

                # Check if principal matches actor
                if entry.principal.matches(actor):
                    # Check if permission level is sufficient
                    granted_perms = self.PERMISSION_HIERARCHY.get(entry.permission, set())
                    if required in granted_perms:
                        return True

            except (ValueError, KeyError) as e:
                logger.warning(f"Invalid ACL entry: {entry_dict}, error: {e}")
                continue

        return False

    def check_permission_or_raise(
        self,
        actor: str,
        node_id: str,
        acl: list[dict[str, str]],
        required: Permission,
        owner_actor: str,
    ) -> None:
        """Check permission and raise if denied.

        Args:
            actor: Actor requesting access
            node_id: Node being accessed
            acl: Node's access control list
            required: Required permission level
            owner_actor: Node owner

        Raises:
            AccessDeniedError: If access is denied
        """
        if not self.check_permission(actor, acl, required, owner_actor):
            raise AccessDeniedError(actor, node_id, required)

    def extract_principals(
        self,
        acl: list[dict[str, str]],
        owner_actor: str,
    ) -> set[str]:
        """Extract all principals from an ACL for visibility indexing.

        Args:
            acl: Access control list
            owner_actor: Node owner

        Returns:
            Set of principal strings
        """
        principals = {owner_actor}

        for entry_dict in acl:
            try:
                principal_str = entry_dict.get("principal")
                if principal_str:
                    principals.add(principal_str)
            except Exception:
                continue

        return principals

    def merge_acls(
        self,
        base_acl: list[dict[str, str]],
        additional_acl: list[dict[str, str]],
    ) -> list[dict[str, str]]:
        """Merge two ACLs, with additional entries taking precedence.

        Args:
            base_acl: Base ACL entries
            additional_acl: Additional entries (override)

        Returns:
            Merged ACL
        """
        # Index base ACL by principal
        by_principal: dict[str, dict[str, str]] = {}
        for entry in base_acl:
            principal = entry.get("principal")
            if principal:
                by_principal[principal] = entry

        # Override with additional
        for entry in additional_acl:
            principal = entry.get("principal")
            if principal:
                by_principal[principal] = entry

        return list(by_principal.values())

    def validate_acl(self, acl: list[dict[str, str]]) -> list[str]:
        """Validate an ACL for correctness.

        Args:
            acl: Access control list to validate

        Returns:
            List of validation errors (empty if valid)
        """
        errors = []

        for i, entry in enumerate(acl):
            if not isinstance(entry, dict):
                errors.append(f"Entry {i}: must be a dictionary")
                continue

            principal_str = entry.get("principal")
            if not principal_str:
                errors.append(f"Entry {i}: missing 'principal'")
            else:
                try:
                    Principal.parse(principal_str)
                except ValueError as e:
                    errors.append(f"Entry {i}: {e}")

            permission_str = entry.get("permission")
            if not permission_str:
                errors.append(f"Entry {i}: missing 'permission'")
            else:
                try:
                    Permission(permission_str)
                except ValueError:
                    valid = [p.value for p in Permission]
                    errors.append(
                        f"Entry {i}: invalid permission '{permission_str}', must be one of {valid}"
                    )

        return errors

    def create_default_acl(
        self,
        owner_actor: str,
        tenant_readable: bool = False,
    ) -> list[dict[str, str]]:
        """Create a default ACL for a new node.

        Args:
            owner_actor: Node owner
            tenant_readable: Whether all tenant users can read

        Returns:
            Default ACL entries
        """
        acl = [
            {
                "principal": owner_actor,
                "permission": Permission.ADMIN.value,
            }
        ]

        if tenant_readable:
            acl.append(
                {
                    "principal": "tenant:*",
                    "permission": Permission.READ.value,
                }
            )

        return acl

    def can_modify_acl(
        self,
        actor: str,
        acl: list[dict[str, str]],
        owner_actor: str,
    ) -> bool:
        """Check if actor can modify the ACL.

        Requires ADMIN permission.

        Args:
            actor: Actor attempting modification
            acl: Current ACL
            owner_actor: Node owner

        Returns:
            True if modification is allowed
        """
        return self.check_permission(actor, acl, Permission.ADMIN, owner_actor)


# Default ACL manager instance
_default_manager: AclManager | None = None


def get_acl_manager() -> AclManager:
    """Get the default ACL manager instance."""
    global _default_manager
    if _default_manager is None:
        _default_manager = AclManager()
    return _default_manager
