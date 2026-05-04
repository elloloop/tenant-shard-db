"""
Internal gRPC client for EntDB SDK.

This module provides the low-level gRPC communication layer.
It is internal to the SDK and should not be used directly by users.

Users should use DbClient instead, which provides a clean Python API.
"""

from __future__ import annotations

import asyncio
import json
import logging
import re
from dataclasses import dataclass, field
from typing import Any

import grpc
from google.protobuf import json_format
from google.protobuf.struct_pb2 import Struct
from grpc import aio as grpc_aio

from ._generated import (
    AclEntry,
    ArchiveTenantRequest,
    CancelUserDeletionRequest,
    ChangeMemberRoleRequest,
    CreateEdgeOp,
    CreateNodeOp,
    # Tenant registry
    CreateTenantRequest,
    # User registry
    CreateUserRequest,
    DelegateAccessRequest,
    DeleteEdgeOp,
    DeleteNodeOp,
    # GDPR operations (Issue #103)
    DeleteUserRequest,
    EntDBServiceStub,
    ExecuteAtomicRequest,
    ExportUserDataRequest,
    FieldFilter,
    FreezeUserRequest,
    # ACL v2
    GetConnectedNodesRequest,
    GetEdgesRequest,
    GetMailboxRequest,
    GetNodeByKeyRequest,
    GetNodeRequest,
    GetNodesRequest,
    GetReceiptStatusRequest,
    GetSchemaRequest,
    GetTenantMembersRequest,
    GetTenantRequest,
    GetUserRequest,
    GetUserTenantsRequest,
    GroupMemberRequest,
    HealthRequest,
    LegalHoldRequest,
    ListSharedWithMeRequest,
    ListUsersRequest,
    NodeRef,
    Operation,
    QueryNodesRequest,
    # Enums
    ReceiptStatus,
    # Request types
    RequestContext,
    RevokeAccessRequest,
    RevokeAllUserAccessRequest,
    SearchMailboxRequest,
    SearchNodesRequest,
    ShareNodeRequest,
    # Tenant membership
    TenantMemberRequest,
    TransferOwnershipRequest,
    # Admin operations (Issue #90)
    TransferUserContentRequest,
    TypedNodeRef,
    UpdateNodeOp,
    UpdateUserRequest,
    WaitForOffsetRequest,
)


def _dict_to_struct(d):
    """Convert Python dict to protobuf Struct."""
    s = Struct()
    if d:
        s.update(d)
    return s


def _struct_to_dict(s):
    """Convert protobuf Struct to Python dict."""
    return json_format.MessageToDict(s) if s and s.fields else {}


def _acl_proto_to_list(acl_entries):
    """Convert repeated AclEntry to list of dicts."""
    return [{"principal": e.principal, "permission": e.permission} for e in acl_entries]


_UNIQUE_DETAIL_RE = re.compile(
    r"type_id=(?P<type_id>\d+)\s+field_id=(?P<field_id>\d+)\s+value=(?P<value>.+?)\s+already exists",
    re.IGNORECASE,
)

_COMPOSITE_UNIQUE_DETAIL_RE = re.compile(
    r"type_id=(?P<type_id>\d+)\s+constraint=(?P<constraint>.+?)\s+"
    r"fields=(?P<fields>\[[^\]]*\])\s+values=(?P<values>\[.*\])\s+already exists",
    re.IGNORECASE,
)


def _parse_unique_constraint_detail(detail: str) -> tuple[int | None, int | None, Any]:
    """Parse the server's ALREADY_EXISTS detail string into structured fields.

    The server emits unique-constraint failures using a stable format
    (see ``dbaas/entdb_server/api/grpc_server.py``)::

        Unique constraint violation: type_id=<int> field_id=<int>
        value=<repr> already exists

    The trailing ``value`` is a Python ``repr`` of a scalar, so for
    strings it comes through quoted (``'alice@example.com'``) and for
    numbers / bools / None it is the bare literal. We parse with
    ``ast.literal_eval`` so the typed error carries the original
    Python value, not a stringified one. If the format does not match
    (e.g. a server change the SDK doesn't know about), we return
    ``(None, None, None)`` and let the caller fall back to the raw
    detail message.
    """
    if not detail:
        return None, None, None
    m = _UNIQUE_DETAIL_RE.search(detail)
    if not m:
        return None, None, None
    type_id = int(m.group("type_id"))
    field_id = int(m.group("field_id"))
    raw_value = m.group("value").strip()
    try:
        import ast

        parsed_value: Any = ast.literal_eval(raw_value)
    except (ValueError, SyntaxError):
        parsed_value = raw_value
    return type_id, field_id, parsed_value


def _parse_composite_unique_constraint_detail(
    detail: str,
) -> tuple[int | None, str | None, tuple[int, ...] | None, tuple[Any, ...] | None]:
    """Parse a composite-unique ``ALREADY_EXISTS`` detail.

    The server emits composite (multi-field) unique violations as::

        Composite unique constraint violation: type_id=<int>
        constraint=<repr> fields=[<int>, ...] values=[<repr>, ...]
        already exists

    Returns ``(type_id, constraint_name, field_ids, values)`` or
    ``(None, None, None, None)`` if the detail doesn't match — single-
    field violations parse via ``_parse_unique_constraint_detail``
    instead.
    """
    if not detail:
        return None, None, None, None
    m = _COMPOSITE_UNIQUE_DETAIL_RE.search(detail)
    if not m:
        return None, None, None, None
    import ast

    type_id = int(m.group("type_id"))
    raw_constraint = m.group("constraint").strip()
    try:
        constraint_name = ast.literal_eval(raw_constraint)
    except (ValueError, SyntaxError):
        constraint_name = raw_constraint
    if not isinstance(constraint_name, str):
        constraint_name = str(constraint_name)
    raw_fields = m.group("fields").strip()
    raw_values = m.group("values").strip()
    try:
        fields_parsed = ast.literal_eval(raw_fields)
    except (ValueError, SyntaxError):
        fields_parsed = []
    try:
        values_parsed = ast.literal_eval(raw_values)
    except (ValueError, SyntaxError):
        values_parsed = []
    field_ids = tuple(int(f) for f in (fields_parsed or []))
    values_t = tuple(values_parsed or [])
    return type_id, constraint_name, field_ids, values_t


