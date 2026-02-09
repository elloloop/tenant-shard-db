"""
Playground API routes.

Simple write endpoints for experimenting with EntDB.
All data goes to a sandbox tenant that can be viewed in Console.
"""

import logging
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, Field

from entdb_sdk import DbClient, EdgeTypeDef, NodeTypeDef

from .schema import get_edge_type, get_node_type

logger = logging.getLogger(__name__)

router = APIRouter(tags=["Playground"])


# =============================================================================
# Request/Response Models
# =============================================================================


class CreateNodeRequest(BaseModel):
    """Create a node."""

    type_id: int = Field(..., description="Node type ID")
    type_name: str = Field(default="", description="Node type name (for display)")
    payload: dict[str, Any] = Field(default_factory=dict, description="Node data as JSON")


class CreateEdgeRequest(BaseModel):
    """Create an edge."""

    edge_type_id: int = Field(..., description="Edge type ID")
    from_id: str = Field(..., description="Source node ID")
    to_id: str = Field(..., description="Target node ID")
    props: dict[str, Any] | None = Field(None, description="Edge properties")


class PlaygroundResponse(BaseModel):
    """Response with result."""

    success: bool
    message: str
    data: dict[str, Any] | None = None


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
# Endpoints
# =============================================================================


@router.post("/nodes", response_model=PlaygroundResponse)
async def create_node(
    request: CreateNodeRequest,
    db: DbClient = Depends(get_db_client),
    settings=Depends(get_settings),
):
    """Create a node in the sandbox."""
    # Try predefined schema first, fall back to dynamic type
    node_type = get_node_type(request.type_id)
    if not node_type:
        name = request.type_name or f"Type_{request.type_id}"
        node_type = NodeTypeDef(
            type_id=request.type_id,
            name=name,
            fields=(),
        )

    try:
        plan = db.atomic(settings.sandbox_tenant, settings.sandbox_actor)
        plan.create(node_type, request.payload)
        result = await plan.commit(wait_applied=True)

        if not result.success:
            raise HTTPException(status_code=400, detail=result.error)

        node_id = result.created_node_ids[0] if result.created_node_ids else None

        return PlaygroundResponse(
            success=True,
            message=f"Created {node_type.name} node",
            data={
                "node_id": node_id,
                "type_id": request.type_id,
                "type_name": node_type.name,
                "payload": request.payload,
            },
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Create node failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/edges", response_model=PlaygroundResponse)
async def create_edge(
    request: CreateEdgeRequest,
    db: DbClient = Depends(get_db_client),
    settings=Depends(get_settings),
):
    """Create an edge in the sandbox."""
    # Try predefined schema first, fall back to dynamic type
    edge_type = get_edge_type(request.edge_type_id)
    if not edge_type:
        # Create a minimal edge type — SDK just needs edge_id and dummy from/to types
        dummy_type = NodeTypeDef(type_id=0, name="Node", fields=())
        edge_type = EdgeTypeDef(
            edge_id=request.edge_type_id,
            name=f"Edge_{request.edge_type_id}",
            from_type=dummy_type,
            to_type=dummy_type,
        )

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
                "edge_type_id": request.edge_type_id,
                "from_id": request.from_id,
                "to_id": request.to_id,
                "props": request.props,
            },
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Create edge failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))
