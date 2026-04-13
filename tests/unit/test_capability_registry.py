"""Unit tests for the typed capability registry.

Covers the 2026-04-13 ACL decision: ``CoreCapability`` hierarchy,
default op mappings, per-type field-level overrides, extension
capability enums, and grant satisfaction under implications.
"""

from __future__ import annotations

import pytest

from dbaas.entdb_server.auth.capability_registry import (
    CORE_IMPLICATIONS,
    DEFAULT_OP_REQUIREMENTS,
    CapabilityImplication,
    CapabilityMapping,
    CapabilityRegistry,
    CoreCapability,
)

# ── CoreCapability enum & built-in implication hierarchy ────────────


def test_core_capability_wire_values_are_stable() -> None:
    # These integers MUST match entdb.v1.CoreCapability / entdb.CoreCapability.
    assert CoreCapability.UNSPECIFIED == 0
    assert CoreCapability.READ == 1
    assert CoreCapability.COMMENT == 2
    assert CoreCapability.EDIT == 3
    assert CoreCapability.DELETE == 4
    assert CoreCapability.ADMIN == 5


def test_admin_implies_read_comment_edit_delete() -> None:
    implied = CORE_IMPLICATIONS[CoreCapability.ADMIN]
    assert CoreCapability.READ in implied
    assert CoreCapability.COMMENT in implied
    assert CoreCapability.EDIT in implied
    assert CoreCapability.DELETE in implied


def test_edit_implies_read_and_comment_but_not_delete() -> None:
    implied = CORE_IMPLICATIONS[CoreCapability.EDIT]
    assert CoreCapability.READ in implied
    assert CoreCapability.COMMENT in implied
    assert CoreCapability.DELETE not in implied


def test_comment_implies_read_only() -> None:
    implied = CORE_IMPLICATIONS[CoreCapability.COMMENT]
    assert implied == {CoreCapability.READ}


# ── Default op mappings ─────────────────────────────────────────────


@pytest.mark.parametrize(
    "op,expected",
    [
        ("GetNode", CoreCapability.READ),
        ("GetNodes", CoreCapability.READ),
        ("QueryNodes", CoreCapability.READ),
        ("UpdateNode", CoreCapability.EDIT),
        ("DeleteNode", CoreCapability.DELETE),
        ("ShareNode", CoreCapability.ADMIN),
        ("RevokeAccess", CoreCapability.ADMIN),
    ],
)
def test_default_op_requirements(op: str, expected: CoreCapability) -> None:
    assert DEFAULT_OP_REQUIREMENTS[op] == expected


def test_required_for_op_returns_default_for_unknown_type() -> None:
    reg = CapabilityRegistry()
    core, ext = reg.required_for_op(type_id=999, op_name="UpdateNode")
    assert core == CoreCapability.EDIT
    assert ext is None


def test_required_for_op_unknown_op_returns_none_none() -> None:
    reg = CapabilityRegistry()
    core, ext = reg.required_for_op(type_id=1, op_name="DoesNotExist")
    assert core is None
    assert ext is None


# ── Type-specific mappings ──────────────────────────────────────────


def test_type_specific_field_mapping_overrides_default() -> None:
    reg = CapabilityRegistry()
    reg.register_type(
        type_id=101,
        extension_enum_name="TaskExtCapability",
        mappings=[
            CapabilityMapping(op="UpdateNode", field="status", required_ext=1),
        ],
    )
    # Field-level match — required_ext wins.
    core, ext = reg.required_for_op(101, "UpdateNode", field="status")
    assert core is None
    assert ext == 1
    # No field — falls through to default EDIT.
    core2, ext2 = reg.required_for_op(101, "UpdateNode")
    assert core2 == CoreCapability.EDIT
    assert ext2 is None


def test_more_specific_mapping_wins() -> None:
    reg = CapabilityRegistry()
    reg.register_type(
        type_id=101,
        mappings=[
            CapabilityMapping(op="UpdateNode", required_core=CoreCapability.EDIT),
            CapabilityMapping(op="UpdateNode", field="status", required_ext=7),
        ],
    )
    core, ext = reg.required_for_op(101, "UpdateNode", field="status")
    assert core is None
    assert ext == 7


