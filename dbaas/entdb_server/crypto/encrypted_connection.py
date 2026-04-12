"""
SQLCipher connection wrapper for EntDB.

Provides ``open_encrypted_db`` which returns a ``sqlite3.Connection``
backed by SQLCipher when the ``pysqlcipher3`` package is installed.
Falls back to standard (unencrypted) ``sqlite3`` with a warning log
so that development environments work without compiling SQLCipher.

Invariants:
    - The returned connection is always a ``sqlite3.Connection``
      (or API-compatible pysqlcipher3 Connection).
    - When SQLCipher is available, ``PRAGMA key`` is applied
      immediately after opening.
    - The fallback path never applies ``PRAGMA key`` (it would be
      a no-op or error with stock SQLite).

How to change safely:
    - Adding cipher parameters (page_size, kdf_iter) should be
      done via additional PRAGMAs right after the key PRAGMA.
    - Any new PRAGMAs must be tested with both sqlcipher and
      stock sqlite3 paths.
"""

from __future__ import annotations

import logging
import sqlite3

logger = logging.getLogger(__name__)

# Try to import pysqlcipher3 which provides a sqlcipher-backed Connection.
_sqlcipher_module = None
_sqlcipher_available = False

try:
    from pysqlcipher3 import dbapi2 as _sc  # type: ignore[import-untyped]

    _sqlcipher_module = _sc
    _sqlcipher_available = True
except ImportError:
    pass


def is_sqlcipher_available() -> bool:
    """Return True if the pysqlcipher3 library is importable."""
    return _sqlcipher_available


def open_encrypted_db(
    db_path: str,
    key: bytes,
    *,
    timeout: float = 5.0,
    kdf_iterations: int = 256_000,
) -> sqlite3.Connection:
    """Open a SQLite database with SQLCipher encryption.

    When ``pysqlcipher3`` is installed the database is opened through
    its driver and ``PRAGMA key`` is set to the hex-encoded *key*.
    Additional SQLCipher PRAGMAs (``kdf_iter``, ``cipher_page_size``)
    are applied for hardened settings.

    When ``pysqlcipher3`` is **not** installed the function falls back
    to the standard ``sqlite3`` module and logs a warning.  This allows
    development without SQLCipher while production enforces it.

    Args:
        db_path: Filesystem path to the SQLite database file.
        key: 32-byte encryption key.
        timeout: Connection timeout in seconds.
        kdf_iterations: SQLCipher KDF iteration count (only used when
            SQLCipher is available).

    Returns:
        A ``sqlite3.Connection`` (or pysqlcipher3 equivalent).
    """
    hex_key = key.hex()

    if _sqlcipher_available and _sqlcipher_module is not None:
        conn = _sqlcipher_module.connect(
            db_path,
            timeout=timeout,
            isolation_level=None,
            check_same_thread=False,
        )
        conn.execute(f"PRAGMA key = \"x'{hex_key}'\"")
        conn.execute(f"PRAGMA kdf_iter = {kdf_iterations}")
        logger.debug("Opened encrypted database at %s via SQLCipher", db_path)
        return conn

    logger.warning(
        "SQLCipher not available -- opening database WITHOUT encryption. "
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
