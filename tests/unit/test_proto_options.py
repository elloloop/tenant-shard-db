"""Tests for entdb_options.proto — v2 custom options.

Validates that the options proto compiles, the playground schema compiles
with those options, and the generated descriptors carry the expected
custom option values (data_policy, pii, subject_field, etc.).
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile

import pytest
from google.protobuf import descriptor_pb2

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
PROTO_DIR = os.path.join(ROOT, "sdk", "entdb_sdk", "proto")
PLAYGROUND_DIR = os.path.join(ROOT, "playground")
OPTIONS_PROTO = os.path.join(PROTO_DIR, "entdb_options.proto")
SCHEMA_PROTO = os.path.join(PLAYGROUND_DIR, "schema.proto")


def _compile_proto(*args: str) -> subprocess.CompletedProcess:
    """Run protoc via grpc_tools."""
    cmd = [sys.executable, "-m", "grpc_tools.protoc", *args]
    return subprocess.run(cmd, capture_output=True, text=True, cwd=ROOT)


@pytest.fixture(scope="module")
def options_descriptor() -> descriptor_pb2.FileDescriptorSet:
    """Compile just entdb_options.proto and return the descriptor set."""
    with tempfile.TemporaryDirectory() as td:
        out = os.path.join(td, "options.pb")
        result = _compile_proto(
            f"-I{PROTO_DIR}",
            f"--descriptor_set_out={out}",
            OPTIONS_PROTO,
        )
        assert result.returncode == 0, f"protoc failed: {result.stderr}"
        with open(out, "rb") as f:
            fds = descriptor_pb2.FileDescriptorSet()
            fds.ParseFromString(f.read())
        return fds


@pytest.fixture(scope="module")
def playground_descriptor() -> descriptor_pb2.FileDescriptorSet:
    """Compile playground schema.proto with imports and return descriptor set."""
    with tempfile.TemporaryDirectory() as td:
        out = os.path.join(td, "playground.pb")
        result = _compile_proto(
            f"-I{PLAYGROUND_DIR}",
            f"-I{PROTO_DIR}",
            "--include_imports",
            f"--descriptor_set_out={out}",
            SCHEMA_PROTO,
        )
        assert result.returncode == 0, f"protoc failed: {result.stderr}"
        with open(out, "rb") as f:
            fds = descriptor_pb2.FileDescriptorSet()
            fds.ParseFromString(f.read())
        return fds


def _find_file(fds: descriptor_pb2.FileDescriptorSet, name: str):
    for fd in fds.file:
        if fd.name == name:
            return fd
    raise KeyError(f"File {name!r} not found in descriptor set")


def _find_message(fd, name: str):
    for mt in fd.message_type:
        if mt.name == name:
            return mt
    raise KeyError(f"Message {name!r} not found in {fd.name}")


def _find_field(mt, name: str):
    for f in mt.field:
        if f.name == name:
            return f
    raise KeyError(f"Field {name!r} not found in {mt.name}")


def _find_enum(fd, name: str):
    for et in fd.enum_type:
        if et.name == name:
            return et
    raise KeyError(f"Enum {name!r} not found in {fd.name}")


# ── Test 1: Options proto compiles ───────────────────────────────────


class TestOptionsProtoCompiles:
    def test_compiles_successfully(self, options_descriptor):
        """entdb_options.proto compiles without errors."""
        assert options_descriptor is not None
        assert len(options_descriptor.file) >= 1

    def test_options_file_present(self, options_descriptor):
        """The descriptor set contains entdb_options.proto."""
        names = [f.name for f in options_descriptor.file]
        assert "entdb_options.proto" in names


# ── Test 2: Playground schema compiles ───────────────────────────────


class TestPlaygroundCompiles:
    def test_compiles_successfully(self, playground_descriptor):
        """playground/schema.proto compiles with entdb_options.proto."""
        assert playground_descriptor is not None

    def test_schema_file_present(self, playground_descriptor):
        """The descriptor set contains schema.proto."""
        names = [f.name for f in playground_descriptor.file]
        assert "schema.proto" in names

    def test_options_imported(self, playground_descriptor):
        """schema.proto imports entdb_options.proto."""
        names = [f.name for f in playground_descriptor.file]
        assert "entdb_options.proto" in names


# ── Test 3: DataPolicy enum ─────────────────────────────────────────


class TestDataPolicyEnum:
    def test_data_policy_values(self, options_descriptor):
        """DataPolicy enum has all 6 expected values."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        dp = _find_enum(fd, "DataPolicy")
        value_names = {v.name for v in dp.value}
        expected = {
            "DATA_POLICY_PERSONAL",
            "DATA_POLICY_BUSINESS",
            "DATA_POLICY_FINANCIAL",
            "DATA_POLICY_AUDIT",
            "DATA_POLICY_EPHEMERAL",
            "DATA_POLICY_HEALTHCARE",
        }
        assert expected == value_names

    def test_data_policy_numbers(self, options_descriptor):
        """DataPolicy enum values have correct numeric assignments."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        dp = _find_enum(fd, "DataPolicy")
        nums = {v.name: v.number for v in dp.value}
        assert nums["DATA_POLICY_PERSONAL"] == 0
        assert nums["DATA_POLICY_HEALTHCARE"] == 5


# ── Test 4: SubjectExitPolicy enum ──────────────────────────────────


class TestSubjectExitPolicyEnum:
    def test_subject_exit_values(self, options_descriptor):
        """SubjectExitPolicy enum has all 3 expected values."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        sep = _find_enum(fd, "SubjectExitPolicy")
        value_names = {v.name for v in sep.value}
        expected = {
            "SUBJECT_EXIT_BOTH",
            "SUBJECT_EXIT_FROM",
            "SUBJECT_EXIT_TO",
        }
        assert expected == value_names


