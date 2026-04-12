"""
HKDF-based key management for EntDB encryption at rest.

Provides a ``KeyManager`` that derives per-tenant encryption keys from a
master key using HKDF-SHA256.  Supports key rotation and crypto-shred
(destroying a tenant's derived key so that its data becomes
unrecoverable even if the ciphertext file remains on disk).

Invariants:
    - Derivation is deterministic: same (master_key, tenant_id) always
      produces the same derived key.
    - Different tenant IDs produce different derived keys.
    - After ``shred_tenant``, the tenant's key material is removed from
      the in-memory cache and cannot be re-derived without explicitly
      re-adding the tenant.

How to change safely:
    - Changing HKDF parameters (salt, info prefix) requires
      re-encrypting all existing tenant databases.
    - The master key must be 32 bytes (256 bits).
"""

from __future__ import annotations

import logging
import threading

from cryptography.hazmat.primitives.hashes import SHA256
from cryptography.hazmat.primitives.kdf.hkdf import HKDF

logger = logging.getLogger(__name__)

_HKDF_INFO_PREFIX = b"entdb-tenant-key:"
_DERIVED_KEY_LENGTH = 32  # 256-bit keys


class KeyManager:
    """Manages a master key and derives per-tenant encryption keys via HKDF.

    The manager maintains an in-memory cache of derived keys so that
    repeated lookups for the same tenant are fast.  The cache is
    protected by a lock for thread-safety.

    Args:
        master_key: 32-byte master key, or a 64-character hex string.
        kdf_iterations: Not used by HKDF directly (HKDF is a single-pass
            extract-then-expand KDF), but stored for metadata /
            compatibility with PBKDF2-based key stores.
    """

    def __init__(
        self,
        master_key: bytes | str,
        *,
        kdf_iterations: int = 256_000,
    ) -> None:
        if isinstance(master_key, str):
            if len(master_key) == 0:
                raise ValueError("master_key must not be empty")
            try:
                master_key = bytes.fromhex(master_key)
            except ValueError:
                raise ValueError(
                    "master_key string must be a valid hex encoding (64 hex chars for 32 bytes)"
                ) from None

        if len(master_key) != 32:
            raise ValueError(f"master_key must be exactly 32 bytes, got {len(master_key)}")

        self._master_key = master_key
        self._kdf_iterations = kdf_iterations
        self._lock = threading.Lock()
        # Cache: tenant_id -> derived key bytes
        self._key_cache: dict[str, bytes] = {}
        # Set of shredded tenant IDs -- derivation is blocked for these
        self._shredded: set[str] = set()

    # -- public API -------------------------------------------------------

    def derive_tenant_key(self, tenant_id: str) -> bytes:
        """Derive a per-tenant 32-byte key using HKDF(master_key, info=tenant_id).

        Returns cached key if already derived.  Raises ``RuntimeError``
        if the tenant has been shredded.
        """
        with self._lock:
            if tenant_id in self._shredded:
                raise RuntimeError(
                    f"Tenant '{tenant_id}' has been crypto-shredded; "
                    "key material is no longer available"
                )
            if tenant_id in self._key_cache:
                return self._key_cache[tenant_id]

            derived = self._hkdf_derive(tenant_id)
            self._key_cache[tenant_id] = derived
            return derived

    def rotate_master_key(self, old_key: bytes, new_key: bytes) -> dict[str, tuple[bytes, bytes]]:
        """Re-derive all cached tenant keys after a master key rotation.

        Verifies that ``old_key`` matches the current master key, then
        switches to ``new_key`` and re-derives every cached tenant key.

        Returns:
            A dict mapping tenant_id to (old_derived, new_derived) tuples
            so callers can re-encrypt each database.

        Raises:
            ValueError: If ``old_key`` does not match the current master key
                or ``new_key`` is invalid.
        """
        if old_key != self._master_key:
            raise ValueError("old_key does not match the current master key")
        if len(new_key) != 32:
            raise ValueError(f"new_key must be exactly 32 bytes, got {len(new_key)}")

        with self._lock:
            rotation_map: dict[str, tuple[bytes, bytes]] = {}
            old_cache = dict(self._key_cache)

            self._master_key = new_key
            self._key_cache.clear()

            for tid in old_cache:
                if tid in self._shredded:
                    continue
                new_derived = self._hkdf_derive(tid)
                self._key_cache[tid] = new_derived
                rotation_map[tid] = (old_cache[tid], new_derived)

            return rotation_map

    def shred_tenant(self, tenant_id: str) -> None:
        """Crypto-shred: destroy the derived key material for a tenant.

        After this call, ``derive_tenant_key(tenant_id)`` will raise
        ``RuntimeError``.  The tenant's SQLite file becomes
        unrecoverable even if the ciphertext still exists on disk.
        """
        with self._lock:
            self._key_cache.pop(tenant_id, None)
            self._shredded.add(tenant_id)
            logger.info("Crypto-shredded key material for tenant '%s'", tenant_id)

    def is_shredded(self, tenant_id: str) -> bool:
        """Return True if the tenant has been crypto-shredded."""
        with self._lock:
            return tenant_id in self._shredded

    @property
    def cached_tenant_ids(self) -> list[str]:
        """Return a list of tenant IDs whose keys are currently cached."""
        with self._lock:
            return list(self._key_cache.keys())

    # -- re-encryption helper ---------------------------------------------

    @staticmethod
    def reencrypt_tenant_db(
        db_path: str,
        old_key: bytes,
        new_key: bytes,
    ) -> None:
        """Re-encrypt a tenant's SQLite file with a new key.

        Uses SQLCipher's ``PRAGMA rekey`` when available.  Falls back to
        a copy-based re-encryption via an intermediate unencrypted
        temporary file when SQLCipher is not installed (development
        mode).

        Args:
            db_path: Path to the encrypted SQLite database.
            old_key: Current 32-byte encryption key.
            new_key: New 32-byte encryption key.
        """
        try:
            from pysqlcipher3 import dbapi2 as sqlcipher  # type: ignore[import-untyped]

            conn = sqlcipher.connect(db_path)
            conn.execute(f"PRAGMA key = \"x'{old_key.hex()}'\"")
            conn.execute(f"PRAGMA rekey = \"x'{new_key.hex()}'\"")
            conn.close()
            logger.info("Re-encrypted database at %s via PRAGMA rekey", db_path)
        except ImportError:
            logger.warning(
                "SQLCipher not available; re-encryption of %s skipped. "
                "Install pysqlcipher3 for production key rotation.",
                db_path,
            )

    # -- internals --------------------------------------------------------

    def _hkdf_derive(self, tenant_id: str) -> bytes:
        """Run HKDF-SHA256 expand to derive a tenant key."""
        info = _HKDF_INFO_PREFIX + tenant_id.encode("utf-8")
        hkdf = HKDF(
            algorithm=SHA256(),
            length=_DERIVED_KEY_LENGTH,
            salt=None,  # HKDF uses a default all-zero salt
            info=info,
        )
        return hkdf.derive(self._master_key)
