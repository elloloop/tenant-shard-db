"""
Playground API routes.

Provides write operations for experimenting with EntDB.
All data goes to a sandbox tenant that can be viewed in Console.

Supports two modes:
1. Demo mode: Use predefined schema (User, Project, Task, Comment)
2. Dynamic mode: Define custom schema via YAML/JSON, generate code

Each response includes the equivalent SDK code.
"""

import time
import logging
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, Field

from entdb_sdk import DbClient, NodeTypeDef, EdgeTypeDef, field as sdk_field

from .schema import (
    ALL_NODE_TYPES,
    ALL_EDGE_TYPES,
    get_node_type,
    get_edge_type,
)
from .schema_format import (
    PlaygroundSchema,
    parse_yaml,
    parse_json,
    AI_PROMPT_TEMPLATE,
)
from .codegen import (
    generate_python_schema,
    generate_go_schema,
    generate_python_usage,
    generate_go_usage,
    generate_data_python,
)

logger = logging.getLogger(__name__)

router = APIRouter(tags=["Playground"])


# =============================================================================
# Request/Response Models
# =============================================================================


class CreateNodeRequest(BaseModel):
    """Create a node (demo mode)."""
    type_id: int = Field(..., description="Node type ID (1=User, 2=Project, 3=Task, 4=Comment)")
    payload: dict[str, Any] = Field(..., description="Node data")


class UpdateNodeRequest(BaseModel):
    """Update a node (demo mode)."""
    type_id: int = Field(..., description="Node type ID")
    node_id: str = Field(..., description="Node ID to update")
    patch: dict[str, Any] = Field(..., description="Fields to update")


class DeleteNodeRequest(BaseModel):
    """Delete a node (demo mode)."""
    type_id: int = Field(..., description="Node type ID")
    node_id: str = Field(..., description="Node ID to delete")


class CreateEdgeRequest(BaseModel):
    """Create an edge (demo mode)."""
    edge_id: int = Field(..., description="Edge type ID")
    from_id: str = Field(..., description="Source node ID")
    to_id: str = Field(..., description="Target node ID")
    props: dict[str, Any] | None = Field(None, description="Edge properties")


class DeleteEdgeRequest(BaseModel):
    """Delete an edge (demo mode)."""
    edge_id: int = Field(..., description="Edge type ID")
    from_id: str = Field(..., description="Source node ID")
    to_id: str = Field(..., description="Target node ID")


class AtomicRequest(BaseModel):
    """Execute multiple operations atomically (demo mode)."""
    operations: list[dict[str, Any]] = Field(..., description="List of operations")


class PlaygroundResponse(BaseModel):
    """Response with result and SDK code."""
    success: bool
    message: str
    data: dict[str, Any] | None = None
    sdk_code: str = Field(..., description="Equivalent Python SDK code")


# --- Dynamic Schema Models ---


class SchemaParseRequest(BaseModel):
    """Request to parse and validate a schema."""
    content: str = Field(..., description="YAML or JSON schema content")
    format: str = Field(default="yaml", description="Format: 'yaml' or 'json'")


class SchemaParseResponse(BaseModel):
    """Response with parsed schema and generated code."""
    valid: bool
    errors: list[str] = Field(default_factory=list)
    schema_data: dict[str, Any] | None = None
    python_schema: str | None = None
    python_usage: str | None = None
    go_schema: str | None = None
    go_usage: str | None = None
    python_data: str | None = None


class SchemaExecuteRequest(BaseModel):
    """Request to parse schema and execute data operations."""
    content: str = Field(..., description="YAML or JSON schema content")
    format: str = Field(default="yaml", description="Format: 'yaml' or 'json'")
    execute_data: bool = Field(default=True, description="Execute data operations")


class SchemaExecuteResponse(BaseModel):
    """Response from schema execution."""
    success: bool
    message: str
    errors: list[str] = Field(default_factory=list)
    created_node_ids: list[str] = Field(default_factory=list)
    python_schema: str | None = None
    go_schema: str | None = None


class AIPromptRequest(BaseModel):
    """Request for AI prompt generation."""
    requirements: str = Field(..., description="Natural language requirements for the schema")


class AIPromptResponse(BaseModel):
    """Response with AI prompt."""
    prompt: str


# =============================================================================
# Dependencies
# =============================================================================


def get_db_client(request: Request) -> DbClient:
    """Get SDK client from app state."""
    return request.app.state.db_client


