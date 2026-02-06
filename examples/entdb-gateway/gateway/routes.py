"""
API routes for EntDB HTTP Gateway.

Provides REST endpoints that wrap the EntDB SDK, demonstrating
how to build HTTP APIs on top of EntDB.
"""

import logging
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Query, Request
from pydantic import BaseModel, Field

from .sdk_client import SdkClientManager

logger = logging.getLogger(__name__)

router = APIRouter(tags=["EntDB Gateway"])


# --- Request/Response Models ---


class NodeCreateRequest(BaseModel):
    """Request to create a node."""

    type_id: int = Field(..., description="Node type ID")
    payload: dict[str, Any] = Field(..., description="Node payload")
    acl: list[str] | None = Field(None, description="Access control list")
    idempotency_key: str | None = Field(None, description="Idempotency key")


class NodeUpdateRequest(BaseModel):
    """Request to update a node."""

    payload: dict[str, Any] = Field(..., description="Fields to update")


class EdgeCreateRequest(BaseModel):
    """Request to create an edge."""

    edge_type_id: int = Field(..., description="Edge type ID")
    from_id: str = Field(..., description="Source node ID")
    to_id: str = Field(..., description="Target node ID")
    payload: dict[str, Any] | None = Field(None, description="Edge payload")


class AtomicOperationRequest(BaseModel):
    """Request for atomic transaction."""

    operations: list[dict[str, Any]] = Field(..., description="List of operations")
    idempotency_key: str | None = Field(None, description="Idempotency key")


class NodeResponse(BaseModel):
    """Node response."""

    id: str
    type_id: int
    tenant_id: str
    payload: dict[str, Any]
    owner_actor: str
    created_at: int | None = None
    updated_at: int | None = None


class EdgeResponse(BaseModel):
    """Edge response."""

    id: str
    edge_type_id: int
    from_id: str
    to_id: str
    tenant_id: str
    payload: dict[str, Any] | None = None


class PaginatedResponse(BaseModel):
    """Paginated list response."""

    items: list[dict[str, Any]]
    total: int
    offset: int
    limit: int
    has_more: bool


class SchemaTypeResponse(BaseModel):
    """Schema type information."""

    type_id: int
    name: str
    fields: list[dict[str, Any]]
    deprecated: bool = False


class SchemaResponse(BaseModel):
    """Full schema response."""

    node_types: list[SchemaTypeResponse]
    edge_types: list[dict[str, Any]]
    fingerprint: str


# --- Dependencies ---


def get_sdk_manager(request: Request) -> SdkClientManager:
    """Get SDK manager from app state."""
    return request.app.state.sdk_manager


def get_tenant_id(
    request: Request,
    x_tenant_id: str | None = Query(None, alias="X-Tenant-ID"),
) -> str:
    """Get tenant ID from header or query param."""
    # Check header first
    tenant = request.headers.get("X-Tenant-ID") or x_tenant_id
    if not tenant:
        # Use default from settings
        tenant = request.app.state.sdk_manager.settings.default_tenant_id
    return tenant


def get_actor(
    request: Request,
    x_actor: str | None = Query(None, alias="X-Actor"),
) -> str:
    """Get actor from header or query param."""
    actor = request.headers.get("X-Actor") or x_actor
    return actor or "gateway:anonymous"


# --- Schema Routes ---


