"""
EntDB Client for Python SDK.

This module provides the main client interface:
- DbClient: Connection to EntDB server
- Plan: Atomic transaction builder
- Receipt: Transaction receipt for status checking

Example:
    >>> async with DbClient("localhost:50051") as db:
    ...     plan = db.atomic("tenant_1", "user:42")
    ...     plan.create(Task, {"title": "My Task"})
    ...     result = await plan.commit()

Invariants:
    - All operations require tenant_id and actor
    - Writes are batched and committed atomically
    - Reads are independent of the transaction
"""

from __future__ import annotations

import json
import logging
import uuid
from dataclasses import dataclass, field
from typing import Any

from ._grpc_client import GrpcClient
from .errors import (
    ConnectionError,
    UnknownFieldError,
    ValidationError,
)
from .registry import SchemaRegistry, get_registry
from .schema import EdgeTypeDef, NodeTypeDef

logger = logging.getLogger(__name__)


@dataclass
class Receipt:
    """Transaction receipt for status tracking.

    Attributes:
        tenant_id: Tenant identifier
        idempotency_key: Unique transaction key
        stream_position: Position in WAL stream
    """

    tenant_id: str
    idempotency_key: str
    stream_position: str | None = None


@dataclass
class Node:
    """A node from the database.

    Attributes:
        tenant_id: Tenant identifier
        node_id: Unique node ID
        type_id: Node type ID
        payload: Field values
        created_at: Creation timestamp (Unix ms)
        updated_at: Last update timestamp (Unix ms)
        owner_actor: Creator
        acl: Access control list
    """

    tenant_id: str
    node_id: str
    type_id: int
    payload: dict[str, Any]
    created_at: int
    updated_at: int
    owner_actor: str
    acl: list[dict[str, str]] = field(default_factory=list)


@dataclass
class Edge:
    """An edge from the database.

    Attributes:
        tenant_id: Tenant identifier
        edge_type_id: Edge type ID
        from_node_id: Source node
        to_node_id: Target node
        props: Edge properties
        created_at: Creation timestamp
    """

    tenant_id: str
    edge_type_id: int
    from_node_id: str
    to_node_id: str
    props: dict[str, Any]
    created_at: int


@dataclass
class CommitResult:
    """Result of committing a Plan.

    Attributes:
        success: Whether commit succeeded
        receipt: Transaction receipt
        created_node_ids: IDs of created nodes
        applied: Whether event has been applied
        error: Error message if failed
    """

    success: bool
    receipt: Receipt | None = None
    created_node_ids: list[str] = field(default_factory=list)
    applied: bool = False
    error: str | None = None


