"""
Field-ID based payload translation.

EntDB stores node payloads keyed by ``field_id`` (stable numeric identifier)
instead of by field ``name`` (human-readable, may be renamed).  Translation
between the two happens ONLY at the gRPC boundary:

- Ingress (create/update/query filters): translate ``{"title": "x"}`` to
  ``{"1": "x"}`` using the tenant's schema registry.
- Egress (read responses):              translate ``{"1": "x"}`` back to
  ``{"title": "x"}``.

The on-disk payload is therefore stable across schema field renames: the
numeric id never changes, while the name is just a label maintained by the
registry.

Invariants:
    - Stored payload is ALWAYS keyed by stringified ``field_id`` after
      migration.  Legacy name-keyed rows are handled by the idempotent
      migration helper in ``canonical_store.migrate_payloads_to_field_ids``.
    - Unknown field names during ingress are DROPPED (silently) so that
      newer clients sending deprecated fields don't break writes.
    - Unknown id keys during egress are kept as-is (string key) so that
      migrations and forward-compatibility scenarios don't lose data.

How to change safely:
    - Never reuse a retired ``field_id``.  The translation helpers assume
      a 1:1 mapping between id and name, so reusing an id would corrupt
      historical payloads on read.
    - If you add a new field, egress for old rows simply omits it — no
      migration required.

Example:
    >>> from dbaas.entdb_server.schema.field_id_translation import (
    ...     name_to_id_keys, id_to_name_keys,
    ... )
    >>> registry.register_node_type(NodeTypeDef(
    ...     type_id=101, name="Task",
    ...     fields=(field(1, "title", "str"), field(2, "status", "str")),
    ... ))
    >>> name_to_id_keys({"title": "x", "status": "open"}, 101, registry)
    {'1': 'x', '2': 'open'}
    >>> id_to_name_keys({"1": "x", "2": "open"}, 101, registry)
    {'title': 'x', 'status': 'open'}
"""

from __future__ import annotations

import json
import logging
from typing import Any

logger = logging.getLogger(__name__)


def looks_id_keyed(payload: dict[str, Any]) -> bool:
    """Return True when every key in *payload* is a non-empty digit string.

    Used by the migration helper to detect rows that are already stored in
    the new id-keyed format.  Empty payloads count as id-keyed (nothing to
    migrate).

    Args:
        payload: Parsed payload dictionary.

    Returns:
        True when keys look like stringified field ids, False otherwise.
    """
    if not payload:
        return True
    return all(not (not isinstance(key, str) or not key or not key.isdigit()) for key in payload)


def name_to_id_keys(
    payload: dict[str, Any] | None,
    type_id: int,
    registry: Any,
) -> dict[str, Any]:
    """Translate a name-keyed payload to an id-keyed payload.

    Unknown field names are dropped with a debug log.  When the registry
    has no definition for *type_id* (schema-less mode) the payload is
    returned unchanged so that tests and legacy deployments still work.

    Args:
        payload:  Name-keyed payload (may be None/empty).
        type_id:  Node type identifier.
        registry: Schema registry providing ``get_node_type``.

    Returns:
        Dict with ``str(field_id)`` keys, or an empty dict when payload is
        falsy.
    """
    if not payload:
        return {}

    if registry is None or not hasattr(registry, "get_node_type"):
        return dict(payload)

    node_type = registry.get_node_type(type_id)
    if node_type is None:
        # Schema-less: pass payload through untouched so unit tests and
        # data-driven tenants keep working.
        return dict(payload)

    name_to_id: dict[str, int] = {f.name: f.field_id for f in node_type.fields}

    translated: dict[str, Any] = {}
    for key, value in payload.items():
        # Support callers that already passed id-keyed entries (e.g. the
        # filter translation path for numeric fields).  A digit-only string
        # matching a registered field id is left alone.
        if isinstance(key, str) and key.isdigit():
            fid = int(key)
            if any(f.field_id == fid for f in node_type.fields):
                translated[key] = value
                continue

        fid = name_to_id.get(key) if isinstance(key, str) else None
        if fid is None:
            logger.debug(
                "Dropping unknown field '%s' on type_id=%d during ingress",
                key,
                type_id,
            )
            continue
        translated[str(fid)] = value

    return translated


def id_to_name_keys(
    payload: dict[str, Any] | None,
    type_id: int,
    registry: Any,
) -> dict[str, Any]:
    """Translate an id-keyed payload back to a name-keyed payload.

    Unknown id keys are preserved verbatim so that rows written by a
    future schema revision do not lose data on read.  When the registry
    has no definition for *type_id* the payload is returned unchanged.

    Args:
        payload:  Id-keyed payload (may be None/empty).
        type_id:  Node type identifier.
        registry: Schema registry providing ``get_node_type``.

    Returns:
        Dict with human-readable string keys.
    """
    if not payload:
        return {}

    if registry is None or not hasattr(registry, "get_node_type"):
        return dict(payload)

    node_type = registry.get_node_type(type_id)
    if node_type is None:
        return dict(payload)

    id_to_name: dict[int, str] = {f.field_id: f.name for f in node_type.fields}

    translated: dict[str, Any] = {}
    for key, value in payload.items():
        # Legacy/name-keyed fallthrough: if caller passed a name-keyed row
        # (e.g. before migration) we hand it back as-is.
        if not (isinstance(key, str) and key.isdigit()):
            translated[key] = value
            continue

        fid = int(key)
        name = id_to_name.get(fid)
        if name is None:
            # Unknown id (deprecated/forward-compat): keep the raw key
            # to avoid data loss.
            translated[key] = value
            continue
        translated[name] = value

    return translated


def translate_payload_json_to_names(
    payload_json: str | None,
    type_id: int,
    registry: Any,
) -> str:
    """Translate a raw JSON string payload from id-keyed to name-keyed.

    Convenience for egress paths that deal in serialized JSON
    (``Node.payload_json``).  Parses, translates, and re-serializes.

    Args:
        payload_json: Raw JSON string (id-keyed on disk).
        type_id:      Node type identifier.
        registry:     Schema registry instance.

    Returns:
        Re-serialized JSON string with name keys.
    """
    if not payload_json or payload_json == "{}":
        return "{}"
    try:
        parsed = json.loads(payload_json)
    except (ValueError, TypeError):
        logger.warning("Could not parse payload_json for translation: %r", payload_json)
        return payload_json
    if not isinstance(parsed, dict):
        return payload_json
    translated = id_to_name_keys(parsed, type_id, registry)
    return json.dumps(translated)


def translate_filter_name_to_id(
    filter_dict: dict[str, Any] | None,
    type_id: int,
    registry: Any,
) -> dict[str, Any] | None:
    """Translate a name-keyed filter dict into an id-keyed filter dict.

    Used by ``QueryNodes`` before calling ``canonical_store.query_nodes``
    so that the in-memory filter compares against the on-disk id keys.

    Args:
        filter_dict: Name-keyed filter (e.g. ``{"status": "open"}``).
        type_id:     Node type identifier.
        registry:    Schema registry instance.

    Returns:
        Id-keyed filter dict (e.g. ``{"2": "open"}``) or None when the
        input is None.  Unknown names are dropped.
    """
    if filter_dict is None:
        return None
    return name_to_id_keys(filter_dict, type_id, registry)
