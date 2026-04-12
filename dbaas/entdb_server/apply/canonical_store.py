"""
Canonical tenant SQLite store for EntDB.

This module manages the per-tenant SQLite database that stores:
- Nodes with their payloads and ACLs
- Edges connecting nodes
- Applied events for idempotency checking
- Node visibility index for ACL filtering
- Notifications (per-user mailbox within the tenant)
- Read cursors (per-user, per-channel last-read timestamps)

The canonical store is the materialized view of the WAL stream.
It can be rebuilt by replaying events from the stream or archive.

Invariants:
    - One SQLite file per tenant
    - All operations are atomic (single transaction)
    - applied_events table prevents duplicate processing
    - Visibility index is always consistent with node ACLs

How to change safely:
    - Schema migrations must be backward compatible
    - Test with large datasets before production
    - Use transactions for all write operations
    - Monitor SQLite file size and performance

Table schema:
    nodes:
        - tenant_id TEXT
        - node_id TEXT (UUID)
        - type_id INTEGER
        - payload_json TEXT
        - created_at INTEGER (Unix ms)
        - updated_at INTEGER (Unix ms)
        - owner_actor TEXT
        - acl_blob TEXT (JSON)
        - PRIMARY KEY (tenant_id, node_id)

    edges:
        - tenant_id TEXT
        - edge_id INTEGER (edge type)
        - from_node_id TEXT
        - to_node_id TEXT
        - props_json TEXT
        - created_at INTEGER
        - PRIMARY KEY (tenant_id, edge_id, from_node_id, to_node_id)

    node_visibility:
        - tenant_id TEXT
        - node_id TEXT
        - principal TEXT
        - INDEX on (tenant_id, principal, node_id)

    applied_events:
        - tenant_id TEXT
        - idempotency_key TEXT
        - stream_pos TEXT
        - applied_at INTEGER
        - UNIQUE (tenant_id, idempotency_key)
"""

from __future__ import annotations

import asyncio
import contextlib
import hashlib
import json
import logging
import sqlite3
import time
import uuid
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from ..config import EncryptionConfig
from ..encryption import derive_tenant_key, open_encrypted_connection
from ..metrics import metrics_enabled as _metrics_enabled
from ..metrics import record_sqlite_op as _record_sqlite_op

logger = logging.getLogger(__name__)


def _parse_stream_offset(pos: str) -> int:
    """Extract the numeric offset from a stream position string.

    Stream positions have the format "topic:partition:offset" (from StreamPos.__str__).
    If the string is a plain integer, parse it directly.
    """
    parts = pos.rsplit(":", 1)
    try:
        return int(parts[-1])
    except (ValueError, IndexError):
        return 0


def _compare_stream_pos(a: str, b: str) -> int:
    """Compare two stream position strings by their numeric offset.

    Returns:
        Positive if a > b, negative if a < b, zero if equal.
    """
    return _parse_stream_offset(a) - _parse_stream_offset(b)


def _compute_entry_hash(
    event_id: str,
    prev_hash: str,
    actor_id: str,
    action: str,
    target_type: str,
    target_id: str,
    created_at: int,
) -> str:
    """Compute the SHA-256 hash for an audit log entry.

    The hash covers the core immutable fields so that any modification
    to a row (or its predecessor) breaks the chain.
    """
    content = f"{event_id}:{prev_hash}:{actor_id}:{action}:{target_type}:{target_id}:{created_at}"
    return hashlib.sha256(content.encode()).hexdigest()


class TenantNotFoundError(Exception):
    """Tenant database does not exist."""

    pass


class IdempotencyViolationError(Exception):
    """Event has already been applied."""

    pass


class Node:
    """Represents a node in the graph.

    Stores payload and ACL as raw JSON strings internally to avoid
    unnecessary json.loads/json.dumps round-trips on the read path.
    The parsed forms are available via lazy properties.

    Attributes:
        tenant_id: Tenant identifier
        node_id: Unique node identifier (UUID)
        type_id: Node type identifier
        payload: Field values (lazy-parsed from payload_json)
        created_at: Creation timestamp (Unix ms)
        updated_at: Last update timestamp (Unix ms)
        owner_actor: Actor who created the node
        acl: Access control list (lazy-parsed from acl_json)
        payload_json: Raw JSON string for payload
        acl_json: Raw JSON string for ACL
    """

    __slots__ = (
        "tenant_id",
        "node_id",
        "type_id",
        "_payload_json",
        "_payload_parsed",
        "created_at",
        "updated_at",
        "owner_actor",
        "_acl_json",
        "_acl_parsed",
    )

    _SENTINEL = object()

    def __init__(
        self,
        tenant_id: str,
        node_id: str,
        type_id: int,
        payload: dict[str, Any] | None = None,
        created_at: int = 0,
        updated_at: int = 0,
        owner_actor: str = "",
        acl: list[dict[str, str]] | None = None,
        *,
        payload_json: str | None = None,
        acl_json: str | None = None,
    ) -> None:
        self.tenant_id = tenant_id
        self.node_id = node_id
        self.type_id = type_id
        self.created_at = created_at
        self.updated_at = updated_at
        self.owner_actor = owner_actor

        # Payload: prefer raw JSON string to avoid serialization round-trip
        if payload_json is not None:
            self._payload_json: str = payload_json
            self._payload_parsed: Any = Node._SENTINEL
        elif payload is not None:
            self._payload_json = json.dumps(payload)
            self._payload_parsed = payload
        else:
            self._payload_json = "{}"
            self._payload_parsed: Any = {}

        # ACL: same approach
        if acl_json is not None:
            self._acl_json: str = acl_json
            self._acl_parsed: Any = Node._SENTINEL
        elif acl is not None:
            self._acl_json = json.dumps(acl)
            self._acl_parsed = acl
        else:
            self._acl_json = "[]"
            self._acl_parsed: Any = []

    @classmethod
    def from_row(
        cls,
        tenant_id: str,
        node_id: str,
        type_id: int,
        payload_json: str,
        created_at: int,
        updated_at: int,
        owner_actor: str,
        acl_json: str,
    ) -> Node:
        """Construct from raw SQLite row data without parsing JSON."""
        return cls(
            tenant_id=tenant_id,
            node_id=node_id,
            type_id=type_id,
            created_at=created_at,
            updated_at=updated_at,
            owner_actor=owner_actor,
            payload_json=payload_json,
            acl_json=acl_json,
        )

    @property
    def payload(self) -> dict[str, Any]:
        """Lazily parse payload from JSON string."""
        val = self._payload_parsed
        if val is Node._SENTINEL:
            val = json.loads(self._payload_json)
            self._payload_parsed = val
        return val

    @property
    def acl(self) -> list[dict[str, str]]:
        """Lazily parse ACL from JSON string."""
        val = self._acl_parsed
        if val is Node._SENTINEL:
            val = json.loads(self._acl_json)
            self._acl_parsed = val
        return val

    @property
    def payload_json(self) -> str:
        """Raw JSON string for payload -- no serialization needed."""
        return self._payload_json

    @property
    def acl_json(self) -> str:
        """Raw JSON string for ACL -- no serialization needed."""
        return self._acl_json

    def __repr__(self) -> str:
        return (
            f"Node(tenant_id={self.tenant_id!r}, node_id={self.node_id!r}, "
            f"type_id={self.type_id!r}, created_at={self.created_at!r})"
        )

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Node):
            return NotImplemented
        return (
            self.tenant_id == other.tenant_id
            and self.node_id == other.node_id
            and self.type_id == other.type_id
            and self.created_at == other.created_at
            and self.updated_at == other.updated_at
            and self.owner_actor == other.owner_actor
            and self._payload_json == other._payload_json
            and self._acl_json == other._acl_json
        )


@dataclass
class Edge:
    """Represents an edge in the graph.

    Attributes:
        tenant_id: Tenant identifier
        edge_type_id: Edge type identifier
        from_node_id: Source node ID
        to_node_id: Target node ID
        props: Edge properties
        created_at: Creation timestamp (Unix ms)
    """

    tenant_id: str
    edge_type_id: int
    from_node_id: str
    to_node_id: str
    props: dict[str, Any]
    created_at: int


