"""
Encryption-at-rest foundation for EntDB.

Provides per-tenant key derivation using HKDF, SQLCipher integration,
key rotation, and crypto-shred capabilities.

Modules:
    key_manager          -- HKDF-based key hierarchy and rotation
    encrypted_connection -- SQLCipher connection wrapper with fallback
    crypto_shred         -- Tenant crypto-shred operations
"""

from __future__ import annotations

from .crypto_shred import crypto_shred_tenant
from .encrypted_connection import open_encrypted_db
from .key_manager import KeyManager

__all__ = [
    "KeyManager",
    "crypto_shred_tenant",
    "open_encrypted_db",
]
