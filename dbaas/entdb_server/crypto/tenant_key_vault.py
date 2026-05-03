"""
SQLite-backed vault for per-tenant encryption keys.

Stores random per-tenant data-encryption keys (DEKs) wrapped with the
master key-encryption-key (KEK) using AES-GCM. Wrapping with the master
KEK means the master is never present in the persistent store; rotating
the master only requires re-wrapping each row.

Crypto-shred is implemented by NULLing out the ``wrapped_key`` column
and stamping ``shredded_at``. After shred the row remains as a
tombstone — ``KeyManager`` consults it to refuse re-derivation. The
ciphertext is irrecoverable because the wrapped DEK is gone, even if
backups of the SQLite file are recovered.

Invariants:
    - ``wrapped_key`` is NULL **iff** ``shredded_at`` is non-NULL.
    - A tombstone row is never re-promoted to active by ``provision``;
      the caller must explicitly delete the row first if they want to
      re-create the tenant (a separate operation that breaks shred
      semantics and is intentionally not exposed here).
    - Wrap nonces are random and 12 bytes; AES-GCM authentication tag
      protects integrity, so any modification of the stored wrapped_key
      surfaces as ``InvalidTag`` on unwrap rather than silent corruption.

Wire format (BLOB column ``wrapped_key``):
    ``nonce (12 bytes) || ciphertext (len(DEK) bytes) || tag (16 bytes)``

For a 32-byte DEK the BLOB is 60 bytes total.
"""

from __future__ import annotations

import logging
import os
import sqlite3
import threading
import time
from dataclasses import dataclass

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

logger = logging.getLogger(__name__)

_NONCE_LEN = 12
_TAG_LEN = 16
_DEK_LEN = 32  # 256-bit data-encryption keys


class TenantKeyVaultError(Exception):
    """Base class for vault errors."""


class TenantShreddedError(TenantKeyVaultError):
    """Raised when a caller asks for a tenant whose key has been shredded."""

    def __init__(self, tenant_id: str, shredded_at: int) -> None:
        super().__init__(
            f"Tenant {tenant_id!r} was crypto-shredded at "
            f"{shredded_at} (epoch ms); key material is unrecoverable"
        )
        self.tenant_id = tenant_id
        self.shredded_at = shredded_at


class TenantKeyAlreadyProvisionedError(TenantKeyVaultError):
    """Raised when ``provision`` is called for a tenant whose row already exists."""


@dataclass(frozen=True)
class _VaultRow:
    tenant_id: str
    wrapped_key: bytes | None
    created_at: int
    shredded_at: int | None


