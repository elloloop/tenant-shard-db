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

import logging
import uuid
from dataclasses import dataclass, field
from enum import Enum
from typing import Any


class _Unset(Enum):
    """Sentinel for distinguishing 'not provided' from explicit None."""

    TOKEN = 0


_UNSET = _Unset.TOKEN

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
                "Plan has already been committed. Create a new Plan for additional operations."
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
                "data": payload,
            }
        }

        if acl:
            op["create_node"]["acl"] = acl
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
                "patch": patch,
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
            op["create_edge"]["props"] = props

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
                "Plan has already been committed. Create a new Plan for additional operations."
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
        self._last_offsets: dict[str, str] = {}  # tenant_id -> stream_position

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

    def clear_offsets(self) -> None:
        """Clear tracked offsets. Useful for testing."""
        self._last_offsets.clear()

    def _resolve_offset(self, tenant_id: str, after_offset: str | None | _Unset) -> str | None:
        """Resolve the after_offset for a read operation.

        If after_offset is _UNSET (not provided by caller), use the tracked
        offset for this tenant. If explicitly None, return None (opt-out).
        If a specific string, use that value.
        """
        if isinstance(after_offset, _Unset):
            if not hasattr(self, "_last_offsets"):
                return None
            return self._last_offsets.get(tenant_id)
        return after_offset

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

    async def wait_for_offset(
        self,
        tenant_id: str,
        stream_position: str,
        timeout_ms: int = 30000,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> tuple[bool, str]:
        """Wait for a stream position to be applied.

        Args:
            tenant_id: Tenant identifier
            stream_position: Target stream position string
            timeout_ms: Server-side wait timeout in milliseconds
            trace_id: Optional trace ID
            timeout: Per-call gRPC timeout in seconds

        Returns:
            Tuple of (reached, current_position)
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.wait_for_offset(
            tenant_id=tenant_id,
            stream_position=stream_position,
            timeout_ms=timeout_ms,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def get(
        self,
        node_type: NodeTypeDef,
        node_id: str,
        tenant_id: str,
        actor: str,
        *,
        after_offset: str | None | _Unset = _UNSET,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Get a node by ID.

        Args:
            node_type: Expected node type
            node_id: Node identifier
            tenant_id: Tenant identifier
            actor: Actor making request
            after_offset: Wait for this stream position before reading.
                Omit to use automatic offset tracking.
                Pass None to opt out of offset tracking.
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Node if found, None otherwise
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        resolved_offset = self._resolve_offset(tenant_id, after_offset)

        grpc_node = await self._grpc.get_node(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            node_id=node_id,
            after_offset=resolved_offset,
            wait_timeout_ms=30000 if resolved_offset else 0,
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
        after_offset: str | None | _Unset = _UNSET,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> tuple[list[Node], list[str]]:
        """Get multiple nodes by IDs.

        Args:
            node_type: Expected node type
            node_ids: Node identifiers
            tenant_id: Tenant identifier
            actor: Actor making request
            after_offset: Wait for this stream position before reading.
                Omit to use automatic offset tracking.
                Pass None to opt out of offset tracking.
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (found nodes, missing node IDs)
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        resolved_offset = self._resolve_offset(tenant_id, after_offset)

        grpc_nodes, missing = await self._grpc.get_nodes(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            node_ids=node_ids,
            after_offset=resolved_offset,
            wait_timeout_ms=30000 if resolved_offset else 0,
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
        after_offset: str | None | _Unset = _UNSET,
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
            after_offset: Wait for this stream position before reading.
                Omit to use automatic offset tracking.
                Pass None to opt out of offset tracking.
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of nodes
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        resolved_offset = self._resolve_offset(tenant_id, after_offset)

        grpc_nodes, _ = await self._grpc.query_nodes(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            limit=limit,
            offset=offset,
            filter=filter or {},
            order_by=order_by,
            descending=descending,
            after_offset=resolved_offset,
            wait_timeout_ms=30000 if resolved_offset else 0,
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
        after_offset: str | None | _Unset = _UNSET,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        """Get outgoing edges from a node.

        Args:
            node_id: Source node ID
            tenant_id: Tenant identifier
            actor: Actor making request
            edge_type: Optional edge type filter
            after_offset: Wait for this stream position before reading.
                Omit to use automatic offset tracking.
                Pass None to opt out of offset tracking.
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of edges
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        resolved_offset = self._resolve_offset(tenant_id, after_offset)
        _ = resolved_offset  # reserved for gRPC support

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
        after_offset: str | None | _Unset = _UNSET,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        """Get incoming edges to a node.

        Args:
            node_id: Target node ID
            tenant_id: Tenant identifier
            actor: Actor making request
            edge_type: Optional edge type filter
            after_offset: Wait for this stream position before reading.
                Omit to use automatic offset tracking.
                Pass None to opt out of offset tracking.
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of edges
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        resolved_offset = self._resolve_offset(tenant_id, after_offset)
        _ = resolved_offset  # reserved for gRPC support

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
            # Track last write offset per tenant for read-after-write consistency
            if result.receipt.stream_position:
                if not hasattr(self, "_last_offsets"):
                    self._last_offsets = {}
                self._last_offsets[tenant_id] = result.receipt.stream_position

        return CommitResult(
            success=True,
            receipt=receipt,
            created_node_ids=result.created_node_ids,
            applied=result.applied,
        )

    # --- ACL v2 methods ---

    async def connected(
        self,
        node_id: str,
        edge_type: EdgeTypeDef,
        tenant_id: str,
        actor: str,
        *,
        limit: int = 100,
        offset: int = 0,
        after_offset: str | None | _Unset = _UNSET,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Get connected nodes via edge type with ACL filtering.

        Args:
            node_id: Source node ID
            edge_type: Edge type to traverse
            tenant_id: Tenant identifier
            actor: Actor making request
            limit: Maximum nodes to return
            offset: Pagination offset
            after_offset: Wait for this stream position before reading.
                Omit to use automatic offset tracking.
                Pass None to opt out of offset tracking.
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of connected nodes the actor can see
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        resolved_offset = self._resolve_offset(tenant_id, after_offset)
        _ = resolved_offset  # reserved for gRPC support

        grpc_nodes, _ = await self._grpc.get_connected_nodes(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            edge_type_id=edge_type.edge_id,
            limit=limit,
            offset=offset,
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

    async def share(
        self,
        node_id: str,
        actor_id: str,
        tenant_id: str,
        actor: str,
        permission: str = "read",
        *,
        actor_type: str = "user",
        expires_at: int | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        """Share a node with an actor.

        Args:
            node_id: Node to share
            actor_id: Actor to share with
            tenant_id: Tenant identifier
            actor: Actor performing the share (granted_by)
            permission: Permission level (read, write, admin)
            actor_type: Type of the target actor (user, group)
            expires_at: Optional expiry timestamp (Unix ms)
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if successful
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.share_node(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            actor_id=actor_id,
            permission=permission,
            actor_type=actor_type,
            expires_at=expires_at,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def revoke(
        self,
        node_id: str,
        actor_id: str,
        tenant_id: str,
        actor: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        """Revoke access from an actor on a node.

        Args:
            node_id: Node to revoke access from
            actor_id: Actor to revoke
            tenant_id: Tenant identifier
            actor: Actor performing the revocation
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if a grant existed and was removed
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.revoke_access(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            actor_id=actor_id,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def shared_with_me(
        self,
        tenant_id: str,
        actor: str,
        *,
        limit: int = 100,
        offset: int = 0,
        after_offset: str | None | _Unset = _UNSET,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """List nodes shared with the calling actor.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            limit: Maximum nodes to return
            offset: Pagination offset
            after_offset: Wait for this stream position before reading.
                Omit to use automatic offset tracking.
                Pass None to opt out of offset tracking.
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of shared nodes
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        resolved_offset = self._resolve_offset(tenant_id, after_offset)
        _ = resolved_offset  # reserved for gRPC support

        grpc_nodes, _ = await self._grpc.list_shared_with_me(
            tenant_id=tenant_id,
            actor=actor,
            limit=limit,
            offset=offset,
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

    async def group_add(
        self,
        group_id: str,
        member_actor_id: str,
        tenant_id: str,
        actor: str,
        role: str = "member",
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        """Add a member to a group.

        Args:
            group_id: Group to add member to
            member_actor_id: Actor to add
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            role: Role in the group
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if successful
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.add_group_member(
            tenant_id=tenant_id,
            actor=actor,
            group_id=group_id,
            member_actor_id=member_actor_id,
            role=role,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def group_remove(
        self,
        group_id: str,
        member_actor_id: str,
        tenant_id: str,
        actor: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        """Remove a member from a group.

        Args:
            group_id: Group to remove member from
            member_actor_id: Actor to remove
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if member existed and was removed
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.remove_group_member(
            tenant_id=tenant_id,
            actor=actor,
            group_id=group_id,
            member_actor_id=member_actor_id,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def transfer_ownership(
        self,
        node_id: str,
        new_owner: str,
        tenant_id: str,
        actor: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        """Transfer ownership of a node.

        Args:
            node_id: Node to transfer
            new_owner: New owner actor
            tenant_id: Tenant identifier
            actor: Actor performing the transfer
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if node existed and ownership was transferred
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.transfer_ownership(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            new_owner=new_owner,
            trace_id=trace_id,
            timeout=timeout,
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

    # --- User registry methods ---

    async def create_user(
        self,
        user_id: str,
        email: str,
        name: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Create a new user in the global registry.

        Args:
            user_id: Unique user identifier
            email: User email address
            name: Display name
            actor: Actor performing the operation (must be admin or system)
            timeout: Per-call timeout in seconds

        Returns:
            Dict with 'success', 'user' (if created), and 'error' (if failed)
        """
        self._ensure_connected()
        return await self._grpc.create_user(
            user_id=user_id,
            email=email,
            name=name,
            actor=actor,
            timeout=timeout,
        )

    async def get_user(
        self,
        user_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any] | None:
        """Get a user by ID.

        Args:
            user_id: User identifier
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            User dict or None if not found
        """
        self._ensure_connected()
        return await self._grpc.get_user(
            user_id=user_id,
            actor=actor,
            timeout=timeout,
        )

    async def update_user(
        self,
        user_id: str,
        *,
        name: str = "",
        email: str = "",
        status: str = "",
        actor: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Update user fields.

        Args:
            user_id: User identifier
            name: New display name (empty = no change)
            email: New email (empty = no change)
            status: New status (empty = no change)
            actor: Actor performing the update (defaults to user:user_id)
            timeout: Per-call timeout in seconds

        Returns:
            Dict with 'success' and optional 'error'
        """
        self._ensure_connected()
        return await self._grpc.update_user(
            user_id=user_id,
            name=name,
            email=email,
            status=status,
            actor=actor,
            timeout=timeout,
        )

    async def list_users(
        self,
        *,
        status: str = "active",
        limit: int = 100,
        offset: int = 0,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """List users filtered by status.

        Args:
            status: Status filter (e.g. 'active', 'suspended')
            limit: Maximum results to return
            offset: Pagination offset
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            List of user dicts
        """
        self._ensure_connected()
        return await self._grpc.list_users(
            status=status,
            limit=limit,
            offset=offset,
            actor=actor,
            timeout=timeout,
        )

    # --- Tenant registry methods ---

    async def create_tenant(
        self,
        tenant_id: str,
        name: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Create a new tenant in the global registry.

        Args:
            tenant_id: Unique tenant identifier
            name: Tenant display name
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, tenant, and error fields
        """
        self._ensure_connected()
        return await self._grpc.create_tenant(
            actor=actor,
            tenant_id=tenant_id,
            name=name,
            timeout=timeout,
        )

    async def get_tenant(
        self,
        tenant_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any] | None:
        """Get a tenant by ID.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Tenant dict or None if not found
        """
        self._ensure_connected()
        return await self._grpc.get_tenant(
            actor=actor,
            tenant_id=tenant_id,
            timeout=timeout,
        )

    async def archive_tenant(
        self,
        tenant_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Archive a tenant.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        self._ensure_connected()
        return await self._grpc.archive_tenant(
            actor=actor,
            tenant_id=tenant_id,
            timeout=timeout,
        )

    async def add_tenant_member(
        self,
        tenant_id: str,
        user_id: str,
        role: str = "member",
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Add a member to a tenant.

        Args:
            tenant_id: Tenant identifier
            user_id: User to add
            role: Membership role (default: 'member')
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        self._ensure_connected()
        return await self._grpc.add_tenant_member(
            actor=actor,
            tenant_id=tenant_id,
            user_id=user_id,
            role=role,
            timeout=timeout,
        )

    async def remove_tenant_member(
        self,
        tenant_id: str,
        user_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Remove a member from a tenant.

        Args:
            tenant_id: Tenant identifier
            user_id: User to remove
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        self._ensure_connected()
        return await self._grpc.remove_tenant_member(
            actor=actor,
            tenant_id=tenant_id,
            user_id=user_id,
            timeout=timeout,
        )

    async def get_tenant_members(
        self,
        tenant_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """List all members of a tenant.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            List of member dicts
        """
        self._ensure_connected()
        return await self._grpc.get_tenant_members(
            actor=actor,
            tenant_id=tenant_id,
            timeout=timeout,
        )

    async def get_user_tenants(
        self,
        user_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """List all tenants a user belongs to.

        Args:
            user_id: User identifier
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            List of membership dicts
        """
        self._ensure_connected()
        return await self._grpc.get_user_tenants(
            actor=actor,
            user_id=user_id,
            timeout=timeout,
        )

    async def change_member_role(
        self,
        tenant_id: str,
        user_id: str,
        new_role: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Change a member's role in a tenant.

        Args:
            tenant_id: Tenant identifier
            user_id: User whose role to change
            new_role: New role
            actor: Actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        self._ensure_connected()
        return await self._grpc.change_member_role(
            actor=actor,
            tenant_id=tenant_id,
            user_id=user_id,
            new_role=new_role,
            timeout=timeout,
        )

    # --- Admin operations (Issue #90, ADR-003) -----------------------

    async def transfer_user_content(
        self,
        tenant_id: str,
        from_user: str,
        to_user: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Reassign ownership of all nodes from from_user to to_user.

        Use case: employee offboarding. Requires admin or owner role.

        Args:
            tenant_id: Tenant identifier
            from_user: Current owner actor
            to_user: New owner actor
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, transferred count, and error fields.
        """
        self._ensure_connected()
        return await self._grpc.transfer_user_content(
            tenant_id=tenant_id,
            from_user=from_user,
            to_user=to_user,
            actor=actor,
            timeout=timeout,
        )

    async def delegate_access(
        self,
        tenant_id: str,
        from_user: str,
        to_user: str,
        permission: str = "read",
        expires_at: int | None = None,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Grant to_user temporary access to all of from_user's nodes.

        Args:
            tenant_id: Tenant identifier
            from_user: Content owner
            to_user: Delegation recipient
            permission: Permission level
            expires_at: Expiry timestamp in Unix ms (None = permanent)
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, delegated count, expires_at, and error fields.
        """
        self._ensure_connected()
        return await self._grpc.delegate_access(
            tenant_id=tenant_id,
            from_user=from_user,
            to_user=to_user,
            permission=permission,
            expires_at=expires_at,
            actor=actor,
            timeout=timeout,
        )

    async def set_legal_hold(
        self,
        tenant_id: str,
        enabled: bool = True,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Enable or disable legal hold on a tenant.

        Args:
            tenant_id: Tenant identifier
            enabled: True to enable, False to release
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, status, and error fields.
        """
        self._ensure_connected()
        return await self._grpc.set_legal_hold(
            tenant_id=tenant_id,
            enabled=enabled,
            actor=actor,
            timeout=timeout,
        )

    async def revoke_all_user_access(
        self,
        tenant_id: str,
        user_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Remove all access for a user in a tenant (instant termination).

        Args:
            tenant_id: Tenant identifier
            user_id: User to revoke
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, revoked_grants, revoked_groups,
            revoked_shared, and error fields.
        """
        self._ensure_connected()
        return await self._grpc.revoke_all_user_access(
            tenant_id=tenant_id,
            user_id=user_id,
            actor=actor,
            timeout=timeout,
        )
