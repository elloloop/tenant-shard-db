"""
Unit tests for SDK payload validation.

Tests cover:
- Field type validation
- Required field checking
- Unknown field detection with suggestions
- Enum validation
"""

import pytest

from sdk.entdb_sdk.errors import UnknownFieldError, ValidationError
from sdk.entdb_sdk.schema import FieldDef, FieldKind, NodeTypeDef
from sdk.entdb_sdk.validate import (
    suggest_fields,
    validate_or_raise,
    validate_payload,
)


def field(id: int, name: str, kind: str, **kwargs) -> FieldDef:
    """Helper to create field definitions."""
    kind_map = {
        "str": FieldKind.STRING,
        "int": FieldKind.INTEGER,
        "float": FieldKind.FLOAT,
        "bool": FieldKind.BOOLEAN,
        "timestamp": FieldKind.TIMESTAMP,
        "enum": FieldKind.ENUM,
        "list_str": FieldKind.LIST_STRING,
        "list_int": FieldKind.LIST_INT,
        "ref": FieldKind.REFERENCE,
    }
    return FieldDef(field_id=id, name=name, kind=kind_map[kind], **kwargs)


class TestPayloadValidation:
    """Tests for validate_payload."""

    @pytest.fixture
    def user_type(self):
        """User node type for testing."""
        return NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str", required=True),
                field(2, "name", "str"),
                field(3, "age", "int"),
                field(4, "score", "float"),
                field(5, "active", "bool"),
                field(6, "created_at", "timestamp"),
                field(7, "status", "enum", enum_values=("active", "inactive", "pending")),
                field(8, "tags", "list_str"),
                field(9, "scores", "list_int"),
            ),
        )

    def test_valid_payload(self, user_type):
        """Valid payload passes validation."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "name": "Test User",
            },
        )
        assert is_valid
        assert len(errors) == 0

    def test_required_field_missing(self, user_type):
        """Missing required field fails."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "name": "Test User",
            },
        )
        assert not is_valid
        assert any("email" in e and "required" in e for e in errors)

    def test_unknown_field_detected(self, user_type):
        """Unknown field is reported."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "unknown_field": "value",
            },
        )
        assert not is_valid
        assert any("unknown_field" in e.lower() for e in errors)

    def test_unknown_field_suggests_similar(self, user_type):
        """Unknown field suggests similar names."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "naem": "Test",  # Typo of "name"
            },
        )
        assert not is_valid
        # Should suggest "name"
        assert any("name" in e for e in errors)

    def test_string_type_validation(self, user_type):
        """String field rejects non-string."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": 123,  # Should be string
            },
        )
        assert not is_valid
        assert any("string" in e for e in errors)

    def test_integer_type_validation(self, user_type):
        """Integer field rejects non-integer."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "age": "twenty",  # Should be int
            },
        )
        assert not is_valid
        assert any("integer" in e for e in errors)

    def test_integer_rejects_bool(self, user_type):
        """Integer field rejects boolean (isinstance check)."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "age": True,  # bool is subclass of int
            },
        )
        assert not is_valid

    def test_float_accepts_int(self, user_type):
        """Float field accepts integer."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "score": 100,  # int is valid for float
            },
        )
        assert is_valid

    def test_boolean_type_validation(self, user_type):
        """Boolean field rejects non-boolean."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "active": "yes",  # Should be bool
            },
        )
        assert not is_valid
        assert any("boolean" in e for e in errors)

    def test_timestamp_validation(self, user_type):
        """Timestamp requires positive integer."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "created_at": -1,  # Should be positive
            },
        )
        assert not is_valid
        assert any("timestamp" in e for e in errors)

    def test_enum_validation(self, user_type):
        """Enum field rejects invalid value."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "status": "invalid",  # Not in enum values
            },
        )
        assert not is_valid
        assert any("active" in e for e in errors)  # Should show valid values

    def test_enum_accepts_valid(self, user_type):
        """Enum field accepts valid value."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "status": "pending",
            },
        )
        assert is_valid

    def test_list_string_validation(self, user_type):
        """List string field validates elements."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "tags": ["valid", 123, "also valid"],  # 123 is not string
            },
        )
        assert not is_valid
        assert any("tags" in e for e in errors)

    def test_list_string_accepts_valid(self, user_type):
        """List string accepts string list."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "tags": ["tag1", "tag2", "tag3"],
            },
        )
        assert is_valid

    def test_list_int_validation(self, user_type):
        """List int field validates elements."""
        is_valid, errors = validate_payload(
            user_type,
            {
                "email": "test@example.com",
                "scores": [100, "ninety", 80],  # "ninety" is not int
            },
        )
        assert not is_valid


class TestValidateOrRaise:
    """Tests for validate_or_raise."""

    @pytest.fixture
    def user_type(self):
        """User node type for testing."""
        return NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str", required=True),
                field(2, "name", "str"),
            ),
        )

    def test_valid_payload_no_raise(self, user_type):
        """Valid payload doesn't raise."""
        validate_or_raise(user_type, {"email": "test@example.com"})

    def test_unknown_field_raises_specific_error(self, user_type):
        """Unknown field raises UnknownFieldError."""
        with pytest.raises(UnknownFieldError) as exc_info:
            validate_or_raise(
                user_type,
                {
                    "email": "test@example.com",
                    "unknown": "value",
                },
            )
        assert exc_info.value.field_name == "unknown"
        assert exc_info.value.type_name == "User"

    def test_validation_failure_raises(self, user_type):
        """Validation failure raises ValidationError."""
        with pytest.raises(ValidationError) as exc_info:
            validate_or_raise(user_type, {"name": "Test"})  # missing email
        assert len(exc_info.value.errors) > 0


class TestSuggestFields:
    """Tests for field suggestion."""

    @pytest.fixture
    def user_type(self):
        """User node type for testing."""
        return NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str"),
                field(2, "email_verified", "bool"),
                field(3, "name", "str"),
                field(4, "username", "str"),
                field(5, "legacy_field", "str", deprecated=True),
            ),
        )

    def test_exact_prefix_match(self, user_type):
        """Suggests fields starting with prefix."""
        suggestions = suggest_fields("email", user_type)
        assert "email" in suggestions
        assert "email_verified" in suggestions

    def test_fuzzy_match(self, user_type):
        """Suggests similar fields for typos."""
        suggestions = suggest_fields("naem", user_type)
        assert "name" in suggestions

    def test_excludes_deprecated(self, user_type):
        """Doesn't suggest deprecated fields."""
        suggestions = suggest_fields("legacy", user_type)
        assert "legacy_field" not in suggestions

    def test_respects_limit(self, user_type):
        """Respects suggestion limit."""
        suggestions = suggest_fields("e", user_type, limit=2)
        assert len(suggestions) <= 2
