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
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from google.protobuf import descriptor_pb2


class DataPolicy(Enum):
    """Data classification for GDPR compliance."""

    PERSONAL = 0  # user's own data, fully exportable, DELETE on exit
    BUSINESS = 1  # business data, contributions exportable, ANONYMIZE on exit
    FINANCIAL = 2  # legal retention required, RETAIN + ANONYMIZE PII
    AUDIT = 3  # security logs, not user-exportable, RETAIN + ANONYMIZE PII
    EPHEMERAL = 4  # temporary, not exportable, DELETE on exit
    HEALTHCARE = 5  # PHI, field-level encryption, 6yr retention


class SubjectExitPolicy(Enum):
    """What happens to edges when referenced user exits."""

    BOTH = 0  # act on edges in both directions
    FROM = 1  # only when user is the source (from)
    TO = 2  # only when user is the target (to)


@dataclass(frozen=True)
class AclDefaults:
    """Default ACL configuration for a node type.

    Controls how a node's access is determined at creation time.

    Attributes:
        public: Whether nodes of this type are world-readable by default
        tenant_visible: Whether nodes are visible to all tenant members
        inherit: Whether to inherit ACL from the structural parent.
            When False, the node is private — only direct grants and
            owner have access. No acl_inherit row is created.
    """

    public: bool = False
    tenant_visible: bool = True
    inherit: bool = True

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        return {
            "public": self.public,
            "tenant_visible": self.tenant_visible,
            "inherit": self.inherit,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> AclDefaults:
        """Create from dictionary."""
        return cls(
            public=data.get("public", False),
            tenant_visible=data.get("tenant_visible", True),
            inherit=data.get("inherit", True),
        )


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
    pii: bool = False
    phi: bool = False
    pii_false: bool = False
    deprecated: bool = False
    description: str = ""
    unique: bool = False

    def __post_init__(self) -> None:
        """Validate field definition."""
        if self.field_id <= 0 or self.field_id > 65535:
            raise ValueError(f"field_id must be 1-65535, got {self.field_id}")
        if not self.name:
            raise ValueError("Field name cannot be empty")
        if self.kind == FieldKind.ENUM and not self.enum_values:
            raise ValueError(f"enum_values required for ENUM field '{self.name}'")

    def validate_value(self, value: Any) -> tuple[bool, str | None]:
        """Validate a value against this field.

        Checks:
        - None against required constraint
        - Type compatibility for all FieldKind variants
        - Enum membership for ENUM fields
        """
        if value is None:
            if self.required:
                return False, f"Field '{self.name}' is required"
            return True, None

        if self.kind == FieldKind.STRING:
            if not isinstance(value, str):
                return False, f"Field '{self.name}' must be a string, got {type(value).__name__}"

        elif self.kind == FieldKind.INTEGER:
            if not isinstance(value, int) or isinstance(value, bool):
                return False, f"Field '{self.name}' must be an integer, got {type(value).__name__}"

        elif self.kind == FieldKind.FLOAT:
            if not isinstance(value, int | float) or isinstance(value, bool):
                return False, f"Field '{self.name}' must be a number, got {type(value).__name__}"

        elif self.kind == FieldKind.BOOLEAN:
            if not isinstance(value, bool):
                return False, f"Field '{self.name}' must be a boolean, got {type(value).__name__}"

        elif self.kind == FieldKind.TIMESTAMP:
            if not isinstance(value, int) or isinstance(value, bool) or value < 0:
                return False, f"Field '{self.name}' must be a non-negative integer timestamp"

        elif self.kind == FieldKind.ENUM:
            if not isinstance(value, str):
                return False, f"Field '{self.name}' must be a string, got {type(value).__name__}"
            if self.enum_values is not None and value not in self.enum_values:
                return False, f"Field '{self.name}' must be one of {self.enum_values}"

        elif self.kind == FieldKind.JSON:
            if not isinstance(value, dict | list):
                return (
                    False,
                    f"Field '{self.name}' must be a dict or list, got {type(value).__name__}",
                )

        elif self.kind == FieldKind.BYTES:
            if not isinstance(value, bytes | str):
                return (
                    False,
                    f"Field '{self.name}' must be bytes or string, got {type(value).__name__}",
                )

        elif self.kind == FieldKind.REFERENCE:
            if not isinstance(value, dict):
                return False, f"Field '{self.name}' must be a reference object (dict)"
            if "type_id" not in value or "id" not in value:
                return False, f"Field '{self.name}' must have 'type_id' and 'id'"

        elif self.kind == FieldKind.LIST_STRING:
            if not isinstance(value, list):
                return False, f"Field '{self.name}' must be a list, got {type(value).__name__}"
            for i, item in enumerate(value):
                if not isinstance(item, str):
                    return False, f"Field '{self.name}[{i}]' must be a string"

        elif self.kind == FieldKind.LIST_INT:
            if not isinstance(value, list):
                return False, f"Field '{self.name}' must be a list, got {type(value).__name__}"
            for i, item in enumerate(value):
                if not isinstance(item, int) or isinstance(item, bool):
                    return False, f"Field '{self.name}[{i}]' must be an integer"

        elif self.kind == FieldKind.LIST_REF:
            if not isinstance(value, list):
                return False, f"Field '{self.name}' must be a list, got {type(value).__name__}"
            for i, item in enumerate(value):
                if not isinstance(item, dict):
                    return False, f"Field '{self.name}[{i}]' must be a reference object"

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
        if self.pii:
            result["pii"] = True
        if self.phi:
            result["phi"] = True
        if self.pii_false:
            result["pii_false"] = True
        if self.deprecated:
            result["deprecated"] = True
        if self.description:
            result["description"] = self.description
        if self.unique:
            result["unique"] = True
        return result

    @classmethod
    def from_descriptor(
        cls,
        fd: descriptor_pb2.FieldDescriptorProto,
        field_opts: Any | None = None,
    ) -> FieldDef:
        """Build a FieldDef from a proto FieldDescriptorProto + FieldOpts.

        This factory lets codegen construct FieldDefs directly from parsed
        proto descriptors without going through an intermediate FieldInfo.

        Args:
            fd: A ``FieldDescriptorProto`` from a compiled .proto.
            field_opts: Resolved ``FieldOpts`` extension message (or ``None``).

        Returns:
            A fully populated ``FieldDef``.
        """
        # Proto type number -> EntDB FieldKind value string
        _proto_type_map = {
            1: "float",  # TYPE_DOUBLE
            2: "float",  # TYPE_FLOAT
            3: "int",  # TYPE_INT64
            4: "int",  # TYPE_UINT64
            5: "int",  # TYPE_INT32
            8: "bool",  # TYPE_BOOL
            9: "str",  # TYPE_STRING
            12: "bytes",  # TYPE_BYTES
            13: "int",  # TYPE_UINT32
            17: "int",  # TYPE_SINT32
            18: "int",  # TYPE_SINT64
        }

        kind_override = field_opts.kind if field_opts is not None else ""
        enum_str = field_opts.enum_values if field_opts is not None else ""
        enum_values = (
            tuple(v.strip() for v in enum_str.split(",") if v.strip()) if enum_str else None
        )

        # Resolve kind from proto type + optional override
        if kind_override:
            kind_str = kind_override
        elif fd.label == 3:  # LABEL_REPEATED
            base = _proto_type_map.get(fd.type, "str")
            if base == "str":
                kind_str = "list_str"
            elif base == "int":
                kind_str = "list_int"
            else:
                kind_str = "json"
        else:
            kind_str = _proto_type_map.get(fd.type, "str")

        if enum_values and kind_str == "str":
            kind_str = "enum"

        default_value = (field_opts.default_value or None) if field_opts is not None else None

        return cls(
            field_id=fd.number,
            name=fd.name,
            kind=FieldKind.from_str(kind_str),
            required=bool(field_opts.required) if field_opts is not None else False,
            default=default_value,
            enum_values=enum_values,
            ref_type_id=(field_opts.ref_type_id or None) if field_opts is not None else None,
            indexed=bool(field_opts.indexed) if field_opts is not None else False,
            searchable=bool(field_opts.searchable) if field_opts is not None else False,
            pii=bool(field_opts.pii) if field_opts is not None else False,
            phi=bool(field_opts.phi) if field_opts is not None else False,
            pii_false=bool(field_opts.pii_false) if field_opts is not None else False,
            deprecated=bool(field_opts.deprecated) if field_opts is not None else False,
            description=field_opts.description if field_opts is not None else "",
            unique=bool(field_opts.unique) if field_opts is not None else False,
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
    pii: bool = False,
    phi: bool = False,
    pii_false: bool = False,
    deprecated: bool = False,
    description: str = "",
    unique: bool = False,
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
        pii: Personally identifiable information
        phi: Protected health information
        pii_false: Explicitly declared as NOT PII
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
        pii=pii,
        phi=phi,
        pii_false=pii_false,
        deprecated=deprecated,
        description=description,
        unique=unique,
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
    acl_defaults: AclDefaults = dataclass_field(default_factory=AclDefaults)
    data_policy: DataPolicy = DataPolicy.PERSONAL
    subject_field: str = ""
    retention_days: int = 0
    legal_basis: str = ""
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
        result: dict[str, Any] = {
            "type_id": self.type_id,
            "name": self.name,
            "fields": [f.to_dict() for f in self.fields],
            "acl_defaults": self.acl_defaults.to_dict(),
            "data_policy": self.data_policy.name,
            "deprecated": self.deprecated,
            "description": self.description,
        }
        if self.subject_field:
            result["subject_field"] = self.subject_field
        if self.retention_days:
            result["retention_days"] = self.retention_days
        if self.legal_basis:
            result["legal_basis"] = self.legal_basis
        return result

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

    @classmethod
    def from_descriptor(
        cls,
        msg: descriptor_pb2.DescriptorProto,
        node_opts: Any,
        *,
        _reparse_field_options: Any = None,
    ) -> NodeTypeDef:
        """Build a NodeTypeDef from a proto DescriptorProto + NodeOpts.

        This factory lets codegen construct NodeTypeDefs directly from
        parsed proto descriptors rather than going through NodeInfo.

        Args:
            msg: A ``DescriptorProto`` for the message.
            node_opts: Resolved ``NodeOpts`` extension message.
            _reparse_field_options: Optional callable to re-parse
                ``FieldOptions`` for extension access. When ``None``, field
                options on ``msg.field`` entries are used as-is.

        Returns:
            A fully populated ``NodeTypeDef``.
        """
        _data_policy_map = {
            0: DataPolicy.PERSONAL,
            1: DataPolicy.BUSINESS,
            2: DataPolicy.FINANCIAL,
            3: DataPolicy.AUDIT,
            4: DataPolicy.EPHEMERAL,
            5: DataPolicy.HEALTHCARE,
        }

        fields: list[FieldDef] = []
        for fd in msg.field:
            fext = None
            if _reparse_field_options is not None:
                from ._generated import entdb_options_pb2

                opts = _reparse_field_options(fd.options) if fd.options else None
                if opts is not None and opts.HasExtension(entdb_options_pb2.field):
                    fext = opts.Extensions[entdb_options_pb2.field]
            fields.append(FieldDef.from_descriptor(fd, fext))

        return cls(
            type_id=node_opts.type_id,
            name=msg.name,
            fields=tuple(fields),
            acl_defaults=AclDefaults(
                public=node_opts.public,
                tenant_visible=node_opts.tenant_visible,
                inherit=node_opts.inherit,
            ),
            data_policy=_data_policy_map.get(int(node_opts.data_policy), DataPolicy.PERSONAL),
            subject_field=node_opts.subject_field,
            retention_days=node_opts.retention_days,
            legal_basis=node_opts.legal_basis,
            deprecated=node_opts.deprecated,
            description=node_opts.description,
        )

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
    propagate_share: bool = False
    unique_per_from: bool = False
    data_policy: DataPolicy = DataPolicy.PERSONAL
    on_subject_exit: SubjectExitPolicy = SubjectExitPolicy.BOTH
    retention_days: int = 0
    legal_basis: str = ""
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
        result: dict[str, Any] = {
            "edge_id": self.edge_id,
            "name": self.name,
            "from_type_id": self.from_type_id,
            "to_type_id": self.to_type_id,
            "props": [p.to_dict() for p in self.props],
            "propagate_share": self.propagate_share,
            "unique_per_from": self.unique_per_from,
            "data_policy": self.data_policy.name,
            "on_subject_exit": self.on_subject_exit.name,
            "deprecated": self.deprecated,
            "description": self.description,
        }
        if self.retention_days:
            result["retention_days"] = self.retention_days
        if self.legal_basis:
            result["legal_basis"] = self.legal_basis
        return result

    @classmethod
    def from_descriptor(
        cls,
        msg: descriptor_pb2.DescriptorProto,
        edge_opts: Any,
        *,
        _reparse_field_options: Any = None,
    ) -> EdgeTypeDef:
        """Build an EdgeTypeDef from a proto DescriptorProto + EdgeOpts.

        Args:
            msg: A ``DescriptorProto`` for the message.
            edge_opts: Resolved ``EdgeOpts`` extension message.
            _reparse_field_options: Optional callable to re-parse
                ``FieldOptions`` for extension access.

        Returns:
            A fully populated ``EdgeTypeDef``.
        """
        _data_policy_map = {
            0: DataPolicy.PERSONAL,
            1: DataPolicy.BUSINESS,
            2: DataPolicy.FINANCIAL,
            3: DataPolicy.AUDIT,
            4: DataPolicy.EPHEMERAL,
            5: DataPolicy.HEALTHCARE,
        }
        _subject_exit_map = {
            0: SubjectExitPolicy.BOTH,
            1: SubjectExitPolicy.FROM,
            2: SubjectExitPolicy.TO,
        }

        props: list[FieldDef] = []
        for fd in msg.field:
            fext = None
            if _reparse_field_options is not None:
                from ._generated import entdb_options_pb2

                opts = _reparse_field_options(fd.options) if fd.options else None
                if opts is not None and opts.HasExtension(entdb_options_pb2.field):
                    fext = opts.Extensions[entdb_options_pb2.field]
            props.append(FieldDef.from_descriptor(fd, fext))

        return cls(
            edge_id=edge_opts.edge_id,
            name=edge_opts.name or msg.name,
            from_type=0,
            to_type=0,
            props=tuple(props),
            propagate_share=edge_opts.propagate_share,
            unique_per_from=edge_opts.unique_per_from,
            data_policy=_data_policy_map.get(int(edge_opts.data_policy), DataPolicy.PERSONAL),
            on_subject_exit=_subject_exit_map.get(
                int(edge_opts.on_subject_exit), SubjectExitPolicy.BOTH
            ),
            retention_days=edge_opts.retention_days,
            legal_basis=edge_opts.legal_basis,
            deprecated=edge_opts.deprecated,
            description=edge_opts.description,
        )

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