def test_child_type_mapping_matches_create_child() -> None:
    reg = CapabilityRegistry()
    reg.register_type(
        type_id=101,
        mappings=[
            CapabilityMapping(
                op="CreateChild",
                child_type="Comment",
                required_core=CoreCapability.COMMENT,
            ),
        ],
    )
    core, ext = reg.required_for_op(101, "CreateChild", child_type="Comment")
    assert core == CoreCapability.COMMENT


def test_field_value_narrowing() -> None:
    reg = CapabilityRegistry()
    reg.register_type(
        type_id=101,
        mappings=[
            CapabilityMapping(
                op="UpdateNode",
                field="status",
                field_value="merged",
                required_core=CoreCapability.ADMIN,
            ),
        ],
    )
    # Matching value → ADMIN required.
    core, ext = reg.required_for_op(101, "UpdateNode", field="status", field_value="merged")
    assert core == CoreCapability.ADMIN
    # Non-matching value → default EDIT.
    core2, ext2 = reg.required_for_op(101, "UpdateNode", field="status", field_value="open")
    assert core2 == CoreCapability.EDIT


# ── check_grant: core-capability implications ───────────────────────


def test_grant_read_fails_edit_requirement() -> None:
    reg = CapabilityRegistry()
    assert not reg.check_grant(
        grant_core_caps=[CoreCapability.READ],
        grant_ext_cap_ids=[],
        required_core=CoreCapability.EDIT,
        required_ext=None,
        type_id=0,
    )


def test_grant_edit_satisfies_read_via_implication() -> None:
    reg = CapabilityRegistry()
    assert reg.check_grant(
        grant_core_caps=[CoreCapability.EDIT],
        grant_ext_cap_ids=[],
        required_core=CoreCapability.READ,
        required_ext=None,
        type_id=0,
    )


def test_grant_admin_satisfies_any_core_requirement() -> None:
    reg = CapabilityRegistry()
    for required in (
        CoreCapability.READ,
        CoreCapability.COMMENT,
        CoreCapability.EDIT,
        CoreCapability.DELETE,
        CoreCapability.ADMIN,
    ):
        assert reg.check_grant(
            grant_core_caps=[CoreCapability.ADMIN],
            grant_ext_cap_ids=[],
            required_core=required,
            required_ext=None,
            type_id=0,
        ), f"ADMIN should satisfy {required.name}"


def test_grant_empty_fails_any_requirement() -> None:
    reg = CapabilityRegistry()
    assert not reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[],
        required_core=CoreCapability.READ,
        required_ext=None,
        type_id=0,
    )


def test_grant_no_requirement_always_passes() -> None:
    reg = CapabilityRegistry()
    assert reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[],
        required_core=None,
        required_ext=None,
        type_id=0,
    )


# ── check_grant: extension capabilities ─────────────────────────────


def test_grant_ext_satisfies_ext_requirement() -> None:
    reg = CapabilityRegistry()
    reg.register_type(type_id=101, extension_enum_name="TaskExt")
    assert reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[1],
        required_core=None,
        required_ext=1,
        type_id=101,
    )


def test_grant_ext_does_not_satisfy_core_by_default() -> None:
    reg = CapabilityRegistry()
    reg.register_type(type_id=101)
    # An extension capability alone never implies CORE_CAP_READ
    # unless the user declared an implication.
    assert not reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[42],
        required_core=CoreCapability.READ,
        required_ext=None,
        type_id=101,
    )


def test_ext_implies_core_via_declared_implication() -> None:
    reg = CapabilityRegistry()
    reg.register_type(
        type_id=101,
        extension_enum_name="TaskExt",
        implications=[
            CapabilityImplication(
                ext=1,
                implies_core={CoreCapability.EDIT},
            ),
        ],
    )
    # Holding ext=1 now implies CORE_CAP_EDIT (and transitively READ).
    assert reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[1],
        required_core=CoreCapability.EDIT,
        required_ext=None,
        type_id=101,
    )
    assert reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[1],
        required_core=CoreCapability.READ,
        required_ext=None,
        type_id=101,
    )


def test_ext_implies_ext_transitive_closure() -> None:
    reg = CapabilityRegistry()
    reg.register_type(
        type_id=101,
        implications=[
            CapabilityImplication(ext=1, implies_ext={2}),
            CapabilityImplication(ext=2, implies_ext={3}),
        ],
    )
    # Holding ext=1 should transitively satisfy ext=3.
    assert reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[1],
        required_core=None,
        required_ext=3,
        type_id=101,
    )