logger = logging.getLogger(__name__)

_RETRYABLE_STATUS_CODES = frozenset(
    {grpc.StatusCode.UNAVAILABLE, grpc.StatusCode.DEADLINE_EXCEEDED}
)

_DEFAULT_TIMEOUT: float = 30.0


def _multicallable_rebinder(fn: Any):
    """Return a callable ``(channel) -> new_multicallable`` that
    re-issues ``fn``'s RPC on a different gRPC AIO channel.

    The grpc.aio stub binds each RPC to a fixed channel; to follow
    a redirect we need an equivalent multicallable on the redirect
    target's channel. We read the method path and serializers from
    the original — these are stable across the grpc-python releases
    used by the SDK (see the ``UnaryUnaryMultiCallable`` API).

    Returns ``None`` for callables we can't recognise (e.g. a
    custom test fake), so the caller can skip the redirect path
    rather than raising a confusing AttributeError.
    """
    method = getattr(fn, "_method", None)
    if method is None:
        return None
    request_serializer = getattr(fn, "_request_serializer", None)
    response_deserializer = getattr(fn, "_response_deserializer", None)

    method_str = method.decode() if isinstance(method, (bytes, bytearray)) else method

    def _rebind(channel: grpc_aio.Channel):
        return channel.unary_unary(
            method_str,
            request_serializer=request_serializer,
            response_deserializer=response_deserializer,
        )

    return _rebind


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


def _payload_id_to_name(payload: dict[str, Any], type_id: int, registry: Any) -> dict[str, Any]:
    """Translate an id-keyed payload dict to a name-keyed payload dict.

    The wire format is field-id-keyed (per ``CLAUDE.md`` invariant #6
    and the Go SDK contract at ``sdk/go/entdb/marshal.go:342``). The
    Python SDK exposes payloads to users as name-keyed for ergonomics,
    so we translate at the SDK boundary using the local schema
    registry.

    Behavior:
        * Empty payload → empty dict.
        * Registry without a definition for ``type_id`` → pass through
          unchanged (schema-less mode, e.g. raw tests).
        * Keys that aren't digit strings (legacy / partial migrations)
          are passed through verbatim.
        * Unknown id keys are kept as the raw id-string so that
          forward-compatible clients don't lose data.
    """
    if not payload:
        return {}
    if registry is None or not hasattr(registry, "get_node_type"):
        return dict(payload)
    node_type = registry.get_node_type(type_id)
    if node_type is None:
        return dict(payload)
    id_to_name: dict[int, str] = {}
    for f in getattr(node_type, "fields", ()) or ():
        fid = getattr(f, "field_id", None)
        fname = getattr(f, "name", None)
        if fid is not None and fname is not None:
            id_to_name[int(fid)] = fname
    out: dict[str, Any] = {}
    for key, value in payload.items():
        if isinstance(key, str) and key.isdigit():
            name = id_to_name.get(int(key))
            out[name if name is not None else key] = value
        else:
            out[key] = value
    return out


def _node_from_proto(n: Any, registry: Any = None) -> Node:
    """Build a user-facing Node directly from a proto Node message.

    The wire payload is id-keyed; the SDK translates to name-keyed for
    ergonomic user code via the supplied schema ``registry``. When no
    registry is supplied (or the type is not registered) the payload
    is passed through unchanged.
    """
    raw = _struct_to_dict(n.payload)
    payload = _payload_id_to_name(raw, n.type_id, registry) if registry is not None else raw
    return Node(
        tenant_id=n.tenant_id,
        node_id=n.node_id,
        type_id=n.type_id,
        payload=payload,
        created_at=n.created_at,
        updated_at=n.updated_at,
        owner_actor=n.owner_actor,
        acl=_acl_proto_to_list(n.acl),
    )