@router.get("/schema", response_model=SchemaResponse)
async def get_schema(
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get the full schema definition.

    Returns all registered node types, edge types, and schema fingerprint.
    Useful for frontend to dynamically render forms and displays.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        schema = await client.get_schema()
        return schema


@router.get("/schema/types/{type_id}", response_model=SchemaTypeResponse)
async def get_type_schema(
    type_id: int,
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get schema for a specific node type.

    Returns field definitions, validation rules, and metadata.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        schema = await client.get_schema()

        for node_type in schema.get("node_types", []):
            if node_type["type_id"] == type_id:
                return node_type

        raise HTTPException(status_code=404, detail=f"Type {type_id} not found")


# --- Node Routes ---


@router.get("/nodes", response_model=PaginatedResponse)
async def list_nodes(
    type_id: int | None = Query(None, description="Filter by type"),
    offset: int = Query(0, ge=0, description="Pagination offset"),
    limit: int = Query(50, ge=1, le=200, description="Page size"),
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    List nodes with optional filtering and pagination.

    Supports filtering by node type and pagination.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        if type_id is not None:
            nodes = await client.query(type_id=type_id, limit=limit + 1, offset=offset)
        else:
            # Query all types (simplified - in production you'd want proper multi-type query)
            nodes = await client.query(type_id=0, limit=limit + 1, offset=offset)

        has_more = len(nodes) > limit
        items = nodes[:limit]

        return PaginatedResponse(
            items=[_node_to_dict(n) for n in items],
            total=-1,  # Total count would require separate query
            offset=offset,
            limit=limit,
            has_more=has_more,
        )


@router.get("/nodes/{node_id}", response_model=NodeResponse)
async def get_node(
    node_id: str,
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get a single node by ID.

    Returns the node with its full payload.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        node = await client.get(node_id)
        if node is None:
            raise HTTPException(status_code=404, detail=f"Node {node_id} not found")
        return _node_to_dict(node)


@router.post("/nodes", response_model=NodeResponse, status_code=201)
async def create_node(
    request: NodeCreateRequest,
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Create a new node.

    The node will be created atomically and replicated to the WAL.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        result = await client.atomic(
            lambda plan: plan.create(
                type_id=request.type_id,
                payload=request.payload,
                acl=request.acl,
                alias="node",
            ),
            idempotency_key=request.idempotency_key,
        )

        node_id = result["results"][0]["node_id"]
        node = await client.get(node_id)
        return _node_to_dict(node)


@router.patch("/nodes/{node_id}", response_model=NodeResponse)
async def update_node(
    node_id: str,
    request: NodeUpdateRequest,
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Update an existing node.

    Only the specified fields will be updated.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        await client.atomic(
            lambda plan: plan.update(node_id, request.payload)
        )

        node = await client.get(node_id)
        return _node_to_dict(node)


@router.delete("/nodes/{node_id}", status_code=204)
async def delete_node(
    node_id: str,
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Delete a node.

    The node and all its edges will be removed.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        await client.atomic(
            lambda plan: plan.delete(node_id)
        )


# --- Edge Routes ---


@router.get("/nodes/{node_id}/edges/out", response_model=list[EdgeResponse])
async def get_outgoing_edges(
    node_id: str,
    edge_type_id: int | None = Query(None, description="Filter by edge type"),
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get outgoing edges from a node.

    Returns all edges where this node is the source.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        edges = await client.edges_out(node_id, edge_type_id=edge_type_id)
        return [_edge_to_dict(e) for e in edges]


@router.get("/nodes/{node_id}/edges/in", response_model=list[EdgeResponse])
async def get_incoming_edges(
    node_id: str,
    edge_type_id: int | None = Query(None, description="Filter by edge type"),
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get incoming edges to a node.

    Returns all edges where this node is the target.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        edges = await client.edges_in(node_id, edge_type_id=edge_type_id)
        return [_edge_to_dict(e) for e in edges]


@router.post("/edges", response_model=EdgeResponse, status_code=201)
async def create_edge(
    request: EdgeCreateRequest,
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Create an edge between two nodes.

    The edge is unidirectional from `from_id` to `to_id`.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        result = await client.atomic(
            lambda plan: plan.link(
                edge_type_id=request.edge_type_id,
                from_id=request.from_id,
                to_id=request.to_id,
                payload=request.payload,
            )
        )

        # Return created edge
        return EdgeResponse(
            id=result["results"][0]["edge_id"],
            edge_type_id=request.edge_type_id,
            from_id=request.from_id,
            to_id=request.to_id,
            tenant_id=tenant_id,
            payload=request.payload,
        )


# --- Search Routes ---


@router.get("/search")
async def search_mailbox(
    q: str = Query(..., min_length=1, description="Search query"),
    limit: int = Query(50, ge=1, le=200, description="Max results"),
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Full-text search in user's mailbox.

    Searches across all mailbox items using SQLite FTS5.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        results = await client.search(q, limit=limit)
        return {"query": q, "results": results}


# --- Atomic Transaction Routes ---


@router.post("/atomic")
async def execute_atomic(
    request: AtomicOperationRequest,
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Execute multiple operations atomically.

    All operations succeed or fail together. Supports:
    - create_node
    - update_node
    - delete_node
    - create_edge
    - delete_edge

    Operations can reference each other using aliases.
    """
    async with sdk.acquire(tenant_id, actor) as client:

        def build_plan(plan):
            for op in request.operations:
                op_type = op.get("op")
                if op_type == "create_node":
                    plan.create(
                        type_id=op["type_id"],
                        payload=op["payload"],
                        alias=op.get("alias"),
                        acl=op.get("acl"),
                    )
                elif op_type == "update_node":
                    plan.update(op["node_id"], op["payload"])
                elif op_type == "delete_node":
                    plan.delete(op["node_id"])
                elif op_type == "create_edge":
                    plan.link(
                        edge_type_id=op["edge_type_id"],
                        from_id=op["from_id"],
                        to_id=op["to_id"],
                        payload=op.get("payload"),
                    )
                elif op_type == "delete_edge":
                    plan.unlink(op["edge_id"])
                else:
                    raise HTTPException(
                        status_code=400,
                        detail=f"Unknown operation type: {op_type}",
                    )

        result = await client.atomic(build_plan, idempotency_key=request.idempotency_key)
        return result


# --- Browse Routes (for frontend) ---


@router.get("/browse/types")
async def browse_types(
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get all node types for browsing.

    Returns type names, IDs, and sample counts for the browser sidebar.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        schema = await client.get_schema()

        types_info = []
        for node_type in schema.get("node_types", []):
            # Get sample count for each type
            nodes = await client.query(type_id=node_type["type_id"], limit=1)
            types_info.append({
                "type_id": node_type["type_id"],
                "name": node_type["name"],
                "field_count": len(node_type.get("fields", [])),
                "deprecated": node_type.get("deprecated", False),
            })

        return {"types": types_info}


@router.get("/browse/graph/{node_id}")
async def browse_graph(
    node_id: str,
    depth: int = Query(1, ge=1, le=3, description="Traversal depth"),
    sdk: SdkClientManager = Depends(get_sdk_manager),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get graph neighborhood of a node.

    Returns the node and its connected nodes up to specified depth.
    Useful for graph visualization in the frontend.
    """
    async with sdk.acquire(tenant_id, actor) as client:
        nodes = {}
        edges = []
        to_visit = [(node_id, 0)]
        visited = set()

        while to_visit:
            current_id, current_depth = to_visit.pop(0)
            if current_id in visited or current_depth > depth:
                continue

            visited.add(current_id)

            # Get node
            node = await client.get(current_id)
            if node:
                nodes[current_id] = _node_to_dict(node)

                if current_depth < depth:
                    # Get edges
                    out_edges = await client.edges_out(current_id)
                    in_edges = await client.edges_in(current_id)

                    for edge in out_edges:
                        edges.append(_edge_to_dict(edge))
                        to_visit.append((edge.to_id, current_depth + 1))

                    for edge in in_edges:
                        edges.append(_edge_to_dict(edge))
                        to_visit.append((edge.from_id, current_depth + 1))

        return {
            "root_id": node_id,
            "nodes": list(nodes.values()),
            "edges": edges,
        }


# --- Helpers ---


def _node_to_dict(node) -> dict[str, Any]:
    """Convert node object to dict."""
    return {
        "id": node.id,
        "type_id": node.type_id,
        "tenant_id": node.tenant_id,
        "payload": node.payload,
        "owner_actor": node.owner_actor,
        "created_at": getattr(node, "created_at", None),
        "updated_at": getattr(node, "updated_at", None),
    }


def _edge_to_dict(edge) -> dict[str, Any]:
    """Convert edge object to dict."""
    return {
        "id": edge.id,
        "edge_type_id": edge.edge_type_id,
        "from_id": edge.from_id,
        "to_id": edge.to_id,
        "tenant_id": edge.tenant_id,
        "payload": getattr(edge, "payload", None),
    }
