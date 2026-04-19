"""
Event applier for EntDB.

The Applier consumes TransactionEvents from the WAL stream and applies
them to the canonical per-tenant SQLite store. It ensures:
- Idempotent processing (events are never applied twice)
- Atomic application (all ops in an event succeed or fail together)
- Ordered processing within each tenant

Invariants:
    - Events are processed in stream order
    - Idempotency is checked before any modifications
    - Failed events are logged but don't block processing
    - Notification fanout happens after canonical store update

How to change safely:
    - Add new operation types with backward-compatible handling
    - Test idempotency with duplicate event injection
    - Monitor applier lag in production
"""

from __future__ import annotations

import asyncio
import json
import logging
import re
import sqlite3
import time
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from typing import Any

from ..data_policy import REQUIRES_LEGAL_BASIS
from ..global_store import GlobalStore
from ..metrics import record_applier_event
from ..schema.registry import get_registry
from ..wal.base import StreamPos, StreamRecord, WalStream
from .acl import AclManager, get_acl_manager
from .canonical_store import CanonicalStore, Edge, Node

logger = logging.getLogger(__name__)


class ApplierError(Exception):
    """Error during event application."""

    pass


class SchemaFingerprintMismatch(ApplierError):
    """Event schema doesn't match server schema."""

    pass


class ValidationError(ApplierError):
    """Event is structurally invalid (rejected before any DB write).

    Raised when:
    - an event mixes storage modes across its ops,
    - ``update_node`` tries to change an immutable ``storage_mode``,
    - ``USER_MAILBOX`` ops lack a ``target_user_id``,
    - a ``create_edge`` op violates the
      ``USER_MAILBOX -> TENANT -> PUBLIC`` hierarchy.
    """

    pass


_UNIQUE_INDEX_NAME_RE = re.compile(r"idx_unique_t(\d+)_f(\d+)")


def _parse_unique_index_name(msg: str) -> tuple[int, int] | None:
    """Extract ``(type_id, field_id)`` from a SQLite IntegrityError message.

    SQLite formats unique-index violations as::

        UNIQUE constraint failed: index 'idx_unique_t201_f1'

    The applier creates indexes named ``idx_unique_t<type>_f<field>`` so
    this regex round-trips the identifiers used by
    ``CanonicalStore._ensure_unique_indexes``. Returns ``None`` when the
    message doesn't match (e.g. an unrelated integrity error).
    """
    m = _UNIQUE_INDEX_NAME_RE.search(msg)
    if m is None:
        return None
    return (int(m.group(1)), int(m.group(2)))


