"""
YAML/JSON Schema Format for EntDB Playground.

This module defines the schema format and provides parsing/validation.
The schema is the source of truth - code is always generated from it.

Example schema:
    node_types:
      - type_id: 1
        name: Product
        fields:
          - id: 1
            name: title
            kind: str
            required: true

    edge_types:
      - edge_id: 101
        name: CONTAINS
        from_type: 2
        to_type: 1

    data:
      - create_node:
          type_id: 1
          payload:
            title: "Widget"
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any

import yaml

# Valid field kinds
VALID_KINDS = {
    "str",
    "int",
    "float",
    "bool",
    "timestamp",
    "json",
    "bytes",
    "enum",
    "ref",
    "list_str",
    "list_int",
    "list_ref",
}


@dataclass
class FieldSchema:
    """Schema for a single field."""

    id: int
    name: str
    kind: str
    required: bool = False
    default: Any = None
    values: list[str] | None = None  # For enum
    indexed: bool = False
    searchable: bool = False

    def validate(self) -> list[str]:
        """Validate the field schema."""
        errors = []
        if self.id <= 0:
            errors.append(f"Field '{self.name}': id must be positive")
        if not self.name:
            errors.append("Field name is required")
        if self.kind not in VALID_KINDS:
            errors.append(f"Field '{self.name}': invalid kind '{self.kind}'. Valid: {VALID_KINDS}")
        if self.kind == "enum" and not self.values:
            errors.append(f"Field '{self.name}': enum requires 'values' list")
        return errors

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        d: dict[str, Any] = {
            "id": self.id,
            "name": self.name,
            "kind": self.kind,
        }
        if self.required:
            d["required"] = True
        if self.default is not None:
            d["default"] = self.default
        if self.values:
            d["values"] = self.values
        if self.indexed:
            d["indexed"] = True
        if self.searchable:
            d["searchable"] = True
        return d


@dataclass
class NodeTypeSchema:
    """Schema for a node type."""

    type_id: int
    name: str
    fields: list[FieldSchema] = field(default_factory=list)
    description: str = ""

    def validate(self) -> list[str]:
        """Validate the node type schema."""
        errors = []
        if self.type_id <= 0:
            errors.append(f"Node type '{self.name}': type_id must be positive")
        if not self.name:
            errors.append("Node type name is required")

        # Check for duplicate field IDs
        field_ids = [f.id for f in self.fields]
        if len(field_ids) != len(set(field_ids)):
            errors.append(f"Node type '{self.name}': duplicate field IDs")

        # Validate each field
        for f in self.fields:
            errors.extend(f.validate())

        return errors

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        d: dict[str, Any] = {
            "type_id": self.type_id,
            "name": self.name,
            "fields": [f.to_dict() for f in self.fields],
        }
        if self.description:
            d["description"] = self.description
        return d


@dataclass
class EdgeTypeSchema:
    """Schema for an edge type."""

    edge_id: int
    name: str
    from_type: int
    to_type: int
    props: list[FieldSchema] = field(default_factory=list)
    description: str = ""

    def validate(self, node_type_ids: set[int]) -> list[str]:
        """Validate the edge type schema."""
        errors = []
        if self.edge_id <= 0:
            errors.append(f"Edge type '{self.name}': edge_id must be positive")
        if not self.name:
            errors.append("Edge type name is required")
        if self.from_type not in node_type_ids:
            errors.append(f"Edge type '{self.name}': from_type {self.from_type} not found")
        if self.to_type not in node_type_ids:
            errors.append(f"Edge type '{self.name}': to_type {self.to_type} not found")

        for p in self.props:
            errors.extend(p.validate())

        return errors

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        d: dict[str, Any] = {
            "edge_id": self.edge_id,
            "name": self.name,
            "from_type": self.from_type,
            "to_type": self.to_type,
        }
        if self.props:
            d["props"] = [p.to_dict() for p in self.props]
        if self.description:
            d["description"] = self.description
        return d


@dataclass
class DataOperation:
    """A data operation (create/update/delete)."""

    operation: str  # create_node, update_node, delete_node, create_edge, delete_edge
    type_id: int | None = None
    edge_id: int | None = None
    node_id: str | None = None
    payload: dict[str, Any] | None = None
    from_id: str | None = None
    to_id: str | None = None
    props: dict[str, Any] | None = None
    as_alias: str | None = None

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        if self.operation == "create_node":
            d: dict[str, Any] = {
                "create_node": {
                    "type_id": self.type_id,
                    "payload": self.payload or {},
                }
            }
            if self.as_alias:
                d["create_node"]["as"] = self.as_alias
            return d
        elif self.operation == "update_node":
            return {
                "update_node": {
                    "type_id": self.type_id,
                    "node_id": self.node_id,
                    "patch": self.payload or {},
                }
            }
        elif self.operation == "delete_node":
            return {
                "delete_node": {
                    "type_id": self.type_id,
                    "node_id": self.node_id,
                }
            }
        elif self.operation == "create_edge":
            return {
                "create_edge": {
                    "edge_id": self.edge_id,
                    "from_id": self.from_id,
                    "to_id": self.to_id,
                    "props": self.props or {},
                }
            }
        elif self.operation == "delete_edge":
            return {
                "delete_edge": {
                    "edge_id": self.edge_id,
                    "from_id": self.from_id,
                    "to_id": self.to_id,
                }
            }
        return {}


@dataclass
class PlaygroundSchema:
    """Complete playground schema."""

    version: int = 1
    tenant: str = "playground"
    node_types: list[NodeTypeSchema] = field(default_factory=list)
    edge_types: list[EdgeTypeSchema] = field(default_factory=list)
    data: list[DataOperation] = field(default_factory=list)

    def validate(self) -> list[str]:
        """Validate the entire schema."""
        errors = []

        # Check for duplicate type IDs
        type_ids = [t.type_id for t in self.node_types]
        if len(type_ids) != len(set(type_ids)):
            errors.append("Duplicate node type IDs found")

        # Check for duplicate edge IDs
        edge_ids = [e.edge_id for e in self.edge_types]
        if len(edge_ids) != len(set(edge_ids)):
            errors.append("Duplicate edge type IDs found")

        # Validate node types
        for nt in self.node_types:
            errors.extend(nt.validate())

        # Validate edge types
        type_id_set = set(type_ids)
        for et in self.edge_types:
            errors.extend(et.validate(type_id_set))

        return errors

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary."""
        d: dict[str, Any] = {
            "version": self.version,
            "tenant": self.tenant,
            "node_types": [nt.to_dict() for nt in self.node_types],
            "edge_types": [et.to_dict() for et in self.edge_types],
        }
        if self.data:
            d["data"] = [op.to_dict() for op in self.data]
        return d

    def to_yaml(self) -> str:
        """Convert to YAML string."""
        return yaml.dump(self.to_dict(), default_flow_style=False, sort_keys=False)

    def to_json(self) -> str:
        """Convert to JSON string."""
        return json.dumps(self.to_dict(), indent=2)


