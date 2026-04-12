"""Tests for entdb_sdk.lint — strict semantic validation.

One test per lint rule, plus integration tests for clean schemas,
multiple errors, and CLI exit code behavior.
"""

from __future__ import annotations

import os
import subprocess
import sys

# Ensure the local sdk/ tree is importable (it has codegen, lint, cli).
_SDK_ROOT = os.path.join(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))),
    "sdk",
)
if _SDK_ROOT not in sys.path:
    sys.path.insert(0, _SDK_ROOT)

from entdb_sdk.codegen import EdgeInfo, FieldInfo, NodeInfo
from entdb_sdk.lint import lint_parsed

# ── Helpers ──────────────────────────────────────────────────────────


def _node(
    type_id: int = 1,
    name: str = "TestNode",
    fields: list[FieldInfo] | None = None,
    acl_inherit: bool = False,
    is_private: bool = False,
    data_policy: str = "BUSINESS",
    subject_field: str = "",
    retention_days: int = 0,
    legal_basis: str = "",
) -> NodeInfo:
    return NodeInfo(
        type_id=type_id,
        name=name,
        fields=fields or [],
        acl_inherit=acl_inherit,
        is_private=is_private,
        data_policy=data_policy,
        subject_field=subject_field,
        retention_days=retention_days,
        legal_basis=legal_basis,
    )


def _edge(
    edge_id: int = 101,
    name: str = "TestEdge",
    from_type: int = 1,
    to_type: int = 2,
    props: list[FieldInfo] | None = None,
    propagate_share: bool = False,
    data_policy: str = "BUSINESS",
    on_subject_exit: str = "FROM",
    retention_days: int = 0,
    legal_basis: str = "",
) -> EdgeInfo:
    return EdgeInfo(
        edge_id=edge_id,
        name=name,
        from_type=from_type,
        to_type=to_type,
        props=props or [],
        propagate_share=propagate_share,
        data_policy=data_policy,
        on_subject_exit=on_subject_exit,
        retention_days=retention_days,
        legal_basis=legal_basis,
    )


def _field(
    field_id: int = 1,
    name: str = "test_field",
    kind: str = "str",
    pii: bool = False,
    pii_false: bool = False,
) -> FieldInfo:
    return FieldInfo(
        field_id=field_id,
        name=name,
        kind=kind,
        pii=pii,
        pii_false=pii_false,
    )


# ── Rule 1: type_id_unique ──────────────────────────────────────────


class TestTypeIdUnique:
    def test_duplicate_type_id_error(self):
        """Two nodes with the same type_id produce an error."""
        nodes = [_node(type_id=1, name="Alpha"), _node(type_id=1, name="Beta")]
        result = lint_parsed(nodes, [])
        assert any("Duplicate type_id 1" in e for e in result.errors)

    def test_unique_type_ids_no_error(self):
        """Distinct type_ids produce no error."""
        nodes = [_node(type_id=1, name="Alpha"), _node(type_id=2, name="Beta")]
        result = lint_parsed(nodes, [])
        assert not any("Duplicate type_id" in e for e in result.errors)


# ── Rule 2: edge_id_unique ──────────────────────────────────────────


class TestEdgeIdUnique:
    def test_duplicate_edge_id_error(self):
        """Two edges with the same edge_id produce an error."""
        edges = [_edge(edge_id=101, name="E1"), _edge(edge_id=101, name="E2")]
        result = lint_parsed([], edges)
        assert any("Duplicate edge_id 101" in e for e in result.errors)

    def test_unique_edge_ids_no_error(self):
        """Distinct edge_ids produce no error."""
        edges = [_edge(edge_id=101, name="E1"), _edge(edge_id=102, name="E2")]
        result = lint_parsed([], edges)
        assert not any("Duplicate edge_id" in e for e in result.errors)


# ── Rule 3: type_id_nonzero ─────────────────────────────────────────


