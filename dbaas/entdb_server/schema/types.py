"""
Core type definitions for EntDB schema system.

This module defines the foundational types for the node/edge graph model:
- FieldDef: Individual field within a node or edge
- NodeTypeDef: Definition of a node type
- EdgeTypeDef: Definition of a unidirectional edge type

Invariants:
    - type_id must be positive integers (1-2^31)
    - edge_id must be positive integers (1-2^31)
    - field_id must be positive integers within each type (1-2^16)
    - Names are labels only; IDs are canonical
    - enum_values must be immutable once defined (append-only)

How to change safely:
    - Add new fields with new field_ids
    - Add new enum values at the end of enum_values tuple
    - Never remove or reorder enum values
    - Deprecate fields by adding deprecated=True (keep in schema)

Example:
    >>> from entdb_server.schema.types import NodeTypeDef, field
    >>> Task = NodeTypeDef(
    ...     type_id=101,
    ...     name="Task",
    ...     fields=(
    ...         field(1, "title", "str"),
    ...         field(2, "status", "enum", enum_values=("todo", "doing", "done")),
    ...     ),
    ... )
"""

from __future__ import annotations

from dataclasses import dataclass
from dataclasses import field as dataclass_field
from enum import Enum
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    pass


class FieldKind(Enum):
    """Supported field types in the schema.

    These map to storage representations and validation rules.
    """

    STRING = "str"
    INTEGER = "int"
    FLOAT = "float"
    BOOLEAN = "bool"
    TIMESTAMP = "timestamp"  # Unix milliseconds
    JSON = "json"  # Arbitrary JSON object
    BYTES = "bytes"  # Binary data (base64 encoded in JSON)
    ENUM = "enum"  # Enumerated string values
    REFERENCE = "ref"  # Reference to another node (type_id, node_id)
    LIST_STRING = "list_str"  # List of strings
    LIST_INT = "list_int"  # List of integers
    LIST_REF = "list_ref"  # List of references

    @classmethod
    def from_str(cls, value: str) -> FieldKind:
        """Convert string representation to FieldKind.

        Args:
            value: String name of the field kind

        Returns:
            Corresponding FieldKind enum value

        Raises:
            ValueError: If value is not a valid field kind
        """
        for kind in cls:
            if kind.value == value:
                return kind
        valid = [k.value for k in cls]
        raise ValueError(f"Invalid field kind '{value}'. Valid kinds: {valid}")


class AclPermission(Enum):
    """Permission levels for ACL entries."""

    READ = "read"
    WRITE = "write"
    DELETE = "delete"
    ADMIN = "admin"  # Can modify ACL


