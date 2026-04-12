"""Hierarchical scope API for EntDB SDK.

Provides a builder pattern that captures tenant and actor context,
so callers don't need to pass ``tenant_id`` and ``actor`` on every call.

Example:
    >>> async with DbClient("localhost:50051") as db:
    ...     alice = db.tenant("alice").actor("user:bob")
    ...     task = await alice.get(Task, "t1")
    ...     tasks = await alice.query(Task, filter={"status": "todo"})
    ...     await alice.share("t1", "user:charlie", perm="write")

The flat API on ``DbClient`` remains fully functional and unchanged.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from ._grpc_client import Edge, Node
    from .schema import EdgeTypeDef, NodeTypeDef


class ScopedPlan:
    """A ``Plan`` pre-bound to a tenant and actor.

    All mutating helpers (``create``, ``update``, ``delete``, ``edge_create``,
    ``edge_delete``) delegate to the underlying ``Plan`` but never require
    the caller to supply ``tenant_id`` or ``actor``.

    Commit still returns a ``CommitResult`` via ``await plan.commit()``.
    """

    def __init__(self, plan: Any) -> None:
        self._plan = plan

    # --- Mutations --------------------------------------------------------

    def create(
        self,
        node_type: NodeTypeDef,
        data: dict[str, Any] | None = None,
        *,
        acl: list[dict[str, str]] | None = None,
        as_: str | None = None,
        fanout_to: list[str] | None = None,
        **kwargs: Any,
    ) -> ScopedPlan:
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
        """
        self._plan.create(node_type, data, acl=acl, as_=as_, fanout_to=fanout_to, **kwargs)
        return self

    def update(
        self,
        node_type: NodeTypeDef,
        node_id: str,
        patch: dict[str, Any],
        *,
        field_mask: list[str] | None = None,
    ) -> ScopedPlan:
        """Add an update_node operation.

        Args:
            node_type: Type of node to update
            node_id: ID of node to update
            patch: Fields to update
            field_mask: Optional explicit field mask

        Returns:
            Self for chaining
        """
        self._plan.update(node_type, node_id, patch, field_mask=field_mask)
        return self

    def delete(
        self,
        node_type: NodeTypeDef,
        node_id: str,
    ) -> ScopedPlan:
        """Add a delete_node operation.

        Args:
            node_type: Type of node to delete
            node_id: ID of node to delete

        Returns:
            Self for chaining
        """
        self._plan.delete(node_type, node_id)
        return self

    def edge_create(
        self,
        edge_type: EdgeTypeDef,
        from_: str | dict[str, Any],
        to: str | dict[str, Any],
        props: dict[str, Any] | None = None,
    ) -> ScopedPlan:
        """Add a create_edge operation.

        Args:
            edge_type: Type of edge to create
            from_: Source node (ID, alias ref, or typed ref)
            to: Target node
            props: Edge properties

        Returns:
            Self for chaining
        """
        self._plan.edge_create(edge_type, from_, to, props)
        return self

    def edge_delete(
        self,
        edge_type: EdgeTypeDef,
        from_: str | dict[str, Any],
        to: str | dict[str, Any],
    ) -> ScopedPlan:
        """Add a delete_edge operation.

        Args:
            edge_type: Type of edge to delete
            from_: Source node
            to: Target node

        Returns:
            Self for chaining
        """
        self._plan.edge_delete(edge_type, from_, to)
        return self

    async def commit(self, wait_applied: bool = False, *, timeout: float | None = None) -> Any:
        """Commit the transaction.

        Args:
            wait_applied: Whether to wait for application
            timeout: Per-call timeout in seconds

        Returns:
            CommitResult indicating success/failure
        """
        return await self._plan.commit(wait_applied=wait_applied, timeout=timeout)


class TenantScope:
    """Captures a ``(db_client, tenant_id)`` pair.

    Call ``.actor(actor)`` to obtain an ``ActorScope`` that is fully bound
    and ready for reads/writes.
    """

    def __init__(self, client: Any, tenant_id: str) -> None:
        self._client = client
        self._tenant_id = tenant_id

    def actor(self, actor: str) -> ActorScope:
        """Bind an actor to this tenant scope.

        Args:
            actor: Actor identifier (e.g. ``"user:bob"``)

        Returns:
            ActorScope with tenant + actor bound
        """
        return ActorScope(self._client, self._tenant_id, actor)


