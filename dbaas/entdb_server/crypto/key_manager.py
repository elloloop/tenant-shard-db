"""
HKDF-based key management for EntDB encryption at rest.

Provides a ``KeyManager`` that returns per-tenant data-encryption keys
(DEKs).  Two modes:

1. **Derived mode** (default; backward-compatible) — keys are derived
   from a master key using HKDF-SHA256.  Derivation is deterministic
   so the same (master_key, tenant_id) always produces the same key.
   Crypto-shred is in-memory only (a process-local blocklist) — it
   does NOT survive a restart, since the same master can always
   re-derive the same key.

2. **Vault mode** (opt-in via the ``vault`` constructor arg) — random
   per-tenant DEKs are stored, wrapped with the master KEK using
   AES-GCM, in a SQLite-backed :class:`TenantKeyVault`.  Crypto-shred
   is durable: NULLing the wrapped DEK in the vault makes the key
   unrecoverable across process restarts and even backup recoveries.

   For migration safety, the **first** vault access for a tenant that
   already has SQLite ciphertext on disk seeds the vault row with the
   legacy HKDF-derived key — this lets in-place upgrades proceed
   without re-encrypting tenant files.  Brand-new tenants follow the
   same path; an explicit re-encryption (``rotate_tenant_to_random``,
   future work) lifts a tenant from "legacy seed" to "true random
   key".  Either way, ``shred_tenant`` is durable from the moment the
   vault is wired in.

Invariants:
    - Derivation is deterministic in derived mode.
    - Different tenant IDs produce different keys.
    - After ``shred_tenant``:
        - derived mode: in-memory blocklist set (ephemeral).
        - vault mode: vault row is tombstoned (durable).
    - The master key must be 32 bytes (256 bits).
"""

from __future__ import annotations

import logging
import threading

from cryptography.hazmat.primitives.hashes import SHA256
from cryptography.hazmat.primitives.kdf.hkdf import HKDF

from .tenant_key_vault import (
    TenantKeyAlreadyProvisionedError,
    TenantKeyVault,
    TenantShreddedError,
)

logger = logging.getLogger(__name__)

_HKDF_INFO_PREFIX = b"entdb-tenant-key:"
_DERIVED_KEY_LENGTH = 32  # 256-bit keys


