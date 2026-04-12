"""Tests for validate.py → FieldDef.validate_value delegation (Issue #105, step 1).

The old validate.py had its own ``_validate_field_value`` that duplicated
``FieldDef.validate_value`` in schema.py. The two drifted apart and the
validate.py copy was missing JSON, BYTES, LIST_REF branches as well as
bool exclusion for TIMESTAMP.

We deleted the duplicate and route all per-field type checking through
``FieldDef.validate_value``. These tests document the contract so a
regression that re-introduces a parallel implementation is caught.
"""

from __future__ import annotations

import pytest

from sdk.entdb_sdk import validate as validate_mod
from sdk.entdb_sdk.errors import ValidationError
from sdk.entdb_sdk.schema import FieldDef, FieldKind, NodeTypeDef
from sdk.entdb_sdk.validate import validate_or_raise, validate_payload


def _fd(fid: int, name: str, kind: FieldKind, **kwargs) -> FieldDef:
    return FieldDef(field_id=fid, name=name, kind=kind, **kwargs)


def _nt(*fields: FieldDef) -> NodeTypeDef:
    return NodeTypeDef(type_id=1, name="Test", fields=fields)


# ---------------------------------------------------------------------------
# Duplicate symbol is gone
# ---------------------------------------------------------------------------


class TestDuplicateRemoved:
    """_validate_field_value should no longer exist in validate.py."""

    def test_private_helper_removed(self):
        assert not hasattr(validate_mod, "_validate_field_value"), (
            "validate.py should no longer define a parallel field-value validator; "
            "it must delegate to FieldDef.validate_value."
        )

    def test_public_api_preserved(self):
        assert hasattr(validate_mod, "validate_payload")
        assert hasattr(validate_mod, "validate_or_raise")
        assert hasattr(validate_mod, "suggest_fields")


# ---------------------------------------------------------------------------
# validate_payload now matches FieldDef on every FieldKind
# ---------------------------------------------------------------------------


ALL_KIND_CASES: list[tuple[FieldKind, object, bool, dict]] = [
    # STRING
    (FieldKind.STRING, "hi", True, {}),
    (FieldKind.STRING, 1, False, {}),
    # INTEGER
    (FieldKind.INTEGER, 42, True, {}),
    (FieldKind.INTEGER, True, False, {}),  # bool excluded
    (FieldKind.INTEGER, "42", False, {}),
    # FLOAT
    (FieldKind.FLOAT, 1.5, True, {}),
    (FieldKind.FLOAT, 2, True, {}),
    (FieldKind.FLOAT, False, False, {}),
    # BOOLEAN
    (FieldKind.BOOLEAN, True, True, {}),
    (FieldKind.BOOLEAN, 0, False, {}),
    # TIMESTAMP
    (FieldKind.TIMESTAMP, 1_700_000_000, True, {}),
    (FieldKind.TIMESTAMP, True, False, {}),
    (FieldKind.TIMESTAMP, -1, False, {}),
    # JSON (was missing from validate.py)
    (FieldKind.JSON, {"a": 1}, True, {}),
    (FieldKind.JSON, [1, 2], True, {}),
    (FieldKind.JSON, "string", False, {}),
    # BYTES (was missing)
    (FieldKind.BYTES, b"\x01", True, {}),
    (FieldKind.BYTES, "base64", True, {}),
    (FieldKind.BYTES, 123, False, {}),
    # ENUM
    (FieldKind.ENUM, "a", True, {"enum_values": ("a", "b")}),
    (FieldKind.ENUM, "c", False, {"enum_values": ("a", "b")}),
    # REFERENCE
    (FieldKind.REFERENCE, {"type_id": 1, "id": "x"}, True, {}),
    (FieldKind.REFERENCE, {"type_id": 1}, False, {}),
    (FieldKind.REFERENCE, "node", False, {}),
    # LIST_STRING
    (FieldKind.LIST_STRING, ["a", "b"], True, {}),
    (FieldKind.LIST_STRING, ["a", 1], False, {}),
    # LIST_INT
    (FieldKind.LIST_INT, [1, 2], True, {}),
    (FieldKind.LIST_INT, [1, "2"], False, {}),
    # LIST_REF (was missing)
    (FieldKind.LIST_REF, [{"type_id": 1, "id": "a"}], True, {}),
    (FieldKind.LIST_REF, "not a list", False, {}),
]


@pytest.mark.parametrize("kind,value,expect_ok,kwargs", ALL_KIND_CASES)
def test_validate_payload_matches_field_def(kind, value, expect_ok, kwargs):
    """For every kind, validate_payload and FieldDef.validate_value agree."""
    fd = _fd(1, "f", kind, **kwargs)
    nt = _nt(fd)

    fd_ok, _ = fd.validate_value(value)
    vp_ok, _ = validate_payload(nt, {"f": value})

    assert fd_ok == expect_ok
    assert vp_ok == expect_ok
    assert fd_ok == vp_ok, "validate_payload must mirror FieldDef.validate_value"


# ---------------------------------------------------------------------------
# Unknown fields still get friendly suggestions
# ---------------------------------------------------------------------------


class TestUnknownFieldSuggestions:
    def test_suggestion_present_for_typo(self):
        nt = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                _fd(1, "email", FieldKind.STRING),
                _fd(2, "name", FieldKind.STRING),
            ),
        )
        ok, errors = validate_payload(nt, {"email": "x@x", "naem": "typo"})
        assert not ok
        assert any("name" in e for e in errors)

    def test_validate_or_raise_uses_unknown_field_error(self):
        from sdk.entdb_sdk.errors import UnknownFieldError

        nt = NodeTypeDef(type_id=1, name="User", fields=(_fd(1, "email", FieldKind.STRING),))
        with pytest.raises(UnknownFieldError):
            validate_or_raise(nt, {"email": "x@x", "unknown": "v"})


# ---------------------------------------------------------------------------
# Required checks still flow through
# ---------------------------------------------------------------------------


class TestRequiredFieldDelegation:
    def test_missing_required_is_caught(self):
        nt = _nt(_fd(1, "email", FieldKind.STRING, required=True))
        ok, errors = validate_payload(nt, {})
        assert not ok
        assert any("required" in e for e in errors)

    def test_validate_or_raise_raises_on_missing_required(self):
        nt = _nt(_fd(1, "email", FieldKind.STRING, required=True))
        with pytest.raises(ValidationError):
            validate_or_raise(nt, {})


# ---------------------------------------------------------------------------
# Default values are still consulted through FieldDef.validate_value
# ---------------------------------------------------------------------------


class TestDefaultValueHandling:
    def test_default_satisfies_required(self):
        nt = _nt(_fd(1, "status", FieldKind.STRING, required=True, default="active"))
        ok, errors = validate_payload(nt, {})
        assert ok, errors

    def test_default_does_not_shadow_invalid_override(self):
        nt = _nt(_fd(1, "status", FieldKind.STRING, default="active"))
        ok, errors = validate_payload(nt, {"status": 123})
        assert not ok
        assert any("status" in e for e in errors)
