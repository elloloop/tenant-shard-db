"""
Schema registry for EntDB SDK.

This module provides a local schema registry for:
- Registering node and edge types
- Type lookup by ID or name
- Schema fingerprinting

The registry is frozen at startup to prevent runtime modifications.

Example:
    >>> from entdb_sdk import get_registry, NodeTypeDef, field
    >>>
    >>> User = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
    >>> registry = get_registry()
    >>> registry.register_node_type(User)
"""

from __future__ import annotations

import hashlib
import json
import threading
from collections.abc import Iterator

from .schema import EdgeTypeDef, NodeTypeDef

# Global registry
_global_registry: SchemaRegistry | None = None
_registry_lock = threading.Lock()


class RegistryFrozenError(Exception):
    """Registry is frozen and cannot be modified."""

    pass


class DuplicateRegistrationError(Exception):
    """Type or edge with this ID is already registered."""

    pass


class SchemaRegistry:
    """Local schema registry for type management.

    The registry stores all node and edge type definitions and provides
    lookup by ID or name. It can be frozen to prevent modifications.

    Example:
        >>> registry = SchemaRegistry()
        >>> registry.register_node_type(User)
        >>> registry.register_edge_type(AssignedTo)
        >>> registry.freeze()
    """

    def __init__(self) -> None:
        """Initialize empty registry."""
        self._node_types: dict[int, NodeTypeDef] = {}
        self._edge_types: dict[int, EdgeTypeDef] = {}
        self._node_types_by_name: dict[str, NodeTypeDef] = {}
        self._edge_types_by_name: dict[str, EdgeTypeDef] = {}
        self._frozen = False
        self._fingerprint: str | None = None
        self._lock = threading.Lock()

    @property
    def frozen(self) -> bool:
        """Whether registry is frozen."""
        return self._frozen

    @property
    def fingerprint(self) -> str | None:
        """Schema fingerprint (available after freeze)."""
        return self._fingerprint

    def register_node_type(self, node_type: NodeTypeDef) -> None:
        """Register a node type.

        Args:
            node_type: NodeTypeDef to register

        Raises:
            RegistryFrozenError: If registry is frozen
            DuplicateRegistrationError: If type_id already registered
        """
        with self._lock:
            if self._frozen:
                raise RegistryFrozenError("Cannot register: registry is frozen")

            if node_type.type_id in self._node_types:
                existing = self._node_types[node_type.type_id]
                raise DuplicateRegistrationError(
                    f"type_id {node_type.type_id} already registered as '{existing.name}'"
                )

            self._node_types[node_type.type_id] = node_type
            self._node_types_by_name[node_type.name] = node_type

    def register_edge_type(self, edge_type: EdgeTypeDef) -> None:
        """Register an edge type.

        Args:
            edge_type: EdgeTypeDef to register

        Raises:
            RegistryFrozenError: If registry is frozen
            DuplicateRegistrationError: If edge_id already registered
        """
        with self._lock:
            if self._frozen:
                raise RegistryFrozenError("Cannot register: registry is frozen")

            if edge_type.edge_id in self._edge_types:
                existing = self._edge_types[edge_type.edge_id]
                raise DuplicateRegistrationError(
                    f"edge_id {edge_type.edge_id} already registered as '{existing.name}'"
                )

            self._edge_types[edge_type.edge_id] = edge_type
            self._edge_types_by_name[edge_type.name] = edge_type

    def get_node_type(self, type_id_or_name: int | str) -> NodeTypeDef | None:
        """Get node type by ID or name."""
        if isinstance(type_id_or_name, int):
            return self._node_types.get(type_id_or_name)
        return self._node_types_by_name.get(type_id_or_name)

    def get_edge_type(self, edge_id_or_name: int | str) -> EdgeTypeDef | None:
        """Get edge type by ID or name."""
        if isinstance(edge_id_or_name, int):
            return self._edge_types.get(edge_id_or_name)
        return self._edge_types_by_name.get(edge_id_or_name)

    def node_types(self) -> Iterator[NodeTypeDef]:
        """Iterate over all node types."""
        yield from self._node_types.values()

    def edge_types(self) -> Iterator[EdgeTypeDef]:
        """Iterate over all edge types."""
        yield from self._edge_types.values()

    def freeze(self) -> str:
        """Freeze registry and compute fingerprint.

        Returns:
            Schema fingerprint

        Raises:
            RegistryFrozenError: If already frozen
        """
        with self._lock:
            if self._frozen:
                raise RegistryFrozenError("Registry is already frozen")

            self._fingerprint = self._compute_fingerprint()
            self._frozen = True
            return self._fingerprint

    def _compute_fingerprint(self) -> str:
        """Compute SHA-256 fingerprint."""
        schema_dict = self.to_dict()
        canonical = json.dumps(schema_dict, sort_keys=True, separators=(",", ":"))
        hash_bytes = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
        return f"sha256:{hash_bytes}"

    def to_dict(self) -> dict:
        """Convert to dictionary."""
        return {
            "node_types": [
                self._node_types[tid].to_dict() for tid in sorted(self._node_types.keys())
            ],
            "edge_types": [
                self._edge_types[eid].to_dict() for eid in sorted(self._edge_types.keys())
            ],
        }

    def to_json(self, indent: int | None = 2) -> str:
        """Convert to JSON string."""
        return json.dumps(self.to_dict(), indent=indent, sort_keys=True)


def get_registry() -> SchemaRegistry:
    """Get the global schema registry."""
    global _global_registry
    with _registry_lock:
        if _global_registry is None:
            _global_registry = SchemaRegistry()
        return _global_registry


def register_node_type(node_type: NodeTypeDef) -> None:
    """Register a node type in the global registry."""
    get_registry().register_node_type(node_type)


def register_edge_type(edge_type: EdgeTypeDef) -> None:
    """Register an edge type in the global registry."""
    get_registry().register_edge_type(edge_type)


def reset_registry() -> None:
    """Reset the global registry (for testing only)."""
    global _global_registry
    with _registry_lock:
        _global_registry = None
