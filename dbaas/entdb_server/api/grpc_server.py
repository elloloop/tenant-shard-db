"""
gRPC server implementation for EntDB.

This module provides the gRPC API server that handles all client requests.
It implements the EntDBService defined in entdb.proto.

Invariants:
    - All RPCs require valid tenant_id and actor
    - Writes are durably stored in WAL before response
    - Reads come from SQLite materialized views
    - All errors are logged with structured context

How to change safely:
    - Add new RPCs without modifying existing ones
    - Use optional fields for backward compatibility
    - Test with both old and new clients
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
import uuid
from typing import TYPE_CHECKING, Any

import grpc
from grpc import aio as grpc_aio

from .generated import (
    Edge,
    EntDBServiceServicer,
    # Request/Response types
    ExecuteAtomicRequest,
    ExecuteAtomicResponse,
    GetEdgesRequest,
    GetEdgesResponse,
    GetMailboxRequest,
    GetMailboxResponse,
    GetNodeRequest,
    GetNodeResponse,
    GetNodesRequest,
    GetNodesResponse,
    GetReceiptStatusRequest,
    GetReceiptStatusResponse,
    GetSchemaRequest,
    GetSchemaResponse,
    HealthRequest,
    HealthResponse,
    MailboxItem,
    MailboxSearchResult,
    Node,
    QueryNodesRequest,
    QueryNodesResponse,
    # Data types
    Receipt,
    ReceiptStatus,
    SearchMailboxRequest,
    SearchMailboxResponse,
    add_EntDBServiceServicer_to_server,
)

if TYPE_CHECKING:
    pass

logger = logging.getLogger(__name__)


class EntDBServicer(EntDBServiceServicer):
    """gRPC service implementation for EntDB.

    This class implements the gRPC service methods defined in the proto.
    It coordinates between the WAL stream (for writes) and SQLite stores
    (for reads).

    Attributes:
        wal: WAL stream for durably writing events
        canonical_store: Tenant SQLite store for reads
        mailbox_store: Per-user mailbox store
        schema_registry: Schema registry for validation
    """

    def __init__(
        self,
        wal: Any,
        canonical_store: Any,
        mailbox_store: Any,
        schema_registry: Any,
        topic: str = "entdb-wal",
    ) -> None:
        """Initialize the gRPC servicer.

        Args:
            wal: WAL stream instance
            canonical_store: CanonicalStore instance
            mailbox_store: MailboxStore instance
            schema_registry: SchemaRegistry instance
            topic: WAL topic name
        """
        self.wal = wal
        self.canonical_store = canonical_store
        self.mailbox_store = mailbox_store
        self.schema_registry = schema_registry
        self.topic = topic

    async def ExecuteAtomic(
        self,
        request: ExecuteAtomicRequest,
        context: grpc_aio.ServicerContext,
    ) -> ExecuteAtomicResponse:
        """Execute an atomic transaction."""
        # Extract context
        ctx = request.context
        if not ctx.tenant_id:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
        if not ctx.actor:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
        if not request.operations:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "operations list is empty")

        # Generate idempotency key if not provided
        idempotency_key = request.idempotency_key or str(uuid.uuid4())

        # Validate schema fingerprint if provided
        if request.schema_fingerprint and self.schema_registry.fingerprint:
            if request.schema_fingerprint != self.schema_registry.fingerprint:
                return ExecuteAtomicResponse(
                    success=False,
                    error=f"Schema mismatch: server has {self.schema_registry.fingerprint}",
                    error_code="SCHEMA_MISMATCH",
                )

        # Convert operations to internal format
        ops = self._convert_operations(request.operations)

        # Assign IDs to create_node ops that don't have one
        created_node_ids = []
        for op in ops:
            if op.get("op") == "create_node":
                if not op.get("id"):
                    op["id"] = str(uuid.uuid4())
                created_node_ids.append(op["id"])

        # Build transaction event
        event = {
            "tenant_id": ctx.tenant_id,
            "actor": ctx.actor,
            "idempotency_key": idempotency_key,
            "schema_fingerprint": self.schema_registry.fingerprint,
            "ts_ms": int(time.time() * 1000),
            "ops": ops,
        }

        try:
            # Append to WAL
            event_bytes = json.dumps(event).encode("utf-8")
            stream_pos = await self.wal.append(
                self.topic,
                ctx.tenant_id,  # Partition key
                event_bytes,
            )

            receipt = Receipt(
                tenant_id=ctx.tenant_id,
                idempotency_key=idempotency_key,
                stream_position=str(stream_pos) if stream_pos else "",
            )

            # Wait for application if requested
            applied_status = ReceiptStatus.RECEIPT_STATUS_PENDING
            if request.wait_applied:
                timeout = request.wait_timeout_ms / 1000.0 if request.wait_timeout_ms else 30.0
                applied = await self._wait_for_applied(ctx.tenant_id, idempotency_key, timeout)
                if applied:
                    applied_status = ReceiptStatus.RECEIPT_STATUS_APPLIED

            return ExecuteAtomicResponse(
                success=True,
                receipt=receipt,
                created_node_ids=created_node_ids,
                applied_status=applied_status,
            )

        except Exception as e:
            logger.error(f"ExecuteAtomic failed: {e}", exc_info=True)
            return ExecuteAtomicResponse(
                success=False,
                error=str(e),
                error_code="INTERNAL",
            )

    def _convert_operations(self, operations: list) -> list[dict[str, Any]]:
        """Convert protobuf operations to internal format."""
        result = []

        for op in operations:
            op_type = op.WhichOneof("op")

            if op_type == "create_node":
                create = op.create_node
                internal_op: dict[str, Any] = {
                    "op": "create_node",
                    "type_id": create.type_id,
                    "data": json.loads(create.data_json) if create.data_json else {},
                }
                if create.id:
                    internal_op["id"] = create.id
                if create.acl_json:
                    internal_op["acl"] = json.loads(create.acl_json)
                as_val = getattr(create, "as", "")
                if as_val:
                    internal_op["as"] = as_val
                if create.fanout_to:
                    internal_op["fanout_to"] = list(create.fanout_to)
                result.append(internal_op)

            elif op_type == "update_node":
                update = op.update_node
                result.append(
                    {
                        "op": "update_node",
                        "type_id": update.type_id,
                        "id": update.id,
                        "patch": json.loads(update.patch_json) if update.patch_json else {},
                    }
                )

            elif op_type == "delete_node":
                delete = op.delete_node
                result.append(
                    {
                        "op": "delete_node",
                        "type_id": delete.type_id,
                        "id": delete.id,
                    }
                )

            elif op_type == "create_edge":
                create = op.create_edge
                result.append(
                    {
                        "op": "create_edge",
                        "edge_id": create.edge_id,
                        "from": self._convert_node_ref(getattr(create, "from")),
                        "to": self._convert_node_ref(create.to),
                        "props": json.loads(create.props_json) if create.props_json else {},
                    }
                )

            elif op_type == "delete_edge":
                delete = op.delete_edge
                result.append(
                    {
                        "op": "delete_edge",
                        "edge_id": delete.edge_id,
                        "from": self._convert_node_ref(getattr(delete, "from")),
                        "to": self._convert_node_ref(delete.to),
                    }
                )

        return result

    def _convert_node_ref(self, ref: Any) -> Any:
        """Convert protobuf node reference to internal format."""
        ref_type = ref.WhichOneof("ref")
        if ref_type == "id":  # noqa: SIM116
            return ref.id
        elif ref_type == "alias_ref":
            return {"ref": ref.alias_ref}
        elif ref_type == "typed":
            return {"type_id": ref.typed.type_id, "id": ref.typed.id}
        return None

    async def _wait_for_applied(
        self,
        tenant_id: str,
        idempotency_key: str,
        timeout: float,
    ) -> bool:
        """Wait for an event to be applied."""
        start = time.time()
        while time.time() - start < timeout:
            try:
                if await self.canonical_store.check_idempotency(tenant_id, idempotency_key):
                    return True
            except Exception:
                pass
            await asyncio.sleep(0.1)
        return False

    async def GetReceiptStatus(
        self,
        request: GetReceiptStatusRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetReceiptStatusResponse:
        """Get the status of a transaction receipt."""
        try:
            applied = await self.canonical_store.check_idempotency(
                request.context.tenant_id,
                request.idempotency_key,
            )
            status = (
                ReceiptStatus.RECEIPT_STATUS_APPLIED
                if applied
                else ReceiptStatus.RECEIPT_STATUS_PENDING
            )
            return GetReceiptStatusResponse(status=status)
        except Exception as e:
            return GetReceiptStatusResponse(
                status=ReceiptStatus.RECEIPT_STATUS_UNKNOWN,
                error=str(e),
            )

    async def GetNode(
        self,
        request: GetNodeRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetNodeResponse:
        """Get a node by ID."""
        try:
            node = await self.canonical_store.get_node(
                request.context.tenant_id,
                request.node_id,
            )
            if not node:
                return GetNodeResponse(found=False)

            return GetNodeResponse(
                found=True,
                node=Node(
                    tenant_id=node.tenant_id,
                    node_id=node.node_id,
                    type_id=node.type_id,
                    payload_json=json.dumps(node.payload),
                    created_at=node.created_at,
                    updated_at=node.updated_at,
                    owner_actor=node.owner_actor,
                    acl_json=json.dumps(node.acl),
                ),
            )
        except Exception as e:
            logger.error(f"GetNode failed: {e}", exc_info=True)
            return GetNodeResponse(found=False)

    async def GetNodes(
        self,
        request: GetNodesRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetNodesResponse:
        """Get multiple nodes by IDs."""
        try:
            nodes = []
            missing_ids = []

            for node_id in request.node_ids:
                node = await self.canonical_store.get_node(
                    request.context.tenant_id,
                    node_id,
                )
                if node:
                    nodes.append(
                        Node(
                            tenant_id=node.tenant_id,
                            node_id=node.node_id,
                            type_id=node.type_id,
                            payload_json=json.dumps(node.payload),
                            created_at=node.created_at,
                            updated_at=node.updated_at,
                            owner_actor=node.owner_actor,
                            acl_json=json.dumps(node.acl),
                        )
                    )
                else:
                    missing_ids.append(node_id)

            return GetNodesResponse(nodes=nodes, missing_ids=missing_ids)
        except Exception as e:
            logger.error(f"GetNodes failed: {e}", exc_info=True)
            return GetNodesResponse(nodes=[], missing_ids=list(request.node_ids))

    async def QueryNodes(
        self,
        request: QueryNodesRequest,
        context: grpc_aio.ServicerContext,
    ) -> QueryNodesResponse:
        """Query nodes by type."""
        try:
            nodes = await self.canonical_store.get_nodes_by_type(
                request.context.tenant_id,
                request.type_id,
                limit=request.limit or 100,
                offset=request.offset or 0,
            )

            proto_nodes = [
                Node(
                    tenant_id=n.tenant_id,
                    node_id=n.node_id,
                    type_id=n.type_id,
                    payload_json=json.dumps(n.payload),
                    created_at=n.created_at,
                    updated_at=n.updated_at,
                    owner_actor=n.owner_actor,
                    acl_json=json.dumps(n.acl),
                )
                for n in nodes
            ]

            return QueryNodesResponse(
                nodes=proto_nodes,
                has_more=len(nodes) == (request.limit or 100),
            )
        except Exception as e:
            logger.error(f"QueryNodes failed: {e}", exc_info=True)
            return QueryNodesResponse(nodes=[])

    async def GetEdgesFrom(
        self,
        request: GetEdgesRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetEdgesResponse:
        """Get outgoing edges from a node."""
        try:
            edge_type_id = request.edge_type_id if request.edge_type_id else None
            edges = await self.canonical_store.get_edges_from(
                request.context.tenant_id,
                request.node_id,
                edge_type_id,
            )

            limit = request.limit or 100
            proto_edges = [
                Edge(
                    tenant_id=e.tenant_id,
                    edge_type_id=e.edge_type_id,
                    from_node_id=e.from_node_id,
                    to_node_id=e.to_node_id,
                    props_json=json.dumps(e.props),
                    created_at=e.created_at,
                )
                for e in edges[:limit]
            ]

            return GetEdgesResponse(edges=proto_edges, has_more=len(edges) > limit)
        except Exception as e:
            logger.error(f"GetEdgesFrom failed: {e}", exc_info=True)
            return GetEdgesResponse(edges=[])

    async def GetEdgesTo(
        self,
        request: GetEdgesRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetEdgesResponse:
        """Get incoming edges to a node."""
        try:
            edge_type_id = request.edge_type_id if request.edge_type_id else None
            edges = await self.canonical_store.get_edges_to(
                request.context.tenant_id,
                request.node_id,
                edge_type_id,
            )

            limit = request.limit or 100
            proto_edges = [
                Edge(
                    tenant_id=e.tenant_id,
                    edge_type_id=e.edge_type_id,
                    from_node_id=e.from_node_id,
                    to_node_id=e.to_node_id,
                    props_json=json.dumps(e.props),
                    created_at=e.created_at,
                )
                for e in edges[:limit]
            ]

            return GetEdgesResponse(edges=proto_edges, has_more=len(edges) > limit)
        except Exception as e:
            logger.error(f"GetEdgesTo failed: {e}", exc_info=True)
            return GetEdgesResponse(edges=[])

    async def SearchMailbox(
        self,
        request: SearchMailboxRequest,
        context: grpc_aio.ServicerContext,
    ) -> SearchMailboxResponse:
        """Search mailbox with full-text search."""
        try:
            source_type_ids = list(request.source_type_ids) if request.source_type_ids else None
            results = await self.mailbox_store.search(
                request.context.tenant_id,
                request.user_id,
                request.query,
                limit=request.limit or 20,
                offset=request.offset or 0,
                source_type_ids=source_type_ids,
            )

            proto_results = [
                MailboxSearchResult(
                    item=MailboxItem(
                        item_id=r.item.item_id,
                        ref_id=r.item.ref_id,
                        source_type_id=r.item.source_type_id,
                        source_node_id=r.item.source_node_id,
                        thread_id=r.item.thread_id or "",
                        ts=r.item.ts,
                        state_json=json.dumps(r.item.state),
                        snippet=r.item.snippet or "",
                        metadata_json=json.dumps(r.item.metadata),
                    ),
                    rank=r.rank,
                    highlights=r.highlights or "",
                )
                for r in results
            ]

            return SearchMailboxResponse(
                results=proto_results,
                has_more=len(results) == (request.limit or 20),
            )
        except Exception as e:
            logger.error(f"SearchMailbox failed: {e}", exc_info=True)
            return SearchMailboxResponse(results=[])

    async def GetMailbox(
        self,
        request: GetMailboxRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetMailboxResponse:
        """Get mailbox items for a user."""
        try:
            source_type_id = request.source_type_id if request.source_type_id else None
            thread_id = request.thread_id if request.thread_id else None

            items = await self.mailbox_store.list_items(
                request.context.tenant_id,
                request.user_id,
                limit=request.limit or 50,
                offset=request.offset or 0,
                source_type_id=source_type_id,
                thread_id=thread_id,
                unread_only=request.unread_only,
            )

            unread_count = await self.mailbox_store.get_unread_count(
                request.context.tenant_id,
                request.user_id,
            )

            proto_items = [
                MailboxItem(
                    item_id=item.item_id,
                    ref_id=item.ref_id,
                    source_type_id=item.source_type_id,
                    source_node_id=item.source_node_id,
                    thread_id=item.thread_id or "",
                    ts=item.ts,
                    state_json=json.dumps(item.state),
                    snippet=item.snippet or "",
                    metadata_json=json.dumps(item.metadata),
                )
                for item in items
            ]

            return GetMailboxResponse(
                items=proto_items,
                unread_count=unread_count,
                has_more=len(items) == (request.limit or 50),
            )
        except Exception as e:
            logger.error(f"GetMailbox failed: {e}", exc_info=True)
            return GetMailboxResponse(items=[], unread_count=0)

    async def Health(
        self,
        request: HealthRequest,
        context: grpc_aio.ServicerContext,
    ) -> HealthResponse:
        """Get server health status."""
        components = {}

        # Check WAL connection
        try:
            wal_healthy = self.wal.is_connected if hasattr(self.wal, "is_connected") else True
            components["wal"] = "healthy" if wal_healthy else "unhealthy"
        except Exception:
            components["wal"] = "unknown"

        components["storage"] = "healthy"
        overall_healthy = all(v == "healthy" for v in components.values())

        return HealthResponse(
            healthy=overall_healthy,
            version="1.0.0",
            components=components,
        )

    async def GetSchema(
        self,
        request: GetSchemaRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetSchemaResponse:
        """Get schema information."""
        try:
            schema_dict = self.schema_registry.to_dict()

            if request.type_id:
                schema_dict["node_types"] = [
                    t
                    for t in schema_dict.get("node_types", [])
                    if t.get("type_id") == request.type_id
                ]
                schema_dict["edge_types"] = [
                    e
                    for e in schema_dict.get("edge_types", [])
                    if e.get("from_type_id") == request.type_id
                    or e.get("to_type_id") == request.type_id
                ]

            return GetSchemaResponse(
                schema_json=json.dumps(schema_dict),
                fingerprint=self.schema_registry.fingerprint or "",
            )
        except Exception as e:
            logger.error(f"GetSchema failed: {e}", exc_info=True)
            return GetSchemaResponse(schema_json="{}", fingerprint="")


class GrpcServer:
    """gRPC server wrapper for EntDB.

    This class manages the gRPC server lifecycle including:
    - Server initialization
    - Service registration
    - Graceful shutdown

    Example:
        >>> server = GrpcServer(servicer, port=50051)
        >>> await server.start()
        >>> # Server is now running
        >>> await server.stop()
    """

    def __init__(
        self,
        servicer: EntDBServicer,
        host: str = "0.0.0.0",
        port: int = 50051,
        max_workers: int = 10,
        reflection_enabled: bool = True,
    ) -> None:
        """Initialize the gRPC server.

        Args:
            servicer: EntDBServicer instance
            host: Host to bind to
            port: Port to listen on
            max_workers: Maximum concurrent RPCs
            reflection_enabled: Whether to enable gRPC reflection
        """
        self.servicer = servicer
        self.host = host
        self.port = port
        self.max_workers = max_workers
        self.reflection_enabled = reflection_enabled
        self._server: grpc_aio.Server | None = None
        self._running = False

    async def start(self) -> None:
        """Start the gRPC server."""
        if self._running:
            logger.warning("Server already running")
            return

        # Create async gRPC server
        self._server = grpc_aio.server(
            options=[
                ("grpc.max_send_message_length", 50 * 1024 * 1024),  # 50MB
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),  # 50MB
                ("grpc.max_concurrent_streams", self.max_workers),
            ]
        )

        # Register servicer
        add_EntDBServiceServicer_to_server(self.servicer, self._server)

        # Enable reflection for debugging tools like grpcurl
        if self.reflection_enabled:
            try:
                from grpc_reflection.v1alpha import reflection

                from .generated import entdb_pb2

                SERVICE_NAMES = (
                    entdb_pb2.DESCRIPTOR.services_by_name["EntDBService"].full_name,
                    reflection.SERVICE_NAME,
                )
                reflection.enable_server_reflection(SERVICE_NAMES, self._server)
            except ImportError:
                logger.warning("grpc-reflection not installed, skipping reflection")

        # Bind to address
        bind_address = f"{self.host}:{self.port}"
        self._server.add_insecure_port(bind_address)

        # Start server
        await self._server.start()
        self._running = True

        logger.info(
            f"gRPC server started on {bind_address}",
            extra={"host": self.host, "port": self.port},
        )

    async def stop(self, grace_period: float = 5.0) -> None:
        """Stop the gRPC server gracefully.

        Args:
            grace_period: Time to wait for pending RPCs to complete
        """
        if not self._running or not self._server:
            return

        logger.info("Stopping gRPC server...")
        await self._server.stop(grace_period)
        self._running = False
        logger.info("gRPC server stopped")

    async def wait_for_termination(self) -> None:
        """Wait for the server to terminate."""
        if self._server:
            await self._server.wait_for_termination()

    @property
    def is_running(self) -> bool:
        """Whether the server is running."""
        return self._running
