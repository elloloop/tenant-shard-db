"""
Unit tests for schema types.

Tests cover:
- FieldDef creation and validation
- NodeTypeDef creation and validation
- EdgeTypeDef creation and validation
- Payload validation
- Type serialization/deserialization
"""

import pytest

from dbaas.entdb_server.schema.types import (
    EdgeTypeDef,
    FieldDef,
    FieldKind,
    NodeTypeDef,
    field,
)


class TestFieldDef:
    """Tests for FieldDef."""

    def test_create_string_field(self):
        """String field can be created."""
        f = field(1, "title", "str", required=True)
        assert f.field_id == 1
        assert f.name == "title"
        assert f.kind == FieldKind.STRING
        assert f.required is True

    def test_create_enum_field(self):
        """Enum field requires enum_values."""
        f = field(1, "status", "enum", enum_values=("todo", "done"))
        assert f.kind == FieldKind.ENUM
        assert f.enum_values == ("todo", "done")

    def test_enum_field_without_values_raises(self):
        """Enum field without values raises ValueError."""
        with pytest.raises(ValueError, match="enum_values required"):
            field(1, "status", "enum")

    def test_field_id_must_be_positive(self):
        """Field ID must be positive."""
        with pytest.raises(ValueError, match="field_id must be"):
            field(0, "name", "str")

    def test_field_id_max_value(self):
        """Field ID has max value of 65535."""
        with pytest.raises(ValueError, match="field_id must be"):
            field(70000, "name", "str")

    def test_validate_required_field(self):
        """Required field validation."""
        f = field(1, "title", "str", required=True)
        is_valid, error = f.validate_value(None)
        assert not is_valid
        assert "required" in error

    def test_validate_enum_value(self):
        """Enum value validation."""
        f = field(1, "status", "enum", enum_values=("todo", "done"))

        is_valid, _ = f.validate_value("todo")
        assert is_valid

        is_valid, error = f.validate_value("invalid")
        assert not is_valid
        assert "must be one of" in error

    def test_field_to_dict(self):
        """Field can be serialized to dict."""
        f = field(1, "title", "str", required=True, indexed=True)
        d = f.to_dict()

        assert d["field_id"] == 1
        assert d["name"] == "title"
        assert d["kind"] == "str"
        assert d["required"] is True
        assert d["indexed"] is True

    def test_field_from_dict(self):
        """Field can be deserialized from dict."""
        d = {
            "field_id": 1,
            "name": "status",
            "kind": "enum",
            "enum_values": ["todo", "done"],
        }
        f = FieldDef.from_dict(d)

        assert f.field_id == 1
        assert f.name == "status"
        assert f.kind == FieldKind.ENUM
        assert f.enum_values == ("todo", "done")


class TestNodeTypeDef:
    """Tests for NodeTypeDef."""

    def test_create_node_type(self):
        """Node type can be created."""
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str", required=True),
                field(2, "name", "str"),
            ),
        )

        assert User.type_id == 1
        assert User.name == "User"
        assert len(User.fields) == 2

    def test_type_id_must_be_positive(self):
        """Type ID must be positive."""
        with pytest.raises(ValueError, match="type_id must be positive"):
            NodeTypeDef(type_id=0, name="Invalid")

    def test_duplicate_field_ids_raise(self):
        """Duplicate field IDs raise ValueError."""
        with pytest.raises(ValueError, match="Duplicate field_id"):
            NodeTypeDef(
                type_id=1,
                name="Invalid",
                fields=(
                    field(1, "email", "str"),
                    field(1, "name", "str"),  # Duplicate ID
                ),
            )

    def test_get_field_by_name(self):
        """Can get field by name."""
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str"),),
        )

        f = User.get_field("email")
        assert f is not None
        assert f.field_id == 1

    def test_get_field_by_id(self):
        """Can get field by ID."""
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str"),),
        )

        f = User.get_field(1)
        assert f is not None
        assert f.name == "email"

    def test_validate_payload_valid(self):
        """Valid payload passes validation."""
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str", required=True),
                field(2, "name", "str"),
            ),
        )

        is_valid, errors = User.validate_payload({"email": "test@test.com"})
        assert is_valid
        assert len(errors) == 0

    def test_validate_payload_missing_required(self):
        """Missing required field fails validation."""
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str", required=True),),
        )

        is_valid, errors = User.validate_payload({})
        assert not is_valid
        assert any("required" in e for e in errors)

    def test_validate_payload_unknown_field(self):
        """Unknown field fails validation."""
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str"),),
        )

        is_valid, errors = User.validate_payload({"email": "test@test.com", "unknown": "value"})
        assert not is_valid
        assert any("Unknown" in e for e in errors)

    def test_new_creates_validated_payload(self):
        """new() creates a validated payload."""
        Task = NodeTypeDef(
            type_id=101,
            name="Task",
            fields=(
                field(1, "title", "str", required=True),
                field(2, "status", "enum", enum_values=("todo", "done")),
            ),
        )

        payload = Task.new(title="My Task", status="todo")
        assert payload["title"] == "My Task"
        assert payload["status"] == "todo"

    def test_new_raises_on_unknown_field(self):
        """new() raises TypeError on unknown field."""
        Task = NodeTypeDef(
            type_id=101,
            name="Task",
            fields=(field(1, "title", "str"),),
        )

        with pytest.raises(TypeError, match="Unknown field"):
            Task.new(title="Test", unknown="value")

    def test_node_type_to_dict(self):
        """Node type can be serialized."""
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str"),),
        )

        d = User.to_dict()
        assert d["type_id"] == 1
        assert d["name"] == "User"
        assert len(d["fields"]) == 1


class TestEdgeTypeDef:
    """Tests for EdgeTypeDef."""

    def test_create_edge_type(self):
        """Edge type can be created."""
        User = NodeTypeDef(type_id=1, name="User")
        Task = NodeTypeDef(type_id=2, name="Task")

        AssignedTo = EdgeTypeDef(
            edge_id=401,
            name="AssignedTo",
            from_type=Task,
            to_type=User,
            props=(field(1, "role", "enum", enum_values=("primary", "reviewer")),),
        )

        assert AssignedTo.edge_id == 401
        assert AssignedTo.from_type_id == 2
        assert AssignedTo.to_type_id == 1

    def test_edge_type_with_int_refs(self):
        """Edge type can use int type references."""
        AssignedTo = EdgeTypeDef(
            edge_id=401,
            name="AssignedTo",
            from_type=2,
            to_type=1,
        )

        assert AssignedTo.from_type_id == 2
        assert AssignedTo.to_type_id == 1

    def test_validate_props(self):
        """Edge properties can be validated."""
        AssignedTo = EdgeTypeDef(
            edge_id=401,
            name="AssignedTo",
            from_type=1,
            to_type=2,
            props=(field(1, "role", "enum", enum_values=("primary", "reviewer")),),
        )

        is_valid, _ = AssignedTo.validate_props({"role": "primary"})
        assert is_valid

        is_valid, errors = AssignedTo.validate_props({"role": "invalid"})
        assert not is_valid
