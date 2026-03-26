"""
Unit tests for schema payload validation.

Tests cover:
- Valid payload passes validation
- Missing required fields detected
- Wrong field types detected
- Unknown fields in strict mode
- Unknown fields in lenient mode (allowed)
- Enum value validation
- No schema available (passthrough)
- Unknown type ID (passthrough)
- Empty payload handling
- List field validation
"""

from dbaas.entdb_server.schema.types import NodeTypeDef, field
from dbaas.entdb_server.schema.validator import validate_payload


class FakeRegistry:
    """Minimal registry stub for testing."""

    def __init__(self, node_types: dict[int, NodeTypeDef] | None = None):
        self._node_types = node_types or {}

    def get_node_type(self, type_id: int) -> NodeTypeDef | None:
        return self._node_types.get(type_id)


# Shared type definitions for tests
TASK_TYPE = NodeTypeDef(
    type_id=101,
    name="Task",
    fields=(
        field(1, "title", "str", required=True),
        field(2, "status", "enum", enum_values=("todo", "doing", "done")),
        field(3, "priority", "int"),
        field(4, "tags", "list_str"),
        field(5, "done", "bool"),
        field(6, "score", "float"),
    ),
)

REGISTRY = FakeRegistry({101: TASK_TYPE})


class TestValidPayload:
    def test_valid_payload_passes(self):
        """A payload matching the schema should produce no errors."""
        payload = {"title": "Fix bug", "status": "todo", "priority": 1}
        errors = validate_payload(101, payload, REGISTRY)
        assert errors == []


class TestMissingRequiredField:
    def test_missing_required_field(self):
        """Missing a required field should produce an error."""
        payload = {"status": "todo"}  # missing 'title'
        errors = validate_payload(101, payload, REGISTRY)
        assert len(errors) == 1
        assert "title" in errors[0]
        assert "missing" in errors[0].lower()


class TestWrongFieldType:
    def test_wrong_field_type(self):
        """Wrong type for a field should produce an error."""
        payload = {"title": "Fix bug", "priority": "high"}  # priority should be int
        errors = validate_payload(101, payload, REGISTRY)
        assert len(errors) == 1
        assert "priority" in errors[0]
        assert "int" in errors[0]


class TestUnknownFieldStrictMode:
    def test_unknown_field_strict_mode(self):
        """Unknown fields should be rejected in strict mode."""
        payload = {"title": "Fix bug", "extra_field": "surprise"}
        errors = validate_payload(101, payload, REGISTRY, strict=True)
        assert any("extra_field" in e for e in errors)


class TestUnknownFieldLenientMode:
    def test_unknown_field_lenient_mode(self):
        """Unknown fields should be allowed in lenient (default) mode."""
        payload = {"title": "Fix bug", "extra_field": "surprise"}
        errors = validate_payload(101, payload, REGISTRY, strict=False)
        assert not any("extra_field" in e for e in errors)


class TestEnumValidation:
    def test_enum_validation(self):
        """Invalid enum value should produce an error."""
        payload = {"title": "Fix bug", "status": "invalid_status"}
        errors = validate_payload(101, payload, REGISTRY)
        assert len(errors) == 1
        assert "status" in errors[0]
        assert "not in allowed values" in errors[0]

    def test_valid_enum_passes(self):
        """Valid enum value should pass."""
        payload = {"title": "Fix bug", "status": "doing"}
        errors = validate_payload(101, payload, REGISTRY)
        assert errors == []


class TestNoSchema:
    def test_no_schema_passes(self):
        """When no registry is provided, validation is skipped."""
        payload = {"anything": "goes"}
        errors = validate_payload(101, payload, None)
        assert errors == []

    def test_registry_without_get_node_type_passes(self):
        """When registry lacks get_node_type, validation is skipped."""
        errors = validate_payload(101, {"anything": "goes"}, object())
        assert errors == []


class TestUnknownType:
    def test_unknown_type_passes(self):
        """Unknown type_id should pass (schema-less mode)."""
        payload = {"anything": "goes"}
        errors = validate_payload(999, payload, REGISTRY)
        assert errors == []


class TestEmptyPayload:
    def test_empty_payload(self):
        """Empty payload should flag missing required fields."""
        errors = validate_payload(101, {}, REGISTRY)
        assert len(errors) == 1
        assert "title" in errors[0]

    def test_empty_payload_no_required(self):
        """Empty payload with no required fields should pass."""
        optional_type = NodeTypeDef(
            type_id=200,
            name="Optional",
            fields=(field(1, "note", "str"),),
        )
        registry = FakeRegistry({200: optional_type})
        errors = validate_payload(200, {}, registry)
        assert errors == []


class TestListFieldValidation:
    def test_list_field_validation(self):
        """List field with wrong element types should fail."""
        payload = {"title": "Fix bug", "tags": [1, 2, 3]}  # should be list of strings
        errors = validate_payload(101, payload, REGISTRY)
        assert len(errors) == 1
        assert "tags" in errors[0]

    def test_valid_list_field(self):
        """List field with correct element types should pass."""
        payload = {"title": "Fix bug", "tags": ["urgent", "backend"]}
        errors = validate_payload(101, payload, REGISTRY)
        assert errors == []

    def test_bool_field_validation(self):
        """Bool field should accept only booleans."""
        payload = {"title": "Fix bug", "done": 1}  # should be bool
        errors = validate_payload(101, payload, REGISTRY)
        assert len(errors) == 1
        assert "done" in errors[0]

    def test_float_field_accepts_int(self):
        """Float field should accept int values."""
        payload = {"title": "Fix bug", "score": 5}
        errors = validate_payload(101, payload, REGISTRY)
        assert errors == []

    def test_float_field_accepts_float(self):
        """Float field should accept float values."""
        payload = {"title": "Fix bug", "score": 3.14}
        errors = validate_payload(101, payload, REGISTRY)
        assert errors == []