class TestTypeIdNonzero:
    def test_zero_type_id_error(self):
        """type_id=0 produces an error."""
        nodes = [_node(type_id=0, name="Bad")]
        result = lint_parsed(nodes, [])
        assert any("type_id=0" in e and "must be > 0" in e for e in result.errors)

    def test_positive_type_id_no_error(self):
        """type_id=1 is valid."""
        nodes = [_node(type_id=1, name="Good")]
        result = lint_parsed(nodes, [])
        assert not any("type_id=0" in e for e in result.errors)


# ── Rule 4: edge_id_nonzero ─────────────────────────────────────────


class TestEdgeIdNonzero:
    def test_zero_edge_id_error(self):
        """edge_id=0 produces an error."""
        edges = [_edge(edge_id=0, name="Bad")]
        result = lint_parsed([], edges)
        assert any("edge_id=0" in e and "must be > 0" in e for e in result.errors)

    def test_positive_edge_id_no_error(self):
        """edge_id=101 is valid."""
        edges = [_edge(edge_id=101, name="Good")]
        result = lint_parsed([], edges)
        assert not any("edge_id=0" in e for e in result.errors)


# ── Rule 5: node_edge_mutual_exclusion ──────────────────────────────


class TestNodeEdgeMutualExclusion:
    def test_same_name_both_lists_error(self):
        """A name in both nodes and edges produces an error."""
        nodes = [_node(type_id=1, name="Overlap")]
        edges = [_edge(edge_id=101, name="Overlap")]
        result = lint_parsed(nodes, edges)
        assert any("both entdb.node and entdb.edge" in e for e in result.errors)

    def test_disjoint_names_no_error(self):
        """Distinct names in nodes and edges produce no error."""
        nodes = [_node(type_id=1, name="User")]
        edges = [_edge(edge_id=101, name="Owns")]
        result = lint_parsed(nodes, edges)
        assert not any("both entdb.node and entdb.edge" in e for e in result.errors)


# ── Rule 6: edge_has_from_to ────────────────────────────────────────


class TestEdgeHasFromTo:
    def test_missing_from_type_error(self):
        """Edge with from_type=0 produces an error."""
        edges = [_edge(edge_id=101, name="Bad", from_type=0, to_type=2)]
        result = lint_parsed([], edges)
        assert any("missing from_type" in e for e in result.errors)

    def test_missing_to_type_error(self):
        """Edge with to_type=0 produces an error."""
        edges = [_edge(edge_id=101, name="Bad", from_type=1, to_type=0)]
        result = lint_parsed([], edges)
        assert any("missing to_type" in e for e in result.errors)

    def test_both_set_no_error(self):
        """Edge with both from/to set produces no from/to error."""
        edges = [_edge(edge_id=101, name="Good", from_type=1, to_type=2)]
        result = lint_parsed([], edges)
        assert not any("missing from_type" in e or "missing to_type" in e for e in result.errors)


# ── Rule 7: private_inherit_exclusive ────────────────────────────────


class TestPrivateInheritExclusive:
    def test_both_private_and_inherit_error(self):
        """private=true and inherit=true on same node produce an error."""
        nodes = [_node(type_id=1, name="Bad", is_private=True, acl_inherit=True)]
        result = lint_parsed(nodes, [])
        assert any("private=true and inherit=true" in e for e in result.errors)

    def test_private_only_no_error(self):
        """private=true alone is valid."""
        nodes = [_node(type_id=1, name="Ok", is_private=True, acl_inherit=False)]
        result = lint_parsed(nodes, [])
        assert not any("private=true and inherit=true" in e for e in result.errors)

    def test_inherit_only_no_error(self):
        """inherit=true alone is valid."""
        nodes = [_node(type_id=1, name="Ok", is_private=False, acl_inherit=True)]
        result = lint_parsed(nodes, [])
        assert not any("private=true and inherit=true" in e for e in result.errors)


# ── Rule 8: propagate_target_inherits ────────────────────────────────


