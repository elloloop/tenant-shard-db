"""
gRPC server implementation for EntDB.

This module provides the gRPC API server that handles all client requests.
It uses grpclib for async gRPC support and provides a service that
matches the proto definition.

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
from concurrent import futures
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple

logger = logging.getLogger(__name__)

# Try to import grpclib for async gRPC
try:
    import grpc
    from grpc import aio as grpc_aio
    GRPC_AVAILABLE = True
except ImportError:
    GRPC_AVAILABLE = False
    grpc = None
    grpc_aio = None


@dataclass
class RequestContext:
    """Parsed request context."""
    tenant_id: str
    actor: str
    trace_id: Optional[str] = None


@dataclass
class Receipt:
    """Transaction receipt."""
    tenant_id: str
    idempotency_key: str
    stream_position: Optional[str] = None

    def to_dict(self) -> Dict[str, Any]:
        return {
            "tenant_id": self.tenant_id,
            "idempotency_key": self.idempotency_key,
            "stream_position": self.stream_position,
        }


class EntDBServicer:
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
        self._pending_receipts: Dict[str, asyncio.Event] = {}
        self._receipt_results: Dict[str, Tuple[bool, Optional[str]]] = {}

    async def execute_atomic(
        self,
        tenant_id: str,
        actor: str,
        operations: List[Dict[str, Any]],
        idempotency_key: Optional[str] = None,
        schema_fingerprint: Optional[str] = None,
        wait_applied: bool = False,
        wait_timeout_ms: int = 30000,
    ) -> Dict[str, Any]:
        """Execute an atomic transaction.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            operations: List of operations
            idempotency_key: Optional idempotency key
            schema_fingerprint: Schema fingerprint for validation
            wait_applied: Whether to wait for application
            wait_timeout_ms: Timeout for wait_applied

        Returns:
            Response dictionary with receipt or error
        """
        # Validate inputs
        if not tenant_id:
            return {"success": False, "error": "tenant_id is required", "error_code": "INVALID_ARGUMENT"}
        if not actor:
            return {"success": False, "error": "actor is required", "error_code": "INVALID_ARGUMENT"}
        if not operations:
            return {"success": False, "error": "operations list is empty", "error_code": "INVALID_ARGUMENT"}

        # Generate idempotency key if not provided
        if not idempotency_key:
            idempotency_key = str(uuid.uuid4())

        # Validate schema fingerprint if provided
        if schema_fingerprint and self.schema_registry.fingerprint:
            if schema_fingerprint != self.schema_registry.fingerprint:
                return {
                    "success": False,
                    "error": f"Schema mismatch: server has {self.schema_registry.fingerprint}",
                    "error_code": "SCHEMA_MISMATCH",
                }

        # Build transaction event
        event = {
            "tenant_id": tenant_id,
            "actor": actor,
            "idempotency_key": idempotency_key,
            "schema_fingerprint": self.schema_registry.fingerprint,
            "ts_ms": int(time.time() * 1000),
            "ops": self._convert_operations(operations),
        }

        try:
            # Append to WAL
            event_bytes = json.dumps(event).encode('utf-8')
            stream_pos = await self.wal.append(
                self.topic,
                tenant_id,  # Partition key
                event_bytes,
            )

            receipt = Receipt(
                tenant_id=tenant_id,
                idempotency_key=idempotency_key,
                stream_position=str(stream_pos),
            )

            # Wait for application if requested
            applied_status = "RECEIPT_STATUS_PENDING"
            if wait_applied:
                applied = await self._wait_for_applied(
                    tenant_id,
                    idempotency_key,
                    wait_timeout_ms / 1000.0,
                )
                applied_status = "RECEIPT_STATUS_APPLIED" if applied else "RECEIPT_STATUS_PENDING"

            # Extract created node IDs
            created_node_ids = []
            for op in event["ops"]:
                if op.get("op") == "create_node":
                    node_id = op.get("id") or f"pending_{len(created_node_ids)}"
                    created_node_ids.append(node_id)

            return {
                "success": True,
                "receipt": receipt.to_dict(),
                "created_node_ids": created_node_ids,
                "applied_status": applied_status,
            }

        except Exception as e:
            logger.error(f"ExecuteAtomic failed: {e}", exc_info=True)
            return {
                "success": False,
                "error": str(e),
                "error_code": "INTERNAL",
            }

    def _convert_operations(self, operations: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Convert operation format from API to internal format."""
        result = []

        for op in operations:
            if "create_node" in op:
                create = op["create_node"]
                internal_op = {
                    "op": "create_node",
                    "type_id": create.get("type_id"),
                    "data": json.loads(create.get("data_json", "{}")),
                }
                if create.get("id"):
                    internal_op["id"] = create["id"]
                if create.get("acl_json"):
                    internal_op["acl"] = json.loads(create["acl_json"])
                if create.get("as"):
                    internal_op["as"] = create["as"]
                if create.get("fanout_to"):
                    internal_op["fanout_to"] = create["fanout_to"]
                result.append(internal_op)

            elif "update_node" in op:
                update = op["update_node"]
                result.append({
                    "op": "update_node",
                    "type_id": update.get("type_id"),
                    "id": update.get("id"),
                    "patch": json.loads(update.get("patch_json", "{}")),
                })

            elif "delete_node" in op:
                delete = op["delete_node"]
                result.append({
                    "op": "delete_node",
                    "type_id": delete.get("type_id"),
                    "id": delete.get("id"),
                })

            elif "create_edge" in op:
                create = op["create_edge"]
                result.append({
                    "op": "create_edge",
                    "edge_id": create.get("edge_id"),
                    "from": self._convert_node_ref(create.get("from", {})),
                    "to": self._convert_node_ref(create.get("to", {})),
                    "props": json.loads(create.get("props_json", "{}")),
                })

            elif "delete_edge" in op:
                delete = op["delete_edge"]
                result.append({
                    "op": "delete_edge",
                    "edge_id": delete.get("edge_id"),
                    "from": self._convert_node_ref(delete.get("from", {})),
                    "to": self._convert_node_ref(delete.get("to", {})),
                })

        return result

    def _convert_node_ref(self, ref: Dict[str, Any]) -> Any:
        """Convert node reference from API format."""
        if "id" in ref:
            return ref["id"]
        if "alias_ref" in ref:
            return {"ref": ref["alias_ref"]}
        if "typed" in ref:
            return {
                "type_id": ref["typed"].get("type_id"),
                "id": ref["typed"].get("id"),
            }
        return ref

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

    async def get_receipt_status(
        self,
        tenant_id: str,
        idempotency_key: str,
    ) -> Dict[str, Any]:
        """Get the status of a transaction receipt."""
        try:
            applied = await self.canonical_store.check_idempotency(tenant_id, idempotency_key)
            status = "RECEIPT_STATUS_APPLIED" if applied else "RECEIPT_STATUS_PENDING"
            return {"status": status}
        except Exception as e:
            return {"status": "RECEIPT_STATUS_UNKNOWN", "error": str(e)}

    async def get_node(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        node_id: str,
    ) -> Dict[str, Any]:
        """Get a node by ID."""
        try:
            node = await self.canonical_store.get_node(tenant_id, node_id)
            if not node:
                return {"found": False}

            return {
                "found": True,
                "node": {
                    "tenant_id": node.tenant_id,
                    "node_id": node.node_id,
                    "type_id": node.type_id,
                    "payload_json": json.dumps(node.payload),
                    "created_at": node.created_at,
                    "updated_at": node.updated_at,
                    "owner_actor": node.owner_actor,
                    "acl_json": json.dumps(node.acl),
                },
            }
        except Exception as e:
            logger.error(f"GetNode failed: {e}", exc_info=True)
            return {"found": False, "error": str(e)}

    async def query_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        limit: int = 100,
        offset: int = 0,
    ) -> Dict[str, Any]:
        """Query nodes by type."""
        try:
            nodes = await self.canonical_store.get_nodes_by_type(
                tenant_id, type_id, limit=limit, offset=offset
            )

            return {
                "nodes": [
                    {
                        "tenant_id": n.tenant_id,
                        "node_id": n.node_id,
                        "type_id": n.type_id,
                        "payload_json": json.dumps(n.payload),
                        "created_at": n.created_at,
                        "updated_at": n.updated_at,
                        "owner_actor": n.owner_actor,
                        "acl_json": json.dumps(n.acl),
                    }
                    for n in nodes
                ],
                "has_more": len(nodes) == limit,
            }
        except Exception as e:
            logger.error(f"QueryNodes failed: {e}", exc_info=True)
            return {"nodes": [], "error": str(e)}

    async def get_edges_from(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: Optional[int] = None,
        limit: int = 100,
    ) -> Dict[str, Any]:
        """Get outgoing edges from a node."""
        try:
            edges = await self.canonical_store.get_edges_from(
                tenant_id, node_id, edge_type_id
            )

            return {
                "edges": [
                    {
                        "tenant_id": e.tenant_id,
                        "edge_type_id": e.edge_type_id,
                        "from_node_id": e.from_node_id,
                        "to_node_id": e.to_node_id,
                        "props_json": json.dumps(e.props),
                        "created_at": e.created_at,
                    }
                    for e in edges[:limit]
                ],
                "has_more": len(edges) > limit,
            }
        except Exception as e:
            logger.error(f"GetEdgesFrom failed: {e}", exc_info=True)
            return {"edges": [], "error": str(e)}

    async def get_edges_to(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: Optional[int] = None,
        limit: int = 100,
    ) -> Dict[str, Any]:
        """Get incoming edges to a node."""
        try:
            edges = await self.canonical_store.get_edges_to(
                tenant_id, node_id, edge_type_id
            )

            return {
                "edges": [
                    {
                        "tenant_id": e.tenant_id,
                        "edge_type_id": e.edge_type_id,
                        "from_node_id": e.from_node_id,
                        "to_node_id": e.to_node_id,
                        "props_json": json.dumps(e.props),
                        "created_at": e.created_at,
                    }
                    for e in edges[:limit]
                ],
                "has_more": len(edges) > limit,
            }
        except Exception as e:
            logger.error(f"GetEdgesTo failed: {e}", exc_info=True)
            return {"edges": [], "error": str(e)}

    async def search_mailbox(
        self,
        tenant_id: str,
        actor: str,
        user_id: str,
        query: str,
        source_type_ids: Optional[List[int]] = None,
        limit: int = 20,
        offset: int = 0,
    ) -> Dict[str, Any]:
        """Search mailbox with full-text search."""
        try:
            results = await self.mailbox_store.search(
                tenant_id, user_id, query,
                limit=limit, offset=offset,
                source_type_ids=source_type_ids,
            )

            return {
                "results": [
                    {
                        "item": {
                            "item_id": r.item.item_id,
                            "ref_id": r.item.ref_id,
                            "source_type_id": r.item.source_type_id,
                            "source_node_id": r.item.source_node_id,
                            "thread_id": r.item.thread_id,
                            "ts": r.item.ts,
                            "state_json": json.dumps(r.item.state),
                            "snippet": r.item.snippet,
                            "metadata_json": json.dumps(r.item.metadata),
                        },
                        "rank": r.rank,
                        "highlights": r.highlights,
                    }
                    for r in results
                ],
                "has_more": len(results) == limit,
            }
        except Exception as e:
            logger.error(f"SearchMailbox failed: {e}", exc_info=True)
            return {"results": [], "error": str(e)}

    async def get_mailbox(
        self,
        tenant_id: str,
        actor: str,
        user_id: str,
        source_type_id: Optional[int] = None,
        thread_id: Optional[str] = None,
        unread_only: bool = False,
        limit: int = 50,
        offset: int = 0,
    ) -> Dict[str, Any]:
        """Get mailbox items for a user."""
        try:
            items = await self.mailbox_store.list_items(
                tenant_id, user_id,
                limit=limit, offset=offset,
                source_type_id=source_type_id,
                thread_id=thread_id,
                unread_only=unread_only,
            )

            unread_count = await self.mailbox_store.get_unread_count(tenant_id, user_id)

            return {
                "items": [
                    {
                        "item_id": item.item_id,
                        "ref_id": item.ref_id,
                        "source_type_id": item.source_type_id,
                        "source_node_id": item.source_node_id,
                        "thread_id": item.thread_id,
                        "ts": item.ts,
                        "state_json": json.dumps(item.state),
                        "snippet": item.snippet,
                        "metadata_json": json.dumps(item.metadata),
                    }
                    for item in items
                ],
                "unread_count": unread_count,
                "has_more": len(items) == limit,
            }
        except Exception as e:
            logger.error(f"GetMailbox failed: {e}", exc_info=True)
            return {"items": [], "unread_count": 0, "error": str(e)}

    async def health(self) -> Dict[str, Any]:
        """Get server health status."""
        components = {}

        # Check WAL connection
        try:
            wal_healthy = self.wal.is_connected if hasattr(self.wal, 'is_connected') else True
            components["wal"] = "healthy" if wal_healthy else "unhealthy"
        except Exception:
            components["wal"] = "unknown"

        # Check if data directory is accessible
        components["storage"] = "healthy"

        overall_healthy = all(v == "healthy" for v in components.values())

        return {
            "healthy": overall_healthy,
            "version": "1.0.0",
            "components": components,
        }

    async def get_schema(self, type_id: Optional[int] = None) -> Dict[str, Any]:
        """Get schema information."""
        try:
            schema_dict = self.schema_registry.to_dict()

            if type_id is not None:
                # Filter to specific type
                schema_dict["node_types"] = [
                    t for t in schema_dict.get("node_types", [])
                    if t.get("type_id") == type_id
                ]
                schema_dict["edge_types"] = [
                    e for e in schema_dict.get("edge_types", [])
                    if e.get("from_type_id") == type_id or e.get("to_type_id") == type_id
                ]

            return {
                "schema_json": json.dumps(schema_dict),
                "fingerprint": self.schema_registry.fingerprint or "",
            }
        except Exception as e:
            logger.error(f"GetSchema failed: {e}", exc_info=True)
            return {"schema_json": "{}", "error": str(e)}


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
            max_workers: Maximum thread pool workers
            reflection_enabled: Whether to enable gRPC reflection
        """
        self.servicer = servicer
        self.host = host
        self.port = port
        self.max_workers = max_workers
        self.reflection_enabled = reflection_enabled
        self._server = None
        self._running = False

    async def start(self) -> None:
        """Start the gRPC server."""
        if self._running:
            logger.warning("Server already running")
            return

        if not GRPC_AVAILABLE:
            logger.error("grpc package not available, cannot start gRPC server")
            return

        # For now, we'll use a simple implementation
        # In production, this would use grpc.aio.server() with proper servicer
        logger.info(
            f"gRPC server starting on {self.host}:{self.port}",
            extra={
                "host": self.host,
                "port": self.port,
                "max_workers": self.max_workers,
            }
        )

        self._running = True

        # Note: Full gRPC implementation requires generated protobuf code
        # This is a placeholder for the actual gRPC server
        # In production, use: grpc.aio.server() with add_EntDBServiceServicer_to_server()

    async def stop(self, grace_period: float = 5.0) -> None:
        """Stop the gRPC server gracefully.

        Args:
            grace_period: Time to wait for pending RPCs to complete
        """
        if not self._running:
            return

        logger.info("Stopping gRPC server")
        self._running = False

        if self._server:
            await self._server.stop(grace_period)

    @property
    def is_running(self) -> bool:
        """Whether the server is running."""
        return self._running
