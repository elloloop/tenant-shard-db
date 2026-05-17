# SPDX-License-Identifier: MIT
"""SDK-side regression for issue #545: schema-less DeleteWhere.

A server started without a schema cannot resolve a field NAME to a
payload field id. The documented workaround is to pass the digit-only
numeric payload field id as ``Filter.field`` — exactly the
schema-optional escape hatch the query path already accepts. These
tests pin that the Python SDK forwards a numeric ``Filter.field``
VERBATIM to the wire (no client-side name resolution, no rewrite), so
the schema-less workflow keeps working end-to-end. The matching
server-side behaviour is pinned in
``tests/python/integration/test_delete_where.py`` against a live
schema-less Go server.
"""

from __future__ import annotations

from entdb_sdk._grpc_client import GrpcClient
from entdb_sdk.filter import Filter, FilterOp, filters_to_filter_dict


def test_filters_to_filter_dict_preserves_numeric_field_key() -> None:
    """A numeric field id survives the MongoDB-style lowering unchanged."""
    out = filters_to_filter_dict([Filter(field="4", op=FilterOp.LT, value=1700000000000)])
    assert out == {"4": {"$lt": 1700000000000}}


def test_delete_where_numeric_field_id_reaches_wire_verbatim() -> None:
    """delete_where with Filter(field="4") emits FieldFilter(field="4").

    The numeric id must NOT be name-resolved client-side — the
    schema-less server resolves a digit-only key to a raw payload
    field id with no schema lookup (issue #545).
    """
    client = GrpcClient(host="127.0.0.1", port=1)  # no connect()

    # "4" is the caller's own (server-unknown) expires_at field id.
    where_dict = filters_to_filter_dict([Filter(field="4", op=FilterOp.LT, value=1700000000000)])
    proto_ops = client._convert_operations(
        [{"delete_where": {"type_id": 525, "where": where_dict, "limit": 1000}}]
    )

    assert len(proto_ops) == 1
    dw = proto_ops[0].delete_where
    assert dw.type_id == 525
    assert dw.limit == 1000
    assert len(dw.where) == 1
    # The decisive assertion: the numeric id travels unchanged so a
    # schema-less server can resolve it without a schema.
    assert dw.where[0].field == "4"
    # {"$lt": v} rides through as a Value; the server's inlined-operator
    # decoder fans it into the LT comparison (issue #501).
    inner = dw.where[0].value.struct_value
    assert "$lt" in inner.fields
    assert inner.fields["$lt"].number_value == 1700000000000


def test_delete_where_field_name_also_passes_through() -> None:
    """A field NAME is still forwarded verbatim (schema-mode path).

    The SDK never resolves names client-side — a schema-configured
    server resolves the name; a schema-less server rejects it with
    INVALID_ARGUMENT. Either way the SDK's job is to pass it through.
    """
    client = GrpcClient(host="127.0.0.1", port=1)
    where_dict = filters_to_filter_dict([Filter(field="expires_at", op=FilterOp.LT, value=100)])
    proto_ops = client._convert_operations(
        [{"delete_where": {"type_id": 525, "where": where_dict, "limit": 0}}]
    )
    assert proto_ops[0].delete_where.where[0].field == "expires_at"
