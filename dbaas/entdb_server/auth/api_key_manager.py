"""
API key management with scoping and rotation for EntDB.

Supports creating, validating, rotating, and revoking API keys.
Keys are stored as SHA-256 hashes -- plaintext secrets are never persisted.

Scopes: read, write, admin, gdpr, audit

Usage:
    manager = ApiKeyManager(global_store)
    result = await manager.create_key("my-key", ["read", "write"])
    # result["key_secret"] is the plaintext secret (shown once)
    info = await manager.validate_key(result["key_secret"])

Invariants:
    - Secrets are stored as sha256 hashes, never plaintext
    - Scopes are validated against the known set
    - Rotated keys immediately invalidate the old secret
    - Revoked keys cannot be validated or rotated

How to change safely:
    - Add new scopes to VALID_SCOPES
    - Keep validate_key constant-time where possible
    - Never return key_secret_hash in public API responses
"""

from __future__ import annotations

import hashlib
import logging
import secrets
import time
from typing import Any

logger = logging.getLogger(__name__)

VALID_SCOPES = frozenset({"read", "write", "admin", "gdpr", "audit"})


class ApiKeyError(Exception):
    """Raised on API key operation failures."""


class ApiKeyManager:
    """Manage API keys with scoping and rotation.

    Keys are stored in an in-memory dict keyed by ``key_id``.  Each entry
    stores the SHA-256 hash of the secret, associated scopes, metadata,
    and revocation status.

    In production, the backing store would be the GlobalStore (SQLite or
    Postgres).  This implementation uses an in-memory dict for the
    foundation layer.

    Args:
        global_store: Optional backing store (unused in the in-memory
            implementation; reserved for future persistence).
    """

    def __init__(self, global_store: Any = None) -> None:
        self._store = global_store
        # key_id -> key metadata
        self._keys: dict[str, dict[str, Any]] = {}
        # sha256(secret) -> key_id   (reverse lookup for validate)
        self._hash_index: dict[str, str] = {}

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    async def create_key(
        self,
        name: str,
        scopes: list[str],
        *,
        expires_at: int | None = None,
    ) -> dict:
        """Create a new API key with specific scopes.

        Args:
            name: Human-readable name for the key.
            scopes: List of scope strings (must be a subset of VALID_SCOPES).
            expires_at: Optional Unix timestamp for key expiration.

        Returns:
            Dict with ``key_id``, ``key_secret``, ``name``, ``scopes``,
            ``created_at``, and ``expires_at``.

        Raises:
            ApiKeyError: If any scope is invalid.
        """
        self._validate_scopes(scopes)

        key_id = f"ek_{secrets.token_hex(16)}"
        key_secret = f"eks_{secrets.token_hex(32)}"
        secret_hash = self._hash_secret(key_secret)
        now = int(time.time())

        entry: dict[str, Any] = {
            "key_id": key_id,
            "name": name,
            "scopes": sorted(set(scopes)),
            "secret_hash": secret_hash,
            "created_at": now,
            "expires_at": expires_at,
            "revoked": False,
            "rotated_at": None,
        }
        self._keys[key_id] = entry
        self._hash_index[secret_hash] = key_id

        logger.info("Created API key %s (name=%s, scopes=%s)", key_id, name, entry["scopes"])

        return {
            "key_id": key_id,
            "key_secret": key_secret,
            "name": name,
            "scopes": entry["scopes"],
            "created_at": now,
            "expires_at": expires_at,
        }

    async def validate_key(self, key_secret: str) -> dict:
        """Validate an API key and return its metadata + scopes.

        Args:
            key_secret: The plaintext API key secret.

        Returns:
            Dict with ``key_id``, ``name``, ``scopes``, ``created_at``,
            and ``expires_at``.

        Raises:
            ApiKeyError: If the key is invalid, revoked, or expired.
        """
        secret_hash = self._hash_secret(key_secret)
        key_id = self._hash_index.get(secret_hash)
        if key_id is None:
            raise ApiKeyError("Invalid API key")

        entry = self._keys.get(key_id)
        if entry is None:
            raise ApiKeyError("Invalid API key")

        if entry["revoked"]:
            raise ApiKeyError("API key has been revoked")

        if entry["expires_at"] is not None and int(time.time()) >= entry["expires_at"]:
            raise ApiKeyError("API key has expired")

        return {
            "key_id": entry["key_id"],
            "name": entry["name"],
            "scopes": entry["scopes"],
            "created_at": entry["created_at"],
            "expires_at": entry["expires_at"],
        }

    async def rotate_key(self, key_id: str) -> dict:
        """Generate a new secret for an existing key.

        The old secret is immediately invalidated.

        Args:
            key_id: The ID of the key to rotate.

        Returns:
            Dict with ``key_id``, ``key_secret`` (new), ``name``,
            ``scopes``, ``created_at``, ``expires_at``, and ``rotated_at``.

        Raises:
            ApiKeyError: If the key does not exist or is revoked.
        """
        entry = self._keys.get(key_id)
        if entry is None:
            raise ApiKeyError(f"API key '{key_id}' not found")

        if entry["revoked"]:
            raise ApiKeyError(f"API key '{key_id}' has been revoked")

        # Remove old hash from index
        old_hash = entry["secret_hash"]
        self._hash_index.pop(old_hash, None)

        # Generate new secret
        new_secret = f"eks_{secrets.token_hex(32)}"
        new_hash = self._hash_secret(new_secret)
        now = int(time.time())

        entry["secret_hash"] = new_hash
        entry["rotated_at"] = now
        self._hash_index[new_hash] = key_id

        logger.info("Rotated API key %s", key_id)

        return {
            "key_id": key_id,
            "key_secret": new_secret,
            "name": entry["name"],
            "scopes": entry["scopes"],
            "created_at": entry["created_at"],
            "expires_at": entry["expires_at"],
            "rotated_at": now,
        }

    async def revoke_key(self, key_id: str) -> None:
        """Permanently revoke an API key.

        Args:
            key_id: The ID of the key to revoke.

        Raises:
            ApiKeyError: If the key does not exist.
        """
        entry = self._keys.get(key_id)
        if entry is None:
            raise ApiKeyError(f"API key '{key_id}' not found")

        # Remove from hash index so validate_key fails fast
        old_hash = entry["secret_hash"]
        self._hash_index.pop(old_hash, None)

        entry["revoked"] = True
        logger.info("Revoked API key %s", key_id)

    async def list_keys(self) -> list[dict]:
        """List all API keys (without secrets or hashes).

        Returns:
            List of dicts with ``key_id``, ``name``, ``scopes``,
            ``created_at``, ``expires_at``, ``revoked``, and ``rotated_at``.
        """
        return [
            {
                "key_id": e["key_id"],
                "name": e["name"],
                "scopes": e["scopes"],
                "created_at": e["created_at"],
                "expires_at": e["expires_at"],
                "revoked": e["revoked"],
                "rotated_at": e["rotated_at"],
            }
            for e in self._keys.values()
        ]

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    @staticmethod
    def _hash_secret(secret: str) -> str:
        """Return the SHA-256 hex digest of a secret."""
        return hashlib.sha256(secret.encode("utf-8")).hexdigest()

    @staticmethod
    def _validate_scopes(scopes: list[str]) -> None:
        """Validate that all scopes are known."""
        invalid = set(scopes) - VALID_SCOPES
        if invalid:
            raise ApiKeyError(f"Invalid scopes: {sorted(invalid)}. Valid: {sorted(VALID_SCOPES)}")
