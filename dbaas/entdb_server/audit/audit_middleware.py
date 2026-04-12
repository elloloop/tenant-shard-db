"""Audit middleware for gRPC event logging.

Provides a simple function that gRPC handlers call to record an audit
event.  This keeps audit logic out of ``grpc_server.py`` and makes it
easy to wire into any handler with a single call.

Supported actions
-----------------
Node operations : ``node.create``, ``node.update``, ``node.delete``
Edge operations : ``edge.create``, ``edge.delete``
ACL operations  : ``acl.grant``, ``acl.revoke``
User lifecycle  : ``user.delete``, ``user.freeze``, ``user.export``
Tenant mgmt     : ``tenant.create``, ``tenant.delete``

Usage
-----
::

    from dbaas.entdb_server.audit import log_audit_event

    async def SomeRPC(self, request, context):
        # ... perform the operation ...
        await log_audit_event(
            audit_store,
            tenant_id=request.tenant_id,
            actor=request.actor,
            action="node.create",
            resource_type="node",
            resource_id=new_node_id,
            details={"type_id": request.type_id},
        )
"""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from .audit_store import AuditStore

logger = logging.getLogger(__name__)

# Actions that the audit system recognises.  This set is informational --
# unknown actions are still logged, but a warning is emitted to aid
# debugging typos in caller code.
KNOWN_ACTIONS: frozenset[str] = frozenset(
    {
        "node.create",
        "node.update",
        "node.delete",
        "edge.create",
        "edge.delete",
        "acl.grant",
        "acl.revoke",
        "user.delete",
        "user.freeze",
        "user.export",
        "tenant.create",
        "tenant.delete",
    }
)


async def log_audit_event(
    audit_store: AuditStore,
    tenant_id: str,
    actor: str,
    action: str,
    resource_type: str,
    resource_id: str,
    details: dict[str, Any] | None = None,
) -> str | None:
    """Record an audit event, swallowing errors so callers never fail.

    Parameters
    ----------
    audit_store:
        The ``AuditStore`` to write to.
    tenant_id:
        Tenant context.
    actor:
        Identity of the caller.
    action:
        Dot-separated action (e.g. ``"node.create"``).
    resource_type:
        Kind of resource (``"node"``, ``"edge"``, ``"acl"``, etc.).
    resource_id:
        Identifier of the affected resource.
    details:
        Optional dict of additional metadata.

    Returns
    -------
    str | None
        The ``event_id`` on success, or ``None`` if the write failed.
    """
    if action not in KNOWN_ACTIONS:
        logger.warning("Unknown audit action %r -- logging anyway", action)

    try:
        event_id = await audit_store.append(
            tenant_id=tenant_id,
            actor=actor,
            action=action,
            resource_type=resource_type,
            resource_id=resource_id,
            details=details,
        )
        logger.debug(
            "Audit event %s: %s %s/%s by %s",
            event_id,
            action,
            resource_type,
            resource_id,
            actor,
        )
        return event_id
    except Exception:
        logger.exception(
            "Failed to write audit event: %s %s/%s by %s",
            action,
            resource_type,
            resource_id,
            actor,
        )
        return None
