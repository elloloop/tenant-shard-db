"""
Schema Registry for EntDB.

The SchemaRegistry is the central authority for all type definitions.
It provides:
- Registration of node and edge types
- Lookup by ID or name
- Schema fingerprinting for consistency checks
- Freeze mechanism to prevent runtime modifications

Invariants:
    - Registry is mutable during startup, frozen before serving
    - Once frozen, no new types can be registered
    - type_id and edge_id must be globally unique
    - Fingerprint changes when schema changes (for compatibility checking)

How to change safely:
    - Register all types before calling freeze_registry()
    - Use schema CLI to verify compatibility before deployment
    - Never modify registered types after freeze

Example:
    >>> from entdb_server.schema import SchemaRegistry, NodeTypeDef, field
    >>> registry = SchemaRegistry()
    >>> User = NodeTypeDef(type_id=1, name="User", fields=(field(1, "email", "str"),))
    >>> registry.register_node_type(User)
    >>> registry.freeze()
    >>> registry.get_node_type(1)
    NodeTypeDef(type_id=1, name='User', ...)
"""

from __future__ import annotations

import hashlib
import json
import threading
from typing import Dict, Iterator, Optional, Union
import logging

from .types import NodeTypeDef, EdgeTypeDef

logger = logging.getLogger(__name__)

# Global registry instance
_global_registry: Optional[SchemaRegistry] = None
_registry_lock = threading.Lock()


class RegistryFrozenError(Exception):
    """Raised when attempting to modify a frozen registry."""
    pass


class DuplicateRegistrationError(Exception):
    """Raised when attempting to register a duplicate type/edge ID."""
    pass