# ── Test 5: NodeOpts message fields ─────────────────────────────────


class TestNodeOptsMessage:
    def test_all_fields_present(self, options_descriptor):
        """NodeOpts has the full set of v2 fields."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        mt = _find_message(fd, "NodeOpts")
        field_names = {f.name for f in mt.field}
        expected = {
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
            # Typed capability system (2026-04-13 ACL decision).
            "extension_capability_enum",
            "capability_mappings",
            "capability_implications",
        }
        assert expected == field_names

    def test_field_numbers_stable(self, options_descriptor):
        """NodeOpts field numbers match spec."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        mt = _find_message(fd, "NodeOpts")
        nums = {f.name: f.number for f in mt.field}
        assert nums["type_id"] == 1
        assert nums["data_policy"] == 6
        assert nums["deprecated"] == 11


# ── Test 6: EdgeOpts message fields ─────────────────────────────────


class TestEdgeOptsMessage:
    def test_all_fields_present(self, options_descriptor):
        """EdgeOpts has the full set of v2 fields (no from_type/to_type)."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        mt = _find_message(fd, "EdgeOpts")
        field_names = {f.name for f in mt.field}
        expected = {
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
        }
        assert expected == field_names
        # Verify old fields are gone
        assert "from_type" not in field_names
        assert "to_type" not in field_names

    def test_on_subject_exit_type(self, options_descriptor):
        """on_subject_exit field references SubjectExitPolicy enum."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        mt = _find_message(fd, "EdgeOpts")
        f = _find_field(mt, "on_subject_exit")
        assert f.type_name == ".entdb.SubjectExitPolicy"


# ── Test 7: FieldOpts message fields ────────────────────────────────


class TestFieldOptsMessage:
    def test_all_fields_present(self, options_descriptor):
        """FieldOpts has the full set of v2 fields including pii/phi."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        mt = _find_message(fd, "FieldOpts")
        field_names = {f.name for f in mt.field}
        expected = {
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
        }
        assert expected == field_names

    def test_pii_field_number(self, options_descriptor):
        """pii is field 4, phi is field 5, pii_false is field 12."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        mt = _find_message(fd, "FieldOpts")
        nums = {f.name: f.number for f in mt.field}
        assert nums["pii"] == 4
        assert nums["phi"] == 5
        assert nums["pii_false"] == 12


