"""
End-to-end tests for the schema-apply pipeline a customer follows:

  1. compile a .proto with stock protoc → ``schema_pb2.py``
  2. ``entdb-schema snapshot --module schema_pb2 -o schema.lock.json``
  3. boot the server with ``SCHEMA_FILE=/path/to/schema.lock.json``

Two contracts pinned here, both reported by a customer:

- Step 2 must accept a generated ``_pb2`` module **directly** (no
  hand-written adapter). The CLI auto-detects ``DESCRIPTOR`` and runs
  ``register_proto_schema``.

- Step 3 must accept the wrapped JSON shape ``snapshot`` produces
  (``{"version": 1, "fingerprint": "...", "schema": {...}}``) without
  an extra ``jq '.schema'`` step.

If either of these breaks, the customer's published runbook breaks.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from dbaas.entdb_server.main import Server
from dbaas.entdb_server.schema.registry import reset_registry
from dbaas.entdb_server.tools.schema_cli import _load_registry

# ── Step 2: snapshot accepts a _pb2 module directly ───────────────────


def test_snapshot_accepts_generated_pb2_module_directly():
    """Customer compiles their .proto with stock protoc, points
    ``entdb-schema snapshot --module`` at the resulting `_pb2` module,
    and gets a registry back — no adapter file required.
    """
    # tests/_test_schemas/test_schema_pb2.py is a generated proto
    # module that carries (entdb.node) / (entdb.edge) extensions
    # — same shape a customer's compiled proto has.
    sys.path.insert(0, str(Path(__file__).parent.parent / "_test_schemas"))
    try:
        reset_registry()
        registry = _load_registry("test_schema_pb2")
    finally:
        reset_registry()

    # The CLI may return either the server-side SchemaRegistry or
    # the SDK's parallel class depending on the input module shape.
    # The downstream consumer (``snapshot``) only calls
    # ``to_dict()`` on it, so verify via that contract.
    snapshot_dict = registry.to_dict()
    type_ids = {n["type_id"] for n in snapshot_dict["node_types"]}
    edge_ids = {e["edge_id"] for e in snapshot_dict["edge_types"]}
    # test_schema.proto declares Product (9001), Category (9002),
    # BelongsTo (9101).
    assert 9001 in type_ids
    assert 9002 in type_ids
    assert 9101 in edge_ids


def test_snapshot_module_without_proto_or_registry_raises():
    """A module that's neither a proto module nor exposes ``registry``
    / ``get_registry()`` must error clearly — not silently return an
    empty registry, not import any random Python module that happens
    to be on PYTHONPATH.
    """
    with pytest.raises(ValueError, match="has no 'registry'"):
        _load_registry("json")  # stdlib module — has no schema bits


# ── Step 3: server loader accepts snapshot's wrapped format ───────────


def test_server_load_accepts_snapshot_wrapped_format(tmp_path):
    """``entdb-schema snapshot`` produces a wrapped envelope. The
    server's ``_load_schema_file`` must accept it directly so the
    customer can mount the snapshot output into the container with
    ``SCHEMA_FILE=/path/to/schema.lock.json`` and no extra jq.
    """
    snapshot_envelope = {
        "version": 1,
        "fingerprint": "abc123",
        "schema": {
            "node_types": [
                {
                    "type_id": 7001,
                    "name": "Product",
                    "fields": [
                        {"id": 1, "name": "sku", "kind": "str", "unique": True},
                        {"id": 2, "name": "name", "kind": "str"},
                    ],
                },
            ],
            "edge_types": [],
        },
    }
    schema_path = tmp_path / "schema.lock.json"
    schema_path.write_text(json.dumps(snapshot_envelope))

    registry = MagicMock()
    registry.register_node_type = MagicMock()
    registry.register_edge_type = MagicMock()

    Server._load_schema_file(registry, str(schema_path))

    # Type 7001 was registered despite the wrapper envelope.
    assert registry.register_node_type.call_count == 1
    registered = registry.register_node_type.call_args[0][0]
    assert registered.type_id == 7001
    assert registered.name == "Product"


def test_server_load_still_accepts_plain_top_level_format(tmp_path):
    """The pre-v1.7 unwrapped shape must still load — existing
    deployments that hand-wrote ``schema.json`` keep working.
    """
    plain_format = {
        "node_types": [
            {
                "type_id": 7002,
                "name": "Order",
                "fields": [
                    {"id": 1, "name": "order_id", "kind": "str", "unique": True},
                ],
            },
        ],
        "edge_types": [],
    }
    schema_path = tmp_path / "schema.json"
    schema_path.write_text(json.dumps(plain_format))

    registry = MagicMock()
    registry.register_node_type = MagicMock()
    registry.register_edge_type = MagicMock()

    Server._load_schema_file(registry, str(schema_path))

    assert registry.register_node_type.call_count == 1
    registered = registry.register_node_type.call_args[0][0]
    assert registered.type_id == 7002


# ── Step 2 + 3 chained: snapshot output is bootable as-is ─────────────


def test_snapshot_output_is_directly_loadable_by_server(tmp_path):
    """The whole pipeline: compile → snapshot → server boot.
    No transformation needed between snapshot and SCHEMA_FILE.
    """
    sys.path.insert(0, str(Path(__file__).parent.parent / "_test_schemas"))
    reset_registry()
    try:
        registry = _load_registry("test_schema_pb2")

        # Mimic what `entdb-schema snapshot --output X` writes.
        from dbaas.entdb_server.tools.schema_cli import SchemaCLI

        cli = SchemaCLI()
        if registry.fingerprint is None:
            registry.freeze()
        snapshot_json = cli.snapshot(registry)

        snapshot_path = tmp_path / "schema.lock.json"
        snapshot_path.write_text(snapshot_json)
    finally:
        reset_registry()

    # Now boot the server's loader against that file. No errors.
    server_registry = MagicMock()
    server_registry.register_node_type = MagicMock()
    server_registry.register_edge_type = MagicMock()

    Server._load_schema_file(server_registry, str(snapshot_path))

    # All three test types from test_schema.proto should land in
    # the server registry.
    type_ids = {call.args[0].type_id for call in server_registry.register_node_type.call_args_list}
    assert 9001 in type_ids
    assert 9002 in type_ids
    edge_ids = {call.args[0].edge_id for call in server_registry.register_edge_type.call_args_list}
    assert 9101 in edge_ids