class TestPropagateTargetInherits:
    def test_propagate_to_private_target_error(self):
        """propagate_share=true targeting a private node is an error."""
        nodes = [
            _node(type_id=1, name="Source"),
            _node(type_id=2, name="Target", is_private=True),
        ]
        edges = [_edge(edge_id=101, from_type=1, to_type=2, propagate_share=True)]
        result = lint_parsed(nodes, edges)
        assert any("propagate_share=true" in e and "private=true" in e for e in result.errors)

    def test_propagate_to_non_private_no_error(self):
        """propagate_share=true targeting a non-private node is valid."""
        nodes = [
            _node(type_id=1, name="Source"),
            _node(type_id=2, name="Target", is_private=False),
        ]
        edges = [_edge(edge_id=101, from_type=1, to_type=2, propagate_share=True)]
        result = lint_parsed(nodes, edges)
        assert not any("propagate_share" in e and "private" in e for e in result.errors)

    def test_no_propagate_private_target_no_error(self):
        """Non-propagating edge to private node is fine."""
        nodes = [
            _node(type_id=1, name="Source"),
            _node(type_id=2, name="Target", is_private=True),
        ]
        edges = [_edge(edge_id=101, from_type=1, to_type=2, propagate_share=False)]
        result = lint_parsed(nodes, edges)
        assert not any("propagate_share" in e for e in result.errors)


# ── Rule 9: financial_needs_legal_basis ──────────────────────────────


class TestFinancialNeedsLegalBasis:
    def test_financial_no_basis_error(self):
        """FINANCIAL data_policy without legal_basis is an error."""
        nodes = [_node(type_id=1, name="Invoice", data_policy="FINANCIAL")]
        result = lint_parsed(nodes, [])
        assert any("FINANCIAL" in e and "legal_basis" in e for e in result.errors)

    def test_financial_with_basis_no_error(self):
        """FINANCIAL with legal_basis is valid."""
        nodes = [
            _node(
                type_id=1,
                name="Invoice",
                data_policy="FINANCIAL",
                legal_basis="Tax law 26 USC",
            )
        ]
        result = lint_parsed(nodes, [])
        assert not any("FINANCIAL" in e and "legal_basis" in e for e in result.errors)

    def test_financial_edge_no_basis_error(self):
        """Edge with FINANCIAL data_policy without legal_basis is an error."""
        edges = [_edge(edge_id=101, data_policy="FINANCIAL")]
        result = lint_parsed([], edges)
        assert any("FINANCIAL" in e and "legal_basis" in e for e in result.errors)


# ── Rule 10: healthcare_needs_legal_basis ────────────────────────────


class TestHealthcareNeedsLegalBasis:
    def test_healthcare_no_basis_error(self):
        """HEALTHCARE data_policy without legal_basis is an error."""
        nodes = [_node(type_id=1, name="PatientRecord", data_policy="HEALTHCARE")]
        result = lint_parsed(nodes, [])
        assert any("HEALTHCARE" in e and "legal_basis" in e for e in result.errors)

    def test_healthcare_with_basis_no_error(self):
        """HEALTHCARE with legal_basis is valid."""
        nodes = [
            _node(
                type_id=1,
                name="PatientRecord",
                data_policy="HEALTHCARE",
                legal_basis="HIPAA 45 CFR 164",
            )
        ]
        result = lint_parsed(nodes, [])
        assert not any("HEALTHCARE" in e and "legal_basis" in e for e in result.errors)


# ── Rule 11: retention_needs_legal_basis ─────────────────────────────


class TestRetentionNeedsLegalBasis:
    def test_retention_no_basis_error(self):
        """retention_days > 0 without legal_basis is an error."""
        nodes = [_node(type_id=1, name="AuditLog", retention_days=365)]
        result = lint_parsed(nodes, [])
        assert any("retention_days=365" in e and "legal_basis" in e for e in result.errors)

    def test_retention_with_basis_no_error(self):
        """retention_days > 0 with legal_basis is valid."""
        nodes = [
            _node(
                type_id=1,
                name="AuditLog",
                retention_days=365,
                legal_basis="SOX compliance",
            )
        ]
        result = lint_parsed(nodes, [])
        assert not any("retention_days" in e and "legal_basis" in e for e in result.errors)

    def test_zero_retention_no_error(self):
        """retention_days=0 needs no legal_basis."""
        nodes = [_node(type_id=1, name="Temp")]
        result = lint_parsed(nodes, [])
        assert not any("retention_days" in e for e in result.errors)

    def test_edge_retention_no_basis_error(self):
        """Edge with retention_days > 0 without legal_basis is an error."""
        edges = [_edge(edge_id=101, retention_days=90)]
        result = lint_parsed([], edges)
        assert any("retention_days=90" in e and "legal_basis" in e for e in result.errors)


