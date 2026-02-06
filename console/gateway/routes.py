"""
Read-only API routes for EntDB Console.

Provides REST endpoints for browsing EntDB data. This is intentionally
read-only - all write operations must go through the SDK.

The Console fetches schema from the server and displays data dynamically,
without needing schema definitions in code.
"""

import logging
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Query, Request
from pydantic import BaseModel, Field

from .console_client import ConsoleClient

logger = logging.getLogger(__name__)

router = APIRouter(tags=["EntDB Console"])


# --- Response Models ---


class NodeResponse(BaseModel):
    """Node data."""
    node_id: str
    type_id: int
    tenant_id: str
    payload: dict[str, Any]
    owner_actor: str
    created_at: int | None = None
    updated_at: int | None = None


class EdgeResponse(BaseModel):
    """Edge data."""
    edge_type_id: int
    from_node_id: str
    to_node_id: str
    tenant_id: str
    props: dict[str, Any] | None = None
    created_at: int | None = None


class PaginatedNodesResponse(BaseModel):
    """Paginated nodes response."""
    nodes: list[NodeResponse]
    offset: int
    limit: int
    has_more: bool


class SchemaTypeResponse(BaseModel):
    """Schema type info."""
    type_id: int
    name: str
    fields: list[dict[str, Any]]
    deprecated: bool = False


class SchemaEdgeTypeResponse(BaseModel):
    """Schema edge type info."""
    edge_id: int
    name: str
    from_type_id: int
    to_type_id: int
    props: list[dict[str, Any]] = Field(default_factory=list)


class SchemaResponse(BaseModel):
    """Full schema."""
    node_types: list[SchemaTypeResponse]
    edge_types: list[SchemaEdgeTypeResponse]
    fingerprint: str


class HealthResponse(BaseModel):
    """Health status."""
    healthy: bool
    version: str
    components: dict[str, str]


class GraphResponse(BaseModel):
    """Graph neighborhood response."""
    root_id: str
    nodes: list[NodeResponse]
    edges: list[EdgeResponse]


# --- Dependencies ---


def get_client(request: Request) -> ConsoleClient:
    """Get Console client from app state."""
    return request.app.state.console_client


def get_tenant_id(
    request: Request,
    x_tenant_id: str | None = Query(None, alias="tenant_id"),
) -> str:
    """Get tenant ID from header or query param."""
    tenant = request.headers.get("X-Tenant-ID") or x_tenant_id
    if not tenant:
        tenant = request.app.state.settings.default_tenant_id
    return tenant


def get_actor(
    request: Request,
    x_actor: str | None = Query(None, alias="actor"),
) -> str:
    """Get actor from header or query param."""
    actor = request.headers.get("X-Actor") or x_actor
    return actor or "console:browser"


# --- Health & Schema ---


@router.get("/health", response_model=HealthResponse)
async def health(client: ConsoleClient = Depends(get_client)):
    """
    Check EntDB server health.

    Returns health status of server components.
    """
    return await client.health()


@router.get("/schema", response_model=SchemaResponse)
async def get_schema(client: ConsoleClient = Depends(get_client)):
    """
    Get the full schema.

    Returns all node types and edge types registered on the server.
    Use this to understand the data model and render dynamic forms.
    """
    result = await client.get_schema()
    schema = result.get("schema", {})

    return SchemaResponse(
        node_types=[
            SchemaTypeResponse(**t) for t in schema.get("node_types", [])
        ],
        edge_types=[
            SchemaEdgeTypeResponse(**e) for e in schema.get("edge_types", [])
        ],
        fingerprint=result.get("fingerprint", ""),
    )


@router.get("/schema/types/{type_id}", response_model=SchemaTypeResponse)
async def get_type_schema(
    type_id: int,
    client: ConsoleClient = Depends(get_client),
):
    """
    Get schema for a specific node type.

    Returns field definitions and metadata for the type.
    """
    result = await client.get_schema()
    schema = result.get("schema", {})

    for node_type in schema.get("node_types", []):
        if node_type.get("type_id") == type_id:
            return SchemaTypeResponse(**node_type)

    raise HTTPException(status_code=404, detail=f"Type {type_id} not found")


