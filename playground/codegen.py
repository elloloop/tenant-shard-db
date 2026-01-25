"""
Code generation from PlaygroundSchema.

Generates Python and Go SDK code from a YAML/JSON schema.
The schema is the source of truth - code is always derived.
"""

from __future__ import annotations

from .schema_format import PlaygroundSchema, NodeTypeSchema, EdgeTypeSchema, FieldSchema


def generate_python_schema(schema: PlaygroundSchema) -> str:
    """Generate Python SDK schema code."""
    lines = [
        '"""',
        "EntDB Schema Definition",
        "",
        "Auto-generated from playground schema.",
        "Do not edit directly - modify the YAML/JSON schema instead.",
        '"""',
        "",
        "from entdb_sdk import NodeTypeDef, EdgeTypeDef, field",
        "",
        "",
        "# =============================================================================",
        "# Node Types",
        "# =============================================================================",
        "",
    ]

    # Generate node types
    for nt in schema.node_types:
        lines.extend(_generate_python_node_type(nt))
        lines.append("")

    lines.extend([
        "",
        "# =============================================================================",
        "# Edge Types",
        "# =============================================================================",
        "",
    ])

    # Generate edge types
    node_name_map = {nt.type_id: nt.name for nt in schema.node_types}
    for et in schema.edge_types:
        lines.extend(_generate_python_edge_type(et, node_name_map))
        lines.append("")

    # Generate registry lists
    lines.extend([
        "",
        "# =============================================================================",
        "# Registry",
        "# =============================================================================",
        "",
        "ALL_NODE_TYPES = [",
    ])
    for nt in schema.node_types:
        lines.append(f"    {nt.name},")
    lines.append("]")
    lines.append("")

    lines.append("ALL_EDGE_TYPES = [")
    for et in schema.edge_types:
        lines.append(f"    {et.name},")
    lines.append("]")

    return "\n".join(lines)


def _generate_python_node_type(nt: NodeTypeSchema) -> list[str]:
    """Generate Python code for a node type."""
    lines = [f"{nt.name} = NodeTypeDef("]
    lines.append(f"    type_id={nt.type_id},")
    lines.append(f'    name="{nt.name}",')

    if nt.description:
        lines.append(f'    description="{nt.description}",')

    lines.append("    fields=(")
    for f in nt.fields:
        lines.append(f"        {_generate_python_field(f)},")
    lines.append("    ),")
    lines.append(")")

    return lines


def _generate_python_field(f: FieldSchema) -> str:
    """Generate Python field() call."""
    args = [str(f.id), f'"{f.name}"', f'"{f.kind}"']

    kwargs = []
    if f.required:
        kwargs.append("required=True")
    if f.default is not None:
        if isinstance(f.default, str):
            kwargs.append(f'default="{f.default}"')
        else:
            kwargs.append(f"default={f.default}")
    if f.values:
        values_str = ", ".join(f'"{v}"' for v in f.values)
        kwargs.append(f"enum_values=({values_str})")
    if f.indexed:
        kwargs.append("indexed=True")
    if f.searchable:
        kwargs.append("searchable=True")

    all_args = args + kwargs
    return f"field({', '.join(all_args)})"


def _generate_python_edge_type(et: EdgeTypeSchema, node_map: dict[int, str]) -> list[str]:
    """Generate Python code for an edge type."""
    from_name = node_map.get(et.from_type, str(et.from_type))
    to_name = node_map.get(et.to_type, str(et.to_type))

    lines = [f"{et.name} = EdgeTypeDef("]
    lines.append(f"    edge_id={et.edge_id},")
    lines.append(f'    name="{et.name}",')
    lines.append(f"    from_type={from_name},")
    lines.append(f"    to_type={to_name},")

    if et.description:
        lines.append(f'    description="{et.description}",')

    if et.props:
        lines.append("    props=(")
        for p in et.props:
            lines.append(f"        {_generate_python_field(p)},")
        lines.append("    ),")

    lines.append(")")
    return lines