class SchemaRegistry:
    """Central registry for all node and edge type definitions.

    The registry maintains a mapping of type IDs to definitions and
    provides methods for lookup, registration, and schema fingerprinting.

    Thread-safety:
        - Registration is thread-safe (uses internal lock)
        - Lookups after freeze are lock-free
        - Freeze is atomic and irreversible

    Attributes:
        frozen: Whether the registry is frozen (immutable)
        fingerprint: SHA-256 hash of the schema (computed on freeze)

    Example:
        >>> registry = SchemaRegistry()
        >>> registry.register_node_type(User)
        >>> registry.register_node_type(Task)
        >>> registry.register_edge_type(AssignedTo)
        >>> registry.freeze()
        >>> print(registry.fingerprint)
        'sha256:abc123...'
    """

    def __init__(self) -> None:
        """Initialize an empty, mutable registry."""
        self._node_types: Dict[int, NodeTypeDef] = {}
        self._edge_types: Dict[int, EdgeTypeDef] = {}
        self._node_types_by_name: Dict[str, NodeTypeDef] = {}
        self._edge_types_by_name: Dict[str, EdgeTypeDef] = {}
        self._frozen = False
        self._fingerprint: Optional[str] = None
        self._lock = threading.Lock()

    @property
    def frozen(self) -> bool:
        """Whether the registry is frozen."""
        return self._frozen

    @property
    def fingerprint(self) -> Optional[str]:
        """Schema fingerprint (available after freeze)."""
        return self._fingerprint

    def register_node_type(self, node_type: NodeTypeDef) -> None:
        """Register a node type definition.

        Args:
            node_type: The node type to register

        Raises:
            RegistryFrozenError: If registry is frozen
            DuplicateRegistrationError: If type_id is already registered

        Example:
            >>> registry.register_node_type(User)
        """
        with self._lock:
            if self._frozen:
                raise RegistryFrozenError(
                    f"Cannot register node type '{node_type.name}': registry is frozen"
                )

            if node_type.type_id in self._node_types:
                existing = self._node_types[node_type.type_id]
                raise DuplicateRegistrationError(
                    f"type_id {node_type.type_id} already registered as '{existing.name}'"
                )

            if node_type.name in self._node_types_by_name:
                existing = self._node_types_by_name[node_type.name]
                raise DuplicateRegistrationError(
                    f"Node type name '{node_type.name}' already registered with type_id {existing.type_id}"
                )

            self._node_types[node_type.type_id] = node_type
            self._node_types_by_name[node_type.name] = node_type
            logger.debug(
                f"Registered node type: {node_type.name} (type_id={node_type.type_id})"
            )

    def register_edge_type(self, edge_type: EdgeTypeDef) -> None:
        """Register an edge type definition.

        Args:
            edge_type: The edge type to register

        Raises:
            RegistryFrozenError: If registry is frozen
            DuplicateRegistrationError: If edge_id is already registered

        Example:
            >>> registry.register_edge_type(AssignedTo)
        """
        with self._lock:
            if self._frozen:
                raise RegistryFrozenError(
                    f"Cannot register edge type '{edge_type.name}': registry is frozen"
                )

            if edge_type.edge_id in self._edge_types:
                existing = self._edge_types[edge_type.edge_id]
                raise DuplicateRegistrationError(
                    f"edge_id {edge_type.edge_id} already registered as '{existing.name}'"
                )

            if edge_type.name in self._edge_types_by_name:
                existing = self._edge_types_by_name[edge_type.name]
                raise DuplicateRegistrationError(
                    f"Edge type name '{edge_type.name}' already registered with edge_id {existing.edge_id}"
                )

            # Validate from_type and to_type if they are NodeTypeDefs
            from_id = edge_type.from_type_id
            to_id = edge_type.to_type_id

            if from_id not in self._node_types:
                logger.warning(
                    f"Edge type '{edge_type.name}' references unregistered from_type_id {from_id}"
                )

            if to_id not in self._node_types:
                logger.warning(
                    f"Edge type '{edge_type.name}' references unregistered to_type_id {to_id}"
                )

            self._edge_types[edge_type.edge_id] = edge_type
            self._edge_types_by_name[edge_type.name] = edge_type
            logger.debug(
                f"Registered edge type: {edge_type.name} (edge_id={edge_type.edge_id})"
            )

    def get_node_type(self, type_id_or_name: Union[int, str]) -> Optional[NodeTypeDef]:
        """Get a node type by ID or name.

        Args:
            type_id_or_name: type_id (int) or name (str)

        Returns:
            NodeTypeDef if found, None otherwise

        Example:
            >>> user = registry.get_node_type(1)
            >>> user = registry.get_node_type("User")
        """
        if isinstance(type_id_or_name, int):
            return self._node_types.get(type_id_or_name)
        return self._node_types_by_name.get(type_id_or_name)

    def get_edge_type(self, edge_id_or_name: Union[int, str]) -> Optional[EdgeTypeDef]:
        """Get an edge type by ID or name.

        Args:
            edge_id_or_name: edge_id (int) or name (str)

        Returns:
            EdgeTypeDef if found, None otherwise
        """
        if isinstance(edge_id_or_name, int):
            return self._edge_types.get(edge_id_or_name)
        return self._edge_types_by_name.get(edge_id_or_name)

    def node_types(self) -> Iterator[NodeTypeDef]:
        """Iterate over all registered node types."""
        yield from self._node_types.values()

    def edge_types(self) -> Iterator[EdgeTypeDef]:
        """Iterate over all registered edge types."""
        yield from self._edge_types.values()

    def freeze(self) -> str:
        """Freeze the registry and compute fingerprint.

        After freezing, no new types can be registered.
        The fingerprint is computed from the canonical schema representation.

        Returns:
            Schema fingerprint string

        Raises:
            RegistryFrozenError: If already frozen

        Example:
            >>> fingerprint = registry.freeze()
            >>> print(fingerprint)
            'sha256:abc123...'
        """
        with self._lock:
            if self._frozen:
                raise RegistryFrozenError("Registry is already frozen")

            self._fingerprint = self._compute_fingerprint()
            self._frozen = True
            logger.info(
                f"Schema registry frozen with {len(self._node_types)} node types, "
                f"{len(self._edge_types)} edge types, fingerprint={self._fingerprint}"
            )
            return self._fingerprint

    def _compute_fingerprint(self) -> str:
        """Compute SHA-256 fingerprint of the schema.

        The fingerprint is computed from a canonical JSON representation
        of all types, sorted by ID for determinism.

        Returns:
            Fingerprint string in format 'sha256:<hash>'
        """
        schema_dict = self.to_dict()
        # Sort for determinism
        canonical = json.dumps(schema_dict, sort_keys=True, separators=(',', ':'))
        hash_bytes = hashlib.sha256(canonical.encode('utf-8')).hexdigest()
        return f"sha256:{hash_bytes}"

    def to_dict(self) -> dict:
        """Convert registry to dictionary representation.

        Returns:
            Dictionary with 'node_types' and 'edge_types' lists,
            sorted by ID for determinism.
        """
        return {
            "node_types": [
                self._node_types[tid].to_dict()
                for tid in sorted(self._node_types.keys())
            ],
            "edge_types": [
                self._edge_types[eid].to_dict()
                for eid in sorted(self._edge_types.keys())
            ],
        }

    def to_json(self, indent: Optional[int] = 2) -> str:
        """Convert registry to JSON string.

        Args:
            indent: JSON indentation (None for compact)

        Returns:
            JSON string representation
        """
        return json.dumps(self.to_dict(), indent=indent, sort_keys=True)

    @classmethod
    def from_dict(cls, data: dict) -> SchemaRegistry:
        """Create registry from dictionary representation.

        Args:
            data: Dictionary with 'node_types' and 'edge_types'

        Returns:
            New SchemaRegistry with types registered (not frozen)
        """
        registry = cls()
        for node_data in data.get("node_types", []):
            registry.register_node_type(NodeTypeDef.from_dict(node_data))
        for edge_data in data.get("edge_types", []):
            registry.register_edge_type(EdgeTypeDef.from_dict(edge_data))
        return registry

    @classmethod
    def from_json(cls, json_str: str) -> SchemaRegistry:
        """Create registry from JSON string.

        Args:
            json_str: JSON string representation

        Returns:
            New SchemaRegistry (not frozen)
        """
        data = json.loads(json_str)
        return cls.from_dict(data)

    def validate_all(self) -> list[str]:
        """Validate all registered types for consistency.

        Returns:
            List of validation errors (empty if valid)
        """
        errors = []

        # Check edge types reference valid node types
        for edge in self._edge_types.values():
            if edge.from_type_id not in self._node_types:
                errors.append(
                    f"Edge '{edge.name}' (edge_id={edge.edge_id}) references "
                    f"unknown from_type_id {edge.from_type_id}"
                )
            if edge.to_type_id not in self._node_types:
                errors.append(
                    f"Edge '{edge.name}' (edge_id={edge.edge_id}) references "
                    f"unknown to_type_id {edge.to_type_id}"
                )

        # Check field references in node types
        for node in self._node_types.values():
            for field in node.fields:
                if field.ref_type_id is not None:
                    if field.ref_type_id not in self._node_types:
                        errors.append(
                            f"Field '{field.name}' in node type '{node.name}' "
                            f"references unknown type_id {field.ref_type_id}"
                        )

        return errors


def get_registry() -> SchemaRegistry:
    """Get the global schema registry.

    Creates a new registry if none exists.

    Returns:
        Global SchemaRegistry instance

    Example:
        >>> registry = get_registry()
        >>> registry.register_node_type(User)
    """
    global _global_registry
    with _registry_lock:
        if _global_registry is None:
            _global_registry = SchemaRegistry()
        return _global_registry


def freeze_registry() -> str:
    """Freeze the global registry.

    This should be called after all types are registered
    and before the server starts accepting requests.

    Returns:
        Schema fingerprint

    Raises:
        RegistryFrozenError: If already frozen
    """
    return get_registry().freeze()


def reset_registry() -> None:
    """Reset the global registry (for testing only).

    Warning: This is intended for test cleanup only.
    Never use in production code.
    """
    global _global_registry
    with _registry_lock:
        _global_registry = None