# --- Node Browsing ---


@router.get("/nodes", response_model=PaginatedNodesResponse)
async def list_nodes(
    type_id: int = Query(..., description="Node type ID to query"),
    offset: int = Query(0, ge=0, description="Pagination offset"),
    limit: int = Query(50, ge=1, le=200, description="Page size"),
    client: ConsoleClient = Depends(get_client),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    List nodes of a specific type.

    Paginated results for browsing nodes.
    """
    nodes, has_more = await client.query_nodes(
        tenant_id=tenant_id,
        actor=actor,
        type_id=type_id,
        limit=limit,
        offset=offset,
    )

    return PaginatedNodesResponse(
        nodes=[
            NodeResponse(
                node_id=n.node_id,
                type_id=n.type_id,
                tenant_id=n.tenant_id,
                payload=n.payload,
                owner_actor=n.owner_actor,
                created_at=n.created_at,
                updated_at=n.updated_at,
            )
            for n in nodes
        ],
        offset=offset,
        limit=limit,
        has_more=has_more,
    )


@router.get("/nodes/{node_id}", response_model=NodeResponse)
async def get_node(
    node_id: str,
    client: ConsoleClient = Depends(get_client),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get a single node by ID.

    Returns the full node with all payload fields.
    """
    node = await client.get_node(
        tenant_id=tenant_id,
        actor=actor,
        node_id=node_id,
    )

    if not node:
        raise HTTPException(status_code=404, detail=f"Node {node_id} not found")

    return NodeResponse(
        node_id=node.node_id,
        type_id=node.type_id,
        tenant_id=node.tenant_id,
        payload=node.payload,
        owner_actor=node.owner_actor,
        created_at=node.created_at,
        updated_at=node.updated_at,
    )


# --- Edge Browsing ---


@router.get("/nodes/{node_id}/edges/out", response_model=list[EdgeResponse])
async def get_outgoing_edges(
    node_id: str,
    edge_type_id: int | None = Query(None, description="Filter by edge type"),
    limit: int = Query(100, ge=1, le=500),
    client: ConsoleClient = Depends(get_client),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get outgoing edges from a node.

    Returns edges where this node is the source.
    """
    edges = await client.get_edges_from(
        tenant_id=tenant_id,
        actor=actor,
        node_id=node_id,
        edge_type_id=edge_type_id,
        limit=limit,
    )

    return [
        EdgeResponse(
            edge_type_id=e.edge_type_id,
            from_node_id=e.from_node_id,
            to_node_id=e.to_node_id,
            tenant_id=e.tenant_id,
            props=e.props,
            created_at=e.created_at,
        )
        for e in edges
    ]


@router.get("/nodes/{node_id}/edges/in", response_model=list[EdgeResponse])
async def get_incoming_edges(
    node_id: str,
    edge_type_id: int | None = Query(None, description="Filter by edge type"),
    limit: int = Query(100, ge=1, le=500),
    client: ConsoleClient = Depends(get_client),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get incoming edges to a node.

    Returns edges where this node is the target.
    """
    edges = await client.get_edges_to(
        tenant_id=tenant_id,
        actor=actor,
        node_id=node_id,
        edge_type_id=edge_type_id,
        limit=limit,
    )

    return [
        EdgeResponse(
            edge_type_id=e.edge_type_id,
            from_node_id=e.from_node_id,
            to_node_id=e.to_node_id,
            tenant_id=e.tenant_id,
            props=e.props,
            created_at=e.created_at,
        )
        for e in edges
    ]


# --- Graph Visualization ---


@router.get("/graph/{node_id}", response_model=GraphResponse)
async def get_graph_neighborhood(
    node_id: str,
    depth: int = Query(1, ge=1, le=3, description="Traversal depth"),
    client: ConsoleClient = Depends(get_client),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Get graph neighborhood of a node.

    Returns the node and connected nodes up to specified depth.
    Useful for graph visualization.
    """
    nodes_map: dict[str, NodeResponse] = {}
    edges_list: list[EdgeResponse] = []
    to_visit = [(node_id, 0)]
    visited: set[str] = set()

    while to_visit:
        current_id, current_depth = to_visit.pop(0)
        if current_id in visited or current_depth > depth:
            continue

        visited.add(current_id)

        node = await client.get_node(
            tenant_id=tenant_id,
            actor=actor,
            node_id=current_id,
        )

        if node:
            nodes_map[current_id] = NodeResponse(
                node_id=node.node_id,
                type_id=node.type_id,
                tenant_id=node.tenant_id,
                payload=node.payload,
                owner_actor=node.owner_actor,
                created_at=node.created_at,
                updated_at=node.updated_at,
            )

            if current_depth < depth:
                # Get edges
                out_edges = await client.get_edges_from(
                    tenant_id=tenant_id,
                    actor=actor,
                    node_id=current_id,
                )
                in_edges = await client.get_edges_to(
                    tenant_id=tenant_id,
                    actor=actor,
                    node_id=current_id,
                )

                for e in out_edges:
                    edges_list.append(EdgeResponse(
                        edge_type_id=e.edge_type_id,
                        from_node_id=e.from_node_id,
                        to_node_id=e.to_node_id,
                        tenant_id=e.tenant_id,
                        props=e.props,
                        created_at=e.created_at,
                    ))
                    to_visit.append((e.to_node_id, current_depth + 1))

                for e in in_edges:
                    edges_list.append(EdgeResponse(
                        edge_type_id=e.edge_type_id,
                        from_node_id=e.from_node_id,
                        to_node_id=e.to_node_id,
                        tenant_id=e.tenant_id,
                        props=e.props,
                        created_at=e.created_at,
                    ))
                    to_visit.append((e.from_node_id, current_depth + 1))

    return GraphResponse(
        root_id=node_id,
        nodes=list(nodes_map.values()),
        edges=edges_list,
    )


# --- Search ---


@router.get("/search")
async def search(
    q: str = Query(..., min_length=1, description="Search query"),
    user_id: str = Query(..., description="User ID for mailbox search"),
    limit: int = Query(20, ge=1, le=100),
    client: ConsoleClient = Depends(get_client),
    tenant_id: str = Depends(get_tenant_id),
    actor: str = Depends(get_actor),
):
    """
    Full-text search in a user's mailbox.

    Searches using SQLite FTS5.
    """
    results = await client.search(
        tenant_id=tenant_id,
        actor=actor,
        user_id=user_id,
        query=q,
        limit=limit,
    )

    return {"query": q, "results": results}


# --- Browse Helpers ---


@router.get("/browse/types")
async def browse_types(
    client: ConsoleClient = Depends(get_client),
):
    """
    Get all node types for sidebar navigation.

    Returns type names and IDs for the browser interface.
    """
    result = await client.get_schema()
    schema = result.get("schema", {})

    return {
        "types": [
            {
                "type_id": t.get("type_id"),
                "name": t.get("name"),
                "field_count": len(t.get("fields", [])),
                "deprecated": t.get("deprecated", False),
            }
            for t in schema.get("node_types", [])
        ]
    }


@router.get("/browse/edge-types")
async def browse_edge_types(
    client: ConsoleClient = Depends(get_client),
):
    """
    Get all edge types for reference.

    Returns edge type definitions.
    """
    result = await client.get_schema()
    schema = result.get("schema", {})

    return {
        "edge_types": [
            {
                "edge_id": e.get("edge_id"),
                "name": e.get("name"),
                "from_type_id": e.get("from_type_id"),
                "to_type_id": e.get("to_type_id"),
            }
            for e in schema.get("edge_types", [])
        ]
    }