def parse_field(data: dict[str, Any]) -> FieldSchema:
    """Parse a field from dict."""
    return FieldSchema(
        id=data.get("id", 0),
        name=data.get("name", ""),
        kind=data.get("kind", "str"),
        required=data.get("required", False),
        default=data.get("default"),
        values=data.get("values"),
        indexed=data.get("indexed", False),
        searchable=data.get("searchable", False),
    )


def parse_node_type(data: dict[str, Any]) -> NodeTypeSchema:
    """Parse a node type from dict."""
    fields = [parse_field(f) for f in data.get("fields", [])]
    return NodeTypeSchema(
        type_id=data.get("type_id", 0),
        name=data.get("name", ""),
        fields=fields,
        description=data.get("description", ""),
    )


def parse_edge_type(data: dict[str, Any]) -> EdgeTypeSchema:
    """Parse an edge type from dict."""
    props = [parse_field(p) for p in data.get("props", [])]
    return EdgeTypeSchema(
        edge_id=data.get("edge_id", 0),
        name=data.get("name", ""),
        from_type=data.get("from_type", 0),
        to_type=data.get("to_type", 0),
        props=props,
        description=data.get("description", ""),
    )


def parse_data_operation(data: dict[str, Any]) -> DataOperation:
    """Parse a data operation from dict."""
    if "create_node" in data:
        c = data["create_node"]
        return DataOperation(
            operation="create_node",
            type_id=c.get("type_id"),
            payload=c.get("payload"),
            as_alias=c.get("as"),
        )
    elif "update_node" in data:
        u = data["update_node"]
        return DataOperation(
            operation="update_node",
            type_id=u.get("type_id"),
            node_id=u.get("node_id"),
            payload=u.get("patch"),
        )
    elif "delete_node" in data:
        d = data["delete_node"]
        return DataOperation(
            operation="delete_node",
            type_id=d.get("type_id"),
            node_id=d.get("node_id"),
        )
    elif "create_edge" in data:
        e = data["create_edge"]
        return DataOperation(
            operation="create_edge",
            edge_id=e.get("edge_id"),
            from_id=e.get("from_id"),
            to_id=e.get("to_id"),
            props=e.get("props"),
        )
    elif "delete_edge" in data:
        e = data["delete_edge"]
        return DataOperation(
            operation="delete_edge",
            edge_id=e.get("edge_id"),
            from_id=e.get("from_id"),
            to_id=e.get("to_id"),
        )
    return DataOperation(operation="unknown")


