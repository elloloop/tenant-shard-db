"""
Internal gRPC client for EntDB SDK.

This module provides the low-level gRPC communication layer.
It is internal to the SDK and should not be used directly by users.

Users should use DbClient instead, which provides a clean Python API.
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from typing import Any

import grpc
from grpc import aio as grpc_aio

from ._generated import (
    EntDBServiceStub,
    # Request types
    RequestContext,
    ExecuteAtomicRequest,
    Operation,
    CreateNodeOp,
    UpdateNodeOp,
    DeleteNodeOp,
    CreateEdgeOp,
    DeleteEdgeOp,
    NodeRef,
    TypedNodeRef,
    GetReceiptStatusRequest,
    GetNodeRequest,
    GetNodesRequest,
    QueryNodesRequest,
    GetEdgesRequest,
    SearchMailboxRequest,
    GetMailboxRequest,
    HealthRequest,
    GetSchemaRequest,
    # Enums
    ReceiptStatus,
)

logger = logging.getLogger(__name__)


@dataclass
class GrpcNode:
    """Internal node representation from gRPC response."""

    tenant_id: str
    node_id: str
    type_id: int
    payload: dict[str, Any]
    created_at: int
    updated_at: int
    owner_actor: str
    acl: list[dict[str, str]]


@dataclass
class GrpcEdge:
    """Internal edge representation from gRPC response."""

    tenant_id: str
    edge_type_id: int
    from_node_id: str
    to_node_id: str
    props: dict[str, Any]
    created_at: int


@dataclass
class GrpcReceipt:
    """Internal receipt representation from gRPC response."""

    tenant_id: str
    idempotency_key: str
    stream_position: str | None


@dataclass
class GrpcCommitResult:
    """Internal commit result from gRPC response."""

    success: bool
    receipt: GrpcReceipt | None
    created_node_ids: list[str]
    applied: bool
    error: str | None


class GrpcClient:
    """Internal gRPC client for EntDB.

    This class handles all gRPC communication with the server.
    It manages connection lifecycle and provides async methods
    for all RPC operations.

    This is an internal class - users should use DbClient instead.
    """

    def __init__(
        self,
        host: str = "localhost",
        port: int = 50051,
        *,
        secure: bool = False,
        credentials: grpc.ChannelCredentials | None = None,
    ) -> None:
        """Initialize the gRPC client.

        Args:
            host: Server hostname
            port: Server port
            secure: Whether to use TLS
            credentials: Optional TLS credentials
        """
        self._host = host
        self._port = port
        self._secure = secure
        self._credentials = credentials
        self._channel: grpc_aio.Channel | None = None
        self._stub: EntDBServiceStub | None = None

    async def connect(self) -> None:
        """Establish connection to the server."""
        if self._channel is not None:
            return

        address = f"{self._host}:{self._port}"

        if self._secure:
            if self._credentials:
                self._channel = grpc_aio.secure_channel(address, self._credentials)
            else:
                self._channel = grpc_aio.secure_channel(
                    address,
                    grpc.ssl_channel_credentials(),
                )
        else:
            self._channel = grpc_aio.insecure_channel(
                address,
                options=[
                    ("grpc.max_send_message_length", 50 * 1024 * 1024),
                    ("grpc.max_receive_message_length", 50 * 1024 * 1024),
                ],
            )

        self._stub = EntDBServiceStub(self._channel)
        logger.debug(f"Connected to EntDB server at {address}")

    async def close(self) -> None:
        """Close the connection."""
        if self._channel:
            await self._channel.close()
            self._channel = None
            self._stub = None
            logger.debug("Disconnected from EntDB server")

    async def __aenter__(self) -> GrpcClient:
        await self.connect()
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()

    def _ensure_connected(self) -> EntDBServiceStub:
        """Ensure we're connected and return the stub."""
        if self._stub is None:
            raise RuntimeError("Not connected. Call connect() first.")
        return self._stub

    def _make_context(self, tenant_id: str, actor: str) -> RequestContext:
        """Create a request context."""
        return RequestContext(tenant_id=tenant_id, actor=actor)

    async def execute_atomic(
        self,
        tenant_id: str,
        actor: str,
        operations: list[dict[str, Any]],
        *,
        idempotency_key: str | None = None,
        schema_fingerprint: str | None = None,
        wait_applied: bool = False,
        wait_timeout_ms: int = 30000,
    ) -> GrpcCommitResult:
        """Execute an atomic transaction.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            operations: List of operations in SDK format
            idempotency_key: Optional idempotency key
            schema_fingerprint: Schema fingerprint for validation
            wait_applied: Whether to wait for application
            wait_timeout_ms: Timeout for wait_applied

        Returns:
            GrpcCommitResult with success/failure info
        """
        stub = self._ensure_connected()

        # Convert operations to protobuf format
        proto_ops = self._convert_operations(operations)

        request = ExecuteAtomicRequest(
            context=self._make_context(tenant_id, actor),
            idempotency_key=idempotency_key or "",
            schema_fingerprint=schema_fingerprint or "",
            operations=proto_ops,
            wait_applied=wait_applied,
            wait_timeout_ms=wait_timeout_ms,
        )

        try:
            response = await stub.ExecuteAtomic(request)

            receipt = None
            if response.receipt and response.receipt.idempotency_key:
                receipt = GrpcReceipt(
                    tenant_id=response.receipt.tenant_id,
                    idempotency_key=response.receipt.idempotency_key,
                    stream_position=response.receipt.stream_position or None,
                )

            return GrpcCommitResult(
                success=response.success,
                receipt=receipt,
                created_node_ids=list(response.created_node_ids),
                applied=response.applied_status == ReceiptStatus.RECEIPT_STATUS_APPLIED,
                error=response.error if response.error else None,
            )
        except grpc.RpcError as e:
            return GrpcCommitResult(
                success=False,
                receipt=None,
                created_node_ids=[],
                applied=False,
                error=str(e),
            )

    def _convert_operations(self, operations: list[dict[str, Any]]) -> list[Operation]:
        """Convert SDK operations to protobuf format."""
        result = []

        for op in operations:
            proto_op = Operation()

            if "create_node" in op:
                create = op["create_node"]
                create_op = CreateNodeOp(
                    type_id=create.get("type_id", 0),
                    id=create.get("id", ""),
                    data_json=create.get("data_json", "{}"),
                    acl_json=create.get("acl_json", ""),
                )
                if create.get("as"):
                    # protobuf field is named 'as' but Python keyword conflict
                    setattr(create_op, "as", create["as"])
                if create.get("fanout_to"):
                    create_op.fanout_to.extend(create["fanout_to"])
                proto_op.create_node.CopyFrom(create_op)

            elif "update_node" in op:
                update = op["update_node"]
                update_op = UpdateNodeOp(
                    type_id=update.get("type_id", 0),
                    id=update.get("id", ""),
                    patch_json=update.get("patch_json", "{}"),
                )
                if update.get("field_mask"):
                    update_op.field_mask.extend(update["field_mask"])
                proto_op.update_node.CopyFrom(update_op)

            elif "delete_node" in op:
                delete = op["delete_node"]
                proto_op.delete_node.CopyFrom(DeleteNodeOp(
                    type_id=delete.get("type_id", 0),
                    id=delete.get("id", ""),
                ))

            elif "create_edge" in op:
                create = op["create_edge"]
                create_op = CreateEdgeOp(
                    edge_id=create.get("edge_id", 0),
                    props_json=create.get("props_json", "{}"),
                )
                create_op.from_.CopyFrom(self._convert_node_ref(create.get("from", {})))
                create_op.to.CopyFrom(self._convert_node_ref(create.get("to", {})))
                proto_op.create_edge.CopyFrom(create_op)

            elif "delete_edge" in op:
                delete = op["delete_edge"]
                delete_op = DeleteEdgeOp(edge_id=delete.get("edge_id", 0))
                delete_op.from_.CopyFrom(self._convert_node_ref(delete.get("from", {})))
                delete_op.to.CopyFrom(self._convert_node_ref(delete.get("to", {})))
                proto_op.delete_edge.CopyFrom(delete_op)

            result.append(proto_op)

        return result

    def _convert_node_ref(self, ref: dict[str, Any]) -> NodeRef:
        """Convert node reference to protobuf format."""
        node_ref = NodeRef()

        if "id" in ref:
            node_ref.id = ref["id"]
        elif "alias_ref" in ref:
            node_ref.alias_ref = ref["alias_ref"]
        elif "typed" in ref:
            node_ref.typed.CopyFrom(TypedNodeRef(
                type_id=ref["typed"].get("type_id", 0),
                id=ref["typed"].get("id", ""),
            ))

        return node_ref

    async def get_receipt_status(
        self,
        tenant_id: str,
        idempotency_key: str,
    ) -> str:
        """Get the status of a transaction receipt.

        Returns:
            Status string: "PENDING", "APPLIED", "FAILED", or "UNKNOWN"
        """
        stub = self._ensure_connected()

        request = GetReceiptStatusRequest(
            context=self._make_context(tenant_id, "system"),
            idempotency_key=idempotency_key,
        )

        response = await stub.GetReceiptStatus(request)

        status_map = {
            ReceiptStatus.RECEIPT_STATUS_PENDING: "PENDING",
            ReceiptStatus.RECEIPT_STATUS_APPLIED: "APPLIED",
            ReceiptStatus.RECEIPT_STATUS_FAILED: "FAILED",
            ReceiptStatus.RECEIPT_STATUS_UNKNOWN: "UNKNOWN",
        }
        return status_map.get(response.status, "UNKNOWN")

    async def get_node(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        node_id: str,
    ) -> GrpcNode | None:
        """Get a node by ID."""
        stub = self._ensure_connected()

        request = GetNodeRequest(
            context=self._make_context(tenant_id, actor),
            type_id=type_id,
            node_id=node_id,
        )

        response = await stub.GetNode(request)

        if not response.found:
            return None

        node = response.node
        return GrpcNode(
            tenant_id=node.tenant_id,
            node_id=node.node_id,
            type_id=node.type_id,
            payload=json.loads(node.payload_json) if node.payload_json else {},
            created_at=node.created_at,
            updated_at=node.updated_at,
            owner_actor=node.owner_actor,
            acl=json.loads(node.acl_json) if node.acl_json else [],
        )

    async def get_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        node_ids: list[str],
    ) -> tuple[list[GrpcNode], list[str]]:
        """Get multiple nodes by IDs.

        Returns:
            Tuple of (found nodes, missing node IDs)
        """
        stub = self._ensure_connected()

        request = GetNodesRequest(
            context=self._make_context(tenant_id, actor),
            type_id=type_id,
            node_ids=node_ids,
        )

        response = await stub.GetNodes(request)

        nodes = [
            GrpcNode(
                tenant_id=n.tenant_id,
                node_id=n.node_id,
                type_id=n.type_id,
                payload=json.loads(n.payload_json) if n.payload_json else {},
                created_at=n.created_at,
                updated_at=n.updated_at,
                owner_actor=n.owner_actor,
                acl=json.loads(n.acl_json) if n.acl_json else [],
            )
            for n in response.nodes
        ]

        return nodes, list(response.missing_ids)

    async def query_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        *,
        limit: int = 100,
        offset: int = 0,
        filter_json: str | None = None,
        order_by: str | None = None,
        descending: bool = False,
    ) -> tuple[list[GrpcNode], bool]:
        """Query nodes by type.

        Returns:
            Tuple of (nodes, has_more)
        """
        stub = self._ensure_connected()

        request = QueryNodesRequest(
            context=self._make_context(tenant_id, actor),
            type_id=type_id,
            limit=limit,
            offset=offset,
            filter_json=filter_json or "",
            order_by=order_by or "",
            descending=descending,
        )

        response = await stub.QueryNodes(request)

        nodes = [
            GrpcNode(
                tenant_id=n.tenant_id,
                node_id=n.node_id,
                type_id=n.type_id,
                payload=json.loads(n.payload_json) if n.payload_json else {},
                created_at=n.created_at,
                updated_at=n.updated_at,
                owner_actor=n.owner_actor,
                acl=json.loads(n.acl_json) if n.acl_json else [],
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
    ) -> tuple[list[GrpcEdge], bool]:
        """Get outgoing edges from a node.

        Returns:
            Tuple of (edges, has_more)
        """
        stub = self._ensure_connected()

        request = GetEdgesRequest(
            context=self._make_context(tenant_id, actor),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await stub.GetEdgesFrom(request)

        edges = [
            GrpcEdge(
                tenant_id=e.tenant_id,
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                props=json.loads(e.props_json) if e.props_json else {},
                created_at=e.created_at,
            )
            for e in response.edges
        ]

        return edges, response.has_more

    async def get_edges_to(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int | None = None,
        limit: int = 100,
    ) -> tuple[list[GrpcEdge], bool]:
        """Get incoming edges to a node.

        Returns:
            Tuple of (edges, has_more)
        """
        stub = self._ensure_connected()

        request = GetEdgesRequest(
            context=self._make_context(tenant_id, actor),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await stub.GetEdgesTo(request)

        edges = [
            GrpcEdge(
                tenant_id=e.tenant_id,
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                props=json.loads(e.props_json) if e.props_json else {},
                created_at=e.created_at,
            )
            for e in response.edges
        ]

        return edges, response.has_more

    async def search_mailbox(
        self,
        tenant_id: str,
        actor: str,
        user_id: str,
        query: str,
        *,
        source_type_ids: list[int] | None = None,
        limit: int = 20,
        offset: int = 0,
    ) -> list[dict[str, Any]]:
        """Search user's mailbox with full-text search."""
        stub = self._ensure_connected()

        request = SearchMailboxRequest(
            context=self._make_context(tenant_id, actor),
            user_id=user_id,
            query=query,
            limit=limit,
            offset=offset,
        )
        if source_type_ids:
            request.source_type_ids.extend(source_type_ids)

        response = await stub.SearchMailbox(request)

        return [
            {
                "item": {
                    "item_id": r.item.item_id,
                    "ref_id": r.item.ref_id,
                    "source_type_id": r.item.source_type_id,
                    "source_node_id": r.item.source_node_id,
                    "thread_id": r.item.thread_id,
                    "ts": r.item.ts,
                    "state": json.loads(r.item.state_json) if r.item.state_json else {},
                    "snippet": r.item.snippet,
                    "metadata": json.loads(r.item.metadata_json) if r.item.metadata_json else {},
                },
                "rank": r.rank,
                "highlights": r.highlights,
            }
            for r in response.results
        ]

    async def get_mailbox(
        self,
        tenant_id: str,
        actor: str,
        user_id: str,
        *,
        source_type_id: int | None = None,
        thread_id: str | None = None,
        unread_only: bool = False,
        limit: int = 50,
        offset: int = 0,
    ) -> tuple[list[dict[str, Any]], int, bool]:
        """Get mailbox items for a user.

        Returns:
            Tuple of (items, unread_count, has_more)
        """
        stub = self._ensure_connected()

        request = GetMailboxRequest(
            context=self._make_context(tenant_id, actor),
            user_id=user_id,
            source_type_id=source_type_id or 0,
            thread_id=thread_id or "",
            unread_only=unread_only,
            limit=limit,
            offset=offset,
        )

        response = await stub.GetMailbox(request)

        items = [
            {
                "item_id": item.item_id,
                "ref_id": item.ref_id,
                "source_type_id": item.source_type_id,
                "source_node_id": item.source_node_id,
                "thread_id": item.thread_id,
                "ts": item.ts,
                "state": json.loads(item.state_json) if item.state_json else {},
                "snippet": item.snippet,
                "metadata": json.loads(item.metadata_json) if item.metadata_json else {},
            }
            for item in response.items
        ]

        return items, response.unread_count, response.has_more

    async def health(self) -> dict[str, Any]:
        """Get server health status."""
        stub = self._ensure_connected()

        response = await stub.Health(HealthRequest())

        return {
            "healthy": response.healthy,
            "version": response.version,
            "components": dict(response.components),
        }

    async def get_schema(self, type_id: int | None = None) -> dict[str, Any]:
        """Get schema information."""
        stub = self._ensure_connected()

        request = GetSchemaRequest(type_id=type_id or 0)
        response = await stub.GetSchema(request)

        return {
            "schema": json.loads(response.schema_json) if response.schema_json else {},
            "fingerprint": response.fingerprint,
        }
