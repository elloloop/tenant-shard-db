"""EntDB schema linter — strict semantic validation.

Second-pass validation gate: catches every schema mistake that protoc
cannot catch. Operates on the parsed NodeInfo/EdgeInfo output from codegen.

Usage:
    from entdb_sdk.lint import lint_schema
    result = lint_schema("schema.proto")
    if result.errors:
        for e in result.errors:
            print(f"ERROR: {e}")
"""

from __future__ import annotations

from dataclasses import dataclass, field

from .codegen import EdgeInfo, FieldInfo, NodeInfo, parse_proto


@dataclass
class LintResult:
    """Lint pass outcome — errors block CI, warnings don't."""

    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)


# ── ID Safety (errors) ──────────────────────────────────────────────


def _type_id_unique(nodes: list[NodeInfo], result: LintResult) -> None:
    """No two node messages may share a type_id."""
    seen: dict[int, str] = {}
    for n in nodes:
        if n.type_id in seen:
            result.errors.append(
                f"Duplicate type_id {n.type_id}: '{n.name}' conflicts with '{seen[n.type_id]}'"
            )
        else:
            seen[n.type_id] = n.name


def _edge_id_unique(edges: list[EdgeInfo], result: LintResult) -> None:
    """No two edge messages may share an edge_id."""
    seen: dict[int, str] = {}
    for e in edges:
        if e.edge_id in seen:
            result.errors.append(
                f"Duplicate edge_id {e.edge_id}: '{e.name}' conflicts with '{seen[e.edge_id]}'"
            )
        else:
            seen[e.edge_id] = e.name


def _type_id_nonzero(nodes: list[NodeInfo], result: LintResult) -> None:
    """type_id must be > 0 (proto3 default 0 = unset)."""
    for n in nodes:
        if n.type_id <= 0:
            result.errors.append(f"Node '{n.name}' has type_id={n.type_id}; must be > 0")


def _edge_id_nonzero(edges: list[EdgeInfo], result: LintResult) -> None:
    """edge_id must be > 0 (proto3 default 0 = unset)."""
    for e in edges:
        if e.edge_id <= 0:
            result.errors.append(f"Edge '{e.name}' has edge_id={e.edge_id}; must be > 0")


# ── Annotation Safety (errors) ──────────────────────────────────────
#
# Rule 5 (node_edge_mutual_exclusion) is enforced structurally:
# parse_proto yields either a NodeInfo or an EdgeInfo per message,
# never both. The extraction functions _extract_node and _extract_edge
# are tried in order and `continue` is used. To catch proto files that
# somehow have both annotations we'd need raw descriptor access. Since
# parse_proto already guarantees mutual exclusion, we include the rule
# as a pass-through check with a comment.


