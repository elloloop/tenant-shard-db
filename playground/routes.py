"""
Playground API routes.

Provides write operations for experimenting with EntDB.
All data goes to a sandbox tenant that can be viewed in Console.
Each response includes the equivalent SDK code.
"""

import time
import logging
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, Field

from entdb_sdk import DbClient

from .schema import (
    ALL_NODE_TYPES,
    ALL_EDGE_TYPES,
    get_node_type,
    get_edge_type,
)
from .codegen import (
    generate_create_node_code,
    generate_update_node_code,
    generate_delete_node_code,
    generate_create_edge_code,
    generate_delete_edge_code,
    generate_atomic_code,
)

logger = logging.getLogger(__name__)

router = APIRouter(tags=["Playground"])


# --- Request/Response Models ---


class CreateNodeRequest(BaseModel):
    """Create a node."""
    type_id: int = Field(..., description="Node type ID (1=User, 2=Project, 3=Task, 4=Comment)")
    payload: dict[str, Any] = Field(..., description="Node data")


class UpdateNodeRequest(BaseModel):
    """Update a node."""
    type_id: int = Field(..., description="Node type ID")
    node_id: str = Field(..., description="Node ID to update")
    patch: dict[str, Any] = Field(..., description="Fields to update")


class DeleteNodeRequest(BaseModel):
    """Delete a node."""
    type_id: int = Field(..., description="Node type ID")
    node_id: str = Field(..., description="Node ID to delete")


class CreateEdgeRequest(BaseModel):
    """Create an edge."""
    edge_id: int = Field(..., description="Edge type ID")
    from_id: str = Field(..., description="Source node ID")
    to_id: str = Field(..., description="Target node ID")
    props: dict[str, Any] | None = Field(None, description="Edge properties")


class DeleteEdgeRequest(BaseModel):
    """Delete an edge."""
    edge_id: int = Field(..., description="Edge type ID")
    from_id: str = Field(..., description="Source node ID")
    to_id: str = Field(..., description="Target node ID")


class AtomicRequest(BaseModel):
    """Execute multiple operations atomically."""
    operations: list[dict[str, Any]] = Field(..., description="List of operations")


class PlaygroundResponse(BaseModel):
    """Response with result and SDK code."""
    success: bool
    message: str
    data: dict[str, Any] | None = None
    sdk_code: str = Field(..., description="Equivalent Python SDK code")


# --- Dependencies ---


def get_db_client(request: Request) -> DbClient:
    """Get SDK client from app state."""
    return request.app.state.db_client


def get_settings(request: Request):
    """Get settings from app state."""
    return request.app.state.settings


# --- Schema Info ---


@router.get("/schema")
async def get_playground_schema():
    """
    Get the playground schema.

    Returns all available node types and edge types for the sandbox.
    """
    return {
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
        "info": "All data created here is visible in Console under tenant 'playground'",
    }


# --- Node Operations ---


@router.post("/nodes", response_model=PlaygroundResponse)
async def create_node(
    request: CreateNodeRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Create a node in the sandbox.

    Returns the created node ID and equivalent SDK code.
    """
    node_type = get_node_type(request.type_id)
    if not node_type:
        raise HTTPException(status_code=400, detail=f"Unknown type_id: {request.type_id}")

    # Add timestamp
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
            sdk_code=generate_create_node_code(request.type_id, payload),
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
    """
    Update a node in the sandbox.
    """
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
            sdk_code=generate_update_node_code(request.type_id, request.node_id, request.patch),
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
    """
    Delete a node from the sandbox.
    """
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
            sdk_code=generate_delete_node_code(request.type_id, request.node_id),
        )

    except Exception as e:
        logger.error(f"Delete node failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# --- Edge Operations ---


@router.post("/edges", response_model=PlaygroundResponse)
async def create_edge(
    request: CreateEdgeRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Create an edge in the sandbox.
    """
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
            sdk_code=generate_create_edge_code(request.edge_id, request.from_id, request.to_id, request.props),
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
    """
    Delete an edge from the sandbox.
    """
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
            sdk_code=generate_delete_edge_code(request.edge_id, request.from_id, request.to_id),
        )

    except Exception as e:
        logger.error(f"Delete edge failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# --- Atomic Transactions ---


@router.post("/atomic", response_model=PlaygroundResponse)
async def execute_atomic(
    request: AtomicRequest,
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Execute multiple operations atomically.

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

        return PlaygroundResponse(
            success=True,
            message=f"Executed {len(request.operations)} operations atomically",
            data={
                "created_node_ids": result.created_node_ids,
                "operation_count": len(request.operations),
            },
            sdk_code=generate_atomic_code(request.operations),
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Atomic transaction failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# --- Cleanup ---


@router.post("/reset", response_model=PlaygroundResponse)
async def reset_sandbox(
    db: DbClient = Depends(get_db_client),
    settings = Depends(get_settings),
):
    """
    Delete all data in the sandbox tenant.

    Warning: This deletes everything!
    """
    # Note: Full implementation would query and delete all nodes
    # For now, this is a placeholder
    return PlaygroundResponse(
        success=True,
        message="Sandbox reset not yet implemented. Nodes are deleted after TTL.",
        data={"tenant": settings.sandbox_tenant},
        sdk_code="# Sandbox cleanup is handled automatically by TTL",
    )