class ActorScope:
    """Fully-scoped handle with ``(db_client, tenant_id, actor)`` pre-bound.

    Every read/write method delegates to the corresponding flat method on
    ``DbClient``, injecting ``tenant_id`` and ``actor`` automatically.
    """

    def __init__(self, client: Any, tenant_id: str, actor: str) -> None:
        self._client = client
        self._tenant_id = tenant_id
        self._actor = actor

    @property
    def tenant_id(self) -> str:
        """The tenant this scope is bound to."""
        return self._tenant_id

    @property
    def actor(self) -> str:
        """The actor this scope is bound to."""
        return self._actor

    # --- Reads ------------------------------------------------------------

    async def get(
        self,
        node_type: NodeTypeDef,
        node_id: str,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Get a node by ID.

        Args:
            node_type: Expected node type
            node_id: Node identifier
            after_offset: Optional stream position to wait for
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            Node if found, None otherwise
        """
        kwargs: dict[str, Any] = {}
        if after_offset is not None:
            kwargs["after_offset"] = after_offset
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.get(node_type, node_id, self._tenant_id, self._actor, **kwargs)

    async def get_many(
        self,
        node_type: NodeTypeDef,
        node_ids: list[str],
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> tuple[list[Node], list[str]]:
        """Get multiple nodes by IDs.

        Args:
            node_type: Expected node type
            node_ids: Node identifiers
            after_offset: Optional stream position to wait for
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (found nodes, missing node IDs)
        """
        kwargs: dict[str, Any] = {}
        if after_offset is not None:
            kwargs["after_offset"] = after_offset
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.get_many(
            node_type, node_ids, self._tenant_id, self._actor, **kwargs
        )

    async def query(
        self,
        node_type: NodeTypeDef,
        *,
        filter: dict[str, Any] | None = None,
        limit: int = 100,
        offset: int = 0,
        order_by: str = "created_at",
        descending: bool = True,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Query nodes by type.

        Args:
            node_type: Node type to query
            filter: Optional payload field filter
            limit: Maximum nodes to return
            offset: Pagination offset
            order_by: Field to order by
            descending: Sort descending
            after_offset: Optional stream position to wait for
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            List of matching nodes
        """
        kwargs: dict[str, Any] = {
            "limit": limit,
            "offset": offset,
            "order_by": order_by,
            "descending": descending,
        }
        if filter is not None:
            kwargs["filter"] = filter
        if after_offset is not None:
            kwargs["after_offset"] = after_offset
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.query(node_type, self._tenant_id, self._actor, **kwargs)

    async def edges_out(
        self,
        node_id: str,
        edge_type: EdgeTypeDef | None = None,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        """Get outgoing edges from a node.

        Args:
            node_id: Source node ID
            edge_type: Optional edge type filter
            after_offset: Optional stream position to wait for
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            List of outgoing edges
        """
        kwargs: dict[str, Any] = {}
        if edge_type is not None:
            kwargs["edge_type"] = edge_type
        if after_offset is not None:
            kwargs["after_offset"] = after_offset
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.edges_out(node_id, self._tenant_id, self._actor, **kwargs)

    async def edges_in(
        self,
        node_id: str,
        edge_type: EdgeTypeDef | None = None,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        """Get incoming edges to a node.

        Args:
            node_id: Target node ID
            edge_type: Optional edge type filter
            after_offset: Optional stream position to wait for
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            List of incoming edges
        """
        kwargs: dict[str, Any] = {}
        if edge_type is not None:
            kwargs["edge_type"] = edge_type
        if after_offset is not None:
            kwargs["after_offset"] = after_offset
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.edges_in(node_id, self._tenant_id, self._actor, **kwargs)

    async def connected(
        self,
        node_id: str,
        edge_type: EdgeTypeDef,
        *,
        limit: int = 100,
        offset: int = 0,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Get connected nodes via edge type with ACL filtering.

        Args:
            node_id: Source node ID
            edge_type: Edge type to traverse
            limit: Maximum nodes to return
            offset: Pagination offset
            after_offset: Optional stream position to wait for
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            List of connected nodes the actor can see
        """
        kwargs: dict[str, Any] = {
            "limit": limit,
            "offset": offset,
        }
        if after_offset is not None:
            kwargs["after_offset"] = after_offset
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.connected(
            node_id, edge_type, self._tenant_id, self._actor, **kwargs
        )

    # --- ACL / sharing ----------------------------------------------------

    async def share(
        self,
        node_id: str,
        grantee: str,
        *,
        perm: str = "read",
        actor_type: str = "user",
        expires_at: int | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        """Share a node with another actor.

        Args:
            node_id: Node to share
            grantee: Actor to share with
            perm: Permission level (read, write, admin)
            actor_type: Type of the target actor
            expires_at: Optional expiry timestamp (Unix ms)
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            True if successful
        """
        kwargs: dict[str, Any] = {
            "permission": perm,
            "actor_type": actor_type,
        }
        if expires_at is not None:
            kwargs["expires_at"] = expires_at
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.share(node_id, grantee, self._tenant_id, self._actor, **kwargs)

    async def revoke(
        self,
        node_id: str,
        actor_id: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        """Revoke access from an actor on a node.

        Args:
            node_id: Node to revoke access from
            actor_id: Actor to revoke
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            True if a grant existed and was removed
        """
        kwargs: dict[str, Any] = {}
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.revoke(node_id, actor_id, self._tenant_id, self._actor, **kwargs)

    async def shared_with_me(
        self,
        *,
        limit: int = 100,
        offset: int = 0,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """List nodes shared with the current actor.

        Args:
            limit: Maximum nodes to return
            offset: Pagination offset
            after_offset: Optional stream position to wait for
            trace_id: Optional trace ID
            timeout: Per-call timeout in seconds

        Returns:
            List of shared nodes
        """
        kwargs: dict[str, Any] = {
            "limit": limit,
            "offset": offset,
        }
        if after_offset is not None:
            kwargs["after_offset"] = after_offset
        if trace_id is not None:
            kwargs["trace_id"] = trace_id
        if timeout is not None:
            kwargs["timeout"] = timeout
        return await self._client.shared_with_me(self._tenant_id, self._actor, **kwargs)

    # --- Atomic writes ----------------------------------------------------

    def plan(
        self,
        *,
        idempotency_key: str | None = None,
        trace_id: str | None = None,
    ) -> ScopedPlan:
        """Create a scoped atomic transaction plan.

        The returned ``ScopedPlan`` has ``tenant_id`` and ``actor`` pre-bound,
        so ``create``/``update``/``delete``/``edge_create`` calls don't need
        those arguments.

        Args:
            idempotency_key: Optional deduplication key
            trace_id: Optional trace ID

        Returns:
            ScopedPlan builder
        """
        raw_plan = self._client.atomic(
            self._tenant_id,
            self._actor,
            idempotency_key,
            trace_id=trace_id,
        )
        return ScopedPlan(raw_plan)
