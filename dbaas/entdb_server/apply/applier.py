"""
Event applier for EntDB.

The Applier consumes TransactionEvents from the WAL stream and applies
them to the SQLite stores (canonical and mailbox). It ensures:
- Idempotent processing (events are never applied twice)
- Atomic application (all ops in an event succeed or fail together)
- Ordered processing within each tenant

Invariants:
    - Events are processed in stream order
    - Idempotency is checked before any modifications
    - Failed events are logged but don't block processing
    - Mailbox fanout happens after canonical store update

How to change safely:
    - Add new operation types with backward-compatible handling
    - Test idempotency with duplicate event injection
    - Monitor applier lag in production
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
from dataclasses import dataclass, field
from typing import Any

from ..wal.base import StreamPos, StreamRecord, WalStream
from .acl import AclManager, get_acl_manager
from .canonical_store import CanonicalStore, Edge, Node
from .mailbox_store import MailboxStore

logger = logging.getLogger(__name__)


class ApplierError(Exception):
    """Error during event application."""

    pass


class SchemaFingerprintMismatch(ApplierError):
    """Event schema doesn't match server schema."""

    pass


@dataclass
class TransactionEvent:
    """A transaction event from the WAL stream.

    This is the core data structure for all write operations.
    Each event contains one or more atomic operations.

    Attributes:
        tenant_id: Tenant identifier (required)
        actor: Actor performing the operation (required)
        idempotency_key: Unique key for deduplication (required)
        schema_fingerprint: Schema version hash
        ts_ms: Event timestamp (Unix ms)
        ops: List of operations to apply
        stream_pos: Position in the WAL stream

    Example:
        {
            "tenant_id": "t_123",
            "actor": "user:42",
            "idempotency_key": "uuid-abc-123",
            "schema_fingerprint": "sha256:...",
            "ts_ms": 1730000000000,
            "ops": [
                {"op": "create_node", "type_id": 101, "data": {"title": "Task"}}
            ]
        }
    """

    tenant_id: str
    actor: str
    idempotency_key: str
    schema_fingerprint: str | None
    ts_ms: int
    ops: list[dict[str, Any]]
    stream_pos: StreamPos | None = None

    @classmethod
    def from_dict(
        cls, data: dict[str, Any], stream_pos: StreamPos | None = None
    ) -> TransactionEvent:
        """Create from dictionary representation.

        Args:
            data: Event dictionary
            stream_pos: Optional stream position

        Returns:
            TransactionEvent instance

        Raises:
            ValueError: If required fields are missing
        """
        required = ["tenant_id", "actor", "idempotency_key", "ops"]
        missing = [f for f in required if f not in data]
        if missing:
            raise ValueError(f"Missing required fields: {missing}")

        return cls(
            tenant_id=data["tenant_id"],
            actor=data["actor"],
            idempotency_key=data["idempotency_key"],
            schema_fingerprint=data.get("schema_fingerprint"),
            ts_ms=data.get("ts_ms", int(time.time() * 1000)),
            ops=data["ops"],
            stream_pos=stream_pos,
        )


@dataclass
class ApplyResult:
    """Result of applying a transaction event.

    Attributes:
        success: Whether the event was applied
        event: The transaction event
        created_nodes: IDs of created nodes
        created_edges: Created edge tuples (type_id, from, to)
        error: Error message if failed
        skipped: Whether the event was skipped (already applied)
    """

    success: bool
    event: TransactionEvent
    created_nodes: list[str] = field(default_factory=list)
    created_edges: list[tuple] = field(default_factory=list)
    error: str | None = None
    skipped: bool = False


@dataclass
class MailboxFanoutConfig:
    """Configuration for mailbox fanout.

    Attributes:
        enabled: Whether mailbox fanout is enabled
        node_types: Node types that trigger fanout
        recipient_extractor: Function to extract recipients from node
    """

    enabled: bool = True
    node_types: set[int] = field(default_factory=set)