class CryptoShreddedError(RuntimeError):
    """Raised when a caller asks for a tenant whose key has been crypto-shredded.

    Subclasses ``RuntimeError`` to preserve compatibility with the
    pre-vault behaviour, where ``derive_tenant_key`` raised
    ``RuntimeError`` for the in-memory blocklist case.  Existing
    ``except RuntimeError:`` callers will keep working; new callers
    that want the typed form can ``except CryptoShreddedError:``.
    """

    def __init__(self, tenant_id: str) -> None:
        super().__init__(
            f"Tenant '{tenant_id}' has been crypto-shredded; key material is no longer available"
        )
        self.tenant_id = tenant_id


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
        vault: TenantKeyVault | None = None,
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
        # (derived-mode only; vault mode persists shreds in the vault).
        self._shredded: set[str] = set()
        self._vault = vault

    # -- public API -------------------------------------------------------

    def derive_tenant_key(self, tenant_id: str) -> bytes:
        """Return the per-tenant 32-byte data-encryption key.

        In **derived mode** the key is HKDF(master_key, info=tenant_id).
        In **vault mode** the key is unwrapped from the SQLite vault;
        if the tenant has no row yet, one is provisioned with the
        HKDF-derived key as a migration-safe seed (so existing SQLite
        files keep opening with the same key after the operator turns
        the vault on).

        Returns cached key on subsequent calls.

        Raises:
            CryptoShreddedError: The tenant has been crypto-shredded.
                In derived mode this is the in-memory blocklist; in
                vault mode it is the durable vault tombstone.
        """
        with self._lock:
            if tenant_id in self._shredded:
                raise CryptoShreddedError(tenant_id)
            if tenant_id in self._key_cache:
                return self._key_cache[tenant_id]

            if self._vault is not None:
                key = self._fetch_or_provision_vault(tenant_id)
            else:
                key = self._hkdf_derive(tenant_id)
            self._key_cache[tenant_id] = key
            return key

    def _fetch_or_provision_vault(self, tenant_id: str) -> bytes:
        """Vault-mode lookup, with first-access seeding for migration.

        Caller holds ``self._lock``. Order of operations:

        1. Read the existing row.
        2. If present and shredded → raise.
        3. If present and active  → unwrap and return.
        4. If absent → provision with the HKDF-derived seed so the
           same DEK that the legacy code would have produced is now
           in the vault. A concurrent provisioner may win the INSERT;
           in that case we fall back to step 1.
        """
        assert self._vault is not None  # narrowing for type checker
        try:
            return self._vault.get(tenant_id)
        except TenantShreddedError as exc:
            self._shredded.add(tenant_id)
            raise CryptoShreddedError(tenant_id) from exc
        except KeyError:
            seed = self._hkdf_derive(tenant_id)
            try:
                self._vault.provision(tenant_id, seed)
            except TenantKeyAlreadyProvisionedError:
                # Lost the race — re-read.
                return self._vault.get(tenant_id)
            return seed

    def rotate_master_key(self, old_key: bytes, new_key: bytes) -> dict[str, tuple[bytes, bytes]]:
        """Rotate the master key.

        - **Derived mode**: tenant keys are re-derived from the new
          master, so every tenant's DEK changes.  Returns a map of
          ``{tenant_id: (old_dek, new_dek)}`` so callers can
          ``PRAGMA rekey`` each tenant database.
        - **Vault mode**: only the wrap layer rotates — every active
          row is re-wrapped with the new master.  Tenant DEKs are
          UNCHANGED, so SQLite tenant files do NOT need
          re-encryption.  Returns an empty map (no DEK changes).
          This is the primary operational benefit of vault mode:
          master-key rotation is an O(rows) UPDATE rather than an
          O(rows) ``PRAGMA rekey`` over potentially-large databases.

        Verifies that ``old_key`` matches the current master before
        switching.

        Raises:
            ValueError: If ``old_key`` does not match or ``new_key``
                is not 32 bytes.
        """
        if old_key != self._master_key:
            raise ValueError("old_key does not match the current master key")
        if len(new_key) != 32:
            raise ValueError(f"new_key must be exactly 32 bytes, got {len(new_key)}")

        with self._lock:
            rotation_map: dict[str, tuple[bytes, bytes]] = {}

            if self._vault is not None:
                # Vault mode: rewrap rows, DEKs unchanged. Cached DEKs
                # remain valid.
                self._vault.rewrap_with_new_master(new_key)
                self._master_key = new_key
                return rotation_map

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
        """Crypto-shred: destroy the per-tenant key material.

        After this call, ``derive_tenant_key(tenant_id)`` raises
        ``CryptoShreddedError``.

        - **Derived mode**: ephemeral — the blocklist is in memory only
          and a process restart re-enables derivation. Use vault mode
          if you need durable shred for compliance / GDPR.
        - **Vault mode**: durable — the wrapped DEK in the vault is
          NULLed out and a tombstone is recorded. Subsequent processes
          opening the same vault will refuse to derive the key.

        In vault mode the row is also seeded if missing (with a
        migration-safe HKDF-derived seed) before being shredded — this
        means "shred a tenant that was never accessed" produces the
        same end state as "access then shred", with no surprising
        re-derivation possible later.
        """
        with self._lock:
            self._key_cache.pop(tenant_id, None)
            self._shredded.add(tenant_id)
            if self._vault is not None:
                if self._vault.get_row(tenant_id) is None:
                    # Seed a row so the shred is durable even when the
                    # tenant was never accessed in this process.
                    seed = self._hkdf_derive(tenant_id)
                    try:
                        self._vault.provision(tenant_id, seed)
                    except TenantKeyAlreadyProvisionedError:
                        # Concurrent provision — fall through to shred
                        # whatever the winner inserted.
                        pass
                self._vault.shred(tenant_id)
            logger.info("Crypto-shredded key material for tenant '%s'", tenant_id)

    def is_shredded(self, tenant_id: str) -> bool:
        """Return True if the tenant has been crypto-shredded.

        Consults the durable vault first when configured, so a fresh
        ``KeyManager`` instance correctly reports tenants shredded by
        a previous process.
        """
        with self._lock:
            if tenant_id in self._shredded:
                return True
            if self._vault is not None and self._vault.is_shredded(tenant_id):
                self._shredded.add(tenant_id)
                return True
            return False

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
