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

import asyncio
import json
import uuid
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Union
import logging

from .schema import NodeTypeDef, EdgeTypeDef
from .registry import SchemaRegistry, get_registry
from .errors import (
    EntDbError,
    ConnectionError,
    ValidationError,
    UnknownFieldError,
    NotFoundError,
)

logger = logging.getLogger(__name__)

# Try to import HTTP client
try:
    import aiohttp
    AIOHTTP_AVAILABLE = True
except ImportError:
    AIOHTTP_AVAILABLE = False
    aiohttp = None


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
    stream_position: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> Receipt:
        """Create from dictionary."""
        return cls(
            tenant_id=data["tenant_id"],
            idempotency_key=data["idempotency_key"],
            stream_position=data.get("stream_position"),
        )


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
    payload: Dict[str, Any]
    created_at: int
    updated_at: int
    owner_actor: str
    acl: List[Dict[str, str]] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> Node:
        """Create from dictionary."""
        payload = data.get("payload_json")
        if isinstance(payload, str):
            payload = json.loads(payload)

        acl = data.get("acl_json")
        if isinstance(acl, str):
            acl = json.loads(acl)

        return cls(
            tenant_id=data["tenant_id"],
            node_id=data["node_id"],
            type_id=data["type_id"],
            payload=payload or {},
            created_at=data.get("created_at", 0),
            updated_at=data.get("updated_at", 0),
            owner_actor=data.get("owner_actor", ""),
            acl=acl or [],
        )


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
    props: Dict[str, Any]
    created_at: int

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> Edge:
        """Create from dictionary."""
        props = data.get("props_json")
        if isinstance(props, str):
            props = json.loads(props)

        return cls(
            tenant_id=data["tenant_id"],
            edge_type_id=data["edge_type_id"],
            from_node_id=data["from_node_id"],
            to_node_id=data["to_node_id"],
            props=props or {},
            created_at=data.get("created_at", 0),
        )


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
    receipt: Optional[Receipt] = None
    created_node_ids: List[str] = field(default_factory=list)
    applied: bool = False
    error: Optional[str] = None


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
        idempotency_key: Optional[str] = None,
    ) -> None:
        """Initialize a plan.

        Args:
            client: DbClient instance
            tenant_id: Tenant identifier
            actor: Actor performing operations
            idempotency_key: Optional unique key for deduplication
        """
        self._client = client
        self._tenant_id = tenant_id
        self._actor = actor
        self._idempotency_key = idempotency_key or str(uuid.uuid4())
        self._operations: List[Dict[str, Any]] = []

    def create(
        self,
        node_type: NodeTypeDef,
        data: Union[Dict[str, Any], None] = None,
        *,
        acl: Optional[List[Dict[str, str]]] = None,
        as_: Optional[str] = None,
        fanout_to: Optional[List[str]] = None,
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
            ValidationError: If payload validation fails
            UnknownFieldError: If unknown field is provided
        """
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

        op = {
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
        patch: Dict[str, Any],
        *,
        field_mask: Optional[List[str]] = None,
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
        op = {
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
        self._operations.append({
            "delete_node": {
                "type_id": node_type.type_id,
                "id": node_id,
            }
        })
        return self

    def edge_create(
        self,
        edge_type: EdgeTypeDef,
        from_: Union[str, Dict[str, Any]],
        to: Union[str, Dict[str, Any]],
        props: Optional[Dict[str, Any]] = None,
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
        # Validate props
        if props:
            is_valid, errors = edge_type.validate_props(props)
            if not is_valid:
                raise ValidationError("; ".join(errors), errors=errors)

        op: Dict[str, Any] = {
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
        from_: Union[str, Dict[str, Any]],
        to: Union[str, Dict[str, Any]],
    ) -> Plan:
        """Add a delete_edge operation.

        Args:
            edge_type: Type of edge to delete
            from_: Source node
            to: Target node

        Returns:
            Self for chaining
        """
        self._operations.append({
            "delete_edge": {
                "edge_id": edge_type.edge_id,
                "from": self._convert_ref(from_),
                "to": self._convert_ref(to),
            }
        })
        return self

    def _convert_ref(self, ref: Union[str, Dict[str, Any]]) -> Dict[str, Any]:
        """Convert reference to API format."""
        if isinstance(ref, str):
            if ref.startswith("$"):
                return {"alias_ref": ref}
            return {"id": ref}
        if "type_id" in ref and "id" in ref:
            return {"typed": ref}
        return ref

    async def commit(self, wait_applied: bool = False) -> CommitResult:
        """Commit the transaction.

        Args:
            wait_applied: Whether to wait for application

        Returns:
            CommitResult indicating success/failure
        """
        if not self._operations:
            return CommitResult(success=True, created_node_ids=[])

        result = await self._client._execute(
            tenant_id=self._tenant_id,
            actor=self._actor,
            operations=self._operations,
            idempotency_key=self._idempotency_key,
            wait_applied=wait_applied,
        )

        return result


class DbClient:
    """Client for connecting to EntDB server.

    Supports both gRPC and HTTP connections. Falls back to HTTP
    if gRPC is not available.

    Example:
        >>> async with DbClient("localhost:50051") as db:
        ...     node = await db.get(Task, "node_123", "tenant_1", "user:42")
    """

    def __init__(
        self,
        address: str,
        *,
        use_http: bool = False,
        registry: Optional[SchemaRegistry] = None,
    ) -> None:
        """Initialize client.

        Args:
            address: Server address (host:port)
            use_http: Force HTTP instead of gRPC
            registry: Optional schema registry
        """
        self.address = address
        self.use_http = use_http
        self.registry = registry or get_registry()
        self._session: Optional[aiohttp.ClientSession] = None
        self._connected = False

    async def connect(self) -> None:
        """Connect to the server."""
        if self._connected:
            return

        if self.use_http or not AIOHTTP_AVAILABLE:
            if not AIOHTTP_AVAILABLE:
                raise ConnectionError(
                    "aiohttp required for HTTP client",
                    address=self.address,
                )

            # Parse address
            if not self.address.startswith("http"):
                self.address = f"http://{self.address}"

            self._session = aiohttp.ClientSession()
            self._connected = True
        else:
            # gRPC connection would go here
            # For now, fall back to HTTP
            if not self.address.startswith("http"):
                self.address = f"http://{self.address}"
            self._session = aiohttp.ClientSession()
            self._connected = True

    async def close(self) -> None:
        """Close the connection."""
        if self._session:
            await self._session.close()
            self._session = None
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
        idempotency_key: Optional[str] = None,
    ) -> Plan:
        """Create an atomic transaction plan.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing operations
            idempotency_key: Optional deduplication key

        Returns:
            Plan builder

        Example:
            >>> plan = db.atomic("tenant_1", "user:42")
            >>> plan.create(Task, {"title": "New Task"})
            >>> await plan.commit()
        """
        return Plan(self, tenant_id, actor, idempotency_key)

    async def get(
        self,
        node_type: NodeTypeDef,
        node_id: str,
        tenant_id: str,
        actor: str,
    ) -> Optional[Node]:
        """Get a node by ID.

        Args:
            node_type: Expected node type
            node_id: Node identifier
            tenant_id: Tenant identifier
            actor: Actor making request

        Returns:
            Node if found, None otherwise
        """
        url = f"{self.address}/v1/nodes/{node_id}"
        params = {"type_id": node_type.type_id}
        headers = {"X-Tenant-ID": tenant_id, "X-Actor": actor}

        async with self._session.get(url, params=params, headers=headers) as resp:
            data = await resp.json()
            if not data.get("found"):
                return None
            return Node.from_dict(data["node"])

    async def query(
        self,
        node_type: NodeTypeDef,
        tenant_id: str,
        actor: str,
        limit: int = 100,
        offset: int = 0,
    ) -> List[Node]:
        """Query nodes by type.

        Args:
            node_type: Node type to query
            tenant_id: Tenant identifier
            actor: Actor making request
            limit: Maximum nodes to return
            offset: Pagination offset

        Returns:
            List of nodes
        """
        url = f"{self.address}/v1/nodes"
        params = {
            "type_id": node_type.type_id,
            "limit": limit,
            "offset": offset,
        }
        headers = {"X-Tenant-ID": tenant_id, "X-Actor": actor}

        async with self._session.get(url, params=params, headers=headers) as resp:
            data = await resp.json()
            return [Node.from_dict(n) for n in data.get("nodes", [])]

    async def edge_out(
        self,
        node_id: str,
        tenant_id: str,
        actor: str,
        edge_type: Optional[EdgeTypeDef] = None,
    ) -> List[Edge]:
        """Get outgoing edges from a node.

        Args:
            node_id: Source node ID
            tenant_id: Tenant identifier
            actor: Actor making request
            edge_type: Optional edge type filter

        Returns:
            List of edges
        """
        url = f"{self.address}/v1/nodes/{node_id}/edges/from"
        params = {}
        if edge_type:
            params["edge_type_id"] = edge_type.edge_id
        headers = {"X-Tenant-ID": tenant_id, "X-Actor": actor}

        async with self._session.get(url, params=params, headers=headers) as resp:
            data = await resp.json()
            return [Edge.from_dict(e) for e in data.get("edges", [])]

    async def edge_in(
        self,
        node_id: str,
        tenant_id: str,
        actor: str,
        edge_type: Optional[EdgeTypeDef] = None,
    ) -> List[Edge]:
        """Get incoming edges to a node.

        Args:
            node_id: Target node ID
            tenant_id: Tenant identifier
            actor: Actor making request
            edge_type: Optional edge type filter

        Returns:
            List of edges
        """
        url = f"{self.address}/v1/nodes/{node_id}/edges/to"
        params = {}
        if edge_type:
            params["edge_type_id"] = edge_type.edge_id
        headers = {"X-Tenant-ID": tenant_id, "X-Actor": actor}

        async with self._session.get(url, params=params, headers=headers) as resp:
            data = await resp.json()
            return [Edge.from_dict(e) for e in data.get("edges", [])]

    async def search(
        self,
        query: str,
        user_id: str,
        tenant_id: str,
        actor: str,
        types: Optional[List[NodeTypeDef]] = None,
        limit: int = 20,
    ) -> List[Dict[str, Any]]:
        """Search user's mailbox.

        Args:
            query: Search query
            user_id: User whose mailbox to search
            tenant_id: Tenant identifier
            actor: Actor making request
            types: Optional type filter
            limit: Maximum results

        Returns:
            List of search results
        """
        url = f"{self.address}/v1/mailbox/{user_id}/search"
        params: Dict[str, Any] = {"q": query, "limit": limit}
        if types:
            params["source_type_ids"] = ",".join(str(t.type_id) for t in types)
        headers = {"X-Tenant-ID": tenant_id, "X-Actor": actor}

        async with self._session.get(url, params=params, headers=headers) as resp:
            data = await resp.json()
            return data.get("results", [])

    async def _execute(
        self,
        tenant_id: str,
        actor: str,
        operations: List[Dict[str, Any]],
        idempotency_key: str,
        wait_applied: bool = False,
    ) -> CommitResult:
        """Execute atomic transaction.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing operations
            operations: List of operations
            idempotency_key: Deduplication key
            wait_applied: Wait for application

        Returns:
            CommitResult
        """
        url = f"{self.address}/v1/execute"
        headers = {"X-Tenant-ID": tenant_id, "X-Actor": actor}
        body = {
            "operations": operations,
            "idempotency_key": idempotency_key,
            "wait_applied": wait_applied,
        }

        if self.registry.fingerprint:
            body["schema_fingerprint"] = self.registry.fingerprint

        async with self._session.post(url, json=body, headers=headers) as resp:
            data = await resp.json()

            if not data.get("success"):
                return CommitResult(
                    success=False,
                    error=data.get("error"),
                )

            receipt = None
            if "receipt" in data:
                receipt = Receipt.from_dict(data["receipt"])

            return CommitResult(
                success=True,
                receipt=receipt,
                created_node_ids=data.get("created_node_ids", []),
                applied=data.get("applied_status") == "RECEIPT_STATUS_APPLIED",
            )

    async def get_receipt_status(
        self,
        tenant_id: str,
        idempotency_key: str,
    ) -> str:
        """Get receipt status.

        Args:
            tenant_id: Tenant identifier
            idempotency_key: Transaction key

        Returns:
            Status string (PENDING, APPLIED, FAILED)
        """
        url = f"{self.address}/v1/receipt/{idempotency_key}"
        headers = {"X-Tenant-ID": tenant_id, "X-Actor": "system"}

        async with self._session.get(url, headers=headers) as resp:
            data = await resp.json()
            return data.get("status", "UNKNOWN")
