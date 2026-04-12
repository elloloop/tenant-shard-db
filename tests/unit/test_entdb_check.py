"""Unit tests for entdb check — backward compatibility checker.

Tests the _check_compat function and CLI commands (init, check)
for all breaking and non-breaking change rules.
"""

from __future__ import annotations

import copy
import json

from sdk.entdb_sdk.cli import _check_compat

# ── Helpers ──────────────────────────────────────────────────────────────


def _snapshot(node_types=None, edge_types=None, version=2):
    """Build a minimal snapshot dict."""
    return {
        "version": version,
        "source": "test.proto",
        "node_types": node_types or [],
        "edge_types": edge_types or [],
    }


def _node(type_id, name, fields=None, **kwargs):
    """Build a node type dict for snapshots."""
    n = {
        "type_id": type_id,
        "name": name,
        "deprecated": False,
        "description": "",
        "data_policy": "PERSONAL",
        "fields": fields or [],
    }
    n.update(kwargs)
    return n


def _field(field_id, name, kind="str", **kwargs):
    """Build a field dict for snapshots."""
    f = {
        "field_id": field_id,
        "name": name,
        "kind": kind,
        "required": False,
        "pii": False,
        "enum_values": None,
        "default_value": None,
        "deprecated": False,
        "description": "",
    }
    f.update(kwargs)
    return f


def _edge(edge_id, name, **kwargs):
    """Build an edge type dict for snapshots."""
    e = {
        "edge_id": edge_id,
        "name": name,
        "from_type": 0,
        "to_type": 0,
        "propagate_share": False,
        "data_policy": "PERSONAL",
        "deprecated": False,
        "description": "",
    }
    e.update(kwargs)
    return e


def _codes(issues):
    """Extract just the codes from issues list."""
    return [i["code"] for i in issues]


def _breaking(issues):
    return [i for i in issues if i["breaking"]]


def _non_breaking(issues):
    return [i for i in issues if not i["breaking"]]


# ── Breaking change tests ────────────────────────────────────────────────


