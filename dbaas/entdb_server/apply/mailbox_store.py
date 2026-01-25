"""
Per-user mailbox SQLite store with FTS5 for EntDB.

This module manages per-user mailbox databases that store:
- References to canonical content (not duplicated)
- Per-user state (read/unread, flags, etc.)
- Indexed snippets for full-text search

The mailbox pattern allows:
- Fast per-user queries without scanning tenant data
- User-specific state without modifying canonical data
- Local FTS search over relevant content

Invariants:
    - Mailbox stores references, not full content
    - FTS index contains searchable snippets only
    - One database per (tenant_id, user_id)
    - State updates don't affect canonical store

How to change safely:
    - Add new columns with defaults for backward compatibility
    - Never delete data that might be referenced
    - Test FTS index rebuild performance before deployment

Table schema:
    mailbox_items:
        - item_id TEXT PRIMARY KEY
        - ref_id TEXT (reference to canonical node)
        - source_type_id INTEGER
        - source_node_id TEXT
        - thread_id TEXT
        - ts INTEGER (timestamp)
        - state_json TEXT (user-specific state)
        - snippet TEXT (searchable text)
        - metadata_json TEXT

    fts_mailbox:
        - FTS5 virtual table over snippet
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


@dataclass
class MailboxItem:
    """An item in a user's mailbox.

    Attributes:
        item_id: Unique mailbox item ID
        ref_id: Reference to source (can be node_id or external ref)
        source_type_id: Type of source node
        source_node_id: Source node ID in canonical store
        thread_id: Thread/conversation ID for grouping
        ts: Timestamp (Unix ms)
        state: User-specific state (read, flags, etc.)
        snippet: Searchable text snippet
        metadata: Additional metadata
    """

    item_id: str
    ref_id: str
    source_type_id: int
    source_node_id: str
    thread_id: str | None
    ts: int
    state: dict[str, Any]
    snippet: str
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class SearchResult:
    """Search result from mailbox FTS.

    Attributes:
        item: Matching mailbox item
        rank: Search relevance score
        highlights: Highlighted snippet matches
    """

    item: MailboxItem
    rank: float
    highlights: str | None = None


class MailboxStore:
    """Per-user mailbox store with full-text search.

    This class manages per-user mailbox databases providing:
    - Mailbox item CRUD operations
    - Full-text search via SQLite FTS5
    - User-specific state management
    - Thread grouping

    Thread safety:
        Each database connection is created per-operation.
        SQLite handles concurrent access via WAL mode.

    Example:
        >>> store = MailboxStore("/var/lib/entdb/mailboxes")
        >>> await store.add_item(
        ...     tenant_id="tenant_123",
        ...     user_id="user:42",
        ...     source_type_id=101,
        ...     source_node_id="node_abc",
        ...     snippet="Important task needs attention",
        ... )
        >>> results = await store.search(
        ...     tenant_id="tenant_123",
        ...     user_id="user:42",
        ...     query="important task",
        ... )
    """

    def __init__(
        self,
        data_dir: str,
        wal_mode: bool = True,
        busy_timeout_ms: int = 5000,
    ) -> None:
        """Initialize the mailbox store.

        Args:
            data_dir: Directory for mailbox database files
            wal_mode: Enable SQLite WAL mode
            busy_timeout_ms: SQLite busy timeout
        """
        self.data_dir = Path(data_dir)
        self.wal_mode = wal_mode
        self.busy_timeout_ms = busy_timeout_ms
        self._lock = asyncio.Lock()

    def _get_db_path(self, tenant_id: str, user_id: str) -> Path:
        """Get database file path for a user's mailbox."""
        # Sanitize IDs to prevent path traversal
        safe_tenant = "".join(c for c in tenant_id if c.isalnum() or c in "-_")
        safe_user = "".join(c for c in user_id if c.isalnum() or c in "-_:")
        safe_user = safe_user.replace(":", "_")
        return self.data_dir / f"mailbox_{safe_tenant}_{safe_user}.db"

    @contextmanager
    def _get_connection(
        self,
        tenant_id: str,
        user_id: str,
        create: bool = True,
    ) -> Iterator[sqlite3.Connection]:
        """Get a database connection for a user's mailbox.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            create: Whether to create database if not exists

        Yields:
            SQLite connection
        """
        db_path = self._get_db_path(tenant_id, user_id)

        if not create and not db_path.exists():
            raise FileNotFoundError(f"Mailbox not found: {tenant_id}/{user_id}")

        # Ensure directory exists
        db_path.parent.mkdir(parents=True, exist_ok=True)

        conn = sqlite3.connect(
            str(db_path),
            timeout=self.busy_timeout_ms / 1000.0,
            isolation_level=None,
        )
        conn.row_factory = sqlite3.Row

        try:
            # Configure connection
            conn.execute(f"PRAGMA busy_timeout = {self.busy_timeout_ms}")
            if self.wal_mode:
                conn.execute("PRAGMA journal_mode = WAL")
            conn.execute("PRAGMA synchronous = NORMAL")

            # Ensure schema exists
            self._ensure_schema(conn)

            yield conn
        finally:
            conn.close()

    def _ensure_schema(self, conn: sqlite3.Connection) -> None:
        """Ensure database schema exists."""
        conn.executescript("""
            -- Mailbox items table
            CREATE TABLE IF NOT EXISTS mailbox_items (
                item_id TEXT PRIMARY KEY,
                ref_id TEXT NOT NULL,
                source_type_id INTEGER NOT NULL,
                source_node_id TEXT NOT NULL,
                thread_id TEXT,
                ts INTEGER NOT NULL,
                state_json TEXT NOT NULL DEFAULT '{}',
                snippet TEXT NOT NULL DEFAULT '',
                metadata_json TEXT NOT NULL DEFAULT '{}'
            );

            CREATE INDEX IF NOT EXISTS idx_mailbox_ts ON mailbox_items(ts DESC);
            CREATE INDEX IF NOT EXISTS idx_mailbox_thread ON mailbox_items(thread_id);
            CREATE INDEX IF NOT EXISTS idx_mailbox_source ON mailbox_items(source_node_id);
            CREATE INDEX IF NOT EXISTS idx_mailbox_type ON mailbox_items(source_type_id);

            -- FTS5 virtual table for search
            CREATE VIRTUAL TABLE IF NOT EXISTS fts_mailbox USING fts5(
                snippet,
                content='mailbox_items',
                content_rowid='rowid'
            );

            -- Triggers to keep FTS in sync
            CREATE TRIGGER IF NOT EXISTS mailbox_ai AFTER INSERT ON mailbox_items BEGIN
                INSERT INTO fts_mailbox(rowid, snippet) VALUES (new.rowid, new.snippet);
            END;

            CREATE TRIGGER IF NOT EXISTS mailbox_ad AFTER DELETE ON mailbox_items BEGIN
                INSERT INTO fts_mailbox(fts_mailbox, rowid, snippet)
                VALUES('delete', old.rowid, old.snippet);
            END;

            CREATE TRIGGER IF NOT EXISTS mailbox_au AFTER UPDATE ON mailbox_items BEGIN
                INSERT INTO fts_mailbox(fts_mailbox, rowid, snippet)
                VALUES('delete', old.rowid, old.snippet);
                INSERT INTO fts_mailbox(rowid, snippet) VALUES (new.rowid, new.snippet);
            END;
        """)

    async def add_item(
        self,
        tenant_id: str,
        user_id: str,
        source_type_id: int,
        source_node_id: str,
        snippet: str,
        ref_id: str | None = None,
        thread_id: str | None = None,
        state: dict[str, Any] | None = None,
        metadata: dict[str, Any] | None = None,
        ts: int | None = None,
        item_id: str | None = None,
    ) -> MailboxItem:
        """Add an item to a user's mailbox.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            source_type_id: Type of source node
            source_node_id: Source node ID
            snippet: Searchable text snippet
            ref_id: Reference ID (defaults to source_node_id)
            thread_id: Thread ID for grouping
            state: Initial state (default: unread)
            metadata: Additional metadata
            ts: Timestamp (default: now)
            item_id: Item ID (default: generated)

        Returns:
            Created MailboxItem
        """
        item_id = item_id or str(uuid.uuid4())
        ref_id = ref_id or source_node_id
        ts = ts or int(time.time() * 1000)
        state = state or {"read": False}
        metadata = metadata or {}

        with self._get_connection(tenant_id, user_id) as conn:
            conn.execute(
                """
                INSERT INTO mailbox_items
                (item_id, ref_id, source_type_id, source_node_id, thread_id,
                 ts, state_json, snippet, metadata_json)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    item_id,
                    ref_id,
                    source_type_id,
                    source_node_id,
                    thread_id,
                    ts,
                    json.dumps(state),
                    snippet,
                    json.dumps(metadata),
                ),
            )

        logger.debug(
            "Added mailbox item",
            extra={
                "tenant_id": tenant_id,
                "user_id": user_id,
                "item_id": item_id,
                "source_node_id": source_node_id,
            },
        )

        return MailboxItem(
            item_id=item_id,
            ref_id=ref_id,
            source_type_id=source_type_id,
            source_node_id=source_node_id,
            thread_id=thread_id,
            ts=ts,
            state=state,
            snippet=snippet,
            metadata=metadata,
        )

    async def get_item(
        self,
        tenant_id: str,
        user_id: str,
        item_id: str,
    ) -> MailboxItem | None:
        """Get a mailbox item by ID.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            item_id: Item identifier

        Returns:
            MailboxItem or None if not found
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                cursor = conn.execute(
                    "SELECT * FROM mailbox_items WHERE item_id = ?",
                    (item_id,),
                )
                row = cursor.fetchone()
                if not row:
                    return None

                return self._row_to_item(row)
        except FileNotFoundError:
            return None

    async def update_state(
        self,
        tenant_id: str,
        user_id: str,
        item_id: str,
        state_patch: dict[str, Any],
    ) -> MailboxItem | None:
        """Update an item's state.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            item_id: Item identifier
            state_patch: State fields to update

        Returns:
            Updated MailboxItem or None if not found
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                conn.execute("BEGIN IMMEDIATE")
                try:
                    cursor = conn.execute(
                        "SELECT * FROM mailbox_items WHERE item_id = ?",
                        (item_id,),
                    )
                    row = cursor.fetchone()
                    if not row:
                        conn.execute("ROLLBACK")
                        return None

                    # Merge state
                    existing_state = json.loads(row["state_json"])
                    existing_state.update(state_patch)

                    conn.execute(
                        "UPDATE mailbox_items SET state_json = ? WHERE item_id = ?",
                        (json.dumps(existing_state), item_id),
                    )
                    conn.execute("COMMIT")

                    return MailboxItem(
                        item_id=row["item_id"],
                        ref_id=row["ref_id"],
                        source_type_id=row["source_type_id"],
                        source_node_id=row["source_node_id"],
                        thread_id=row["thread_id"],
                        ts=row["ts"],
                        state=existing_state,
                        snippet=row["snippet"],
                        metadata=json.loads(row["metadata_json"]),
                    )

                except Exception:
                    conn.execute("ROLLBACK")
                    raise

        except FileNotFoundError:
            return None

    async def delete_item(
        self,
        tenant_id: str,
        user_id: str,
        item_id: str,
    ) -> bool:
        """Delete a mailbox item.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            item_id: Item identifier

        Returns:
            True if deleted, False if not found
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                cursor = conn.execute(
                    "DELETE FROM mailbox_items WHERE item_id = ?",
                    (item_id,),
                )
                return cursor.rowcount > 0
        except FileNotFoundError:
            return False

    async def list_items(
        self,
        tenant_id: str,
        user_id: str,
        limit: int = 50,
        offset: int = 0,
        thread_id: str | None = None,
        source_type_id: int | None = None,
        unread_only: bool = False,
    ) -> list[MailboxItem]:
        """List mailbox items with optional filters.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            limit: Maximum items to return
            offset: Pagination offset
            thread_id: Filter by thread
            source_type_id: Filter by source type
            unread_only: Only return unread items

        Returns:
            List of mailbox items
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                query = "SELECT * FROM mailbox_items WHERE 1=1"
                params: list[Any] = []

                if thread_id:
                    query += " AND thread_id = ?"
                    params.append(thread_id)

                if source_type_id is not None:
                    query += " AND source_type_id = ?"
                    params.append(source_type_id)

                if unread_only:
                    query += " AND json_extract(state_json, '$.read') = 0"

                query += " ORDER BY ts DESC LIMIT ? OFFSET ?"
                params.extend([limit, offset])

                cursor = conn.execute(query, params)
                return [self._row_to_item(row) for row in cursor.fetchall()]

        except FileNotFoundError:
            return []

    async def search(
        self,
        tenant_id: str,
        user_id: str,
        query: str,
        limit: int = 20,
        offset: int = 0,
        source_type_ids: list[int] | None = None,
    ) -> list[SearchResult]:
        """Full-text search in mailbox.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            query: Search query
            limit: Maximum results
            offset: Pagination offset
            source_type_ids: Optional filter by source types

        Returns:
            List of search results with relevance scores
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                # Build search query
                sql = """
                    SELECT m.*, fts.rank, highlight(fts_mailbox, 0, '<b>', '</b>') as highlights
                    FROM mailbox_items m
                    JOIN fts_mailbox fts ON m.rowid = fts.rowid
                    WHERE fts_mailbox MATCH ?
                """
                params: list[Any] = [query]

                if source_type_ids:
                    placeholders = ",".join("?" * len(source_type_ids))
                    sql += f" AND m.source_type_id IN ({placeholders})"
                    params.extend(source_type_ids)

                sql += " ORDER BY fts.rank LIMIT ? OFFSET ?"
                params.extend([limit, offset])

                cursor = conn.execute(sql, params)

                results = []
                for row in cursor.fetchall():
                    item = self._row_to_item(row)
                    results.append(
                        SearchResult(
                            item=item,
                            rank=row["rank"],
                            highlights=row["highlights"],
                        )
                    )

                return results

        except FileNotFoundError:
            return []
        except sqlite3.OperationalError as e:
            # Handle FTS query errors gracefully
            if "fts5" in str(e).lower():
                logger.warning(f"FTS query error: {e}")
                return []
            raise

    async def get_thread(
        self,
        tenant_id: str,
        user_id: str,
        thread_id: str,
    ) -> list[MailboxItem]:
        """Get all items in a thread.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            thread_id: Thread identifier

        Returns:
            List of items in the thread, ordered by timestamp
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                cursor = conn.execute(
                    """
                    SELECT * FROM mailbox_items
                    WHERE thread_id = ?
                    ORDER BY ts ASC
                    """,
                    (thread_id,),
                )
                return [self._row_to_item(row) for row in cursor.fetchall()]
        except FileNotFoundError:
            return []

    async def mark_read(
        self,
        tenant_id: str,
        user_id: str,
        item_ids: list[str],
    ) -> int:
        """Mark multiple items as read.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            item_ids: List of item IDs to mark read

        Returns:
            Number of items updated
        """
        if not item_ids:
            return 0

        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                placeholders = ",".join("?" * len(item_ids))
                cursor = conn.execute(
                    f"""
                    UPDATE mailbox_items
                    SET state_json = json_set(state_json, '$.read', 1)
                    WHERE item_id IN ({placeholders})
                    """,
                    item_ids,
                )
                return cursor.rowcount
        except FileNotFoundError:
            return 0

    async def get_unread_count(
        self,
        tenant_id: str,
        user_id: str,
    ) -> int:
        """Get count of unread items.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier

        Returns:
            Number of unread items
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                cursor = conn.execute(
                    """
                    SELECT COUNT(*) FROM mailbox_items
                    WHERE json_extract(state_json, '$.read') = 0
                    """
                )
                return cursor.fetchone()[0]
        except FileNotFoundError:
            return 0

    async def rebuild_fts_index(
        self,
        tenant_id: str,
        user_id: str,
    ) -> None:
        """Rebuild the FTS index.

        Use this if the FTS index becomes corrupted or out of sync.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
        """
        with self._get_connection(tenant_id, user_id) as conn:
            conn.execute("INSERT INTO fts_mailbox(fts_mailbox) VALUES('rebuild')")
            logger.info("Rebuilt FTS index", extra={"tenant_id": tenant_id, "user_id": user_id})

    async def delete_by_source(
        self,
        tenant_id: str,
        user_id: str,
        source_node_id: str,
    ) -> int:
        """Delete all items referencing a source node.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            source_node_id: Source node ID

        Returns:
            Number of items deleted
        """
        try:
            with self._get_connection(tenant_id, user_id, create=False) as conn:
                cursor = conn.execute(
                    "DELETE FROM mailbox_items WHERE source_node_id = ?",
                    (source_node_id,),
                )
                return cursor.rowcount
        except FileNotFoundError:
            return 0

    def _row_to_item(self, row: sqlite3.Row) -> MailboxItem:
        """Convert database row to MailboxItem."""
        return MailboxItem(
            item_id=row["item_id"],
            ref_id=row["ref_id"],
            source_type_id=row["source_type_id"],
            source_node_id=row["source_node_id"],
            thread_id=row["thread_id"],
            ts=row["ts"],
            state=json.loads(row["state_json"]),
            snippet=row["snippet"],
            metadata=json.loads(row["metadata_json"]),
        )

    def get_db_path(self, tenant_id: str, user_id: str) -> Path:
        """Get the database file path.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier

        Returns:
            Path to database file
        """
        return self._get_db_path(tenant_id, user_id)

    async def mailbox_exists(self, tenant_id: str, user_id: str) -> bool:
        """Check if a mailbox exists.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier

        Returns:
            True if mailbox database exists
        """
        return self._get_db_path(tenant_id, user_id).exists()
