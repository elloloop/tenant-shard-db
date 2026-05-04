"""Schema-layer tests for composite (multi-field) unique constraints.

Covers the registration / validation surface introduced for
``(entdb.node).composite_unique``:

- ``CompositeUniqueDef`` invariants (≥2 fields, no duplicate field_ids,
  non-empty name).
- ``NodeTypeDef`` validation rejects composites that reference unknown
  field ids, duplicate constraint names, or duplicate field-id sets.
- ``SchemaRegistry.get_composite_unique_constraints`` returns
  ``(name, (field_id, ...))`` tuples for the applier to consume on
  first write.
- Proto-driven path (``register_proto_schema``) resolves field *names*
  in the proto annotation to numeric ``field_id`` values via the
  parent message descriptor.

These tests are intentionally schema-only — enforcement at apply time
is exercised by ``tests/integration/test_composite_unique_enforcement.py``.
"""

from __future__ import annotations

import pytest

from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import (
    CompositeUniqueDef,
    NodeTypeDef,
    field,
)


class TestCompositeUniqueDef:
    def test_valid_constraint_round_trips(self) -> None:
        cu = CompositeUniqueDef(name="provider_user_id", field_ids=(1, 2))
        d = cu.to_dict()
        assert d == {"name": "provider_user_id", "field_ids": [1, 2]}
        assert CompositeUniqueDef.from_dict(d) == cu

    def test_rejects_single_field(self) -> None:
        with pytest.raises(ValueError, match="at least 2 fields"):
            CompositeUniqueDef(name="solo", field_ids=(1,))

    def test_rejects_duplicate_field_ids(self) -> None:
        with pytest.raises(ValueError, match="duplicate field_id"):
            CompositeUniqueDef(name="dup", field_ids=(1, 2, 1))

    def test_rejects_empty_name(self) -> None:
        with pytest.raises(ValueError, match="name cannot be empty"):
            CompositeUniqueDef(name="", field_ids=(1, 2))


class TestNodeTypeDefCompositeValidation:
    def _node_with(self, *constraints: CompositeUniqueDef) -> NodeTypeDef:
        return NodeTypeDef(
            type_id=201,
            name="OAuthIdentity",
            fields=(
                field(1, "provider", "str"),
                field(2, "provider_user_id", "str"),
                field(3, "user_id", "str"),
            ),
            composite_unique=tuple(constraints),
        )

    def test_valid_composite_constraint_accepted(self) -> None:
        nt = self._node_with(CompositeUniqueDef(name="provider_user_id", field_ids=(1, 2)))
        assert nt.composite_unique[0].name == "provider_user_id"
        assert nt.composite_unique[0].field_ids == (1, 2)

    def test_rejects_unknown_field_id(self) -> None:
        with pytest.raises(ValueError, match="unknown field_id 99"):
            self._node_with(CompositeUniqueDef(name="bogus", field_ids=(1, 99)))

    def test_rejects_duplicate_constraint_name(self) -> None:
        with pytest.raises(ValueError, match="Duplicate composite_unique name"):
            self._node_with(
                CompositeUniqueDef(name="ck", field_ids=(1, 2)),
                CompositeUniqueDef(name="ck", field_ids=(1, 3)),
            )

    def test_rejects_duplicate_field_set(self) -> None:
        # Different names but same field-id set → still a duplicate.
        with pytest.raises(ValueError, match="already declared"):
            self._node_with(
                CompositeUniqueDef(name="a", field_ids=(1, 2)),
                CompositeUniqueDef(name="b", field_ids=(2, 1)),
            )

    def test_to_dict_round_trip_includes_composite(self) -> None:
        nt = self._node_with(CompositeUniqueDef(name="provider_user_id", field_ids=(1, 2)))
        d = nt.to_dict()
        assert d["composite_unique"] == [{"name": "provider_user_id", "field_ids": [1, 2]}]
        round_tripped = NodeTypeDef.from_dict(d)
        assert round_tripped.composite_unique == nt.composite_unique