class Plan:
    """Atomic transaction builder.

    A Plan collects operations to be executed atomically.
    Operations are validated locally before sending to server.

    Example:
        >>> plan = db.atomic("tenant_1", "user:42")
        >>> plan.create(Task, {"title": "My Task"}, as_="t")
        >>> plan.edge_create(AssignedTo, from_="$t.id", to={"type_id": 1, "id": "user:7"})
        >>> result = await plan.commit()
    """

    def __init__(
        self,
        client: DbClient,
        tenant_id: str,
        actor: str,
        idempotency_key: str | None = None,
        trace_id: str | None = None,
    ) -> None:
        """Initialize a plan.

        Args:
            client: DbClient instance
            tenant_id: Tenant identifier
            actor: Actor performing operations
            idempotency_key: Optional unique key for deduplication
            trace_id: Optional trace ID for distributed tracing
        """
        self._client = client
        self._tenant_id = tenant_id
        self._actor = actor
        self._idempotency_key = idempotency_key or str(uuid.uuid4())
        self._trace_id = trace_id or str(uuid.uuid4())
        self._operations: list[dict[str, Any]] = []
        self._committed = False

    def _ensure_not_committed(self) -> None:
        """Raise if this Plan has already been committed."""
        if self._committed:
            raise RuntimeError(
                "Plan has already been committed. Create a new Plan for "
                "additional operations."
            )

    def create(
        self,
        node_type: NodeTypeDef,
        data: dict[str, Any] | None = None,
        *,
        acl: list[dict[str, str]] | None = None,
        as_: str | None = None,
        fanout_to: list[str] | None = None,
        **kwargs: Any,
    ) -> Plan:
        """Add a create_node operation.

        Args:
            node_type: Type of node to create
            data: Payload dictionary (or use kwargs)
            acl: Access control list
            as_: Alias for referencing in later operations
            fanout_to: Users to fanout to
            **kwargs: Field values (alternative to data)

        Returns:
            Self for chaining

        Raises:
            RuntimeError: If Plan has already been committed
            ValidationError: If payload validation fails
            UnknownFieldError: If unknown field is provided
        """
        self._ensure_not_committed()

        # Merge data and kwargs
        payload = dict(data or {})
        payload.update(kwargs)

        # Validate payload
        is_valid, errors = node_type.validate_payload(payload)
        if not is_valid:
            # Check for unknown fields
            known = {f.name for f in node_type.fields}
            unknown = set(payload.keys()) - known
            if unknown:
                field_name = list(unknown)[0]
                suggestions = [n for n in known if field_name.lower() in n.lower()][:3]
                raise UnknownFieldError(field_name, node_type.name, suggestions)
            raise ValidationError("; ".join(errors), errors=errors)

        op: dict[str, Any] = {
            "create_node": {
                "type_id": node_type.type_id,
                "data_json": json.dumps(payload),
            }
        }

        if acl:
            op["create_node"]["acl_json"] = json.dumps(acl)
        if as_:
            op["create_node"]["as"] = as_
        if fanout_to:
            op["create_node"]["fanout_to"] = fanout_to

        self._operations.append(op)
        return self

    def update(
        self,
        node_type: NodeTypeDef,
        node_id: str,
        patch: dict[str, Any],
        *,
        field_mask: list[str] | None = None,
    ) -> Plan:
        """Add an update_node operation.

        Args:
            node_type: Type of node to update
            node_id: ID of node to update
            patch: Fields to update
            field_mask: Optional explicit field mask

        Returns:
            Self for chaining
        """
        self._ensure_not_committed()

        op: dict[str, Any] = {
            "update_node": {
                "type_id": node_type.type_id,
                "id": node_id,
                "patch_json": json.dumps(patch),
            }
        }

        if field_mask:
            op["update_node"]["field_mask"] = field_mask

        self._operations.append(op)
        return self

    def delete(
        self,
        node_type: NodeTypeDef,
        node_id: str,
    ) -> Plan:
        """Add a delete_node operation.

        Args:
            node_type: Type of node to delete
            node_id: ID of node to delete

        Returns:
            Self for chaining
        """
        self._ensure_not_committed()

        self._operations.append(
            {
                "delete_node": {
                    "type_id": node_type.type_id,
                    "id": node_id,
                }
            }
        )
        return self

    def edge_create(
        self,
        edge_type: EdgeTypeDef,
        from_: str | dict[str, Any],
        to: str | dict[str, Any],
        props: dict[str, Any] | None = None,
    ) -> Plan:
        """Add a create_edge operation.

        Args:
            edge_type: Type of edge to create
            from_: Source node (ID, alias ref, or typed ref)
            to: Target node
            props: Edge properties

        Returns:
            Self for chaining
        """
        self._ensure_not_committed()

        # Validate props
        if props:
            is_valid, errors = edge_type.validate_props(props)
            if not is_valid:
                raise ValidationError("; ".join(errors), errors=errors)

        op: dict[str, Any] = {
            "create_edge": {
                "edge_id": edge_type.edge_id,
                "from": self._convert_ref(from_),
                "to": self._convert_ref(to),
            }
        }

        if props:
            op["create_edge"]["props_json"] = json.dumps(props)

        self._operations.append(op)
        return self

    def edge_delete(
        self,
        edge_type: EdgeTypeDef,
        from_: str | dict[str, Any],
        to: str | dict[str, Any],
    ) -> Plan:
        """Add a delete_edge operation.

        Args:
            edge_type: Type of edge to delete
            from_: Source node
            to: Target node

        Returns:
            Self for chaining
        """
        self._ensure_not_committed()

        self._operations.append(
            {
                "delete_edge": {
                    "edge_id": edge_type.edge_id,
                    "from": self._convert_ref(from_),
                    "to": self._convert_ref(to),
                }
            }
        )
        return self

    def _convert_ref(self, ref: str | dict[str, Any]) -> dict[str, Any]:
        """Convert reference to API format."""
        if isinstance(ref, str):
            if ref.startswith("$"):
                return {"alias_ref": ref}
            return {"id": ref}
        if "type_id" in ref and "id" in ref:
            return {"typed": ref}
        return ref

    async def commit(
        self, wait_applied: bool = False, *, timeout: float | None = None
    ) -> CommitResult:
        """Commit the transaction.

        Args:
            wait_applied: Whether to wait for application
            timeout: Per-call timeout in seconds

        Returns:
            CommitResult indicating success/failure

        Raises:
            RuntimeError: If commit() has already been called on this Plan
        """
        if self._committed:
            raise RuntimeError(
                "Plan has already been committed. Create a new Plan for "
                "additional operations."
            )

        if not self._operations:
            self._committed = True
            return CommitResult(success=True, created_node_ids=[])

        result = await self._client._execute(
            tenant_id=self._tenant_id,
            actor=self._actor,
            operations=self._operations,
            idempotency_key=self._idempotency_key,
            wait_applied=wait_applied,
            trace_id=self._trace_id,
            timeout=timeout,
        )

        self._committed = True
        return result