class UniqueConstraintError(ApplierError):
    """A create/update op violated a declared unique field.

    Raised by the applier when ``INSERT INTO nodes`` trips a unique
    expression index defined via ``(entdb.field).unique = true``.
    The gRPC boundary converts this into ``ALREADY_EXISTS`` with
    structured metadata so SDKs can reconstruct a typed error.
    """

    def __init__(
        self,
        tenant_id: str,
        type_id: int,
        field_id: int,
        value: Any,
        *,
        message: str | None = None,
    ) -> None:
        self.tenant_id = tenant_id
        self.type_id = int(type_id)
        self.field_id = int(field_id)
        self.value = value
        super().__init__(
            message
            or (
                f"Unique constraint violation: tenant={tenant_id} "
                f"type_id={type_id} field_id={field_id} value={value!r} "
                "already exists"
            )
        )


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
        >>> applier = Applier(wal, canonical_store)
        >>> await applier.start()  # Runs until stopped
    """

    def __init__(
        self,
        wal: WalStream,
        canonical_store: CanonicalStore,
        topic: str = "entdb-wal",
        group_id: str = "entdb-applier",
        schema_fingerprint: str | None = None,
        acl_manager: AclManager | None = None,
        fanout_config: MailboxFanoutConfig | None = None,
        batch_size: int = 1,
        poll_timeout_ms: int = 100,
        assigned_tenants: frozenset[str] | None = None,
        global_store: GlobalStore | None = None,
        halt_on_error: bool = True,
    ) -> None:
        """Initialize the applier.

        Args:
            wal: WAL stream to consume from
            canonical_store: Tenant SQLite store
            topic: WAL topic name
            group_id: Consumer group ID
            schema_fingerprint: Expected schema fingerprint
            acl_manager: ACL manager instance
            fanout_config: Mailbox fanout configuration
            global_store: GlobalStore for cross-tenant shared_index
            halt_on_error: If True (default, production), the applier halts
                on any apply failure so the WAL offset is never advanced
                past a failed event. Tests may opt out by passing False.
        """
        self.wal = wal
        self.canonical_store = canonical_store
        self.topic = topic
        self.group_id = group_id
        self.schema_fingerprint = schema_fingerprint
        self.acl_manager = acl_manager or get_acl_manager()
        self.fanout_config = fanout_config or MailboxFanoutConfig()
        self.batch_size = batch_size
        self.poll_timeout_ms = poll_timeout_ms
        self.global_store = global_store
        self.halt_on_error = halt_on_error

        self._assigned_tenants = assigned_tenants or frozenset()
        self._skipped_count = 0

        self._running = False
        self._processed_count = 0
        self._error_count = 0
        self._last_position: StreamPos | None = None
        self._node_alias_map: dict[str, str] = {}  # For $ref resolution

    async def start(self) -> None:
        """Start the applier loop.

        This runs until stop() is called, consuming and applying events.

        When batch_size > 1, uses poll_batch() to fetch multiple records
        at once from the WAL and applies them in a single SQLite transaction.
        This dramatically improves throughput by amortizing the fsync cost.

        When batch_size == 1 (default), uses the original single-record
        subscribe() loop for backward compatibility.
        """
        if self._running:
            logger.warning("Applier already running")
            return

        self._running = True
        logger.info(
            "Starting applier",
            extra={
                "topic": self.topic,
                "group_id": self.group_id,
                "batch_size": self.batch_size,
            },
        )

        try:
            if self.batch_size > 1:
                await self._start_batched()
            else:
                await self._start_single()

        except asyncio.CancelledError:
            logger.info("Applier cancelled")
        except Exception as e:
            logger.error(f"Applier error: {e}", exc_info=True)
            raise

        finally:
            self._running = False

    async def _start_single(self) -> None:
        """Original single-record processing loop."""
        async for record in self.wal.subscribe(self.topic, self.group_id):  # type: ignore[attr-defined]
            if not self._running:
                break

            result = await self._process_record(record)
            self._log_result(result)

            # Bug fix: never commit the WAL offset for a failed event.
            # On failure, halt the applier so the same offset is redelivered.
            if not result.success:
                logger.error(
                    "Applier halting on failed event — WAL offset NOT advanced",
                    extra={
                        "tenant_id": result.event.tenant_id,
                        "idempotency_key": result.event.idempotency_key,
                        "error": result.error,
                        "position": str(record.position) if record.position else None,
                    },
                )
                if self.halt_on_error:
                    self._running = False
                    return
                # halt_on_error=False: skip commit but continue processing.
                continue

            # Notify offset waiters
            if not result.skipped and record.position is not None:
                await self.canonical_store.update_applied_offset(
                    result.event.tenant_id, str(record.position)
                )

            # Commit the position (only on success)
            await self.wal.commit(record)
            self._last_position = record.position

    async def _start_batched(self) -> None:
        """Batch processing loop using poll_batch().

        Fetches whatever records are available (up to batch_size),
        groups them by tenant, applies each tenant's events in a single
        SQLite transaction, then commits the last record's position.
        """
        while self._running:
            records = await self.wal.poll_batch(  # type: ignore[attr-defined]
                topic=self.topic,
                group_id=self.group_id,
                max_records=self.batch_size,
                timeout_ms=self.poll_timeout_ms,
            )

            if not records:
                continue

            # Group records by tenant for per-tenant batch transactions
            tenant_records: dict[str, list] = {}
            for record in records:
                if not self._running:
                    break
                try:
                    data = record.value_json()
                    tenant_id = data.get("tenant_id", "")

                    # Skip events for unassigned tenants
                    if self._assigned_tenants and tenant_id not in self._assigned_tenants:
                        self._skipped_count += 1
                        continue

                    if tenant_id not in tenant_records:
                        tenant_records[tenant_id] = []
                    tenant_records[tenant_id].append((record, data))
                except Exception as e:
                    logger.error(f"Error parsing record: {e}", exc_info=True)
                    self._error_count += 1

            # Track which records failed so we can avoid committing
            # any partition that contained a failed record.
            failed_record_ids: set[int] = set()
            halt_after_batch = False

            # Apply each tenant's batch in a single transaction
            for tenant_id, items in tenant_records.items():
                try:
                    await self._apply_tenant_batch(tenant_id, items)
                except Exception as e:
                    logger.error(
                        f"Error applying batch for {tenant_id}: {e}",
                        exc_info=True,
                    )
                    # Fall back to individual processing. Halt at the
                    # first failure so we don't advance past a bad event.
                    fallback_failed = False
                    for record, _data in items:
                        if fallback_failed:
                            # Mark the rest of this tenant's records as
                            # failed so their partitions are not committed.
                            failed_record_ids.add(id(record))
                            continue
                        result = await self._process_record(record)
                        self._log_result(result)
                        if not result.success:
                            fallback_failed = True
                            failed_record_ids.add(id(record))
                            logger.error(
                                "Applier halting in fallback path — WAL offset NOT advanced",
                                extra={
                                    "tenant_id": tenant_id,
                                    "idempotency_key": (result.event.idempotency_key),
                                    "error": result.error,
                                },
                            )
                            if self.halt_on_error:
                                halt_after_batch = True
                    if fallback_failed and self.halt_on_error:
                        break

            # Commit the last record for EACH partition to avoid
            # re-delivering records from non-last partitions on next poll.
            # CRITICAL: only commit a partition if EVERY record from that
            # partition in this batch succeeded. If any record in a
            # partition failed, skip that partition's commit so it is
            # redelivered.
            if records:
                # Collect failed partitions
                failed_partitions: set[int] = set()
                for record in records:
                    if id(record) in failed_record_ids:
                        failed_partitions.add(record.position.partition)

                per_partition: dict[int, StreamRecord] = {}
                for record in records:
                    p = record.position.partition
                    if p in failed_partitions:
                        continue
                    per_partition[p] = record
                for record in per_partition.values():
                    await self.wal.commit(record)
                if per_partition:
                    self._last_position = records[-1].position

                if len(records) > 1:
                    logger.debug(
                        "Applied batch",
                        extra={
                            "batch_size": len(records),
                            "tenants": len(tenant_records),
                            "last_offset": records[-1].position.offset,
                            "failed_partitions": sorted(failed_partitions),
                        },
                    )

            if halt_after_batch:
                logger.error("Applier halting after batch due to apply failures")
                self._running = False
                return

    async def _apply_tenant_batch(self, tenant_id: str, items: list[tuple]) -> None:
        """Apply a batch of events for a single tenant in one transaction.

        The synchronous SQLite work runs inside the canonical_store
        ThreadPoolExecutor so the gRPC asyncio event loop is never
        blocked by a long batch transaction.
        """
        # Ensure tenant exists (uses canonical_store's own thread-pool path)
        if not await self.canonical_store.tenant_exists(tenant_id):
            await self.canonical_store.initialize_tenant(tenant_id)

        loop = asyncio.get_running_loop()
        applied_count, skipped_count, applied_ops_count = await loop.run_in_executor(
            self.canonical_store._executor,
            self._sync_apply_tenant_batch_body,
            tenant_id,
            items,
        )

        self._processed_count += applied_count
        if applied_count > 0:
            # Notify offset waiters with the latest stream position
            last_record = items[-1][0]
            if last_record.position is not None:
                await self.canonical_store.update_applied_offset(
                    tenant_id, str(last_record.position)
                )
            # Phase 1 quota: bump the durable write counter once per
            # batch, covering every op across every event that
            # actually committed. Fire-and-forget.
            await self._increment_usage_safe(tenant_id, applied_ops_count)
            logger.debug(
                "Batch applied",
                extra={
                    "tenant_id": tenant_id,
                    "applied": applied_count,
                    "skipped": skipped_count,
                    "ops": applied_ops_count,
                },
            )

    # ── Storage routing helpers (2026-04-13) ────────────────────────────
    #
    # Every WAL event lives in exactly one physical SQLite file: the
    # tenant.db, a per-user mailbox.db, or the singleton public.db.
    # ``_event_storage_mode`` inspects the create_node ops in the event
    # and returns the single storage mode; mixed-mode events are
    # rejected. ``_open_event_connection`` picks the matching
    # ``batch_transaction`` context manager on the canonical store.
    # ``_build_event_storage_map`` returns the
    # ``{node_id: (mode, user_id)}`` map needed for edge direction
    # validation. These helpers are additive — they don't change the
    # existing tenant-only code path when every op is TENANT.

    def _event_storage_mode(self, event: TransactionEvent) -> tuple[str, str | None]:
        """Return the event's storage mode as ``(mode, target_user_id)``.

        Raises :class:`ValidationError` if the event mixes storage
        modes (different create_node ops declaring different
        ``storage_mode`` / ``target_user_id`` values) or if a
        ``USER_MAILBOX`` op is missing its ``target_user_id``.
        """
        observed_mode: str | None = None
        observed_user: str | None = None

        for op in event.ops:
            if op.get("op") != "create_node":
                continue
            mode = op.get("storage_mode") or "TENANT"
            tgt_user = op.get("target_user_id") or None
            if mode == "USER_MAILBOX" and not tgt_user:
                raise ValidationError("USER_MAILBOX create_node requires target_user_id")
            if mode != "USER_MAILBOX" and tgt_user:
                # target_user_id on a non-mailbox create is ignored
                # but don't let it pollute mode resolution.
                tgt_user = None
            if observed_mode is None:
                observed_mode = mode
                observed_user = tgt_user
            else:
                if mode != observed_mode or tgt_user != observed_user:
                    raise ValidationError(
                        "Event mixes storage modes: "
                        f"{observed_mode}/{observed_user} vs "
                        f"{mode}/{tgt_user}. Every op in a single "
                        "event must target the same physical file."
                    )

        # Events that contain no create_node ops (updates, deletes,
        # edges, admin ops) default to TENANT.
        if observed_mode is None:
            return ("TENANT", None)
        return (observed_mode, observed_user)

    @contextmanager
    def _open_event_connection(self, event: TransactionEvent) -> Iterator[Any]:
        """Open the correct ``batch_transaction`` for this event.

        Returns the same kind of SQLite connection as
        ``canonical_store.batch_transaction`` but picks the physical
        file (tenant / mailbox / public) based on the event's storage
        mode. Raises :class:`ValidationError` for mixed-mode events.
        """
        mode, user_id = self._event_storage_mode(event)
        store = self.canonical_store
        if mode == "TENANT":
            with store.batch_transaction(event.tenant_id) as conn:
                yield conn
        elif mode == "USER_MAILBOX":
            assert user_id is not None  # enforced above
            with store.mailbox_batch_transaction(event.tenant_id, user_id) as conn:
                yield conn
        elif mode == "PUBLIC":
            with store.public_batch_transaction() as conn:
                yield conn
        else:  # pragma: no cover - defensive
            raise ValidationError(f"Unknown storage mode: {mode}")

    def _build_event_storage_map(
        self, event: TransactionEvent
    ) -> dict[str, tuple[str, str | None]]:
        """Build ``{node_id: (storage_mode, target_user_id)}`` for an event.

        Walks every ``create_node`` op and records its declared storage
        mode. Nodes are keyed by ``id`` when present and by their
        ``as`` alias so that edge validation can look up both forms
        before alias resolution has run.
        """
        out: dict[str, tuple[str, str | None]] = {}
        for op in event.ops:
            if op.get("op") != "create_node":
                continue
            mode = op.get("storage_mode") or "TENANT"
            tgt_user = op.get("target_user_id") if mode == "USER_MAILBOX" else None
            nid = op.get("id")
            if nid:
                out[nid] = (mode, tgt_user)
            alias = op.get("as")
            if alias:
                out[alias] = (mode, tgt_user)
        return out

    def _sync_apply_tenant_batch_body(
        self, tenant_id: str, items: list[tuple]
    ) -> tuple[int, int, int]:
        """Synchronous body of a tenant batch apply.

        Runs entirely inside a worker thread. Opens the
        ``batch_transaction`` context manager and executes every op for
        every event in the batch in a single SQLite transaction.

        Returns:
            (applied_count, skipped_count, applied_ops_count)

            ``applied_ops_count`` is the total number of ops across
            events that actually committed (used for the post-apply
            usage-increment hook). Skipped / duplicate events are
            excluded so billing counters do not double-count replays.
        """
        applied_count = 0
        skipped_count = 0
        applied_ops_count = 0

        # Storage routing (2026-04-13): if *any* event in the batch is
        # non-TENANT, fall back to the single-event path so each event
        # lands in the correct physical file. The existing halt-on-error
        # logic in ``apply_event`` re-dispatches each one via
        # ``_open_event_connection``.
        for _, data in items:
            for op in data.get("ops", []):
                if op.get("op") == "create_node" and (
                    op.get("storage_mode") not in (None, "", "TENANT") or op.get("target_user_id")
                ):
                    raise ValidationError(
                        "Batch apply cannot mix tenant + non-tenant storage "
                        "modes; re-dispatch per event."
                    )

        with self.canonical_store.batch_transaction(tenant_id) as conn:
            for record, data in items:
                # NOTE: We intentionally do NOT reject events with a
                # stale schema_fingerprint here. The WAL is an immutable
                # historical log: older events were valid when written
                # and must still be applied on replay (e.g. during a
                # full read-model rebuild after a schema deploy).
                # schema_fingerprint is only enforced on the WRITE path
                # at the gRPC boundary to reject stale clients.

                idempotency_key = data.get("idempotency_key", "")

                # Check idempotency within the transaction
                cursor = conn.execute(
                    "SELECT 1 FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?",
                    (tenant_id, idempotency_key),
                )
                if cursor.fetchone():
                    skipped_count += 1
                    continue

                # Apply operations
                event = TransactionEvent.from_dict(data, record.position)
                self._node_alias_map.clear()
                applied_ops_count += len(event.ops)

                for op in event.ops:
                    op_type = op.get("op")
                    if op_type == "create_node":
                        node = self.canonical_store.create_node_raw(
                            conn,
                            tenant_id=event.tenant_id,
                            type_id=op["type_id"],
                            payload=op.get("data", {}),
                            owner_actor=event.actor,
                            node_id=op.get("id"),
                            acl=op.get("acl", []),
                            created_at=event.ts_ms,
                        )
                        self._log_data_policy(op["type_id"])
                        alias = op.get("as")
                        if alias:
                            self._node_alias_map[alias] = node.node_id
                    elif op_type == "update_node":
                        node_id = self._resolve_ref(op.get("id", ""))
                        patch = op.get("patch", {})
                        cursor = conn.execute(
                            "SELECT payload_json FROM nodes WHERE tenant_id = ? AND node_id = ?",
                            (tenant_id, node_id),
                        )
                        row = cursor.fetchone()
                        if row:
                            existing = json.loads(
                                row[0] if isinstance(row, tuple) else row["payload_json"]
                            )
                            existing.update(patch)
                            conn.execute(
                                "UPDATE nodes SET payload_json = ?, updated_at = ? "
                                "WHERE tenant_id = ? AND node_id = ?",
                                (json.dumps(existing), event.ts_ms, tenant_id, node_id),
                            )
                    elif op_type == "delete_node":
                        node_id = self._resolve_ref(op.get("id", ""))
                        conn.execute(
                            "DELETE FROM edges WHERE tenant_id = ? "
                            "AND (from_node_id = ? OR to_node_id = ?)",
                            (tenant_id, node_id, node_id),
                        )
                        conn.execute(
                            "DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?",
                            (tenant_id, node_id),
                        )
                        conn.execute(
                            "DELETE FROM nodes WHERE tenant_id = ? AND node_id = ?",
                            (tenant_id, node_id),
                        )
                    elif op_type == "create_edge":
                        edge_type_id = op["edge_id"]
                        from_ref = self._resolve_node_ref(op["from"])
                        to_ref = self._resolve_node_ref(op["to"])
                        props = op.get("props", {})
                        propagates_acl = 1 if op.get("propagates_acl", False) else 0
                        conn.execute(
                            "INSERT OR REPLACE INTO edges "
                            "(tenant_id, edge_type_id, from_node_id, to_node_id, "
                            "props_json, propagates_acl, created_at) "
                            "VALUES (?, ?, ?, ?, ?, ?, ?)",
                            (
                                tenant_id,
                                edge_type_id,
                                from_ref,
                                to_ref,
                                json.dumps(props),
                                propagates_acl,
                                event.ts_ms,
                            ),
                        )
                        if propagates_acl:
                            cycle = conn.execute(
                                """
                                WITH RECURSIVE chain(nid, depth) AS (
                                    SELECT ?, 0
                                    UNION ALL
                                    SELECT ai.inherit_from, c.depth + 1
                                    FROM acl_inherit ai
                                    JOIN chain c ON ai.node_id = c.nid
                                    WHERE c.depth < 10
                                )
                                SELECT 1 FROM chain WHERE nid = ? LIMIT 1
                                """,
                                (from_ref, to_ref),
                            ).fetchone()
                            if not cycle:
                                conn.execute(
                                    "INSERT OR IGNORE INTO acl_inherit "
                                    "(node_id, inherit_from) VALUES (?, ?)",
                                    (to_ref, from_ref),
                                )
                    elif op_type == "delete_edge":
                        edge_type_id = op["edge_id"]
                        from_ref = self._resolve_node_ref(op["from"])
                        to_ref = self._resolve_node_ref(op["to"])
                        conn.execute(
                            "DELETE FROM edges WHERE tenant_id = ? AND edge_type_id = ? "
                            "AND from_node_id = ? AND to_node_id = ?",
                            (tenant_id, edge_type_id, from_ref, to_ref),
                        )
                        conn.execute(
                            "DELETE FROM acl_inherit WHERE node_id = ? AND inherit_from = ?",
                            (to_ref, from_ref),
                        )

                # Record applied event within the same transaction
                stream_pos_str = str(event.stream_pos) if event.stream_pos else None
                conn.execute(
                    "INSERT INTO applied_events "
                    "(tenant_id, idempotency_key, stream_pos, applied_at) "
                    "VALUES (?, ?, ?, ?)",
                    (tenant_id, idempotency_key, stream_pos_str, int(time.time() * 1000)),
                )
                applied_count += 1

        return applied_count, skipped_count, applied_ops_count

    def _sync_apply_event_body(
        self, event: TransactionEvent
    ) -> tuple[bool, list[str], list[tuple], list[str]]:
        """Synchronous body of ``apply_event``.

        Runs entirely inside a worker thread. Opens the
        ``batch_transaction`` and applies every op in a single SQLite
        transaction. Returns ``(skipped, created_nodes, created_edges,
        deleted_node_ids)``. Raises on failure so the async wrapper can
        catch and turn it into a failed ``ApplyResult``.
        """
        created_nodes: list[str] = []
        created_edges: list[tuple] = []
        deleted_node_ids: list[str] = []
        self._node_alias_map.clear()
        tenant_id = event.tenant_id

        # Storage routing: pick the physical file (tenant / mailbox /
        # public) for this event. Also precompute the per-node storage
        # map used by the edge direction validator.
        event_storage_map = self._build_event_storage_map(event)

        with self._open_event_connection(event) as conn:
            # Check idempotency within the transaction
            cursor = conn.execute(
                "SELECT 1 FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?",
                (tenant_id, event.idempotency_key),
            )
            if cursor.fetchone():
                # Already applied — empty COMMIT is harmless.
                return (True, [], [], [])

            for op in event.ops:
                op_type = op.get("op")

                if op_type == "create_node":
                    # Lazy field-index creation (2026-04-14 SDK v0.3,
                    # extended 2026-04-19 for non-unique query indexes).
                    # Runs once per ``(db_file, type_id)`` for the life
                    # of the process. The schema registry is the
                    # authority for which fields are unique/indexed.
                    type_id_int = int(op["type_id"])
                    registry = get_registry()
                    unique_fids = registry.get_unique_field_ids(type_id_int)
                    indexed_fids = registry.get_indexed_field_ids(type_id_int)
                    if unique_fids or indexed_fids:
                        self.canonical_store._ensure_field_indexes(
                            conn, tenant_id, type_id_int, unique_fids, indexed_fids
                        )
                    try:
                        node = self.canonical_store.create_node_raw(
                            conn,
                            tenant_id=tenant_id,
                            type_id=op["type_id"],
                            payload=op.get("data", {}),
                            owner_actor=event.actor,
                            node_id=op.get("id"),
                            acl=op.get("acl", []),
                            created_at=event.ts_ms,
                        )
                    except sqlite3.IntegrityError as ie:
                        parsed = _parse_unique_index_name(str(ie))
                        if parsed is not None:
                            dup_type_id, dup_field_id = parsed
                            dup_value = (op.get("data") or {}).get(str(dup_field_id))
                            raise UniqueConstraintError(
                                tenant_id,
                                dup_type_id,
                                dup_field_id,
                                dup_value,
                            ) from ie
                        raise
                    created_nodes.append(node.node_id)
                    self._log_data_policy(op["type_id"])
                    alias = op.get("as")
                    if alias:
                        self._node_alias_map[alias] = node.node_id

                elif op_type == "update_node":
                    # Storage routing (2026-04-13): ``storage_mode`` is
                    # immutable — reject any patch that tries to set it.
                    patch = op.get("patch", {})
                    if isinstance(patch, dict) and "storage_mode" in patch:
                        raise ValidationError(
                            "storage_mode is immutable; it cannot be changed by update_node"
                        )
                    node_id = self._resolve_ref(op.get("id", ""))
                    cursor = conn.execute(
                        "SELECT payload_json FROM nodes WHERE tenant_id = ? AND node_id = ?",
                        (tenant_id, node_id),
                    )
                    row = cursor.fetchone()
                    if row:
                        existing = json.loads(
                            row[0] if isinstance(row, tuple) else row["payload_json"]
                        )
                        existing.update(patch)
                        # Lazy field-index creation so updates that
                        # first touch a newly-declared unique/indexed
                        # field on an existing type still get enforced.
                        type_id_int = int(op.get("type_id", 0) or 0)
                        if type_id_int:
                            registry = get_registry()
                            unique_fids = registry.get_unique_field_ids(type_id_int)
                            indexed_fids = registry.get_indexed_field_ids(type_id_int)
                            if unique_fids or indexed_fids:
                                self.canonical_store._ensure_field_indexes(
                                    conn, tenant_id, type_id_int, unique_fids, indexed_fids
                                )
                        try:
                            conn.execute(
                                "UPDATE nodes SET payload_json = ?, updated_at = ? "
                                "WHERE tenant_id = ? AND node_id = ?",
                                (
                                    json.dumps(existing),
                                    event.ts_ms,
                                    tenant_id,
                                    node_id,
                                ),
                            )
                        except sqlite3.IntegrityError as ie:
                            parsed = _parse_unique_index_name(str(ie))
                            if parsed is not None:
                                dup_type_id, dup_field_id = parsed
                                dup_value = existing.get(str(dup_field_id))
                                raise UniqueConstraintError(
                                    tenant_id,
                                    dup_type_id,
                                    dup_field_id,
                                    dup_value,
                                ) from ie
                            raise

                elif op_type == "delete_node":
                    node_id = self._resolve_ref(op.get("id", ""))
                    conn.execute(
                        "DELETE FROM edges WHERE tenant_id = ? "
                        "AND (from_node_id = ? OR to_node_id = ?)",
                        (tenant_id, node_id, node_id),
                    )
                    conn.execute(
                        "DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?",
                        (tenant_id, node_id),
                    )
                    # 2026-04-14 SDK v0.3: unique values live inside
                    # the ``nodes`` row itself — no cascade table.
                    conn.execute(
                        "DELETE FROM nodes WHERE tenant_id = ? AND node_id = ?",
                        (tenant_id, node_id),
                    )
                    deleted_node_ids.append(node_id)

                elif op_type == "create_edge":
                    edge_type_id = op["edge_id"]
                    from_ref = self._resolve_node_ref(op["from"])
                    to_ref = self._resolve_node_ref(op["to"])
                    # Storage routing (2026-04-13): enforce
                    # ``USER_MAILBOX -> TENANT -> PUBLIC`` hierarchy on
                    # every edge write.
                    try:
                        CanonicalStore._validate_edge_direction(
                            from_ref,
                            to_ref,
                            tenant_id,
                            event_storage_map,
                        )
                    except ValueError as ev:
                        raise ValidationError(str(ev)) from ev
                    props = op.get("props", {})
                    propagates_acl = 1 if op.get("propagates_acl", False) else 0
                    conn.execute(
                        "INSERT OR REPLACE INTO edges "
                        "(tenant_id, edge_type_id, from_node_id, to_node_id, "
                        "props_json, propagates_acl, created_at) "
                        "VALUES (?, ?, ?, ?, ?, ?, ?)",
                        (
                            tenant_id,
                            edge_type_id,
                            from_ref,
                            to_ref,
                            json.dumps(props),
                            propagates_acl,
                            event.ts_ms,
                        ),
                    )
                    # Create ACL inheritance pointer if edge propagates
                    if propagates_acl:
                        # Cycle detection: check if from_ref already
                        # inherits from to_ref
                        cycle = conn.execute(
                            """
                            WITH RECURSIVE chain(nid, depth) AS (
                                SELECT ?, 0
                                UNION ALL
                                SELECT ai.inherit_from, c.depth + 1
                                FROM acl_inherit ai
                                JOIN chain c ON ai.node_id = c.nid
                                WHERE c.depth < 10
                            )
                            SELECT 1 FROM chain WHERE nid = ? LIMIT 1
                            """,
                            (from_ref, to_ref),
                        ).fetchone()
                        if not cycle:
                            conn.execute(
                                "INSERT OR IGNORE INTO acl_inherit "
                                "(node_id, inherit_from) VALUES (?, ?)",
                                (to_ref, from_ref),
                            )
                    created_edges.append((edge_type_id, from_ref, to_ref))

                elif op_type == "delete_edge":
                    edge_type_id = op["edge_id"]
                    from_ref = self._resolve_node_ref(op["from"])
                    to_ref = self._resolve_node_ref(op["to"])
                    conn.execute(
                        "DELETE FROM edges WHERE tenant_id = ? "
                        "AND edge_type_id = ? AND from_node_id = ? "
                        "AND to_node_id = ?",
                        (tenant_id, edge_type_id, from_ref, to_ref),
                    )
                    # Clean up ACL inheritance pointer
                    conn.execute(
                        "DELETE FROM acl_inherit WHERE node_id = ? AND inherit_from = ?",
                        (to_ref, from_ref),
                    )

                elif op_type == "admin_transfer_content":
                    from_user = op["from_user"]
                    to_user = op["to_user"]
                    conn.execute(
                        "UPDATE nodes SET owner_actor = ?, updated_at = ? "
                        "WHERE tenant_id = ? AND owner_actor = ?",
                        (to_user, event.ts_ms, tenant_id, from_user),
                    )

                elif op_type == "admin_revoke_access":
                    user_id = op["user_id"]
                    conn.execute(
                        "DELETE FROM node_visibility WHERE tenant_id = ? AND grantee = ?",
                        (tenant_id, user_id),
                    )

                else:
                    logger.warning(f"Unknown operation type: {op_type}")

            # Record the event as applied — same transaction
            stream_pos_str = str(event.stream_pos) if event.stream_pos else None
            conn.execute(
                "INSERT INTO applied_events "
                "(tenant_id, idempotency_key, stream_pos, applied_at) "
                "VALUES (?, ?, ?, ?)",
                (
                    tenant_id,
                    event.idempotency_key,
                    stream_pos_str,
                    int(time.time() * 1000),
                ),
            )

        return (False, created_nodes, created_edges, deleted_node_ids)

    def _log_result(self, result: ApplyResult) -> None:
        """Log the result of applying an event."""
        if result.success and not result.skipped:
            self._processed_count += 1
            record_applier_event("applied")
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
            record_applier_event("skipped")
            logger.debug(
                "Skipped duplicate event",
                extra={
                    "tenant_id": result.event.tenant_id,
                    "idempotency_key": result.event.idempotency_key,
                },
            )
        else:
            self._error_count += 1
            record_applier_event("error")
            logger.error(
                "Failed to apply event",
                extra={
                    "tenant_id": result.event.tenant_id,
                    "idempotency_key": result.event.idempotency_key,
                    "error": result.error,
                },
            )

    async def stop(self) -> None:
        """Stop the applier loop."""
        self._running = False
        logger.info("Stopping applier")

    async def _increment_usage_safe(self, tenant_id: str, n_ops: int) -> None:
        """Post-apply usage-counter bump for Phase 1 quotas.

        Called after a successful ``ExecuteAtomic`` apply (both the
        single-event and tenant-batch paths). This MUST be
        fire-and-forget: a failure here cannot be allowed to block the
        apply path or affect the return value of ``apply_event`` /
        ``_apply_tenant_batch``. Billing drift is preferable to a write
        outage, per ``docs/decisions/quotas.md``.
        """
        if self.global_store is None or n_ops <= 0:
            return
        try:
            await self.global_store.increment_usage(tenant_id, n_ops)
        except Exception:
            logger.warning(
                "usage increment failed for %s",
                tenant_id,
                exc_info=True,
            )

    async def apply_event(self, event: TransactionEvent) -> ApplyResult:
        """Apply a single transaction event atomically.

        All operations + idempotency check + record happen in a single
        SQLite transaction. If any operation fails, everything rolls back
        (including the idempotency record), so retries are safe.

        Args:
            event: Transaction event to apply

        Returns:
            ApplyResult indicating success/failure
        """
        # Ensure tenant exists
        if not await self.canonical_store.tenant_exists(event.tenant_id):
            await self.canonical_store.initialize_tenant(event.tenant_id)

        # Apply all operations atomically in a single transaction.
        # The synchronous SQLite work runs in the canonical_store
        # ThreadPoolExecutor so we never block the gRPC event loop.
        try:
            tenant_id = event.tenant_id
            loop = asyncio.get_running_loop()
            skipped, created_nodes, created_edges, deleted_node_ids = await loop.run_in_executor(
                self.canonical_store._executor,
                self._sync_apply_event_body,
                event,
            )

            if skipped:
                return ApplyResult(success=True, event=event, skipped=True)

            # Mailbox fanout happens AFTER the transaction commits
            # (outside the batch_transaction context manager)
            if self.fanout_config.enabled and created_nodes:
                for op in event.ops:
                    if op.get("op") == "create_node":
                        node_id = op.get("id") or created_nodes[0]
                        node = await self.canonical_store.get_node(tenant_id, node_id)
                        if node:
                            await self._fanout_node(event, node, op)

            # Shared index cleanup for deleted nodes
            for del_nid in deleted_node_ids:
                await self._update_shared_index_on_delete(tenant_id, del_nid)

            # Phase 1 quota: bump the durable write counter after the
            # apply transaction commits. Fire-and-forget — a failure here
            # must NEVER block the apply path (billing drift is
            # preferable to write outage, per ADR quotas.md).
            await self._increment_usage_safe(tenant_id, len(event.ops))

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

            tenant_id = data.get("tenant_id", "")

            # Skip events for tenants not assigned to this node
            if self._assigned_tenants and tenant_id not in self._assigned_tenants:
                self._skipped_count += 1
                return ApplyResult(
                    success=True,
                    event=TransactionEvent(
                        tenant_id=tenant_id,
                        actor="",
                        idempotency_key=data.get("idempotency_key", ""),
                        schema_fingerprint=None,
                        ts_ms=0,
                        ops=[],
                        stream_pos=record.position,
                    ),
                    skipped=True,
                )

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

    def _log_data_policy(self, type_id: int) -> None:
        """Log the data policy for a created node and warn if needed.

        If the type requires a legal_basis (FINANCIAL, AUDIT, HEALTHCARE)
        but none is defined, emit a warning as defense-in-depth.
        """
        try:
            registry = get_registry()
            node_type = registry.get_node_type(type_id)
            if node_type is None:
                return
            policy = registry.get_data_policy(type_id)
            logger.info(
                "create_node type_id=%d data_policy=%s",
                type_id,
                policy.value,
            )
            if policy.value in REQUIRES_LEGAL_BASIS and node_type.legal_basis is None:
                logger.warning(
                    "Node type '%s' (type_id=%d) has data_policy=%s but no legal_basis defined",
                    node_type.name,
                    type_id,
                    policy.value,
                )
        except Exception:
            # Registry may not be initialized in all environments
            pass

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
        """Fanout a node to user mailboxes via canonical store notifications.

        Writes notification rows into the per-tenant SQLite notifications
        table instead of opening separate mailbox files per user, avoiding
        the file-descriptor tax.

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

        # Build batch notification entries for the canonical store
        snippet = self._generate_snippet(node.payload)
        entries: list[dict[str, Any]] = []
        for recipient in set(recipients):
            if recipient.startswith("user:"):
                entries.append(
                    {
                        "user_id": recipient,
                        "type": str(node.type_id),
                        "node_id": node.node_id,
                        "snippet": snippet,
                    }
                )

        if not entries:
            return

        try:
            await self.canonical_store.batch_create_notifications(event.tenant_id, entries)
        except Exception as e:
            logger.warning(
                f"Failed to fanout notifications: {e}",
                extra={
                    "tenant_id": event.tenant_id,
                    "node_id": node.node_id,
                    "recipient_count": len(entries),
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

    # ── shared_index integration ────────────────────────────────────────

    async def _update_shared_index_on_share(
        self,
        tenant_id: str,
        node_id: str,
        actor_id: str,
        permission: str,
    ) -> None:
        """Write to GlobalStore.shared_index when a share_node event is applied.

        If the actor_id is a group (starts with "group:"), resolves group members
        and creates individual shared_index entries for each member.
        """
        if not self.global_store:
            return

        try:
            if actor_id.startswith("group:"):
                # Group expansion: resolve members and create entries for each
                members = await self.canonical_store.get_group_members(tenant_id, actor_id)
                for member_id in members:
                    await self.global_store.add_shared(
                        user_id=member_id,
                        source_tenant=tenant_id,
                        node_id=node_id,
                        permission=permission,
                    )
            else:
                await self.global_store.add_shared(
                    user_id=actor_id,
                    source_tenant=tenant_id,
                    node_id=node_id,
                    permission=permission,
                )
        except Exception as e:
            logger.warning(
                f"Failed to update shared_index on share: {e}",
                extra={"tenant_id": tenant_id, "node_id": node_id, "actor_id": actor_id},
            )

    async def _update_shared_index_on_revoke(
        self,
        tenant_id: str,
        node_id: str,
        actor_id: str,
    ) -> None:
        """Remove from GlobalStore.shared_index when a revoke_access event is applied.

        If the actor_id is a group, removes entries for all group members.
        """
        if not self.global_store:
            return

        try:
            if actor_id.startswith("group:"):
                members = await self.canonical_store.get_group_members(tenant_id, actor_id)
                for member_id in members:
                    await self.global_store.remove_shared(
                        user_id=member_id,
                        source_tenant=tenant_id,
                        node_id=node_id,
                    )
            else:
                await self.global_store.remove_shared(
                    user_id=actor_id,
                    source_tenant=tenant_id,
                    node_id=node_id,
                )
        except Exception as e:
            logger.warning(
                f"Failed to update shared_index on revoke: {e}",
                extra={"tenant_id": tenant_id, "node_id": node_id, "actor_id": actor_id},
            )

    async def _update_shared_index_on_delete(
        self,
        tenant_id: str,
        node_id: str,
    ) -> None:
        """Clean up shared_index entries when a node is deleted."""
        if not self.global_store:
            return

        try:
            await self.global_store.cleanup_stale_shared(
                source_tenant=tenant_id,
                node_id=node_id,
            )
        except Exception as e:
            logger.warning(
                f"Failed to cleanup shared_index on delete: {e}",
                extra={"tenant_id": tenant_id, "node_id": node_id},
            )

    async def _update_shared_index_on_group_member_add(
        self,
        tenant_id: str,
        group_id: str,
        member_actor_id: str,
    ) -> None:
        """When a member is added to a group, add shared_index entries for all nodes
        that the group has access to.
        """
        if not self.global_store:
            return

        try:
            access_entries = await self.canonical_store.list_node_access_for_group(
                tenant_id, group_id
            )
            for entry in access_entries:
                await self.global_store.add_shared(
                    user_id=member_actor_id,
                    source_tenant=tenant_id,
                    node_id=entry["node_id"],
                    permission=entry["permission"],
                )
        except Exception as e:
            logger.warning(
                f"Failed to update shared_index on group member add: {e}",
                extra={
                    "tenant_id": tenant_id,
                    "group_id": group_id,
                    "member": member_actor_id,
                },
            )

    async def _update_shared_index_on_group_member_remove(
        self,
        tenant_id: str,
        group_id: str,
        member_actor_id: str,
    ) -> None:
        """When a member is removed from a group, remove shared_index entries
        for all nodes that the group has access to.
        """
        if not self.global_store:
            return

        try:
            access_entries = await self.canonical_store.list_node_access_for_group(
                tenant_id, group_id
            )
            for entry in access_entries:
                await self.global_store.remove_shared(
                    user_id=member_actor_id,
                    source_tenant=tenant_id,
                    node_id=entry["node_id"],
                )
        except Exception as e:
            logger.warning(
                f"Failed to update shared_index on group member remove: {e}",
                extra={
                    "tenant_id": tenant_id,
                    "group_id": group_id,
                    "member": member_actor_id,
                },
            )

    @property
    def stats(self) -> dict[str, Any]:
        """Get applier statistics."""
        return {
            "running": self._running,
            "processed_count": self._processed_count,
            "error_count": self._error_count,
            "last_position": str(self._last_position) if self._last_position else None,
            "skipped_count": self._skipped_count,
        }
