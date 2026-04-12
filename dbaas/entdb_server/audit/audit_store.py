"""Tamper-evident, hash-chained audit log store for EntDB.

Each tenant's SQLite database contains an ``audit_log`` table whose entries
form a hash chain: every entry stores ``prev_hash``, the SHA-256 digest of
the preceding entry's immutable fields.  Tampering with (or deleting) any
row is detectable via ``verify_chain``.

This module provides ``AuditStore`` -- a thin, focused facade over the
per-tenant audit capabilities already present in
:class:`~dbaas.entdb_server.apply.canonical_store.CanonicalStore`.  It adds
higher-level helpers (JSON export, typed entry dicts) without duplicating
the underlying storage logic.

Invariants
----------
* One ``AuditStore`` instance wraps exactly one ``CanonicalStore``.
* The hash chain is computed as:
  ``sha256(event_id : prev_hash : actor_id : action : target_type : target_id : created_at)``
* The first entry in every chain has ``prev_hash = "genesis"``.
* ``verify_chain`` walks the entire chain and recomputes every hash.

How to change safely
--------------------
* Never alter the hash computation without a migration for existing rows.
* New columns may be added to ``audit_log`` but must **not** be included in
  the hash unless a chain-migration is performed.
"""

from __future__ import annotations

import json
import logging
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from ..apply.canonical_store import CanonicalStore

logger = logging.getLogger(__name__)


class AuditStore:
    """Facade for reading and writing tamper-evident audit entries.

    Parameters
    ----------
    canonical_store:
        The ``CanonicalStore`` instance that owns the underlying SQLite
        databases.

    Example
    -------
    >>> store = AuditStore(canonical_store)
    >>> await store.append(
    ...     tenant_id="t1",
    ...     actor="user:alice",
    ...     action="node.create",
    ...     resource_type="node",
    ...     resource_id="n-123",
    ... )
    """

    def __init__(self, canonical_store: CanonicalStore) -> None:
        self._store = canonical_store

    # ── write ────────────────────────────────────────────────────────────

    async def append(
        self,
        tenant_id: str,
        actor: str,
        action: str,
        resource_type: str,
        resource_id: str,
        details: dict[str, Any] | None = None,
        ip_address: str | None = None,
        user_agent: str | None = None,
    ) -> str:
        """Append a hash-chained audit entry.

        Parameters
        ----------
        tenant_id:
            Tenant whose audit log receives the entry.
        actor:
            Identity of the caller (e.g. ``"user:alice"``).
        action:
            Dot-separated action verb (e.g. ``"node.create"``).
        resource_type:
            Kind of resource acted upon (``"node"``, ``"edge"``, etc.).
        resource_id:
            Identifier of the affected resource.
        details:
            Optional free-form metadata dict (serialised as JSON).
        ip_address:
            Optional client IP address.
        user_agent:
            Optional client user-agent string.

        Returns
        -------
        str
            The ``event_id`` (UUID) of the newly created entry.
        """
        metadata_json: str | None = None
        if details is not None:
            metadata_json = json.dumps(details)

        return await self._store.append_audit(
            tenant_id=tenant_id,
            actor_id=actor,
            action=action,
            target_type=resource_type,
            target_id=resource_id,
            ip_address=ip_address,
            user_agent=user_agent,
            metadata=metadata_json,
        )

    # ── read / query ─────────────────────────────────────────────────────

    async def get_audit_log(
        self,
        tenant_id: str,
        actor: str | None = None,
        action: str | None = None,
        since: int | None = None,
        limit: int = 100,
    ) -> list[dict[str, Any]]:
        """Query the audit log with optional filters.

        Parameters
        ----------
        tenant_id:
            Tenant whose log to query.
        actor:
            If given, only entries by this actor.
        action:
            If given, only entries with this action.
        since:
            If given, only entries created after this Unix-ms timestamp.
        limit:
            Maximum number of entries to return (default 100).

        Returns
        -------
        list[dict]
            Entries ordered newest-first.  Each dict contains:
            ``seq``, ``timestamp``, ``actor``, ``action``,
            ``resource_type``, ``resource_id``, ``details_json``,
            ``prev_hash``, ``entry_hash``.
        """
        raw = await self._store.get_audit_log(
            tenant_id=tenant_id,
            actor_id=actor,
            action=action,
            after=since,
            limit=limit,
        )
        return [_normalise_entry(e) for e in raw]

    # ── chain verification ───────────────────────────────────────────────

    async def verify_chain(self, tenant_id: str) -> bool:
        """Verify the full hash chain of a tenant's audit log.

        Returns ``True`` when the chain is intact, ``False`` if any entry
        has been tampered with or deleted.
        """
        valid, _msg = await self._store.verify_audit_chain(tenant_id)
        return valid

    async def verify_chain_detail(self, tenant_id: str) -> tuple[bool, str]:
        """Like :meth:`verify_chain` but also returns a diagnostic message."""
        return await self._store.verify_audit_chain(tenant_id)

    # ── export ───────────────────────────────────────────────────────────

    async def export_audit_log(
        self,
        tenant_id: str,
        format: str = "json",  # noqa: A002 -- matches spec
    ) -> str:
        """Export the full audit chain as a serialised string.

        Parameters
        ----------
        tenant_id:
            Tenant whose log to export.
        format:
            Output format.  Currently only ``"json"`` is supported.

        Returns
        -------
        str
            The serialised audit log.

        Raises
        ------
        ValueError
            If an unsupported format is requested.
        """
        if format != "json":
            raise ValueError(f"Unsupported export format: {format!r}")

        raw = await self._store.export_audit_log(tenant_id)
        entries = [_normalise_entry(e) for e in raw]
        return json.dumps(entries, indent=2)


# ── helpers ──────────────────────────────────────────────────────────────


def _normalise_entry(raw: dict[str, Any]) -> dict[str, Any]:
    """Convert a raw ``canonical_store`` audit row into the public schema.

    The public schema uses friendlier names (``actor`` instead of
    ``actor_id``, ``resource_type`` instead of ``target_type``, etc.)
    and includes a computed ``entry_hash`` for convenience.
    """
    from ..apply.canonical_store import _compute_entry_hash

    entry_hash = _compute_entry_hash(
        raw["event_id"],
        raw["prev_hash"],
        raw["actor_id"],
        raw["action"],
        raw["target_type"],
        raw["target_id"],
        raw["created_at"],
    )

    return {
        "seq": raw["event_id"],
        "timestamp": raw["created_at"],
        "actor": raw["actor_id"],
        "action": raw["action"],
        "resource_type": raw["target_type"],
        "resource_id": raw["target_id"],
        "details_json": raw.get("metadata"),
        "prev_hash": raw["prev_hash"],
        "entry_hash": entry_hash,
    }
