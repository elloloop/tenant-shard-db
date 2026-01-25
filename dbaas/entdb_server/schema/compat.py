"""
Schema compatibility checking for EntDB.

This module enforces protobuf-like schema evolution rules:
- IDs (type_id, edge_id, field_id) are immutable
- Field kinds cannot change
- Fields/types can be deprecated but not removed
- IDs cannot be reused after deprecation
- New fields/types can be added with new IDs

Invariants:
    - Breaking changes are NEVER allowed
    - Compatibility is checked before deployment
    - CI/CD must fail if compatibility check fails

How to change safely:
    - Add new fields with new field_ids
    - Add new types with new type_ids
    - Deprecate (never delete) obsolete fields/types
    - Run schema check before every deployment

Example:
    >>> from entdb_server.schema.compat import check_compatibility
    >>> changes = check_compatibility(old_registry, new_registry)
    >>> breaking = [c for c in changes if c.is_breaking]
    >>> if breaking:
    ...     raise CompatibilityError(breaking)
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from enum import Enum, auto
from typing import List, Optional, Dict, Any
import logging

from .types import NodeTypeDef, EdgeTypeDef, FieldDef, FieldKind
from .registry import SchemaRegistry

logger = logging.getLogger(__name__)


class ChangeKind(Enum):
    """Types of schema changes."""
    # Non-breaking changes (allowed)
    NODE_TYPE_ADDED = auto()
    EDGE_TYPE_ADDED = auto()
    FIELD_ADDED = auto()
    PROP_ADDED = auto()
    TYPE_DEPRECATED = auto()
    FIELD_DEPRECATED = auto()
    DESCRIPTION_CHANGED = auto()
    NAME_CHANGED = auto()
    ENUM_VALUE_ADDED = auto()
    INDEX_ADDED = auto()
    SEARCHABLE_ADDED = auto()

    # Breaking changes (forbidden)
    NODE_TYPE_REMOVED = auto()
    EDGE_TYPE_REMOVED = auto()
    FIELD_REMOVED = auto()
    PROP_REMOVED = auto()
    FIELD_KIND_CHANGED = auto()
    TYPE_ID_REUSED = auto()
    EDGE_ID_REUSED = auto()
    FIELD_ID_REUSED = auto()
    ENUM_VALUE_REMOVED = auto()
    ENUM_VALUE_REORDERED = auto()
    FROM_TYPE_CHANGED = auto()
    TO_TYPE_CHANGED = auto()
    REQUIRED_ADDED = auto()  # Making optional field required

    @property
    def is_breaking(self) -> bool:
        """Whether this change kind is a breaking change."""
        breaking_kinds = {
            ChangeKind.NODE_TYPE_REMOVED,
            ChangeKind.EDGE_TYPE_REMOVED,
            ChangeKind.FIELD_REMOVED,
            ChangeKind.PROP_REMOVED,
            ChangeKind.FIELD_KIND_CHANGED,
            ChangeKind.TYPE_ID_REUSED,
            ChangeKind.EDGE_ID_REUSED,
            ChangeKind.FIELD_ID_REUSED,
            ChangeKind.ENUM_VALUE_REMOVED,
            ChangeKind.ENUM_VALUE_REORDERED,
            ChangeKind.FROM_TYPE_CHANGED,
            ChangeKind.TO_TYPE_CHANGED,
            ChangeKind.REQUIRED_ADDED,
        }
        return self in breaking_kinds


@dataclass
class SchemaChange:
    """Represents a single schema change between versions.

    Attributes:
        kind: The type of change
        path: Path to the changed element (e.g., "NodeType:User.field:email")
        old_value: Previous value (if applicable)
        new_value: New value (if applicable)
        message: Human-readable description of the change
    """
    kind: ChangeKind
    path: str
    old_value: Optional[Any] = None
    new_value: Optional[Any] = None
    message: str = ""

    @property
    def is_breaking(self) -> bool:
        """Whether this is a breaking change."""
        return self.kind.is_breaking

    def __str__(self) -> str:
        status = "BREAKING" if self.is_breaking else "OK"
        return f"[{status}] {self.kind.name}: {self.path} - {self.message}"


class CompatibilityError(Exception):
    """Raised when breaking schema changes are detected.

    Attributes:
        changes: List of breaking changes detected
    """

    def __init__(self, changes: List[SchemaChange]):
        self.changes = changes
        messages = [str(c) for c in changes]
        super().__init__(
            f"Schema compatibility check failed with {len(changes)} breaking change(s):\n"
            + "\n".join(messages)
        )


def check_compatibility(
    old_registry: SchemaRegistry,
    new_registry: SchemaRegistry,
) -> List[SchemaChange]:
    """Check compatibility between two schema versions.

    This performs a comprehensive comparison of two schema registries
    and returns a list of all changes detected. Breaking changes
    should cause deployment to fail.

    Args:
        old_registry: The baseline (currently deployed) schema
        new_registry: The new (to be deployed) schema

    Returns:
        List of SchemaChange objects describing all differences

    Example:
        >>> changes = check_compatibility(old_registry, new_registry)
        >>> breaking = [c for c in changes if c.is_breaking]
        >>> if breaking:
        ...     raise CompatibilityError(breaking)
    """
    changes: List[SchemaChange] = []

    # Convert to dicts for comparison
    old_dict = old_registry.to_dict()
    new_dict = new_registry.to_dict()

    # Build lookup maps
    old_nodes: Dict[int, dict] = {n["type_id"]: n for n in old_dict["node_types"]}
    new_nodes: Dict[int, dict] = {n["type_id"]: n for n in new_dict["node_types"]}
    old_edges: Dict[int, dict] = {e["edge_id"]: e for e in old_dict["edge_types"]}
    new_edges: Dict[int, dict] = {e["edge_id"]: e for e in new_dict["edge_types"]}

    # Check node types
    changes.extend(_check_node_types(old_nodes, new_nodes))

    # Check edge types
    changes.extend(_check_edge_types(old_edges, new_edges))

    # Check for ID reuse (type_id used for different type)
    changes.extend(_check_id_reuse(old_nodes, new_nodes, old_edges, new_edges))

    return changes


def _check_node_types(
    old_nodes: Dict[int, dict],
    new_nodes: Dict[int, dict],
) -> List[SchemaChange]:
    """Check node type changes."""
    changes: List[SchemaChange] = []

    # Check for removed node types
    for type_id, old_node in old_nodes.items():
        if type_id not in new_nodes:
            changes.append(SchemaChange(
                kind=ChangeKind.NODE_TYPE_REMOVED,
                path=f"NodeType:{old_node['name']}",
                old_value=type_id,
                message=f"Node type '{old_node['name']}' (type_id={type_id}) was removed"
            ))

    # Check for added and modified node types
    for type_id, new_node in new_nodes.items():
        if type_id not in old_nodes:
            changes.append(SchemaChange(
                kind=ChangeKind.NODE_TYPE_ADDED,
                path=f"NodeType:{new_node['name']}",
                new_value=type_id,
                message=f"Node type '{new_node['name']}' (type_id={type_id}) added"
            ))
        else:
            old_node = old_nodes[type_id]
            changes.extend(_check_node_type_diff(old_node, new_node))

    return changes


def _check_node_type_diff(old_node: dict, new_node: dict) -> List[SchemaChange]:
    """Check differences between two versions of a node type."""
    changes: List[SchemaChange] = []
    path_prefix = f"NodeType:{old_node['name']}"

    # Check name change
    if old_node["name"] != new_node["name"]:
        changes.append(SchemaChange(
            kind=ChangeKind.NAME_CHANGED,
            path=path_prefix,
            old_value=old_node["name"],
            new_value=new_node["name"],
            message=f"Node type renamed from '{old_node['name']}' to '{new_node['name']}'"
        ))

    # Check deprecation
    old_deprecated = old_node.get("deprecated", False)
    new_deprecated = new_node.get("deprecated", False)
    if not old_deprecated and new_deprecated:
        changes.append(SchemaChange(
            kind=ChangeKind.TYPE_DEPRECATED,
            path=path_prefix,
            message=f"Node type '{old_node['name']}' deprecated"
        ))

    # Check description change
    if old_node.get("description", "") != new_node.get("description", ""):
        changes.append(SchemaChange(
            kind=ChangeKind.DESCRIPTION_CHANGED,
            path=path_prefix,
            old_value=old_node.get("description", ""),
            new_value=new_node.get("description", ""),
            message="Description changed"
        ))

    # Check fields
    old_fields = {f["field_id"]: f for f in old_node.get("fields", [])}
    new_fields = {f["field_id"]: f for f in new_node.get("fields", [])}

    changes.extend(_check_fields(
        old_fields, new_fields,
        parent_path=path_prefix,
        field_label="field"
    ))

    return changes


def _check_edge_types(
    old_edges: Dict[int, dict],
    new_edges: Dict[int, dict],
) -> List[SchemaChange]:
    """Check edge type changes."""
    changes: List[SchemaChange] = []

    # Check for removed edge types
    for edge_id, old_edge in old_edges.items():
        if edge_id not in new_edges:
            changes.append(SchemaChange(
                kind=ChangeKind.EDGE_TYPE_REMOVED,
                path=f"EdgeType:{old_edge['name']}",
                old_value=edge_id,
                message=f"Edge type '{old_edge['name']}' (edge_id={edge_id}) was removed"
            ))

    # Check for added and modified edge types
    for edge_id, new_edge in new_edges.items():
        if edge_id not in old_edges:
            changes.append(SchemaChange(
                kind=ChangeKind.EDGE_TYPE_ADDED,
                path=f"EdgeType:{new_edge['name']}",
                new_value=edge_id,
                message=f"Edge type '{new_edge['name']}' (edge_id={edge_id}) added"
            ))
        else:
            old_edge = old_edges[edge_id]
            changes.extend(_check_edge_type_diff(old_edge, new_edge))

    return changes


def _check_edge_type_diff(old_edge: dict, new_edge: dict) -> List[SchemaChange]:
    """Check differences between two versions of an edge type."""
    changes: List[SchemaChange] = []
    path_prefix = f"EdgeType:{old_edge['name']}"

    # Check name change
    if old_edge["name"] != new_edge["name"]:
        changes.append(SchemaChange(
            kind=ChangeKind.NAME_CHANGED,
            path=path_prefix,
            old_value=old_edge["name"],
            new_value=new_edge["name"],
            message=f"Edge type renamed from '{old_edge['name']}' to '{new_edge['name']}'"
        ))

    # Check from_type_id change (breaking)
    if old_edge["from_type_id"] != new_edge["from_type_id"]:
        changes.append(SchemaChange(
            kind=ChangeKind.FROM_TYPE_CHANGED,
            path=path_prefix,
            old_value=old_edge["from_type_id"],
            new_value=new_edge["from_type_id"],
            message=f"Edge from_type_id changed from {old_edge['from_type_id']} to {new_edge['from_type_id']}"
        ))

    # Check to_type_id change (breaking)
    if old_edge["to_type_id"] != new_edge["to_type_id"]:
        changes.append(SchemaChange(
            kind=ChangeKind.TO_TYPE_CHANGED,
            path=path_prefix,
            old_value=old_edge["to_type_id"],
            new_value=new_edge["to_type_id"],
            message=f"Edge to_type_id changed from {old_edge['to_type_id']} to {new_edge['to_type_id']}"
        ))

    # Check deprecation
    old_deprecated = old_edge.get("deprecated", False)
    new_deprecated = new_edge.get("deprecated", False)
    if not old_deprecated and new_deprecated:
        changes.append(SchemaChange(
            kind=ChangeKind.TYPE_DEPRECATED,
            path=path_prefix,
            message=f"Edge type '{old_edge['name']}' deprecated"
        ))

    # Check props
    old_props = {p["field_id"]: p for p in old_edge.get("props", [])}
    new_props = {p["field_id"]: p for p in new_edge.get("props", [])}

    changes.extend(_check_fields(
        old_props, new_props,
        parent_path=path_prefix,
        field_label="prop"
    ))

    return changes


def _check_fields(
    old_fields: Dict[int, dict],
    new_fields: Dict[int, dict],
    parent_path: str,
    field_label: str = "field",
) -> List[SchemaChange]:
    """Check field/property changes."""
    changes: List[SchemaChange] = []

    # Check for removed fields
    for field_id, old_field in old_fields.items():
        if field_id not in new_fields:
            kind = ChangeKind.FIELD_REMOVED if field_label == "field" else ChangeKind.PROP_REMOVED
            changes.append(SchemaChange(
                kind=kind,
                path=f"{parent_path}.{field_label}:{old_field['name']}",
                old_value=field_id,
                message=f"Field '{old_field['name']}' (field_id={field_id}) was removed"
            ))

    # Check for added and modified fields
    for field_id, new_field in new_fields.items():
        if field_id not in old_fields:
            kind = ChangeKind.FIELD_ADDED if field_label == "field" else ChangeKind.PROP_ADDED
            changes.append(SchemaChange(
                kind=kind,
                path=f"{parent_path}.{field_label}:{new_field['name']}",
                new_value=field_id,
                message=f"Field '{new_field['name']}' (field_id={field_id}) added"
            ))
        else:
            old_field = old_fields[field_id]
            changes.extend(_check_field_diff(
                old_field, new_field,
                parent_path=parent_path,
                field_label=field_label
            ))

    return changes


def _check_field_diff(
    old_field: dict,
    new_field: dict,
    parent_path: str,
    field_label: str,
) -> List[SchemaChange]:
    """Check differences between two versions of a field."""
    changes: List[SchemaChange] = []
    path = f"{parent_path}.{field_label}:{old_field['name']}"

    # Check kind change (breaking)
    if old_field["kind"] != new_field["kind"]:
        changes.append(SchemaChange(
            kind=ChangeKind.FIELD_KIND_CHANGED,
            path=path,
            old_value=old_field["kind"],
            new_value=new_field["kind"],
            message=f"Field kind changed from '{old_field['kind']}' to '{new_field['kind']}'"
        ))

    # Check name change (allowed)
    if old_field["name"] != new_field["name"]:
        changes.append(SchemaChange(
            kind=ChangeKind.NAME_CHANGED,
            path=path,
            old_value=old_field["name"],
            new_value=new_field["name"],
            message=f"Field renamed from '{old_field['name']}' to '{new_field['name']}'"
        ))

    # Check required change (making optional -> required is breaking)
    old_required = old_field.get("required", False)
    new_required = new_field.get("required", False)
    if not old_required and new_required:
        changes.append(SchemaChange(
            kind=ChangeKind.REQUIRED_ADDED,
            path=path,
            message=f"Field '{old_field['name']}' changed from optional to required"
        ))

    # Check deprecation
    old_deprecated = old_field.get("deprecated", False)
    new_deprecated = new_field.get("deprecated", False)
    if not old_deprecated and new_deprecated:
        changes.append(SchemaChange(
            kind=ChangeKind.FIELD_DEPRECATED,
            path=path,
            message=f"Field '{old_field['name']}' deprecated"
        ))

    # Check enum values
    old_enum = old_field.get("enum_values", [])
    new_enum = new_field.get("enum_values", [])
    if old_enum or new_enum:
        changes.extend(_check_enum_values(old_enum, new_enum, path))

    # Check indexed added (allowed)
    if not old_field.get("indexed") and new_field.get("indexed"):
        changes.append(SchemaChange(
            kind=ChangeKind.INDEX_ADDED,
            path=path,
            message=f"Index added to field '{old_field['name']}'"
        ))

    # Check searchable added (allowed)
    if not old_field.get("searchable") and new_field.get("searchable"):
        changes.append(SchemaChange(
            kind=ChangeKind.SEARCHABLE_ADDED,
            path=path,
            message=f"Searchable added to field '{old_field['name']}'"
        ))

    return changes


def _check_enum_values(
    old_values: List[str],
    new_values: List[str],
    path: str,
) -> List[SchemaChange]:
    """Check enum value changes."""
    changes: List[SchemaChange] = []

    # Check for removed values (breaking)
    old_set = set(old_values)
    new_set = set(new_values)
    removed = old_set - new_set
    for value in removed:
        changes.append(SchemaChange(
            kind=ChangeKind.ENUM_VALUE_REMOVED,
            path=path,
            old_value=value,
            message=f"Enum value '{value}' was removed"
        ))

    # Check for added values (allowed)
    added = new_set - old_set
    for value in added:
        changes.append(SchemaChange(
            kind=ChangeKind.ENUM_VALUE_ADDED,
            path=path,
            new_value=value,
            message=f"Enum value '{value}' was added"
        ))

    # Check for reordering (breaking - affects wire format in some protocols)
    # Only check if no values were added/removed
    if not removed and not added and old_values != new_values:
        changes.append(SchemaChange(
            kind=ChangeKind.ENUM_VALUE_REORDERED,
            path=path,
            old_value=old_values,
            new_value=new_values,
            message="Enum values were reordered"
        ))

    return changes


def _check_id_reuse(
    old_nodes: Dict[int, dict],
    new_nodes: Dict[int, dict],
    old_edges: Dict[int, dict],
    new_edges: Dict[int, dict],
) -> List[SchemaChange]:
    """Check for ID reuse (same ID, different type name)."""
    changes: List[SchemaChange] = []

    # Check node type_id reuse
    for type_id, old_node in old_nodes.items():
        if type_id in new_nodes:
            new_node = new_nodes[type_id]
            # If deprecated in old, but different name in new -> reuse
            if old_node.get("deprecated"):
                # Allow if same base name (just renamed)
                if old_node["name"].lower() != new_node["name"].lower():
                    changes.append(SchemaChange(
                        kind=ChangeKind.TYPE_ID_REUSED,
                        path=f"NodeType:{new_node['name']}",
                        old_value=old_node["name"],
                        new_value=new_node["name"],
                        message=f"type_id {type_id} was deprecated as '{old_node['name']}' but reused for '{new_node['name']}'"
                    ))

    # Check edge_id reuse
    for edge_id, old_edge in old_edges.items():
        if edge_id in new_edges:
            new_edge = new_edges[edge_id]
            if old_edge.get("deprecated"):
                if old_edge["name"].lower() != new_edge["name"].lower():
                    changes.append(SchemaChange(
                        kind=ChangeKind.EDGE_ID_REUSED,
                        path=f"EdgeType:{new_edge['name']}",
                        old_value=old_edge["name"],
                        new_value=new_edge["name"],
                        message=f"edge_id {edge_id} was deprecated as '{old_edge['name']}' but reused for '{new_edge['name']}'"
                    ))

    return changes


def generate_fingerprint(registry: SchemaRegistry) -> str:
    """Generate a schema fingerprint from a registry.

    The fingerprint is a SHA-256 hash of the canonical schema
    representation. It changes when the schema changes.

    Args:
        registry: The schema registry to fingerprint

    Returns:
        Fingerprint string in format 'sha256:<hash>'
    """
    schema_dict = registry.to_dict()
    canonical = json.dumps(schema_dict, sort_keys=True, separators=(',', ':'))
    hash_bytes = hashlib.sha256(canonical.encode('utf-8')).hexdigest()
    return f"sha256:{hash_bytes}"


def validate_breaking_changes(
    old_registry: SchemaRegistry,
    new_registry: SchemaRegistry,
) -> None:
    """Validate that there are no breaking changes.

    This is a convenience function for CI/CD pipelines.

    Args:
        old_registry: Baseline schema
        new_registry: New schema

    Raises:
        CompatibilityError: If breaking changes are detected
    """
    changes = check_compatibility(old_registry, new_registry)
    breaking = [c for c in changes if c.is_breaking]
    if breaking:
        raise CompatibilityError(breaking)
    logger.info(f"Schema compatibility check passed with {len(changes)} non-breaking changes")