def parse_schema(data: dict[str, Any]) -> PlaygroundSchema:
    """Parse a complete schema from dict."""
    node_types = [parse_node_type(nt) for nt in data.get("node_types", [])]
    edge_types = [parse_edge_type(et) for et in data.get("edge_types", [])]
    operations = [parse_data_operation(op) for op in data.get("data", [])]

    return PlaygroundSchema(
        version=data.get("version", 1),
        tenant=data.get("tenant", "playground"),
        node_types=node_types,
        edge_types=edge_types,
        data=operations,
    )


def parse_yaml(yaml_str: str) -> PlaygroundSchema:
    """Parse schema from YAML string."""
    data = yaml.safe_load(yaml_str)
    return parse_schema(data or {})


def parse_json(json_str: str) -> PlaygroundSchema:
    """Parse schema from JSON string."""
    data = json.loads(json_str)
    return parse_schema(data or {})


# AI Prompt Template
AI_PROMPT_TEMPLATE = """Generate an EntDB schema in YAML format for the following requirements:

{requirements}

The schema should follow this format:

```yaml
version: 1
tenant: playground

node_types:
  - type_id: <unique positive integer>
    name: <PascalCase name>
    description: <optional description>
    fields:
      - id: <unique positive integer within type>
        name: <snake_case field name>
        kind: <str|int|float|bool|timestamp|enum|json>
        required: <true|false>
        searchable: <true|false for text search>
        values: [<enum values if kind is enum>]
        default: <default value>

edge_types:
  - edge_id: <unique positive integer>
    name: <UPPER_SNAKE_CASE relationship name>
    from_type: <source node type_id>
    to_type: <target node type_id>
    props:
      - id: <field id>
        name: <field name>
        kind: <field type>

data:  # Optional seed data
  - create_node:
      type_id: <node type id>
      payload:
        <field>: <value>
```

Rules:
- type_id and edge_id must be unique positive integers
- field id must be unique within each type
- kind must be one of: str, int, float, bool, timestamp, enum, json, bytes, ref
- For enum fields, provide 'values' as a list
- Edge from_type and to_type must reference existing node type_ids
- Use descriptive names (Product not P, CustomerOrder not CO)

Generate the complete YAML schema:
"""
