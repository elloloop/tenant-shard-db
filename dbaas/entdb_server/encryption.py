"""
Encryption support for EntDB tenant SQLite files.

Provides per-tenant key derivation from a master key and SQLCipher
integration for encrypted databases.  SQLCipher is optional — if it
is not installed the module logs a warning and falls back to standard
unencrypted sqlite3.

Key derivation uses HMAC-SHA256 which provides a deterministic,
cryptographically strong mapping from (master_key, tenant_id) to a
per-tenant hex key.

Invariants:
    - Key derivation is deterministic: same inputs always produce the
      same key.
    - Different tenant IDs produce different keys.
    - The master key is never stored on disk or logged.

How to change safely:
    - Changing the derivation scheme requires re-encrypting all
      existing databases.
    - Adding KMS integration should replace the master_key string
      with a KMS-backed secret.
"""

from __future__ import annotations

import hashlib
import hmac
import logging
import sqlite3

logger = logging.getLogger(__name__)

# Try to import pysqlcipher3 which provides a sqlcipher-backed Connection.
# If unavailable we fall back to the standard sqlite3 module.
_sqlcipher_available = False
try:
    from pysqlcipher3 import dbapi2 as sqlcipher  # type: ignore[import-untyped]

    _sqlcipher_available = True
except ImportError:
    sqlcipher = None  # type: ignore[assignment]


def is_sqlcipher_available() -> bool:
    """Return True if the SQLCipher library is importable."""
    return _sqlcipher_available


def derive_tenant_key(master_key: str, tenant_id: str) -> str:
    """Derive a per-tenant encryption key from the master key.

    Uses HMAC-SHA256 with the master key as the HMAC key and a
    structured message incorporating the tenant ID.

    Args:
        master_key: The master encryption key.
        tenant_id: Unique tenant identifier used as the derivation salt.

    Returns:
        A 64-character hex string suitable for use as a SQLCipher PRAGMA key.
    """
    key_bytes = hmac.new(
        master_key.encode(),
        msg=f"entdb-tenant:{tenant_id}".encode(),
        digestmod=hashlib.sha256,
    ).hexdigest()
    return key_bytes


def derive_global_key(master_key: str) -> str:
    """Derive the encryption key for the global database.

    Uses a fixed info string so the global.db key is deterministic
    and distinct from any tenant key.

    Args:
        master_key: The master encryption key.

    Returns:
        A 64-character hex string.
    """
    key_bytes = hmac.new(
        master_key.encode(),
        msg=b"entdb-global:global.db",
        digestmod=hashlib.sha256,
    ).hexdigest()
    return key_bytes


def open_encrypted_connection(
    db_path: str,
    key: str,
    *,
    timeout: float = 5.0,
) -> sqlite3.Connection:
    """Open a SQLCipher-encrypted database connection.

    If sqlcipher is available, opens with the pysqlcipher3 driver and
    sets PRAGMA key.  Otherwise falls back to standard sqlite3 (data
    will NOT be encrypted) and logs a warning.

    Args:
        db_path: Filesystem path to the SQLite database file.
        key: Hex encryption key for PRAGMA key.
        timeout: Connection timeout in seconds.

    Returns:
        A sqlite3.Connection (or pysqlcipher3 Connection).
    """
    if _sqlcipher_available and sqlcipher is not None:
        conn = sqlcipher.connect(
            db_path,
            timeout=timeout,
            isolation_level=None,
            check_same_thread=False,
        )
        conn.execute(f"PRAGMA key = \"x'{key}'\"")
        return conn
    else:
        logger.warning(
            "SQLCipher not available — opening database WITHOUT encryption. "
            "Install pysqlcipher3 for encrypted tenant databases. "
            "db_path=%s",
            db_path,
        )
        conn = sqlite3.connect(
            db_path,
            timeout=timeout,
            isolation_level=None,
            check_same_thread=False,
        )
        return conn


def shred_tenant(data_dir: str, tenant_id: str) -> bool:
    """Delete a tenant's SQLite database file (and WAL / SHM sidecars).

    This is the file-deletion half of crypto-shred. For the full
    durable shred — which makes the data unrecoverable even if the
    SQLite ciphertext is restored from a backup — wire a
    :class:`~dbaas.entdb_server.crypto.tenant_key_vault.TenantKeyVault`
    into your :class:`~dbaas.entdb_server.crypto.key_manager.KeyManager`
    and call
    :func:`~dbaas.entdb_server.crypto.crypto_shred.crypto_shred_tenant`.
    With the vault configured, the per-tenant DEK is destroyed in the
    vault tombstone so re-deriving it is impossible.

    Args:
        data_dir: Directory containing tenant database files.
        tenant_id: Tenant to shred.

    Returns:
        True if files were removed, False if no files existed.
    """
    from pathlib import Path

    safe_id = "".join(c for c in tenant_id if c.isalnum() or c in "-_")
    base = Path(data_dir) / f"tenant_{safe_id}.db"

    removed = False
    for suffix in ("", "-wal", "-shm"):
        p = Path(str(base) + suffix)
        if p.exists():
            p.unlink()
            removed = True
            logger.info("Removed %s for tenant crypto-shred", p)

    return removed