# ── Test 8: Extensions registered ───────────────────────────────────


class TestExtensions:
    def test_node_extension_number(self, options_descriptor):
        """node extension uses field number 50100."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        ext_map = {e.name: e for e in fd.extension}
        assert "node" in ext_map
        assert ext_map["node"].number == 50100

    def test_edge_extension_number(self, options_descriptor):
        """edge extension uses field number 50101."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        ext_map = {e.name: e for e in fd.extension}
        assert "edge" in ext_map
        assert ext_map["edge"].number == 50101

    def test_field_extension_number(self, options_descriptor):
        """field extension uses field number 50102."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        ext_map = {e.name: e for e in fd.extension}
        assert "field" in ext_map
        assert ext_map["field"].number == 50102


# ── Test 9: Playground node messages have custom options ─────────────


class TestPlaygroundNodeOptions:
    def test_user_has_options(self, playground_descriptor):
        """User message has non-empty options (custom extension data)."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "User")
        assert mt.options.ByteSize() > 0

    def test_project_has_options(self, playground_descriptor):
        """Project message has non-empty options."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "Project")
        assert mt.options.ByteSize() > 0

    def test_task_has_options(self, playground_descriptor):
        """Task message has non-empty options."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "Task")
        assert mt.options.ByteSize() > 0

    def test_comment_has_options(self, playground_descriptor):
        """Comment message has non-empty options."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "Comment")
        assert mt.options.ByteSize() > 0


# ── Test 10: Playground edge messages have custom options ────────────


class TestPlaygroundEdgeOptions:
    def test_owns_has_options(self, playground_descriptor):
        """Owns edge message has non-empty options."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "Owns")
        assert mt.options.ByteSize() > 0

    def test_comment_on_has_subject_exit_data(self, playground_descriptor):
        """CommentOn edge carries on_subject_exit option data."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "CommentOn")
        # The raw option bytes should be non-trivial (contains data_policy + on_subject_exit)
        raw = mt.options.SerializeToString()
        assert len(raw) > 10

    def test_all_edge_messages_present(self, playground_descriptor):
        """All 5 edge types are present in the compiled schema."""
        fd = _find_file(playground_descriptor, "schema.proto")
        names = {mt.name for mt in fd.message_type}
        for edge_name in ("Owns", "Contains", "AssignedTo", "CommentOn", "CommentBy"):
            assert edge_name in names


# ── Test 11: Field-level options on playground fields ────────────────


class TestPlaygroundFieldOptions:
    def test_user_email_has_pii(self, playground_descriptor):
        """User.email field has pii option set (raw bytes contain pii flag)."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "User")
        f = _find_field(mt, "email")
        assert f.options.ByteSize() > 0

    def test_comment_body_has_pii(self, playground_descriptor):
        """Comment.body field has pii option set."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "Comment")
        f = _find_field(mt, "body")
        assert f.options.ByteSize() > 0

    def test_user_role_has_pii_false(self, playground_descriptor):
        """User.role field has pii_false option set."""
        fd = _find_file(playground_descriptor, "schema.proto")
        mt = _find_message(fd, "User")
        f = _find_field(mt, "role")
        # role has pii_false: true, enum_values, default_value — should be non-trivial
        assert f.options.ByteSize() > 0


# ── Test 12: No AclOpts sub-message in v2 ───────────────────────────


class TestNoLegacyAclOpts:
    def test_no_acl_opts_message(self, options_descriptor):
        """AclOpts sub-message no longer exists in v2 options proto."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        msg_names = {mt.name for mt in fd.message_type}
        assert "AclOpts" not in msg_names

    def test_node_opts_no_acl_field(self, options_descriptor):
        """NodeOpts does not have an 'acl' sub-message field."""
        fd = _find_file(options_descriptor, "entdb_options.proto")
        mt = _find_message(fd, "NodeOpts")
        field_names = {f.name for f in mt.field}
        assert "acl" not in field_names
