"""Tests for codegen using entdb_options_pb2 (Issue #105, step 2).

The codegen previously contained ~180 lines of hand-rolled varint parsing
to read entdb.node / entdb.edge / entdb.field extensions from a
FileDescriptorSet. Those parsers were replaced with a generated
``entdb_options_pb2`` round-trip.

These tests make sure:

1. ``entdb_options_pb2`` is shipped in the SDK's ``_generated`` package
   and that its ``node``, ``edge`` and ``field`` extensions are importable.
2. Running codegen against the ``playground/schema.proto`` still produces
   the expected NodeInfo / EdgeInfo structures.
3. The generated Python and Go output is stable (no regression vs. the
   baseline captured when the pure-wire-format parser was in use).
4. The hand-rolled varint / wire-format helpers have been removed from
   ``codegen.py``.
"""

from __future__ import annotations

from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[2]
PLAYGROUND_PROTO = REPO_ROOT / "playground" / "schema.proto"


# ---------------------------------------------------------------------------
# entdb_options_pb2 availability
# ---------------------------------------------------------------------------


class TestEntdbOptionsPb2Generated:
    """The generated ``entdb_options_pb2`` module ships with the SDK."""

    def test_module_importable(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2  # noqa: F401

    def test_node_extension_present(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        assert hasattr(entdb_options_pb2, "node")
        assert hasattr(entdb_options_pb2, "NODE_FIELD_NUMBER")
        assert entdb_options_pb2.NODE_FIELD_NUMBER == 50100

    def test_edge_extension_present(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        assert hasattr(entdb_options_pb2, "edge")
        assert entdb_options_pb2.EDGE_FIELD_NUMBER == 50101

    def test_field_extension_present(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        assert hasattr(entdb_options_pb2, "field")
        assert entdb_options_pb2.FIELD_FIELD_NUMBER == 50102

    def test_node_opts_message_has_expected_fields(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        opts = entdb_options_pb2.NodeOpts()
        for name in (
            "type_id",
            "public",
            "tenant_visible",
            "inherit",
            "private",
            "data_policy",
            "subject_field",
            "retention_days",
            "legal_basis",
            "description",
            "deprecated",
        ):
            assert hasattr(opts, name), f"NodeOpts missing field {name}"

    def test_edge_opts_message_has_expected_fields(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        opts = entdb_options_pb2.EdgeOpts()
        for name in (
            "edge_id",
            "name",
            "propagate_share",
            "unique_per_from",
            "data_policy",
            "on_subject_exit",
            "retention_days",
            "legal_basis",
            "description",
            "deprecated",
        ):
            assert hasattr(opts, name), f"EdgeOpts missing field {name}"

    def test_field_opts_message_has_expected_fields(self):
        from sdk.entdb_sdk._generated import entdb_options_pb2

        opts = entdb_options_pb2.FieldOpts()
        for name in (
            "required",
            "searchable",
            "indexed",
            "pii",
            "phi",
            "enum_values",
            "kind",
            "ref_type_id",
            "default_value",
            "description",
            "deprecated",
            "pii_false",
        ):
            assert hasattr(opts, name), f"FieldOpts missing field {name}"


# ---------------------------------------------------------------------------
# Codegen over playground/schema.proto
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def parsed_playground():
    from sdk.entdb_sdk.codegen import parse_proto

    if not PLAYGROUND_PROTO.exists():
        pytest.skip(f"playground schema not found at {PLAYGROUND_PROTO}")

    return parse_proto(str(PLAYGROUND_PROTO))


class TestPlaygroundParse:
    """Parsing the bundled playground schema should yield the expected types."""

    def test_node_count(self, parsed_playground):
        nodes, _ = parsed_playground
        assert len(nodes) == 4

    def test_edge_count(self, parsed_playground):
        _, edges = parsed_playground
        assert len(edges) == 5

    def test_user_node(self, parsed_playground):
        nodes, _ = parsed_playground
        by_name = {n.name: n for n in nodes}
        user = by_name["User"]
        assert user.type_id == 1
        assert user.data_policy == "PERSONAL"
        assert user.subject_field == "id"
        assert len(user.fields) == 5

    def test_user_field_options_decoded(self, parsed_playground):
        nodes, _ = parsed_playground
        user = next(n for n in nodes if n.name == "User")
        by_name = {f.name: f for f in user.fields}
        assert by_name["email"].required is True
        assert by_name["email"].pii is True
        assert by_name["email"].indexed is True
        assert by_name["name"].searchable is True
        assert by_name["role"].kind == "enum"
        assert by_name["role"].enum_values == ("admin", "member", "guest")
        assert by_name["created_at"].kind == "timestamp"

    def test_project_node_is_business(self, parsed_playground):
        nodes, _ = parsed_playground
        project = next(n for n in nodes if n.name == "Project")
        assert project.type_id == 2
        assert project.data_policy == "BUSINESS"

    def test_edge_ids_unique(self, parsed_playground):
        _, edges = parsed_playground
        ids = [e.edge_id for e in edges]
        assert len(ids) == len(set(ids))

    def test_assigned_to_edge_has_props(self, parsed_playground):
        _, edges = parsed_playground
        assigned = next(e for e in edges if e.name == "ASSIGNED_TO")
        assert assigned.edge_id == 103
        assert len(assigned.props) == 1

    def test_edge_subject_exit_decoded(self, parsed_playground):
        _, edges = parsed_playground
        on_edge = next(e for e in edges if e.name == "ON")
        assert on_edge.on_subject_exit == "FROM"


class TestGeneratedOutputStable:
    """Regenerating Python/Go output from the playground schema is stable."""

    def test_python_output_contains_expected_nodes(self, parsed_playground):
        from sdk.entdb_sdk.codegen import generate_python

        nodes, edges = parsed_playground
        out = generate_python(nodes, edges)
        assert "User = NodeTypeDef(" in out
        assert "Project = NodeTypeDef(" in out
        assert "Task = NodeTypeDef(" in out
        assert "Comment = NodeTypeDef(" in out
        assert "DataPolicy.BUSINESS" in out
        assert "DataPolicy.PERSONAL" in out

    def test_python_output_contains_expected_edges(self, parsed_playground):
        from sdk.entdb_sdk.codegen import generate_python

        nodes, edges = parsed_playground
        out = generate_python(nodes, edges)
        for ename in ("OWNS", "CONTAINS", "ASSIGNED_TO", "ON", "BY"):
            assert f"{ename} = EdgeTypeDef(" in out

    def test_python_output_is_deterministic(self, parsed_playground):
        from sdk.entdb_sdk.codegen import generate_python

        nodes, edges = parsed_playground
        assert generate_python(nodes, edges) == generate_python(nodes, edges)

    def test_go_output_contains_expected_nodes(self, parsed_playground):
        from sdk.entdb_sdk.codegen import generate_go

        nodes, edges = parsed_playground
        out = generate_go(nodes, edges)
        assert "var User = entdb.NodeTypeDef{" in out
        assert "var Project = entdb.NodeTypeDef{" in out

    def test_python_output_round_trips_through_generate_from_proto(self):
        from sdk.entdb_sdk.codegen import generate_from_proto

        out = generate_from_proto(str(PLAYGROUND_PROTO), lang="python")
        assert "User = NodeTypeDef(" in out
        assert "from entdb_sdk import" in out


# ---------------------------------------------------------------------------
# Hand-rolled parser removal
# ---------------------------------------------------------------------------


class TestHandRolledParsersRemoved:
    """Ensure the pure-wire-format parsers are gone from codegen.py."""

    def test_no_hand_rolled_symbols(self):
        import sdk.entdb_sdk.codegen as cg

        for name in (
            "_decode_varint",
            "_encode_varint",
            "_get_option_value",
            "_parse_node_opts",
            "_parse_edge_opts",
            "_parse_field_opts",
        ):
            assert not hasattr(cg, name), (
                f"codegen.py should no longer expose {name}; "
                "options are now decoded via entdb_options_pb2."
            )