class DbClient:
    """Client for connecting to EntDB server.

    Provides a clean Python API for interacting with EntDB.
    Handles connection management and exposes high-level operations.

    Example:
        >>> async with DbClient("localhost:50051") as db:
        ...     node = await db.get(Task, "node_123", "tenant_1", "user:42")
    """

    def __init__(
        self,
        address: str,
        *,
        secure: bool = False,
        api_key: str | None = None,
        max_retries: int = 3,
        registry: SchemaRegistry | None = None,
    ) -> None:
        """Initialize client.

        Args:
            address: Server address (host:port or just host)
            secure: Whether to use TLS
            api_key: Optional API key for authentication
            max_retries: Maximum number of retries for transient gRPC failures
            registry: Optional schema registry
        """
        # Parse address
        if ":" in address:
            host, port_str = address.rsplit(":", 1)
            port = int(port_str)
        else:
            host = address
            port = 50051  # Default gRPC port

        self._grpc = GrpcClient(
            host=host,
            port=port,
            secure=secure,
            api_key=api_key,
            max_retries=max_retries,
        )
        self.registry = registry or get_registry()
        self._connected = False

    def _ensure_connected(self) -> None:
        """Raise if not connected.

        Raises:
            ConnectionError: If the client is not connected
        """
        if not self._connected:
            raise ConnectionError(
                "Not connected. Use 'async with DbClient(...)' or call "
                "'await client.connect()' first.",
                address=f"{self._grpc._host}:{self._grpc._port}",
            )

    async def connect(self) -> None:
        """Connect to the server."""
        if self._connected:
            return

        try:
            await self._grpc.connect()
            self._connected = True
        except Exception as e:
            raise ConnectionError(
                f"Failed to connect: {e}",
                address=f"{self._grpc._host}:{self._grpc._port}",
            ) from e

    async def close(self) -> None:
        """Close the connection."""
        if self._connected:
            await self._grpc.close()
            self._connected = False

    async def __aenter__(self) -> DbClient:
        await self.connect()
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()

    def atomic(
        self,
        tenant_id: str,
        actor: str,
        idempotency_key: str | None = None,
        *,
        trace_id: str | None = None,
    ) -> Plan:
        """Create an atomic transaction plan.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing operations
            idempotency_key: Optional deduplication key
            trace_id: Optional trace ID for distributed tracing

        Returns:
            Plan builder

        Example:
            >>> plan = db.atomic("tenant_1", "user:42")
            >>> plan.create(Task, {"title": "New Task"})
            >>> await plan.commit()
        """
        self._ensure_connected()
        return Plan(self, tenant_id, actor, idempotency_key, trace_id=trace_id)

    async def get(
        self,
        node_type: NodeTypeDef,
        node_id: str,
        tenant_id: str,
        actor: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Get a node by ID.

        Args:
            node_type: Expected node type
            node_id: Node identifier
            tenant_id: Tenant identifier
            actor: Actor making request
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Node if found, None otherwise
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        grpc_node = await self._grpc.get_node(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            node_id=node_id,
            trace_id=trace_id,
            timeout=timeout,
        )

        if grpc_node is None:
            return None

        return Node(
            tenant_id=grpc_node.tenant_id,
            node_id=grpc_node.node_id,
            type_id=grpc_node.type_id,
            payload=grpc_node.payload,
            created_at=grpc_node.created_at,
            updated_at=grpc_node.updated_at,
            owner_actor=grpc_node.owner_actor,
            acl=grpc_node.acl,
        )

    async def get_many(
        self,
        node_type: NodeTypeDef,
        node_ids: list[str],
        tenant_id: str,
        actor: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> tuple[list[Node], list[str]]:
        """Get multiple nodes by IDs.

        Args:
            node_type: Expected node type
            node_ids: Node identifiers
            tenant_id: Tenant identifier
            actor: Actor making request
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (found nodes, missing node IDs)
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        grpc_nodes, missing = await self._grpc.get_nodes(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            node_ids=node_ids,
            trace_id=trace_id,
            timeout=timeout,
        )

        nodes = [
            Node(
                tenant_id=n.tenant_id,
                node_id=n.node_id,
                type_id=n.type_id,
                payload=n.payload,
                created_at=n.created_at,
                updated_at=n.updated_at,
                owner_actor=n.owner_actor,
                acl=n.acl,
            )
            for n in grpc_nodes
        ]

        return nodes, missing

    async def query(
        self,
        node_type: NodeTypeDef,
        tenant_id: str,
        actor: str,
        *,
        filter: dict[str, Any] | None = None,
        limit: int = 100,
        offset: int = 0,
        order_by: str = "created_at",
        descending: bool = True,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Query nodes by type.

        Args:
            node_type: Node type to query
            tenant_id: Tenant identifier
            actor: Actor making request
            filter: Optional payload field filter (serialized as JSON)
            limit: Maximum nodes to return
            offset: Pagination offset
            order_by: Field to order results by
            descending: Whether to sort in descending order
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of nodes
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        grpc_nodes, _ = await self._grpc.query_nodes(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            limit=limit,
            offset=offset,
            filter_json=json.dumps(filter) if filter else "",
            order_by=order_by,
            descending=descending,
            trace_id=trace_id,
            timeout=timeout,
        )

        return [
            Node(
                tenant_id=n.tenant_id,
                node_id=n.node_id,
                type_id=n.type_id,
                payload=n.payload,
                created_at=n.created_at,
                updated_at=n.updated_at,
                owner_actor=n.owner_actor,
                acl=n.acl,
            )
            for n in grpc_nodes
        ]

    async def edges_out(
        self,
        node_id: str,
        tenant_id: str,
        actor: str,
        edge_type: EdgeTypeDef | None = None,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        """Get outgoing edges from a node.

        Args:
            node_id: Source node ID
            tenant_id: Tenant identifier
            actor: Actor making request
            edge_type: Optional edge type filter
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of edges
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        edge_type_id = edge_type.edge_id if edge_type else None
        grpc_edges, _ = await self._grpc.get_edges_from(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            edge_type_id=edge_type_id,
            trace_id=trace_id,
            timeout=timeout,
        )

        return [
            Edge(
                tenant_id=e.tenant_id,
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                props=e.props,
                created_at=e.created_at,
            )
            for e in grpc_edges
        ]

    async def edges_in(
        self,
        node_id: str,
        tenant_id: str,
        actor: str,
        edge_type: EdgeTypeDef | None = None,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        """Get incoming edges to a node.

        Args:
            node_id: Target node ID
            tenant_id: Tenant identifier
            actor: Actor making request
            edge_type: Optional edge type filter
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of edges
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        edge_type_id = edge_type.edge_id if edge_type else None
        grpc_edges, _ = await self._grpc.get_edges_to(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            edge_type_id=edge_type_id,
            trace_id=trace_id,
            timeout=timeout,
        )

        return [
            Edge(
                tenant_id=e.tenant_id,
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                props=e.props,
                created_at=e.created_at,
            )
            for e in grpc_edges
        ]

    async def search(
        self,
        query: str,
        user_id: str,
        tenant_id: str,
        actor: str,
        types: list[NodeTypeDef] | None = None,
        limit: int = 20,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """Search user's mailbox.

        Args:
            query: Search query
            user_id: User whose mailbox to search
            tenant_id: Tenant identifier
            actor: Actor making request
            types: Optional type filter
            limit: Maximum results
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of search results
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        source_type_ids = [t.type_id for t in types] if types else None

        return await self._grpc.search_mailbox(
            tenant_id=tenant_id,
            actor=actor,
            user_id=user_id,
            query=query,
            source_type_ids=source_type_ids,
            limit=limit,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def health(
        self,
        *,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Check server health.

        Args:
            timeout: Per-call timeout in seconds

        Returns:
            Health status dictionary
        """
        self._ensure_connected()
        return await self._grpc.health(timeout=timeout)

    async def _execute(
        self,
        tenant_id: str,
        actor: str,
        operations: list[dict[str, Any]],
        idempotency_key: str,
        wait_applied: bool = False,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> CommitResult:
        """Execute atomic transaction.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing operations
            operations: List of operations
            idempotency_key: Deduplication key
            wait_applied: Wait for application
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            CommitResult
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        result = await self._grpc.execute_atomic(
            tenant_id=tenant_id,
            actor=actor,
            operations=operations,
            idempotency_key=idempotency_key,
            schema_fingerprint=self.registry.fingerprint,
            wait_applied=wait_applied,
            trace_id=trace_id,
            timeout=timeout,
        )

        if not result.success:
            return CommitResult(
                success=False,
                error=result.error,
            )

        receipt = None
        if result.receipt:
            receipt = Receipt(
                tenant_id=result.receipt.tenant_id,
                idempotency_key=result.receipt.idempotency_key,
                stream_position=result.receipt.stream_position,
            )

        return CommitResult(
            success=True,
            receipt=receipt,
            created_node_ids=result.created_node_ids,
            applied=result.applied,
        )

    async def get_receipt_status(
        self,
        tenant_id: str,
        idempotency_key: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> str:
        """Get receipt status.

        Args:
            tenant_id: Tenant identifier
            idempotency_key: Transaction key
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Status string (PENDING, APPLIED, FAILED)
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.get_receipt_status(
            tenant_id,
            idempotency_key,
            trace_id=trace_id,
            timeout=timeout,
        )