class CanonicalStore:
    """Per-tenant SQLite store for canonical node/edge data.

    This class manages SQLite databases for tenants, providing:
    - Node CRUD operations
    - Edge CRUD operations
    - Idempotency checking via applied_events table
    - Visibility index management
    - Transaction support

    Thread safety:
        One pooled connection per tenant database file.
        SQLite handles concurrent access via WAL mode.

    Example:
        >>> store = CanonicalStore("/var/lib/entdb")
        >>> await store.initialize_tenant("tenant_123")
        >>> node = await store.create_node(
        ...     tenant_id="tenant_123",
        ...     type_id=101,
        ...     payload={"title": "My Task"},
        ...     owner_actor="user:42",
        ... )
    """

    # SQLite schema version for migrations
    SCHEMA_VERSION = 1

    def __init__(
        self,
        data_dir: str,
        wal_mode: bool = True,
        busy_timeout_ms: int = 5000,
        cache_size_pages: int = -64000,
        encryption_config: EncryptionConfig | None = None,
    ) -> None:
        """Initialize the canonical store.

        Args:
            data_dir: Directory for SQLite database files
            wal_mode: Enable SQLite WAL mode
            busy_timeout_ms: SQLite busy timeout
            cache_size_pages: SQLite cache size (negative = KB)
            encryption_config: Optional encryption configuration. When
                enabled, tenant databases are encrypted with per-tenant
                keys derived from the master key.
        """
        self.data_dir = Path(data_dir)
        self.wal_mode = wal_mode
        self.busy_timeout_ms = busy_timeout_ms
        self.cache_size_pages = cache_size_pages
        self.encryption_config = encryption_config or EncryptionConfig()
        self._connections: dict[str, sqlite3.Connection] = {}
        # Single-thread executor — SQLite connections are not thread-safe,
        # so all DB ops must run on the same thread when using connection pooling.
        from concurrent.futures import ThreadPoolExecutor

        self._executor = ThreadPoolExecutor(max_workers=1, thread_name_prefix="entdb-sqlite")

        # Offset tracking for read-after-write consistency.
        # Updated by the applier after each batch; read RPCs can wait on these.
        self._applied_offsets: dict[str, str] = {}
        self._offset_conditions: dict[str, asyncio.Condition] = {}

    # ── offset tracking (read-after-write consistency) ──────────────────

    def _get_offset_condition(self, tenant_id: str) -> asyncio.Condition:
        """Get or create an asyncio.Condition for a tenant's offset notifications."""
        cond = self._offset_conditions.get(tenant_id)
        if cond is None:
            cond = asyncio.Condition()
            self._offset_conditions[tenant_id] = cond
        return cond

    async def update_applied_offset(self, tenant_id: str, stream_pos: str) -> None:
        """Record the latest applied stream position for a tenant.

        Called by the applier after successfully applying a batch.
        Notifies all waiters via the Condition.

        Args:
            tenant_id: Tenant identifier
            stream_pos: Stream position string (e.g. "topic:0:5")
        """
        cond = self._get_offset_condition(tenant_id)
        async with cond:
            self._applied_offsets[tenant_id] = stream_pos
            cond.notify_all()

    async def wait_for_offset(
        self,
        tenant_id: str,
        target_pos: str,
        timeout: float = 30.0,
    ) -> bool:
        """Wait until the applied offset for *tenant_id* reaches *target_pos*.

        Uses an asyncio.Condition so the caller sleeps efficiently instead of
        polling.

        Args:
            tenant_id: Tenant identifier
            target_pos: Stream position to wait for
            timeout: Maximum seconds to wait

        Returns:
            True if the target position was reached, False on timeout.
        """
        cond = self._get_offset_condition(tenant_id)

        def _reached() -> bool:
            current = self._applied_offsets.get(tenant_id)
            if current is None:
                return False
            return _compare_stream_pos(current, target_pos) >= 0

        async with cond:
            try:
                return await asyncio.wait_for(
                    cond.wait_for(_reached),
                    timeout=timeout,
                )
            except asyncio.TimeoutError:
                return False

    def get_applied_offset(self, tenant_id: str) -> str | None:
        """Return the latest applied offset for a tenant (or None)."""
        return self._applied_offsets.get(tenant_id)

    # Pre-computed op-type cache: avoids repeated string-in-name checks
    # on every _run_sync call.  Populated lazily per function object.
    _op_type_cache: dict[Callable, str] = {}

    @staticmethod
    def _classify_op(fn: Callable) -> str:
        """Classify a sync helper as 'read' or 'write' by its name."""
        name = fn.__name__
        if "create" in name or "update" in name or "delete" in name or "mark" in name or "batch" in name:
            return "write"
        return "read"

    async def _run_sync(self, fn: Callable, *args: Any) -> Any:
        """Run a synchronous function in a dedicated thread to avoid blocking the event loop."""
        if _metrics_enabled():
            cache = CanonicalStore._op_type_cache
            op_type = cache.get(fn)
            if op_type is None:
                op_type = self._classify_op(fn)
                cache[fn] = op_type
            _record_sqlite_op(op_type)
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            # No running loop (e.g., pytest-xdist worker) — run directly
            return fn(*args)
        return await loop.run_in_executor(self._executor, fn, *args)

    def _get_db_path(self, tenant_id: str) -> Path:
        """Get database file path for a tenant."""
        # Sanitize tenant_id to prevent path traversal
        safe_id = "".join(c for c in tenant_id if c.isalnum() or c in "-_")
        return self.data_dir / f"tenant_{safe_id}.db"

    @contextmanager
    def _get_connection(self, tenant_id: str, create: bool = False) -> Iterator[sqlite3.Connection]:
        """Get a database connection for a tenant.

        Returns a pooled connection (one per tenant db path). Connections are
        reused across operations and PRAGMAs are configured only once.

        Args:
            tenant_id: Tenant identifier
            create: Whether to create database if not exists

        Yields:
            SQLite connection

        Raises:
            TenantNotFoundError: If database doesn't exist and create=False
        """
        db_path = self._get_db_path(tenant_id)

        if not create and not db_path.exists():
            raise TenantNotFoundError(f"Tenant database not found: {tenant_id}")

        # Ensure directory exists
        db_path.parent.mkdir(parents=True, exist_ok=True)

        # Reuse cached connection
        cache_key = str(db_path)
        if cache_key not in self._connections:
            # Open with encryption if configured
            if self.encryption_config.enabled:
                tenant_key = derive_tenant_key(
                    self.encryption_config.master_key, tenant_id
                )
                conn = open_encrypted_connection(
                    str(db_path),
                    tenant_key,
                    timeout=self.busy_timeout_ms / 1000.0,
                )
            else:
                conn = sqlite3.connect(
                    str(db_path),
                    timeout=self.busy_timeout_ms / 1000.0,
                    isolation_level=None,  # Autocommit by default, explicit transactions
                    check_same_thread=False,
                )
            conn.row_factory = sqlite3.Row

            # Configure connection — only once per cached connection
            conn.execute(f"PRAGMA busy_timeout = {self.busy_timeout_ms}")
            conn.execute(f"PRAGMA cache_size = {self.cache_size_pages}")
            if self.wal_mode:
                conn.execute("PRAGMA journal_mode = WAL")
            conn.execute("PRAGMA synchronous = NORMAL")
            conn.execute("PRAGMA foreign_keys = ON")

            self._connections[cache_key] = conn

        yield self._connections[cache_key]
        # Don't close — connection is pooled

    def close_all(self) -> None:
        """Close all pooled connections."""
        for conn in self._connections.values():
            with contextlib.suppress(Exception):
                conn.close()
        self._connections.clear()

    def _create_schema(self, conn: sqlite3.Connection) -> None:
        """Create database schema."""
        conn.executescript("""
            -- Schema version tracking
            CREATE TABLE IF NOT EXISTS schema_version (
                version INTEGER PRIMARY KEY,
                applied_at INTEGER NOT NULL
            );

            -- Nodes table
            CREATE TABLE IF NOT EXISTS nodes (
                tenant_id TEXT NOT NULL,
                node_id TEXT NOT NULL,
                type_id INTEGER NOT NULL,
                payload_json TEXT NOT NULL DEFAULT '{}',
                created_at INTEGER NOT NULL,
                updated_at INTEGER NOT NULL,
                owner_actor TEXT NOT NULL,
                acl_blob TEXT NOT NULL DEFAULT '[]',
                PRIMARY KEY (tenant_id, node_id)
            );

            CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(tenant_id, type_id);
            CREATE INDEX IF NOT EXISTS idx_nodes_owner ON nodes(tenant_id, owner_actor);
            CREATE INDEX IF NOT EXISTS idx_nodes_updated ON nodes(tenant_id, updated_at DESC);

            -- Edges table
            CREATE TABLE IF NOT EXISTS edges (
                tenant_id TEXT NOT NULL,
                edge_type_id INTEGER NOT NULL,
                from_node_id TEXT NOT NULL,
                to_node_id TEXT NOT NULL,
                props_json TEXT NOT NULL DEFAULT '{}',
                propagates_acl INTEGER NOT NULL DEFAULT 0,
                created_at INTEGER NOT NULL,
                PRIMARY KEY (tenant_id, edge_type_id, from_node_id, to_node_id)
            );

            CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(tenant_id, from_node_id);
            CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(tenant_id, to_node_id);
            CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(tenant_id, edge_type_id);

            -- Node visibility for ACL filtering
            CREATE TABLE IF NOT EXISTS node_visibility (
                tenant_id TEXT NOT NULL,
                node_id TEXT NOT NULL,
                principal TEXT NOT NULL,
                PRIMARY KEY (tenant_id, node_id, principal)
            );

            CREATE INDEX IF NOT EXISTS idx_visibility_principal
                ON node_visibility(tenant_id, principal, node_id);

            -- Applied events for idempotency
            CREATE TABLE IF NOT EXISTS applied_events (
                tenant_id TEXT NOT NULL,
                idempotency_key TEXT NOT NULL,
                stream_pos TEXT,
                applied_at INTEGER NOT NULL,
                UNIQUE (tenant_id, idempotency_key)
            );

            CREATE INDEX IF NOT EXISTS idx_applied_events_key
                ON applied_events(tenant_id, idempotency_key);

            -- ACL: direct grants (who has explicit access to a node)
            CREATE TABLE IF NOT EXISTS node_access (
                node_id     TEXT NOT NULL,
                actor_id    TEXT NOT NULL,
                actor_type  TEXT NOT NULL DEFAULT 'user',
                permission  TEXT NOT NULL,
                granted_by  TEXT NOT NULL,
                granted_at  INTEGER NOT NULL,
                expires_at  INTEGER DEFAULT NULL,
                PRIMARY KEY (node_id, actor_id)
            );

            CREATE INDEX IF NOT EXISTS idx_access_actor
                ON node_access(actor_id, node_id);

            -- ACL: group membership (supports nested groups)
            CREATE TABLE IF NOT EXISTS group_users (
                group_id         TEXT NOT NULL,
                member_actor_id  TEXT NOT NULL,
                role             TEXT NOT NULL DEFAULT 'member',
                joined_at        INTEGER NOT NULL,
                PRIMARY KEY (group_id, member_actor_id)
            );

            CREATE INDEX IF NOT EXISTS idx_group_users_member
                ON group_users(member_actor_id);

            -- ACL: inheritance pointers (structural parent for permission)
            CREATE TABLE IF NOT EXISTS acl_inherit (
                node_id      TEXT NOT NULL,
                inherit_from TEXT NOT NULL,
                PRIMARY KEY (node_id, inherit_from)
            );

            CREATE INDEX IF NOT EXISTS idx_inherit_from
                ON acl_inherit(inherit_from);

            -- Notifications (per-user mailbox within tenant)
            CREATE TABLE IF NOT EXISTS notifications (
                id          TEXT PRIMARY KEY,
                user_id     TEXT NOT NULL,
                type        TEXT NOT NULL,
                node_id     TEXT NOT NULL,
                snippet     TEXT,
                read        INTEGER DEFAULT 0,
                created_at  INTEGER NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_notif_user
                ON notifications(user_id, read, created_at DESC);

            -- Read cursors (per-user, per-channel last-read timestamp)
            CREATE TABLE IF NOT EXISTS read_cursors (
                user_id      TEXT NOT NULL,
                channel_id   TEXT NOT NULL,
                last_read_at INTEGER NOT NULL,
                PRIMARY KEY (user_id, channel_id)
            );

            -- Type metadata (data policy, PII fields, subject field)
            CREATE TABLE IF NOT EXISTS type_metadata (
                type_id       INTEGER PRIMARY KEY,
                data_policy   TEXT NOT NULL DEFAULT 'personal',
                pii_fields    TEXT NOT NULL DEFAULT '[]',
                subject_field TEXT DEFAULT NULL
            );

            -- Audit log with hash chain for tamper evidence
            CREATE TABLE IF NOT EXISTS audit_log (
                event_id    TEXT PRIMARY KEY,
                prev_hash   TEXT NOT NULL,
                actor_id    TEXT NOT NULL,
                action      TEXT NOT NULL,
                target_type TEXT NOT NULL,
                target_id   TEXT NOT NULL,
                ip_address  TEXT,
                user_agent  TEXT,
                metadata    TEXT,
                created_at  INTEGER NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_audit_time
                ON audit_log(created_at DESC);
            CREATE INDEX IF NOT EXISTS idx_audit_actor
                ON audit_log(actor_id, created_at DESC);

            -- Record schema version
            INSERT OR IGNORE INTO schema_version (version, applied_at)
            VALUES (1, strftime('%s', 'now') * 1000);
        """)

    def sync_type_metadata(
        self,
        tenant_id: str,
        registry: object,
    ) -> None:
        """Populate the type_metadata table from a SchemaRegistry.

        Inserts or replaces rows for every node type in the registry,
        caching data_policy, pii_fields, and subject_field so that
        runtime queries can look them up without touching the registry.

        Args:
            tenant_id: Tenant identifier
            registry: SchemaRegistry instance (typed as object to avoid
                circular imports; must have node_types() iterator and
                get_data_policy / get_pii_fields / get_subject_field)
        """
        from ..schema.registry import SchemaRegistry

        reg: SchemaRegistry = registry  # type: ignore[assignment]
        with self._get_connection(tenant_id) as conn:
            for node_type in reg.node_types():
                policy = reg.get_data_policy(node_type.type_id)
                pii = reg.get_pii_fields(node_type.type_id)
                subject = reg.get_subject_field(node_type.type_id)
                conn.execute(
                    "INSERT OR REPLACE INTO type_metadata "
                    "(type_id, data_policy, pii_fields, subject_field) "
                    "VALUES (?, ?, ?, ?)",
                    (
                        node_type.type_id,
                        policy.value,
                        json.dumps(pii),
                        subject,
                    ),
                )
            conn.commit()

    @contextmanager
    def batch_transaction(self, tenant_id: str) -> Iterator[sqlite3.Connection]:
        """Open a single transaction for multiple operations.

        Use this to batch multiple creates/updates/deletes into one
        SQLite transaction, amortizing the fsync cost across all operations.

        Args:
            tenant_id: Tenant identifier

        Yields:
            SQLite connection with an open transaction

        Example:
            >>> with store.batch_transaction("tenant_1") as conn:
            ...     for event in batch:
            ...         store.create_node_raw(conn, ...)
        """
        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                yield conn
                conn.execute("COMMIT")
            except Exception:
                conn.execute("ROLLBACK")
                raise

    def create_node_raw(
        self,
        conn: sqlite3.Connection,
        tenant_id: str,
        type_id: int,
        payload: dict[str, Any],
        owner_actor: str,
        node_id: str | None = None,
        acl: list[dict[str, str]] | None = None,
        created_at: int | None = None,
    ) -> Node:
        """Create a node within an existing transaction (no BEGIN/COMMIT).

        Use inside batch_transaction() for batched writes.

        Args:
            conn: SQLite connection with open transaction
            tenant_id: Tenant identifier
            type_id: Node type identifier
            payload: Field values
            owner_actor: Actor creating the node
            node_id: Optional specific node ID
            acl: Access control list entries
            created_at: Optional creation timestamp

        Returns:
            Created Node object
        """
        if node_id is None:
            node_id = str(uuid.uuid4())
        now = created_at or int(time.time() * 1000)
        acl = acl or []

        # Serialize once; reuse for both the INSERT and the Node object.
        payload_str = json.dumps(payload)
        acl_str = json.dumps(acl)

        conn.execute(
            """
            INSERT INTO nodes (tenant_id, node_id, type_id, payload_json,
                               created_at, updated_at, owner_actor, acl_blob)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                tenant_id,
                node_id,
                type_id,
                payload_str,
                now,
                now,
                owner_actor,
                acl_str,
            ),
        )

        self._update_visibility(conn, tenant_id, node_id, owner_actor, acl)

        return Node(
            tenant_id=tenant_id,
            node_id=node_id,
            type_id=type_id,
            created_at=now,
            updated_at=now,
            owner_actor=owner_actor,
            payload_json=payload_str,
            acl_json=acl_str,
        )

    async def initialize_tenant(self, tenant_id: str) -> None:
        """Initialize database for a new tenant.

        Creates the database file and schema if they don't exist.
        Runs synchronously (not in executor) because it's only called
        once per tenant and modifies the connection pool.

        Args:
            tenant_id: Tenant identifier
        """
        with self._get_connection(tenant_id, create=True) as conn:
            self._create_schema(conn)
            logger.info(f"Initialized tenant database: {tenant_id}")

    async def tenant_exists(self, tenant_id: str) -> bool:
        """Check if tenant database exists."""
        return self._get_db_path(tenant_id).exists()

    # ── check_idempotency ──────────────────────────────────────────────

    def _sync_check_idempotency(
        self,
        tenant_id: str,
        idempotency_key: str,
    ) -> bool:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "SELECT 1 FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?",
                (tenant_id, idempotency_key),
            )
            return cursor.fetchone() is not None

    async def check_idempotency(
        self,
        tenant_id: str,
        idempotency_key: str,
    ) -> bool:
        """Check if an event has already been applied.

        Args:
            tenant_id: Tenant identifier
            idempotency_key: Event idempotency key

        Returns:
            True if event was already applied, False otherwise
        """
        return await self._run_sync(self._sync_check_idempotency, tenant_id, idempotency_key)

    # ── record_applied_event ───────────────────────────────────────────

    def _sync_record_applied_event(
        self,
        tenant_id: str,
        idempotency_key: str,
        stream_pos: str | None,
    ) -> None:
        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at)
                VALUES (?, ?, ?, ?)
                """,
                (
                    tenant_id,
                    idempotency_key,
                    stream_pos,
                    int(time.time() * 1000),
                ),
            )

    async def record_applied_event(
        self,
        tenant_id: str,
        idempotency_key: str,
        stream_pos: str | None = None,
    ) -> None:
        """Record that an event has been applied.

        Args:
            tenant_id: Tenant identifier
            idempotency_key: Event idempotency key
            stream_pos: Stream position string
        """
        await self._run_sync(
            self._sync_record_applied_event,
            tenant_id,
            idempotency_key,
            stream_pos,
        )

    # ── apply_with_idempotency (synchronous, transaction context) ──────

    def apply_with_idempotency(
        self,
        tenant_id: str,
        idempotency_key: str,
        stream_pos: str | None,
        apply_fn: Callable,
    ) -> bool:
        """Apply operations atomically with idempotency check.

        Checks idempotency, calls apply_fn(conn) within the same transaction,
        and records the event -- all in one BEGIN/COMMIT. Returns True if applied,
        False if already applied (idempotent skip).
        """
        with self._get_connection(tenant_id) as conn:
            # Check idempotency within this transaction
            cursor = conn.execute(
                "SELECT 1 FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?",
                (tenant_id, idempotency_key),
            )
            if cursor.fetchone():
                return False  # Already applied

            conn.execute("BEGIN IMMEDIATE")
            try:
                # Re-check under exclusive lock
                cursor = conn.execute(
                    "SELECT 1 FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?",
                    (tenant_id, idempotency_key),
                )
                if cursor.fetchone():
                    conn.execute("ROLLBACK")
                    return False

                # Apply operations
                apply_fn(conn)

                # Record applied event
                conn.execute(
                    "INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at) "
                    "VALUES (?, ?, ?, ?)",
                    (
                        tenant_id,
                        idempotency_key,
                        stream_pos,
                        int(time.time() * 1000),
                    ),
                )
                conn.execute("COMMIT")
                return True
            except Exception:
                conn.execute("ROLLBACK")
                raise

    # ── create_node ────────────────────────────────────────────────────

    def _sync_create_node(
        self,
        tenant_id: str,
        type_id: int,
        payload: dict[str, Any],
        owner_actor: str,
        node_id: str,
        acl: list[dict[str, str]],
        now: int,
    ) -> Node:
        # Serialize once; reuse for both the INSERT and the Node object.
        payload_str = json.dumps(payload)
        acl_str = json.dumps(acl)

        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                conn.execute(
                    """
                    INSERT INTO nodes (tenant_id, node_id, type_id, payload_json,
                                       created_at, updated_at, owner_actor, acl_blob)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                    """,
                    (
                        tenant_id,
                        node_id,
                        type_id,
                        payload_str,
                        now,
                        now,
                        owner_actor,
                        acl_str,
                    ),
                )

                # Update visibility index
                self._update_visibility(conn, tenant_id, node_id, owner_actor, acl)

                conn.execute("COMMIT")

            except Exception:
                conn.execute("ROLLBACK")
                raise

        logger.debug(
            "Created node",
            extra={
                "tenant_id": tenant_id,
                "node_id": node_id,
                "type_id": type_id,
            },
        )

        return Node(
            tenant_id=tenant_id,
            node_id=node_id,
            type_id=type_id,
            created_at=now,
            updated_at=now,
            owner_actor=owner_actor,
            payload_json=payload_str,
            acl_json=acl_str,
        )

    async def create_node(
        self,
        tenant_id: str,
        type_id: int,
        payload: dict[str, Any],
        owner_actor: str,
        node_id: str | None = None,
        acl: list[dict[str, str]] | None = None,
        created_at: int | None = None,
    ) -> Node:
        """Create a new node.

        Args:
            tenant_id: Tenant identifier
            type_id: Node type identifier
            payload: Field values
            owner_actor: Actor creating the node
            node_id: Optional specific node ID (generated if not provided)
            acl: Access control list entries
            created_at: Optional creation timestamp

        Returns:
            Created Node object
        """
        if node_id is None:
            node_id = str(uuid.uuid4())
        now = created_at or int(time.time() * 1000)
        acl = acl or []

        return await self._run_sync(
            self._sync_create_node,
            tenant_id,
            type_id,
            payload,
            owner_actor,
            node_id,
            acl,
            now,
        )

    # ── update_node ────────────────────────────────────────────────────

    def _sync_update_node(
        self,
        tenant_id: str,
        node_id: str,
        patch: dict[str, Any],
        now: int,
    ) -> Node | None:
        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                # Get existing node
                cursor = conn.execute(
                    "SELECT * FROM nodes WHERE tenant_id = ? AND node_id = ?",
                    (tenant_id, node_id),
                )
                row = cursor.fetchone()
                if not row:
                    conn.execute("ROLLBACK")
                    return None

                # Merge payloads
                existing_payload = json.loads(row["payload_json"])
                existing_payload.update(patch)
                payload_str = json.dumps(existing_payload)

                # Update node
                conn.execute(
                    """
                    UPDATE nodes SET payload_json = ?, updated_at = ?
                    WHERE tenant_id = ? AND node_id = ?
                    """,
                    (payload_str, now, tenant_id, node_id),
                )

                conn.execute("COMMIT")

                return Node(
                    tenant_id=tenant_id,
                    node_id=node_id,
                    type_id=row["type_id"],
                    created_at=row["created_at"],
                    updated_at=now,
                    owner_actor=row["owner_actor"],
                    payload_json=payload_str,
                    acl_json=row["acl_blob"],
                )

            except Exception:
                conn.execute("ROLLBACK")
                raise

    async def update_node(
        self,
        tenant_id: str,
        node_id: str,
        patch: dict[str, Any],
        updated_at: int | None = None,
    ) -> Node | None:
        """Update a node's payload.

        Uses PATCH semantics - merges with existing payload.

        Args:
            tenant_id: Tenant identifier
            node_id: Node identifier
            patch: Fields to update
            updated_at: Optional update timestamp

        Returns:
            Updated Node or None if not found
        """
        now = updated_at or int(time.time() * 1000)
        return await self._run_sync(self._sync_update_node, tenant_id, node_id, patch, now)

    # ── delete_node ────────────────────────────────────────────────────

    def _sync_delete_node(self, tenant_id: str, node_id: str) -> bool:
        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                # Delete edges
                conn.execute(
                    "DELETE FROM edges WHERE tenant_id = ? AND (from_node_id = ? OR to_node_id = ?)",
                    (tenant_id, node_id, node_id),
                )

                # Delete visibility
                conn.execute(
                    "DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?",
                    (tenant_id, node_id),
                )

                # Delete node
                cursor = conn.execute(
                    "DELETE FROM nodes WHERE tenant_id = ? AND node_id = ?",
                    (tenant_id, node_id),
                )

                conn.execute("COMMIT")
                return cursor.rowcount > 0

            except Exception:
                conn.execute("ROLLBACK")
                raise

    async def delete_node(self, tenant_id: str, node_id: str) -> bool:
        """Delete a node and its edges.

        Args:
            tenant_id: Tenant identifier
            node_id: Node identifier

        Returns:
            True if deleted, False if not found
        """
        return await self._run_sync(self._sync_delete_node, tenant_id, node_id)

    # ── get_node ───────────────────────────────────────────────────────

    def _sync_get_node(self, tenant_id: str, node_id: str) -> Node | None:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "SELECT * FROM nodes WHERE tenant_id = ? AND node_id = ?",
                (tenant_id, node_id),
            )
            row = cursor.fetchone()
            if not row:
                return None

            return Node.from_row(
                tenant_id=row["tenant_id"],
                node_id=row["node_id"],
                type_id=row["type_id"],
                payload_json=row["payload_json"],
                created_at=row["created_at"],
                updated_at=row["updated_at"],
                owner_actor=row["owner_actor"],
                acl_json=row["acl_blob"],
            )

    async def get_node(self, tenant_id: str, node_id: str) -> Node | None:
        """Get a node by ID.

        Args:
            tenant_id: Tenant identifier
            node_id: Node identifier

        Returns:
            Node or None if not found
        """
        return await self._run_sync(self._sync_get_node, tenant_id, node_id)

    # ── get_nodes_by_type ──────────────────────────────────────────────

    def _sync_get_nodes_by_type(
        self,
        tenant_id: str,
        type_id: int,
        limit: int,
        offset: int,
    ) -> list[Node]:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                """
                SELECT * FROM nodes
                WHERE tenant_id = ? AND type_id = ?
                ORDER BY created_at DESC
                LIMIT ? OFFSET ?
                """,
                (tenant_id, type_id, limit, offset),
            )

            return [
                Node.from_row(
                    tenant_id=row["tenant_id"],
                    node_id=row["node_id"],
                    type_id=row["type_id"],
                    payload_json=row["payload_json"],
                    created_at=row["created_at"],
                    updated_at=row["updated_at"],
                    owner_actor=row["owner_actor"],
                    acl_json=row["acl_blob"],
                )
                for row in cursor.fetchall()
            ]

    async def get_nodes_by_type(
        self,
        tenant_id: str,
        type_id: int,
        limit: int = 100,
        offset: int = 0,
    ) -> list[Node]:
        """Get nodes by type.

        Args:
            tenant_id: Tenant identifier
            type_id: Node type identifier
            limit: Maximum nodes to return
            offset: Pagination offset

        Returns:
            List of nodes
        """
        return await self._run_sync(self._sync_get_nodes_by_type, tenant_id, type_id, limit, offset)

    # ── query_nodes (with payload filtering) ─────────────────────────

    def _sync_query_nodes(
        self,
        tenant_id: str,
        type_id: int,
        filter_json: dict[str, Any] | None = None,
        limit: int = 100,
        offset: int = 0,
        order_by: str = "created_at",
        descending: bool = True,
    ) -> list[Node]:
        """Query nodes with optional payload field filtering."""
        with self._get_connection(tenant_id) as conn:
            order = "DESC" if descending else "ASC"
            valid_columns = {"created_at", "updated_at", "node_id", "type_id"}
            if order_by not in valid_columns:
                order_by = "created_at"

            if filter_json:
                # Fetch all matching rows for this type, filter in Python, then paginate.
                # Note: payload must be parsed here for filtering, but ACL can stay raw.
                cursor = conn.execute(
                    f"SELECT * FROM nodes WHERE tenant_id = ? AND type_id = ? "
                    f"ORDER BY {order_by} {order}",
                    (tenant_id, type_id),
                )
                nodes = []
                skipped = 0
                for row in cursor:
                    payload = json.loads(row["payload_json"])
                    if not all(payload.get(k) == v for k, v in filter_json.items()):
                        continue
                    if skipped < offset:
                        skipped += 1
                        continue
                    if len(nodes) >= limit:
                        break
                    # payload already parsed for filtering; pass both raw + parsed
                    nodes.append(
                        Node(
                            tenant_id=row["tenant_id"],
                            node_id=row["node_id"],
                            type_id=row["type_id"],
                            payload=payload,
                            created_at=row["created_at"],
                            updated_at=row["updated_at"],
                            owner_actor=row["owner_actor"],
                            acl_json=row["acl_blob"],
                        )
                    )
                return nodes
            else:
                # No filter -- use SQL LIMIT/OFFSET (efficient)
                cursor = conn.execute(
                    f"SELECT * FROM nodes WHERE tenant_id = ? AND type_id = ? "
                    f"ORDER BY {order_by} {order} LIMIT ? OFFSET ?",
                    (tenant_id, type_id, limit, offset),
                )
                return [
                    Node.from_row(
                        tenant_id=row["tenant_id"],
                        node_id=row["node_id"],
                        type_id=row["type_id"],
                        payload_json=row["payload_json"],
                        created_at=row["created_at"],
                        updated_at=row["updated_at"],
                        owner_actor=row["owner_actor"],
                        acl_json=row["acl_blob"],
                    )
                    for row in cursor.fetchall()
                ]

    async def query_nodes(
        self,
        tenant_id: str,
        type_id: int,
        filter_json: dict[str, Any] | None = None,
        limit: int = 100,
        offset: int = 0,
        order_by: str = "created_at",
        descending: bool = True,
    ) -> list[Node]:
        """Query nodes with optional payload filtering."""
        return await self._run_sync(
            self._sync_query_nodes,
            tenant_id,
            type_id,
            filter_json,
            limit,
            offset,
            order_by,
            descending,
        )

    # ── create_edge ────────────────────────────────────────────────────

    def _sync_create_edge(
        self,
        tenant_id: str,
        edge_type_id: int,
        from_node_id: str,
        to_node_id: str,
        props: dict[str, Any],
        now: int,
        propagates_acl: bool = False,
    ) -> Edge:
        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO edges
                (tenant_id, edge_type_id, from_node_id, to_node_id, props_json,
                 propagates_acl, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    tenant_id,
                    edge_type_id,
                    from_node_id,
                    to_node_id,
                    json.dumps(props),
                    1 if propagates_acl else 0,
                    now,
                ),
            )

        logger.debug(
            "Created edge",
            extra={
                "tenant_id": tenant_id,
                "edge_type_id": edge_type_id,
                "from": from_node_id,
                "to": to_node_id,
            },
        )

        return Edge(
            tenant_id=tenant_id,
            edge_type_id=edge_type_id,
            from_node_id=from_node_id,
            to_node_id=to_node_id,
            props=props,
            created_at=now,
        )

    async def create_edge(
        self,
        tenant_id: str,
        edge_type_id: int,
        from_node_id: str,
        to_node_id: str,
        props: dict[str, Any] | None = None,
        created_at: int | None = None,
        propagates_acl: bool = False,
    ) -> Edge:
        """Create an edge between nodes.

        Args:
            tenant_id: Tenant identifier
            edge_type_id: Edge type identifier
            from_node_id: Source node ID
            to_node_id: Target node ID
            props: Edge properties
            created_at: Optional creation timestamp
            propagates_acl: Whether this edge propagates ACL inheritance

        Returns:
            Created Edge object
        """
        now = created_at or int(time.time() * 1000)
        props = props or {}

        return await self._run_sync(
            self._sync_create_edge,
            tenant_id,
            edge_type_id,
            from_node_id,
            to_node_id,
            props,
            now,
            propagates_acl,
        )

    # ── delete_edge ────────────────────────────────────────────────────

    def _sync_delete_edge(
        self,
        tenant_id: str,
        edge_type_id: int,
        from_node_id: str,
        to_node_id: str,
    ) -> bool:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                """
                DELETE FROM edges
                WHERE tenant_id = ? AND edge_type_id = ? AND from_node_id = ? AND to_node_id = ?
                """,
                (tenant_id, edge_type_id, from_node_id, to_node_id),
            )
            return cursor.rowcount > 0

    async def delete_edge(
        self,
        tenant_id: str,
        edge_type_id: int,
        from_node_id: str,
        to_node_id: str,
    ) -> bool:
        """Delete an edge.

        Args:
            tenant_id: Tenant identifier
            edge_type_id: Edge type identifier
            from_node_id: Source node ID
            to_node_id: Target node ID

        Returns:
            True if deleted, False if not found
        """
        return await self._run_sync(
            self._sync_delete_edge,
            tenant_id,
            edge_type_id,
            from_node_id,
            to_node_id,
        )

    # ── get_edges_from ─────────────────────────────────────────────────

    def _sync_get_edges_from(
        self,
        tenant_id: str,
        node_id: str,
        edge_type_id: int | None,
    ) -> list[Edge]:
        with self._get_connection(tenant_id) as conn:
            if edge_type_id is not None:
                cursor = conn.execute(
                    """
                    SELECT * FROM edges
                    WHERE tenant_id = ? AND from_node_id = ? AND edge_type_id = ?
                    """,
                    (tenant_id, node_id, edge_type_id),
                )
            else:
                cursor = conn.execute(
                    "SELECT * FROM edges WHERE tenant_id = ? AND from_node_id = ?",
                    (tenant_id, node_id),
                )

            return [
                Edge(
                    tenant_id=row["tenant_id"],
                    edge_type_id=row["edge_type_id"],
                    from_node_id=row["from_node_id"],
                    to_node_id=row["to_node_id"],
                    props=json.loads(row["props_json"]),
                    created_at=row["created_at"],
                )
                for row in cursor.fetchall()
            ]

    async def get_edges_from(
        self,
        tenant_id: str,
        node_id: str,
        edge_type_id: int | None = None,
    ) -> list[Edge]:
        """Get outgoing edges from a node.

        Args:
            tenant_id: Tenant identifier
            node_id: Source node ID
            edge_type_id: Optional filter by edge type

        Returns:
            List of edges
        """
        return await self._run_sync(self._sync_get_edges_from, tenant_id, node_id, edge_type_id)

    # ── get_edges_to ───────────────────────────────────────────────────

    def _sync_get_edges_to(
        self,
        tenant_id: str,
        node_id: str,
        edge_type_id: int | None,
    ) -> list[Edge]:
        with self._get_connection(tenant_id) as conn:
            if edge_type_id is not None:
                cursor = conn.execute(
                    """
                    SELECT * FROM edges
                    WHERE tenant_id = ? AND to_node_id = ? AND edge_type_id = ?
                    """,
                    (tenant_id, node_id, edge_type_id),
                )
            else:
                cursor = conn.execute(
                    "SELECT * FROM edges WHERE tenant_id = ? AND to_node_id = ?",
                    (tenant_id, node_id),
                )

            return [
                Edge(
                    tenant_id=row["tenant_id"],
                    edge_type_id=row["edge_type_id"],
                    from_node_id=row["from_node_id"],
                    to_node_id=row["to_node_id"],
                    props=json.loads(row["props_json"]),
                    created_at=row["created_at"],
                )
                for row in cursor.fetchall()
            ]

    async def get_edges_to(
        self,
        tenant_id: str,
        node_id: str,
        edge_type_id: int | None = None,
    ) -> list[Edge]:
        """Get incoming edges to a node.

        Args:
            tenant_id: Tenant identifier
            node_id: Target node ID
            edge_type_id: Optional filter by edge type

        Returns:
            List of edges
        """
        return await self._run_sync(self._sync_get_edges_to, tenant_id, node_id, edge_type_id)

    # ── remaining async methods (not in critical list) ─────────────────

    def _sync_get_visible_nodes(
        self,
        tenant_id: str,
        principal: str,
        type_id: int | None,
        limit: int,
        offset: int,
    ) -> list[Node]:
        with self._get_connection(tenant_id) as conn:
            # Build query based on principal patterns
            # Supports: exact match, owner match, tenant wildcard
            query = """
                SELECT DISTINCT n.* FROM nodes n
                LEFT JOIN node_visibility v ON n.tenant_id = v.tenant_id AND n.node_id = v.node_id
                WHERE n.tenant_id = ?
                AND (
                    n.owner_actor = ?
                    OR v.principal = ?
                    OR v.principal = 'tenant:*'
                )
            """
            params: list[Any] = [tenant_id, principal, principal]

            if type_id is not None:
                query += " AND n.type_id = ?"
                params.append(type_id)

            query += " ORDER BY n.created_at DESC LIMIT ? OFFSET ?"
            params.extend([limit, offset])

            cursor = conn.execute(query, params)

            return [
                Node.from_row(
                    tenant_id=row["tenant_id"],
                    node_id=row["node_id"],
                    type_id=row["type_id"],
                    payload_json=row["payload_json"],
                    created_at=row["created_at"],
                    updated_at=row["updated_at"],
                    owner_actor=row["owner_actor"],
                    acl_json=row["acl_blob"],
                )
                for row in cursor.fetchall()
            ]

    async def get_visible_nodes(
        self,
        tenant_id: str,
        principal: str,
        type_id: int | None = None,
        limit: int = 100,
        offset: int = 0,
    ) -> list[Node]:
        """Get nodes visible to a principal.

        Args:
            tenant_id: Tenant identifier
            principal: Actor identifier
            type_id: Optional filter by type
            limit: Maximum nodes to return
            offset: Pagination offset

        Returns:
            List of visible nodes
        """
        return await self._run_sync(
            self._sync_get_visible_nodes, tenant_id, principal, type_id, limit, offset
        )

    def _update_visibility(
        self,
        conn: sqlite3.Connection,
        tenant_id: str,
        node_id: str,
        owner_actor: str,
        acl: list[dict[str, str]],
    ) -> None:
        """Update visibility index for a node.

        Args:
            conn: Database connection
            tenant_id: Tenant identifier
            node_id: Node identifier
            owner_actor: Node owner
            acl: Access control list
        """
        # Clear existing visibility
        conn.execute(
            "DELETE FROM node_visibility WHERE tenant_id = ? AND node_id = ?",
            (tenant_id, node_id),
        )

        # Add visibility for owner
        conn.execute(
            "INSERT INTO node_visibility (tenant_id, node_id, principal) VALUES (?, ?, ?)",
            (tenant_id, node_id, owner_actor),
        )

        # Add visibility for each ACL entry
        for entry in acl:
            principal = entry.get("principal")
            if principal and principal != owner_actor:
                conn.execute(
                    "INSERT OR IGNORE INTO node_visibility (tenant_id, node_id, principal) VALUES (?, ?, ?)",
                    (tenant_id, node_id, principal),
                )

    # ── ACL Engine ─────────────────────────────────────────────────────

    SYSTEM_ACTOR = "__system__"
    _ACL_MAX_DEPTH = 10

    def _sync_resolve_actor_groups(
        self,
        tenant_id: str,
        actor_id: str,
    ) -> list[str]:
        """Resolve all group memberships for an actor, recursively.

        Returns a flat list: [actor_id, group_1, group_2, ...]
        """
        with self._get_connection(tenant_id) as conn:
            rows = conn.execute(
                """
                WITH RECURSIVE membership(gid, depth) AS (
                    SELECT group_id, 0 FROM group_users
                    WHERE member_actor_id = ?
                    UNION ALL
                    SELECT gu.group_id, m.depth + 1
                    FROM group_users gu
                    JOIN membership m ON gu.member_actor_id = m.gid
                    WHERE m.depth < 10
                )
                SELECT DISTINCT gid FROM membership
                """,
                (actor_id,),
            ).fetchall()
        return [actor_id] + [r[0] for r in rows]

    async def resolve_actor_groups(
        self,
        tenant_id: str,
        actor_id: str,
    ) -> list[str]:
        """Resolve actor + all group memberships. Call once per request."""
        return await self._run_sync(
            self._sync_resolve_actor_groups,
            tenant_id,
            actor_id,
        )

    def _sync_can_access(
        self,
        tenant_id: str,
        node_id: str,
        actor_ids: list[str],
    ) -> bool:
        """Check if any of the actor_ids can access node_id.

        Walks the acl_inherit chain upward. Checks at each ancestor:
        - owner match
        - visibility index (includes tenant:* wildcard)
        - node_access table
        - DENY overrides

        actor_ids is the pre-resolved list from resolve_actor_groups.
        """
        if self.SYSTEM_ACTOR in actor_ids:
            return True

        # Include tenant:* in visibility check (wildcard principal)
        visibility_ids = list(actor_ids) + ["tenant:*"]

        with self._get_connection(tenant_id) as conn:
            placeholders_actor = ",".join("?" for _ in actor_ids)
            placeholders_vis = ",".join("?" for _ in visibility_ids)

            # Check for explicit DENY on the target node first
            deny_row = conn.execute(
                f"""
                SELECT 1 FROM node_access
                WHERE node_id = ? AND actor_id IN ({placeholders_actor})
                  AND permission = 'deny'
                LIMIT 1
                """,
                (node_id, *actor_ids),
            ).fetchone()
            if deny_row:
                return False

            # Walk ancestors via acl_inherit, check access at each level
            row = conn.execute(
                f"""
                WITH RECURSIVE ancestry(nid, depth) AS (
                    SELECT ?, 0
                    UNION ALL
                    SELECT ai.inherit_from, a.depth + 1
                    FROM acl_inherit ai
                    JOIN ancestry a ON ai.node_id = a.nid
                    WHERE a.depth < {self._ACL_MAX_DEPTH}
                )
                SELECT 1 FROM ancestry anc
                JOIN nodes n ON n.node_id = anc.nid AND n.tenant_id = ?
                WHERE (
                    n.owner_actor IN ({placeholders_actor})
                    OR EXISTS (
                        SELECT 1 FROM node_visibility nv
                        WHERE nv.tenant_id = ? AND nv.node_id = anc.nid
                          AND nv.principal IN ({placeholders_vis})
                    )
                    OR EXISTS (
                        SELECT 1 FROM node_access na
                        WHERE na.node_id = anc.nid
                          AND na.actor_id IN ({placeholders_actor})
                          AND na.permission != 'deny'
                          AND (na.expires_at IS NULL OR na.expires_at > ?)
                    )
                )
                LIMIT 1
                """,
                (
                    node_id,
                    tenant_id,
                    *actor_ids,
                    tenant_id,
                    *visibility_ids,
                    *actor_ids,
                    int(time.time() * 1000),
                ),
            ).fetchone()
            return row is not None

    async def can_access(
        self,
        tenant_id: str,
        node_id: str,
        actor_ids: list[str],
    ) -> bool:
        """Check if actor can access a node (via inheritance chain)."""
        return await self._run_sync(
            self._sync_can_access,
            tenant_id,
            node_id,
            actor_ids,
        )

    def _sync_share_node(
        self,
        tenant_id: str,
        node_id: str,
        actor_id: str,
        actor_type: str,
        permission: str,
        granted_by: str,
        now: int,
        expires_at: int | None,
    ) -> None:
        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO node_access
                (node_id, actor_id, actor_type, permission, granted_by, granted_at, expires_at)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (node_id, actor_id, actor_type, permission, granted_by, now, expires_at),
            )

    async def share_node(
        self,
        tenant_id: str,
        node_id: str,
        actor_id: str,
        permission: str,
        granted_by: str,
        *,
        actor_type: str = "user",
        expires_at: int | None = None,
    ) -> None:
        """Grant direct access to a node."""
        now = int(time.time() * 1000)
        await self._run_sync(
            self._sync_share_node,
            tenant_id,
            node_id,
            actor_id,
            actor_type,
            permission,
            granted_by,
            now,
            expires_at,
        )

    def _sync_revoke_access(
        self,
        tenant_id: str,
        node_id: str,
        actor_id: str,
    ) -> bool:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "DELETE FROM node_access WHERE node_id = ? AND actor_id = ?",
                (node_id, actor_id),
            )
            return cursor.rowcount > 0

    async def revoke_access(
        self,
        tenant_id: str,
        node_id: str,
        actor_id: str,
    ) -> bool:
        """Remove direct access from a node. Returns True if a grant existed."""
        return await self._run_sync(
            self._sync_revoke_access,
            tenant_id,
            node_id,
            actor_id,
        )

    def _sync_get_connected_nodes(
        self,
        tenant_id: str,
        node_id: str,
        edge_type_id: int,
        actor_ids: list[str],
        limit: int,
        offset: int,
    ) -> list:
        """Get connected nodes via edge type, with ACL filtering.

        If the edge type propagates ACL and the actor can access the source node,
        all connected nodes are returned (minus explicit denies).
        """
        if self.SYSTEM_ACTOR in actor_ids:
            # System actor: no ACL filtering
            return self._sync_get_edges_and_nodes(
                tenant_id,
                node_id,
                edge_type_id,
                limit,
                offset,
            )

        # First: can the actor access the source node?
        if not self._sync_can_access(tenant_id, node_id, actor_ids):
            return []

        placeholders = ",".join("?" for _ in actor_ids)

        with self._get_connection(tenant_id) as conn:
            # Check if this edge type propagates ACL
            edge_row = conn.execute(
                """
                SELECT propagates_acl FROM edges
                WHERE tenant_id = ? AND edge_type_id = ?
                  AND from_node_id = ?
                LIMIT 1
                """,
                (tenant_id, edge_type_id, node_id),
            ).fetchone()

            propagates = edge_row and edge_row[0] == 1 if edge_row else False

            if propagates:
                # Fast path: actor has access to source, edge propagates —
                # return all connected nodes, just filter explicit denies
                rows = conn.execute(
                    f"""
                    SELECT n.* FROM nodes n
                    JOIN edges e ON e.to_node_id = n.node_id
                      AND e.tenant_id = n.tenant_id
                    WHERE e.tenant_id = ?
                      AND e.edge_type_id = ?
                      AND e.from_node_id = ?
                      AND NOT EXISTS (
                          SELECT 1 FROM node_access na
                          WHERE na.node_id = n.node_id
                            AND na.actor_id IN ({placeholders})
                            AND na.permission = 'deny'
                      )
                    ORDER BY n.created_at DESC
                    LIMIT ? OFFSET ?
                    """,
                    (tenant_id, edge_type_id, node_id, *actor_ids, limit, offset),
                ).fetchall()
            else:
                # Slow path: per-node ACL check via visibility index
                rows = conn.execute(
                    f"""
                    SELECT n.* FROM nodes n
                    JOIN edges e ON e.to_node_id = n.node_id
                      AND e.tenant_id = n.tenant_id
                    WHERE e.tenant_id = ?
                      AND e.edge_type_id = ?
                      AND e.from_node_id = ?
                      AND (
                          n.owner_actor IN ({placeholders})
                          OR EXISTS (
                              SELECT 1 FROM node_visibility nv
                              WHERE nv.tenant_id = n.tenant_id
                                AND nv.node_id = n.node_id
                                AND nv.principal IN ({placeholders})
                          )
                          OR EXISTS (
                              SELECT 1 FROM node_access na
                              WHERE na.node_id = n.node_id
                                AND na.actor_id IN ({placeholders})
                                AND na.permission != 'deny'
                          )
                      )
                    ORDER BY n.created_at DESC
                    LIMIT ? OFFSET ?
                    """,
                    (
                        tenant_id,
                        edge_type_id,
                        node_id,
                        *actor_ids,
                        *actor_ids,
                        *actor_ids,
                        limit,
                        offset,
                    ),
                ).fetchall()

            return [
                Node.from_row(
                    tenant_id=r["tenant_id"],
                    node_id=r["node_id"],
                    type_id=r["type_id"],
                    payload_json=r["payload_json"],
                    created_at=r["created_at"],
                    updated_at=r["updated_at"],
                    owner_actor=r["owner_actor"],
                    acl_json=r["acl_blob"],
                )
                for r in rows
            ]

    def _sync_get_edges_and_nodes(
        self,
        tenant_id: str,
        node_id: str,
        edge_type_id: int,
        limit: int,
        offset: int,
    ) -> list:
        """Get connected nodes without ACL filtering (for system actor)."""
        with self._get_connection(tenant_id) as conn:
            rows = conn.execute(
                """
                SELECT n.* FROM nodes n
                JOIN edges e ON e.to_node_id = n.node_id
                  AND e.tenant_id = n.tenant_id
                WHERE e.tenant_id = ?
                  AND e.edge_type_id = ?
                  AND e.from_node_id = ?
                ORDER BY n.created_at DESC
                LIMIT ? OFFSET ?
                """,
                (tenant_id, edge_type_id, node_id, limit, offset),
            ).fetchall()
            return [
                Node.from_row(
                    tenant_id=r["tenant_id"],
                    node_id=r["node_id"],
                    type_id=r["type_id"],
                    payload_json=r["payload_json"],
                    created_at=r["created_at"],
                    updated_at=r["updated_at"],
                    owner_actor=r["owner_actor"],
                    acl_json=r["acl_blob"],
                )
                for r in rows
            ]

    async def get_connected_nodes(
        self,
        tenant_id: str,
        node_id: str,
        edge_type_id: int,
        actor_ids: list[str],
        limit: int = 100,
        offset: int = 0,
    ) -> list:
        """Fetch nodes connected via edge type, with ACL filtering."""
        return await self._run_sync(
            self._sync_get_connected_nodes,
            tenant_id,
            node_id,
            edge_type_id,
            actor_ids,
            limit,
            offset,
        )

    def _sync_list_shared_with_me(
        self,
        tenant_id: str,
        actor_ids: list[str],
        limit: int,
        offset: int,
    ) -> list:
        """List nodes explicitly shared with the actor."""
        placeholders = ",".join("?" for _ in actor_ids)
        with self._get_connection(tenant_id) as conn:
            rows = conn.execute(
                f"""
                SELECT DISTINCT n.* FROM nodes n
                JOIN node_access na ON na.node_id = n.node_id
                WHERE n.tenant_id = ?
                  AND na.actor_id IN ({placeholders})
                  AND na.permission != 'deny'
                  AND (na.expires_at IS NULL OR na.expires_at > ?)
                ORDER BY na.granted_at DESC
                LIMIT ? OFFSET ?
                """,
                (tenant_id, *actor_ids, int(time.time() * 1000), limit, offset),
            ).fetchall()
            return [
                Node.from_row(
                    tenant_id=r["tenant_id"],
                    node_id=r["node_id"],
                    type_id=r["type_id"],
                    payload_json=r["payload_json"],
                    created_at=r["created_at"],
                    updated_at=r["updated_at"],
                    owner_actor=r["owner_actor"],
                    acl_json=r["acl_blob"],
                )
                for r in rows
            ]

    async def list_shared_with_me(
        self,
        tenant_id: str,
        actor_ids: list[str],
        limit: int = 100,
        offset: int = 0,
    ) -> list:
        """List nodes explicitly shared with actor (direct + via groups)."""
        return await self._run_sync(
            self._sync_list_shared_with_me,
            tenant_id,
            actor_ids,
            limit,
            offset,
        )

    def _sync_add_group_member(
        self,
        tenant_id: str,
        group_id: str,
        member_actor_id: str,
        role: str,
        now: int,
    ) -> None:
        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO group_users
                (group_id, member_actor_id, role, joined_at)
                VALUES (?, ?, ?, ?)
                """,
                (group_id, member_actor_id, role, now),
            )

    async def add_group_member(
        self,
        tenant_id: str,
        group_id: str,
        member_actor_id: str,
        role: str = "member",
    ) -> None:
        """Add a member to a group."""
        now = int(time.time() * 1000)
        await self._run_sync(
            self._sync_add_group_member,
            tenant_id,
            group_id,
            member_actor_id,
            role,
            now,
        )

    def _sync_remove_group_member(
        self,
        tenant_id: str,
        group_id: str,
        member_actor_id: str,
    ) -> bool:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "DELETE FROM group_users WHERE group_id = ? AND member_actor_id = ?",
                (group_id, member_actor_id),
            )
            return cursor.rowcount > 0

    async def remove_group_member(
        self,
        tenant_id: str,
        group_id: str,
        member_actor_id: str,
    ) -> bool:
        """Remove a member from a group. Returns True if member existed."""
        return await self._run_sync(
            self._sync_remove_group_member,
            tenant_id,
            group_id,
            member_actor_id,
        )

    def _sync_get_group_members(
        self,
        tenant_id: str,
        group_id: str,
    ) -> list[str]:
        with self._get_connection(tenant_id) as conn:
            rows = conn.execute(
                "SELECT member_actor_id FROM group_users WHERE group_id = ?",
                (group_id,),
            ).fetchall()
            return [r[0] for r in rows]

    async def get_group_members(
        self,
        tenant_id: str,
        group_id: str,
    ) -> list[str]:
        """Get all members of a group.

        Args:
            tenant_id: Tenant identifier
            group_id: Group identifier

        Returns:
            List of member actor IDs
        """
        return await self._run_sync(
            self._sync_get_group_members,
            tenant_id,
            group_id,
        )

    def _sync_list_node_access_for_group(
        self,
        tenant_id: str,
        group_id: str,
    ) -> list[dict]:
        """List all node_access entries where the actor_id is a given group."""
        with self._get_connection(tenant_id) as conn:
            rows = conn.execute(
                """
                SELECT node_id, actor_id, permission, granted_by, granted_at, expires_at
                FROM node_access
                WHERE actor_id = ?
                """,
                (group_id,),
            ).fetchall()
            return [dict(r) for r in rows]

    async def list_node_access_for_group(
        self,
        tenant_id: str,
        group_id: str,
    ) -> list[dict]:
        """List all node_access entries granted to a group.

        Used when group membership changes to cascade shared_index updates.

        Args:
            tenant_id: Tenant identifier
            group_id: Group identifier (e.g. "group:friends")

        Returns:
            List of node_access dicts with node_id, permission, etc.
        """
        return await self._run_sync(
            self._sync_list_node_access_for_group,
            tenant_id,
            group_id,
        )

    def _sync_transfer_ownership(
        self,
        tenant_id: str,
        node_id: str,
        new_owner: str,
    ) -> bool:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "UPDATE nodes SET owner_actor = ? WHERE tenant_id = ? AND node_id = ?",
                (new_owner, tenant_id, node_id),
            )
            if cursor.rowcount > 0:
                self._update_visibility(
                    conn,
                    tenant_id,
                    node_id,
                    new_owner,
                    self._get_node_acl(conn, tenant_id, node_id),
                )
            return cursor.rowcount > 0

    def _get_node_acl(
        self,
        conn: sqlite3.Connection,
        tenant_id: str,
        node_id: str,
    ) -> list[dict[str, str]]:
        row = conn.execute(
            "SELECT acl_blob FROM nodes WHERE tenant_id = ? AND node_id = ?",
            (tenant_id, node_id),
        ).fetchone()
        if not row:
            return []
        return json.loads(row[0]) if row[0] else []

    async def transfer_ownership(
        self,
        tenant_id: str,
        node_id: str,
        new_owner: str,
    ) -> bool:
        """Transfer ownership of a node. Returns True if node exists."""
        return await self._run_sync(
            self._sync_transfer_ownership,
            tenant_id,
            node_id,
            new_owner,
        )

    # ── Admin operations (Issue #90) ────────────────────────────────────

    def _sync_transfer_user_content(
        self,
        tenant_id: str,
        from_user: str,
        to_user: str,
    ) -> int:
        """Reassign all nodes owned by from_user to to_user.

        Returns the number of nodes updated. Also refreshes the visibility
        index for each affected node so that the new owner appears as a
        principal and the old owner is removed (unless they had explicit
        access via ACL).
        """
        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                rows = conn.execute(
                    "SELECT node_id, acl_blob FROM nodes "
                    "WHERE tenant_id = ? AND owner_actor = ?",
                    (tenant_id, from_user),
                ).fetchall()
                cursor = conn.execute(
                    "UPDATE nodes SET owner_actor = ? "
                    "WHERE tenant_id = ? AND owner_actor = ?",
                    (to_user, tenant_id, from_user),
                )
                count = cursor.rowcount
                for row in rows:
                    acl = json.loads(row["acl_blob"]) if row["acl_blob"] else []
                    self._update_visibility(
                        conn, tenant_id, row["node_id"], to_user, acl,
                    )
                conn.execute("COMMIT")
                return count
            except Exception:
                conn.execute("ROLLBACK")
                raise

    async def transfer_user_content(
        self,
        tenant_id: str,
        from_user: str,
        to_user: str,
        actor: str,
    ) -> dict:
        """Reassign all nodes where owner_actor=from_user to to_user.

        Use case: employee offboarding, where their manager takes
        ownership of all their work. Logs the operation to the audit
        log with action ``transfer_content``.

        Args:
            tenant_id: Tenant identifier
            from_user: Current owner actor (e.g. ``user:alice``)
            to_user: New owner actor (e.g. ``user:bob``)
            actor: Admin actor performing the operation

        Returns:
            dict with keys ``transferred`` (count), ``tenant_id``,
            ``from``, ``to``.
        """
        count = await self._run_sync(
            self._sync_transfer_user_content,
            tenant_id,
            from_user,
            to_user,
        )
        import json as _json
        await self.append_audit(
            tenant_id=tenant_id,
            actor_id=actor,
            action="transfer_content",
            target_type="user",
            target_id=from_user,
            metadata=_json.dumps({
                "from_user": from_user,
                "to_user": to_user,
                "transferred": count,
            }),
        )
        return {
            "transferred": count,
            "tenant_id": tenant_id,
            "from": from_user,
            "to": to_user,
        }

    def _sync_delegate_access(
        self,
        tenant_id: str,
        from_user: str,
        to_user: str,
        permission: str,
        expires_at: int | None,
        granted_by: str,
        now: int,
    ) -> int:
        """Grant to_user the given permission on every node owned by from_user."""
        with self._get_connection(tenant_id) as conn:
            rows = conn.execute(
                "SELECT node_id FROM nodes "
                "WHERE tenant_id = ? AND owner_actor = ?",
                (tenant_id, from_user),
            ).fetchall()
            count = 0
            for row in rows:
                conn.execute(
                    """
                    INSERT OR REPLACE INTO node_access
                    (node_id, actor_id, actor_type, permission,
                     granted_by, granted_at, expires_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                    """,
                    (
                        row["node_id"],
                        to_user,
                        "user",
                        permission,
                        granted_by,
                        now,
                        expires_at,
                    ),
                )
                count += 1
            return count

    async def delegate_access(
        self,
        tenant_id: str,
        from_user: str,
        to_user: str,
        permission: str,
        expires_at: int | None,
        actor: str,
    ) -> dict:
        """Grant to_user temporary access to all nodes owned by from_user.

        Args:
            tenant_id: Tenant identifier
            from_user: Content owner whose nodes are being delegated
            to_user: Recipient of the delegation
            permission: Permission level (``read``, ``write``, ...)
            expires_at: Expiry timestamp in Unix ms, or None for permanent
            actor: Admin actor performing the operation

        Returns:
            dict with keys ``delegated`` (count), ``expires_at``.
        """
        now = int(time.time() * 1000)
        count = await self._run_sync(
            self._sync_delegate_access,
            tenant_id,
            from_user,
            to_user,
            permission,
            expires_at,
            actor,
            now,
        )
        import json as _json
        await self.append_audit(
            tenant_id=tenant_id,
            actor_id=actor,
            action="delegate_access",
            target_type="user",
            target_id=from_user,
            metadata=_json.dumps({
                "from_user": from_user,
                "to_user": to_user,
                "permission": permission,
                "expires_at": expires_at,
                "delegated": count,
            }),
        )
        return {
            "delegated": count,
            "expires_at": expires_at,
            "tenant_id": tenant_id,
            "from": from_user,
            "to": to_user,
        }

    def _sync_revoke_user_access(
        self,
        tenant_id: str,
        user_id: str,
    ) -> tuple[int, int]:
        """Remove all node_access grants and group memberships for user.

        Returns (revoked_grants, revoked_groups).
        """
        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                grants = conn.execute(
                    "DELETE FROM node_access WHERE actor_id = ?",
                    (user_id,),
                ).rowcount
                groups = conn.execute(
                    "DELETE FROM group_users WHERE member_actor_id = ?",
                    (user_id,),
                ).rowcount
                # Also remove any visibility rows that were granted via
                # direct access (owner visibility rows are re-generated
                # lazily on next write, so we don't touch those).
                conn.execute(
                    "DELETE FROM node_visibility "
                    "WHERE tenant_id = ? AND principal = ?",
                    (tenant_id, user_id),
                )
                conn.execute("COMMIT")
                return (grants, groups)
            except Exception:
                conn.execute("ROLLBACK")
                raise

    async def revoke_user_access(
        self,
        tenant_id: str,
        user_id: str,
        actor: str,
    ) -> dict:
        """Remove ALL access for a user in a tenant. Instant termination.

        Removes:
            - All ``node_access`` grants (direct + via group-id entries
              whose actor_id equals ``user_id``)
            - All ``group_users`` rows where the user is a member
            - All ``node_visibility`` rows naming the user as principal

        Args:
            tenant_id: Tenant identifier
            user_id: User to revoke (e.g. ``user:alice``)
            actor: Admin actor performing the operation

        Returns:
            dict with keys ``revoked_grants``, ``revoked_groups``.
        """
        grants, groups = await self._run_sync(
            self._sync_revoke_user_access,
            tenant_id,
            user_id,
        )
        import json as _json
        await self.append_audit(
            tenant_id=tenant_id,
            actor_id=actor,
            action="revoke_user_access",
            target_type="user",
            target_id=user_id,
            metadata=_json.dumps({
                "user_id": user_id,
                "revoked_grants": grants,
                "revoked_groups": groups,
            }),
        )
        return {
            "revoked_grants": grants,
            "revoked_groups": groups,
            "tenant_id": tenant_id,
            "user_id": user_id,
        }

    # ── Cross-tenant access helpers ─────────────────────────────────────

    def _sync_has_node_access(
        self,
        tenant_id: str,
        actor_ids: list[str],
    ) -> bool:
        """Check if any of actor_ids has at least one node_access entry in tenant."""
        placeholders = ",".join("?" for _ in actor_ids)
        with self._get_connection(tenant_id) as conn:
            row = conn.execute(
                f"""
                SELECT 1 FROM node_access
                WHERE actor_id IN ({placeholders})
                  AND permission != 'deny'
                  AND (expires_at IS NULL OR expires_at > ?)
                LIMIT 1
                """,
                (*actor_ids, int(time.time() * 1000)),
            ).fetchone()
            return row is not None

    async def has_node_access(
        self,
        tenant_id: str,
        actor_ids: list[str],
    ) -> bool:
        """Check if actor has at least one node_access entry in the tenant.

        Used for cross-tenant QueryNodes authorization: the actor is not
        a tenant member but may have explicit node_access grants.

        Args:
            tenant_id: Tenant identifier
            actor_ids: Pre-resolved list from resolve_actor_groups

        Returns:
            True if at least one non-deny, non-expired entry exists
        """
        return await self._run_sync(
            self._sync_has_node_access,
            tenant_id,
            actor_ids,
        )

    def _sync_has_node_access_for_node(
        self,
        tenant_id: str,
        node_id: str,
        actor_ids: list[str],
    ) -> bool:
        """Check if any of actor_ids has node_access for a specific node."""
        placeholders = ",".join("?" for _ in actor_ids)
        with self._get_connection(tenant_id) as conn:
            row = conn.execute(
                f"""
                SELECT 1 FROM node_access
                WHERE node_id = ?
                  AND actor_id IN ({placeholders})
                  AND permission != 'deny'
                  AND (expires_at IS NULL OR expires_at > ?)
                LIMIT 1
                """,
                (node_id, *actor_ids, int(time.time() * 1000)),
            ).fetchone()
            return row is not None

    async def has_node_access_for_node(
        self,
        tenant_id: str,
        node_id: str,
        actor_ids: list[str],
    ) -> bool:
        """Check if actor has explicit node_access for a specific node.

        Used for cross-tenant GetNode authorization: the actor is not
        a tenant member but may have a direct node_access grant.

        Args:
            tenant_id: Tenant identifier
            node_id: Node identifier
            actor_ids: Pre-resolved list from resolve_actor_groups

        Returns:
            True if a non-deny, non-expired entry exists for the node
        """
        return await self._run_sync(
            self._sync_has_node_access_for_node,
            tenant_id,
            node_id,
            actor_ids,
        )

    def list_tenants(self) -> list[str]:
        """Scan data_dir for tenant database files and return tenant IDs.

        Returns:
            List of tenant ID strings.
        """
        tenants = []
        if not self.data_dir.exists():
            return tenants
        for p in sorted(self.data_dir.glob("tenant_*.db")):
            name = p.stem  # e.g. "tenant_myapp"
            tenant_id = name[len("tenant_") :]
            if tenant_id:
                tenants.append(tenant_id)
        return tenants

    def _sync_get_distinct_type_ids(self, tenant_id: str) -> list[dict[str, Any]]:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                """SELECT type_id, COUNT(*) as node_count
                   FROM nodes WHERE tenant_id = ?
                   GROUP BY type_id ORDER BY type_id""",
                (tenant_id,),
            )
            return [
                {"type_id": row["type_id"], "node_count": row["node_count"]}
                for row in cursor.fetchall()
            ]

    async def get_distinct_type_ids(self, tenant_id: str) -> list[dict[str, Any]]:
        """Get distinct node type_ids with counts for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            List of dicts with "type_id" and "node_count" keys.
        """
        return await self._run_sync(self._sync_get_distinct_type_ids, tenant_id)

    def _sync_get_distinct_edge_type_ids(self, tenant_id: str) -> list[dict[str, Any]]:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                """SELECT edge_type_id, COUNT(*) as edge_count
                   FROM edges WHERE tenant_id = ?
                   GROUP BY edge_type_id ORDER BY edge_type_id""",
                (tenant_id,),
            )
            return [
                {
                    "edge_type_id": row["edge_type_id"],
                    "edge_count": row["edge_count"],
                }
                for row in cursor.fetchall()
            ]

    async def get_distinct_edge_type_ids(self, tenant_id: str) -> list[dict[str, Any]]:
        """Get distinct edge type_ids with counts for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            List of dicts with "edge_type_id" and "edge_count" keys.
        """
        return await self._run_sync(self._sync_get_distinct_edge_type_ids, tenant_id)

    def _sync_get_stats(self, tenant_id: str) -> dict[str, int]:
        with self._get_connection(tenant_id) as conn:
            stats = {}

            cursor = conn.execute("SELECT COUNT(*) FROM nodes WHERE tenant_id = ?", (tenant_id,))
            stats["nodes"] = cursor.fetchone()[0]

            cursor = conn.execute("SELECT COUNT(*) FROM edges WHERE tenant_id = ?", (tenant_id,))
            stats["edges"] = cursor.fetchone()[0]

            cursor = conn.execute(
                "SELECT COUNT(*) FROM applied_events WHERE tenant_id = ?",
                (tenant_id,),
            )
            stats["applied_events"] = cursor.fetchone()[0]

            return stats

    async def get_stats(self, tenant_id: str) -> dict[str, int]:
        """Get statistics for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Dictionary with counts
        """
        return await self._run_sync(self._sync_get_stats, tenant_id)

    def _sync_get_last_applied_position(self, tenant_id: str) -> str | None:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                """
                SELECT stream_pos FROM applied_events
                WHERE tenant_id = ?
                ORDER BY applied_at DESC
                LIMIT 1
                """,
                (tenant_id,),
            )
            row = cursor.fetchone()
            return row[0] if row else None

    async def get_last_applied_position(self, tenant_id: str) -> str | None:
        """Get the last applied stream position.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Stream position string or None
        """
        return await self._run_sync(self._sync_get_last_applied_position, tenant_id)

    # ── audit log (hash-chained) ─────────────────────────────────────────

    def _sync_append_audit(
        self,
        tenant_id: str,
        actor_id: str,
        action: str,
        target_type: str,
        target_id: str,
        ip_address: str | None,
        user_agent: str | None,
        metadata: str | None,
    ) -> str:
        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                # 1. Get the last entry's hash (or "genesis" for the first)
                cursor = conn.execute(
                    """
                    SELECT event_id, prev_hash, actor_id, action,
                           target_type, target_id, created_at
                    FROM audit_log ORDER BY created_at ASC, rowid ASC
                    """,
                )
                rows = cursor.fetchall()

                if rows:
                    last = rows[-1]
                    prev_hash = _compute_entry_hash(
                        last["event_id"],
                        last["prev_hash"],
                        last["actor_id"],
                        last["action"],
                        last["target_type"],
                        last["target_id"],
                        last["created_at"],
                    )
                else:
                    prev_hash = "genesis"

                # 2. Create new entry
                event_id = str(uuid.uuid4())
                now = int(time.time() * 1000)

                conn.execute(
                    """
                    INSERT INTO audit_log
                        (event_id, prev_hash, actor_id, action, target_type,
                         target_id, ip_address, user_agent, metadata, created_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    """,
                    (
                        event_id,
                        prev_hash,
                        actor_id,
                        action,
                        target_type,
                        target_id,
                        ip_address,
                        user_agent,
                        metadata,
                        now,
                    ),
                )

                conn.execute("COMMIT")
                return event_id

            except Exception:
                conn.execute("ROLLBACK")
                raise

    async def append_audit(
        self,
        tenant_id: str,
        actor_id: str,
        action: str,
        target_type: str,
        target_id: str,
        ip_address: str | None = None,
        user_agent: str | None = None,
        metadata: str | None = None,
    ) -> str:
        """Append a tamper-evident audit entry.

        Each entry stores the SHA-256 hash of the previous entry's content,
        forming a hash chain.  Any modification or deletion of a prior entry
        will break the chain (detectable via ``verify_audit_chain``).

        Args:
            tenant_id: Tenant identifier
            actor_id: Who performed the action
            action: Action performed (e.g. "create_node")
            target_type: Type of target (e.g. "node")
            target_id: ID of target
            ip_address: Optional client IP
            user_agent: Optional client user-agent
            metadata: Optional JSON metadata string

        Returns:
            The new event_id.
        """
        return await self._run_sync(
            self._sync_append_audit,
            tenant_id,
            actor_id,
            action,
            target_type,
            target_id,
            ip_address,
            user_agent,
            metadata,
        )

    def _sync_get_audit_log(
        self,
        tenant_id: str,
        limit: int,
        offset: int,
        actor_id: str | None,
        action: str | None,
        after: int | None,
        before: int | None,
    ) -> list[dict]:
        with self._get_connection(tenant_id) as conn:
            clauses: list[str] = []
            params: list[Any] = []

            if actor_id is not None:
                clauses.append("actor_id = ?")
                params.append(actor_id)
            if action is not None:
                clauses.append("action = ?")
                params.append(action)
            if after is not None:
                clauses.append("created_at > ?")
                params.append(after)
            if before is not None:
                clauses.append("created_at < ?")
                params.append(before)

            where = ""
            if clauses:
                where = "WHERE " + " AND ".join(clauses)

            query = f"""
                SELECT event_id, prev_hash, actor_id, action, target_type,
                       target_id, ip_address, user_agent, metadata, created_at
                FROM audit_log
                {where}
                ORDER BY created_at DESC
                LIMIT ? OFFSET ?
            """
            params.extend([limit, offset])

            cursor = conn.execute(query, params)
            return [dict(row) for row in cursor.fetchall()]

    async def get_audit_log(
        self,
        tenant_id: str,
        limit: int = 100,
        offset: int = 0,
        actor_id: str | None = None,
        action: str | None = None,
        after: int | None = None,
        before: int | None = None,
    ) -> list[dict]:
        """Query audit log with optional filters.

        Args:
            tenant_id: Tenant identifier
            limit: Max entries to return
            offset: Pagination offset
            actor_id: Filter by actor
            action: Filter by action type
            after: Only entries after this timestamp (Unix ms)
            before: Only entries before this timestamp (Unix ms)

        Returns:
            List of audit entries (newest first).
        """
        return await self._run_sync(
            self._sync_get_audit_log,
            tenant_id,
            limit,
            offset,
            actor_id,
            action,
            after,
            before,
        )

    def _sync_verify_audit_chain(self, tenant_id: str) -> tuple[bool, str]:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                """
                SELECT event_id, prev_hash, actor_id, action,
                       target_type, target_id, created_at
                FROM audit_log
                ORDER BY created_at ASC, rowid ASC
                """,
            )
            rows = cursor.fetchall()

            if not rows:
                return (True, "ok")

            # First entry must have prev_hash == "genesis"
            first = rows[0]
            if first["prev_hash"] != "genesis":
                return (False, f"Break at event_id {first['event_id']}: genesis expected")

            for i in range(1, len(rows)):
                prev = rows[i - 1]
                curr = rows[i]

                expected_prev_hash = _compute_entry_hash(
                    prev["event_id"],
                    prev["prev_hash"],
                    prev["actor_id"],
                    prev["action"],
                    prev["target_type"],
                    prev["target_id"],
                    prev["created_at"],
                )

                if curr["prev_hash"] != expected_prev_hash:
                    return (False, f"Break at event_id {curr['event_id']}")

            return (True, "ok")

    async def verify_audit_chain(self, tenant_id: str) -> tuple[bool, str]:
        """Verify the entire audit chain is intact.

        Walks through all entries in chronological order and recomputes
        each entry's expected ``prev_hash``.  If any mismatch is found
        the chain has been tampered with.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Tuple of (valid, message).  ``valid`` is True when the chain
            is intact; ``message`` is "ok" or describes the break.
        """
        return await self._run_sync(self._sync_verify_audit_chain, tenant_id)

    def _sync_export_audit_log(
        self,
        tenant_id: str,
        after: int | None,
        before: int | None,
    ) -> list[dict]:
        with self._get_connection(tenant_id) as conn:
            clauses: list[str] = []
            params: list[Any] = []

            if after is not None:
                clauses.append("created_at > ?")
                params.append(after)
            if before is not None:
                clauses.append("created_at < ?")
                params.append(before)

            where = ""
            if clauses:
                where = "WHERE " + " AND ".join(clauses)

            query = f"""
                SELECT event_id, prev_hash, actor_id, action, target_type,
                       target_id, ip_address, user_agent, metadata, created_at
                FROM audit_log
                {where}
                ORDER BY created_at ASC, rowid ASC
            """

            cursor = conn.execute(query, params)
            return [dict(row) for row in cursor.fetchall()]

    async def export_audit_log(
        self,
        tenant_id: str,
        after: int | None = None,
        before: int | None = None,
    ) -> list[dict]:
        """Export audit entries for compliance.

        Returns entries in chronological order (oldest first).

        Args:
            tenant_id: Tenant identifier
            after: Only entries after this timestamp (Unix ms)
            before: Only entries before this timestamp (Unix ms)

        Returns:
            List of audit entries (oldest first).
        """
        return await self._run_sync(
            self._sync_export_audit_log,
            tenant_id,
            after,
            before,
        )

    def get_db_path(self, tenant_id: str) -> Path:
        """Get the database file path for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Path to database file
        """
        return self._get_db_path(tenant_id)

    def delete_tenant_database(self, tenant_id: str) -> bool:
        """Remove a tenant's SQLite database file from disk.

        Also closes and evicts any pooled connection for the tenant and
        best-effort removes the WAL/SHM sidecar files. Used by the GDPR
        deletion worker when a personal tenant is being erased.

        Args:
            tenant_id: Tenant identifier

        Returns:
            True if the database file existed and was removed.
        """
        db_path = self._get_db_path(tenant_id)
        cache_key = str(db_path)
        conn = self._connections.pop(cache_key, None)
        if conn is not None:
            with contextlib.suppress(Exception):
                conn.close()
        removed = False
        if db_path.exists():
            with contextlib.suppress(FileNotFoundError):
                db_path.unlink()
                removed = True
        # WAL / SHM sidecars
        for suffix in ("-wal", "-shm"):
            sidecar = db_path.with_name(db_path.name + suffix)
            if sidecar.exists():
                with contextlib.suppress(FileNotFoundError):
                    sidecar.unlink()
        return removed

    # ── Notifications ─────────────────────────────────────────────────

    def _sync_create_notification(
        self,
        tenant_id: str,
        user_id: str,
        type_: str,
        node_id: str,
        snippet: str | None,
    ) -> str:
        notif_id = str(uuid.uuid4())
        now = int(time.time() * 1000)
        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT INTO notifications (id, user_id, type, node_id, snippet, read, created_at)
                VALUES (?, ?, ?, ?, ?, 0, ?)
                """,
                (notif_id, user_id, type_, node_id, snippet, now),
            )
        return notif_id

    async def create_notification(
        self,
        tenant_id: str,
        user_id: str,
        type: str,
        node_id: str,
        snippet: str | None = None,
    ) -> str:
        """Create a notification for a user.

        Args:
            tenant_id: Tenant identifier
            user_id: User to notify
            type: Notification type (e.g. 'mention', 'reply')
            node_id: Related node ID
            snippet: Optional preview text

        Returns:
            Notification ID
        """
        return await self._run_sync(
            self._sync_create_notification, tenant_id, user_id, type, node_id, snippet
        )

    def _sync_get_notifications(
        self,
        tenant_id: str,
        user_id: str,
        unread_only: bool,
        limit: int,
        offset: int,
    ) -> list[dict]:
        with self._get_connection(tenant_id) as conn:
            if unread_only:
                cursor = conn.execute(
                    """
                    SELECT id, user_id, type, node_id, snippet, read, created_at
                    FROM notifications
                    WHERE user_id = ? AND read = 0
                    ORDER BY created_at DESC
                    LIMIT ? OFFSET ?
                    """,
                    (user_id, limit, offset),
                )
            else:
                cursor = conn.execute(
                    """
                    SELECT id, user_id, type, node_id, snippet, read, created_at
                    FROM notifications
                    WHERE user_id = ?
                    ORDER BY created_at DESC
                    LIMIT ? OFFSET ?
                    """,
                    (user_id, limit, offset),
                )
            return [dict(row) for row in cursor.fetchall()]

    async def get_notifications(
        self,
        tenant_id: str,
        user_id: str,
        unread_only: bool = False,
        limit: int = 50,
        offset: int = 0,
    ) -> list[dict]:
        """Get notifications for a user.

        Args:
            tenant_id: Tenant identifier
            user_id: User to get notifications for
            unread_only: If True, only return unread notifications
            limit: Maximum number of results
            offset: Pagination offset

        Returns:
            List of notification dicts
        """
        return await self._run_sync(
            self._sync_get_notifications, tenant_id, user_id, unread_only, limit, offset
        )

    def _sync_mark_notification_read(
        self,
        tenant_id: str,
        notification_id: str,
    ) -> bool:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "UPDATE notifications SET read = 1 WHERE id = ?",
                (notification_id,),
            )
            return cursor.rowcount > 0

    async def mark_notification_read(
        self,
        tenant_id: str,
        notification_id: str,
    ) -> bool:
        """Mark a single notification as read.

        Args:
            tenant_id: Tenant identifier
            notification_id: Notification to mark read

        Returns:
            True if notification was found and updated
        """
        return await self._run_sync(
            self._sync_mark_notification_read, tenant_id, notification_id
        )

    def _sync_mark_all_read(
        self,
        tenant_id: str,
        user_id: str,
    ) -> int:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "UPDATE notifications SET read = 1 WHERE user_id = ? AND read = 0",
                (user_id,),
            )
            return cursor.rowcount

    async def mark_all_read(
        self,
        tenant_id: str,
        user_id: str,
    ) -> int:
        """Mark all notifications as read for a user.

        Args:
            tenant_id: Tenant identifier
            user_id: User whose notifications to mark read

        Returns:
            Number of notifications marked read
        """
        return await self._run_sync(self._sync_mark_all_read, tenant_id, user_id)

    def _sync_get_unread_count(
        self,
        tenant_id: str,
        user_id: str,
    ) -> int:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read = 0",
                (user_id,),
            )
            return cursor.fetchone()[0]

    async def get_unread_count(
        self,
        tenant_id: str,
        user_id: str,
    ) -> int:
        """Get unread notification count for a user.

        Args:
            tenant_id: Tenant identifier
            user_id: User to count unread for

        Returns:
            Number of unread notifications
        """
        return await self._run_sync(self._sync_get_unread_count, tenant_id, user_id)

    def _sync_batch_create_notifications(
        self,
        tenant_id: str,
        entries: list[dict],
    ) -> int:
        now = int(time.time() * 1000)
        rows = []
        for entry in entries:
            rows.append((
                str(uuid.uuid4()),
                entry["user_id"],
                entry["type"],
                entry["node_id"],
                entry.get("snippet"),
                0,
                now,
            ))
        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                conn.executemany(
                    """
                    INSERT INTO notifications (id, user_id, type, node_id, snippet, read, created_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                    """,
                    rows,
                )
                conn.execute("COMMIT")
            except Exception:
                conn.execute("ROLLBACK")
                raise
        return len(rows)

    async def batch_create_notifications(
        self,
        tenant_id: str,
        entries: list[dict],
    ) -> int:
        """Batch create notifications (e.g. for @everyone).

        Uses a single transaction with executemany for efficiency.
        Should handle 1000 inserts in ~5-10ms.

        Args:
            tenant_id: Tenant identifier
            entries: List of dicts with keys: user_id, type, node_id, snippet (optional)

        Returns:
            Number of notifications created
        """
        return await self._run_sync(
            self._sync_batch_create_notifications, tenant_id, entries
        )

    # ── Read Cursors ──────────────────────────────────────────────────

    def _sync_update_read_cursor(
        self,
        tenant_id: str,
        user_id: str,
        channel_id: str,
    ) -> None:
        now = int(time.time() * 1000)
        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT INTO read_cursors (user_id, channel_id, last_read_at)
                VALUES (?, ?, ?)
                ON CONFLICT (user_id, channel_id)
                DO UPDATE SET last_read_at = excluded.last_read_at
                """,
                (user_id, channel_id, now),
            )

    async def update_read_cursor(
        self,
        tenant_id: str,
        user_id: str,
        channel_id: str,
    ) -> None:
        """Update the read cursor for a user in a channel.

        Sets last_read_at to current time. Uses UPSERT for idempotency.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            channel_id: Channel identifier
        """
        await self._run_sync(
            self._sync_update_read_cursor, tenant_id, user_id, channel_id
        )

    def _sync_get_read_cursor(
        self,
        tenant_id: str,
        user_id: str,
        channel_id: str,
    ) -> int | None:
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "SELECT last_read_at FROM read_cursors WHERE user_id = ? AND channel_id = ?",
                (user_id, channel_id),
            )
            row = cursor.fetchone()
            return row[0] if row else None

    async def get_read_cursor(
        self,
        tenant_id: str,
        user_id: str,
        channel_id: str,
    ) -> int | None:
        """Get the read cursor for a user in a channel.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            channel_id: Channel identifier

        Returns:
            Last read timestamp (Unix ms) or None if no cursor exists
        """
        return await self._run_sync(
            self._sync_get_read_cursor, tenant_id, user_id, channel_id
        )

    def _sync_get_unread_channels(
        self,
        tenant_id: str,
        user_id: str,
    ) -> list[dict]:
        with self._get_connection(tenant_id) as conn:
            # Find channels where notifications exist after the user's read cursor.
            # Notifications act as the source of channel activity; the read_cursor
            # records when the user last viewed a channel.
            cursor = conn.execute(
                """
                SELECT n.node_id AS channel_id,
                       COUNT(*)  AS unread_count
                FROM notifications n
                LEFT JOIN read_cursors rc
                    ON rc.user_id = ? AND rc.channel_id = n.node_id
                WHERE n.user_id = ?
                  AND n.read = 0
                  AND (rc.last_read_at IS NULL OR n.created_at > rc.last_read_at)
                GROUP BY n.node_id
                """,
                (user_id, user_id),
            )
            return [dict(row) for row in cursor.fetchall()]

    async def get_unread_channels(
        self,
        tenant_id: str,
        user_id: str,
    ) -> list[dict]:
        """Get channels with unread notification counts for a user.

        Returns channels where the user has unread notifications that were
        created after their last read cursor for that channel.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier

        Returns:
            List of dicts with 'channel_id' and 'unread_count'
        """
        return await self._run_sync(
            self._sync_get_unread_channels, tenant_id, user_id
        )

    # ── GDPR: anonymization and export ──────────────────────────────────

    @staticmethod
    def anon_id(user_id: str, salt: str = "entdb-gdpr-v1") -> str:
        """Derive a deterministic anonymous identifier for a user.

        The same (user_id, salt) pair always produces the same anonymous
        ID so that anonymized conversations and relationships still make
        sense after deletion. The salt is a per-process constant.

        Args:
            user_id: Original user identifier (e.g. ``user:alice``)
            salt: Deterministic salt applied before hashing

        Returns:
            String of the form ``user:anon-<12 hex chars>``
        """
        digest = hashlib.sha256((salt + user_id).encode()).hexdigest()[:12]
        return f"user:anon-{digest}"

    def _sync_anonymize_user_in_tenant(
        self,
        tenant_id: str,
        user_id: str,
        registry: object,
        salt: str,
    ) -> dict:
        """Apply per-type data_policy rules for GDPR user deletion.

        Runs in a single transaction. See
        :meth:`anonymize_user_in_tenant` for the behavioural contract.
        """
        from ..data_policy import DataPolicy
        from ..schema.registry import SchemaRegistry

        reg: SchemaRegistry = registry  # type: ignore[assignment]
        anon = self.anon_id(user_id, salt)
        deleted_nodes = 0
        anonymized_nodes = 0
        deleted_edges = 0
        anonymized_edges = 0

        def _scrub_payload(payload: dict, pii_fields: list[str], subject_field: str | None) -> dict:
            """Return a new payload with pii=true fields scrubbed and
            subject_field (if any) remapped to the anonymous id when it
            matches the user being deleted."""
            out = dict(payload)
            for fname in pii_fields:
                if fname in out:
                    value = out[fname]
                    if isinstance(value, str):
                        out[fname] = ""
                    elif isinstance(value, (int, float)):
                        out[fname] = 0
                    elif isinstance(value, list):
                        out[fname] = []
                    elif isinstance(value, dict):
                        out[fname] = {}
                    else:
                        out[fname] = None
            if subject_field and out.get(subject_field) == user_id:
                out[subject_field] = anon
            return out

        with self._get_connection(tenant_id) as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                # Collect all candidate nodes (owner_actor = user OR subject_field matches)
                # We scan the full table once and filter in Python so that subject_field
                # matches work for JSON-stored payloads. For larger tenants a per-type
                # path would be faster, but this is straightforward and consistent.
                rows = conn.execute(
                    "SELECT node_id, type_id, payload_json, owner_actor "
                    "FROM nodes WHERE tenant_id = ?",
                    (tenant_id,),
                ).fetchall()

                for row in rows:
                    type_id = row["type_id"]
                    node_id = row["node_id"]
                    owner = row["owner_actor"]
                    try:
                        policy = reg.get_data_policy(type_id)
                        pii_fields = reg.get_pii_fields(type_id)
                        subject_field = reg.get_subject_field(type_id)
                    except KeyError:
                        # Unknown type: treat as PERSONAL (strictest)
                        policy = DataPolicy.PERSONAL
                        pii_fields = []
                        subject_field = None

                    try:
                        payload = json.loads(row["payload_json"]) if row["payload_json"] else {}
                    except json.JSONDecodeError:
                        payload = {}

                    is_owner = owner == user_id
                    is_subject = (
                        subject_field is not None
                        and payload.get(subject_field) == user_id
                    )
                    if not (is_owner or is_subject):
                        continue

                    if policy in (DataPolicy.PERSONAL, DataPolicy.EPHEMERAL):
                        conn.execute(
                            "DELETE FROM edges WHERE tenant_id = ? "
                            "AND (from_node_id = ? OR to_node_id = ?)",
                            (tenant_id, node_id, node_id),
                        )
                        conn.execute(
                            "DELETE FROM node_visibility "
                            "WHERE tenant_id = ? AND node_id = ?",
                            (tenant_id, node_id),
                        )
                        conn.execute(
                            "DELETE FROM node_access WHERE node_id = ?",
                            (node_id,),
                        )
                        conn.execute(
                            "DELETE FROM nodes WHERE tenant_id = ? AND node_id = ?",
                            (tenant_id, node_id),
                        )
                        deleted_nodes += 1
                    else:
                        # BUSINESS, FINANCIAL, AUDIT, HEALTHCARE → anonymize
                        new_payload = _scrub_payload(payload, pii_fields, subject_field)
                        new_owner = anon if is_owner else owner
                        conn.execute(
                            "UPDATE nodes SET payload_json = ?, owner_actor = ?, updated_at = ? "
                            "WHERE tenant_id = ? AND node_id = ?",
                            (
                                json.dumps(new_payload),
                                new_owner,
                                int(time.time() * 1000),
                                tenant_id,
                                node_id,
                            ),
                        )
                        if is_owner:
                            # Owner visibility row: rewrite from user_id to anon
                            conn.execute(
                                "UPDATE node_visibility SET principal = ? "
                                "WHERE tenant_id = ? AND node_id = ? AND principal = ?",
                                (anon, tenant_id, node_id, user_id),
                            )
                        anonymized_nodes += 1

                # Edges — consult registry on_subject_exit
                edge_rows = conn.execute(
                    "SELECT edge_type_id, from_node_id, to_node_id, props_json "
                    "FROM edges WHERE tenant_id = ? "
                    "AND (from_node_id = ? OR to_node_id = ?)",
                    (tenant_id, user_id, user_id),
                ).fetchall()

                for er in edge_rows:
                    edge_type_id = er["edge_type_id"]
                    from_id = er["from_node_id"]
                    to_id = er["to_node_id"]
                    try:
                        direction = reg.get_edge_on_subject_exit(edge_type_id)
                        edge_policy = reg.get_edge_data_policy(edge_type_id)
                    except KeyError:
                        direction = "both"
                        edge_policy = DataPolicy.PERSONAL

                    hit_from = from_id == user_id
                    hit_to = to_id == user_id
                    if direction == "from" and not hit_from:
                        continue
                    if direction == "to" and not hit_to:
                        continue

                    if edge_policy in (DataPolicy.PERSONAL, DataPolicy.EPHEMERAL):
                        conn.execute(
                            "DELETE FROM edges WHERE tenant_id = ? "
                            "AND edge_type_id = ? AND from_node_id = ? AND to_node_id = ?",
                            (tenant_id, edge_type_id, from_id, to_id),
                        )
                        deleted_edges += 1
                    else:
                        # Anonymize edge: scrub pii props + replace user endpoints with anon
                        try:
                            edge_pii = []
                            edge_type = reg.get_edge_type(edge_type_id)
                            if edge_type is not None:
                                edge_pii = [
                                    p.name for p in edge_type.props
                                    if getattr(p, "pii", False) and not p.deprecated
                                ]
                        except Exception:
                            edge_pii = []
                        try:
                            props = json.loads(er["props_json"]) if er["props_json"] else {}
                        except json.JSONDecodeError:
                            props = {}
                        for fname in edge_pii:
                            if fname in props:
                                props[fname] = "" if isinstance(props[fname], str) else None
                        new_from = anon if hit_from else from_id
                        new_to = anon if hit_to else to_id
                        # Delete old row then insert new (primary key includes endpoints)
                        conn.execute(
                            "DELETE FROM edges WHERE tenant_id = ? "
                            "AND edge_type_id = ? AND from_node_id = ? AND to_node_id = ?",
                            (tenant_id, edge_type_id, from_id, to_id),
                        )
                        conn.execute(
                            "INSERT OR REPLACE INTO edges "
                            "(tenant_id, edge_type_id, from_node_id, to_node_id, props_json, "
                            " propagates_acl, created_at) "
                            "VALUES (?, ?, ?, ?, ?, 0, ?)",
                            (
                                tenant_id,
                                edge_type_id,
                                new_from,
                                new_to,
                                json.dumps(props),
                                int(time.time() * 1000),
                            ),
                        )
                        anonymized_edges += 1

                # Global cleanup within this tenant: remove user from
                # groups, direct grants, and visibility index.
                conn.execute(
                    "DELETE FROM node_access WHERE actor_id = ?",
                    (user_id,),
                )
                conn.execute(
                    "DELETE FROM group_users WHERE member_actor_id = ?",
                    (user_id,),
                )
                conn.execute(
                    "DELETE FROM node_visibility "
                    "WHERE tenant_id = ? AND principal = ?",
                    (tenant_id, user_id),
                )

                conn.execute("COMMIT")
            except Exception:
                conn.execute("ROLLBACK")
                raise

        return {
            "deleted_nodes": deleted_nodes,
            "anonymized_nodes": anonymized_nodes,
            "deleted_edges": deleted_edges,
            "anonymized_edges": anonymized_edges,
            "anon_id": anon,
        }

    async def anonymize_user_in_tenant(
        self,
        tenant_id: str,
        user_id: str,
        registry: object,
        salt: str = "entdb-gdpr-v1",
    ) -> dict:
        """Apply per-type data_policy rules for GDPR user deletion.

        For each node in the tenant where owner_actor == user_id or
        subject_field matches user_id:

            - PERSONAL    → delete the node
            - EPHEMERAL   → delete the node
            - BUSINESS    → anonymize: scrub pii=true fields, replace owner
            - FINANCIAL   → anonymize PII but retain the record
            - AUDIT       → anonymize PII but retain the record
            - HEALTHCARE  → anonymize PII but retain the record

        For each edge where the user is ``from`` or ``to`` endpoint,
        the edge's ``on_subject_exit`` policy decides which direction is
        affected (``from``, ``to``, or ``both``). Deletable policies
        delete the edge; retained policies rewrite the endpoint to the
        anonymous id and scrub pii=true props.

        Args:
            tenant_id: Tenant whose canonical store to anonymize
            user_id: User to delete (format: ``user:<id>``)
            registry: :class:`SchemaRegistry` with node/edge types
            salt: Deterministic salt for the anonymous id derivation

        Returns:
            Dict with counts: ``deleted_nodes``, ``anonymized_nodes``,
            ``deleted_edges``, ``anonymized_edges``, ``anon_id``.
        """
        return await self._run_sync(
            self._sync_anonymize_user_in_tenant,
            tenant_id,
            user_id,
            registry,
            salt,
        )

    def _sync_export_user_data(
        self,
        tenant_id: str,
        user_id: str,
        registry: object,
    ) -> dict:
        """Collect exportable nodes for a user in a single tenant.

        Returns a dict containing a list of node records that belong to
        the user either by ownership or by ``subject_field`` match. Nodes
        whose type has ``data_policy = AUDIT`` are excluded from export
        (audit records are not subject access requestable).
        """
        from ..data_policy import DataPolicy
        from ..schema.registry import SchemaRegistry

        reg: SchemaRegistry = registry  # type: ignore[assignment]
        nodes_out: list[dict] = []

        with self._get_connection(tenant_id) as conn:
            rows = conn.execute(
                "SELECT node_id, type_id, payload_json, created_at, updated_at, owner_actor "
                "FROM nodes WHERE tenant_id = ?",
                (tenant_id,),
            ).fetchall()

            for row in rows:
                type_id = row["type_id"]
                owner = row["owner_actor"]
                try:
                    policy = reg.get_data_policy(type_id)
                    subject_field = reg.get_subject_field(type_id)
                except KeyError:
                    policy = DataPolicy.PERSONAL
                    subject_field = None

                try:
                    payload = json.loads(row["payload_json"]) if row["payload_json"] else {}
                except json.JSONDecodeError:
                    payload = {}

                is_owner = owner == user_id
                is_subject = (
                    subject_field is not None and payload.get(subject_field) == user_id
                )
                if not (is_owner or is_subject):
                    continue
                # AUDIT policy → not exportable
                if policy == DataPolicy.AUDIT:
                    continue

                nodes_out.append(
                    {
                        "tenant_id": tenant_id,
                        "node_id": row["node_id"],
                        "type_id": type_id,
                        "payload": payload,
                        "created_at": row["created_at"],
                        "updated_at": row["updated_at"],
                        "owner_actor": owner,
                        "data_policy": policy.value,
                    }
                )

        return {"tenant_id": tenant_id, "nodes": nodes_out}

    async def export_user_data_for_tenant(
        self,
        tenant_id: str,
        user_id: str,
        registry: object,
    ) -> dict:
        """Export a user's content from a single tenant.

        Returns only nodes the user owns or is the subject of, excluding
        AUDIT records (per the export policy table in ADR-004).
        """
        return await self._run_sync(
            self._sync_export_user_data, tenant_id, user_id, registry
        )