def generate_python_usage(schema: PlaygroundSchema) -> str:
    """Generate Python usage example code."""
    if not schema.node_types:
        return "# No node types defined"

    nt = schema.node_types[0]
    example_payload = {}
    for f in nt.fields[:3]:  # First 3 fields
        if f.kind == "str":
            example_payload[f.name] = f"example_{f.name}"
        elif f.kind == "int":
            example_payload[f.name] = 42
        elif f.kind == "float":
            example_payload[f.name] = 3.14
        elif f.kind == "bool":
            example_payload[f.name] = True
        elif f.kind == "enum" and f.values:
            example_payload[f.name] = f.values[0]

    payload_str = ", ".join(f'{k}="{v}"' if isinstance(v, str) else f"{k}={v}"
                           for k, v in example_payload.items())

    return f'''from entdb_sdk import DbClient
from schema import {nt.name}

async with DbClient("localhost:50051") as db:
    # Create a {nt.name}
    plan = db.atomic("{schema.tenant}", "user:you")
    plan.create({nt.name}, {{{payload_str}}})
    result = await plan.commit()

    print(f"Created: {{result.created_node_ids[0]}}")

    # Query {nt.name}s
    nodes = await db.query({nt.name}, "{schema.tenant}", "user:you")
    for node in nodes:
        print(node.payload)
'''


def generate_go_schema(schema: PlaygroundSchema) -> str:
    """Generate Go SDK schema code."""
    lines = [
        "// EntDB Schema Definition",
        "//",
        "// Auto-generated from playground schema.",
        "// Do not edit directly - modify the YAML/JSON schema instead.",
        "",
        "package schema",
        "",
        "import (",
        '\t"github.com/your-org/entdb-go/sdk"',
        ")",
        "",
        "// =============================================================================",
        "// Node Types",
        "// =============================================================================",
        "",
    ]

    # Generate node types
    for nt in schema.node_types:
        lines.extend(_generate_go_node_type(nt))
        lines.append("")

    lines.extend([
        "// =============================================================================",
        "// Edge Types",
        "// =============================================================================",
        "",
    ])

    # Generate edge types
    node_name_map = {nt.type_id: nt.name for nt in schema.node_types}
    for et in schema.edge_types:
        lines.extend(_generate_go_edge_type(et, node_name_map))
        lines.append("")

    return "\n".join(lines)


def _go_type(kind: str) -> str:
    """Map schema kind to Go type."""
    mapping = {
        "str": "string",
        "int": "int64",
        "float": "float64",
        "bool": "bool",
        "timestamp": "int64",
        "json": "map[string]any",
        "bytes": "[]byte",
        "enum": "string",
        "ref": "string",
        "list_str": "[]string",
        "list_int": "[]int64",
        "list_ref": "[]string",
    }
    return mapping.get(kind, "any")


def _generate_go_node_type(nt: NodeTypeSchema) -> list[str]:
    """Generate Go code for a node type."""
    lines = [
        f"// {nt.name} - {nt.description or 'Node type'}",
        f"var {nt.name} = sdk.NodeTypeDef{{",
        f"\tTypeID: {nt.type_id},",
        f'\tName:   "{nt.name}",',
        "\tFields: []sdk.FieldDef{",
    ]

    for f in nt.fields:
        lines.append(_generate_go_field(f))

    lines.append("\t},")
    lines.append("}")

    # Also generate a struct for type-safe payloads
    lines.append("")
    lines.append(f"// {nt.name}Payload is the typed payload for {nt.name}")
    lines.append(f"type {nt.name}Payload struct {{")
    for f in nt.fields:
        go_type = _go_type(f.kind)
        json_tag = f.name
        if not f.required:
            json_tag += ",omitempty"
        lines.append(f'\t{_pascal_case(f.name)} {go_type} `json:"{json_tag}"`')
    lines.append("}")

    return lines


def _generate_go_field(f: FieldSchema) -> str:
    """Generate Go FieldDef."""
    parts = [
        f"\t\t{{FieldID: {f.id}",
        f'Name: "{f.name}"',
        f'Kind: "{f.kind}"',
    ]
    if f.required:
        parts.append("Required: true")
    if f.default is not None:
        if isinstance(f.default, str):
            parts.append(f'Default: "{f.default}"')
        else:
            parts.append(f"Default: {f.default}")
    if f.values:
        values_str = ", ".join(f'"{v}"' for v in f.values)
        parts.append(f"EnumValues: []string{{{values_str}}}")

    return ", ".join(parts) + "},"


def _generate_go_edge_type(et: EdgeTypeSchema, node_map: dict[int, str]) -> list[str]:
    """Generate Go code for an edge type."""
    from_name = node_map.get(et.from_type, str(et.from_type))
    to_name = node_map.get(et.to_type, str(et.to_type))

    lines = [
        f"// {et.name} - {et.description or 'Edge type'}",
        f"var {et.name} = sdk.EdgeTypeDef{{",
        f"\tEdgeID:   {et.edge_id},",
        f'\tName:     "{et.name}",',
        f"\tFromType: {from_name}.TypeID,",
        f"\tToType:   {to_name}.TypeID,",
    ]

    if et.props:
        lines.append("\tProps: []sdk.FieldDef{")
        for p in et.props:
            lines.append(_generate_go_field(p))
        lines.append("\t},")

    lines.append("}")
    return lines


