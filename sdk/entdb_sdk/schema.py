"""
Schema types for EntDB SDK.

This module provides type definitions for the graph model:
- NodeTypeDef: Definition of a node type
- EdgeTypeDef: Definition of an edge type
- FieldDef: Individual field definition

These types are used to define the schema and enforce type safety.
IDs are stable and immutable once assigned.

Invariants:
    - type_id, edge_id, field_id never change after first release
    - Names can be renamed (ID is canonical)
    - Fields can be added but not removed
    - enum_values can only be appended

Example:
    >>> User = NodeTypeDef(
    ...     type_id=1,
    ...     name="User",
    ...     fields=(
    ...         field(1, "email", "str", required=True),
    ...         field(2, "name", "str"),
    ...     ),
    ... )
"""

from __future__ import annotations

from dataclasses import dataclass
from dataclasses import field as dataclass_field
from enum import Enum
from typing import Any


class FieldKind(Enum):
    """Supported field types."""

    STRING = "str"
    INTEGER = "int"
    FLOAT = "float"
    BOOLEAN = "bool"
    TIMESTAMP = "timestamp"
    JSON = "json"
    BYTES = "bytes"
    ENUM = "enum"
    REFERENCE = "ref"
    LIST_STRING = "list_str"
    LIST_INT = "list_int"
    LIST_REF = "list_ref"

    @classmethod
    def from_str(cls, value: str) -> FieldKind:
        """Convert string to FieldKind."""
        for kind in cls:
            if kind.value == value:
                return kind
        raise ValueError(f"Invalid field kind: {value}")


@dataclass(frozen=True)
class FieldDef:
    """Field definition within a node or edge type.

    Attributes:
        field_id: Stable numeric identifier (1-65535)
        name: Human-readable name
        kind: Data type
        required: Whether field is required
        default: Default value
        enum_values: Valid values for enum type
        ref_type_id: Target type for references
        indexed: Whether to create index
        searchable: Whether to include in FTS
        deprecated: Whether field is deprecated
        description: Documentation
    """

    field_id: int
    name: str
    kind: FieldKind
    required: bool = False
    default: Any = None
    enum_values: tuple[str, ...] | None = None
    ref_type_id: int | None = None
    indexed: bool = False
    searchable: bool = False
    deprecated: bool = False
    description: str = ""

    def __post_init__(self) -> None:
        """Validate field definition."""
        if self.field_id <= 0 or self.field_id > 65535:
            raise ValueError(f"field_id must be 1-65535, got {self.field_id}")
        if not self.name:
            raise ValueError("Field name cannot be empty")
        if self.kind == FieldKind.ENUM and not self.enum_values:
            raise ValueError(f"enum_values required for ENUM field '{self.name}'")

    def validate_value(self, value: Any) -> tuple[bool, str | None]:
        """Validate a value against this field."""
        if value is None:
            if self.required:
                return False, f"Field '{self.name}' is required"
            return True, None

        if self.kind == FieldKind.ENUM:
            if self.enum_values is not None and value not in self.enum_values:
                return False, f"Field '{self.name}' must be one of {self.enum_values}"

        return True, None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        result: dict[str, Any] = {
            "field_id": self.field_id,
            "name": self.name,
            "kind": self.kind.value,
        }
        if self.required:
            result["required"] = True
        if self.default is not None:
            result["default"] = self.default
        if self.enum_values:
            result["enum_values"] = list(self.enum_values)
        if self.ref_type_id is not None:
            result["ref_type_id"] = self.ref_type_id
        if self.indexed:
            result["indexed"] = True
        if self.searchable:
            result["searchable"] = True
        if self.deprecated:
            result["deprecated"] = True
        if self.description:
            result["description"] = self.description
        return result


