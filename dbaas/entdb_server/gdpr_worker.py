"""
GDPR deletion worker for EntDB.

This module runs the background deletion pipeline for users who have
been queued for erasure via :class:`GlobalStore.queue_deletion`.

Lifecycle:
    1. A user calls ``delete_user(user_id)``; the server queues a
       deletion entry with a 30-day grace period in
       ``global_store.deletion_queue``.
    2. Every ``poll_interval`` seconds the worker selects entries whose
       ``execute_at`` has passed.
    3. For each such user the worker:
         - enumerates tenants the user belongs to,
         - if the user is the sole member of a tenant (their personal
           tenant), deletes the entire tenant SQLite file,
         - otherwise runs
           :meth:`CanonicalStore.anonymize_user_in_tenant` to apply the
           per-type data_policy rules,
         - removes the user's entries from ``tenant_members``,
           ``shared_index``, and global group memberships,
         - marks the deletion queue entry as ``completed``.

Invariants:
    - The worker is idempotent: running ``run_once`` twice on an
      already-processed user has no side effect.
    - The worker does not delete the ``user_registry`` row directly;
      instead it marks the user as ``deleted`` via ``set_user_status``
      so downstream systems can still resolve the id when rendering
      audit logs.

How to change safely:
    - Keep ``run_once`` async and side-effect free on exceptions.
    - Do not run destructive ops outside the per-user transaction.
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any

logger = logging.getLogger(__name__)


class GdprDeletionWorker:
    """Polls the deletion queue and applies scheduled erasures.

    Attributes:
        global_store: The cross-tenant :class:`GlobalStore`.
        canonical_store: The per-tenant :class:`CanonicalStore`.
        schema_registry: :class:`SchemaRegistry` used to look up
            data_policy and pii_fields.
        poll_interval: Seconds between polls for the background loop.
        salt: Deterministic salt passed to the anonymization routine.

    Example:
        >>> worker = GdprDeletionWorker(global_store, canonical_store, registry)
        >>> results = await worker.run_once()
        >>> print(results["processed"])
    """

    def __init__(
        self,
        global_store: Any,
        canonical_store: Any,
        schema_registry: Any,
        *,
        poll_interval: float = 60.0,
        salt: str = "entdb-gdpr-v1",
    ) -> None:
        self.global_store = global_store
        self.canonical_store = canonical_store
        self.schema_registry = schema_registry
        self.poll_interval = poll_interval
        self.salt = salt
        self._stopped = asyncio.Event()

    async def run_forever(self) -> None:
        """Run the deletion loop until :meth:`stop` is called.

        The loop sleeps for :attr:`poll_interval` seconds between
        iterations. Exceptions are logged and the loop continues.
        """
        logger.info(
            "GDPR deletion worker starting (poll_interval=%s)", self.poll_interval
        )
        while not self._stopped.is_set():
            try:
                await self.run_once()
            except Exception:  # pragma: no cover - defensive
                logger.exception("GDPR deletion worker iteration failed")
            try:
                await asyncio.wait_for(
                    self._stopped.wait(), timeout=self.poll_interval
                )
            except asyncio.TimeoutError:
                pass

    def stop(self) -> None:
        """Signal :meth:`run_forever` to exit after the current iteration."""
        self._stopped.set()

    async def run_once(self, now: int | None = None) -> dict:
        """Process all deletions whose grace period has passed.

        Args:
            now: Optional reference timestamp for the "grace period has
                passed" check. Useful in tests to advance simulated time.

        Returns:
            Summary dict with a list of per-user ``results``. Each result
            contains either the anonymization counts per tenant or an
            ``error`` field when processing failed.
        """
        entries = await self.global_store.get_executable_deletions(now=now)
        results = []
        for entry in entries:
            user_id = entry["user_id"]
            try:
                result = await self._process_user(user_id)
                results.append({"user_id": user_id, **result})
            except Exception as e:  # pragma: no cover - defensive
                logger.exception("Failed to process deletion for %s", user_id)
                results.append({"user_id": user_id, "error": str(e)})
        return {"processed": len(results), "results": results}

    async def _process_user(self, user_id: str) -> dict:
        """Apply the deletion pipeline for a single user."""
        per_tenant: list[dict] = []
        deleted_tenants: list[str] = []
        memberships = await self.global_store.get_user_tenants(user_id)

        for m in memberships:
            tenant_id = m["tenant_id"]
            members = await self.global_store.get_members(tenant_id)
            is_sole = len(members) == 1 and members[0]["user_id"] == user_id
            if is_sole:
                # Personal tenant: drop the entire SQLite file
                existed = self.canonical_store.delete_tenant_database(tenant_id)
                try:
                    await self.global_store.set_tenant_status(tenant_id, "deleted")
                except Exception:  # pragma: no cover
                    pass
                deleted_tenants.append(tenant_id)
                per_tenant.append(
                    {
                        "tenant_id": tenant_id,
                        "mode": "deleted_tenant",
                        "db_removed": existed,
                    }
                )
            else:
                counts = await self.canonical_store.anonymize_user_in_tenant(
                    tenant_id,
                    user_id,
                    self.schema_registry,
                    salt=self.salt,
                )
                per_tenant.append(
                    {"tenant_id": tenant_id, "mode": "anonymized", **counts}
                )

        # Global cleanup
        await self.global_store.remove_all_memberships_for_user(user_id)
        await self.global_store.remove_all_shared_for_user(user_id)
        await self.global_store.set_user_status(user_id, "deleted")
        await self.global_store.mark_deletion_completed(user_id)

        return {
            "tenants_processed": per_tenant,
            "deleted_tenants": deleted_tenants,
        }