class TenantKeyVault:
    """SQLite-backed wrap-and-stash store for per-tenant DEKs.

    The vault is a thin persistence layer — it does not enforce
    "provision-before-get" ordering; ``KeyManager`` orchestrates that.
    Methods are individually atomic and thread-safe. Multi-step
    workflows (e.g. provision-if-missing) are the caller's
    responsibility and should use the higher-level ``KeyManager`` API.

    Args:
        db_path: Filesystem path to the vault SQLite file. The schema
            is created on first connection if absent.
        master_kek: 32-byte master key. Used as the AES-GCM key for
            wrapping/unwrapping. Never stored on disk.
    """

    def __init__(self, db_path: str, master_kek: bytes) -> None:
        if len(master_kek) != _DEK_LEN:
            raise ValueError(f"master_kek must be exactly {_DEK_LEN} bytes, got {len(master_kek)}")
        self._db_path = db_path
        self._aead = AESGCM(master_kek)
        self._lock = threading.Lock()
        self._init_schema()

    # ------------------------------------------------------------------
    # Schema
    # ------------------------------------------------------------------

    def _connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(
            self._db_path,
            timeout=5.0,
            isolation_level=None,  # autocommit; we use BEGIN for atomicity
            check_same_thread=False,
        )
        conn.execute("PRAGMA journal_mode = WAL")
        conn.execute("PRAGMA synchronous = NORMAL")
        return conn

    def _init_schema(self) -> None:
        with self._connect() as conn:
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS tenant_keys (
                    tenant_id    TEXT PRIMARY KEY NOT NULL,
                    wrapped_key  BLOB,
                    created_at   INTEGER NOT NULL,
                    shredded_at  INTEGER,
                    CHECK (
                        (wrapped_key IS NOT NULL AND shredded_at IS NULL) OR
                        (wrapped_key IS NULL AND shredded_at IS NOT NULL)
                    )
                )
                """
            )

    # ------------------------------------------------------------------
    # Wrap / unwrap
    # ------------------------------------------------------------------

    def _wrap(self, dek: bytes, tenant_id: str) -> bytes:
        if len(dek) != _DEK_LEN:
            raise ValueError(f"dek must be exactly {_DEK_LEN} bytes, got {len(dek)}")
        nonce = os.urandom(_NONCE_LEN)
        # Bind the wrap to the tenant_id via AAD so a row swapped to
        # another tenant_id (e.g. via a backup-restore tampering attack)
        # fails to unwrap rather than silently authenticating.
        aad = tenant_id.encode("utf-8")
        ct_with_tag = self._aead.encrypt(nonce, dek, aad)
        return nonce + ct_with_tag

    def _unwrap(self, blob: bytes, tenant_id: str) -> bytes:
        if len(blob) < _NONCE_LEN + _TAG_LEN:
            raise TenantKeyVaultError(
                f"vault row for {tenant_id!r} is too short to be a valid wrap"
            )
        nonce, ct = blob[:_NONCE_LEN], blob[_NONCE_LEN:]
        aad = tenant_id.encode("utf-8")
        try:
            return self._aead.decrypt(nonce, ct, aad)
        except InvalidTag as exc:
            raise TenantKeyVaultError(
                f"vault row for {tenant_id!r} failed AES-GCM authentication"
            ) from exc

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def get_row(self, tenant_id: str) -> _VaultRow | None:
        """Return the raw vault row for ``tenant_id`` or ``None`` if absent."""
        with self._lock, self._connect() as conn:
            cursor = conn.execute(
                "SELECT tenant_id, wrapped_key, created_at, shredded_at "
                "FROM tenant_keys WHERE tenant_id = ?",
                (tenant_id,),
            )
            row = cursor.fetchone()
            if row is None:
                return None
            return _VaultRow(
                tenant_id=row[0],
                wrapped_key=bytes(row[1]) if row[1] is not None else None,
                created_at=row[2],
                shredded_at=row[3],
            )

    def get(self, tenant_id: str) -> bytes:
        """Return the unwrapped DEK for ``tenant_id``.

        Raises:
            TenantShreddedError: The tenant was crypto-shredded.
            KeyError: No row exists for the tenant — caller should
                provision before retrying.
        """
        row = self.get_row(tenant_id)
        if row is None:
            raise KeyError(tenant_id)
        if row.shredded_at is not None:
            raise TenantShreddedError(tenant_id, row.shredded_at)
        if row.wrapped_key is None:
            # Defensive — the CHECK constraint should prevent this.
            raise TenantKeyVaultError(
                f"vault row for {tenant_id!r} is in an inconsistent state "
                "(wrapped_key NULL but shredded_at also NULL)"
            )
        return self._unwrap(row.wrapped_key, tenant_id)

    def provision(self, tenant_id: str, dek: bytes) -> None:
        """Insert a new vault row holding *dek* wrapped with the master KEK.

        Idempotency is intentionally NOT provided — the caller is
        expected to ``get_row`` first. This matters because re-provision
        on top of a shredded tenant would defeat the shred. To re-create
        a tenant the caller must explicitly delete the row first.

        Raises:
            TenantKeyAlreadyProvisionedError: A row already exists.
            TenantShreddedError: The row exists and is shredded.
        """
        existing = self.get_row(tenant_id)
        if existing is not None:
            if existing.shredded_at is not None:
                raise TenantShreddedError(tenant_id, existing.shredded_at)
            raise TenantKeyAlreadyProvisionedError(f"tenant {tenant_id!r} already has a vault row")
        wrapped = self._wrap(dek, tenant_id)
        now_ms = int(time.time() * 1000)
        with self._lock, self._connect() as conn:
            try:
                conn.execute(
                    "INSERT INTO tenant_keys (tenant_id, wrapped_key, created_at) VALUES (?, ?, ?)",
                    (tenant_id, wrapped, now_ms),
                )
            except sqlite3.IntegrityError as exc:
                # Lost a race with a concurrent provisioner — surface
                # the same error shape the caller would have gotten.
                raise TenantKeyAlreadyProvisionedError(
                    f"tenant {tenant_id!r} already has a vault row (race)"
                ) from exc

    def shred(self, tenant_id: str) -> bool:
        """Crypto-shred the tenant's DEK.

        Sets ``wrapped_key = NULL`` and ``shredded_at = now``. Idempotent
        — calling shred on an already-shredded tenant is a no-op.

        Returns:
            True if the row existed (active or already shredded),
            False if no row was present (nothing to shred).
        """
        now_ms = int(time.time() * 1000)
        with self._lock, self._connect() as conn:
            # First check existence so we can return a meaningful bool.
            cursor = conn.execute(
                "SELECT shredded_at FROM tenant_keys WHERE tenant_id = ?",
                (tenant_id,),
            )
            row = cursor.fetchone()
            if row is None:
                # No row — caller may want to insert a tombstone so
                # future provisioning attempts surface the shred. We
                # intentionally do NOT do that automatically; "shred a
                # tenant that never existed" is almost always a bug
                # the caller should notice.
                return False
            if row[0] is not None:
                # Already shredded — idempotent.
                return True
            conn.execute(
                "UPDATE tenant_keys SET wrapped_key = NULL, shredded_at = ? WHERE tenant_id = ?",
                (now_ms, tenant_id),
            )
            logger.info("Crypto-shredded vault row for tenant '%s' at %d", tenant_id, now_ms)
            return True

    def is_shredded(self, tenant_id: str) -> bool:
        """Return True if the tenant has an active shred tombstone."""
        row = self.get_row(tenant_id)
        return row is not None and row.shredded_at is not None

    def list_tenant_ids(self, *, include_shredded: bool = False) -> list[str]:
        """Return the tenant IDs present in the vault.

        Used by ``KeyManager.rotate_master_key`` to enumerate rows that
        need re-wrapping after a master rotation. Shredded rows are
        skipped by default — they have no ``wrapped_key`` to re-wrap.
        """
        with self._lock, self._connect() as conn:
            if include_shredded:
                cursor = conn.execute("SELECT tenant_id FROM tenant_keys")
            else:
                cursor = conn.execute("SELECT tenant_id FROM tenant_keys WHERE shredded_at IS NULL")
            return [r[0] for r in cursor.fetchall()]

    def rewrap_with_new_master(self, new_master_kek: bytes) -> int:
        """Rewrap every active row with a new master KEK.

        Used by master-key rotation. The unwrapped DEKs are unchanged —
        SQLite tenant files do NOT need re-encrypting because their key
        is the DEK, not the KEK. Only the wrap layer rotates.

        Returns:
            Number of rows rewrapped.
        """
        if len(new_master_kek) != _DEK_LEN:
            raise ValueError(
                f"new_master_kek must be exactly {_DEK_LEN} bytes, got {len(new_master_kek)}"
            )
        new_aead = AESGCM(new_master_kek)
        rewrapped = 0
        with self._lock, self._connect() as conn:
            conn.execute("BEGIN IMMEDIATE")
            try:
                cursor = conn.execute(
                    "SELECT tenant_id, wrapped_key FROM tenant_keys WHERE shredded_at IS NULL"
                )
                rows = cursor.fetchall()
                for tenant_id, wrapped in rows:
                    dek = self._unwrap(bytes(wrapped), tenant_id)
                    nonce = os.urandom(_NONCE_LEN)
                    aad = tenant_id.encode("utf-8")
                    new_blob = nonce + new_aead.encrypt(nonce, dek, aad)
                    conn.execute(
                        "UPDATE tenant_keys SET wrapped_key = ? WHERE tenant_id = ?",
                        (new_blob, tenant_id),
                    )
                    rewrapped += 1
                conn.execute("COMMIT")
            except Exception:
                conn.execute("ROLLBACK")
                raise
        # Switch our own AEAD over so subsequent get/provision uses the new KEK.
        self._aead = new_aead
        return rewrapped
