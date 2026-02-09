"""
Schema inference utilities for observing field types from data writes.

Pure functions with no I/O - fully testable. These are used by the
SchemaObserver to infer schema from node/edge payloads at write time.
"""

from __future__ import annotations

import hashlib
import json
from typing import Any


def infer_field_kind(value: Any) -> str:
    """Map a Python value to a FieldKind string.

    Checks bool before int since bool is a subclass of int.

    Args:
        value: Any Python value from a payload field.

    Returns:
        FieldKind string (e.g. "str", "int", "float", "bool", "json").
    """
    if value is None:
        return "json"
    if isinstance(value, bool):
        return "bool"
    if isinstance(value, int):
        return "int"
    if isinstance(value, float):
        return "float"
    if isinstance(value, str):
        return "str"
    if isinstance(value, list):
        if value and all(isinstance(v, str) for v in value):
            return "list_str"
        if value and all(isinstance(v, int) and not isinstance(v, bool) for v in value):
            return "list_int"
        return "json"
    return "json"


def observe_fields(payload: dict[str, Any]) -> list[dict[str, Any]]:
    """Convert a payload dict into sorted field definitions.

    Field IDs are assigned alphabetically by name for determinism.

    Args:
        payload: Node/edge payload dictionary.

    Returns:
        List of field dicts: [{"field_id": N, "name": "...", "kind": "..."}]
    """
    fields = []
    for idx, name in enumerate(sorted(payload.keys()), start=1):
        fields.append({
            "field_id": idx,
            "name": name,
            "kind": infer_field_kind(payload[name]),
        })
    return fields


def compute_fields_fingerprint(fields: list[dict[str, Any]]) -> str:
    """Compute a SHA-256 fingerprint of field names + kinds for dedup.

    Args:
        fields: List of field dicts with "name" and "kind" keys.

    Returns:
        Hex SHA-256 digest string.
    """
    canonical = json.dumps(
        [{"name": f["name"], "kind": f["kind"]} for f in sorted(fields, key=lambda f: f["name"])],
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def merge_field_sets(
    existing: list[dict[str, Any]],
    new_fields: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    """Union two field sets. Conflicting kinds widen to "json".

    Args:
        existing: Current field definitions.
        new_fields: Newly observed field definitions.

    Returns:
        Merged field definitions sorted by name with reassigned IDs.
    """
    by_name: dict[str, str] = {}
    for f in existing:
        by_name[f["name"]] = f["kind"]
    for f in new_fields:
        name = f["name"]
        kind = f["kind"]
        if name in by_name:
            if by_name[name] != kind:
                by_name[name] = "json"
        else:
            by_name[name] = kind

    merged = []
    for idx, name in enumerate(sorted(by_name.keys()), start=1):
        merged.append({
            "field_id": idx,
            "name": name,
            "kind": by_name[name],
        })
    return merged


def merge_schemas(
    registry_schema: dict[str, Any],
    observed_schema: dict[str, Any],
) -> dict[str, Any]:
    """Merge a SCHEMA_FILE schema with observed schema.

    Registry wins for names/types that exist in both; observed supplements
    with extra fields and types not present in the registry.

    Args:
        registry_schema: Schema from SchemaRegistry.to_dict().
        observed_schema: Schema from get_observed_schema().

    Returns:
        Merged schema dict in the same format as SchemaRegistry.to_dict().
    """
    # Index registry types by type_id / edge_id
    reg_nodes = {t["type_id"]: t for t in registry_schema.get("node_types", [])}
    reg_edges = {e["edge_id"]: e for e in registry_schema.get("edge_types", [])}

    obs_nodes = {t["type_id"]: t for t in observed_schema.get("node_types", [])}
    obs_edges = {e["edge_id"]: e for e in observed_schema.get("edge_types", [])}

    # Merge node types
    merged_nodes: dict[int, dict] = {}
    for tid, reg_type in reg_nodes.items():
        merged = dict(reg_type)
        if tid in obs_nodes:
            # Supplement registry fields with observed fields
            reg_field_names = {f["name"] for f in merged.get("fields", [])}
            extra_fields = [
                f for f in obs_nodes[tid].get("fields", [])
                if f["name"] not in reg_field_names
            ]
            if extra_fields:
                # Reassign field_ids for extras starting after existing max
                max_id = max((f["field_id"] for f in merged.get("fields", [])), default=0)
                all_fields = list(merged.get("fields", []))
                for f in extra_fields:
                    max_id += 1
                    all_fields.append({**f, "field_id": max_id})
                merged["fields"] = all_fields
        merged_nodes[tid] = merged

    # Add observed-only node types
    for tid, obs_type in obs_nodes.items():
        if tid not in merged_nodes:
            merged_nodes[tid] = obs_type

    # Merge edge types
    merged_edges: dict[int, dict] = {}
    for eid, reg_edge in reg_edges.items():
        merged = dict(reg_edge)
        if eid in obs_edges:
            reg_prop_names = {p["name"] for p in merged.get("props", [])}
            extra_props = [
                p for p in obs_edges[eid].get("props", [])
                if p["name"] not in reg_prop_names
            ]
            if extra_props:
                max_id = max((p["field_id"] for p in merged.get("props", [])), default=0)
                all_props = list(merged.get("props", []))
                for p in extra_props:
                    max_id += 1
                    all_props.append({**p, "field_id": max_id})
                merged["props"] = all_props
        merged_edges[eid] = merged

    for eid, obs_edge in obs_edges.items():
        if eid not in merged_edges:
            merged_edges[eid] = obs_edge

    return {
        "node_types": [merged_nodes[tid] for tid in sorted(merged_nodes.keys())],
        "edge_types": [merged_edges[eid] for eid in sorted(merged_edges.keys())],
    }