def field(
    field_id: int,
    name: str,
    kind: str | FieldKind,
    *,
    required: bool = False,
    default: Any = None,
    enum_values: tuple[str, ...] | None = None,
    ref_type_id: int | None = None,
    indexed: bool = False,
    searchable: bool = False,
    deprecated: bool = False,
    description: str = "",
) -> FieldDef:
    """Convenience function to create a FieldDef.

    Args:
        field_id: Stable numeric identifier
        name: Human-readable name
        kind: Field type
        required: Whether required
        default: Default value
        enum_values: Valid enum values
        ref_type_id: Target type for references
        indexed: Create index
        searchable: Include in FTS
        deprecated: Deprecated flag
        description: Documentation

    Returns:
        FieldDef instance

    Example:
        >>> title = field(1, "title", "str", required=True)
        >>> status = field(2, "status", "enum", enum_values=("todo", "done"))
    """
    if isinstance(kind, str):
        kind = FieldKind.from_str(kind)
    return FieldDef(
        field_id=field_id,
        name=name,
        kind=kind,
        required=required,
        default=default,
        enum_values=enum_values,
        ref_type_id=ref_type_id,
        indexed=indexed,
        searchable=searchable,
        deprecated=deprecated,
        description=description,
    )


@dataclass(frozen=True)
class NodeTypeDef:
    """Definition of a node type.

    Nodes are the primary entities in the graph. Each node type
    defines the structure (fields) and behavior of nodes.

    Attributes:
        type_id: Stable numeric identifier (1-2^31)
        name: Human-readable name
        fields: Tuple of field definitions
        deprecated: Whether type is deprecated
        description: Documentation

    Example:
        >>> Task = NodeTypeDef(
        ...     type_id=101,
        ...     name="Task",
        ...     fields=(
        ...         field(1, "title", "str", required=True),
        ...         field(2, "status", "enum", enum_values=("todo", "done")),
        ...     ),
        ... )
    """

    type_id: int
    name: str
    fields: tuple[FieldDef, ...] = dataclass_field(default_factory=tuple)
    deprecated: bool = False
    description: str = ""

    def __post_init__(self) -> None:
        """Validate node type definition."""
        if self.type_id <= 0:
            raise ValueError(f"type_id must be positive, got {self.type_id}")
        if not self.name:
            raise ValueError("Node type name cannot be empty")

        # Check for duplicate field IDs
        field_ids = [f.field_id for f in self.fields]
        if len(field_ids) != len(set(field_ids)):
            raise ValueError(f"Duplicate field_id in node type '{self.name}'")

    def get_field(self, name_or_id: str | int) -> FieldDef | None:
        """Get field by name or ID."""
        for f in self.fields:
            if isinstance(name_or_id, int):
                if f.field_id == name_or_id:
                    return f
            else:
                if f.name == name_or_id:
                    return f
        return None

    def get_field_names(self) -> list[str]:
        """Get list of field names."""
        return [f.name for f in self.fields if not f.deprecated]

    def validate_payload(self, payload: dict[str, Any]) -> tuple[bool, list[str]]:
        """Validate a payload against this type."""
        errors: list[str] = []

        # Check for unknown fields
        known = {f.name for f in self.fields}
        unknown = set(payload.keys()) - known
        if unknown:
            errors.append(f"Unknown fields: {sorted(unknown)}")

        # Validate each field
        for f in self.fields:
            value = payload.get(f.name, f.default)
            is_valid, error = f.validate_value(value)
            if not is_valid and error:
                errors.append(error)

        return len(errors) == 0, errors

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        return {
            "type_id": self.type_id,
            "name": self.name,
            "fields": [f.to_dict() for f in self.fields],
            "deprecated": self.deprecated,
            "description": self.description,
        }

    def new(self, **kwargs: Any) -> dict[str, Any]:
        """Create a validated payload for this type.

        Args:
            **kwargs: Field values

        Returns:
            Validated payload dictionary

        Raises:
            TypeError: If unknown field is provided
            ValueError: If validation fails

        Example:
            >>> payload = Task.new(title="My Task", status="todo")
        """
        # Check for unknown fields
        known = {f.name for f in self.fields}
        unknown = set(kwargs.keys()) - known
        if unknown:
            suggestions = _find_suggestions(list(unknown)[0], list(known))
            msg = f"Unknown field(s): {sorted(unknown)}"
            if suggestions:
                msg += f". Did you mean: {suggestions}?"
            raise TypeError(msg)

        # Build payload with defaults
        payload: dict[str, Any] = {}
        for f in self.fields:
            if f.name in kwargs:
                payload[f.name] = kwargs[f.name]
            elif f.default is not None:
                payload[f.name] = f.default

        # Validate
        is_valid, errors = self.validate_payload(payload)
        if not is_valid:
            raise ValueError(f"Validation failed: {'; '.join(errors)}")

        return payload

    def __hash__(self) -> int:
        return hash(self.type_id)


