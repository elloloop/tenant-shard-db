# SPDX-License-Identifier: MIT
"""Python SDK regression for Bug C (#563): the typed payload path
(ADR-028) must round-trip int64 > 2^53 losslessly, unlike the legacy
google.protobuf.Struct path whose numbers are IEEE-754 doubles."""

from __future__ import annotations

import pytest

from entdb_sdk._generated import (
    CreateNodeOp,
    FieldFilter,
    GetNodeByKeyRequest,
    UpdateNodePrecondition,
)
from entdb_sdk._grpc_client import (
    _entvalue_to_python,
    _populate_typed,
    _typed_to_dict,
    _value_to_entvalue,
)

INT64_SPECTRUM = [
    0,
    1,
    -1,
    (1 << 53) + 1,  # 9007199254740993 — first unsafe odd int
    10_000_000_000_000_001,  # 10^16 + 1
    (1 << 62) + 1,
    (1 << 63) - 1,  # MaxInt64
    -(1 << 63),  # MinInt64
    -((1 << 53) + 1),
]


@pytest.mark.parametrize("want", INT64_SPECTRUM)
def test_value_entvalue_int64_roundtrip(want: int) -> None:
    got = _entvalue_to_python(_value_to_entvalue(want))
    assert isinstance(got, int), f"expected int, got {type(got)}"
    assert got == want


def test_populate_and_read_typed_map_preserves_types() -> None:
    big = (1 << 53) + 1
    op = CreateNodeOp()
    _populate_typed(op.typed_data, {"3": big, "1": "hello", "2": 3.5, "4": True})
    out = _typed_to_dict(op.typed_data)
    assert out["3"] == big and isinstance(out["3"], int)
    assert out["1"] == "hello"
    assert out["2"] == 3.5
    assert out["4"] is True


def test_non_digit_keys_skipped() -> None:
    op = CreateNodeOp()
    _populate_typed(op.typed_data, {"email": "x", "5": 9})
    out = _typed_to_dict(op.typed_data)
    assert out == {"5": 9}


# ── #572: scalar-value surfaces (filter / unique-key / CAS) ──────────
# The legacy google.protobuf.Struct Value carries numbers as float64,
# so a big-int filter / unique-key / CAS predicate would silently
# mismatch the stored int64. These pin the typed-scalar wire encode the
# SDK now dual-writes alongside the legacy Value.


@pytest.mark.parametrize("want", INT64_SPECTRUM)
def test_field_filter_typed_value_int64_roundtrip(want: int) -> None:
    ff = FieldFilter(field="price_cents", typed_value=_value_to_entvalue(want))
    got = _entvalue_to_python(ff.typed_value)
    assert isinstance(got, int) and got == want


@pytest.mark.parametrize("want", INT64_SPECTRUM)
def test_get_node_by_key_typed_value_int64_roundtrip(want: int) -> None:
    req = GetNodeByKeyRequest(field_id=1, typed_value=_value_to_entvalue(want))
    got = _entvalue_to_python(req.typed_value)
    assert isinstance(got, int) and got == want


@pytest.mark.parametrize("want", INT64_SPECTRUM)
def test_precondition_typed_equals_int64_roundtrip(want: int) -> None:
    pre = UpdateNodePrecondition(field_id=3, typed_equals=_value_to_entvalue(want))
    got = _entvalue_to_python(pre.typed_equals)
    assert isinstance(got, int) and got == want
