"""Hierarchical scope API for EntDB SDK.

Provides a builder pattern that captures tenant and actor context:

    >>> async with DbClient("localhost:50051") as db:
    ...     alice = db.tenant("alice").actor("user:bob")
    ...     task = await alice.get(Task, "t1")
    ...     tasks = await alice.query(Task, status="todo")
    ...     await alice.share("t1", "user:charlie", perm=Permission.WRITE)
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .typed import ACLEntry, Actor, Permission, TypedEdge, TypedNode

if TYPE_CHECKING:
    from ._grpc_client import Edge, Node
    from .schema import EdgeTypeDef, NodeTypeDef


class ScopedPlan:
    """A ``Plan`` pre-bound to a tenant and actor.

    Supports chaining and accepts ``TypedNode`` instances directly.
    """

    def __init__(self, plan: Any) -> None:
        self._plan = plan

    def create(
        self,
        node: Any,
        data: dict[str, Any] | None = None,
        *,
        acl: list[ACLEntry] | None = None,
        as_: str | None = None,
        fanout_to: list[Actor | str] | None = None,
    ) -> ScopedPlan:
        """Add a create_node operation.

        Accepts a proto message instance, a ``TypedNode`` instance, or
        a ``NodeTypeDef`` + data dict.
        """
        acl_dicts = _acl_to_dicts(acl) if acl else None
        fanout_strs = [str(a) for a in fanout_to] if fanout_to else None
        self._plan.create(node, data, acl=acl_dicts, as_=as_, fanout_to=fanout_strs)
        return self

    def update(
        self,
        node_type: type[TypedNode] | NodeTypeDef,
        node_id: str,
        patch: dict[str, Any] | TypedNode,
        *,
        field_mask: list[str] | None = None,
    ) -> ScopedPlan:
        """Add an update_node operation.

        ``patch`` can be a ``TypedNode`` instance (uses ``to_payload``)
        or a plain dict.
        """
        resolved_type = _resolve_node_type(node_type)
        resolved_patch = patch.to_payload() if isinstance(patch, TypedNode) else patch
        self._plan.update(resolved_type, node_id, resolved_patch, field_mask=field_mask)
        return self

    def delete(
        self,
        node_type: type[TypedNode] | NodeTypeDef,
        node_id: str,
    ) -> ScopedPlan:
        self._plan.delete(_resolve_node_type(node_type), node_id)
        return self

    def edge_create(
        self,
        edge_type: type[TypedEdge] | EdgeTypeDef,
        from_: str | dict[str, Any],
        to: str | dict[str, Any],
        props: dict[str, Any] | TypedEdge | None = None,
    ) -> ScopedPlan:
        resolved_props = props.to_props() if isinstance(props, TypedEdge) else props
        self._plan.edge_create(_resolve_edge_type(edge_type), from_, to, resolved_props)
        return self

    def edge_delete(
        self,
        edge_type: type[TypedEdge] | EdgeTypeDef,
        from_: str | dict[str, Any],
        to: str | dict[str, Any],
    ) -> ScopedPlan:
        self._plan.edge_delete(_resolve_edge_type(edge_type), from_, to)
        return self

    async def commit(self, wait_applied: bool = False, *, timeout: float | None = None) -> Any:
        return await self._plan.commit(wait_applied=wait_applied, timeout=timeout)


class TenantScope:
    """Captures ``(client, tenant_id)``.  Call ``.actor()`` to bind an actor."""

    def __init__(self, client: Any, tenant_id: str) -> None:
        self._client = client
        self._tenant_id = tenant_id

    def actor(self, actor: Actor | str) -> ActorScope:
        return ActorScope(self._client, self._tenant_id, str(actor))


class ActorScope:
    """Fully-scoped handle with ``(client, tenant_id, actor)`` bound."""

    def __init__(self, client: Any, tenant_id: str, actor: str) -> None:
        self._client = client
        self._tenant_id = tenant_id
        self._actor = actor

    @property
    def tenant_id(self) -> str:
        return self._tenant_id

    @property
    def actor(self) -> str:
        return self._actor

    # ── Reads ────────────────────────────────────────────────────────

    async def get(
        self,
        node_type: type[TypedNode] | NodeTypeDef,
        node_id: str,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        kwargs = _optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout)
        return await self._client.get(
            _resolve_node_type(node_type), node_id, self._tenant_id, self._actor, **kwargs
        )

    async def get_by_key(
        self,
        node_type: type[TypedNode] | NodeTypeDef,
        key_name: str,
        key_value: str,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Resolve a node via a declared unique/secondary key.

        Implements the client-facing half of the 2026-04-13
        unique_keys decision. Returns ``None`` if the key is
        unknown, and raises a typed ``PermissionError`` / gRPC
        ``PERMISSION_DENIED`` when the actor lacks
        ``CORE_CAP_READ`` on the resolved node.
        """
        resolved_type = _resolve_node_type(node_type)
        return await self._client.get_by_key(
            resolved_type,
            key_name,
            key_value,
            self._tenant_id,
            self._actor,
            after_offset=after_offset,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def get_many(
        self,
        node_type: type[TypedNode] | NodeTypeDef,
        node_ids: list[str],
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> tuple[list[Node], list[str]]:
        kwargs = _optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout)
        return await self._client.get_many(
            _resolve_node_type(node_type), node_ids, self._tenant_id, self._actor, **kwargs
        )

    async def query(
        self,
        node_type: type[TypedNode] | NodeTypeDef,
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
        kwargs: dict[str, Any] = {
            "limit": limit,
            "offset": offset,
            "order_by": order_by,
            "descending": descending,
        }
        kwargs.update(
            _optional(filter=filter, after_offset=after_offset, trace_id=trace_id, timeout=timeout)
        )
        return await self._client.query(
            _resolve_node_type(node_type), self._tenant_id, self._actor, **kwargs
        )

    async def edges_out(
        self,
        node_id: str,
        edge_type: type[TypedEdge] | EdgeTypeDef | None = None,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        kwargs: dict[str, Any] = {}
        if edge_type is not None:
            kwargs["edge_type"] = _resolve_edge_type(edge_type)
        kwargs.update(_optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout))
        return await self._client.edges_out(node_id, self._tenant_id, self._actor, **kwargs)

    async def edges_in(
        self,
        node_id: str,
        edge_type: type[TypedEdge] | EdgeTypeDef | None = None,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Edge]:
        kwargs: dict[str, Any] = {}
        if edge_type is not None:
            kwargs["edge_type"] = _resolve_edge_type(edge_type)
        kwargs.update(_optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout))
        return await self._client.edges_in(node_id, self._tenant_id, self._actor, **kwargs)

    async def connected(
        self,
        node_id: str,
        edge_type: type[TypedEdge] | EdgeTypeDef,
        *,
        limit: int = 100,
        offset: int = 0,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        kwargs: dict[str, Any] = {"limit": limit, "offset": offset}
        kwargs.update(_optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout))
        return await self._client.connected(
            node_id, _resolve_edge_type(edge_type), self._tenant_id, self._actor, **kwargs
        )

    # ── ACL ──────────────────────────────────────────────────────────

    async def share(
        self,
        node_id: str,
        grantee: Actor | str,
        perm: Permission = Permission.READ,
        *,
        actor_type: str = "user",
        expires_at: int | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        kwargs: dict[str, Any] = {"permission": perm.value, "actor_type": actor_type}
        kwargs.update(_optional(expires_at=expires_at, trace_id=trace_id, timeout=timeout))
        return await self._client.share(
            node_id, str(grantee), self._tenant_id, self._actor, **kwargs
        )

    async def revoke(
        self,
        node_id: str,
        actor_id: str,
        *,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> bool:
        kwargs = _optional(trace_id=trace_id, timeout=timeout)
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
        kwargs: dict[str, Any] = {"limit": limit, "offset": offset}
        kwargs.update(_optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout))
        return await self._client.shared_with_me(self._tenant_id, self._actor, **kwargs)

    # ── Writes ───────────────────────────────────────────────────────

    def plan(
        self,
        *,
        idempotency_key: str | None = None,
        trace_id: str | None = None,
    ) -> ScopedPlan:
        raw_plan = self._client.atomic(
            self._tenant_id,
            self._actor,
            idempotency_key,
            trace_id=trace_id,
        )
        return ScopedPlan(raw_plan)


# ── Helpers ──────────────────────────────────────────────────────────


def _resolve_node_type(t: type[TypedNode] | Any) -> Any:
    """Resolve a TypedNode class to its NodeTypeDef from the registry."""
    if isinstance(t, type) and issubclass(t, TypedNode) and t is not TypedNode:
        from .registry import get_registry

        registry = get_registry()
        node_type = registry.get_node_type(t._type_id)
        if node_type is None:
            raise ValueError(f"Type {t._type_name} (id={t._type_id}) not registered")
        return node_type
    return t


def _resolve_edge_type(t: type[TypedEdge] | Any) -> Any:
    """Resolve a TypedEdge class to its EdgeTypeDef from the registry."""
    if isinstance(t, type) and issubclass(t, TypedEdge) and t is not TypedEdge:
        from .registry import get_registry

        registry = get_registry()
        edge_type = registry.get_edge_type(t._edge_type_id)
        if edge_type is None:
            raise ValueError(f"Edge {t._edge_type_name} (id={t._edge_type_id}) not registered")
        return edge_type
    return t


def _acl_to_dicts(entries: list[ACLEntry]) -> list[dict[str, str]]:
    """Convert typed ACL entries to the dict format the Plan expects."""
    return [
        {
            "grantee": e.grantee,
            "permission": e.permission.value,
            "actor_type": e.actor_type,
            **({"expires_at": str(e.expires_at)} if e.expires_at else {}),
        }
        for e in entries
    ]


def _optional(**kwargs: Any) -> dict[str, Any]:
    """Filter out None-valued kwargs."""
    return {k: v for k, v in kwargs.items() if v is not None}