class TestBreakingChanges:
    """Each breaking change rule produces an error."""

    def test_node_type_removed(self):
        """Rule 1: Removing a node type is breaking."""
        old = _snapshot(node_types=[_node(1, "User"), _node(2, "Task")])
        new = _snapshot(node_types=[_node(1, "User")])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert len(breaking) == 1
        assert breaking[0]["code"] == "NODE_REMOVED"
        assert "Task" in breaking[0]["message"]

    def test_edge_type_removed(self):
        """Rule 2: Removing an edge type is breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS"), _edge(102, "CONTAINS")])
        new = _snapshot(edge_types=[_edge(101, "OWNS")])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert len(breaking) == 1
        assert breaking[0]["code"] == "EDGE_REMOVED"
        assert "CONTAINS" in breaking[0]["message"]

    def test_field_removed(self):
        """Rule 3: Removing a field from a node type is breaking."""
        old = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email"),
                        _field(2, "name"),
                    ],
                )
            ]
        )
        new = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email"),
                    ],
                )
            ]
        )

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "FIELD_REMOVED" for i in breaking)
        assert any("User.name" in i["message"] for i in breaking)

    def test_field_kind_changed(self):
        """Rule 4: Changing field kind/type is breaking."""
        old = _snapshot(node_types=[_node(1, "User", fields=[_field(1, "age", "str")])])
        new = _snapshot(node_types=[_node(1, "User", fields=[_field(1, "age", "int")])])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "FIELD_KIND_CHANGED" for i in breaking)
        assert any("'str'" in i["message"] and "'int'" in i["message"] for i in breaking)

    def test_type_id_changed(self):
        """Rule 5: type_id changed on a node (same name, different ID) is breaking."""
        old = _snapshot(node_types=[_node(1, "User")])
        new = _snapshot(node_types=[_node(99, "User")])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "TYPE_ID_CHANGED" for i in breaking)
        assert any("type_id changed from 1 to 99" in i["message"] for i in breaking)

    def test_edge_id_changed(self):
        """Rule 6: edge_id changed on an edge is breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS")])
        new = _snapshot(edge_types=[_edge(999, "OWNS")])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "EDGE_ID_CHANGED" for i in breaking)
        assert any("edge_id changed from 101 to 999" in i["message"] for i in breaking)

    def test_required_field_added_without_default(self):
        """Rule 7: Required field added without default is breaking."""
        old = _snapshot(node_types=[_node(1, "User", fields=[_field(1, "email")])])
        new = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email"),
                        _field(2, "name", required=True),
                    ],
                )
            ]
        )

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "REQUIRED_FIELD_ADDED" for i in breaking)
        assert any("required without default" in i["message"] for i in breaking)

    def test_required_field_added_with_default_is_ok(self):
        """Rule 7 exception: Required + default is non-breaking."""
        old = _snapshot(node_types=[_node(1, "User", fields=[_field(1, "email")])])
        new = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email"),
                        _field(2, "role", required=True, default_value="member"),
                    ],
                )
            ]
        )

        issues = _check_compat(old, new)

        assert len(_breaking(issues)) == 0
        assert any(i["code"] == "FIELD_ADDED" for i in _non_breaking(issues))

    def test_enum_value_removed(self):
        """Rule 8: Removing an enum value is breaking."""
        old = _snapshot(
            node_types=[
                _node(
                    1,
                    "Task",
                    fields=[_field(1, "status", "enum", enum_values=["todo", "doing", "done"])],
                )
            ]
        )
        new = _snapshot(
            node_types=[
                _node(1, "Task", fields=[_field(1, "status", "enum", enum_values=["todo", "done"])])
            ]
        )

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "ENUM_VALUE_REMOVED" for i in breaking)
        assert any("'doing'" in i["message"] for i in breaking)

    def test_propagate_share_changed(self):
        """Rule 9: propagate_share changed is breaking."""
        old = _snapshot(edge_types=[_edge(101, "CONTAINS", propagate_share=True)])
        new = _snapshot(edge_types=[_edge(101, "CONTAINS", propagate_share=False)])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "PROPAGATE_SHARE_CHANGED" for i in breaking)

    def test_from_type_changed(self):
        """Rule 10: from_type changed on an edge is breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS", from_type=1, to_type=2)])
        new = _snapshot(edge_types=[_edge(101, "OWNS", from_type=3, to_type=2)])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "FROM_TYPE_CHANGED" for i in breaking)

    def test_to_type_changed(self):
        """Rule 10: to_type changed on an edge is breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS", from_type=1, to_type=2)])
        new = _snapshot(edge_types=[_edge(101, "OWNS", from_type=1, to_type=5)])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "TO_TYPE_CHANGED" for i in breaking)

    def test_data_policy_more_restrictive_node(self):
        """Rule 11: data_policy more restrictive on node is breaking."""
        old = _snapshot(node_types=[_node(1, "User", data_policy="BUSINESS")])
        new = _snapshot(node_types=[_node(1, "User", data_policy="FINANCIAL")])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "DATA_POLICY_MORE_RESTRICTIVE" for i in breaking)

    def test_data_policy_more_restrictive_edge(self):
        """Rule 11: data_policy more restrictive on edge is breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS", data_policy="EPHEMERAL")])
        new = _snapshot(edge_types=[_edge(101, "OWNS", data_policy="PERSONAL")])

        issues = _check_compat(old, new)

        breaking = _breaking(issues)
        assert any(i["code"] == "DATA_POLICY_MORE_RESTRICTIVE" for i in breaking)


# ── Non-breaking change tests ────────────────────────────────────────────


class TestNonBreakingChanges:
    """Each non-breaking change rule produces info."""

    def test_node_type_added(self):
        """Rule 12: New node type added is non-breaking."""
        old = _snapshot(node_types=[_node(1, "User")])
        new = _snapshot(node_types=[_node(1, "User"), _node(2, "Task")])

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "NODE_ADDED" for i in non_breaking)
        assert any("Task" in i["message"] for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_edge_type_added(self):
        """Rule 13: New edge type added is non-breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS")])
        new = _snapshot(edge_types=[_edge(101, "OWNS"), _edge(102, "CONTAINS")])

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "EDGE_ADDED" for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_optional_field_added(self):
        """Rule 14: New optional field added is non-breaking."""
        old = _snapshot(node_types=[_node(1, "User", fields=[_field(1, "email")])])
        new = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email"),
                        _field(2, "avatar_url"),
                    ],
                )
            ]
        )

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "FIELD_ADDED" for i in non_breaking)
        assert any("avatar_url" in i["message"] for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_enum_value_added(self):
        """Rule 15: New enum value added is non-breaking."""
        old = _snapshot(
            node_types=[
                _node(1, "Task", fields=[_field(1, "status", "enum", enum_values=["todo", "done"])])
            ]
        )
        new = _snapshot(
            node_types=[
                _node(
                    1,
                    "Task",
                    fields=[_field(1, "status", "enum", enum_values=["todo", "doing", "done"])],
                )
            ]
        )

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "ENUM_VALUE_ADDED" for i in non_breaking)
        assert any("'doing'" in i["message"] for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_description_changed_node(self):
        """Rule 16: Description changed on node is non-breaking."""
        old = _snapshot(node_types=[_node(1, "User", description="old desc")])
        new = _snapshot(node_types=[_node(1, "User", description="new desc")])

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "DESCRIPTION_CHANGED" for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_description_changed_field(self):
        """Rule 16: Description changed on field is non-breaking."""
        old = _snapshot(
            node_types=[_node(1, "User", fields=[_field(1, "email", description="old")])]
        )
        new = _snapshot(
            node_types=[_node(1, "User", fields=[_field(1, "email", description="new")])]
        )

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "DESCRIPTION_CHANGED" for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_description_changed_edge(self):
        """Rule 16: Description changed on edge is non-breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS", description="old")])
        new = _snapshot(edge_types=[_edge(101, "OWNS", description="new")])

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "DESCRIPTION_CHANGED" for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_node_deprecated(self):
        """Rule 17: Node deprecated is non-breaking."""
        old = _snapshot(node_types=[_node(1, "User", deprecated=False)])
        new = _snapshot(node_types=[_node(1, "User", deprecated=True)])

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "NODE_DEPRECATED" for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_data_policy_less_restrictive_node(self):
        """Rule 19: data_policy less restrictive on node is non-breaking."""
        old = _snapshot(node_types=[_node(1, "User", data_policy="FINANCIAL")])
        new = _snapshot(node_types=[_node(1, "User", data_policy="BUSINESS")])

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "DATA_POLICY_LESS_RESTRICTIVE" for i in non_breaking)
        assert len(_breaking(issues)) == 0

    def test_data_policy_less_restrictive_edge(self):
        """Rule 19: data_policy less restrictive on edge is non-breaking."""
        old = _snapshot(edge_types=[_edge(101, "OWNS", data_policy="PERSONAL")])
        new = _snapshot(edge_types=[_edge(101, "OWNS", data_policy="EPHEMERAL")])

        issues = _check_compat(old, new)

        non_breaking = _non_breaking(issues)
        assert any(i["code"] == "DATA_POLICY_LESS_RESTRICTIVE" for i in non_breaking)
        assert len(_breaking(issues)) == 0


# ── Edge cases and combined tests ────────────────────────────────────────


class TestEdgeCases:
    """Tests for edge cases: no changes, multiple changes, round-trips."""

    def test_no_changes(self):
        """Identical schemas produce no issues."""
        snap = _snapshot(
            node_types=[_node(1, "User", fields=[_field(1, "email")])],
            edge_types=[_edge(101, "OWNS")],
        )
        issues = _check_compat(snap, copy.deepcopy(snap))
        assert issues == []

    def test_empty_schemas(self):
        """Two empty schemas produce no issues."""
        issues = _check_compat(_snapshot(), _snapshot())
        assert issues == []

    def test_multiple_breaking_changes(self):
        """Multiple breaking changes in one check."""
        old = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email", "str"),
                        _field(2, "name", "str"),
                    ],
                ),
                _node(2, "Task"),
            ],
            edge_types=[_edge(101, "OWNS", propagate_share=True)],
        )
        new = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email", "int"),  # kind changed
                        # field 2 removed
                    ],
                ),
                # Task removed
            ],
            edge_types=[_edge(101, "OWNS", propagate_share=False)],  # propagate changed
        )

        issues = _check_compat(old, new)
        breaking = _breaking(issues)

        codes = {i["code"] for i in breaking}
        assert "NODE_REMOVED" in codes
        assert "FIELD_REMOVED" in codes
        assert "FIELD_KIND_CHANGED" in codes
        assert "PROPAGATE_SHARE_CHANGED" in codes
        assert len(breaking) >= 4

    def test_multiple_non_breaking_changes(self):
        """Multiple non-breaking changes in one check."""
        old = _snapshot(
            node_types=[_node(1, "User", description="old")],
            edge_types=[_edge(101, "OWNS")],
        )
        new = _snapshot(
            node_types=[
                _node(1, "User", description="new", deprecated=True),
                _node(2, "Task"),
            ],
            edge_types=[
                _edge(101, "OWNS"),
                _edge(102, "CONTAINS"),
            ],
        )

        issues = _check_compat(old, new)

        assert len(_breaking(issues)) == 0
        codes = {i["code"] for i in _non_breaking(issues)}
        assert "NODE_ADDED" in codes
        assert "EDGE_ADDED" in codes
        assert "DESCRIPTION_CHANGED" in codes
        assert "NODE_DEPRECATED" in codes

    def test_snapshot_round_trip(self):
        """init then check with same data = 0 changes."""
        snap = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email", "str", required=True, pii=True),
                        _field(2, "name", "str"),
                    ],
                    data_policy="PERSONAL",
                    description="A user",
                ),
                _node(
                    2,
                    "Task",
                    fields=[
                        _field(1, "title", "str", required=True),
                        _field(2, "status", "enum", enum_values=["todo", "done"]),
                    ],
                    data_policy="BUSINESS",
                ),
            ],
            edge_types=[
                _edge(101, "OWNS", from_type=1, to_type=2, propagate_share=False),
                _edge(102, "CONTAINS", propagate_share=True, data_policy="BUSINESS"),
            ],
        )

        # Round-trip through JSON (mimics file write/read)
        serialized = json.dumps(snap, indent=2)
        reloaded = json.loads(serialized)

        issues = _check_compat(snap, reloaded)
        assert issues == []

    def test_mixed_breaking_and_non_breaking(self):
        """Both breaking and non-breaking in the same check."""
        old = _snapshot(
            node_types=[
                _node(1, "User", fields=[_field(1, "email", "str")]),
            ],
        )
        new = _snapshot(
            node_types=[
                _node(
                    1,
                    "User",
                    fields=[
                        _field(1, "email", "int"),  # breaking: kind changed
                        _field(2, "avatar", "str"),  # non-breaking: field added
                    ],
                ),
            ],
        )

        issues = _check_compat(old, new)

        assert len(_breaking(issues)) >= 1
        assert len(_non_breaking(issues)) >= 1
        assert any(i["code"] == "FIELD_KIND_CHANGED" for i in _breaking(issues))
        assert any(i["code"] == "FIELD_ADDED" for i in _non_breaking(issues))

    def test_all_issues_have_required_keys(self):
        """Every issue dict has breaking, code, and message keys."""
        old = _snapshot(node_types=[_node(1, "User")])
        new = _snapshot(node_types=[_node(1, "User"), _node(2, "Task")])

        issues = _check_compat(old, new)

        for issue in issues:
            assert "breaking" in issue
            assert "code" in issue
            assert "message" in issue
            assert isinstance(issue["breaking"], bool)
            assert isinstance(issue["code"], str)
            assert isinstance(issue["message"], str)


# ── CLI exit code tests ──────────────────────────────────────────────────


class TestCLIExitCodes:
    """Test that CLI functions return correct exit codes."""

    def test_cmd_check_returns_0_when_compatible(self, tmp_path, monkeypatch):
        """Exit code 0 when no breaking changes."""
        import argparse

        from entdb_sdk.cli import cmd_check

        # Write a baseline snapshot
        snapshot_path = tmp_path / "snapshot.json"
        snap = _snapshot(node_types=[_node(1, "User", fields=[_field(1, "email")])])
        snapshot_path.write_text(json.dumps(snap))

        # Monkeypatch parse_proto to return matching data

        def mock_parse_proto(path, include_dirs=None):
            from entdb_sdk.codegen import FieldInfo, NodeInfo

            return (
                [
                    NodeInfo(
                        type_id=1,
                        name="User",
                        fields=[
                            FieldInfo(field_id=1, name="email", kind="str"),
                            FieldInfo(field_id=2, name="avatar", kind="str"),
                        ],
                    )
                ],
                [],
            )

        monkeypatch.setattr("entdb_sdk.cli.parse_proto", mock_parse_proto)

        # Create a dummy proto file
        proto_file = tmp_path / "schema.proto"
        proto_file.write_text("syntax = 'proto3';")

        args = argparse.Namespace(
            proto=str(proto_file),
            baseline=str(snapshot_path),
            include=None,
        )
        result = cmd_check(args)
        assert result == 0

    def test_cmd_check_returns_1_when_breaking(self, tmp_path, monkeypatch):
        """Exit code 1 when breaking changes detected."""
        import argparse

        from entdb_sdk.cli import cmd_check

        snapshot_path = tmp_path / "snapshot.json"
        snap = _snapshot(
            node_types=[_node(1, "User", fields=[_field(1, "email"), _field(2, "name")])]
        )
        snapshot_path.write_text(json.dumps(snap))

        from entdb_sdk.codegen import FieldInfo, NodeInfo

        def mock_parse_proto(path, include_dirs=None):
            return (
                [
                    NodeInfo(
                        type_id=1,
                        name="User",
                        fields=[
                            FieldInfo(field_id=1, name="email", kind="str"),
                            # field 2 removed — breaking
                        ],
                    )
                ],
                [],
            )

        monkeypatch.setattr("entdb_sdk.cli.parse_proto", mock_parse_proto)

        proto_file = tmp_path / "schema.proto"
        proto_file.write_text("syntax = 'proto3';")

        args = argparse.Namespace(
            proto=str(proto_file),
            baseline=str(snapshot_path),
            include=None,
        )
        result = cmd_check(args)
        assert result == 1

    def test_cmd_check_returns_0_when_unchanged(self, tmp_path, monkeypatch):
        """Exit code 0 when schema is unchanged."""
        import argparse

        from entdb_sdk.cli import cmd_check

        snapshot_path = tmp_path / "snapshot.json"
        snap = _snapshot(node_types=[_node(1, "User", fields=[_field(1, "email")])])
        snapshot_path.write_text(json.dumps(snap))

        from entdb_sdk.codegen import FieldInfo, NodeInfo

        def mock_parse_proto(path, include_dirs=None):
            return (
                [
                    NodeInfo(
                        type_id=1,
                        name="User",
                        fields=[
                            FieldInfo(field_id=1, name="email", kind="str"),
                        ],
                    )
                ],
                [],
            )

        monkeypatch.setattr("entdb_sdk.cli.parse_proto", mock_parse_proto)

        proto_file = tmp_path / "schema.proto"
        proto_file.write_text("syntax = 'proto3';")

        args = argparse.Namespace(
            proto=str(proto_file),
            baseline=str(snapshot_path),
            include=None,
        )
        result = cmd_check(args)
        assert result == 0

    def test_cmd_check_missing_baseline(self, tmp_path, capsys):
        """Exit code 1 when baseline file is missing."""
        import argparse

        from entdb_sdk.cli import cmd_check

        proto_file = tmp_path / "schema.proto"
        proto_file.write_text("syntax = 'proto3';")

        args = argparse.Namespace(
            proto=str(proto_file),
            baseline=str(tmp_path / "nonexistent.json"),
            include=None,
        )
        result = cmd_check(args)
        assert result == 1

    def test_cmd_init_creates_snapshot(self, tmp_path, monkeypatch):
        """entdb init creates .entdb/snapshot.json."""
        import argparse

        from entdb_sdk.cli import cmd_init

        monkeypatch.chdir(tmp_path)

        from entdb_sdk.codegen import EdgeInfo, FieldInfo, NodeInfo

        def mock_parse_proto(path, include_dirs=None):
            return (
                [
                    NodeInfo(
                        type_id=1,
                        name="User",
                        fields=[
                            FieldInfo(
                                field_id=1, name="email", kind="str", required=True, pii=True
                            ),
                        ],
                    )
                ],
                [EdgeInfo(edge_id=101, name="OWNS", from_type=1, to_type=2, props=[])],
            )

        monkeypatch.setattr("entdb_sdk.cli.parse_proto", mock_parse_proto)

        proto_file = tmp_path / "schema.proto"
        proto_file.write_text("syntax = 'proto3';")

        args = argparse.Namespace(
            proto=str(proto_file),
            include=None,
            force=False,
        )
        result = cmd_init(args)
        assert result == 0

        snapshot_path = tmp_path / ".entdb" / "snapshot.json"
        assert snapshot_path.exists()

        snap = json.loads(snapshot_path.read_text())
        assert snap["version"] == 2
        assert len(snap["node_types"]) == 1
        assert len(snap["edge_types"]) == 1
        assert snap["node_types"][0]["name"] == "User"

    def test_cmd_init_then_check_zero_changes(self, tmp_path, monkeypatch):
        """init followed by check with same schema = 0 changes."""
        import argparse

        from entdb_sdk.cli import cmd_check, cmd_init

        monkeypatch.chdir(tmp_path)

        from entdb_sdk.codegen import EdgeInfo, FieldInfo, NodeInfo

        def mock_parse_proto(path, include_dirs=None):
            return (
                [
                    NodeInfo(
                        type_id=1,
                        name="User",
                        fields=[
                            FieldInfo(field_id=1, name="email", kind="str", required=True),
                        ],
                    ),
                    NodeInfo(
                        type_id=2,
                        name="Task",
                        fields=[
                            FieldInfo(field_id=1, name="title", kind="str"),
                        ],
                    ),
                ],
                [EdgeInfo(edge_id=101, name="OWNS", from_type=1, to_type=2, props=[])],
            )

        monkeypatch.setattr("entdb_sdk.cli.parse_proto", mock_parse_proto)

        proto_file = tmp_path / "schema.proto"
        proto_file.write_text("syntax = 'proto3';")

        # Init
        init_args = argparse.Namespace(
            proto=str(proto_file),
            include=None,
            force=False,
        )
        assert cmd_init(init_args) == 0

        # Check — should be unchanged
        check_args = argparse.Namespace(
            proto=str(proto_file),
            baseline=str(tmp_path / ".entdb" / "snapshot.json"),
            include=None,
        )
        assert cmd_check(check_args) == 0
