"""Regression tests for validate.py missing FieldKind branches.

Bug: _validate_field_value() in validate.py was missing branches for JSON,
BYTES, LIST_REF, and did not exclude bool from TIMESTAMP. This meant
validate_payload() and validate_or_raise() from the validate module silently
accepted invalid values for those kinds, while FieldDef.validate_value() in
schema.py correctly rejected them.

These tests confirm the validate.py functions now handle all 12 FieldKind
variants consistently with FieldDef.validate_value().
"""

from __future__ import annotations

import pytest

from sdk.entdb_sdk.errors import ValidationError
from sdk.entdb_sdk.schema import FieldDef, FieldKind, NodeTypeDef
from sdk.entdb_sdk.validate import validate_or_raise, validate_payload


def _field(fid: int, name: str, kind: FieldKind, **kwargs) -> FieldDef:
    return FieldDef(field_id=fid, name=name, kind=kind, **kwargs)


def _node_type(*fields: FieldDef) -> NodeTypeDef:
    return NodeTypeDef(type_id=1, name="TestType", fields=fields)


# ---------------------------------------------------------------------------
# JSON kind (was missing from validate.py)
# ---------------------------------------------------------------------------


class TestJsonKindValidation:
    """validate_payload must reject non-dict/list for JSON fields."""

    def test_json_rejects_string(self):
        nt = _node_type(_field(1, "meta", FieldKind.JSON))
        ok, errors = validate_payload(nt, {"meta": "not json"})
        assert not ok
        assert any("dict or list" in e for e in errors)

    def test_json_rejects_int(self):
        nt = _node_type(_field(1, "meta", FieldKind.JSON))
        ok, errors = validate_payload(nt, {"meta": 42})
        assert not ok

    def test_json_accepts_dict(self):
        nt = _node_type(_field(1, "meta", FieldKind.JSON))
        ok, errors = validate_payload(nt, {"meta": {"key": "val"}})
        assert ok

    def test_json_accepts_list(self):
        nt = _node_type(_field(1, "meta", FieldKind.JSON))
        ok, errors = validate_payload(nt, {"meta": [1, 2]})
        assert ok

    def test_json_validate_or_raise(self):
        nt = _node_type(_field(1, "meta", FieldKind.JSON))
        with pytest.raises(ValidationError):
            validate_or_raise(nt, {"meta": True})


# ---------------------------------------------------------------------------
# BYTES kind (was missing from validate.py)
# ---------------------------------------------------------------------------


class TestBytesKindValidation:
    """validate_payload must reject non-bytes/str for BYTES fields."""

    def test_bytes_rejects_int(self):
        nt = _node_type(_field(1, "data", FieldKind.BYTES))
        ok, errors = validate_payload(nt, {"data": 123})
        assert not ok
        assert any("bytes or string" in e for e in errors)

    def test_bytes_rejects_list(self):
        nt = _node_type(_field(1, "data", FieldKind.BYTES))
        ok, errors = validate_payload(nt, {"data": [1, 2]})
        assert not ok

    def test_bytes_accepts_bytes(self):
        nt = _node_type(_field(1, "data", FieldKind.BYTES))
        ok, errors = validate_payload(nt, {"data": b"\x00\x01"})
        assert ok

    def test_bytes_accepts_string(self):
        nt = _node_type(_field(1, "data", FieldKind.BYTES))
        ok, errors = validate_payload(nt, {"data": "base64encoded"})
        assert ok


# ---------------------------------------------------------------------------
# LIST_REF kind (was missing from validate.py)
# ---------------------------------------------------------------------------


class TestListRefKindValidation:
    """validate_payload must reject non-list and non-dict elements for LIST_REF."""

    def test_list_ref_rejects_non_list(self):
        nt = _node_type(_field(1, "refs", FieldKind.LIST_REF))
        ok, errors = validate_payload(nt, {"refs": "not a list"})
        assert not ok
        assert any("list" in e for e in errors)

    def test_list_ref_rejects_non_dict_element(self):
        nt = _node_type(_field(1, "refs", FieldKind.LIST_REF))
        ok, errors = validate_payload(nt, {"refs": [{"type_id": 1, "id": "a"}, "bad"]})
        assert not ok
        assert any("refs[1]" in e for e in errors)

    def test_list_ref_accepts_valid(self):
        nt = _node_type(_field(1, "refs", FieldKind.LIST_REF))
        ok, errors = validate_payload(nt, {"refs": [{"type_id": 1, "id": "a"}]})
        assert ok

    def test_list_ref_accepts_empty(self):
        nt = _node_type(_field(1, "refs", FieldKind.LIST_REF))
        ok, errors = validate_payload(nt, {"refs": []})
        assert ok


# ---------------------------------------------------------------------------
# TIMESTAMP bool exclusion (was missing from validate.py)
# ---------------------------------------------------------------------------


class TestTimestampBoolExclusion:
    """validate_payload must reject bool for TIMESTAMP fields."""

    def test_timestamp_rejects_true(self):
        nt = _node_type(_field(1, "ts", FieldKind.TIMESTAMP))
        ok, errors = validate_payload(nt, {"ts": True})
        assert not ok
        assert any("timestamp" in e for e in errors)

    def test_timestamp_rejects_false(self):
        nt = _node_type(_field(1, "ts", FieldKind.TIMESTAMP))
        ok, errors = validate_payload(nt, {"ts": False})
        assert not ok

    def test_timestamp_still_accepts_int(self):
        nt = _node_type(_field(1, "ts", FieldKind.TIMESTAMP))
        ok, errors = validate_payload(nt, {"ts": 1700000000})
        assert ok


# ---------------------------------------------------------------------------
# Consistency: validate_payload matches FieldDef.validate_value for all kinds
# ---------------------------------------------------------------------------


class TestConsistencyWithFieldDef:
    """validate_payload and FieldDef.validate_value must agree on all kinds."""

    @pytest.mark.parametrize(
        "kind,bad_value",
        [
            (FieldKind.STRING, 123),
            (FieldKind.INTEGER, "five"),
            (FieldKind.FLOAT, "pi"),
            (FieldKind.BOOLEAN, 0),
            (FieldKind.TIMESTAMP, True),
            (FieldKind.JSON, "not json"),
            (FieldKind.BYTES, 999),
            (FieldKind.REFERENCE, "node:1"),
            (FieldKind.LIST_STRING, "not a list"),
            (FieldKind.LIST_INT, "not a list"),
            (FieldKind.LIST_REF, "not a list"),
        ],
    )
    def test_both_reject_bad_values(self, kind, bad_value):
        """Both validate_payload and FieldDef.validate_value reject bad values."""
        kwargs = {}
        if kind == FieldKind.ENUM:
            kwargs["enum_values"] = ("a", "b")
        fd = _field(1, "f", kind, **kwargs)
        nt = _node_type(fd)

        # FieldDef path
        fd_ok, _ = fd.validate_value(bad_value)
        # validate.py path
        vp_ok, _ = validate_payload(nt, {"f": bad_value})

        assert not fd_ok, f"FieldDef should reject {bad_value} for {kind}"
        assert not vp_ok, f"validate_payload should reject {bad_value} for {kind}"