class Applier:
    """Consumes WAL events and applies them to SQLite stores.

    The Applier is the core processing loop that:
    1. Consumes events from the WAL stream
    2. Checks idempotency (skip already-applied events)
    3. Applies operations to the canonical store
    4. Triggers mailbox fanout for relevant nodes
    5. Commits the stream position

    Thread safety:
        The Applier is designed to run as a single task.
        Multiple instances can run for different tenants.

    Example:
        >>> applier = Applier(wal, canonical_store, mailbox_store)
        >>> await applier.start()  # Runs until stopped
    """

    def __init__(
        self,
        wal: WalStream,
        canonical_store: CanonicalStore,
        mailbox_store: MailboxStore,
        topic: str = "entdb-wal",
        group_id: str = "entdb-applier",
        schema_fingerprint: str | None = None,
        acl_manager: AclManager | None = None,
        fanout_config: MailboxFanoutConfig | None = None,
    ) -> None:
        """Initialize the applier.

        Args:
            wal: WAL stream to consume from
            canonical_store: Tenant SQLite store
            mailbox_store: Mailbox SQLite store
            topic: WAL topic name
            group_id: Consumer group ID
            schema_fingerprint: Expected schema fingerprint
            acl_manager: ACL manager instance
            fanout_config: Mailbox fanout configuration
        """
        self.wal = wal
        self.canonical_store = canonical_store
        self.mailbox_store = mailbox_store
        self.topic = topic
        self.group_id = group_id
        self.schema_fingerprint = schema_fingerprint
        self.acl_manager = acl_manager or get_acl_manager()
        self.fanout_config = fanout_config or MailboxFanoutConfig()

        self._running = False
        self._processed_count = 0
        self._error_count = 0
        self._last_position: StreamPos | None = None
        self._node_alias_map: dict[str, str] = {}  # For $ref resolution

    async def start(self) -> None:
        """Start the applier loop.

        This runs until stop() is called, consuming and applying events.
        """
        if self._running:
            logger.warning("Applier already running")
            return

        self._running = True
        logger.info("Starting applier", extra={"topic": self.topic, "group_id": self.group_id})

        try:
            async for record in self.wal.subscribe(self.topic, self.group_id):  # type: ignore[attr-defined]
                if not self._running:
                    break

                result = await self._process_record(record)

                if result.success and not result.skipped:
                    self._processed_count += 1
                    logger.debug(
                        "Applied event",
                        extra={
                            "tenant_id": result.event.tenant_id,
                            "idempotency_key": result.event.idempotency_key,
                            "nodes": len(result.created_nodes),
                            "edges": len(result.created_edges),
                        },
                    )
                elif result.skipped:
                    logger.debug(
                        "Skipped duplicate event",
                        extra={
                            "tenant_id": result.event.tenant_id,
                            "idempotency_key": result.event.idempotency_key,
                        },
                    )
                else:
                    self._error_count += 1
                    logger.error(
                        "Failed to apply event",
                        extra={
                            "tenant_id": result.event.tenant_id,
                            "idempotency_key": result.event.idempotency_key,
                            "error": result.error,
                        },
                    )

                # Commit the position
                await self.wal.commit(record)
                self._last_position = record.position

        except asyncio.CancelledError:
            logger.info("Applier cancelled")
        except Exception as e:
            logger.error(f"Applier error: {e}", exc_info=True)
            raise

        finally:
            self._running = False

    async def stop(self) -> None:
        """Stop the applier loop."""
        self._running = False
        logger.info("Stopping applier")

    async def apply_event(self, event: TransactionEvent) -> ApplyResult:
        """Apply a single transaction event.

        This is the core application logic, separate from the stream
        consumption loop for testability.

        Args:
            event: Transaction event to apply

        Returns:
            ApplyResult indicating success/failure
        """
        # Ensure tenant exists
        if not await self.canonical_store.tenant_exists(event.tenant_id):
            await self.canonical_store.initialize_tenant(event.tenant_id)

        # Check idempotency
        if await self.canonical_store.check_idempotency(event.tenant_id, event.idempotency_key):
            return ApplyResult(
                success=True,
                event=event,
                skipped=True,
            )

        # Check schema fingerprint if required
        if self.schema_fingerprint and event.schema_fingerprint:
            if event.schema_fingerprint != self.schema_fingerprint:
                return ApplyResult(
                    success=False,
                    event=event,
                    error=f"Schema mismatch: expected {self.schema_fingerprint}, got {event.schema_fingerprint}",
                )

        # Apply operations
        try:
            created_nodes = []
            created_edges = []
            self._node_alias_map.clear()

            for op in event.ops:
                op_type = op.get("op")

                if op_type == "create_node":
                    node = await self._apply_create_node(event, op)
                    created_nodes.append(node.node_id)

                    # Store alias for references
                    alias = op.get("as")
                    if alias:
                        self._node_alias_map[alias] = node.node_id

                    # Trigger mailbox fanout if configured
                    if self.fanout_config.enabled:
                        await self._fanout_node(event, node, op)

                elif op_type == "update_node":
                    await self._apply_update_node(event, op)

                elif op_type == "delete_node":
                    await self._apply_delete_node(event, op)

                elif op_type == "create_edge":
                    edge = await self._apply_create_edge(event, op)
                    created_edges.append((edge.edge_type_id, edge.from_node_id, edge.to_node_id))

                elif op_type == "delete_edge":
                    await self._apply_delete_edge(event, op)

                else:
                    logger.warning(f"Unknown operation type: {op_type}")

            # Record the event as applied
            stream_pos_str = str(event.stream_pos) if event.stream_pos else None
            await self.canonical_store.record_applied_event(
                event.tenant_id,
                event.idempotency_key,
                stream_pos_str,
            )

            return ApplyResult(
                success=True,
                event=event,
                created_nodes=created_nodes,
                created_edges=created_edges,
            )

        except Exception as e:
            logger.error(f"Error applying event: {e}", exc_info=True)
            return ApplyResult(
                success=False,
                event=event,
                error=str(e),
            )

    async def _process_record(self, record: StreamRecord) -> ApplyResult:
        """Process a single WAL record.

        Args:
            record: Stream record to process

        Returns:
            ApplyResult
        """
        try:
            data = record.value_json()
            event = TransactionEvent.from_dict(data, record.position)
            return await self.apply_event(event)

        except Exception as e:
            logger.error(f"Error processing record: {e}", exc_info=True)
            # Create a minimal event for error reporting
            try:
                data = json.loads(record.value.decode("utf-8"))
                event = TransactionEvent(
                    tenant_id=data.get("tenant_id", "unknown"),
                    actor=data.get("actor", "unknown"),
                    idempotency_key=data.get("idempotency_key", "unknown"),
                    schema_fingerprint=None,
                    ts_ms=int(time.time() * 1000),
                    ops=[],
                    stream_pos=record.position,
                )
            except Exception:
                event = TransactionEvent(
                    tenant_id="unknown",
                    actor="unknown",
                    idempotency_key="unknown",
                    schema_fingerprint=None,
                    ts_ms=int(time.time() * 1000),
                    ops=[],
                    stream_pos=record.position,
                )

            return ApplyResult(
                success=False,
                event=event,
                error=str(e),
            )

    async def _apply_create_node(
        self,
        event: TransactionEvent,
        op: dict[str, Any],
    ) -> Node:
        """Apply a create_node operation."""
        type_id = op["type_id"]
        data = op.get("data", {})
        acl = op.get("acl", [])
        node_id = op.get("id")

        return await self.canonical_store.create_node(
            tenant_id=event.tenant_id,
            type_id=type_id,
            payload=data,
            owner_actor=event.actor,
            node_id=node_id,
            acl=acl,
            created_at=event.ts_ms,
        )

    async def _apply_update_node(
        self,
        event: TransactionEvent,
        op: dict[str, Any],
    ) -> Node | None:
        """Apply an update_node operation."""
        node_id = self._resolve_ref(op.get("id", ""))
        patch = op.get("patch", {})

        return await self.canonical_store.update_node(
            tenant_id=event.tenant_id,
            node_id=node_id,
            patch=patch,
            updated_at=event.ts_ms,
        )

    async def _apply_delete_node(
        self,
        event: TransactionEvent,
        op: dict[str, Any],
    ) -> bool:
        """Apply a delete_node operation."""
        node_id = self._resolve_ref(op.get("id", ""))

        return await self.canonical_store.delete_node(
            tenant_id=event.tenant_id,
            node_id=node_id,
        )

    async def _apply_create_edge(
        self,
        event: TransactionEvent,
        op: dict[str, Any],
    ) -> Edge:
        """Apply a create_edge operation."""
        edge_type_id = op["edge_id"]
        from_ref = op["from"]
        to_ref = op["to"]
        props = op.get("props", {})

        # Resolve references
        from_node_id = self._resolve_node_ref(from_ref)
        to_node_id = self._resolve_node_ref(to_ref)

        return await self.canonical_store.create_edge(
            tenant_id=event.tenant_id,
            edge_type_id=edge_type_id,
            from_node_id=from_node_id,
            to_node_id=to_node_id,
            props=props,
            created_at=event.ts_ms,
        )

    async def _apply_delete_edge(
        self,
        event: TransactionEvent,
        op: dict[str, Any],
    ) -> bool:
        """Apply a delete_edge operation."""
        edge_type_id = op["edge_id"]
        from_ref = op["from"]
        to_ref = op["to"]

        from_node_id = self._resolve_node_ref(from_ref)
        to_node_id = self._resolve_node_ref(to_ref)

        return await self.canonical_store.delete_edge(
            tenant_id=event.tenant_id,
            edge_type_id=edge_type_id,
            from_node_id=from_node_id,
            to_node_id=to_node_id,
        )

    def _resolve_ref(self, ref: str) -> str:
        """Resolve a reference (possibly an alias)."""
        if ref.startswith("$"):
            # Extract alias name and resolve
            parts = ref[1:].split(".")
            alias = parts[0]
            if alias in self._node_alias_map:
                return self._node_alias_map[alias]
        return ref

    def _resolve_node_ref(self, ref: Any) -> str:
        """Resolve a node reference from various formats.

        Supports:
        - String ID: "node_123"
        - Alias reference: {"ref": "$t.id"}
        - Type+ID: {"type_id": 1, "id": "node_123"}
        """
        if isinstance(ref, str):
            return self._resolve_ref(ref)

        if isinstance(ref, dict):
            # Check for $ref style
            ref_str = ref.get("ref")
            if ref_str:
                return self._resolve_ref(ref_str)

            # Check for type_id + id style
            node_id = ref.get("id")
            if node_id:
                return self._resolve_ref(node_id)

        raise ValueError(f"Invalid node reference: {ref}")

    async def _fanout_node(
        self,
        event: TransactionEvent,
        node: Node,
        op: dict[str, Any],
    ) -> None:
        """Fanout a node to user mailboxes.

        Args:
            event: Source event
            node: Created node
            op: Create operation
        """
        # Get recipients from operation or ACL
        recipients = op.get("fanout_to", [])

        # Also include ACL principals
        for acl_entry in node.acl:
            principal = acl_entry.get("principal", "")
            if principal.startswith("user:"):
                recipients.append(principal)

        # Create mailbox items for each recipient
        for recipient in set(recipients):
            if recipient.startswith("user:"):
                user_id = recipient

                # Generate snippet from payload
                snippet = self._generate_snippet(node.payload)

                try:
                    await self.mailbox_store.add_item(
                        tenant_id=event.tenant_id,
                        user_id=user_id,
                        source_type_id=node.type_id,
                        source_node_id=node.node_id,
                        snippet=snippet,
                        ts=event.ts_ms,
                    )
                except Exception as e:
                    logger.warning(
                        f"Failed to fanout to mailbox: {e}",
                        extra={
                            "tenant_id": event.tenant_id,
                            "user_id": user_id,
                            "node_id": node.node_id,
                        },
                    )

    def _generate_snippet(self, payload: dict[str, Any]) -> str:
        """Generate searchable snippet from payload.

        Extracts text content from common field names.
        """
        snippet_parts = []

        # Common text field names
        text_fields = ["title", "name", "subject", "content", "body", "text", "description"]

        for field_name in text_fields:
            value = payload.get(field_name)
            if isinstance(value, str):
                snippet_parts.append(value)

        return " ".join(snippet_parts)[:1000]  # Limit snippet length

    @property
    def stats(self) -> dict[str, Any]:
        """Get applier statistics."""
        return {
            "running": self._running,
            "processed_count": self._processed_count,
            "error_count": self._error_count,
            "last_position": str(self._last_position) if self._last_position else None,
        }