def _edge_from_proto(e: Any) -> Edge:
    """Build a user-facing Edge directly from a proto Edge message."""
    return Edge(
        tenant_id=e.tenant_id,
        edge_type_id=e.edge_type_id,
        from_node_id=e.from_node_id,
        to_node_id=e.to_node_id,
        props=_struct_to_dict(e.props),
        created_at=e.created_at,
    )


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
        api_key: str | None = None,
        max_retries: int = 3,
        registry: Any = None,
        node_resolver: Any | None = None,
    ) -> None:
        """Initialize the gRPC client.

        Args:
            host: Server hostname
            port: Server port
            secure: Whether to use TLS
            credentials: Optional TLS credentials
            api_key: Optional API key for authentication
            max_retries: Maximum number of retries for transient failures
            registry: Optional schema registry used to translate the
                id-keyed wire payload back to user-facing field names.
            node_resolver: Optional :class:`NodeResolver` used to map
                server-issued ``node_id`` redirect hints to dial-able
                endpoints. When set, the SDK transparently caches a
                sub-channel per tenant the first time the server
                redirects, and routes future calls there.
        """
        self._host = host
        self._port = port
        self._secure = secure
        self._credentials = credentials
        self._api_key = api_key
        self._max_retries = max_retries
        self._registry = registry
        self._channel: grpc_aio.Channel | None = None
        self._stub: EntDBServiceStub | None = None
        self._node_resolver = node_resolver
        # Lazily initialised on first connect — see
        # ``_redirect_cache.py`` for the design.
        self._redirect_cache: Any = None

    def _open_channel(self, address: str) -> grpc_aio.Channel:
        """Open a gRPC channel mirroring the client's TLS settings.

        Used both for the primary connection and for the per-tenant
        sub-channels managed by the redirect cache.
        """
        if self._secure:
            if self._credentials:
                return grpc_aio.secure_channel(address, self._credentials)
            return grpc_aio.secure_channel(address, grpc.ssl_channel_credentials())
        return grpc_aio.insecure_channel(
            address,
            options=[
                ("grpc.max_send_message_length", 50 * 1024 * 1024),
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),
            ],
        )

    async def connect(self) -> None:
        """Establish connection to the server."""
        if self._channel is not None:
            return

        address = f"{self._host}:{self._port}"
        self._channel = self._open_channel(address)
        self._stub = EntDBServiceStub(self._channel)

        if self._node_resolver is not None:
            from ._redirect_cache import TenantEndpointCache

            self._redirect_cache = TenantEndpointCache(channel_factory=self._open_channel)
        logger.debug(f"Connected to EntDB server at {address}")

    async def close(self) -> None:
        """Close the connection."""
        if self._redirect_cache is not None:
            await self._redirect_cache.close()
            self._redirect_cache = None
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

    def _build_metadata(self) -> list[tuple[str, str]]:
        """Build gRPC call metadata including authentication."""
        metadata: list[tuple[str, str]] = []
        if self._api_key:
            metadata.append(("authorization", f"Bearer {self._api_key}"))
        return metadata

    def _make_context(self, tenant_id: str, actor: str, trace_id: str = "") -> RequestContext:
        """Create a request context.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            trace_id: Optional trace ID for distributed tracing
        """
        return RequestContext(tenant_id=tenant_id, actor=actor, trace_id=trace_id)

    async def _retry(
        self,
        fn: Any,
        *args: Any,
        max_retries: int | None = None,
        tenant_id: str = "",
        **kwargs: Any,
    ) -> Any:
        """Retry on transient gRPC errors and follow tenant redirects.

        Behaviour:
            - On UNAVAILABLE / DEADLINE_EXCEEDED, retry with
              exponential backoff up to ``max_retries`` times.
            - On UNAVAILABLE with the ``entdb-redirect-node``
              trailing metadata header (and a non-empty
              ``tenant_id``), resolve the node, dial a sub-channel,
              cache it, and re-issue the call against the cached
              endpoint. Counts as the redirect retry — does not
              consume the transient-failure budget.
            - On UNAVAILABLE against an already-cached sub-channel,
              evict the cache entry so the next call falls back to
              the primary endpoint.

        Args:
            fn: Bound stub method (e.g. ``stub.GetNode``).
            *args: Positional arguments for fn.
            max_retries: Override for max retry attempts.
            tenant_id: Tenant id of the call. Required for the
                redirect path; pass ``""`` for tenant-agnostic RPCs
                (Health, CreateUser, ...).
            **kwargs: Keyword arguments for fn.
        """
        from ._redirect_cache import extract_redirect_node

        # ``fn`` is a UnaryUnaryMultiCallable bound to a specific
        # channel. To re-issue the same RPC on a different channel
        # we re-create the multicallable from the call's
        # ``_method`` path and serializers — these are stable,
        # non-public attributes maintained by grpc.aio across
        # versions used in the SDK.
        rebind = _multicallable_rebinder(fn)

        # If we already cached an endpoint for this tenant, route
        # the first attempt there.
        if tenant_id and self._redirect_cache is not None and rebind is not None:
            entry = await self._redirect_cache.get(tenant_id)
            if entry is not None:
                fn = rebind(entry.channel)
                rebind = _multicallable_rebinder(fn)

        retries = max_retries if max_retries is not None else self._max_retries
        last_error: grpc.RpcError | None = None
        for attempt in range(retries + 1):
            try:
                return await fn(*args, **kwargs)
            except grpc.RpcError as e:
                # Redirect path: server says "try node X" — resolve,
                # cache, retry once on the new sub-channel.
                node_id = extract_redirect_node(e)
                if (
                    node_id
                    and tenant_id
                    and self._redirect_cache is not None
                    and self._node_resolver is not None
                    and rebind is not None
                ):
                    try:
                        endpoint = self._node_resolver.resolve(node_id)
                    except LookupError:
                        # No endpoint — surface the original error.
                        raise e from None
                    entry = await self._redirect_cache.store(tenant_id, endpoint)
                    redirected = rebind(entry.channel)
                    try:
                        return await redirected(*args, **kwargs)
                    except grpc.RpcError as redir_err:
                        # If the redirect target is also broken,
                        # evict so we don't keep using a bad
                        # sub-channel and fall through to the
                        # transient-retry logic below.
                        if redir_err.code() == grpc.StatusCode.UNAVAILABLE:
                            await self._redirect_cache.evict(tenant_id)
                        raise
                # If the call failed against a cached sub-channel,
                # evict so the next call falls back to the primary
                # endpoint (tenant may have moved).
                primary_channel = getattr(self._stub, "_channel", None) if self._stub else None
                fn_channel = getattr(fn, "_channel", None)
                if (
                    e.code() == grpc.StatusCode.UNAVAILABLE
                    and tenant_id
                    and self._redirect_cache is not None
                    and fn_channel is not None
                    and fn_channel is not primary_channel
                ):
                    await self._redirect_cache.evict(tenant_id)
                if e.code() not in _RETRYABLE_STATUS_CODES or attempt == retries:
                    raise
                last_error = e
                await asyncio.sleep(0.1 * (2**attempt))
        raise last_error  # type: ignore[misc]

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
        trace_id: str = "",
        timeout: float | None = None,
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
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            GrpcCommitResult with success/failure info
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        # Convert operations to protobuf format
        proto_ops = self._convert_operations(operations)

        request = ExecuteAtomicRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            idempotency_key=idempotency_key or "",
            schema_fingerprint=schema_fingerprint or "",
            operations=proto_ops,
            wait_applied=wait_applied,
            wait_timeout_ms=wait_timeout_ms,
        )

        try:
            response = await self._retry(
                stub.ExecuteAtomic,
                request,
                timeout=timeout or _DEFAULT_TIMEOUT,
                metadata=metadata,
                tenant_id=tenant_id,
            )

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
            # SDK v0.3 (2026-04-14): ``ALREADY_EXISTS`` from the
            # server means a unique-field collision — parse the
            # structured error string the server emits and surface
            # it as a typed ``UniqueConstraintError``. The server
            # message format (set in dbaas/entdb_server/api/grpc_server.py)
            # is::
            #
            #     Unique constraint violation: type_id=<int>
            #     field_id=<int> value=<repr> already exists
            if getattr(e, "code", None) and e.code() == grpc.StatusCode.ALREADY_EXISTS:
                from .errors import UniqueConstraintError

                detail = e.details() if hasattr(e, "details") else str(e)
                # Try the composite (multi-field) shape first — it's
                # more specific. Fall through to the single-field
                # parser if the detail doesn't match the composite
                # format.
                (
                    c_type_id,
                    c_constraint,
                    c_field_ids,
                    c_values,
                ) = _parse_composite_unique_constraint_detail(detail)
                if c_constraint is not None:
                    raise UniqueConstraintError(
                        detail,
                        tenant_id=tenant_id,
                        type_id=c_type_id,
                        constraint_name=c_constraint,
                        field_ids=c_field_ids,
                        values=c_values,
                    ) from e
                type_id_parsed, field_id_parsed, value_parsed = _parse_unique_constraint_detail(
                    detail
                )
                raise UniqueConstraintError(
                    detail,
                    tenant_id=tenant_id,
                    type_id=type_id_parsed,
                    field_id=field_id_parsed,
                    value=value_parsed,
                ) from e
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
                )
                data = create.get("data")
                if data:
                    create_op.data.update(data)
                acl = create.get("acl")
                if acl:
                    create_op.acl.extend(
                        [
                            AclEntry(
                                principal=e.get("principal", ""), permission=e.get("permission", "")
                            )
                            for e in acl
                        ]
                    )
                if create.get("as"):
                    setattr(create_op, "as", create["as"])
                if create.get("fanout_to"):
                    create_op.fanout_to.extend(create["fanout_to"])
                # Storage routing (2026-04-13)
                sm = create.get("storage_mode")
                if sm:
                    _sm_to_proto = {
                        "TENANT": 0,
                        "USER_MAILBOX": 1,
                        "PUBLIC": 2,
                    }
                    create_op.storage_mode = _sm_to_proto.get(sm, 0)
                if create.get("target_user_id"):
                    create_op.target_user_id = create["target_user_id"]
                # 2026-04-14 SDK v0.3 — the wire-level ``keys`` map is
                # retired. Unique values travel inside the regular
                # payload and are enforced by the server-side unique
                # expression index on the declared proto field.
                proto_op.create_node.CopyFrom(create_op)

            elif "update_node" in op:
                update = op["update_node"]
                update_op = UpdateNodeOp(
                    type_id=update.get("type_id", 0),
                    id=update.get("id", ""),
                )
                patch = update.get("patch")
                if patch:
                    update_op.patch.update(patch)
                if update.get("field_mask"):
                    update_op.field_mask.extend(update["field_mask"])
                # 2026-04-14 SDK v0.3 — see CreateNodeOp note above.
                proto_op.update_node.CopyFrom(update_op)

            elif "delete_node" in op:
                delete = op["delete_node"]
                proto_op.delete_node.CopyFrom(
                    DeleteNodeOp(
                        type_id=delete.get("type_id", 0),
                        id=delete.get("id", ""),
                    )
                )

            elif "create_edge" in op:
                create = op["create_edge"]
                create_op = CreateEdgeOp(
                    edge_id=create.get("edge_id", 0),
                )
                props = create.get("props")
                if props:
                    create_op.props.update(props)
                getattr(create_op, "from").CopyFrom(self._convert_node_ref(create.get("from", {})))
                create_op.to.CopyFrom(self._convert_node_ref(create.get("to", {})))
                proto_op.create_edge.CopyFrom(create_op)

            elif "delete_edge" in op:
                delete = op["delete_edge"]
                delete_op = DeleteEdgeOp(edge_id=delete.get("edge_id", 0))
                getattr(delete_op, "from").CopyFrom(self._convert_node_ref(delete.get("from", {})))
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
            node_ref.typed.CopyFrom(
                TypedNodeRef(
                    type_id=ref["typed"].get("type_id", 0),
                    id=ref["typed"].get("id", ""),
                )
            )

        return node_ref

    async def get_receipt_status(
        self,
        tenant_id: str,
        idempotency_key: str,
        *,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> str:
        """Get the status of a transaction receipt.

        Args:
            tenant_id: Tenant identifier
            idempotency_key: Transaction key
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Status string: "PENDING", "APPLIED", "FAILED", or "UNKNOWN"
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetReceiptStatusRequest(
            context=self._make_context(tenant_id, "system", trace_id),
            idempotency_key=idempotency_key,
        )

        response = await self._retry(
            stub.GetReceiptStatus,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        status_map = {
            ReceiptStatus.RECEIPT_STATUS_PENDING: "PENDING",
            ReceiptStatus.RECEIPT_STATUS_APPLIED: "APPLIED",
            ReceiptStatus.RECEIPT_STATUS_FAILED: "FAILED",
            ReceiptStatus.RECEIPT_STATUS_UNKNOWN: "UNKNOWN",
        }
        return status_map.get(response.status, "UNKNOWN")

    async def wait_for_offset(
        self,
        tenant_id: str,
        stream_position: str,
        *,
        timeout_ms: int = 30000,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[bool, str]:
        """Wait for a stream position to be applied.

        Args:
            tenant_id: Tenant identifier
            stream_position: Target stream position
            timeout_ms: Server-side wait timeout in milliseconds
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call gRPC timeout in seconds

        Returns:
            Tuple of (reached, current_position)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = WaitForOffsetRequest(
            context=self._make_context(tenant_id, "system", trace_id),
            stream_position=stream_position,
            timeout_ms=timeout_ms,
        )

        response = await self._retry(
            stub.WaitForOffset,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return response.reached, response.current_position

    async def get_node(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        node_id: str,
        *,
        after_offset: str | None = None,
        wait_timeout_ms: int = 0,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> Node | None:
        """Get a node by ID.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            type_id: Node type ID
            node_id: Node identifier
            after_offset: Wait for this offset before reading
            wait_timeout_ms: Timeout for offset wait
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Node if found, None otherwise
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetNodeRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            type_id=type_id,
            node_id=node_id,
            after_offset=after_offset or "",
            wait_timeout_ms=wait_timeout_ms,
        )

        response = await self._retry(
            stub.GetNode,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        if not response.found:
            return None

        return _node_from_proto(response.node, self._registry)

    async def get_node_by_key(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        field_id: int,
        value: Any,
        *,
        after_offset: int = 0,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> Node | None:
        """Resolve a node via a declared unique field.

        Per the 2026-04-14 SDK v0.3 decision the unique-key lookup is
        expressed as a ``(type_id, field_id, value)`` triple; the
        scalar value is packed into a ``google.protobuf.Value`` on the
        wire so ints, floats, strings, and bools all round-trip.
        """
        from google.protobuf.struct_pb2 import Value

        stub = self._ensure_connected()
        metadata = self._build_metadata()

        value_msg = Value()
        if value is None:
            value_msg.null_value = 0
        elif isinstance(value, bool):
            value_msg.bool_value = value
        elif isinstance(value, (int, float)):
            value_msg.number_value = float(value)
        elif isinstance(value, str):
            value_msg.string_value = value
        else:
            # Fallback: JSON-encode into a string for complex shapes.
            value_msg.string_value = json.dumps(value)

        request = GetNodeByKeyRequest(
            tenant_id=tenant_id,
            actor=actor,
            type_id=type_id,
            field_id=int(field_id),
            value=value_msg,
            after_offset=after_offset,
        )

        response = await self._retry(
            stub.GetNodeByKey,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        if not response.found:
            return None
        return _node_from_proto(response.node, self._registry)

    async def get_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        node_ids: list[str],
        *,
        after_offset: str | None = None,
        wait_timeout_ms: int = 0,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[list[Node], list[str]]:
        """Get multiple nodes by IDs.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            type_id: Node type ID
            node_ids: Node identifiers
            after_offset: Wait for this offset before reading
            wait_timeout_ms: Timeout for offset wait
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (found nodes, missing node IDs)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetNodesRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            type_id=type_id,
            node_ids=node_ids,
            after_offset=after_offset or "",
            wait_timeout_ms=wait_timeout_ms,
        )

        response = await self._retry(
            stub.GetNodes,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        nodes = [_node_from_proto(n, self._registry) for n in response.nodes]

        return nodes, list(response.missing_ids)

    async def query_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        *,
        limit: int = 100,
        offset: int = 0,
        filter: dict[str, Any] | None = None,
        order_by: str | None = None,
        descending: bool = False,
        after_offset: str | None = None,
        wait_timeout_ms: int = 0,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[list[Node], bool]:
        """Query nodes by type.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            type_id: Node type ID
            limit: Maximum nodes to return
            offset: Pagination offset
            filter: Filter dict (field_name → value for equality)
            order_by: Field to order results by
            descending: Whether to sort in descending order
            after_offset: Wait for this offset before reading
            wait_timeout_ms: Timeout for offset wait
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (nodes, has_more)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        # Convert filter dict to repeated FieldFilter
        filters = []
        if filter:
            from google.protobuf.struct_pb2 import Value

            for field_name, field_value in filter.items():
                v = Value()
                json_format.Parse(json.dumps(field_value), v)
                filters.append(FieldFilter(field=field_name, value=v))

        request = QueryNodesRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            type_id=type_id,
            limit=limit,
            offset=offset,
            filters=filters,
            order_by=order_by or "",
            descending=descending,
            after_offset=after_offset or "",
            wait_timeout_ms=wait_timeout_ms,
        )

        response = await self._retry(
            stub.QueryNodes,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        nodes = [_node_from_proto(n, self._registry) for n in response.nodes]

        return nodes, response.has_more

    async def get_edges_from(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int | None = None,
        limit: int = 100,
        *,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[list[Edge], bool]:
        """Get outgoing edges from a node.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            node_id: Source node ID
            edge_type_id: Optional edge type filter
            limit: Maximum edges to return
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (edges, has_more)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetEdgesRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await self._retry(
            stub.GetEdgesFrom,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        edges = [_edge_from_proto(e) for e in response.edges]

        return edges, response.has_more

    async def get_edges_to(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int | None = None,
        limit: int = 100,
        *,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[list[Edge], bool]:
        """Get incoming edges to a node.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            node_id: Target node ID
            edge_type_id: Optional edge type filter
            limit: Maximum edges to return
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (edges, has_more)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetEdgesRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await self._retry(
            stub.GetEdgesTo,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        edges = [_edge_from_proto(e) for e in response.edges]

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
        trace_id: str = "",
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """Search user's mailbox with full-text search.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            user_id: User whose mailbox to search
            query: Search query string
            source_type_ids: Optional type ID filter
            limit: Maximum results
            offset: Pagination offset
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of search result dictionaries
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = SearchMailboxRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            user_id=user_id,
            query=query,
            limit=limit,
            offset=offset,
        )
        if source_type_ids:
            request.source_type_ids.extend(source_type_ids)

        response = await self._retry(
            stub.SearchMailbox,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return [
            {
                "item": {
                    "item_id": r.item.item_id,
                    "ref_id": r.item.ref_id,
                    "source_type_id": r.item.source_type_id,
                    "source_node_id": r.item.source_node_id,
                    "thread_id": r.item.thread_id,
                    "ts": r.item.ts,
                    "state": _struct_to_dict(r.item.state),
                    "snippet": r.item.snippet,
                    "metadata": _struct_to_dict(r.item.metadata),
                },
                "rank": r.rank,
                "highlights": r.highlights,
            }
            for r in response.results
        ]

    async def search_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        query: str,
        *,
        limit: int = 50,
        offset: int = 0,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> list[Node]:
        """Full-text search across searchable fields of a node type.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            type_id: Node type ID
            query: FTS5 match expression
            limit: Maximum results
            offset: Pagination offset
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of matching nodes ordered by relevance
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = SearchNodesRequest(
            tenant_id=tenant_id,
            actor=actor,
            type_id=type_id,
            query=query,
            limit=limit,
            offset=offset,
        )

        response = await self._retry(
            stub.SearchNodes,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return [_node_from_proto(n, self._registry) for n in response.nodes]

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
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[list[dict[str, Any]], int, bool]:
        """Get mailbox items for a user.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            user_id: User whose mailbox to retrieve
            source_type_id: Optional type ID filter
            thread_id: Optional thread filter
            unread_only: Whether to return only unread items
            limit: Maximum items to return
            offset: Pagination offset
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (items, unread_count, has_more)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetMailboxRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            user_id=user_id,
            source_type_id=source_type_id or 0,
            thread_id=thread_id or "",
            unread_only=unread_only,
            limit=limit,
            offset=offset,
        )

        response = await self._retry(
            stub.GetMailbox,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        items = [
            {
                "item_id": item.item_id,
                "ref_id": item.ref_id,
                "source_type_id": item.source_type_id,
                "source_node_id": item.source_node_id,
                "thread_id": item.thread_id,
                "ts": item.ts,
                "state": _struct_to_dict(item.state),
                "snippet": item.snippet,
                "metadata": _struct_to_dict(item.metadata),
            }
            for item in response.items
        ]

        return items, response.unread_count, response.has_more

    async def health(
        self,
        *,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Get server health status.

        Args:
            timeout: Per-call timeout in seconds

        Returns:
            Health status dictionary
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        response = await self._retry(
            stub.Health,
            HealthRequest(),
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )

        return {
            "healthy": response.healthy,
            "version": response.version,
            "components": dict(response.components),
        }

    async def get_schema(
        self,
        type_id: int | None = None,
        *,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Get schema information.

        Args:
            type_id: Optional type ID to retrieve specific schema
            timeout: Per-call timeout in seconds

        Returns:
            Schema information dictionary
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetSchemaRequest(type_id=type_id or 0)
        response = await self._retry(
            stub.GetSchema,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )

        return {
            "schema": _struct_to_dict(response.schema),
            "fingerprint": response.fingerprint,
        }

    # --- ACL v2 methods ---

    async def get_connected_nodes(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int,
        *,
        limit: int = 100,
        offset: int = 0,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[list[Node], bool]:
        """Get connected nodes via edge type with ACL filtering.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            node_id: Source node ID
            edge_type_id: Edge type to traverse
            limit: Maximum nodes to return
            offset: Pagination offset
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (nodes, has_more)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetConnectedNodesRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            node_id=node_id,
            edge_type_id=edge_type_id,
            limit=limit,
            offset=offset,
        )

        response = await self._retry(
            stub.GetConnectedNodes,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        nodes = [_node_from_proto(n, self._registry) for n in response.nodes]

        return nodes, response.has_more

    async def share_node(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        actor_id: str,
        permission: str = "read",
        *,
        actor_type: str = "user",
        expires_at: int | None = None,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> bool:
        """Share a node with an actor.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the share (granted_by)
            node_id: Node to share
            actor_id: Actor to share with
            permission: Permission level (read, write, admin)
            actor_type: Type of actor (user, group)
            expires_at: Optional expiry timestamp (Unix ms)
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if successful
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = ShareNodeRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            node_id=node_id,
            actor_id=actor_id,
            permission=permission,
            actor_type=actor_type,
            expires_at=expires_at or 0,
        )

        response = await self._retry(
            stub.ShareNode,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return response.success

    async def revoke_access(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        actor_id: str,
        *,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> bool:
        """Revoke access from an actor on a node.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the revocation
            node_id: Node to revoke access from
            actor_id: Actor to revoke
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if a grant existed and was removed
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = RevokeAccessRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            node_id=node_id,
            actor_id=actor_id,
        )

        response = await self._retry(
            stub.RevokeAccess,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return response.found

    async def list_shared_with_me(
        self,
        tenant_id: str,
        actor: str,
        *,
        limit: int = 100,
        offset: int = 0,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> tuple[list[Node], bool]:
        """List nodes shared with the calling actor.

        Args:
            tenant_id: Tenant identifier
            actor: Actor making request
            limit: Maximum nodes to return
            offset: Pagination offset
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            Tuple of (nodes, has_more)
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = ListSharedWithMeRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            limit=limit,
            offset=offset,
        )

        response = await self._retry(
            stub.ListSharedWithMe,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        nodes = [_node_from_proto(n, self._registry) for n in response.nodes]

        return nodes, response.has_more

    async def add_group_member(
        self,
        tenant_id: str,
        actor: str,
        group_id: str,
        member_actor_id: str,
        role: str = "member",
        *,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> bool:
        """Add a member to a group.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            group_id: Group to add member to
            member_actor_id: Actor to add
            role: Role in the group
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if successful
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GroupMemberRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            group_id=group_id,
            member_actor_id=member_actor_id,
            role=role,
        )

        response = await self._retry(
            stub.AddGroupMember,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return response.success

    async def remove_group_member(
        self,
        tenant_id: str,
        actor: str,
        group_id: str,
        member_actor_id: str,
        *,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> bool:
        """Remove a member from a group.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the operation
            group_id: Group to remove member from
            member_actor_id: Actor to remove
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if member existed and was removed
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GroupMemberRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            group_id=group_id,
            member_actor_id=member_actor_id,
        )

        response = await self._retry(
            stub.RemoveGroupMember,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return response.success

    async def transfer_ownership(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        new_owner: str,
        *,
        trace_id: str = "",
        timeout: float | None = None,
    ) -> bool:
        """Transfer ownership of a node.

        Args:
            tenant_id: Tenant identifier
            actor: Actor performing the transfer
            node_id: Node to transfer
            new_owner: New owner actor
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            True if node existed and ownership was transferred
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = TransferOwnershipRequest(
            context=self._make_context(tenant_id, actor, trace_id),
            node_id=node_id,
            new_owner=new_owner,
        )

        response = await self._retry(
            stub.TransferOwnership,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return response.found

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
            email: User email
            name: Display name
            actor: Actor performing the operation (must be admin or system)
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, user, and optional error
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = CreateUserRequest(
            actor=actor,
            user_id=user_id,
            email=email,
            name=name,
        )

        response = await self._retry(
            stub.CreateUser,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )

        result: dict[str, Any] = {
            "success": response.success,
            "error": response.error if response.error else None,
        }
        if response.user and response.user.user_id:
            result["user"] = {
                "user_id": response.user.user_id,
                "email": response.user.email,
                "name": response.user.name,
                "status": response.user.status,
                "created_at": response.user.created_at,
                "updated_at": response.user.updated_at,
            }
        return result

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
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetUserRequest(
            actor=actor,
            user_id=user_id,
        )

        response = await self._retry(
            stub.GetUser,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )

        if not response.found:
            return None

        return {
            "user_id": response.user.user_id,
            "email": response.user.email,
            "name": response.user.name,
            "status": response.user.status,
            "created_at": response.user.created_at,
            "updated_at": response.user.updated_at,
        }

    async def update_user(
        self,
        user_id: str,
        *,
        actor: str = "",
        email: str = "",
        name: str = "",
        status: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Update user fields.

        Args:
            user_id: User identifier
            actor: Actor performing the operation (user themselves or admin)
            email: New email (empty = no change)
            name: New name (empty = no change)
            status: New status (empty = no change)
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and optional error
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = UpdateUserRequest(
            actor=actor or f"user:{user_id}",
            user_id=user_id,
            email=email,
            name=name,
            status=status,
        )

        response = await self._retry(
            stub.UpdateUser,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )

        return {
            "success": response.success,
            "error": response.error if response.error else None,
        }

    async def list_users(
        self,
        *,
        actor: str = "system:admin",
        status: str = "active",
        limit: int = 100,
        offset: int = 0,
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """List users filtered by status.

        Args:
            actor: Actor performing the operation
            status: Status filter (e.g. 'active', 'suspended')
            limit: Maximum results
            offset: Pagination offset
            timeout: Per-call timeout in seconds

        Returns:
            List of user dicts
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = ListUsersRequest(
            actor=actor,
            status=status,
            limit=limit,
            offset=offset,
        )

        response = await self._retry(
            stub.ListUsers,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )

        return [
            {
                "user_id": u.user_id,
                "email": u.email,
                "name": u.name,
                "status": u.status,
                "created_at": u.created_at,
                "updated_at": u.updated_at,
            }
            for u in response.users
        ]

    # --- Tenant registry methods ---

    async def create_tenant(
        self,
        actor: str,
        tenant_id: str,
        name: str,
        *,
        region: str | None = None,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Create a new tenant.

        Args:
            actor: Actor performing the operation
            tenant_id: Unique tenant identifier
            name: Tenant display name
            region: Optional geographic region pin (e.g. ``"us-east-1"``,
                ``"eu-west-1"``). When ``None`` the server defaults the
                tenant to its own served region. Once the pin is set,
                cross-region requests are rejected with
                ``FAILED_PRECONDITION``.
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, tenant (including the resolved region),
            and error fields
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = CreateTenantRequest(
            actor=actor,
            tenant_id=tenant_id,
            name=name,
            region=region or "",
        )

        response = await self._retry(
            stub.CreateTenant,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        result: dict[str, Any] = {
            "success": response.success,
            "error": response.error if response.error else None,
        }
        if response.tenant and response.tenant.tenant_id:
            result["tenant"] = {
                "tenant_id": response.tenant.tenant_id,
                "name": response.tenant.name,
                "status": response.tenant.status,
                "region": response.tenant.region,
                "created_at": response.tenant.created_at,
            }
        return result

    async def get_tenant(
        self,
        actor: str,
        tenant_id: str,
        *,
        timeout: float | None = None,
    ) -> dict[str, Any] | None:
        """Get a tenant by ID.

        Args:
            actor: Actor performing the operation
            tenant_id: Tenant identifier
            timeout: Per-call timeout in seconds

        Returns:
            Tenant dict or None if not found
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetTenantRequest(
            actor=actor,
            tenant_id=tenant_id,
        )

        response = await self._retry(
            stub.GetTenant,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        if not response.found:
            return None

        return {
            "tenant_id": response.tenant.tenant_id,
            "name": response.tenant.name,
            "status": response.tenant.status,
            "region": response.tenant.region,
            "created_at": response.tenant.created_at,
        }

    async def archive_tenant(
        self,
        actor: str,
        tenant_id: str,
        *,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Archive a tenant.

        Args:
            actor: Actor performing the operation
            tenant_id: Tenant identifier
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = ArchiveTenantRequest(
            actor=actor,
            tenant_id=tenant_id,
        )

        response = await self._retry(
            stub.ArchiveTenant,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return {
            "success": response.success,
            "error": response.error if response.error else None,
        }

    async def add_tenant_member(
        self,
        actor: str,
        tenant_id: str,
        user_id: str,
        role: str = "member",
        *,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Add a member to a tenant.

        Args:
            actor: Actor performing the operation
            tenant_id: Tenant identifier
            user_id: User to add
            role: Membership role
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = TenantMemberRequest(
            actor=actor,
            tenant_id=tenant_id,
            user_id=user_id,
            role=role,
        )

        response = await self._retry(
            stub.AddTenantMember,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return {
            "success": response.success,
            "error": response.error if response.error else None,
        }

    async def remove_tenant_member(
        self,
        actor: str,
        tenant_id: str,
        user_id: str,
        *,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Remove a member from a tenant.

        Args:
            actor: Actor performing the operation
            tenant_id: Tenant identifier
            user_id: User to remove
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = TenantMemberRequest(
            actor=actor,
            tenant_id=tenant_id,
            user_id=user_id,
        )

        response = await self._retry(
            stub.RemoveTenantMember,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return {
            "success": response.success,
            "error": response.error if response.error else None,
        }

    async def get_tenant_members(
        self,
        actor: str,
        tenant_id: str,
        *,
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """List all members of a tenant.

        Args:
            actor: Actor performing the operation
            tenant_id: Tenant identifier
            timeout: Per-call timeout in seconds

        Returns:
            List of member dicts
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetTenantMembersRequest(
            actor=actor,
            tenant_id=tenant_id,
        )

        response = await self._retry(
            stub.GetTenantMembers,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return [
            {
                "tenant_id": m.tenant_id,
                "user_id": m.user_id,
                "role": m.role,
                "joined_at": m.joined_at,
            }
            for m in response.members
        ]

    async def get_user_tenants(
        self,
        actor: str,
        user_id: str,
        *,
        timeout: float | None = None,
    ) -> list[dict[str, Any]]:
        """List all tenants a user belongs to.

        Args:
            actor: Actor performing the operation
            user_id: User identifier
            timeout: Per-call timeout in seconds

        Returns:
            List of membership dicts
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = GetUserTenantsRequest(
            actor=actor,
            user_id=user_id,
        )

        response = await self._retry(
            stub.GetUserTenants,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )

        return [
            {
                "tenant_id": m.tenant_id,
                "user_id": m.user_id,
                "role": m.role,
                "joined_at": m.joined_at,
            }
            for m in response.memberships
        ]

    async def change_member_role(
        self,
        actor: str,
        tenant_id: str,
        user_id: str,
        new_role: str,
        *,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Change a member's role in a tenant.

        Args:
            actor: Actor performing the operation
            tenant_id: Tenant identifier
            user_id: User whose role to change
            new_role: New role
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success and error fields
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = ChangeMemberRoleRequest(
            actor=actor,
            tenant_id=tenant_id,
            user_id=user_id,
            new_role=new_role,
        )

        response = await self._retry(
            stub.ChangeMemberRole,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )

        return {
            "success": response.success,
            "error": response.error if response.error else None,
        }

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
        """Reassign ownership of all nodes from one user to another.

        Args:
            tenant_id: Tenant identifier
            from_user: Current owner (e.g. ``user:alice``)
            to_user: New owner (e.g. ``user:bob``)
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with ``success``, ``transferred`` (count), ``error``.
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = TransferUserContentRequest(
            actor=actor,
            tenant_id=tenant_id,
            from_user=from_user,
            to_user=to_user,
        )
        response = await self._retry(
            stub.TransferUserContent,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )
        return {
            "success": response.success,
            "transferred": response.transferred,
            "error": response.error if response.error else None,
        }

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
        """Grant temporary access to another user's content.

        Args:
            tenant_id: Tenant identifier
            from_user: Content owner
            to_user: Recipient of delegated access
            permission: Permission level (``read``, ``write``, ...)
            expires_at: Expiry timestamp in Unix ms, or None for permanent
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with ``success``, ``delegated`` (count), ``expires_at``, ``error``.
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = DelegateAccessRequest(
            actor=actor,
            tenant_id=tenant_id,
            from_user=from_user,
            to_user=to_user,
            permission=permission,
            expires_at=expires_at or 0,
        )
        response = await self._retry(
            stub.DelegateAccess,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )
        return {
            "success": response.success,
            "delegated": response.delegated,
            "expires_at": response.expires_at,
            "error": response.error if response.error else None,
        }

    async def set_legal_hold(
        self,
        tenant_id: str,
        enabled: bool,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Enable or disable legal hold on a tenant.

        Args:
            tenant_id: Tenant identifier
            enabled: True to put tenant on hold, False to release
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with ``success``, ``status`` (``legal_hold`` or
            ``active``), and ``error``.
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = LegalHoldRequest(
            actor=actor,
            tenant_id=tenant_id,
            enabled=enabled,
        )
        response = await self._retry(
            stub.SetLegalHold,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )
        return {
            "success": response.success,
            "status": response.status,
            "error": response.error if response.error else None,
        }

    async def revoke_all_user_access(
        self,
        tenant_id: str,
        user_id: str,
        *,
        actor: str = "system:admin",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Remove ALL access for a user in a tenant.

        Removes node_access grants, group memberships, and
        cross-tenant shared_index entries for the given user.

        Args:
            tenant_id: Tenant identifier
            user_id: User to revoke
            actor: Admin actor performing the operation
            timeout: Per-call timeout in seconds

        Returns:
            Dict with ``success``, ``revoked_grants``,
            ``revoked_groups``, ``revoked_shared``, and ``error``.
        """
        stub = self._ensure_connected()
        metadata = self._build_metadata()

        request = RevokeAllUserAccessRequest(
            actor=actor,
            tenant_id=tenant_id,
            user_id=user_id,
        )
        response = await self._retry(
            stub.RevokeAllUserAccess,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
            tenant_id=tenant_id,
        )
        return {
            "success": response.success,
            "revoked_grants": response.revoked_grants,
            "revoked_groups": response.revoked_groups,
            "revoked_shared": response.revoked_shared,
            "error": response.error if response.error else None,
        }

    # --- GDPR operations (Issue #103) ---

    async def delete_user(
        self,
        user_id: str,
        *,
        actor: str = "",
        grace_days: int = 30,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Queue a user for GDPR right-to-erasure."""
        stub = self._ensure_connected()
        metadata = self._build_metadata()
        actor_str = actor or f"user:{user_id}"
        request = DeleteUserRequest(actor=actor_str, user_id=user_id, grace_days=grace_days)
        response = await self._retry(
            stub.DeleteUser,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )
        return {
            "success": response.success,
            "requested_at": response.requested_at,
            "execute_at": response.execute_at,
            "status": response.status,
            "error": response.error if response.error else None,
        }

    async def cancel_user_deletion(
        self,
        user_id: str,
        *,
        actor: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Cancel a pending user deletion during the grace period."""
        stub = self._ensure_connected()
        metadata = self._build_metadata()
        actor_str = actor or f"user:{user_id}"
        request = CancelUserDeletionRequest(actor=actor_str, user_id=user_id)
        response = await self._retry(
            stub.CancelUserDeletion,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )
        return {
            "success": response.success,
            "error": response.error if response.error else None,
        }

    async def export_user_data(
        self,
        user_id: str,
        *,
        actor: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Export a user's data across all tenants (JSON bundle)."""
        import json as _json

        stub = self._ensure_connected()
        metadata = self._build_metadata()
        actor_str = actor or f"user:{user_id}"
        request = ExportUserDataRequest(actor=actor_str, user_id=user_id)
        response = await self._retry(
            stub.ExportUserData,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )
        bundle = None
        if response.export_json:
            try:
                bundle = _json.loads(response.export_json)
            except Exception:
                bundle = None
        return {
            "success": response.success,
            "bundle": bundle,
            "error": response.error if response.error else None,
        }

    async def freeze_user(
        self,
        user_id: str,
        *,
        enabled: bool = True,
        actor: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Freeze or unfreeze a user (GDPR Article 18)."""
        stub = self._ensure_connected()
        metadata = self._build_metadata()
        actor_str = actor or f"user:{user_id}"
        request = FreezeUserRequest(actor=actor_str, user_id=user_id, enabled=enabled)
        response = await self._retry(
            stub.FreezeUser,
            request,
            timeout=timeout or _DEFAULT_TIMEOUT,
            metadata=metadata,
        )
        return {
            "success": response.success,
            "status": response.status,
            "error": response.error if response.error else None,
        }
