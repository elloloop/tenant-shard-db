"""
Unit tests for schema compatibility checking.

Tests cover:
- Detection of breaking changes
- Detection of non-breaking changes
- Specific change types
"""

import pytest

from dbaas.entdb_server.schema.compat import (
    ChangeKind,
    CompatibilityError,
    check_compatibility,
    validate_breaking_changes,
)
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import EdgeTypeDef, NodeTypeDef, field


def make_registry(*node_types, edge_types=None):
    """Helper to create a registry with types."""
    registry = SchemaRegistry()
    for nt in node_types:
        registry.register_node_type(nt)
    for et in edge_types or []:
        registry.register_edge_type(et)
    return registry


class TestCompatibilityChecking:
    """Tests for compatibility checking."""

    def test_no_changes(self):
        """Identical schemas have no changes."""
        User = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))

        old = make_registry(User)
        new = make_registry(User)

        changes = check_compatibility(old, new)
        assert len(changes) == 0

    def test_add_node_type(self):
        """Adding a node type is allowed."""
        User = NodeTypeDef(type_id=1, name="User")
        Task = NodeTypeDef(type_id=2, name="Task")

        old = make_registry(User)
        new = make_registry(User, Task)

        changes = check_compatibility(old, new)

        assert len(changes) == 1
        assert changes[0].kind == ChangeKind.NODE_TYPE_ADDED
        assert not changes[0].is_breaking

    def test_remove_node_type_is_breaking(self):
        """Removing a node type is breaking."""
        User = NodeTypeDef(type_id=1, name="User")
        Task = NodeTypeDef(type_id=2, name="Task")

        old = make_registry(User, Task)
        new = make_registry(User)

        changes = check_compatibility(old, new)

        breaking = [c for c in changes if c.is_breaking]
        assert len(breaking) == 1
        assert breaking[0].kind == ChangeKind.NODE_TYPE_REMOVED

    def test_add_field(self):
        """Adding a field is allowed."""
        UserV1 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
        UserV2 = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str"),
                field(2, "name", "str"),
            ),
        )

        old = make_registry(UserV1)
        new = make_registry(UserV2)

        changes = check_compatibility(old, new)

        assert len(changes) == 1
        assert changes[0].kind == ChangeKind.FIELD_ADDED
        assert not changes[0].is_breaking

    def test_remove_field_is_breaking(self):
        """Removing a field is breaking."""
        UserV1 = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str"),
                field(2, "name", "str"),
            ),
        )
        UserV2 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))

        old = make_registry(UserV1)
        new = make_registry(UserV2)

        changes = check_compatibility(old, new)

        breaking = [c for c in changes if c.is_breaking]
        assert len(breaking) == 1
        assert breaking[0].kind == ChangeKind.FIELD_REMOVED

    def test_change_field_kind_is_breaking(self):
        """Changing field kind is breaking."""
        UserV1 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "age", "str"),))
        UserV2 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "age", "int"),))

        old = make_registry(UserV1)
        new = make_registry(UserV2)

        changes = check_compatibility(old, new)

        breaking = [c for c in changes if c.is_breaking]
        assert len(breaking) == 1
        assert breaking[0].kind == ChangeKind.FIELD_KIND_CHANGED

    def test_rename_field_allowed(self):
        """Renaming a field (same ID) is allowed."""
        UserV1 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
        UserV2 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "emailAddress", "str"),))

        old = make_registry(UserV1)
        new = make_registry(UserV2)

        changes = check_compatibility(old, new)

        non_breaking = [c for c in changes if not c.is_breaking]
        assert len(non_breaking) == 1
        assert non_breaking[0].kind == ChangeKind.NAME_CHANGED

    def test_deprecate_field_allowed(self):
        """Deprecating a field is allowed."""
        UserV1 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
        UserV2 = NodeTypeDef(
            type_id=1, name="User", fields=(field(1, "email", "str", deprecated=True),)
        )

        old = make_registry(UserV1)
        new = make_registry(UserV2)

        changes = check_compatibility(old, new)

        assert len(changes) == 1
        assert changes[0].kind == ChangeKind.FIELD_DEPRECATED
        assert not changes[0].is_breaking

    def test_add_enum_value_allowed(self):
        """Adding enum value is allowed."""
        StatusV1 = NodeTypeDef(
            type_id=1,
            name="Task",
            fields=(field(1, "status", "enum", enum_values=("todo", "done")),),
        )
        StatusV2 = NodeTypeDef(
            type_id=1,
            name="Task",
            fields=(field(1, "status", "enum", enum_values=("todo", "doing", "done")),),
        )

        old = make_registry(StatusV1)
        new = make_registry(StatusV2)

        changes = check_compatibility(old, new)

        non_breaking = [c for c in changes if not c.is_breaking]
        assert any(c.kind == ChangeKind.ENUM_VALUE_ADDED for c in non_breaking)

    def test_remove_enum_value_is_breaking(self):
        """Removing enum value is breaking."""
        StatusV1 = NodeTypeDef(
            type_id=1,
            name="Task",
            fields=(field(1, "status", "enum", enum_values=("todo", "doing", "done")),),
        )
        StatusV2 = NodeTypeDef(
            type_id=1,
            name="Task",
            fields=(field(1, "status", "enum", enum_values=("todo", "done")),),
        )

        old = make_registry(StatusV1)
        new = make_registry(StatusV2)

        changes = check_compatibility(old, new)

        breaking = [c for c in changes if c.is_breaking]
        assert any(c.kind == ChangeKind.ENUM_VALUE_REMOVED for c in breaking)

    def test_make_optional_required_is_breaking(self):
        """Making an optional field required is breaking."""
        UserV1 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "name", "str"),))
        UserV2 = NodeTypeDef(
            type_id=1, name="User", fields=(field(1, "name", "str", required=True),)
        )

        old = make_registry(UserV1)
        new = make_registry(UserV2)

        changes = check_compatibility(old, new)

        breaking = [c for c in changes if c.is_breaking]
        assert any(c.kind == ChangeKind.REQUIRED_ADDED for c in breaking)

    def test_validate_breaking_changes_raises(self):
        """validate_breaking_changes raises on breaking changes."""
        UserV1 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
        UserV2 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "int"),))

        old = make_registry(UserV1)
        new = make_registry(UserV2)

        with pytest.raises(CompatibilityError) as exc_info:
            validate_breaking_changes(old, new)

        assert len(exc_info.value.changes) >= 1

    def test_edge_type_from_type_change_is_breaking(self):
        """Changing edge from_type is breaking."""
        User = NodeTypeDef(type_id=1, name="User")
        Task = NodeTypeDef(type_id=2, name="Task")
        Project = NodeTypeDef(type_id=3, name="Project")

        EdgeV1 = EdgeTypeDef(edge_id=401, name="AssignedTo", from_type=2, to_type=1)
        EdgeV2 = EdgeTypeDef(edge_id=401, name="AssignedTo", from_type=3, to_type=1)

        old = make_registry(User, Task, Project, edge_types=[EdgeV1])
        new = make_registry(User, Task, Project, edge_types=[EdgeV2])

        changes = check_compatibility(old, new)

        breaking = [c for c in changes if c.is_breaking]
        assert any(c.kind == ChangeKind.FROM_TYPE_CHANGED for c in breaking)
