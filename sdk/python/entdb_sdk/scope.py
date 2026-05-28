# SPDX-License-Identifier: MIT
"""Hierarchical scope API for EntDB SDK.

Provides a builder pattern that captures tenant and actor context:

    >>> async with DbClient("localhost:50051") as db:
    ...     alice = db.tenant("alice").actor("user:bob")
    ...     product = await alice.get(schema_pb2.Product, "p1")
    ...     hits = await alice.query(schema_pb2.Product, filter={"sku": {"$like": "WIDGET-%"}})
    ...     await alice.share("p1", "user:charlie", perm=Permission.WRITE)

Per ADR-025 the scope API is single-shape: proto messages
everywhere, ``UniqueKey`` tokens for unique-field lookup, and one
method per operation. See ``docs/adr/025-single-shape-sdk-api.md``.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .keys import Storage, UniqueKey
from .typed import ACLEntry, Actor, Permission, TypedEdge, TypedNode

if TYPE_CHECKING:
    from ._grpc_client import Edge, Node
    from .filter import Filter


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
        on_conflict: str | None = None,
    ) -> ScopedPlan:
        """Create a node from a proto message.

        ``msg`` is a generated proto message instance. ``storage``
        defaults to :class:`Tenant`; pass :class:`Mailbox` or
        :class:`Public` for non-default routing. ``acl`` overrides the
        per-type default ACL. ``on_conflict`` selects the unique-
        violation policy (v2.2 / issue #599); pass ``"skip"`` to opt
        into the single-RTT InsertIfNotExists path.
        """
        fanout_strs = [str(a) for a in fanout_to] if fanout_to else None
        self._plan.create(
            msg,
            acl=acl,
            storage=storage,
            as_=as_,
            fanout_to=fanout_strs,
            on_conflict=on_conflict,
        )
        return self

    def update(
        self,
        node_id: str,
        msg: Any,
        *,
        precondition: tuple[str, Any] | None = None,
    ) -> ScopedPlan:
        """Update a node from a proto message.

        Per the 2026-04-14 SDK v0.3 decision the patch is exactly the
        set of fields explicitly set on ``msg`` (``ListFields()``
        semantics). ``msg`` carries its own type via
        ``(entdb.node)`` — there is no separate ``type_id`` argument.
        Pass ``precondition=(field_name, expected_value)`` for CAS.
        """
        self._plan.update(node_id, msg, precondition=precondition)
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
        where: list[Filter] | None = None,
        limit: int = 0,
        offset: int = 0,
        order_by: str = "created_at",
        descending: bool = True,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Query nodes by type.

        ``node_type`` is the proto message class.

        ``where`` is the typed comparison-filter shape added in issue
        #501 — a list of :class:`Filter` values AND-ed together, with
        operator support for Eq/Ne/Lt/Le/Gt/Ge. Prefer ``where`` for
        new code; the legacy ``filter`` map shape is kept for
        backwards compatibility.

        ``FilterOp.NE`` cannot use a B-tree index — it forces a full
        type scan. Use sparingly on large tables.

        ``limit`` defaults to ``0`` = the COMPLETE result set: the SDK
        follows the ADR-029 keyset cursor across pages so a query never
        silently truncates at the 100-row page default. Set a positive
        value to cap. ``offset`` is deprecated (legacy single-shot path).
        """
        kwargs: dict[str, Any] = {
            "limit": limit,
            "offset": offset,
            "order_by": order_by,
            "descending": descending,
        }
        kwargs.update(
            _optional(
                filter=filter,
                where=where,
                after_offset=after_offset,
                trace_id=trace_id,
                timeout=timeout,
            )
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
        page_size: int = 0,
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

        ADR-029 FTS carve-out: search is relevance-ranked top-N and is
        OFFSET-paged, NOT cursor-paged — it does NOT auto-follow to
        completion (unlike :meth:`query` / :meth:`edges_out` /
        :meth:`shared_with_me`). Page deliberately with ``page_size`` +
        ``offset``.
        """
        kwargs = _optional(trace_id=trace_id, timeout=timeout)
        return await self._client.search_nodes(
            _resolve_node_type(node_type),
            self._tenant_id,
            self._actor,
            query,
            limit=limit,
            offset=offset,
            page_size=page_size,
            **kwargs,
        )

    # ── Mailbox-scoped reads (#568) ──────────────────────────────────
    #
    # USER_MAILBOX nodes are written with ``InMailbox(...)``; these are
    # the matching read surface. ``target_user`` is a bare user id (e.g.
    # ``"alice"``, not ``"user:alice"``). A node that is not a mailbox
    # node owned by ``target_user`` is invisible — never leaked as found.
    # storage_mode is immutable (ADR-020): a node created InMailbox is
    # only ever reachable through these mailbox reads.

    async def get_in_mailbox(
        self,
        node_type: Any,
        target_user: str,
        node_id: str,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Get a node from ``target_user``'s mailbox by id."""
        kwargs = _optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout)
        return await self._client.get(
            _resolve_node_type(node_type),
            node_id,
            self._tenant_id,
            self._actor,
            target_user=target_user,
            **kwargs,
        )

    async def get_many_in_mailbox(
        self,
        node_type: Any,
        target_user: str,
        node_ids: list[str],
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> tuple[list[Node], list[str]]:
        """Batch-get nodes from ``target_user``'s mailbox by id."""
        kwargs = _optional(after_offset=after_offset, trace_id=trace_id, timeout=timeout)
        return await self._client.get_many(
            _resolve_node_type(node_type),
            node_ids,
            self._tenant_id,
            self._actor,
            target_user=target_user,
            **kwargs,
        )

    async def query_in_mailbox(
        self,
        node_type: Any,
        target_user: str,
        *,
        filter: dict[str, Any] | None = None,
        where: list[Filter] | None = None,
        limit: int = 0,
        offset: int = 0,
        order_by: str = "created_at",
        descending: bool = True,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Query ``target_user``'s mailbox nodes (mailbox-scoped ``query``)."""
        kwargs: dict[str, Any] = {
            "limit": limit,
            "offset": offset,
            "order_by": order_by,
            "descending": descending,
            "target_user": target_user,
        }
        kwargs.update(
            _optional(
                filter=filter,
                where=where,
                after_offset=after_offset,
                trace_id=trace_id,
                timeout=timeout,
            )
        )
        return await self._client.query(
            _resolve_node_type(node_type), self._tenant_id, self._actor, **kwargs
        )

    async def search_in_mailbox(
        self,
        node_type: Any,
        target_user: str,
        query: str,
        *,
        limit: int = 50,
        offset: int = 0,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Full-text search over ``target_user``'s mailbox nodes."""
        kwargs = _optional(trace_id=trace_id, timeout=timeout)
        return await self._client.search_nodes(
            _resolve_node_type(node_type),
            self._tenant_id,
            self._actor,
            query,
            limit=limit,
            offset=offset,
            target_user=target_user,
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
        limit: int = 0,
        offset: int = 0,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """List nodes shared with this scope's actor (cross-tenant included).

        Auto-follows the ADR-029 unified keyset cursor across both merged
        sources, returning the COMPLETE set by default (``limit <= 0``). A
        positive ``limit`` caps the total; the deprecated ``offset`` falls
        back to a single non-cursor request.
        """
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

    async def insert_if_not_exists(
        self,
        msg: Any,
        *,
        idempotency_key: str,
        timeout: float | None = None,
    ) -> tuple[str | None, str | None]:
        """Insert ``msg`` if no node with its unique keys exists.

        Closes the racy ``get_by_key → create`` idiom: under N
        concurrent writers exactly one wins and the rest learn the
        existing canonical node id without a second user-visible round
        trip into client code.

        Returns ``(created_id, existing_id)``; exactly one is non-None
        on success. ``created_id`` is the freshly-minted id when this
        call won the race; ``existing_id`` is the canonical id of the
        prior winner when a single-field unique constraint tripped.

        Wire mechanics by server version:

        - **v2.2+ server (single round trip)**: the SDK sends
          ``on_conflict="skip"`` on the create op. The applier
          swallows the violation, looks the colliding row up in the
          same txn, and returns its id in
          :attr:`CommitResult.existing_node_ids`. Works for BOTH
          single-field and composite unique constraints because the
          lookup is driven by the violated index (not by an
          SDK-visible key token).

        - **v2.1.x server (two round trips, fallback)**: SKIP is
          unknown to the older server, which falls back to its
          legacy :class:`UniqueConstraintError`. The SDK catches
          that, follows up with :py:meth:`get_node_by_key` using the
          typed ``(type_id, field_id, value)`` on the UCE, and
          returns the existing id. Composite violations on a v2.1.x
          server re-raise the typed error (no ``GetByCompositeKey``
          RPC was ever shipped); v2.2's server-side SKIP closes that
          gap.

        ``wait_applied=True`` is forced so the synchronous outcome
        is observed regardless of server version (without it, the
        loser of a v2.1.x race sees a phantom success — issue #606).

        Issue #599.
        """
        from .errors import UniqueConstraintError

        if msg is None:
            raise TypeError("insert_if_not_exists: msg is required (got None)")

        scoped_plan = self.plan(idempotency_key=idempotency_key)
        scoped_plan.create(msg, on_conflict="skip")
        try:
            result = await scoped_plan.commit(wait_applied=True, timeout=timeout)
        except UniqueConstraintError as uce:
            # v2.1.x fallback path: SKIP was not honoured server-side;
            # the legacy UniqueConstraintError surfaced. Do the
            # two-RTT GetNodeByKey lookup. Composite UCE re-raises —
            # no GetByCompositeKey RPC in v2.1.x.
            if uce.is_composite:
                raise
            node = await self._client._grpc.get_node_by_key(
                tenant_id=self._tenant_id,
                actor=self._actor,
                type_id=int(uce.type_id),
                field_id=int(uce.field_id),
                value=uce.value,
                timeout=timeout,
            )
            if node is None:
                raise
            return (None, node.node_id)

        # v2.2+ single-RTT path. The applier filled exactly one of
        # the two parallel slots at index 0. ``existing_node_ids`` is
        # the authoritative signal — pre-v2.2 servers leave it empty,
        # so a non-empty entry there proves the server honoured
        # ``on_conflict="skip"``.
        if result.existing_node_ids and result.existing_node_ids[0]:
            return (None, result.existing_node_ids[0])
        if not result.created_node_ids:
            raise RuntimeError("insert_if_not_exists: commit returned no created_node_ids")
        return (result.created_node_ids[0], None)


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
