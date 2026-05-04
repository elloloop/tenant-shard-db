"""
Unit tests for field-ID based payload translation.

Covers the helpers in
``dbaas.entdb_server.schema.field_id_translation`` which translate
between name-keyed (client/SDK ingress) and id-keyed (on-disk and
on-the-wire) payloads, including:

- ``looks_id_keyed``
- ``name_to_id_keys``
- ``id_to_name_keys``
- ``translate_payload_json_to_names``
- ``translate_filter_name_to_id``
- round-trip + field-rename safety properties

Note: as of PR-D both storage and the gRPC wire are id-keyed; the
only translation point is the **write ingress** boundary where the
server accepts name-keyed input from clients and converts to ids
before persisting. Egress now ships id-keyed payloads to clients,
which translate to names locally via their own schema registry.
"""

from __future__ import annotations

import json

from dbaas.entdb_server.schema.field_id_translation import (
    id_to_name_keys,
    looks_id_keyed,
    name_to_id_keys,
    translate_filter_name_to_id,
    translate_payload_json_to_names,
)
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import NodeTypeDef, field


def _make_task_registry() -> SchemaRegistry:
    """Build a tiny registry with one Task type for reuse in tests."""
    registry = SchemaRegistry()
    registry.register_node_type(
        NodeTypeDef(
            type_id=101,
            name="Task",
            fields=(
                field(1, "title", "str"),
                field(2, "status", "str"),
                field(3, "priority", "int"),
            ),
        )
    )
    return registry


# ---------------------------------------------------------------------------
# looks_id_keyed
# ---------------------------------------------------------------------------


class TestLooksIdKeyed:
    def test_empty_dict_is_id_keyed(self):
        assert looks_id_keyed({}) is True

    def test_all_digit_keys(self):
        assert looks_id_keyed({"1": "a", "2": "b", "42": "c"}) is True

    def test_name_keys_not_id_keyed(self):
        assert looks_id_keyed({"title": "a", "status": "b"}) is False

    def test_mixed_keys_not_id_keyed(self):
        assert looks_id_keyed({"1": "a", "title": "b"}) is False

    def test_empty_string_key_not_id_keyed(self):
        assert looks_id_keyed({"": "a"}) is False

    def test_non_string_key_not_id_keyed(self):
        # Integer keys look numeric but the helper demands strings.
        assert looks_id_keyed({1: "a"}) is False

    def test_digit_with_leading_space_not_id_keyed(self):
        assert looks_id_keyed({" 1": "a"}) is False


# ---------------------------------------------------------------------------
# name_to_id_keys
# ---------------------------------------------------------------------------


class TestNameToIdKeys:
    def test_happy_path(self):
        registry = _make_task_registry()
        out = name_to_id_keys({"title": "x", "status": "open"}, 101, registry)
        assert out == {"1": "x", "2": "open"}

    def test_unknown_field_dropped(self):
        registry = _make_task_registry()
        out = name_to_id_keys({"title": "x", "bogus": "drop-me", "status": "open"}, 101, registry)
        assert out == {"1": "x", "2": "open"}
        assert "bogus" not in out

    def test_none_payload_returns_empty(self):
        registry = _make_task_registry()
        assert name_to_id_keys(None, 101, registry) == {}

    def test_empty_payload_returns_empty(self):
        registry = _make_task_registry()
        assert name_to_id_keys({}, 101, registry) == {}

    def test_already_id_keyed_known_id_passthrough(self):
        registry = _make_task_registry()
        # Caller already sent "1" — it's a registered field id, leave it alone.
        out = name_to_id_keys({"1": "x"}, 101, registry)
        assert out == {"1": "x"}

    def test_already_id_keyed_unknown_id_dropped(self):
        registry = _make_task_registry()
        # "99" is a digit string but not a registered field id — drop it.
        out = name_to_id_keys({"99": "x"}, 101, registry)
        assert out == {}

    def test_registry_none_passthrough(self):
        out = name_to_id_keys({"title": "x"}, 101, None)
        assert out == {"title": "x"}

    def test_registry_without_get_node_type_passthrough(self):
        class NotARegistry:
            pass

        out = name_to_id_keys({"title": "x"}, 101, NotARegistry())
        assert out == {"title": "x"}

    def test_type_not_found_passthrough(self):
        registry = _make_task_registry()
        out = name_to_id_keys({"title": "x"}, 999, registry)
        assert out == {"title": "x"}

    def test_all_unknown_drops_to_empty(self):
        registry = _make_task_registry()
        out = name_to_id_keys({"a": 1, "b": 2}, 101, registry)
        assert out == {}

    def test_preserves_value_types(self):
        registry = _make_task_registry()
        out = name_to_id_keys({"title": None, "status": ["a"], "priority": 3}, 101, registry)
        assert out == {"1": None, "2": ["a"], "3": 3}


# ---------------------------------------------------------------------------
# id_to_name_keys
# ---------------------------------------------------------------------------