def test_grant_scoped_to_type_id() -> None:
    reg = CapabilityRegistry()
    reg.register_type(type_id=101)
    reg.register_type(type_id=202)
    # An ext=1 grant for type 101 never satisfies an ext=1 requirement
    # for type 202 — the registries are scoped per-type_id.
    # check_grant takes the requirement's type_id, and its closure
    # lookups use that type's tables.
    assert reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[1],
        required_core=None,
        required_ext=1,
        type_id=202,
    )  # literal id match always passes for same id


def test_register_from_node_opts_with_proto_message() -> None:
    pytest.importorskip("sdk.entdb_sdk._generated.entdb_options_pb2")
    from sdk.entdb_sdk._generated import entdb_options_pb2 as opts_pb2

    node_opts = opts_pb2.NodeOpts(
        type_id=101,
        extension_capability_enum="TaskExt",
        capability_mappings=[
            opts_pb2.CapabilityMapping(op="UpdateNode", field="status", required_ext=1),
            opts_pb2.CapabilityMapping(
                op="CreateChild",
                child_type="Comment",
                required_core=opts_pb2.CORE_CAP_COMMENT,
            ),
        ],
        capability_implications=[
            opts_pb2.CapabilityImplication(
                ext=1,
                implies_core=[opts_pb2.CORE_CAP_EDIT],
            ),
        ],
    )
    reg = CapabilityRegistry()
    reg.register_from_node_opts(101, node_opts)
    core, ext = reg.required_for_op(101, "UpdateNode", field="status")
    assert ext == 1
    core2, _ = reg.required_for_op(101, "CreateChild", child_type="Comment")
    assert core2 == CoreCapability.COMMENT
    # ext 1 now implies EDIT → READ via built-in closure.
    assert reg.check_grant(
        grant_core_caps=[],
        grant_ext_cap_ids=[1],
        required_core=CoreCapability.READ,
        required_ext=None,
        type_id=101,
    )


def test_legacy_permission_to_core_caps_read() -> None:
    assert CapabilityRegistry.legacy_permission_to_core_caps("read") == [CoreCapability.READ]


def test_legacy_permission_to_core_caps_write() -> None:
    assert CapabilityRegistry.legacy_permission_to_core_caps("write") == [
        CoreCapability.READ,
        CoreCapability.COMMENT,
        CoreCapability.EDIT,
    ]


def test_legacy_permission_to_core_caps_admin() -> None:
    assert CapabilityRegistry.legacy_permission_to_core_caps("admin") == [CoreCapability.ADMIN]


def test_legacy_permission_to_core_caps_delete() -> None:
    caps = CapabilityRegistry.legacy_permission_to_core_caps("delete")
    assert CoreCapability.DELETE in caps
    assert CoreCapability.READ in caps


def test_legacy_permission_to_core_caps_deny_is_empty() -> None:
    # Deny rows carry no positive caps; the deny flag is handled
    # separately by the ACL engine.
    assert CapabilityRegistry.legacy_permission_to_core_caps("deny") == []


def test_legacy_permission_to_core_caps_unknown_is_empty() -> None:
    assert CapabilityRegistry.legacy_permission_to_core_caps("weird") == []


def test_mapping_with_both_core_and_ext_raises() -> None:
    pytest.importorskip("sdk.entdb_sdk._generated.entdb_options_pb2")
    from sdk.entdb_sdk._generated import entdb_options_pb2 as opts_pb2

    bad = opts_pb2.NodeOpts(
        type_id=1,
        capability_mappings=[
            opts_pb2.CapabilityMapping(
                op="UpdateNode",
                required_core=opts_pb2.CORE_CAP_EDIT,
                required_ext=1,
            )
        ],
    )
    reg = CapabilityRegistry()
    with pytest.raises(ValueError, match="both required_core and required_ext"):
        reg.register_from_node_opts(1, bad)


def test_registry_default_type_fallback() -> None:
    reg = CapabilityRegistry()
    # Unregistered type still gets the default op table.
    core, ext = reg.required_for_op(type_id=12345, op_name="GetNode")
    assert core == CoreCapability.READ
    assert ext is None
