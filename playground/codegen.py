"""
SDK code generator for Playground.

Generates equivalent Python SDK code for each operation,
helping developers understand how to use the SDK.
"""

import json
from typing import Any

from .schema import get_node_type, get_edge_type


def generate_create_node_code(
    type_id: int,
    payload: dict[str, Any],
    node_id: str | None = None,
) -> str:
    """Generate SDK code for creating a node."""
    node_type = get_node_type(type_id)
    type_name = node_type.name if node_type else f"NodeType_{type_id}"

    # Format payload as kwargs
    kwargs = ", ".join(f'{k}={json.dumps(v)}' for k, v in payload.items())

    code = f'''from entdb_sdk import DbClient
from your_schema import {type_name}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("your_tenant", "your_actor")
    plan.create({type_name}, {{{kwargs}}})
    result = await plan.commit()

    print(f"Created node: {{result.created_node_ids[0]}}")'''

    return code


def generate_update_node_code(
    type_id: int,
    node_id: str,
    patch: dict[str, Any],
) -> str:
    """Generate SDK code for updating a node."""
    node_type = get_node_type(type_id)
    type_name = node_type.name if node_type else f"NodeType_{type_id}"

    patch_str = ", ".join(f'"{k}": {json.dumps(v)}' for k, v in patch.items())

    code = f'''from entdb_sdk import DbClient
from your_schema import {type_name}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("your_tenant", "your_actor")
    plan.update({type_name}, "{node_id}", {{{patch_str}}})
    result = await plan.commit()

    print("Node updated successfully")'''

    return code


def generate_delete_node_code(
    type_id: int,
    node_id: str,
) -> str:
    """Generate SDK code for deleting a node."""
    node_type = get_node_type(type_id)
    type_name = node_type.name if node_type else f"NodeType_{type_id}"

    code = f'''from entdb_sdk import DbClient
from your_schema import {type_name}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("your_tenant", "your_actor")
    plan.delete({type_name}, "{node_id}")
    result = await plan.commit()

    print("Node deleted successfully")'''

    return code


def generate_create_edge_code(
    edge_id: int,
    from_id: str,
    to_id: str,
    props: dict[str, Any] | None = None,
) -> str:
    """Generate SDK code for creating an edge."""
    edge_type = get_edge_type(edge_id)
    edge_name = edge_type.name if edge_type else f"EdgeType_{edge_id}"

    props_arg = ""
    if props:
        props_str = ", ".join(f'"{k}": {json.dumps(v)}' for k, v in props.items())
        props_arg = f", props={{{props_str}}}"

    code = f'''from entdb_sdk import DbClient
from your_schema import {edge_name}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("your_tenant", "your_actor")
    plan.edge_create({edge_name}, from_="{from_id}", to="{to_id}"{props_arg})
    result = await plan.commit()

    print("Edge created successfully")'''

    return code


def generate_delete_edge_code(
    edge_id: int,
    from_id: str,
    to_id: str,
) -> str:
    """Generate SDK code for deleting an edge."""
    edge_type = get_edge_type(edge_id)
    edge_name = edge_type.name if edge_type else f"EdgeType_{edge_id}"

    code = f'''from entdb_sdk import DbClient
from your_schema import {edge_name}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("your_tenant", "your_actor")
    plan.edge_delete({edge_name}, from_="{from_id}", to="{to_id}")
    result = await plan.commit()

    print("Edge deleted successfully")'''

    return code


def generate_atomic_code(operations: list[dict[str, Any]]) -> str:
    """Generate SDK code for an atomic transaction with multiple operations."""
    lines = [
        "from entdb_sdk import DbClient",
        "from your_schema import User, Project, Task, Comment",
        "from your_schema import Owns, Contains, AssignedTo",
        "",
        'async with DbClient("localhost:50051") as db:',
        '    plan = db.atomic("your_tenant", "your_actor")',
        "",
    ]

    for op in operations:
        if "create_node" in op:
            c = op["create_node"]
            type_id = c.get("type_id")
            node_type = get_node_type(type_id)
            type_name = node_type.name if node_type else f"Type_{type_id}"
            payload = c.get("payload", {})
            kwargs = ", ".join(f'{k}={json.dumps(v)}' for k, v in payload.items())
            alias = c.get("as_")
            alias_arg = f', as_="{alias}"' if alias else ""
            lines.append(f"    plan.create({type_name}, {{{kwargs}}}{alias_arg})")

        elif "update_node" in op:
            u = op["update_node"]
            type_id = u.get("type_id")
            node_type = get_node_type(type_id)
            type_name = node_type.name if node_type else f"Type_{type_id}"
            node_id = u.get("node_id")
            patch = u.get("patch", {})
            patch_str = ", ".join(f'"{k}": {json.dumps(v)}' for k, v in patch.items())
            lines.append(f'    plan.update({type_name}, "{node_id}", {{{patch_str}}})')

        elif "delete_node" in op:
            d = op["delete_node"]
            type_id = d.get("type_id")
            node_type = get_node_type(type_id)
            type_name = node_type.name if node_type else f"Type_{type_id}"
            node_id = d.get("node_id")
            lines.append(f'    plan.delete({type_name}, "{node_id}")')

        elif "create_edge" in op:
            e = op["create_edge"]
            edge_id = e.get("edge_id")
            edge_type = get_edge_type(edge_id)
            edge_name = edge_type.name if edge_type else f"Edge_{edge_id}"
            from_id = e.get("from_id")
            to_id = e.get("to_id")
            lines.append(f'    plan.edge_create({edge_name}, from_="{from_id}", to="{to_id}")')

        elif "delete_edge" in op:
            e = op["delete_edge"]
            edge_id = e.get("edge_id")
            edge_type = get_edge_type(edge_id)
            edge_name = edge_type.name if edge_type else f"Edge_{edge_id}"
            from_id = e.get("from_id")
            to_id = e.get("to_id")
            lines.append(f'    plan.edge_delete({edge_name}, from_="{from_id}", to="{to_id}")')

    lines.extend([
        "",
        "    result = await plan.commit()",
        "    print(f\"Success: {result.success}\")",
        "    print(f\"Created nodes: {result.created_node_ids}\")",
    ])

    return "\n".join(lines)
