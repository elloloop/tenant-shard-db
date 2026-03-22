"""
Multi-node tenant sharding for EntDB.

Determines which tenants this node is responsible for.
When ASSIGNED_TENANTS is empty, the node accepts all tenants
(single-node backward compatibility).

Env vars:
    NODE_ID: Unique identifier for this node (e.g. "node-a")
    ASSIGNED_TENANTS: Comma-separated tenant IDs (e.g. "college-0,college-1")
    TENANT_REGISTRY: Optional JSON mapping tenant_id -> node_id for routing hints

Invariants:
    - Empty ASSIGNED_TENANTS = single-node mode (all tenants accepted)
    - is_mine() is O(1) — used in the hot path for every event
    - Sharding config is immutable after startup

How to change safely:
    - Always preserve backward compatibility with empty ASSIGNED_TENANTS
    - Test with both single-node and multi-node configs
"""

from __future__ import annotations

import json
import logging
import os
from dataclasses import dataclass

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class ShardingConfig:
    """Node identity and tenant assignment.

    Attributes:
        node_id: Unique identifier for this node
        assigned_tenants: Set of tenant IDs this node owns (empty = all)
        tenant_registry: Mapping of tenant_id -> node_id for routing hints
    """

    node_id: str = "default"
    assigned_tenants: frozenset[str] = frozenset()
    tenant_registry: dict[str, str] | None = None

    @classmethod
    def from_env(cls) -> ShardingConfig:
        """Load sharding config from environment variables."""
        node_id = os.getenv("NODE_ID", "default")

        tenants_str = os.getenv("ASSIGNED_TENANTS", "")
        assigned = frozenset(t.strip() for t in tenants_str.split(",") if t.strip())

        # Optional tenant registry for routing hints
        registry_str = os.getenv("TENANT_REGISTRY", "")
        registry = None
        if registry_str:
            try:
                registry = json.loads(registry_str)
            except json.JSONDecodeError:
                logger.warning("Invalid TENANT_REGISTRY JSON, ignoring")

        config = cls(
            node_id=node_id,
            assigned_tenants=assigned,
            tenant_registry=registry,
        )

        if assigned:
            logger.info(
                "Multi-node sharding enabled",
                extra={
                    "node_id": node_id,
                    "assigned_tenants": sorted(assigned),
                    "tenant_count": len(assigned),
                },
            )
        else:
            logger.info("Single-node mode (no tenant assignment)")

        return config

    def is_mine(self, tenant_id: str) -> bool:
        """Check if this node owns the given tenant.

        Returns True if:
        - assigned_tenants is empty (single-node mode, accepts all)
        - tenant_id is in assigned_tenants

        This is called for every event in the applier hot path,
        so it must be O(1).
        """
        if not self.assigned_tenants:
            return True
        return tenant_id in self.assigned_tenants

    def get_owner(self, tenant_id: str) -> str | None:
        """Get the node_id that owns a tenant (for routing hints).

        Returns None if no registry is configured or tenant not found.
        """
        if self.tenant_registry:
            return self.tenant_registry.get(tenant_id)
        return None

    @property
    def is_multi_node(self) -> bool:
        """Whether this node is running in multi-node mode."""
        return len(self.assigned_tenants) > 0
