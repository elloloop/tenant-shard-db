"""
Async schema observer for EntDB.

Observes field names and types from node/edge payloads at write time,
buffers observations in memory, and flushes to per-tenant SQLite
asynchronously. This ensures write performance is not impacted.
"""

from __future__ import annotations

import asyncio
import logging
import time
from typing import TYPE_CHECKING, Any

from ..schema.observer import (
    compute_fields_fingerprint,
    merge_field_sets,
    observe_fields,
)

if TYPE_CHECKING:
    from ..schema.registry import SchemaRegistry
    from .canonical_store import CanonicalStore

logger = logging.getLogger(__name__)


class SchemaObserver:
    """Observes schema from data writes and persists to SQLite.

    The observe_node() and observe_edge() methods are synchronous
    dict operations (~5us, no I/O). SQLite writes happen in a
    background flush loop, batched by interval or buffer size.
    """

    def __init__(
        self,
        canonical_store: CanonicalStore,
        schema_registry: SchemaRegistry | None = None,
        flush_interval_seconds: float = 5.0,
        max_buffer_size: int = 100,
    ) -> None:
        self._canonical_store = canonical_store
        self._schema_registry = schema_registry
        self._flush_interval = flush_interval_seconds
        self._max_buffer_size = max_buffer_size

        # Buffer: (tenant_id, type_id) -> {"name": str, "fields": {name: kind}, "count": int}
        self._node_buffer: dict[tuple[str, int], dict[str, Any]] = {}
        # Buffer: (tenant_id, edge_type_id) -> {"name": str, "from_type_id": int|None,
        #          "to_type_id": int|None, "props": {name: kind}, "count": int}
        self._edge_buffer: dict[tuple[str, int], dict[str, Any]] = {}

        self._buffer_count = 0
        self._running = False
        self._task: asyncio.Task | None = None
        self._flush_event = asyncio.Event()

    def observe_node(self, tenant_id: str, type_id: int, payload: dict[str, Any]) -> None:
        """Record a node observation. Synchronous, no I/O.

        Args:
            tenant_id: Tenant identifier.
            type_id: Node type ID.
            payload: Node payload dict.
        """
        if not payload:
            return

        key = (tenant_id, type_id)
        existing = self._node_buffer.get(key)

        # Resolve type name
        name = self._resolve_node_name(type_id)

        if existing:
            # Merge fields
            for field_name, value in payload.items():
                from ..schema.observer import infer_field_kind

                kind = infer_field_kind(value)
                if field_name in existing["fields"]:
                    if existing["fields"][field_name] != kind:
                        existing["fields"][field_name] = "json"
                else:
                    existing["fields"][field_name] = kind
            existing["count"] += 1
        else:
            from ..schema.observer import infer_field_kind

            fields = {fname: infer_field_kind(payload[fname]) for fname in payload}
            self._node_buffer[key] = {"name": name, "fields": fields, "count": 1}

        self._buffer_count += 1
        if self._buffer_count >= self._max_buffer_size:
            self._flush_event.set()

    def observe_edge(
        self,
        tenant_id: str,
        edge_type_id: int,
        from_node_id: str,
        to_node_id: str,
        props: dict[str, Any],
        from_type_id: int | None = None,
        to_type_id: int | None = None,
    ) -> None:
        """Record an edge observation. Synchronous, no I/O.

        Args:
            tenant_id: Tenant identifier.
            edge_type_id: Edge type ID.
            from_node_id: Source node ID.
            to_node_id: Target node ID.
            props: Edge properties dict.
            from_type_id: Source node type ID (if known).
            to_type_id: Target node type ID (if known).
        """
        key = (tenant_id, edge_type_id)
        existing = self._edge_buffer.get(key)

        name = self._resolve_edge_name(edge_type_id)

        if existing:
            for prop_name, value in props.items():
                from ..schema.observer import infer_field_kind

                kind = infer_field_kind(value)
                if prop_name in existing["props"]:
                    if existing["props"][prop_name] != kind:
                        existing["props"][prop_name] = "json"
                else:
                    existing["props"][prop_name] = kind
            # Update type IDs if newly provided
            if from_type_id is not None:
                existing["from_type_id"] = from_type_id
            if to_type_id is not None:
                existing["to_type_id"] = to_type_id
            existing["count"] += 1
        else:
            from ..schema.observer import infer_field_kind

            prop_fields = {pname: infer_field_kind(props[pname]) for pname in props}
            self._edge_buffer[key] = {
                "name": name,
                "from_type_id": from_type_id,
                "to_type_id": to_type_id,
                "props": prop_fields,
                "count": 1,
            }

        self._buffer_count += 1
        if self._buffer_count >= self._max_buffer_size:
            self._flush_event.set()

    async def start(self) -> None:
        """Start the background flush loop."""
        if self._running:
            return
        self._running = True
        self._task = asyncio.create_task(self._flush_loop())
        logger.info("SchemaObserver started")

    async def stop(self) -> None:
        """Stop the observer and perform a final flush."""
        if not self._running:
            return
        self._running = False
        self._flush_event.set()
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        # Final flush
        await self._flush()
        logger.info("SchemaObserver stopped")

    async def _flush_loop(self) -> None:
        """Background loop that flushes buffered observations."""
        try:
            while self._running:
                try:
                    await asyncio.wait_for(
                        self._flush_event.wait(),
                        timeout=self._flush_interval,
                    )
                except asyncio.TimeoutError:
                    pass

                self._flush_event.clear()
                if self._buffer_count > 0:
                    await self._flush()
        except asyncio.CancelledError:
            pass

    async def _flush(self) -> None:
        """Batch write buffered observations to SQLite."""
        # Swap out buffers atomically
        node_buf = self._node_buffer
        edge_buf = self._edge_buffer
        self._node_buffer = {}
        self._edge_buffer = {}
        self._buffer_count = 0

        ts = int(time.time() * 1000)

        for (tenant_id, type_id), entry in node_buf.items():
            try:
                # Build field defs from the buffer's name->kind map
                fields = []
                for idx, fname in enumerate(sorted(entry["fields"].keys()), start=1):
                    fields.append({
                        "field_id": idx,
                        "name": fname,
                        "kind": entry["fields"][fname],
                    })

                # Try to merge with existing observed schema
                try:
                    existing_schema = await self._canonical_store.get_observed_schema(tenant_id)
                    existing_type = next(
                        (t for t in existing_schema.get("node_types", [])
                         if t["type_id"] == type_id),
                        None,
                    )
                    if existing_type:
                        fields = merge_field_sets(existing_type.get("fields", []), fields)
                except Exception:
                    pass

                fingerprint = compute_fields_fingerprint(fields)
                await self._canonical_store.upsert_observed_node_type(
                    tenant_id=tenant_id,
                    type_id=type_id,
                    name=entry["name"],
                    fields=fields,
                    fingerprint=fingerprint,
                    ts=ts,
                )
            except Exception as e:
                logger.warning(f"Failed to flush observed node type {type_id}: {e}")

        for (tenant_id, edge_type_id), entry in edge_buf.items():
            try:
                props = []
                for idx, pname in enumerate(sorted(entry["props"].keys()), start=1):
                    props.append({
                        "field_id": idx,
                        "name": pname,
                        "kind": entry["props"][pname],
                    })

                try:
                    existing_schema = await self._canonical_store.get_observed_schema(tenant_id)
                    existing_edge = next(
                        (e for e in existing_schema.get("edge_types", [])
                         if e["edge_id"] == edge_type_id),
                        None,
                    )
                    if existing_edge:
                        props = merge_field_sets(existing_edge.get("props", []), props)
                except Exception:
                    pass

                fingerprint = compute_fields_fingerprint(props)
                await self._canonical_store.upsert_observed_edge_type(
                    tenant_id=tenant_id,
                    edge_type_id=edge_type_id,
                    name=entry["name"],
                    from_type_id=entry.get("from_type_id"),
                    to_type_id=entry.get("to_type_id"),
                    props=props,
                    fingerprint=fingerprint,
                    ts=ts,
                )
            except Exception as e:
                logger.warning(f"Failed to flush observed edge type {edge_type_id}: {e}")

        if node_buf or edge_buf:
            logger.debug(
                f"Flushed {len(node_buf)} node types, {len(edge_buf)} edge types"
            )

    def _resolve_node_name(self, type_id: int) -> str:
        """Get node type name from registry or generate a default."""
        if self._schema_registry:
            node_type = self._schema_registry.get_node_type(type_id)
            if node_type:
                return node_type.name
        return f"Type_{type_id}"

    def _resolve_edge_name(self, edge_type_id: int) -> str:
        """Get edge type name from registry or generate a default."""
        if self._schema_registry:
            edge_type = self._schema_registry.get_edge_type(edge_type_id)
            if edge_type:
                return edge_type.name
        return f"Edge_{edge_type_id}"