def get_settings(request: Request):
    """Get settings from app state."""
    return request.app.state.settings


# =============================================================================
# Dynamic Schema Endpoints
# =============================================================================


@router.post("/schema/parse", response_model=SchemaParseResponse)
async def parse_schema(request: SchemaParseRequest):
    """
    Parse and validate a YAML/JSON schema.

    Returns:
    - Validation errors (if any)
    - Parsed schema data
    - Generated Python SDK code
    - Generated Go SDK code
    - Example usage code

    This is the main endpoint for the YAML ↔ Code generation flow.
    """
    try:
        # Parse based on format
        if request.format.lower() == "json":
            schema = parse_json(request.content)
        else:
            schema = parse_yaml(request.content)

        # Validate
        errors = schema.validate()

        if errors:
            return SchemaParseResponse(
                valid=False,
                errors=errors,
                schema_data=schema.to_dict(),
            )

        # Generate code
        return SchemaParseResponse(
            valid=True,
            errors=[],
            schema_data=schema.to_dict(),
            python_schema=generate_python_schema(schema),
            python_usage=generate_python_usage(schema),
            go_schema=generate_go_schema(schema),
            go_usage=generate_go_usage(schema),
            python_data=generate_data_python(schema) if schema.data else None,
        )

    except Exception as e:
        logger.error(f"Schema parse error: {e}")
        return SchemaParseResponse(
            valid=False,
            errors=[f"Parse error: {str(e)}"],
        )


