"""
Schema module for EntDB.

This module provides the type system for nodes and edges, including:
- Type definitions (NodeTypeDef, EdgeTypeDef, FieldDef)
- Schema registry for type management
- Compatibility checking for schema evolution

Invariants:
    - type_id, edge_id, field_id are immutable once assigned
    - IDs are never reused after deprecation
    - Field types cannot change for existing field_ids
    - All type definitions must be registered before server start

How to change safely:
    - Add new types with new IDs
    - Add new fields to existing types with new field_ids
    - Deprecate (never delete) fields or types
    - Use schema CLI to verify compatibility before deployment
"""

from .compat import (
    ChangeKind,
    CompatibilityError,
    SchemaChange,
    check_compatibility,
    generate_fingerprint,
)
from .registry import SchemaRegistry, freeze_registry, get_registry
from .types import (
    AclEntry,
    AclPermission,
    EdgeTypeDef,
    FieldDef,
    FieldKind,
    NodeTypeDef,
    field,
)

__all__ = [
    # Types
    "FieldDef",
    "NodeTypeDef",
    "EdgeTypeDef",
    "FieldKind",
    "field",
    "AclEntry",
    "AclPermission",
    # Registry
    "SchemaRegistry",
    "get_registry",
    "freeze_registry",
    # Compatibility
    "SchemaChange",
    "ChangeKind",
    "CompatibilityError",
    "check_compatibility",
    "generate_fingerprint",
]
