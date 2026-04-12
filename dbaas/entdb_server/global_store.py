"""
Global SQLite store for cross-tenant state in EntDB.

This module manages a single SQLite database (not per-tenant) that stores:
- User registry (identity and status)
- Tenant registry (tenant metadata)
- Tenant membership (user-to-tenant mapping with roles)
- Shared index (cross-tenant content sharing)
- Deletion queue (GDPR right-to-erasure scheduling)

The global store is separate from the per-tenant canonical stores.
It provides the cross-cutting metadata needed to route requests,
enforce access control, and comply with data protection regulations.

Invariants:
    - Single SQLite file at {data_dir}/global.db
    - All operations are atomic (single transaction)
    - Email uniqueness enforced at the database level
    - Timestamps are Unix epoch seconds (integer)

How to change safely:
    - Schema migrations must be backward compatible
    - Add new columns with defaults for backward compatibility
    - Never delete columns that may still be referenced
    - Test with realistic data volumes before deployment

Table schema:
    user_registry:
        - user_id TEXT PRIMARY KEY
        - email TEXT UNIQUE NOT NULL
        - name TEXT NOT NULL
        - status TEXT NOT NULL DEFAULT 'active'
        - created_at INTEGER NOT NULL
        - updated_at INTEGER NOT NULL

    tenant_registry:
        - tenant_id TEXT PRIMARY KEY
        - name TEXT NOT NULL
        - status TEXT NOT NULL DEFAULT 'active'
        - created_at INTEGER NOT NULL

    tenant_members:
        - tenant_id TEXT NOT NULL
        - user_id TEXT NOT NULL
        - role TEXT NOT NULL DEFAULT 'member'
        - joined_at INTEGER NOT NULL
        - PRIMARY KEY (tenant_id, user_id)

    shared_index:
        - user_id TEXT NOT NULL
        - source_tenant TEXT NOT NULL
        - node_id TEXT NOT NULL
        - permission TEXT NOT NULL
        - shared_at INTEGER NOT NULL
        - PRIMARY KEY (user_id, source_tenant, node_id)

    deletion_queue:
        - user_id TEXT PRIMARY KEY
        - requested_at INTEGER NOT NULL
        - execute_at INTEGER NOT NULL
        - export_path TEXT
        - status TEXT NOT NULL DEFAULT 'pending'
"""

from __future__ import annotations

import asyncio
import logging
import sqlite3
import time
from collections.abc import Callable, Iterator
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
from pathlib import Path
from typing import Any

from .config import EncryptionConfig
from .encryption import derive_global_key, open_encrypted_connection

logger = logging.getLogger(__name__)