class TestIdToNameKeys:
    def test_happy_path(self):
        registry = _make_task_registry()
        out = id_to_name_keys({"1": "x", "2": "open"}, 101, registry)
        assert out == {"title": "x", "status": "open"}

    def test_unknown_id_preserved_verbatim(self):
        registry = _make_task_registry()
        out = id_to_name_keys({"1": "x", "999": "future"}, 101, registry)
        assert out == {"title": "x", "999": "future"}

    def test_none_payload_returns_empty(self):
        registry = _make_task_registry()
        assert id_to_name_keys(None, 101, registry) == {}

    def test_empty_payload_returns_empty(self):
        registry = _make_task_registry()
        assert id_to_name_keys({}, 101, registry) == {}

    def test_registry_none_passthrough(self):
        out = id_to_name_keys({"1": "x"}, 101, None)
        assert out == {"1": "x"}

    def test_type_not_found_passthrough(self):
        registry = _make_task_registry()
        out = id_to_name_keys({"1": "x"}, 999, registry)
        assert out == {"1": "x"}

    def test_legacy_name_key_fallthrough(self):
        # A legacy name-keyed row (not yet migrated) should be handed back
        # as-is so the read path still works.
        registry = _make_task_registry()
        out = id_to_name_keys({"title": "x", "2": "open"}, 101, registry)
        assert out == {"title": "x", "status": "open"}


# ---------------------------------------------------------------------------
# translate_payload_json_to_names
# ---------------------------------------------------------------------------


class TestTranslatePayloadJsonToNames:
    def test_empty_string_returns_empty_object(self):
        registry = _make_task_registry()
        assert translate_payload_json_to_names("", 101, registry) == "{}"

    def test_none_returns_empty_object(self):
        registry = _make_task_registry()
        assert translate_payload_json_to_names(None, 101, registry) == "{}"

    def test_empty_object_literal_returns_empty_object(self):
        registry = _make_task_registry()
        assert translate_payload_json_to_names("{}", 101, registry) == "{}"

    def test_malformed_json_returned_unchanged(self):
        registry = _make_task_registry()
        bad = "{not json"
        assert translate_payload_json_to_names(bad, 101, registry) == bad

    def test_non_dict_json_returned_unchanged(self):
        registry = _make_task_registry()
        arr = "[1, 2, 3]"
        assert translate_payload_json_to_names(arr, 101, registry) == arr

    def test_happy_path_roundtrip(self):
        registry = _make_task_registry()
        raw = json.dumps({"1": "hello", "2": "open"})
        out = translate_payload_json_to_names(raw, 101, registry)
        assert json.loads(out) == {"title": "hello", "status": "open"}

    def test_unknown_id_preserved_in_json(self):
        registry = _make_task_registry()
        raw = json.dumps({"1": "x", "77": "future"})
        out = translate_payload_json_to_names(raw, 101, registry)
        assert json.loads(out) == {"title": "x", "77": "future"}


# ---------------------------------------------------------------------------
# translate_filter_name_to_id
# ---------------------------------------------------------------------------


class TestTranslateFilterNameToId:
    def test_none_returns_none(self):
        registry = _make_task_registry()
        assert translate_filter_name_to_id(None, 101, registry) is None

    def test_happy_path(self):
        registry = _make_task_registry()
        out = translate_filter_name_to_id({"status": "open"}, 101, registry)
        assert out == {"2": "open"}

    def test_unknown_name_dropped(self):
        registry = _make_task_registry()
        out = translate_filter_name_to_id({"status": "open", "bogus": "x"}, 101, registry)
        assert out == {"2": "open"}

    def test_empty_filter_returns_empty_dict(self):
        registry = _make_task_registry()
        out = translate_filter_name_to_id({}, 101, registry)
        assert out == {}


# ---------------------------------------------------------------------------
# Round-trip + field-rename safety
# ---------------------------------------------------------------------------


class TestRoundTripAndRename:
    def test_roundtrip_known_fields(self):
        registry = _make_task_registry()
        original = {"title": "x", "status": "open", "priority": 5}
        ided = name_to_id_keys(original, 101, registry)
        back = id_to_name_keys(ided, 101, registry)
        assert back == original

    def test_field_rename_safety(self):
        # Initial schema: field_id=1 is called "title".
        reg_v1 = SchemaRegistry()
        reg_v1.register_node_type(
            NodeTypeDef(
                type_id=101,
                name="Task",
                fields=(field(1, "title", "str"),),
            )
        )

        # Ingress under v1: "title" -> "1" on disk.
        stored = name_to_id_keys({"title": "x"}, 101, reg_v1)
        assert stored == {"1": "x"}

        # Schema evolves: same field_id=1 but now named "heading".
        reg_v2 = SchemaRegistry()
        reg_v2.register_node_type(
            NodeTypeDef(
                type_id=101,
                name="Task",
                fields=(field(1, "heading", "str"),),
            )
        )

        # The on-disk row is unchanged; read under v2 surfaces the new name.
        out = id_to_name_keys(stored, 101, reg_v2)
        assert out == {"heading": "x"}

        # And the JSON egress path behaves the same way.
        out_json = translate_payload_json_to_names(json.dumps(stored), 101, reg_v2)
        assert json.loads(out_json) == {"heading": "x"}
