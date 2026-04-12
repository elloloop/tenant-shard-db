"""
Admin operation handler skeletons for EntDB (Issues #90, #91, #92).

These handlers coordinate global_store and canonical_store operations
for admin workflows.  They are designed to be called from the gRPC
servicer layer (``grpc_server.py``) or from a future REST/admin API.

Handler responsibilities:
    - Validate inputs
    - Coordinate global_store + canonical_store calls
    - Return structured results

NOTE on GDPR deletion worker (``gdpr_worker.py``):
    The deletion worker MUST call ``global_store.is_under_legal_hold(tenant_id)``
    before processing any deletion for a tenant.  If a tenant is under legal
    hold, the worker must skip that tenant and log a warning.  This check is
    NOT implemented here --- it belongs in the worker loop itself.
"""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from dbaas.entdb_server.apply.canonical_store import CanonicalStore
    from dbaas.entdb_server.global_store import GlobalStore

logger = logging.getLogger(__name__)


async def handle_transfer_user_content(
    global_store: GlobalStore,
    canonical_store: CanonicalStore,
    tenant_id: str,
    from_user: str,
    to_user: str,
    actor: str,
) -> dict[str, Any]:
    """Transfer all content owned by *from_user* to *to_user* in a tenant.

    Steps:
        1. ``global_store.transfer_user_content`` -- ensure *to_user* has
           tenant membership.
        2. ``canonical_store.transfer_user_content`` -- UPDATE nodes
           SET owner_actor = to_user WHERE owner_actor = from_user.

    Args:
        global_store: Global cross-tenant store
        canonical_store: Per-tenant canonical store
        tenant_id: Tenant identifier
        from_user: Current owner (e.g. ``user:alice``)
        to_user: New owner (e.g. ``user:bob``)
        actor: Admin performing the operation

    Returns:
        dict with ``transferred`` count and membership info.
    """
    gs_result = await global_store.transfer_user_content(tenant_id, from_user, to_user)
    cs_result = await canonical_store.transfer_user_content(
        tenant_id=tenant_id,
        from_user=from_user,
        to_user=to_user,
        actor=actor,
    )
    return {
        "tenant_id": tenant_id,
        "from_user": from_user,
        "to_user": to_user,
        "transferred": cs_result["transferred"],
        "membership_created": gs_result["membership_created"],
    }


async def handle_set_legal_hold(
    global_store: GlobalStore,
    tenant_id: str,
    held_by: str,
    reason: str,
) -> dict[str, Any]:
    """Place a legal hold on a tenant.

    Records the hold in the ``legal_holds`` table.  While any hold is
    active, ``is_under_legal_hold(tenant_id)`` returns True.

    The GDPR deletion worker (``gdpr_worker.py``) should check
    ``global_store.is_under_legal_hold(tenant_id)`` before processing
    deletions for the tenant.

    Args:
        global_store: Global cross-tenant store
        tenant_id: Tenant identifier
        held_by: Authority requesting the hold
        reason: Free-text reason

    Returns:
        dict with hold fields.
    """
    return await global_store.set_legal_hold_record(tenant_id, held_by, reason)


async def handle_remove_legal_hold(
    global_store: GlobalStore,
    tenant_id: str,
    held_by: str,
) -> dict[str, Any]:
    """Remove a specific legal hold from a tenant.

    Args:
        global_store: Global cross-tenant store
        tenant_id: Tenant identifier
        held_by: Authority that placed the hold

    Returns:
        dict with ``removed`` bool.
    """
    removed = await global_store.remove_legal_hold(tenant_id, held_by)
    return {"tenant_id": tenant_id, "held_by": held_by, "removed": removed}


async def handle_revoke_user_access(
    global_store: GlobalStore,
    canonical_store: CanonicalStore,
    tenant_id: str,
    user_id: str,
    actor: str,
) -> dict[str, Any]:
    """Immediately terminate a user's access to a tenant.

    Steps:
        1. ``global_store.revoke_user_access`` -- remove from
           tenant_members and clear shared_index entries.
        2. ``canonical_store.revoke_user_access`` -- DELETE/UPDATE ACL
           entries where grantee = user_id.

    Args:
        global_store: Global cross-tenant store
        canonical_store: Per-tenant canonical store
        tenant_id: Tenant identifier
        user_id: User to revoke (e.g. ``user:bob``)
        actor: Admin performing the operation

    Returns:
        dict with revocation counts.
    """
    gs_result = await global_store.revoke_user_access(tenant_id, user_id)
    cs_result = await canonical_store.revoke_user_access(
        tenant_id=tenant_id,
        user_id=user_id,
        actor=actor,
    )
    return {
        "tenant_id": tenant_id,
        "user_id": user_id,
        "membership_removed": gs_result["membership_removed"],
        "shared_removed": gs_result["shared_removed"],
        "revoked_grants": cs_result["revoked_grants"],
        "revoked_groups": cs_result["revoked_groups"],
    }