# ── Rule 12: data_policy_unset (WARNING) ─────────────────────────────


class TestDataPolicyUnset:
    def test_default_personal_warning(self):
        """PERSONAL (the default) triggers a warning."""
        nodes = [_node(type_id=1, name="Stuff", data_policy="PERSONAL")]
        result = lint_parsed(nodes, [])
        assert any("data_policy=PERSONAL" in w for w in result.warnings)

    def test_explicit_business_no_warning(self):
        """Explicit BUSINESS does not trigger the warning."""
        nodes = [_node(type_id=1, name="Stuff", data_policy="BUSINESS")]
        result = lint_parsed(nodes, [])
        assert not any("data_policy=PERSONAL" in w for w in result.warnings)


# ── Rule 13: string_pii_unmarked (WARNING) ───────────────────────────


class TestStringPiiUnmarked:
    def test_string_without_pii_declaration_warning(self):
        """String field with neither pii nor pii_false triggers a warning."""
        nodes = [
            _node(
                type_id=1,
                name="User",
                fields=[_field(1, "email", "str", pii=False, pii_false=False)],
            )
        ]
        result = lint_parsed(nodes, [])
        assert any("User.email" in w and "pii" in w for w in result.warnings)

    def test_string_with_pii_true_no_warning(self):
        """String field with pii=true does not trigger the warning."""
        nodes = [
            _node(
                type_id=1,
                name="User",
                fields=[_field(1, "email", "str", pii=True)],
            )
        ]
        result = lint_parsed(nodes, [])
        assert not any("User.email" in w for w in result.warnings)

    def test_string_with_pii_false_no_warning(self):
        """String field with pii_false=true does not trigger the warning."""
        nodes = [
            _node(
                type_id=1,
                name="User",
                fields=[_field(1, "role", "str", pii_false=True)],
            )
        ]
        result = lint_parsed(nodes, [])
        assert not any("User.role" in w for w in result.warnings)

    def test_int_field_no_warning(self):
        """Non-string fields are not checked for PII."""
        nodes = [
            _node(
                type_id=1,
                name="Counter",
                fields=[_field(1, "count", "int")],
            )
        ]
        result = lint_parsed(nodes, [])
        assert not any("Counter.count" in w for w in result.warnings)

    def test_edge_prop_string_unmarked_warning(self):
        """Edge property string without PII declaration triggers warning."""
        edges = [
            _edge(
                edge_id=101,
                name="Rel",
                props=[_field(1, "role", "str")],
            )
        ]
        result = lint_parsed([], edges)
        assert any("Rel.role" in w and "pii" in w for w in result.warnings)


# ── Rule 14: edge_user_needs_exit_policy (WARNING) ───────────────────


class TestEdgeUserNeedsExitPolicy:
    def test_user_edge_default_exit_warning(self):
        """Edge referencing a User-type with default BOTH triggers a warning."""
        nodes = [
            _node(type_id=1, name="User", subject_field="id"),
            _node(type_id=2, name="Project"),
        ]
        edges = [_edge(edge_id=101, from_type=1, to_type=2, on_subject_exit="BOTH")]
        result = lint_parsed(nodes, edges)
        assert any("User-type" in w and "exit policy" in w for w in result.warnings)

    def test_user_edge_explicit_exit_no_warning(self):
        """Edge with explicit on_subject_exit set does not warn."""
        nodes = [
            _node(type_id=1, name="User", subject_field="id"),
            _node(type_id=2, name="Project"),
        ]
        edges = [_edge(edge_id=101, from_type=1, to_type=2, on_subject_exit="FROM")]
        result = lint_parsed(nodes, edges)
        assert not any("exit policy" in w for w in result.warnings)

    def test_non_user_edge_no_warning(self):
        """Edge not referencing a User-type does not warn."""
        nodes = [
            _node(type_id=1, name="Project"),
            _node(type_id=2, name="Task"),
        ]
        edges = [_edge(edge_id=101, from_type=1, to_type=2, on_subject_exit="BOTH")]
        result = lint_parsed(nodes, edges)
        assert not any("User-type" in w for w in result.warnings)


