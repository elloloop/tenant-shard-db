"""
gRPC client for EntDB Console.

This is a simple client that talks directly to the EntDB server
using gRPC. Unlike the SDK, this doesn't require schema definitions -
it works with raw type IDs and JSON payloads.

This is intentionally separate from the SDK to keep the Console
decoupled and able to browse any EntDB instance without schema code.
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from typing import Any

from grpc import aio as grpc_aio

# Import generated stubs - Console has its own copy
from entdb_sdk._generated import (
    EntDBServiceStub,
    GetEdgesRequest,
    GetNodeRequest,
    GetSchemaRequest,
    HealthRequest,
    QueryNodesRequest,
    RequestContext,
    SearchMailboxRequest,
)

logger = logging.getLogger(__name__)


@dataclass
class ConsoleNode:
    """Node data for Console display."""

    node_id: str
    type_id: int
    tenant_id: str
    payload: dict[str, Any]
    owner_actor: str
    created_at: int
    updated_at: int


@dataclass
class ConsoleEdge:
    """Edge data for Console display."""

    edge_type_id: int
    from_node_id: str
    to_node_id: str
    tenant_id: str
    props: dict[str, Any]
    created_at: int


class ConsoleClient:
    """
    Read-only gRPC client for EntDB Console.

    Provides direct access to EntDB for browsing data without
    requiring schema definitions in code.
    """

    def __init__(self, host: str, port: int) -> None:
        self._host = host
        self._port = port
        self._channel: grpc_aio.Channel | None = None
        self._stub: EntDBServiceStub | None = None

    async def connect(self) -> None:
        """Connect to EntDB server."""
        if self._channel:
            return

        address = f"{self._host}:{self._port}"
        self._channel = grpc_aio.insecure_channel(
            address,
            options=[
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),
            ],
        )
        self._stub = EntDBServiceStub(self._channel)
        logger.info(f"Console connected to EntDB at {address}")

    async def close(self) -> None:
        """Close connection."""
        if self._channel:
            await self._channel.close()
            self._channel = None
            self._stub = None

    def _context(self, tenant_id: str, actor: str) -> RequestContext:
        return RequestContext(tenant_id=tenant_id, actor=actor)

    async def health(self) -> dict[str, Any]:
        """Check server health."""
        if not self._stub:
            raise RuntimeError("Not connected")

        response = await self._stub.Health(HealthRequest())
        return {
            "healthy": response.healthy,
            "version": response.version,
            "components": dict(response.components),
        }

    async def get_schema(self) -> dict[str, Any]:
        """Get full schema from server."""
        if not self._stub:
            raise RuntimeError("Not connected")

        response = await self._stub.GetSchema(GetSchemaRequest())
        return {
            "schema": json.loads(response.schema_json) if response.schema_json else {},
            "fingerprint": response.fingerprint,
        }

    async def get_node(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        type_id: int = 0,
    ) -> ConsoleNode | None:
        """Get a node by ID."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetNodeRequest(
            context=self._context(tenant_id, actor),
            type_id=type_id,
            node_id=node_id,
        )

        response = await self._stub.GetNode(request)
        if not response.found:
            return None

        n = response.node
        return ConsoleNode(
            node_id=n.node_id,
            type_id=n.type_id,
            tenant_id=n.tenant_id,
            payload=json.loads(n.payload_json) if n.payload_json else {},
            owner_actor=n.owner_actor,
            created_at=n.created_at,
            updated_at=n.updated_at,
        )

    async def query_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        limit: int = 100,
        offset: int = 0,
    ) -> tuple[list[ConsoleNode], bool]:
        """Query nodes by type. Returns (nodes, has_more)."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = QueryNodesRequest(
            context=self._context(tenant_id, actor),
            type_id=type_id,
            limit=limit,
            offset=offset,
        )

        response = await self._stub.QueryNodes(request)

        nodes = [
            ConsoleNode(
                node_id=n.node_id,
                type_id=n.type_id,
                tenant_id=n.tenant_id,
                payload=json.loads(n.payload_json) if n.payload_json else {},
                owner_actor=n.owner_actor,
                created_at=n.created_at,
                updated_at=n.updated_at,
            )
            for n in response.nodes
        ]

        return nodes, response.has_more

    async def get_edges_from(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int | None = None,
        limit: int = 100,
    ) -> list[ConsoleEdge]:
        """Get outgoing edges from a node."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetEdgesRequest(
            context=self._context(tenant_id, actor),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await self._stub.GetEdgesFrom(request)

        return [
            ConsoleEdge(
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                tenant_id=e.tenant_id,
                props=json.loads(e.props_json) if e.props_json else {},
                created_at=e.created_at,
            )
            for e in response.edges
        ]

    async def get_edges_to(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int | None = None,
        limit: int = 100,
    ) -> list[ConsoleEdge]:
        """Get incoming edges to a node."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetEdgesRequest(
            context=self._context(tenant_id, actor),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await self._stub.GetEdgesTo(request)

        return [
            ConsoleEdge(
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                tenant_id=e.tenant_id,
                props=json.loads(e.props_json) if e.props_json else {},
                created_at=e.created_at,
            )
            for e in response.edges
        ]

    async def search(
        self,
        tenant_id: str,
        actor: str,
        user_id: str,
        query: str,
        limit: int = 20,
    ) -> list[dict[str, Any]]:
        """Search mailbox."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = SearchMailboxRequest(
            context=self._context(tenant_id, actor),
            user_id=user_id,
            query=query,
            limit=limit,
        )

        response = await self._stub.SearchMailbox(request)

        return [
            {
                "item": {
                    "item_id": r.item.item_id,
                    "ref_id": r.item.ref_id,
                    "source_type_id": r.item.source_type_id,
                    "source_node_id": r.item.source_node_id,
                    "snippet": r.item.snippet,
                },
                "rank": r.rank,
                "highlights": r.highlights,
            }
            for r in response.results
        ]