@router.post("/schema/execute", response_model=SchemaExecuteResponse)
async def execute_schema(
    request: SchemaExecuteRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Parse schema, register types, and execute data operations.

    This creates actual data in the playground tenant, visible in Console.
    The schema types are dynamically registered with the SDK.
    """
    try:
        # Parse
        if request.format.lower() == "json":
            schema = parse_json(request.content)
        else:
            schema = parse_yaml(request.content)

        # Validate
        errors = schema.validate()
        if errors:
            return SchemaExecuteResponse(
                success=False,
                message="Schema validation failed",
                errors=errors,
            )

        created_ids: list[str] = []

        if request.execute_data and schema.data:
            # Build dynamic node types from schema
            node_type_map: dict[int, NodeTypeDef] = {}
            edge_type_map: dict[int, EdgeTypeDef] = {}

            for nt in schema.node_types:
                fields = tuple(
                    sdk_field(
                        f.id,
                        f.name,
                        f.kind,
                        required=f.required,
                        default=f.default,
                        enum_values=tuple(f.values) if f.values else None,
                        indexed=f.indexed,
                        searchable=f.searchable,
                    )
                    for f in nt.fields
                )
                node_type = NodeTypeDef(
                    type_id=nt.type_id,
                    name=nt.name,
                    description=nt.description,
                    fields=fields,
                )
                node_type_map[nt.type_id] = node_type

            for et in schema.edge_types:
                from_type = node_type_map.get(et.from_type)
                to_type = node_type_map.get(et.to_type)
                if not from_type or not to_type:
                    continue

                props = tuple(
                    sdk_field(
                        p.id,
                        p.name,
                        p.kind,
                        required=p.required,
                        default=p.default,
                        enum_values=tuple(p.values) if p.values else None,
                    )
                    for p in et.props
                ) if et.props else ()

                edge_type = EdgeTypeDef(
                    edge_id=et.edge_id,
                    name=et.name,
                    description=et.description,
                    from_type=from_type,
                    to_type=to_type,
                    props=props,
                )
                edge_type_map[et.edge_id] = edge_type

            # Execute data operations
            plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)
            alias_map: dict[str, str] = {}  # alias -> created node id

            for op in schema.data:
                if op.operation == "create_node":
                    node_type = node_type_map.get(op.type_id)
                    if node_type:
                        payload = dict(op.payload or {})
                        # Add timestamp if field exists
                        field_names = [f.name for f in node_type.fields]
                        if "created_at" in field_names:
                            payload.setdefault("created_at", int(time.time() * 1000))
                        plan.create(node_type, payload, as_=op.as_alias)

                elif op.operation == "update_node":
                    node_type = node_type_map.get(op.type_id)
                    if node_type and op.node_id:
                        plan.update(node_type, op.node_id, op.payload or {})

                elif op.operation == "delete_node":
                    node_type = node_type_map.get(op.type_id)
                    if node_type and op.node_id:
                        plan.delete(node_type, op.node_id)

                elif op.operation == "create_edge":
                    edge_type = edge_type_map.get(op.edge_id)
                    if edge_type and op.from_id and op.to_id:
                        plan.edge_create(edge_type, from_=op.from_id, to=op.to_id, props=op.props)

                elif op.operation == "delete_edge":
                    edge_type = edge_type_map.get(op.edge_id)
                    if edge_type and op.from_id and op.to_id:
                        plan.edge_delete(edge_type, from_=op.from_id, to=op.to_id)

            # Commit
            result = await plan.commit(wait_applied=True)

            if not result.success:
                return SchemaExecuteResponse(
                    success=False,
                    message=f"Execution failed: {result.error}",
                    errors=[result.error] if result.error else [],
                )

            created_ids = list(result.created_node_ids) if result.created_node_ids else []

        return SchemaExecuteResponse(
            success=True,
            message=f"Schema processed. Created {len(created_ids)} nodes.",
            created_node_ids=created_ids,
            python_schema=generate_python_schema(schema),
            go_schema=generate_go_schema(schema),
        )

    except Exception as e:
        logger.error(f"Schema execute error: {e}")
        return SchemaExecuteResponse(
            success=False,
            message=f"Error: {str(e)}",
            errors=[str(e)],
        )


@router.get("/schema/prompt")
async def get_ai_prompt_template():
    """
    Get the AI prompt template for generating schemas.

    Copy this prompt and use it with any AI coding assistant to
    generate a valid YAML/JSON schema from natural language requirements.
    """
    return {
        "template": AI_PROMPT_TEMPLATE,
        "usage": "Replace {requirements} with your natural language description",
        "example_requirements": (
            "Create a simple blog system with users, posts, and comments. "
            "Users can author multiple posts. Posts can have many comments. "
            "Each comment belongs to a user."
        ),
    }


@router.post("/schema/prompt", response_model=AIPromptResponse)
async def generate_ai_prompt(request: AIPromptRequest):
    """
    Generate a ready-to-use AI prompt for your requirements.

    Paste the returned prompt into ChatGPT, Claude, or any AI assistant
    to get a valid YAML schema.
    """
    filled_prompt = AI_PROMPT_TEMPLATE.replace("{requirements}", request.requirements)
    return AIPromptResponse(prompt=filled_prompt)


# =============================================================================
# Demo Schema Info
# =============================================================================


@router.get("/schema")
async def get_demo_schema():
    """
    Get the demo playground schema.

    Returns the predefined schema (User, Project, Task, Comment)
    for experimenting with the basic playground functionality.

    For custom schemas, use POST /schema/parse with YAML/JSON.
    """
    return {
        "mode": "demo",
        "description": "Predefined schema for quick experimentation",
        "node_types": [
            {
                "type_id": t.type_id,
                "name": t.name,
                "description": t.description,
                "fields": [
                    {
                        "field_id": f.field_id,
                        "name": f.name,
                        "kind": f.kind.value,
                        "required": f.required,
                        "default": f.default,
                        "enum_values": list(f.enum_values) if f.enum_values else None,
                    }
                    for f in t.fields
                ],
            }
            for t in ALL_NODE_TYPES
        ],
        "edge_types": [
            {
                "edge_id": e.edge_id,
                "name": e.name,
                "description": e.description,
                "from_type_id": e.from_type_id,
                "to_type_id": e.to_type_id,
                "props": [
                    {
                        "field_id": p.field_id,
                        "name": p.name,
                        "kind": p.kind.value,
                        "enum_values": list(p.enum_values) if p.enum_values else None,
                    }
                    for p in e.props
                ],
            }
            for e in ALL_EDGE_TYPES
        ],
        "sandbox_tenant": "playground",
        "endpoints": {
            "custom_schema": "POST /api/v1/schema/parse - Parse custom YAML/JSON",
            "ai_prompt": "GET /api/v1/schema/prompt - Get AI prompt template",
        },
    }


# =============================================================================
# Demo Mode - Node Operations (predefined schema)
# =============================================================================


def _generate_demo_code(operation: str, **kwargs) -> str:
    """Generate SDK code for demo operations."""
    from .schema import NODE_TYPE_MAP, EDGE_TYPE_MAP

    if operation == "create_node":
        type_name = NODE_TYPE_MAP.get(kwargs["type_id"])
        type_str = type_name.name if type_name else f"Type_{kwargs['type_id']}"
        payload_str = ", ".join(
            f'{k}="{v}"' if isinstance(v, str) else f"{k}={v}"
            for k, v in kwargs.get("payload", {}).items()
        )
        return f'''from entdb_sdk import DbClient
from playground.schema import {type_str}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("playground", "user:you")
    plan.create({type_str}, {{{payload_str}}})
    result = await plan.commit()
    print(f"Created: {{result.created_node_ids[0]}}")
'''

    elif operation == "update_node":
        type_name = NODE_TYPE_MAP.get(kwargs["type_id"])
        type_str = type_name.name if type_name else f"Type_{kwargs['type_id']}"
        patch_str = ", ".join(
            f'"{k}": "{v}"' if isinstance(v, str) else f'"{k}": {v}'
            for k, v in kwargs.get("patch", {}).items()
        )
        return f'''from entdb_sdk import DbClient
from playground.schema import {type_str}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("playground", "user:you")
    plan.update({type_str}, "{kwargs['node_id']}", {{{patch_str}}})
    result = await plan.commit()
'''

    elif operation == "delete_node":
        type_name = NODE_TYPE_MAP.get(kwargs["type_id"])
        type_str = type_name.name if type_name else f"Type_{kwargs['type_id']}"
        return f'''from entdb_sdk import DbClient
from playground.schema import {type_str}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("playground", "user:you")
    plan.delete({type_str}, "{kwargs['node_id']}")
    result = await plan.commit()
'''

    elif operation == "create_edge":
        edge_name = EDGE_TYPE_MAP.get(kwargs["edge_id"])
        edge_str = edge_name.name if edge_name else f"Edge_{kwargs['edge_id']}"
        props_arg = ""
        if kwargs.get("props"):
            props_str = ", ".join(
                f'"{k}": "{v}"' if isinstance(v, str) else f'"{k}": {v}'
                for k, v in kwargs["props"].items()
            )
            props_arg = f", props={{{props_str}}}"
        return f'''from entdb_sdk import DbClient
from playground.schema import {edge_str}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("playground", "user:you")
    plan.edge_create({edge_str}, from_="{kwargs['from_id']}", to="{kwargs['to_id']}"{props_arg})
    result = await plan.commit()
'''

    elif operation == "delete_edge":
        edge_name = EDGE_TYPE_MAP.get(kwargs["edge_id"])
        edge_str = edge_name.name if edge_name else f"Edge_{kwargs['edge_id']}"
        return f'''from entdb_sdk import DbClient
from playground.schema import {edge_str}

async with DbClient("localhost:50051") as db:
    plan = db.atomic("playground", "user:you")
    plan.edge_delete({edge_str}, from_="{kwargs['from_id']}", to="{kwargs['to_id']}")
    result = await plan.commit()
'''

    return "# Unknown operation"


@router.post("/nodes", response_model=PlaygroundResponse)
async def create_node(
    request: CreateNodeRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Create a node in the sandbox (demo mode).

    Uses the predefined schema. For custom schemas, use /schema/execute.
    """
    node_type = get_node_type(request.type_id)
    if not node_type:
        raise HTTPException(status_code=400, detail=f"Unknown type_id: {request.type_id}")

    payload = dict(request.payload)
    if "created_at" in [f.name for f in node_type.fields]:
        payload.setdefault("created_at", int(time.time() * 1000))

    try:
        plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)
        plan.create(node_type, payload)
        result = await plan.commit(wait_applied=True)

        if not result.success:
            raise HTTPException(status_code=400, detail=result.error)

        return PlaygroundResponse(
            success=True,
            message=f"Created {node_type.name} node",
            data={
                "node_id": result.created_node_ids[0] if result.created_node_ids else None,
                "type": node_type.name,
                "payload": payload,
            },
            sdk_code=_generate_demo_code("create_node", type_id=request.type_id, payload=payload),
        )

    except Exception as e:
        logger.error(f"Create node failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.patch("/nodes", response_model=PlaygroundResponse)
async def update_node(
    request: UpdateNodeRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """Update a node in the sandbox (demo mode)."""
    node_type = get_node_type(request.type_id)
    if not node_type:
        raise HTTPException(status_code=400, detail=f"Unknown type_id: {request.type_id}")

    try:
        plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)
        plan.update(node_type, request.node_id, request.patch)
        result = await plan.commit(wait_applied=True)

        if not result.success:
            raise HTTPException(status_code=400, detail=result.error)

        return PlaygroundResponse(
            success=True,
            message=f"Updated {node_type.name} node",
            data={"node_id": request.node_id, "patch": request.patch},
            sdk_code=_generate_demo_code(
                "update_node", type_id=request.type_id, node_id=request.node_id, patch=request.patch
            ),
        )

    except Exception as e:
        logger.error(f"Update node failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/nodes", response_model=PlaygroundResponse)
async def delete_node(
    request: DeleteNodeRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """Delete a node from the sandbox (demo mode)."""
    node_type = get_node_type(request.type_id)
    if not node_type:
        raise HTTPException(status_code=400, detail=f"Unknown type_id: {request.type_id}")

    try:
        plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)
        plan.delete(node_type, request.node_id)
        result = await plan.commit(wait_applied=True)

        if not result.success:
            raise HTTPException(status_code=400, detail=result.error)

        return PlaygroundResponse(
            success=True,
            message=f"Deleted {node_type.name} node",
            data={"node_id": request.node_id},
            sdk_code=_generate_demo_code("delete_node", type_id=request.type_id, node_id=request.node_id),
        )

    except Exception as e:
        logger.error(f"Delete node failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# =============================================================================
# Demo Mode - Edge Operations (predefined schema)
# =============================================================================


@router.post("/edges", response_model=PlaygroundResponse)
async def create_edge(
    request: CreateEdgeRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """Create an edge in the sandbox (demo mode)."""
    edge_type = get_edge_type(request.edge_id)
    if not edge_type:
        raise HTTPException(status_code=400, detail=f"Unknown edge_id: {request.edge_id}")

    try:
        plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)
        plan.edge_create(edge_type, from_=request.from_id, to=request.to_id, props=request.props)
        result = await plan.commit(wait_applied=True)

        if not result.success:
            raise HTTPException(status_code=400, detail=result.error)

        return PlaygroundResponse(
            success=True,
            message=f"Created {edge_type.name} edge",
            data={
                "edge_type": edge_type.name,
                "from_id": request.from_id,
                "to_id": request.to_id,
                "props": request.props,
            },
            sdk_code=_generate_demo_code(
                "create_edge",
                edge_id=request.edge_id,
                from_id=request.from_id,
                to_id=request.to_id,
                props=request.props,
            ),
        )

    except Exception as e:
        logger.error(f"Create edge failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/edges", response_model=PlaygroundResponse)
async def delete_edge(
    request: DeleteEdgeRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """Delete an edge from the sandbox (demo mode)."""
    edge_type = get_edge_type(request.edge_id)
    if not edge_type:
        raise HTTPException(status_code=400, detail=f"Unknown edge_id: {request.edge_id}")

    try:
        plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)
        plan.edge_delete(edge_type, from_=request.from_id, to=request.to_id)
        result = await plan.commit(wait_applied=True)

        if not result.success:
            raise HTTPException(status_code=400, detail=result.error)

        return PlaygroundResponse(
            success=True,
            message=f"Deleted {edge_type.name} edge",
            data={
                "edge_type": edge_type.name,
                "from_id": request.from_id,
                "to_id": request.to_id,
            },
            sdk_code=_generate_demo_code(
                "delete_edge",
                edge_id=request.edge_id,
                from_id=request.from_id,
                to_id=request.to_id,
            ),
        )

    except Exception as e:
        logger.error(f"Delete edge failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# =============================================================================
# Demo Mode - Atomic Transactions
# =============================================================================


@router.post("/atomic", response_model=PlaygroundResponse)
async def execute_atomic(
    request: AtomicRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Execute multiple operations atomically (demo mode).

    Supports:
    - {"create_node": {"type_id": 1, "payload": {...}}}
    - {"update_node": {"type_id": 1, "node_id": "...", "patch": {...}}}
    - {"delete_node": {"type_id": 1, "node_id": "..."}}
    - {"create_edge": {"edge_id": 101, "from_id": "...", "to_id": "..."}}
    - {"delete_edge": {"edge_id": 101, "from_id": "...", "to_id": "..."}}
    """
    if not request.operations:
        raise HTTPException(status_code=400, detail="No operations provided")

    try:
        plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)

        for op in request.operations:
            if "create_node" in op:
                c = op["create_node"]
                node_type = get_node_type(c["type_id"])
                if not node_type:
                    raise HTTPException(status_code=400, detail=f"Unknown type_id: {c['type_id']}")
                payload = c.get("payload", {})
                if "created_at" in [f.name for f in node_type.fields]:
                    payload.setdefault("created_at", int(time.time() * 1000))
                plan.create(node_type, payload, as_=c.get("as_"))

            elif "update_node" in op:
                u = op["update_node"]
                node_type = get_node_type(u["type_id"])
                if not node_type:
                    raise HTTPException(status_code=400, detail=f"Unknown type_id: {u['type_id']}")
                plan.update(node_type, u["node_id"], u.get("patch", {}))

            elif "delete_node" in op:
                d = op["delete_node"]
                node_type = get_node_type(d["type_id"])
                if not node_type:
                    raise HTTPException(status_code=400, detail=f"Unknown type_id: {d['type_id']}")
                plan.delete(node_type, d["node_id"])

            elif "create_edge" in op:
                e = op["create_edge"]
                edge_type = get_edge_type(e["edge_id"])
                if not edge_type:
                    raise HTTPException(status_code=400, detail=f"Unknown edge_id: {e['edge_id']}")
                plan.edge_create(edge_type, from_=e["from_id"], to=e["to_id"], props=e.get("props"))

            elif "delete_edge" in op:
                e = op["delete_edge"]
                edge_type = get_edge_type(e["edge_id"])
                if not edge_type:
                    raise HTTPException(status_code=400, detail=f"Unknown edge_id: {e['edge_id']}")
                plan.edge_delete(edge_type, from_=e["from_id"], to=e["to_id"])

            else:
                raise HTTPException(status_code=400, detail=f"Unknown operation: {list(op.keys())}")

        result = await plan.commit(wait_applied=True)

        if not result.success:
            raise HTTPException(status_code=400, detail=result.error)

        # Generate atomic code
        code_lines = [
            "from entdb_sdk import DbClient",
            "from playground.schema import *",
            "",
            'async with DbClient("localhost:50051") as db:',
            '    plan = db.atomic("playground", "user:you")',
            "",
        ]
        for op in request.operations:
            if "create_node" in op:
                c = op["create_node"]
                type_name = get_node_type(c["type_id"])
                type_str = type_name.name if type_name else f"Type_{c['type_id']}"
                code_lines.append(f"    plan.create({type_str}, {c.get('payload', {})})")
            elif "update_node" in op:
                u = op["update_node"]
                type_name = get_node_type(u["type_id"])
                type_str = type_name.name if type_name else f"Type_{u['type_id']}"
                code_lines.append(f'    plan.update({type_str}, "{u["node_id"]}", {u.get("patch", {})})')
            elif "delete_node" in op:
                d = op["delete_node"]
                type_name = get_node_type(d["type_id"])
                type_str = type_name.name if type_name else f"Type_{d['type_id']}"
                code_lines.append(f'    plan.delete({type_str}, "{d["node_id"]}")')
            elif "create_edge" in op:
                e = op["create_edge"]
                edge_name = get_edge_type(e["edge_id"])
                edge_str = edge_name.name if edge_name else f"Edge_{e['edge_id']}"
                code_lines.append(f'    plan.edge_create({edge_str}, from_="{e["from_id"]}", to="{e["to_id"]}")')
            elif "delete_edge" in op:
                e = op["delete_edge"]
                edge_name = get_edge_type(e["edge_id"])
                edge_str = edge_name.name if edge_name else f"Edge_{e['edge_id']}"
                code_lines.append(f'    plan.edge_delete({edge_str}, from_="{e["from_id"]}", to="{e["to_id"]}")')

        code_lines.extend([
            "",
            "    result = await plan.commit()",
            '    print(f"Created: {result.created_node_ids}")',
        ])

        return PlaygroundResponse(
            success=True,
            message=f"Executed {len(request.operations)} operations atomically",
            data={
                "created_node_ids": list(result.created_node_ids) if result.created_node_ids else [],
                "operation_count": len(request.operations),
            },
            sdk_code="\n".join(code_lines),
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Atomic transaction failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# =============================================================================
# Cleanup
# =============================================================================


@router.post("/reset", response_model=PlaygroundResponse)
async def reset_sandbox(
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Delete all data in the sandbox tenant.

    Warning: This deletes everything in the playground tenant!
    """
    return PlaygroundResponse(
        success=True,
        message="Sandbox reset not yet implemented. Nodes are deleted after TTL.",
        data={"tenant": settings.sandbox_tenant},
        sdk_code="# Sandbox cleanup is handled automatically by TTL",
    )