@dataclass(frozen=True)
class EdgeTypeDef:
    """Definition of an edge type.

    Edges are unidirectional relationships between nodes.
    Each edge type defines the source/target types and properties.

    Attributes:
        edge_id: Stable numeric identifier
        name: Human-readable name
        from_type: Source node type
        to_type: Target node type
        props: Edge properties
        unique_per_from: One edge per source node
        deprecated: Whether deprecated
        description: Documentation

    Example:
        >>> AssignedTo = EdgeTypeDef(
        ...     edge_id=401,
        ...     name="AssignedTo",
        ...     from_type=Task,
        ...     to_type=User,
        ...     props=(field(1, "role", "enum", enum_values=("primary",)),),
        ... )
    """

    edge_id: int
    name: str
    from_type: NodeTypeDef | int
    to_type: NodeTypeDef | int
    props: tuple[FieldDef, ...] = dataclass_field(default_factory=tuple)
    unique_per_from: bool = False
    deprecated: bool = False
    description: str = ""

    def __post_init__(self) -> None:
        """Validate edge type definition."""
        if self.edge_id <= 0:
            raise ValueError(f"edge_id must be positive, got {self.edge_id}")
        if not self.name:
            raise ValueError("Edge type name cannot be empty")

    @property
    def from_type_id(self) -> int:
        """Get source type ID."""
        if isinstance(self.from_type, NodeTypeDef):
            return self.from_type.type_id
        return self.from_type

    @property
    def to_type_id(self) -> int:
        """Get target type ID."""
        if isinstance(self.to_type, NodeTypeDef):
            return self.to_type.type_id
        return self.to_type

    def validate_props(self, props: dict[str, Any]) -> tuple[bool, list[str]]:
        """Validate edge properties."""
        errors: list[str] = []

        known = {p.name for p in self.props}
        unknown = set(props.keys()) - known
        if unknown:
            errors.append(f"Unknown properties: {sorted(unknown)}")

        for p in self.props:
            value = props.get(p.name, p.default)
            is_valid, error = p.validate_value(value)
            if not is_valid and error:
                errors.append(error)

        return len(errors) == 0, errors

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        return {
            "edge_id": self.edge_id,
            "name": self.name,
            "from_type_id": self.from_type_id,
            "to_type_id": self.to_type_id,
            "props": [p.to_dict() for p in self.props],
            "unique_per_from": self.unique_per_from,
            "deprecated": self.deprecated,
            "description": self.description,
        }

    def __hash__(self) -> int:
        return hash(self.edge_id)


def _find_suggestions(unknown: str, known: list[str]) -> list[str]:
    """Find similar field names for suggestions."""
    suggestions = []
    unknown_lower = unknown.lower()

    for name in known:
        # Check prefix match
        if (
            (name.lower().startswith(unknown_lower[:3]) if len(unknown_lower) >= 3 else False)
            or unknown_lower in name.lower()
            or name.lower() in unknown_lower
        ):
            suggestions.append(name)

    return suggestions[:3]
