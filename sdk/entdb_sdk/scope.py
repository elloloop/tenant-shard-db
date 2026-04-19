"""Hierarchical scope API for EntDB SDK.

Provides a builder pattern that captures tenant and actor context:

    >>> async with DbClient("localhost:50051") as db:
    ...     alice = db.tenant("alice").actor("user:bob")
    ...     product = await alice.get(schema_pb2.Product, "p1")
    ...     hits = await alice.query(schema_pb2.Product, filter={"sku": {"$like": "WIDGET-%"}})
    ...     await alice.share("p1", "user:charlie", perm=Permission.WRITE)

Per the 2026-04-14 SDK v0.3 decision the scope API is single-shape:
proto messages everywhere, ``UniqueKey`` tokens for unique-field
lookup, and one method per operation. See
``docs/decisions/sdk_api.md``.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .keys import Storage, UniqueKey
from .typed import ACLEntry, Actor, Permission, TypedEdge, TypedNode

if TYPE_CHECKING:
    from ._grpc_client import Edge, Node


class ScopedPlan:
    """A ``Plan`` pre-bound to a tenant and actor.

    The single-shape SDK v0.3 API: every mutation is expressed via a
    proto message (or a proto class as a type witness for delete /
    edge_create / edge_delete). There are no ``*WithACL`` or
    ``*InMailbox`` parallel methods — pass ``acl=`` or ``storage=``
    keywords to :meth:`create` instead.
    """

    def __init__(self, plan: Any) -> None:
        self._plan = plan

    def create(
        self,
        msg: Any,
        *,
        acl: list[ACLEntry] | None = None,
        storage: Storage | None = None,
        as_: str | None = None,
        fanout_to: list[Actor | str] | None = None,
    ) -> ScopedPlan:
        """Create a node from a proto message.

        ``msg`` is a generated proto message instance. ``storage``
        defaults to :class:`Tenant`; pass :class:`Mailbox` or
        :class:`Public` for non-default routing. ``acl`` overrides the
        per-type default ACL.
        """
        fanout_strs = [str(a) for a in fanout_to] if fanout_to else None
        self._plan.create(
            msg,
            acl=acl,
            storage=storage,
            as_=as_,
            fanout_to=fanout_strs,
        )
        return self

    def update(
        self,
        node_id: str,
        msg: Any,
    ) -> ScopedPlan:
        """Update a node from a proto message.

        Per the 2026-04-14 SDK v0.3 decision the patch is exactly the
        set of fields explicitly set on ``msg`` (``ListFields()``
        semantics). ``msg`` carries its own type via
        ``(entdb.node)`` — there is no separate ``type_id`` argument.
        """
        self._plan.update(node_id, msg)
        return self

    def delete(
        self,
        node_type: Any,
        node_id: str,
    ) -> ScopedPlan:
        """Delete a node. ``node_type`` is the proto message *class*."""
        self._plan.delete(node_type, node_id)
        return self

    def edge_create(
        self,
        edge_type: Any,
        from_id: str | dict[str, Any],
        to_id: str | dict[str, Any],
        *,
        props: dict[str, Any] | None = None,
    ) -> ScopedPlan:
        """Create an edge. ``edge_type`` is the proto message class
        (carrying the ``(entdb.edge)`` annotation)."""
        self._plan.edge_create(edge_type, from_id, to_id, props=props)
        return self

    def edge_delete(
        self,
        edge_type: Any,
        from_id: str | dict[str, Any],
        to_id: str | dict[str, Any],
    ) -> ScopedPlan:
        self._plan.edge_delete(edge_type, from_id, to_id)
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
        node_type: Any,
        node_id: str,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Get a node by id. ``node_type`` is the proto message class
        (or, for back-compat with internal callers, a ``NodeTypeDef`` /
        ``TypedNode`` subclass)."""
        kwargs = _optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout)
        return await self._client.get(
            _resolve_node_type(node_type), node_id, self._tenant_id, self._actor, **kwargs
        )

    async def get_by_key(
        self,
        key: UniqueKey[Any],
        value: Any,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Resolve a node via a typed unique-key token from codegen.

        Per the 2026-04-14 SDK v0.3 decision this is the single
        canonical lookup-by-unique-field path. The ``key`` argument
        is a :class:`UniqueKey` token emitted by the
        ``protoc-gen-entdb-keys`` plugin and exposed in the generated
        ``<schema>_entdb.py`` sidecar — there is no string-name
        fallback. The generic parameter ``T`` on ``UniqueKey[T]``
        statically constrains the value type so passing the wrong
        scalar is a type-checker error, not a runtime one.
        """
        return await self._client.get_by_key(
            key,
            value,
            self._tenant_id,
            self._actor,
            after_offset=after_offset,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def get_many(
        self,
        node_type: Any,
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
        node_type: Any,
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
        """Query nodes by type with MongoDB-style filter operators.

        ``node_type`` is the proto message class. ``filter`` accepts
        the eight operators frozen in the 2026-04-14 SDK v0.3 decision:
        ``$eq`` (default), ``$ne``, ``$gt``, ``$gte``, ``$lt``,
        ``$lte``, ``$in``, ``$nin``, ``$like``, ``$between``, plus
        top-level ``$and`` / ``$or`` composition.
        """
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

    async def search(
        self,
        node_type: Any,
        query: str,
        *,
        limit: int = 50,
        offset: int = 0,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Full-text search across searchable fields.

        ``node_type`` is the proto message class (or ``NodeTypeDef``).
        ``query`` is an FTS5 match expression supporting AND, OR, NOT,
        phrase ("..."), and prefix (word*) syntax.

        Only fields declared with ``(entdb.field).searchable = true``
        are searched. Results are ordered by relevance and ACL-filtered
        server-side.
        """
        kwargs = _optional(trace_id=trace_id, timeout=timeout)
        return await self._client.search_nodes(
            _resolve_node_type(node_type),
            self._tenant_id,
            self._actor,
            query,
            limit=limit,
            offset=offset,
            **kwargs,
        )

    async def edges_out(
        self,
        node_id: str,
        edge_type: Any | None = None,
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
        edge_type: Any | None = None,
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
        edge_type: Any,
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


def _resolve_node_type(t: Any) -> Any:
    """Resolve a node-type witness to a ``NodeTypeDef`` from the registry.

    Accepts:
    - A generated proto message *class* with an ``(entdb.node)``
      option — looked up by ``type_id``.
    - A ``TypedNode`` subclass — looked up by its ``_type_id`` attribute.
    - An existing ``NodeTypeDef`` — returned as-is.
    """
    # Proto message class — read type_id from descriptor option.
    if isinstance(t, type) and hasattr(t, "DESCRIPTOR") and hasattr(t, "ListFields"):
        from ._generated import entdb_options_pb2
        from .registry import get_registry

        opts = t.DESCRIPTOR.GetOptions()
        if opts.HasExtension(entdb_options_pb2.node):
            type_id = int(opts.Extensions[entdb_options_pb2.node].type_id)
            registry = get_registry()
            node_type = registry.get_node_type(type_id)
            if node_type is None:
                raise ValueError(
                    f"Proto message {t.DESCRIPTOR.name} (type_id={type_id}) "
                    "is not registered with the SDK schema registry. Call "
                    "register_proto_schema(<module>) on the generated proto "
                    "module before using it with the SDK."
                )
            return node_type
        raise ValueError(f"Proto message class {t.DESCRIPTOR.name} has no (entdb.node) option")

    if isinstance(t, type) and issubclass(t, TypedNode) and t is not TypedNode:
        from .registry import get_registry

        registry = get_registry()
        node_type = registry.get_node_type(t._type_id)
        if node_type is None:
            raise ValueError(f"Type {t._type_name} (id={t._type_id}) not registered")
        return node_type
    return t


def _resolve_edge_type(t: Any) -> Any:
    """Resolve an edge-type witness to an ``EdgeTypeDef`` from the registry.

    Accepts a generated proto message class with an ``(entdb.edge)``
    option, a ``TypedEdge`` subclass, or an existing ``EdgeTypeDef``.
    """
    if isinstance(t, type) and hasattr(t, "DESCRIPTOR") and hasattr(t, "ListFields"):
        from ._generated import entdb_options_pb2
        from .registry import get_registry

        opts = t.DESCRIPTOR.GetOptions()
        if opts.HasExtension(entdb_options_pb2.edge):
            edge_id = int(opts.Extensions[entdb_options_pb2.edge].edge_id)
            registry = get_registry()
            edge_type = registry.get_edge_type(edge_id)
            if edge_type is None:
                raise ValueError(
                    f"Proto edge {t.DESCRIPTOR.name} (edge_id={edge_id}) "
                    "is not registered with the SDK schema registry."
                )
            return edge_type
        raise ValueError(f"Proto message class {t.DESCRIPTOR.name} has no (entdb.edge) option")

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
