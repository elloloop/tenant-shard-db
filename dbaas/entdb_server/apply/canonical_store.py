"""
Canonical tenant SQLite store for EntDB.

This module manages the per-tenant SQLite database that stores:
- Nodes with their payloads and ACLs
- Edges connecting nodes
- Applied events for idempotency checking
- Node visibility index for ACL filtering

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
import json
import logging
import sqlite3
import time
import uuid
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)


class TenantNotFoundError(Exception):
    """Tenant database does not exist."""

    pass


class IdempotencyViolationError(Exception):
    """Event has already been applied."""

    pass


@dataclass
class Node:
    """Represents a node in the graph.

    Attributes:
        tenant_id: Tenant identifier
        node_id: Unique node identifier (UUID)
        type_id: Node type identifier
        payload: Field values
        created_at: Creation timestamp (Unix ms)
        updated_at: Last update timestamp (Unix ms)
        owner_actor: Actor who created the node
        acl: Access control list
    """

    tenant_id: str
    node_id: str
    type_id: int
    payload: dict[str, Any]
    created_at: int
    updated_at: int
    owner_actor: str
    acl: list[dict[str, str]] = field(default_factory=list)


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
        Each database connection is created per-operation.
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
    ) -> None:
        """Initialize the canonical store.

        Args:
            data_dir: Directory for SQLite database files
            wal_mode: Enable SQLite WAL mode
            busy_timeout_ms: SQLite busy timeout
            cache_size_pages: SQLite cache size (negative = KB)
        """
        self.data_dir = Path(data_dir)
        self.wal_mode = wal_mode
        self.busy_timeout_ms = busy_timeout_ms
        self.cache_size_pages = cache_size_pages
        self._connections: dict[str, sqlite3.Connection] = {}
        self._lock = asyncio.Lock()

    def _get_db_path(self, tenant_id: str) -> Path:
        """Get database file path for a tenant."""
        # Sanitize tenant_id to prevent path traversal
        safe_id = "".join(c for c in tenant_id if c.isalnum() or c in "-_")
        return self.data_dir / f"tenant_{safe_id}.db"

    @contextmanager
    def _get_connection(self, tenant_id: str, create: bool = False) -> Iterator[sqlite3.Connection]:
        """Get a database connection for a tenant.

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

        conn = sqlite3.connect(
            str(db_path),
            timeout=self.busy_timeout_ms / 1000.0,
            isolation_level=None,  # Autocommit by default, explicit transactions
        )
        conn.row_factory = sqlite3.Row

        try:
            # Configure connection
            conn.execute(f"PRAGMA busy_timeout = {self.busy_timeout_ms}")
            conn.execute(f"PRAGMA cache_size = {self.cache_size_pages}")
            if self.wal_mode:
                conn.execute("PRAGMA journal_mode = WAL")
            conn.execute("PRAGMA synchronous = NORMAL")
            conn.execute("PRAGMA foreign_keys = ON")

            yield conn
        finally:
            conn.close()

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

            -- Record schema version
            INSERT OR IGNORE INTO schema_version (version, applied_at)
            VALUES (1, strftime('%s', 'now') * 1000);
        """)

    async def initialize_tenant(self, tenant_id: str) -> None:
        """Initialize database for a new tenant.

        Creates the database file and schema if they don't exist.

        Args:
            tenant_id: Tenant identifier
        """
        async with self._lock:
            with self._get_connection(tenant_id, create=True) as conn:
                self._create_schema(conn)
                logger.info(f"Initialized tenant database: {tenant_id}")

    async def tenant_exists(self, tenant_id: str) -> bool:
        """Check if tenant database exists."""
        return self._get_db_path(tenant_id).exists()

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
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "SELECT 1 FROM applied_events WHERE tenant_id = ? AND idempotency_key = ?",
                (tenant_id, idempotency_key),
            )
            return cursor.fetchone() is not None

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
        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT INTO applied_events (tenant_id, idempotency_key, stream_pos, applied_at)
                VALUES (?, ?, ?, ?)
                """,
                (tenant_id, idempotency_key, stream_pos, int(time.time() * 1000)),
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
                        json.dumps(payload),
                        now,
                        now,
                        owner_actor,
                        json.dumps(acl),
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
            payload=payload,
            created_at=now,
            updated_at=now,
            owner_actor=owner_actor,
            acl=acl,
        )

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

                # Update node
                conn.execute(
                    """
                    UPDATE nodes SET payload_json = ?, updated_at = ?
                    WHERE tenant_id = ? AND node_id = ?
                    """,
                    (json.dumps(existing_payload), now, tenant_id, node_id),
                )

                conn.execute("COMMIT")

                return Node(
                    tenant_id=tenant_id,
                    node_id=node_id,
                    type_id=row["type_id"],
                    payload=existing_payload,
                    created_at=row["created_at"],
                    updated_at=now,
                    owner_actor=row["owner_actor"],
                    acl=json.loads(row["acl_blob"]),
                )

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

    async def get_node(self, tenant_id: str, node_id: str) -> Node | None:
        """Get a node by ID.

        Args:
            tenant_id: Tenant identifier
            node_id: Node identifier

        Returns:
            Node or None if not found
        """
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                "SELECT * FROM nodes WHERE tenant_id = ? AND node_id = ?",
                (tenant_id, node_id),
            )
            row = cursor.fetchone()
            if not row:
                return None

            return Node(
                tenant_id=row["tenant_id"],
                node_id=row["node_id"],
                type_id=row["type_id"],
                payload=json.loads(row["payload_json"]),
                created_at=row["created_at"],
                updated_at=row["updated_at"],
                owner_actor=row["owner_actor"],
                acl=json.loads(row["acl_blob"]),
            )

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
                Node(
                    tenant_id=row["tenant_id"],
                    node_id=row["node_id"],
                    type_id=row["type_id"],
                    payload=json.loads(row["payload_json"]),
                    created_at=row["created_at"],
                    updated_at=row["updated_at"],
                    owner_actor=row["owner_actor"],
                    acl=json.loads(row["acl_blob"]),
                )
                for row in cursor.fetchall()
            ]

    async def create_edge(
        self,
        tenant_id: str,
        edge_type_id: int,
        from_node_id: str,
        to_node_id: str,
        props: dict[str, Any] | None = None,
        created_at: int | None = None,
    ) -> Edge:
        """Create an edge between nodes.

        Args:
            tenant_id: Tenant identifier
            edge_type_id: Edge type identifier
            from_node_id: Source node ID
            to_node_id: Target node ID
            props: Edge properties
            created_at: Optional creation timestamp

        Returns:
            Created Edge object
        """
        now = created_at or int(time.time() * 1000)
        props = props or {}

        with self._get_connection(tenant_id) as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO edges
                (tenant_id, edge_type_id, from_node_id, to_node_id, props_json, created_at)
                VALUES (?, ?, ?, ?, ?, ?)
                """,
                (tenant_id, edge_type_id, from_node_id, to_node_id, json.dumps(props), now),
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
        with self._get_connection(tenant_id) as conn:
            cursor = conn.execute(
                """
                DELETE FROM edges
                WHERE tenant_id = ? AND edge_type_id = ? AND from_node_id = ? AND to_node_id = ?
                """,
                (tenant_id, edge_type_id, from_node_id, to_node_id),
            )
            return cursor.rowcount > 0

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
                Node(
                    tenant_id=row["tenant_id"],
                    node_id=row["node_id"],
                    type_id=row["type_id"],
                    payload=json.loads(row["payload_json"]),
                    created_at=row["created_at"],
                    updated_at=row["updated_at"],
                    owner_actor=row["owner_actor"],
                    acl=json.loads(row["acl_blob"]),
                )
                for row in cursor.fetchall()
            ]

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

    async def get_stats(self, tenant_id: str) -> dict[str, int]:
        """Get statistics for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Dictionary with counts
        """
        with self._get_connection(tenant_id) as conn:
            stats = {}

            cursor = conn.execute("SELECT COUNT(*) FROM nodes WHERE tenant_id = ?", (tenant_id,))
            stats["nodes"] = cursor.fetchone()[0]

            cursor = conn.execute("SELECT COUNT(*) FROM edges WHERE tenant_id = ?", (tenant_id,))
            stats["edges"] = cursor.fetchone()[0]

            cursor = conn.execute(
                "SELECT COUNT(*) FROM applied_events WHERE tenant_id = ?", (tenant_id,)
            )
            stats["applied_events"] = cursor.fetchone()[0]

            return stats

    async def get_last_applied_position(self, tenant_id: str) -> str | None:
        """Get the last applied stream position.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Stream position string or None
        """
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

    def get_db_path(self, tenant_id: str) -> Path:
        """Get the database file path for a tenant.

        Args:
            tenant_id: Tenant identifier

        Returns:
            Path to database file
        """
        return self._get_db_path(tenant_id)