def generate_go_usage(schema: PlaygroundSchema) -> str:
    """Generate Go usage example code."""
    if not schema.node_types:
        return "// No node types defined"

    nt = schema.node_types[0]
    example_fields = []
    for f in nt.fields[:3]:
        pascal = _pascal_case(f.name)
        if f.kind == "str":
            example_fields.append(f'{pascal}: "example"')
        elif f.kind == "int":
            example_fields.append(f"{pascal}: 42")
        elif f.kind == "float":
            example_fields.append(f"{pascal}: 3.14")
        elif f.kind == "bool":
            example_fields.append(f"{pascal}: true")
        elif f.kind == "enum" and f.values:
            example_fields.append(f'{pascal}: "{f.values[0]}"')

    fields_str = ", ".join(example_fields)

    return f'''package main

import (
\t"context"
\t"fmt"

\t"github.com/your-org/entdb-go/sdk"
\t"your-project/schema"
)

func main() {{
\tclient, err := sdk.Connect("localhost:50051")
\tif err != nil {{
\t\tpanic(err)
\t}}
\tdefer client.Close()

\tctx := context.Background()

\t// Create a {nt.name}
\tplan := client.Atomic("{schema.tenant}", "user:you")
\tplan.Create(schema.{nt.name}, schema.{nt.name}Payload{{
\t\t{fields_str},
\t}})

\tresult, err := plan.Commit(ctx)
\tif err != nil {{
\t\tpanic(err)
\t}}

\tfmt.Printf("Created: %s\\n", result.CreatedNodeIDs[0])
}}
'''


def _pascal_case(name: str) -> str:
    """Convert snake_case to PascalCase."""
    return "".join(word.capitalize() for word in name.split("_"))


def generate_data_python(schema: PlaygroundSchema) -> str:
    """Generate Python code for data operations."""
    if not schema.data:
        return "# No data operations defined"

    lines = [
        "from entdb_sdk import DbClient",
        "from schema import *  # Import all types",
        "",
        "",
        f'async with DbClient("localhost:50051") as db:',
        f'    plan = db.atomic("{schema.tenant}", "user:you")',
        "",
    ]

    node_map = {nt.type_id: nt.name for nt in schema.node_types}
    edge_map = {et.edge_id: et.name for et in schema.edge_types}

    for op in schema.data:
        if op.operation == "create_node":
            type_name = node_map.get(op.type_id, f"Type_{op.type_id}")
            payload_str = ", ".join(
                f'{k}="{v}"' if isinstance(v, str) else f"{k}={v}"
                for k, v in (op.payload or {}).items()
            )
            alias_arg = f', as_="{op.as_alias}"' if op.as_alias else ""
            lines.append(f"    plan.create({type_name}, {{{payload_str}}}{alias_arg})")

        elif op.operation == "update_node":
            type_name = node_map.get(op.type_id, f"Type_{op.type_id}")
            payload_str = ", ".join(
                f'"{k}": "{v}"' if isinstance(v, str) else f'"{k}": {v}'
                for k, v in (op.payload or {}).items()
            )
            lines.append(f'    plan.update({type_name}, "{op.node_id}", {{{payload_str}}})')

        elif op.operation == "delete_node":
            type_name = node_map.get(op.type_id, f"Type_{op.type_id}")
            lines.append(f'    plan.delete({type_name}, "{op.node_id}")')

        elif op.operation == "create_edge":
            edge_name = edge_map.get(op.edge_id, f"Edge_{op.edge_id}")
            props_arg = ""
            if op.props:
                props_str = ", ".join(
                    f'"{k}": "{v}"' if isinstance(v, str) else f'"{k}": {v}'
                    for k, v in op.props.items()
                )
                props_arg = f", props={{{props_str}}}"
            lines.append(f'    plan.edge_create({edge_name}, from_="{op.from_id}", to="{op.to_id}"{props_arg})')

        elif op.operation == "delete_edge":
            edge_name = edge_map.get(op.edge_id, f"Edge_{op.edge_id}")
            lines.append(f'    plan.edge_delete({edge_name}, from_="{op.from_id}", to="{op.to_id}")')

    lines.extend([
        "",
        "    result = await plan.commit()",
        '    print(f"Success: {result.success}")',
        '    print(f"Created: {result.created_node_ids}")',
    ])

    return "\n".join(lines)
