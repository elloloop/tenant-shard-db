"""
Crypto-shred operations for EntDB tenants.

Crypto-shredding makes a tenant's data unrecoverable by destroying the
encryption key material.  Even if the SQLite ciphertext file remains on
disk, it cannot be decrypted without the key.

Optionally the ciphertext file can also be deleted for defense in depth.

Invariants:
    - After ``crypto_shred_tenant`` the key manager will refuse to
      derive the tenant's key.
    - File deletion (if requested) removes .db, -wal, and -shm files.
    - The function is idempotent: calling it twice for the same tenant
      is safe.

How to change safely:
    - Adding audit-log integration should happen inside this function
      so that every shred event is recorded.
"""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)


async def crypto_shred_tenant(
    tenant_id: str,
    key_manager: Any,
    canonical_store: Any = None,
    *,
    delete_files: bool = False,
    data_dir: str | None = None,
) -> dict[str, Any]:
    """Crypto-shred a tenant by destroying its encryption key.

    The SQLite file becomes unrecoverable.  Optionally also deletes
    the ciphertext file from disk.

    Args:
        tenant_id: The tenant to shred.
        key_manager: A ``KeyManager`` instance.
        canonical_store: Optional ``CanonicalStore`` -- if provided,
            the tenant's connection is closed before shredding.
        delete_files: If True, also remove the tenant's database files
            from disk.
        data_dir: Directory containing tenant database files.  Required
            when ``delete_files`` is True and ``canonical_store`` is
            None.

    Returns:
        A summary dict with keys:
            - ``tenant_id``: the tenant that was shredded.
            - ``key_destroyed``: True if the key was destroyed.
            - ``files_deleted``: True if database files were removed.
            - ``files_found``: list of deleted file paths (if any).
    """
    summary: dict[str, Any] = {
        "tenant_id": tenant_id,
        "key_destroyed": False,
        "files_deleted": False,
        "files_found": [],
    }

    # 1. Close the tenant's connection if a canonical store is provided.
    if canonical_store is not None:
        try:
            if hasattr(canonical_store, "close_tenant"):
                canonical_store.close_tenant(tenant_id)
            elif hasattr(canonical_store, "close_all"):
                pass  # caller can close_all separately
        except Exception:
            logger.warning(
                "Failed to close connection for tenant '%s' during crypto-shred",
                tenant_id,
                exc_info=True,
            )

    # 2. Destroy the key material.
    try:
        key_manager.shred_tenant(tenant_id)
        summary["key_destroyed"] = True
        logger.info("Destroyed key material for tenant '%s'", tenant_id)
    except Exception:
        logger.error(
            "Failed to shred key material for tenant '%s'",
            tenant_id,
            exc_info=True,
        )

    # 3. Optionally delete the ciphertext files.
    if delete_files:
        resolved_dir = data_dir
        if resolved_dir is None and canonical_store is not None:
            resolved_dir = getattr(canonical_store, "data_dir", None)

        if resolved_dir is None:
            logger.warning(
                "delete_files=True but no data_dir provided and "
                "canonical_store has no data_dir attribute; skipping file deletion"
            )
        else:
            deleted = _delete_tenant_files(resolved_dir, tenant_id)
            summary["files_deleted"] = len(deleted) > 0
            summary["files_found"] = deleted

    return summary


def _delete_tenant_files(data_dir: str, tenant_id: str) -> list[str]:
    """Delete tenant database files (.db, -wal, -shm).

    Sanitizes the tenant_id to prevent path traversal.

    Returns:
        List of deleted file paths.
    """
    safe_id = "".join(c for c in tenant_id if c.isalnum() or c in "-_")
    base = Path(data_dir) / f"tenant_{safe_id}.db"

    deleted: list[str] = []
    for suffix in ("", "-wal", "-shm"):
        p = Path(str(base) + suffix)
        if p.exists():
            p.unlink()
            deleted.append(str(p))
            logger.info("Deleted %s during crypto-shred of tenant '%s'", p, tenant_id)

    return deleted
