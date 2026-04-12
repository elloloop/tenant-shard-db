"""WAL-based audit trail export for compliance.

The WAL is the source of truth for all mutations. This module reads
WAL events from S3 (or any object store) and formats them as
compliance-ready audit reports.
"""

from __future__ import annotations

import json
import logging
from typing import Any

logger = logging.getLogger(__name__)


async def export_audit_trail(
    object_store: Any,
    *,
    prefix: str = "archive",
    tenant_id: str | None = None,
    actor: str | None = None,
    since_ms: int | None = None,
    until_ms: int | None = None,
    format: str = "json",
) -> str:
    """Export WAL events as a compliance-formatted audit trail.

    Reads archived WAL segments from S3 and filters/formats them
    for auditor consumption.

    Args:
        object_store: An S3ObjectStore (or compatible) with ``list_objects``
            and ``get`` methods.
        prefix: S3 key prefix for archived WAL segments.
        tenant_id: Filter to a specific tenant (None = all tenants).
        actor: Filter to a specific actor (None = all actors).
        since_ms: Only include events after this timestamp (Unix ms).
        until_ms: Only include events before this timestamp (Unix ms).
        format: Output format — "json" (default) or "csv".

    Returns:
        Formatted audit trail as a string.
    """
    segments = await object_store.list_objects(prefix)
    entries: list[dict[str, Any]] = []

    for segment in segments:
        try:
            raw = await object_store.get(segment.key)
            _parse_segment(raw, entries, tenant_id, actor, since_ms, until_ms)
        except Exception:
            logger.warning("Failed to read WAL segment %s", segment.key, exc_info=True)

    entries.sort(key=lambda e: e["timestamp_ms"])

    if format == "csv":
        return _format_csv(entries)
    return _format_json(entries)


def _parse_segment(
    raw: bytes,
    entries: list[dict[str, Any]],
    tenant_id: str | None,
    actor: str | None,
    since_ms: int | None,
    until_ms: int | None,
) -> None:
    """Parse a WAL segment and append matching entries."""
    for line in raw.decode("utf-8", errors="replace").splitlines():
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except (json.JSONDecodeError, ValueError):
            continue

        ts = event.get("ts_ms", 0)
        if since_ms and ts < since_ms:
            continue
        if until_ms and ts > until_ms:
            continue
        if tenant_id and event.get("tenant_id") != tenant_id:
            continue
        if actor and event.get("actor") != actor:
            continue

        ops = event.get("ops", [])
        for op in ops:
            op_type = op.get("op", "unknown")
            entries.append(
                {
                    "timestamp_ms": ts,
                    "tenant_id": event.get("tenant_id", ""),
                    "actor": event.get("actor", ""),
                    "idempotency_key": event.get("idempotency_key", ""),
                    "operation": op_type,
                    "type_id": op.get("type_id", ""),
                    "node_id": op.get("id", op.get("as", "")),
                    "schema_fingerprint": event.get("schema_fingerprint", ""),
                }
            )


def _format_json(entries: list[dict[str, Any]]) -> str:
    return json.dumps(entries, indent=2)


def _format_csv(entries: list[dict[str, Any]]) -> str:
    if not entries:
        return ""
    headers = list(entries[0].keys())
    lines = [",".join(headers)]
    for e in entries:
        lines.append(",".join(str(e.get(h, "")) for h in headers))
    return "\n".join(lines)
