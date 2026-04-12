"""Admin operation handlers for EntDB (Issues #90, #91, #92).

ARCHITECTURE: Per-tenant mutations (transfer ownership, revoke ACL)
are appended to the WAL as admin event types. The Applier processes
them like any other event, ensuring they survive WAL replay.

Global-store operations (membership, legal holds) write to the global
SQLite directly — they are cross-tenant metadata, not per-tenant data,
and are not subject to WAL replay.
"""

from __future__ import annotations

import logging
import time
import uuid
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from dbaas.entdb_server.global_store import GlobalStore
    from dbaas.entdb_server.wal.base import WalStream

logger = logging.getLogger(__name__)


async def handle_transfer_user_content(
    global_store: GlobalStore,
    wal: WalStream,
    tenant_id: str,
    from_user: str,
    to_user: str,
    actor: str,
) -> dict[str, Any]:
    """Transfer all content owned by *from_user* to *to_user* in a tenant.

    The per-tenant ownership change is appended to the WAL so it
    survives rebuild. The global membership update is a direct write
    (global_store is not per-tenant WAL-sourced).
    """
    gs_result = await global_store.transfer_user_content(tenant_id, from_user, to_user)

    import json

    event = {
        "tenant_id": tenant_id,
        "actor": actor,
        "idempotency_key": f"admin-transfer-{uuid.uuid4()}",
        "ts_ms": int(time.time() * 1000),
        "ops": [
            {
                "op": "admin_transfer_content",
                "from_user": from_user,
                "to_user": to_user,
            }
        ],
    }
    await wal.append(key=tenant_id, value=json.dumps(event).encode())

    return {
        "tenant_id": tenant_id,
        "from_user": from_user,
        "to_user": to_user,
        "membership_created": gs_result["membership_created"],
        "wal_event": "appended",
    }


async def handle_set_legal_hold(
    global_store: GlobalStore,
    tenant_id: str,
    held_by: str,
    reason: str,
) -> dict[str, Any]:
    """Place a legal hold on a tenant.

    Legal holds are global metadata (not per-tenant WAL data).
    The GDPR deletion worker must check ``is_under_legal_hold``
    before processing deletions.
    """
    return await global_store.set_legal_hold_record(tenant_id, held_by, reason)


async def handle_remove_legal_hold(
    global_store: GlobalStore,
    tenant_id: str,
    held_by: str,
) -> dict[str, Any]:
    """Remove a specific legal hold from a tenant."""
    removed = await global_store.remove_legal_hold(tenant_id, held_by)
    return {"tenant_id": tenant_id, "held_by": held_by, "removed": removed}


async def handle_revoke_user_access(
    global_store: GlobalStore,
    wal: WalStream,
    tenant_id: str,
    user_id: str,
    actor: str,
) -> dict[str, Any]:
    """Immediately terminate a user's access to a tenant.

    Global membership removal is a direct write. Per-tenant ACL
    revocation is appended to the WAL.
    """
    gs_result = await global_store.revoke_user_access(tenant_id, user_id)

    import json

    event = {
        "tenant_id": tenant_id,
        "actor": actor,
        "idempotency_key": f"admin-revoke-{uuid.uuid4()}",
        "ts_ms": int(time.time() * 1000),
        "ops": [
            {
                "op": "admin_revoke_access",
                "user_id": user_id,
            }
        ],
    }
    await wal.append(key=tenant_id, value=json.dumps(event).encode())

    return {
        "tenant_id": tenant_id,
        "user_id": user_id,
        "membership_removed": gs_result["membership_removed"],
        "shared_removed": gs_result["shared_removed"],
        "wal_event": "appended",
    }
