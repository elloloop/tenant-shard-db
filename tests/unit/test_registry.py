"""
Unit tests for schema registry.

Tests cover:
- Type registration
- Registry freezing
- Fingerprint generation
- Duplicate detection
"""

import pytest
from dbaas.entdb_server.schema.types import NodeTypeDef, EdgeTypeDef, field
from dbaas.entdb_server.schema.registry import (
    SchemaRegistry,
    RegistryFrozenError,
    DuplicateRegistrationError,
)


class TestSchemaRegistry:
    """Tests for SchemaRegistry."""

    def test_register_node_type(self):
        """Can register a node type."""
        registry = SchemaRegistry()
        User = NodeTypeDef(type_id=1, name="User")

        registry.register_node_type(User)

        assert registry.get_node_type(1) == User
        assert registry.get_node_type("User") == User

    def test_register_edge_type(self):
        """Can register an edge type."""
        registry = SchemaRegistry()
        User = NodeTypeDef(type_id=1, name="User")
        Task = NodeTypeDef(type_id=2, name="Task")
        AssignedTo = EdgeTypeDef(edge_id=401, name="AssignedTo", from_type=Task, to_type=User)

        registry.register_node_type(User)
        registry.register_node_type(Task)
        registry.register_edge_type(AssignedTo)

        assert registry.get_edge_type(401) == AssignedTo
        assert registry.get_edge_type("AssignedTo") == AssignedTo

    def test_duplicate_type_id_raises(self):
        """Registering duplicate type_id raises error."""
        registry = SchemaRegistry()
        User1 = NodeTypeDef(type_id=1, name="User")
        User2 = NodeTypeDef(type_id=1, name="User2")

        registry.register_node_type(User1)

        with pytest.raises(DuplicateRegistrationError, match="type_id 1 already registered"):
            registry.register_node_type(User2)

    def test_duplicate_name_raises(self):
        """Registering duplicate name raises error."""
        registry = SchemaRegistry()
        User1 = NodeTypeDef(type_id=1, name="User")
        User2 = NodeTypeDef(type_id=2, name="User")

        registry.register_node_type(User1)

        with pytest.raises(DuplicateRegistrationError, match="name 'User' already registered"):
            registry.register_node_type(User2)

    def test_freeze_registry(self):
        """Can freeze registry."""
        registry = SchemaRegistry()
        User = NodeTypeDef(type_id=1, name="User")
        registry.register_node_type(User)

        fingerprint = registry.freeze()

        assert registry.frozen is True
        assert fingerprint is not None
        assert fingerprint.startswith("sha256:")
        assert registry.fingerprint == fingerprint

    def test_freeze_twice_raises(self):
        """Freezing twice raises error."""
        registry = SchemaRegistry()
        registry.freeze()

        with pytest.raises(RegistryFrozenError):
            registry.freeze()

    def test_register_after_freeze_raises(self):
        """Registering after freeze raises error."""
        registry = SchemaRegistry()
        registry.freeze()

        User = NodeTypeDef(type_id=1, name="User")

        with pytest.raises(RegistryFrozenError):
            registry.register_node_type(User)

    def test_fingerprint_deterministic(self):
        """Same schema produces same fingerprint."""
        registry1 = SchemaRegistry()
        registry2 = SchemaRegistry()

        User = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
        Task = NodeTypeDef(type_id=2, name="Task", fields=(field(1, "title", "str"),))

        # Register in same order
        registry1.register_node_type(User)
        registry1.register_node_type(Task)
        registry2.register_node_type(User)
        registry2.register_node_type(Task)

        fp1 = registry1.freeze()
        fp2 = registry2.freeze()

        assert fp1 == fp2

    def test_fingerprint_changes_with_schema(self):
        """Different schema produces different fingerprint."""
        registry1 = SchemaRegistry()
        registry2 = SchemaRegistry()

        User1 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
        User2 = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"), field(2, "name", "str")))

        registry1.register_node_type(User1)
        registry2.register_node_type(User2)

        fp1 = registry1.freeze()
        fp2 = registry2.freeze()

        assert fp1 != fp2

    def test_iterate_node_types(self):
        """Can iterate over node types."""
        registry = SchemaRegistry()
        User = NodeTypeDef(type_id=1, name="User")
        Task = NodeTypeDef(type_id=2, name="Task")

        registry.register_node_type(User)
        registry.register_node_type(Task)

        types = list(registry.node_types())
        assert len(types) == 2

    def test_to_dict(self):
        """Can serialize registry to dict."""
        registry = SchemaRegistry()
        User = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
        registry.register_node_type(User)

        d = registry.to_dict()

        assert "node_types" in d
        assert "edge_types" in d
        assert len(d["node_types"]) == 1
        assert d["node_types"][0]["type_id"] == 1

    def test_from_dict(self):
        """Can deserialize registry from dict."""
        d = {
            "node_types": [
                {"type_id": 1, "name": "User", "fields": [{"field_id": 1, "name": "email", "kind": "str"}]}
            ],
            "edge_types": [],
        }

        registry = SchemaRegistry.from_dict(d)

        assert registry.get_node_type(1) is not None
        assert registry.get_node_type("User").fields[0].name == "email"

    def test_validate_all(self):
        """Can validate registry for consistency."""
        registry = SchemaRegistry()
        User = NodeTypeDef(type_id=1, name="User")
        registry.register_node_type(User)

        # Edge referencing non-existent type
        BadEdge = EdgeTypeDef(edge_id=401, name="Bad", from_type=999, to_type=1)
        registry.register_edge_type(BadEdge)

        errors = registry.validate_all()
        assert len(errors) > 0
        assert any("unknown from_type_id" in e.lower() for e in errors)