@dataclass(frozen=True)
class AclEntry:
    """ACL entry granting permission to a principal.

    Attributes:
        principal: The actor identifier (e.g., "user:42", "role:admin", "tenant:*")
        permission: The permission level granted
    """

    principal: str
    permission: AclPermission

    def to_dict(self) -> dict[str, str]:
        """Convert to dictionary representation."""
        return {
            "principal": self.principal,
            "permission": self.permission.value,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> AclEntry:
        """Create from dictionary representation."""
        return cls(
            principal=data["principal"],
            permission=AclPermission(data["permission"]),
        )


@dataclass(frozen=True)
class FieldDef:
    """Definition of a single field within a node or edge type.

    Attributes:
        field_id: Stable numeric identifier (never changes, never reused)
        name: Human-readable name (can change, ID is canonical)
        kind: The data type of the field
        required: Whether the field is required on create
        default: Default value if not provided
        enum_values: Valid values if kind is ENUM (append-only)
        ref_type_id: Target type_id if kind is REFERENCE
        indexed: Whether to create an index on this field
        searchable: Whether to include in FTS index
        deprecated: Whether this field is deprecated
        description: Human-readable description

    Invariants:
        - field_id must be unique within the containing type
        - kind cannot change after definition
        - enum_values can only be appended, never removed or reordered
        - ref_type_id must reference a valid registered type

    Example:
        >>> title_field = FieldDef(
        ...     field_id=1,
        ...     name="title",
        ...     kind=FieldKind.STRING,
        ...     required=True,
        ...     searchable=True,
        ... )
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
        if self.field_id <= 0:
            raise ValueError(f"field_id must be positive, got {self.field_id}")
        if self.field_id > 65535:
            raise ValueError(f"field_id must be <= 65535, got {self.field_id}")
        if not self.name:
            raise ValueError("Field name cannot be empty")
        if self.kind == FieldKind.ENUM and not self.enum_values:
            raise ValueError(f"enum_values required for ENUM field '{self.name}'")
        if self.kind == FieldKind.REFERENCE and self.ref_type_id is None:
            raise ValueError(f"ref_type_id required for REFERENCE field '{self.name}'")

    def validate_value(self, value: Any) -> tuple[bool, str | None]:
        """Validate a value against this field definition.

        Args:
            value: The value to validate

        Returns:
            Tuple of (is_valid, error_message)
        """
        if value is None:
            if self.required:
                return False, f"Field '{self.name}' is required"
            return True, None

        validators = {
            FieldKind.STRING: lambda v: isinstance(v, str),
            FieldKind.INTEGER: lambda v: isinstance(v, int) and not isinstance(v, bool),
            FieldKind.FLOAT: lambda v: isinstance(v, (int, float)) and not isinstance(v, bool),
            FieldKind.BOOLEAN: lambda v: isinstance(v, bool),
            FieldKind.TIMESTAMP: lambda v: isinstance(v, int) and v >= 0,
            FieldKind.JSON: lambda _: True,  # Any JSON-serializable value
            FieldKind.BYTES: lambda v: isinstance(v, (str, bytes)),
            FieldKind.LIST_STRING: lambda v: isinstance(v, list)
            and all(isinstance(i, str) for i in v),
            FieldKind.LIST_INT: lambda v: isinstance(v, list)
            and all(isinstance(i, int) and not isinstance(i, bool) for i in v),
            FieldKind.LIST_REF: lambda v: isinstance(v, list)
            and all(isinstance(i, dict) for i in v),
        }

        if self.kind == FieldKind.ENUM:
            if not isinstance(value, str):
                return False, f"Field '{self.name}' must be a string, got {type(value).__name__}"
            if self.enum_values and value not in self.enum_values:
                return (
                    False,
                    f"Field '{self.name}' must be one of {self.enum_values}, got '{value}'",
                )
            return True, None

        if self.kind == FieldKind.REFERENCE:
            if not isinstance(value, dict):
                return (
                    False,
                    f"Field '{self.name}' must be a reference dict, got {type(value).__name__}",
                )
            if "type_id" not in value or "id" not in value:
                return False, f"Field '{self.name}' reference must have 'type_id' and 'id'"
            return True, None

        validator = validators.get(self.kind)
        if validator and not validator(value):
            return False, f"Field '{self.name}' has invalid type for kind {self.kind.value}"

        return True, None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary representation for serialization."""
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

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> FieldDef:
        """Create from dictionary representation."""
        return cls(
            field_id=data["field_id"],
            name=data["name"],
            kind=FieldKind.from_str(data["kind"]),
            required=data.get("required", False),
            default=data.get("default"),
            enum_values=tuple(data["enum_values"]) if data.get("enum_values") else None,
            ref_type_id=data.get("ref_type_id"),
            indexed=data.get("indexed", False),
            searchable=data.get("searchable", False),
            deprecated=data.get("deprecated", False),
            description=data.get("description", ""),
        )


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

    This is the preferred way to define fields in schema definitions.

    Args:
        field_id: Stable numeric identifier
        name: Human-readable name
        kind: Field type (string or FieldKind enum)
        required: Whether field is required
        default: Default value
        enum_values: Valid enum values (for enum kind)
        ref_type_id: Target type for references
        indexed: Whether to create index
        searchable: Whether to include in FTS
        deprecated: Whether field is deprecated
        description: Human-readable description

    Returns:
        FieldDef instance

    Example:
        >>> title = field(1, "title", "str", required=True, searchable=True)
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
    """Definition of a node type in the graph.

    Nodes are the primary entities in the graph model. Each node has:
    - A type (defined by this class)
    - A unique ID within the tenant
    - A payload of fields
    - An ACL controlling access
    - Metadata (created_at, updated_at, owner)

    Attributes:
        type_id: Stable numeric identifier (1-2^31, never changes)
        name: Human-readable name (can be renamed, ID is canonical)
        fields: Tuple of field definitions
        deprecated: Whether this type is deprecated
        description: Human-readable description
        default_acl: Default ACL for new nodes of this type

    Invariants:
        - type_id must be unique across all node types
        - type_id cannot be reused after deprecation
        - fields can be added but not removed (deprecate instead)

    Example:
        >>> User = NodeTypeDef(
        ...     type_id=1,
        ...     name="User",
        ...     fields=(
        ...         field(1, "email", "str", required=True, indexed=True),
        ...         field(2, "display_name", "str"),
        ...     ),
        ... )
    """

    type_id: int
    name: str
    fields: tuple[FieldDef, ...] = dataclass_field(default_factory=tuple)
    deprecated: bool = False
    description: str = ""
    default_acl: tuple[AclEntry, ...] = dataclass_field(default_factory=tuple)

    def __post_init__(self) -> None:
        """Validate node type definition."""
        if self.type_id <= 0:
            raise ValueError(f"type_id must be positive, got {self.type_id}")
        if self.type_id > 2147483647:
            raise ValueError(f"type_id must be <= 2^31-1, got {self.type_id}")
        if not self.name:
            raise ValueError("Node type name cannot be empty")

        # Check for duplicate field IDs
        field_ids = [f.field_id for f in self.fields]
        if len(field_ids) != len(set(field_ids)):
            raise ValueError(f"Duplicate field_id in node type '{self.name}'")

        # Check for duplicate field names
        field_names = [f.name for f in self.fields]
        if len(field_names) != len(set(field_names)):
            raise ValueError(f"Duplicate field name in node type '{self.name}'")

    def get_field(self, name_or_id: str | int) -> FieldDef | None:
        """Get a field by name or ID.

        Args:
            name_or_id: Field name (str) or field_id (int)

        Returns:
            FieldDef if found, None otherwise
        """
        for f in self.fields:
            if isinstance(name_or_id, int):
                if f.field_id == name_or_id:
                    return f
            else:
                if f.name == name_or_id:
                    return f
        return None

    def get_field_names(self) -> list[str]:
        """Get list of all field names."""
        return [f.name for f in self.fields if not f.deprecated]

    def get_required_fields(self) -> list[FieldDef]:
        """Get list of required fields."""
        return [f for f in self.fields if f.required and not f.deprecated]

    def get_searchable_fields(self) -> list[FieldDef]:
        """Get list of searchable fields for FTS."""
        return [f for f in self.fields if f.searchable and not f.deprecated]

    def validate_payload(self, payload: dict[str, Any]) -> tuple[bool, list[str]]:
        """Validate a payload against this node type.

        Args:
            payload: Dictionary of field values

        Returns:
            Tuple of (is_valid, list_of_errors)
        """
        errors: list[str] = []

        # Check for unknown fields
        known_names = {f.name for f in self.fields}
        unknown = set(payload.keys()) - known_names
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
        """Convert to dictionary representation."""
        result: dict[str, Any] = {
            "type_id": self.type_id,
            "name": self.name,
            "fields": [f.to_dict() for f in self.fields],
        }
        if self.deprecated:
            result["deprecated"] = True
        if self.description:
            result["description"] = self.description
        if self.default_acl:
            result["default_acl"] = [a.to_dict() for a in self.default_acl]
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> NodeTypeDef:
        """Create from dictionary representation."""
        fields = tuple(FieldDef.from_dict(f) for f in data.get("fields", []))
        default_acl = tuple(AclEntry.from_dict(a) for a in data.get("default_acl", []))
        return cls(
            type_id=data["type_id"],
            name=data["name"],
            fields=fields,
            deprecated=data.get("deprecated", False),
            description=data.get("description", ""),
            default_acl=default_acl,
        )

    def __hash__(self) -> int:
        """Hash based on type_id (stable identifier)."""
        return hash(self.type_id)

    def __eq__(self, other: object) -> bool:
        """Equality based on type_id."""
        if not isinstance(other, NodeTypeDef):
            return NotImplemented
        return self.type_id == other.type_id


@dataclass(frozen=True)
class EdgeTypeDef:
    """Definition of an edge type (unidirectional relationship).

    Edges connect nodes and can have their own properties.
    They are always unidirectional (from -> to).

    Attributes:
        edge_id: Stable numeric identifier (never changes)
        name: Human-readable name (can be renamed)
        from_type: Source node type (or type_id)
        to_type: Target node type (or type_id)
        props: Tuple of property field definitions
        unique_per_from: Whether only one edge of this type per source node
        deprecated: Whether this edge type is deprecated
        description: Human-readable description

    Invariants:
        - edge_id must be unique across all edge types
        - edge_id cannot be reused after deprecation
        - props can be added but not removed

    Example:
        >>> AssignedTo = EdgeTypeDef(
        ...     edge_id=401,
        ...     name="AssignedTo",
        ...     from_type=Task,  # NodeTypeDef
        ...     to_type=User,    # NodeTypeDef
        ...     props=(field(1, "role", "enum", enum_values=("primary", "reviewer")),),
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
        if self.edge_id > 2147483647:
            raise ValueError(f"edge_id must be <= 2^31-1, got {self.edge_id}")
        if not self.name:
            raise ValueError("Edge type name cannot be empty")

        # Check for duplicate prop IDs
        prop_ids = [p.field_id for p in self.props]
        if len(prop_ids) != len(set(prop_ids)):
            raise ValueError(f"Duplicate field_id in edge type '{self.name}'")

    @property
    def from_type_id(self) -> int:
        """Get the source type ID."""
        if isinstance(self.from_type, NodeTypeDef):
            return self.from_type.type_id
        return self.from_type

    @property
    def to_type_id(self) -> int:
        """Get the target type ID."""
        if isinstance(self.to_type, NodeTypeDef):
            return self.to_type.type_id
        return self.to_type

    def get_prop(self, name_or_id: str | int) -> FieldDef | None:
        """Get a property by name or ID."""
        for p in self.props:
            if isinstance(name_or_id, int):
                if p.field_id == name_or_id:
                    return p
            else:
                if p.name == name_or_id:
                    return p
        return None

    def validate_props(self, props: dict[str, Any]) -> tuple[bool, list[str]]:
        """Validate edge properties.

        Args:
            props: Dictionary of property values

        Returns:
            Tuple of (is_valid, list_of_errors)
        """
        errors: list[str] = []

        known_names = {p.name for p in self.props}
        unknown = set(props.keys()) - known_names
        if unknown:
            errors.append(f"Unknown properties: {sorted(unknown)}")

        for p in self.props:
            value = props.get(p.name, p.default)
            is_valid, error = p.validate_value(value)
            if not is_valid and error:
                errors.append(error)

        return len(errors) == 0, errors

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary representation."""
        result: dict[str, Any] = {
            "edge_id": self.edge_id,
            "name": self.name,
            "from_type_id": self.from_type_id,
            "to_type_id": self.to_type_id,
            "props": [p.to_dict() for p in self.props],
        }
        if self.unique_per_from:
            result["unique_per_from"] = True
        if self.deprecated:
            result["deprecated"] = True
        if self.description:
            result["description"] = self.description
        return result

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> EdgeTypeDef:
        """Create from dictionary representation."""
        props = tuple(FieldDef.from_dict(p) for p in data.get("props", []))
        return cls(
            edge_id=data["edge_id"],
            name=data["name"],
            from_type=data["from_type_id"],
            to_type=data["to_type_id"],
            props=props,
            unique_per_from=data.get("unique_per_from", False),
            deprecated=data.get("deprecated", False),
            description=data.get("description", ""),
        )

    def __hash__(self) -> int:
        """Hash based on edge_id."""
        return hash(self.edge_id)

    def __eq__(self, other: object) -> bool:
        """Equality based on edge_id."""
        if not isinstance(other, EdgeTypeDef):
            return NotImplemented
        return self.edge_id == other.edge_id