def _node_edge_mutual_exclusion(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """A message cannot have both entdb.node and entdb.edge options.

    The parser enforces this structurally. We double-check that
    no name appears in both lists.
    """
    node_names = {n.name for n in nodes}
    edge_names = {e.name for e in edges}
    overlap = node_names & edge_names
    for name in sorted(overlap):
        result.errors.append(
            f"Message '{name}' has both entdb.node and entdb.edge annotations"
        )


def _edge_has_from_to(edges: list[EdgeInfo], result: LintResult) -> None:
    """Edge messages must have fields 15 (from) and 16 (to) that are message types.

    In the current v2 proto design, from_type/to_type are set at the
    EdgeOpts level (not as message fields). The codegen sets from_type
    and to_type on EdgeInfo. We validate they are set (non-zero).
    """
    for e in edges:
        if e.from_type == 0:
            result.errors.append(
                f"Edge '{e.name}' (edge_id={e.edge_id}) is missing from_type"
            )
        if e.to_type == 0:
            result.errors.append(
                f"Edge '{e.name}' (edge_id={e.edge_id}) is missing to_type"
            )


# ── ACL Safety (errors) ─────────────────────────────────────────────


def _private_inherit_exclusive(nodes: list[NodeInfo], result: LintResult) -> None:
    """A node cannot have both private=true and inherit=true."""
    for n in nodes:
        if n.is_private and n.acl_inherit:
            result.errors.append(
                f"Node '{n.name}' has both private=true and inherit=true; "
                "these are mutually exclusive"
            )


def _propagate_target_inherits(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """If edge has propagate_share=true, the target type must allow ACL inheritance.

    The target (to_type) must have inherit=true or not have private=true.
    """
    node_map = {n.type_id: n for n in nodes}
    for e in edges:
        if not e.propagate_share:
            continue
        target = node_map.get(e.to_type)
        if target is None:
            continue  # can't validate without knowing the target
        if target.is_private:
            result.errors.append(
                f"Edge '{e.name}' has propagate_share=true but target "
                f"'{target.name}' has private=true (blocks propagation)"
            )


# ── GDPR Safety (errors) ────────────────────────────────────────────

_LEGAL_BASIS_REQUIRED_POLICIES = {"FINANCIAL", "AUDIT", "HEALTHCARE"}


def _financial_needs_legal_basis(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """data_policy FINANCIAL requires non-empty legal_basis."""
    for n in nodes:
        if n.data_policy == "FINANCIAL" and not n.legal_basis:
            result.errors.append(
                f"Node '{n.name}' has data_policy=FINANCIAL but no legal_basis"
            )
    for e in edges:
        if e.data_policy == "FINANCIAL" and not e.legal_basis:
            result.errors.append(
                f"Edge '{e.name}' has data_policy=FINANCIAL but no legal_basis"
            )


def _audit_needs_legal_basis(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """data_policy AUDIT requires non-empty legal_basis."""
    for n in nodes:
        if n.data_policy == "AUDIT" and not n.legal_basis:
            result.errors.append(
                f"Node '{n.name}' has data_policy=AUDIT but no legal_basis"
            )
    for e in edges:
        if e.data_policy == "AUDIT" and not e.legal_basis:
            result.errors.append(
                f"Edge '{e.name}' has data_policy=AUDIT but no legal_basis"
            )


def _healthcare_needs_legal_basis(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """data_policy HEALTHCARE requires non-empty legal_basis."""
    for n in nodes:
        if n.data_policy == "HEALTHCARE" and not n.legal_basis:
            result.errors.append(
                f"Node '{n.name}' has data_policy=HEALTHCARE but no legal_basis"
            )
    for e in edges:
        if e.data_policy == "HEALTHCARE" and not e.legal_basis:
            result.errors.append(
                f"Edge '{e.name}' has data_policy=HEALTHCARE but no legal_basis"
            )


def _retention_needs_legal_basis(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """retention_days > 0 requires non-empty legal_basis."""
    for n in nodes:
        if n.retention_days > 0 and not n.legal_basis:
            result.errors.append(
                f"Node '{n.name}' has retention_days={n.retention_days} but no legal_basis"
            )
    for e in edges:
        if e.retention_days > 0 and not e.legal_basis:
            result.errors.append(
                f"Edge '{e.name}' has retention_days={e.retention_days} but no legal_basis"
            )


# ── GDPR Warnings ───────────────────────────────────────────────────


def _data_policy_unset(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """WARNING: node/edge type has no data_policy set (defaults to PERSONAL)."""
    for n in nodes:
        if n.data_policy == "PERSONAL":
            result.warnings.append(
                f"Node '{n.name}' has data_policy=PERSONAL (the default); "
                "consider setting an explicit data policy"
            )
    for e in edges:
        if e.data_policy == "PERSONAL":
            result.warnings.append(
                f"Edge '{e.name}' has data_policy=PERSONAL (the default); "
                "consider setting an explicit data policy"
            )


def _string_pii_unmarked(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """WARNING: string field without pii=true or pii_false=true must explicitly declare."""
    def _check_fields(entity_name: str, fields: list[FieldInfo]) -> None:
        for f in fields:
            if f.kind in ("str", "enum") and not f.pii and not f.pii_false:
                result.warnings.append(
                    f"Field '{entity_name}.{f.name}' is a string without "
                    "pii=true or pii_false=true; explicitly declare PII status"
                )

    for n in nodes:
        _check_fields(n.name, n.fields)
    for e in edges:
        _check_fields(e.name, e.props)


def _edge_user_needs_exit_policy(
    nodes: list[NodeInfo], edges: list[EdgeInfo], result: LintResult
) -> None:
    """WARNING: edge where from or to is a User-type should have on_subject_exit set.

    Heuristic: a "User-type" is any node whose name contains "User" (case-insensitive)
    or has subject_field set to a non-empty value indicating it represents a person.
    """
    user_type_ids: set[int] = set()
    for n in nodes:
        if "user" in n.name.lower() or n.subject_field:
            user_type_ids.add(n.type_id)

    for e in edges:
        touches_user = e.from_type in user_type_ids or e.to_type in user_type_ids
        if touches_user and e.on_subject_exit == "BOTH":
            result.warnings.append(
                f"Edge '{e.name}' references a User-type but on_subject_exit "
                "defaults to BOTH; consider setting an explicit exit policy"
            )


# ── Edge Safety (warnings) ──────────────────────────────────────────


def _edge_self_referential(edges: list[EdgeInfo], result: LintResult) -> None:
    """WARNING: edge where from and to are the same type."""
    for e in edges:
        if e.from_type != 0 and e.from_type == e.to_type:
            result.warnings.append(
                f"Edge '{e.name}' is self-referential "
                f"(from_type={e.from_type} == to_type={e.to_type})"
            )


# ── Main entry point ────────────────────────────────────────────────


def lint_schema(
    proto_path: str, include_dirs: list[str] | None = None
) -> LintResult:
    """Run all lint rules on a parsed proto schema.

    Args:
        proto_path: Path to the .proto file.
        include_dirs: Additional proto include directories.

    Returns:
        LintResult with errors (block CI) and warnings (informational).
    """
    nodes, edges = parse_proto(proto_path, include_dirs=include_dirs)
    return lint_parsed(nodes, edges)


def lint_parsed(nodes: list[NodeInfo], edges: list[EdgeInfo]) -> LintResult:
    """Run all lint rules on already-parsed schema data.

    This is the core lint engine — useful for testing without needing
    a proto file on disk.

    Args:
        nodes: Parsed node type definitions.
        edges: Parsed edge type definitions.

    Returns:
        LintResult with errors and warnings.
    """
    result = LintResult()

    # ID Safety
    _type_id_unique(nodes, result)
    _edge_id_unique(edges, result)
    _type_id_nonzero(nodes, result)
    _edge_id_nonzero(edges, result)

    # Annotation Safety
    _node_edge_mutual_exclusion(nodes, edges, result)
    _edge_has_from_to(edges, result)

    # ACL Safety
    _private_inherit_exclusive(nodes, result)
    _propagate_target_inherits(nodes, edges, result)

    # GDPR Safety (errors)
    _financial_needs_legal_basis(nodes, edges, result)
    _audit_needs_legal_basis(nodes, edges, result)
    _healthcare_needs_legal_basis(nodes, edges, result)
    _retention_needs_legal_basis(nodes, edges, result)

    # GDPR Warnings
    _data_policy_unset(nodes, edges, result)
    _string_pii_unmarked(nodes, edges, result)
    _edge_user_needs_exit_policy(nodes, edges, result)

    # Edge Safety (warnings)
    _edge_self_referential(edges, result)

    return result