class GlobalStore:
    """Manages the global SQLite database for cross-tenant state.

    This class manages a single SQLite database that stores user registrations,
    tenant registrations, memberships, cross-tenant sharing metadata, and
    deletion scheduling for GDPR compliance.

    Thread safety:
        All database operations run on a single-thread executor to ensure
        SQLite connection safety. Async methods delegate to synchronous
        helpers via _run_sync.

    Example:
        >>> store = GlobalStore("/var/lib/entdb")
        >>> user = await store.create_user("u1", "alice@example.com", "Alice")
        >>> tenant = await store.create_tenant("t1", "Acme Corp")
        >>> await store.add_member("t1", "u1", role="admin")
    """

    def __init__(
        self,
        data_dir: str | Path,
        wal_mode: bool = True,
        busy_timeout_ms: int = 5000,
        encryption_config: EncryptionConfig | None = None,
    ) -> None:
        """Initialize the global store.

        Args:
            data_dir: Directory for the global.db file
            wal_mode: Enable SQLite WAL mode
            busy_timeout_ms: SQLite busy timeout in milliseconds
            encryption_config: Optional encryption configuration.
        """
        self.data_dir = Path(data_dir)
        self.wal_mode = wal_mode
        self.busy_timeout_ms = busy_timeout_ms
        self.encryption_config = encryption_config or EncryptionConfig()
        self._conn: sqlite3.Connection | None = None
        self._executor = ThreadPoolExecutor(
            max_workers=1, thread_name_prefix="entdb-global"
        )

        # Eagerly initialise the database and schema so that callers
        # do not need to await an explicit ``initialise()`` step.
        self._ensure_db()

    # ── connection management ─────────────────────────────────────────

    def _db_path(self) -> Path:
        return self.data_dir / "global.db"

    def _ensure_db(self) -> None:
        """Create the database file and schema if they do not exist."""
        self.data_dir.mkdir(parents=True, exist_ok=True)
        conn = self._open_connection()
        self._create_schema(conn)

    def _open_connection(self) -> sqlite3.Connection:
        """Open (or return cached) connection to global.db."""
        if self._conn is not None:
            return self._conn

        if self.encryption_config.enabled:
            global_key = derive_global_key(self.encryption_config.master_key)
            conn = open_encrypted_connection(
                str(self._db_path()),
                global_key,
                timeout=self.busy_timeout_ms / 1000.0,
            )
        else:
            conn = sqlite3.connect(
                str(self._db_path()),
                timeout=self.busy_timeout_ms / 1000.0,
                isolation_level=None,  # autocommit; we manage transactions manually
                check_same_thread=False,
            )
        conn.row_factory = sqlite3.Row

        conn.execute(f"PRAGMA busy_timeout = {self.busy_timeout_ms}")
        if self.wal_mode:
            conn.execute("PRAGMA journal_mode = WAL")
        conn.execute("PRAGMA synchronous = NORMAL")
        conn.execute("PRAGMA foreign_keys = ON")

        self._conn = conn
        return conn

    @contextmanager
    def _get_connection(self) -> Iterator[sqlite3.Connection]:
        """Yield the pooled connection to global.db."""
        conn = self._open_connection()
        yield conn

    def _create_schema(self, conn: sqlite3.Connection) -> None:
        """Create all global tables if they do not exist."""
        conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS user_registry (
                user_id     TEXT PRIMARY KEY,
                email       TEXT UNIQUE NOT NULL,
                name        TEXT NOT NULL,
                status      TEXT NOT NULL DEFAULT 'active',
                created_at  INTEGER NOT NULL,
                updated_at  INTEGER NOT NULL
            );

            CREATE TABLE IF NOT EXISTS tenant_registry (
                tenant_id   TEXT PRIMARY KEY,
                name        TEXT NOT NULL,
                status      TEXT NOT NULL DEFAULT 'active',
                created_at  INTEGER NOT NULL
            );

            CREATE TABLE IF NOT EXISTS tenant_members (
                tenant_id   TEXT NOT NULL,
                user_id     TEXT NOT NULL,
                role        TEXT NOT NULL DEFAULT 'member',
                joined_at   INTEGER NOT NULL,
                PRIMARY KEY (tenant_id, user_id)
            );
            CREATE INDEX IF NOT EXISTS idx_members_user
                ON tenant_members(user_id);

            CREATE TABLE IF NOT EXISTS shared_index (
                user_id       TEXT NOT NULL,
                source_tenant TEXT NOT NULL,
                node_id       TEXT NOT NULL,
                permission    TEXT NOT NULL,
                shared_at     INTEGER NOT NULL,
                PRIMARY KEY (user_id, source_tenant, node_id)
            );

            CREATE TABLE IF NOT EXISTS deletion_queue (
                user_id       TEXT PRIMARY KEY,
                requested_at  INTEGER NOT NULL,
                execute_at    INTEGER NOT NULL,
                export_path   TEXT,
                status        TEXT NOT NULL DEFAULT 'pending'
            );
            """
        )

    def close(self) -> None:
        """Close the database connection and shut down the executor."""
        if self._conn is not None:
            self._conn.close()
            self._conn = None
        self._executor.shutdown(wait=False)

    # ── async helper ──────────────────────────────────────────────────

    async def _run_sync(self, fn: Callable, *args: Any) -> Any:
        """Run a synchronous function in the dedicated thread executor."""
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            return fn(*args)
        return await loop.run_in_executor(self._executor, fn, *args)

    # ── helpers ───────────────────────────────────────────────────────

    @staticmethod
    def _now() -> int:
        """Current Unix epoch timestamp in seconds."""
        return int(time.time())

    @staticmethod
    def _row_to_dict(row: sqlite3.Row | None) -> dict | None:
        """Convert a sqlite3.Row to a plain dict (or None)."""
        if row is None:
            return None
        return dict(row)

    # ── User CRUD ─────────────────────────────────────────────────────

    async def create_user(self, user_id: str, email: str, name: str) -> dict:
        """Register a new user.

        Args:
            user_id: Unique user identifier
            email: User email (must be unique)
            name: Display name

        Returns:
            Dict with user fields

        Raises:
            sqlite3.IntegrityError: If user_id or email already exists
        """
        return await self._run_sync(self._sync_create_user, user_id, email, name)

    def _sync_create_user(self, user_id: str, email: str, name: str) -> dict:
        now = self._now()
        with self._get_connection() as conn:
            conn.execute(
                """
                INSERT INTO user_registry (user_id, email, name, status, created_at, updated_at)
                VALUES (?, ?, ?, 'active', ?, ?)
                """,
                (user_id, email, name, now, now),
            )
            return {
                "user_id": user_id,
                "email": email,
                "name": name,
                "status": "active",
                "created_at": now,
                "updated_at": now,
            }

    async def get_user(self, user_id: str) -> dict | None:
        """Fetch a user by ID.

        Returns:
            User dict or None if not found
        """
        return await self._run_sync(self._sync_get_user, user_id)

    def _sync_get_user(self, user_id: str) -> dict | None:
        with self._get_connection() as conn:
            row = conn.execute(
                "SELECT * FROM user_registry WHERE user_id = ?", (user_id,)
            ).fetchone()
            return self._row_to_dict(row)

    async def update_user(self, user_id: str, **kwargs: Any) -> bool:
        """Update user fields.

        Allowed keyword arguments: email, name, status.

        Returns:
            True if the user was found and updated, False otherwise
        """
        return await self._run_sync(self._sync_update_user, user_id, kwargs)

    def _sync_update_user(self, user_id: str, kwargs: dict[str, Any]) -> bool:
        allowed = {"email", "name", "status"}
        updates = {k: v for k, v in kwargs.items() if k in allowed}
        if not updates:
            return False
        updates["updated_at"] = self._now()
        set_clause = ", ".join(f"{col} = ?" for col in updates)
        values = list(updates.values()) + [user_id]
        with self._get_connection() as conn:
            cursor = conn.execute(
                f"UPDATE user_registry SET {set_clause} WHERE user_id = ?",
                values,
            )
            return cursor.rowcount > 0

    async def list_users(
        self, status: str = "active", limit: int = 100, offset: int = 0
    ) -> list[dict]:
        """List users filtered by status.

        Args:
            status: Filter by status (e.g. 'active', 'suspended')
            limit: Maximum number of results
            offset: Pagination offset

        Returns:
            List of user dicts
        """
        return await self._run_sync(self._sync_list_users, status, limit, offset)

    def _sync_list_users(self, status: str, limit: int, offset: int) -> list[dict]:
        with self._get_connection() as conn:
            rows = conn.execute(
                "SELECT * FROM user_registry WHERE status = ? ORDER BY created_at LIMIT ? OFFSET ?",
                (status, limit, offset),
            ).fetchall()
            return [dict(r) for r in rows]

    async def set_user_status(self, user_id: str, status: str) -> bool:
        """Change a user's status.

        Returns:
            True if the user was found and updated, False otherwise
        """
        return await self._run_sync(self._sync_set_user_status, user_id, status)

    def _sync_set_user_status(self, user_id: str, status: str) -> bool:
        now = self._now()
        with self._get_connection() as conn:
            cursor = conn.execute(
                "UPDATE user_registry SET status = ?, updated_at = ? WHERE user_id = ?",
                (status, now, user_id),
            )
            return cursor.rowcount > 0

    # ── Tenant CRUD ───────────────────────────────────────────────────

    async def create_tenant(self, tenant_id: str, name: str) -> dict:
        """Register a new tenant.

        Args:
            tenant_id: Unique tenant identifier
            name: Tenant display name

        Returns:
            Dict with tenant fields

        Raises:
            sqlite3.IntegrityError: If tenant_id already exists
        """
        return await self._run_sync(self._sync_create_tenant, tenant_id, name)

    def _sync_create_tenant(self, tenant_id: str, name: str) -> dict:
        now = self._now()
        with self._get_connection() as conn:
            conn.execute(
                """
                INSERT INTO tenant_registry (tenant_id, name, status, created_at)
                VALUES (?, ?, 'active', ?)
                """,
                (tenant_id, name, now),
            )
            return {
                "tenant_id": tenant_id,
                "name": name,
                "status": "active",
                "created_at": now,
            }

    async def get_tenant(self, tenant_id: str) -> dict | None:
        """Fetch a tenant by ID.

        Returns:
            Tenant dict or None if not found
        """
        return await self._run_sync(self._sync_get_tenant, tenant_id)

    def _sync_get_tenant(self, tenant_id: str) -> dict | None:
        with self._get_connection() as conn:
            row = conn.execute(
                "SELECT * FROM tenant_registry WHERE tenant_id = ?", (tenant_id,)
            ).fetchone()
            return self._row_to_dict(row)

    async def list_tenants(self, status: str = "active") -> list[dict]:
        """List tenants filtered by status.

        Returns:
            List of tenant dicts
        """
        return await self._run_sync(self._sync_list_tenants, status)

    def _sync_list_tenants(self, status: str) -> list[dict]:
        with self._get_connection() as conn:
            rows = conn.execute(
                "SELECT * FROM tenant_registry WHERE status = ? ORDER BY created_at",
                (status,),
            ).fetchall()
            return [dict(r) for r in rows]

    async def set_tenant_status(self, tenant_id: str, status: str) -> bool:
        """Change a tenant's status.

        Returns:
            True if the tenant was found and updated, False otherwise
        """
        return await self._run_sync(self._sync_set_tenant_status, tenant_id, status)

    async def set_legal_hold(
        self, tenant_id: str, enabled: bool, actor: str
    ) -> bool:
        """Toggle legal_hold status on a tenant.

        When ``enabled`` is True, the tenant is put on legal hold
        (status = ``legal_hold``); deletes are blocked at the gRPC
        layer by ``_check_tenant_access``. When ``enabled`` is False
        the tenant is returned to ``active``.

        Args:
            tenant_id: Tenant identifier
            enabled: True to enable hold, False to release it
            actor: Admin actor performing the operation (informational;
                the actual audit log write is done by the gRPC handler)

        Returns:
            True if the tenant existed and its status was updated.
        """
        new_status = "legal_hold" if enabled else "active"
        return await self._run_sync(
            self._sync_set_tenant_status, tenant_id, new_status
        )

    def _sync_set_tenant_status(self, tenant_id: str, status: str) -> bool:
        with self._get_connection() as conn:
            cursor = conn.execute(
                "UPDATE tenant_registry SET status = ? WHERE tenant_id = ?",
                (status, tenant_id),
            )
            return cursor.rowcount > 0

    # ── Membership ────────────────────────────────────────────────────

    async def add_member(
        self, tenant_id: str, user_id: str, role: str = "member"
    ) -> None:
        """Add a user as a member of a tenant.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier
            role: Membership role (default: 'member')

        Raises:
            sqlite3.IntegrityError: If the membership already exists
        """
        await self._run_sync(self._sync_add_member, tenant_id, user_id, role)

    def _sync_add_member(self, tenant_id: str, user_id: str, role: str) -> None:
        now = self._now()
        with self._get_connection() as conn:
            conn.execute(
                """
                INSERT INTO tenant_members (tenant_id, user_id, role, joined_at)
                VALUES (?, ?, ?, ?)
                """,
                (tenant_id, user_id, role, now),
            )

    async def remove_member(self, tenant_id: str, user_id: str) -> bool:
        """Remove a user from a tenant.

        Returns:
            True if the membership existed and was removed, False otherwise
        """
        return await self._run_sync(self._sync_remove_member, tenant_id, user_id)

    def _sync_remove_member(self, tenant_id: str, user_id: str) -> bool:
        with self._get_connection() as conn:
            cursor = conn.execute(
                "DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
                (tenant_id, user_id),
            )
            return cursor.rowcount > 0

    async def get_members(self, tenant_id: str) -> list[dict]:
        """List all members of a tenant.

        Returns:
            List of membership dicts (tenant_id, user_id, role, joined_at)
        """
        return await self._run_sync(self._sync_get_members, tenant_id)

    def _sync_get_members(self, tenant_id: str) -> list[dict]:
        with self._get_connection() as conn:
            rows = conn.execute(
                "SELECT * FROM tenant_members WHERE tenant_id = ? ORDER BY joined_at",
                (tenant_id,),
            ).fetchall()
            return [dict(r) for r in rows]

    async def get_user_tenants(self, user_id: str) -> list[dict]:
        """List all tenants a user belongs to.

        Returns:
            List of membership dicts
        """
        return await self._run_sync(self._sync_get_user_tenants, user_id)

    def _sync_get_user_tenants(self, user_id: str) -> list[dict]:
        with self._get_connection() as conn:
            rows = conn.execute(
                "SELECT * FROM tenant_members WHERE user_id = ? ORDER BY joined_at",
                (user_id,),
            ).fetchall()
            return [dict(r) for r in rows]

    async def change_role(self, tenant_id: str, user_id: str, role: str) -> bool:
        """Change a member's role within a tenant.

        Returns:
            True if the membership existed and was updated, False otherwise
        """
        return await self._run_sync(self._sync_change_role, tenant_id, user_id, role)

    def _sync_change_role(self, tenant_id: str, user_id: str, role: str) -> bool:
        with self._get_connection() as conn:
            cursor = conn.execute(
                "UPDATE tenant_members SET role = ? WHERE tenant_id = ? AND user_id = ?",
                (role, tenant_id, user_id),
            )
            return cursor.rowcount > 0

    # ── Membership helpers ────────────────────────────────────────────

    async def is_member(self, tenant_id: str, user_id: str) -> bool:
        """Check if a user is a member of a tenant.

        Args:
            tenant_id: Tenant identifier
            user_id: User identifier (without 'user:' prefix)

        Returns:
            True if the user is a member of the tenant
        """
        return await self._run_sync(self._sync_is_member, tenant_id, user_id)

    def _sync_is_member(self, tenant_id: str, user_id: str) -> bool:
        with self._get_connection() as conn:
            row = conn.execute(
                "SELECT 1 FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
                (tenant_id, user_id),
            ).fetchone()
            return row is not None

    # ── Shared index ──────────────────────────────────────────────────

    async def add_shared(
        self, user_id: str, source_tenant: str, node_id: str, permission: str
    ) -> None:
        """Record a shared node for a user.

        Args:
            user_id: The user receiving access
            source_tenant: Tenant that owns the node
            node_id: Shared node identifier
            permission: Permission level (e.g. 'read', 'write')
        """
        await self._run_sync(
            self._sync_add_shared, user_id, source_tenant, node_id, permission
        )

    def _sync_add_shared(
        self, user_id: str, source_tenant: str, node_id: str, permission: str
    ) -> None:
        now = self._now()
        with self._get_connection() as conn:
            conn.execute(
                """
                INSERT OR REPLACE INTO shared_index
                    (user_id, source_tenant, node_id, permission, shared_at)
                VALUES (?, ?, ?, ?, ?)
                """,
                (user_id, source_tenant, node_id, permission, now),
            )

    async def remove_shared(
        self, user_id: str, source_tenant: str, node_id: str
    ) -> bool:
        """Remove a specific shared entry.

        Returns:
            True if the entry existed and was removed, False otherwise
        """
        return await self._run_sync(
            self._sync_remove_shared, user_id, source_tenant, node_id
        )

    def _sync_remove_shared(
        self, user_id: str, source_tenant: str, node_id: str
    ) -> bool:
        with self._get_connection() as conn:
            cursor = conn.execute(
                "DELETE FROM shared_index WHERE user_id = ? AND source_tenant = ? AND node_id = ?",
                (user_id, source_tenant, node_id),
            )
            return cursor.rowcount > 0

    async def get_shared_with_me(
        self, user_id: str, limit: int = 100, offset: int = 0
    ) -> list[dict]:
        """List nodes shared with a user.

        Args:
            user_id: User identifier
            limit: Maximum results
            offset: Pagination offset

        Returns:
            List of shared entry dicts
        """
        return await self._run_sync(
            self._sync_get_shared_with_me, user_id, limit, offset
        )

    def _sync_get_shared_with_me(
        self, user_id: str, limit: int, offset: int
    ) -> list[dict]:
        with self._get_connection() as conn:
            rows = conn.execute(
                "SELECT * FROM shared_index WHERE user_id = ? ORDER BY shared_at DESC LIMIT ? OFFSET ?",
                (user_id, limit, offset),
            ).fetchall()
            return [dict(r) for r in rows]

    async def remove_all_shared_for_user(self, user_id: str) -> int:
        """Remove all shared entries for a user (e.g. on account deletion).

        Returns:
            Number of entries removed
        """
        return await self._run_sync(self._sync_remove_all_shared_for_user, user_id)

    def _sync_remove_all_shared_for_user(self, user_id: str) -> int:
        with self._get_connection() as conn:
            cursor = conn.execute(
                "DELETE FROM shared_index WHERE user_id = ?", (user_id,)
            )
            return cursor.rowcount

    # ── Shared index (extended) ──────────────────────────────────────

    async def cleanup_stale_shared(
        self, source_tenant: str, node_id: str
    ) -> int:
        """Remove all shared_index entries for a deleted node.

        Called when a node is deleted to clean up stale entries.
        This is safe because shared_index is a hint, not authoritative.

        Args:
            source_tenant: Tenant that owned the node
            node_id: The deleted node's ID

        Returns:
            Number of entries removed
        """
        return await self._run_sync(
            self._sync_cleanup_stale_shared, source_tenant, node_id
        )

    def _sync_cleanup_stale_shared(
        self, source_tenant: str, node_id: str
    ) -> int:
        with self._get_connection() as conn:
            cursor = conn.execute(
                "DELETE FROM shared_index WHERE source_tenant = ? AND node_id = ?",
                (source_tenant, node_id),
            )
            return cursor.rowcount

    async def get_shared_entries_for_node(
        self, source_tenant: str, node_id: str
    ) -> list[dict]:
        """Get all shared_index entries for a specific node.

        Args:
            source_tenant: Tenant that owns the node
            node_id: The node ID

        Returns:
            List of shared entry dicts
        """
        return await self._run_sync(
            self._sync_get_shared_entries_for_node, source_tenant, node_id
        )

    def _sync_get_shared_entries_for_node(
        self, source_tenant: str, node_id: str
    ) -> list[dict]:
        with self._get_connection() as conn:
            rows = conn.execute(
                """
                SELECT * FROM shared_index
                WHERE source_tenant = ? AND node_id = ?
                ORDER BY shared_at DESC
                """,
                (source_tenant, node_id),
            ).fetchall()
            return [dict(r) for r in rows]

    # ── Deletion queue ────────────────────────────────────────────────

    async def queue_deletion(self, user_id: str, grace_days: int = 30) -> dict:
        """Schedule a user for deletion after a grace period.

        Args:
            user_id: User to delete
            grace_days: Days until actual deletion

        Returns:
            Dict with deletion queue entry fields

        Raises:
            sqlite3.IntegrityError: If user already queued for deletion
        """
        return await self._run_sync(self._sync_queue_deletion, user_id, grace_days)

    def _sync_queue_deletion(self, user_id: str, grace_days: int) -> dict:
        now = self._now()
        execute_at = now + (grace_days * 86400)
        with self._get_connection() as conn:
            conn.execute(
                """
                INSERT INTO deletion_queue (user_id, requested_at, execute_at, status)
                VALUES (?, ?, ?, 'pending')
                """,
                (user_id, now, execute_at),
            )
            return {
                "user_id": user_id,
                "requested_at": now,
                "execute_at": execute_at,
                "export_path": None,
                "status": "pending",
            }

    async def cancel_deletion(self, user_id: str) -> bool:
        """Cancel a pending deletion.

        Returns:
            True if the entry existed and was cancelled, False otherwise
        """
        return await self._run_sync(self._sync_cancel_deletion, user_id)

    def _sync_cancel_deletion(self, user_id: str) -> bool:
        with self._get_connection() as conn:
            cursor = conn.execute(
                "DELETE FROM deletion_queue WHERE user_id = ? AND status = 'pending'",
                (user_id,),
            )
            return cursor.rowcount > 0

    async def get_pending_deletions(self) -> list[dict]:
        """List all pending deletion entries.

        Returns:
            List of deletion queue dicts
        """
        return await self._run_sync(self._sync_get_pending_deletions)

    def _sync_get_pending_deletions(self) -> list[dict]:
        with self._get_connection() as conn:
            rows = conn.execute(
                "SELECT * FROM deletion_queue WHERE status = 'pending' ORDER BY execute_at",
            ).fetchall()
            return [dict(r) for r in rows]
