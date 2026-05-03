"""
Unit tests for ``_field_filters_to_filter_dict`` — the wire-level
translation that turns a list of ``FieldFilter`` proto messages into the
MongoDB-style operator dict consumed by ``compile_query_filter``.

The wire format carries the operator out-of-band on
``FieldFilter.op``. Typed clients (Go SDK) populate that field
explicitly; the historical Python SDK inlines the operator into
``value`` as a Struct. Both shapes have to round-trip to the same
operator dict — these tests pin that contract so future SDK or server
changes cannot regress one client family without breaking the test.
"""

from __future__ import annotations

import pytest
from google.protobuf import json_format

from dbaas.entdb_server.api.generated import FieldFilter, FilterOp
from dbaas.entdb_server.api.grpc_server import (
    _field_filters_to_filter_dict,
)


def _value_from(py_value):
    """Build a ``google.protobuf.Value`` carrying ``py_value`` literally."""
    import json

    from google.protobuf.struct_pb2 import Value

    v = Value()
    json_format.Parse(json.dumps(py_value), v)
    return v


class TestEqualityShape:
    def test_python_sdk_scalar_equality(self):
        # Python SDK shape: Op defaults to EQ, Value is a scalar.
        filters = [FieldFilter(field="sku", value=_value_from("WIDGET-1"))]
        assert _field_filters_to_filter_dict(filters) == {"sku": "WIDGET-1"}

    def test_python_sdk_inline_operator_dict(self):
        # Python SDK shape pre-existing-fix: Op=EQ, Value carries the
        # operator dict as a Struct. This MUST keep working — there are
        # downstream callers that build filters this way.
        filters = [FieldFilter(field="price", value=_value_from({"$gte": 100}))]
        assert _field_filters_to_filter_dict(filters) == {"price": {"$gte": 100}}


class TestOperatorShape:
    @pytest.mark.parametrize(
        "wire_op, expected_key",
        [
            (FilterOp.NEQ, "$ne"),
            (FilterOp.GT, "$gt"),
            (FilterOp.GTE, "$gte"),
            (FilterOp.LT, "$lt"),
            (FilterOp.LTE, "$lte"),
            (FilterOp.CONTAINS, "$contains"),
            (FilterOp.IN, "$in"),
        ],
    )
    def test_typed_client_operator_round_trips(self, wire_op, expected_key):
        # Go SDK shape: operator is on the wire's op field, value is
        # the bare argument. The translator must rebuild the operator
        # dict the SQL compiler expects.
        filters = [FieldFilter(field="price", op=wire_op, value=_value_from(100))]
        out = _field_filters_to_filter_dict(filters)
        assert out == {"price": {expected_key: 100}}

    def test_in_operator_carries_list_value(self):
        filters = [
            FieldFilter(field="status", op=FilterOp.IN, value=_value_from(["active", "queued"]))
        ]
        assert _field_filters_to_filter_dict(filters) == {"status": {"$in": ["active", "queued"]}}

    def test_range_query_merges_two_filters_on_same_field(self):
        # ``WHERE price >= 100 AND price <= 200`` shows up as two
        # FieldFilter entries when a Go-style client sends them — the
        # translator must combine them into a single operator dict so
        # the SQL compiler doesn't lose one of the bounds.
        filters = [
            FieldFilter(field="price", op=FilterOp.GTE, value=_value_from(100)),
            FieldFilter(field="price", op=FilterOp.LTE, value=_value_from(200)),
        ]
        assert _field_filters_to_filter_dict(filters) == {"price": {"$gte": 100, "$lte": 200}}

    def test_eq_then_operator_on_same_field_merges_via_eq_key(self):
        # An EQ filter followed by an operator filter on the same
        # field must not silently drop the equality — promote it to
        # ``$eq`` so the merged dict carries both constraints.
        filters = [
            FieldFilter(field="status", value=_value_from("active")),
            FieldFilter(field="status", op=FilterOp.NEQ, value=_value_from("archived")),
        ]
        assert _field_filters_to_filter_dict(filters) == {
            "status": {"$eq": "active", "$ne": "archived"}
        }


class TestEmptyAndAbsentValues:
    def test_no_filters_returns_empty_dict(self):
        assert _field_filters_to_filter_dict([]) == {}

    def test_unset_value_field_is_none(self):
        # ``HasField("value")`` is False when the proto value oneof is
        # unset — the translator must treat that as a None spec
        # (forwarded as ``IS NULL`` by the SQL compiler).
        filters = [FieldFilter(field="status")]
        assert _field_filters_to_filter_dict(filters) == {"status": None}