# ── Rule 15: edge_self_referential (WARNING) ─────────────────────────


class TestEdgeSelfReferential:
    def test_self_referential_edge_warning(self):
        """Edge where from_type == to_type triggers a warning."""
        edges = [_edge(edge_id=101, name="Follows", from_type=1, to_type=1)]
        result = lint_parsed([], edges)
        assert any("self-referential" in w for w in result.warnings)

    def test_non_self_referential_no_warning(self):
        """Edge where from_type != to_type does not warn."""
        edges = [_edge(edge_id=101, name="Owns", from_type=1, to_type=2)]
        result = lint_parsed([], edges)
        assert not any("self-referential" in w for w in result.warnings)


# ── Integration: Clean schema ────────────────────────────────────────


class TestCleanSchema:
    def test_clean_schema_no_errors_no_warnings(self):
        """A well-formed schema produces 0 errors and 0 warnings."""
        nodes = [
            _node(
                type_id=1,
                name="User",
                data_policy="BUSINESS",
                fields=[
                    _field(1, "email", "str", pii=True),
                    _field(2, "name", "str", pii=True),
                    _field(3, "role", "str", pii_false=True),
                    _field(4, "count", "int"),
                ],
            ),
            _node(
                type_id=2,
                name="Project",
                data_policy="BUSINESS",
                acl_inherit=True,
                fields=[
                    _field(1, "name", "str", pii_false=True),
                    _field(2, "created_at", "timestamp"),
                ],
            ),
        ]
        edges = [
            _edge(
                edge_id=101,
                name="Owns",
                from_type=1,
                to_type=2,
                data_policy="BUSINESS",
                on_subject_exit="FROM",
            ),
        ]
        result = lint_parsed(nodes, edges)
        assert result.errors == []
        assert result.warnings == []


# ── Integration: Multiple errors ─────────────────────────────────────


class TestMultipleErrors:
    def test_multiple_errors_in_one_pass(self):
        """Multiple rule violations are all reported in a single pass."""
        nodes = [
            # duplicate type_id
            _node(type_id=1, name="A"),
            _node(type_id=1, name="B"),
            # private + inherit conflict
            _node(type_id=3, name="C", is_private=True, acl_inherit=True),
        ]
        edges = [
            # zero edge_id
            _edge(edge_id=0, name="Bad"),
        ]
        result = lint_parsed(nodes, edges)
        # Should have at least 3 distinct errors
        assert len(result.errors) >= 3
        assert any("Duplicate type_id" in e for e in result.errors)
        assert any("private=true and inherit=true" in e for e in result.errors)
        assert any("edge_id=0" in e for e in result.errors)


# ── Integration: Warnings don't fail ─────────────────────────────────


class TestWarningsDontFail:
    def test_warnings_only_no_errors(self):
        """Schema with only warnings has errors=[] (exit code 0)."""
        nodes = [
            _node(
                type_id=1,
                name="User",
                data_policy="PERSONAL",  # triggers data_policy_unset warning
                fields=[_field(1, "email", "str")],  # triggers pii unmarked warning
            ),
        ]
        result = lint_parsed(nodes, [])
        assert result.errors == []
        assert len(result.warnings) > 0


# ── CLI exit code behavior ───────────────────────────────────────────


class TestCliExitCode:
    """Test the CLI wrapper produces correct exit codes.

    Uses subprocess to invoke the module as the CLI would.
    """

    def test_lint_missing_file_exits_1(self):
        """entdb lint on a missing file exits with code 1."""
        env = os.environ.copy()
        env["PYTHONPATH"] = _SDK_ROOT + os.pathsep + env.get("PYTHONPATH", "")
        proc = subprocess.run(
            [sys.executable, "-m", "entdb_sdk.cli", "lint", "/nonexistent/schema.proto"],
            capture_output=True,
            text=True,
            env=env,
        )
        assert proc.returncode == 1
        assert "Error" in proc.stderr or "not found" in proc.stderr