class TestRegistryCompositeAccessor:
    def test_returns_empty_list_for_unknown_type(self) -> None:
        reg = SchemaRegistry()
        assert reg.get_composite_unique_constraints(999) == []

    def test_returns_empty_list_when_no_composites(self) -> None:
        reg = SchemaRegistry()
        reg.register_node_type(
            NodeTypeDef(
                type_id=10,
                name="Plain",
                fields=(field(1, "a", "str"),),
            )
        )
        assert reg.get_composite_unique_constraints(10) == []

    def test_returns_resolved_field_ids_for_registered_type(self) -> None:
        reg = SchemaRegistry()
        reg.register_node_type(
            NodeTypeDef(
                type_id=201,
                name="OAuthIdentity",
                fields=(
                    field(1, "provider", "str"),
                    field(2, "provider_user_id", "str"),
                ),
                composite_unique=(CompositeUniqueDef(name="provider_user_id", field_ids=(1, 2)),),
            )
        )
        assert reg.get_composite_unique_constraints(201) == [("provider_user_id", (1, 2))]


class TestProtoToNodeTypeDefBridge:
    """The SDK's ``NodeTypeDef.from_descriptor`` resolves proto field
    *names* in ``composite_unique`` to numeric ``field_id`` values.
    A typo in the .proto must fail loudly rather than silently
    dropping the constraint (CLAUDE.md "fail loudly, NOT silently
    skip").
    """

    def test_unknown_field_name_raises(self) -> None:
        from google.protobuf import descriptor_pb2

        from sdk.entdb_sdk._generated import entdb_options_pb2 as ep
        from sdk.entdb_sdk.schema import NodeTypeDef as SdkNodeTypeDef

        msg = descriptor_pb2.DescriptorProto(name="OAuthIdentity")
        f1 = msg.field.add()
        f1.name = "provider"
        f1.number = 1
        f2 = msg.field.add()
        f2.name = "provider_user_id"
        f2.number = 2

        node_opts = ep.NodeOpts(type_id=201)
        cu = node_opts.composite_unique.add()
        cu.name = "provider_user_id"
        cu.fields.append("provider")
        cu.fields.append("does_not_exist")

        with pytest.raises(ValueError, match="references unknown field"):
            SdkNodeTypeDef.from_descriptor(msg, node_opts)

    def test_resolved_constraint_round_trips(self) -> None:
        from google.protobuf import descriptor_pb2

        from sdk.entdb_sdk._generated import entdb_options_pb2 as ep
        from sdk.entdb_sdk.schema import NodeTypeDef as SdkNodeTypeDef

        msg = descriptor_pb2.DescriptorProto(name="OAuthIdentity")
        f1 = msg.field.add()
        f1.name = "provider"
        f1.number = 7
        f2 = msg.field.add()
        f2.name = "provider_user_id"
        f2.number = 9

        node_opts = ep.NodeOpts(type_id=201)
        cu = node_opts.composite_unique.add()
        cu.name = "provider_user_id"
        cu.fields.append("provider")
        cu.fields.append("provider_user_id")

        nt = SdkNodeTypeDef.from_descriptor(msg, node_opts)
        assert len(nt.composite_unique) == 1
        assert nt.composite_unique[0].name == "provider_user_id"
        # Field ids match the proto field numbers (which double as
        # ``field_id`` in the SDK).
        assert nt.composite_unique[0].field_ids == (7, 9)

    def test_omitted_name_derived_from_field_ids(self) -> None:
        from google.protobuf import descriptor_pb2

        from sdk.entdb_sdk._generated import entdb_options_pb2 as ep
        from sdk.entdb_sdk.schema import NodeTypeDef as SdkNodeTypeDef

        msg = descriptor_pb2.DescriptorProto(name="OAuthIdentity")
        f1 = msg.field.add()
        f1.name = "provider"
        f1.number = 1
        f2 = msg.field.add()
        f2.name = "provider_user_id"
        f2.number = 2

        node_opts = ep.NodeOpts(type_id=201)
        cu = node_opts.composite_unique.add()
        # Name intentionally omitted.
        cu.fields.append("provider")
        cu.fields.append("provider_user_id")

        nt = SdkNodeTypeDef.from_descriptor(msg, node_opts)
        assert nt.composite_unique[0].name == "f1_f2"
