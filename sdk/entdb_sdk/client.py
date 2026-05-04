"""
EntDB Client for Python SDK — single-shape v0.3 API.

This module provides the main client interface:
- DbClient: Connection to EntDB server
- Plan: Atomic transaction builder (proto messages everywhere)
- Receipt: Transaction receipt for status checking

Example::

    async with DbClient("localhost:50051") as db:
        plan = db.atomic("tenant_1", "user:42")
        plan.create(schema_pb2.Task(title="My Task"))
        result = await plan.commit()

Invariants:
    - All operations require tenant_id and actor.
    - ``Plan.create`` / ``Plan.update`` take a proto message; the
      type id is read from the ``(entdb.node)`` option, the payload
      from ``ListFields()``.
    - ``Plan.delete`` / ``Plan.edge_create`` / ``Plan.edge_delete``
      take the proto message *class* as the type witness.
    - Writes are batched and committed atomically per ``commit()``.
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

from ._grpc_client import Edge, GrpcClient, Node
from .errors import (
    ConnectionError,
    UnknownFieldError,
    ValidationError,
)
from .keys import Mailbox, Public, Storage, Tenant, UniqueKey
from .registry import SchemaRegistry, get_registry
from .schema import EdgeTypeDef, NodeTypeDef
from .scope import ActorScope, ScopedPlan, TenantScope
from .typed import ACLEntry, TypedNode

# Re-exported for backwards compatibility. Node/Edge are defined in
# _grpc_client.py so the gRPC layer can build them directly from proto
# messages without an intermediate shape.
__all__ = ["Node", "Edge", "TenantScope", "ActorScope", "ScopedPlan"]

logger = logging.getLogger(__name__)


def _is_proto_message(obj: Any) -> bool:
    """Return True if ``obj`` looks like a generated protobuf message."""
    return hasattr(obj, "DESCRIPTOR") and hasattr(obj, "ListFields")


def _is_proto_message_class(obj: Any) -> bool:
    """Return True if ``obj`` is a generated protobuf message *class*."""
    return isinstance(obj, type) and hasattr(obj, "DESCRIPTOR") and hasattr(obj, "ListFields")


def _node_type_id_from_descriptor(descriptor: Any, *, kind: str = "node") -> int:
    """Extract the numeric type_id from a proto message ``DESCRIPTOR``.

    Reads the ``(entdb.node)`` (or ``(entdb.edge)``) option attached
    to the message. Raises :class:`ValidationError` if the message
    has no such annotation, since the SDK would have no way to route
    the operation otherwise.
    """
    from ._generated import entdb_options_pb2

    opts = descriptor.GetOptions()
    if kind == "node":
        if opts.HasExtension(entdb_options_pb2.node):
            return int(opts.Extensions[entdb_options_pb2.node].type_id)
        raise ValidationError(
            f"Proto message {descriptor.name} has no (entdb.node) option",
            errors=[f"{descriptor.name} missing (entdb.node) annotation"],
        )
    if opts.HasExtension(entdb_options_pb2.edge):
        return int(opts.Extensions[entdb_options_pb2.edge].edge_id)
    raise ValidationError(
        f"Proto message {descriptor.name} has no (entdb.edge) option",
        errors=[f"{descriptor.name} missing (entdb.edge) annotation"],
    )


def _proto_payload_from_set_fields(msg: Any) -> dict[str, Any]:
    """Return a name-keyed payload dict containing only SET fields.

    Uses ``ListFields()`` (which only yields explicitly-set fields)
    so that ``Product(price_cents=1499)`` produces ``{"price_cents":
    1499}`` and not the full set of scalar defaults. This is the
    exact semantics we want for both create (only the fields the
    caller intends to set are sent) and update (the patch is exactly
    the set of fields present on the message). The wire-level
    translation from field name to field id happens server-side via
    the schema registry — see CLAUDE.md "Field IDs, not field names,
    on disk".

    We read scalars directly off the proto message rather than going
    through ``MessageToDict`` so that 64-bit integers stay as Python
    ``int`` instead of being coerced to a JSON-safe string (which
    ``MessageToDict`` does for int64 / uint64 by default and which
    would then trip the SDK's payload type-checker).
    """
    from google.protobuf.descriptor import FieldDescriptor
    from google.protobuf.json_format import MessageToDict

    out: dict[str, Any] = {}
    for fd, value in msg.ListFields():
        # Repeated fields and nested messages: defer to MessageToDict
        # for JSON-friendly conversion, then take the right key.
        is_message = fd.type == FieldDescriptor.TYPE_MESSAGE
        is_repeated = fd.label == FieldDescriptor.LABEL_REPEATED
        if is_message or is_repeated:
            sub = MessageToDict(msg, preserving_proto_field_name=True)
            if fd.name in sub:
                out[fd.name] = sub[fd.name]
            continue

        # Scalars: take the raw Python value off the message.
        # For bytes fields the proto runtime already gives us bytes;
        # encode to base64 the same way MessageToDict would for
        # consistency with the wire format.
        if fd.type == FieldDescriptor.TYPE_BYTES:
            import base64

            out[fd.name] = (
                base64.b64encode(value).decode("ascii")
                if isinstance(value, (bytes, bytearray))
                else value
            )
        else:
            out[fd.name] = value
    return out


def _resolve_create_input(
    node_or_type: Any,
    data: dict[str, Any] | None,
    registry: Any,
    **kwargs: Any,
) -> tuple[Any, dict[str, Any]]:
    """Resolve create() input to (NodeTypeDef, payload dict).

    Accepts proto messages, TypedNode instances, or NodeTypeDef + dict.
    """
    # Proto message — has DESCRIPTOR attribute from protoc output
    if _is_proto_message(node_or_type):
        type_id = _node_type_id_from_descriptor(node_or_type.DESCRIPTOR)
        node_type = registry.get_node_type(type_id)
        if node_type is None:
            raise ValidationError(
                f"type_id {type_id} from {node_or_type.DESCRIPTOR.name} not in registry",
                errors=[f"type_id {type_id} not registered"],
            )
        payload = _proto_payload_from_set_fields(node_or_type)
        payload.update(kwargs)
        return node_type, payload

    # TypedNode instance
    if isinstance(node_or_type, TypedNode):
        node_type = registry.get_node_type(node_or_type._type_id)
        if node_type is None:
            raise ValidationError(
                f"Unknown type_id {node_or_type._type_id}",
                errors=[f"type_id {node_or_type._type_id} not in registry"],
            )
        payload = node_or_type.to_payload()
        payload.update(kwargs)
        return node_type, payload

    # NodeTypeDef + data dict (original path)
    payload = dict(data or {})
    payload.update(kwargs)
    return node_or_type, payload


def _acl_entries_to_dicts(
    entries: list[ACLEntry] | list[dict[str, str]],
) -> list[dict[str, str]]:
    """Normalize ACL entries: accept both typed ACLEntry and raw dicts."""
    result = []
    for e in entries:
        if isinstance(e, ACLEntry):
            d: dict[str, str] = {
                "grantee": e.grantee,
                "permission": e.permission.value,
                "actor_type": e.actor_type,
            }
            if e.expires_at is not None:
                d["expires_at"] = str(e.expires_at)
            result.append(d)
        else:
            result.append(e)
    return result


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
    """Atomic transaction builder — single-shape v0.3 API.

    A Plan collects operations to be executed atomically. Every
    write method takes a proto message (``create`` / ``update``) or
    a proto message class (``delete`` / ``edge_create`` /
    ``edge_delete``) — there are no parallel ``*WithACL`` /
    ``*InMailbox`` methods, no ``keys=`` parameter, and no
    ``NodeTypeDef + dict`` shape. See ``docs/decisions/sdk_api.md``.

    Example::

        plan = db.atomic("acme", "user:alice")
        plan.create(schema_pb2.Product(sku="WIDGET-1", name="Widget"))
        plan.edge_create(schema_pb2.BelongsTo, "$prod.id", "$cat.id")
        await plan.commit()
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
        msg: Any,
        *,
        acl: list[ACLEntry] | list[dict[str, str]] | None = None,
        storage: Storage | None = None,
        as_: str | None = None,
        fanout_to: list[str] | None = None,
    ) -> Plan:
        """Create a node from a proto message.

        This is the single-shape ``create`` per the 2026-04-14 SDK
        v0.3 decision. The proto message determines the type
        (read from its ``(entdb.node)`` option) and the payload
        (read from the set fields via ``ListFields()``). The user
        never types a ``type_id`` literal.

        Args:
            msg: A generated proto message instance whose descriptor
                carries an ``(entdb.node)`` option. Only fields that
                are explicitly set on the message are sent.
            acl: Explicit ACL entries to attach to the new node.
            storage: Storage descriptor (``Tenant()``, ``Mailbox(user_id=...)``,
                or ``Public()``). Defaults to :class:`Tenant`.
            as_: Local alias for referencing this node in subsequent
                operations within the same plan (e.g. as an edge
                endpoint).
            fanout_to: Optional list of mailbox user ids to fan this
                node out to.

        Returns:
            Self for chaining.
        """
        self._ensure_not_committed()

        if not _is_proto_message(msg):
            raise TypeError(
                "Plan.create requires a proto message instance — "
                f"got {type(msg).__name__}. Pass an instance like "
                "schema_pb2.Product(sku='WIDGET-1', ...)."
            )

        node_type, payload = _resolve_create_input(msg, None, self._client.registry)

        is_valid, errors = node_type.validate_payload(payload)
        if not is_valid:
            known = {f.name for f in node_type.fields}
            unknown = set(payload.keys()) - known
            if unknown:
                field_name = list(unknown)[0]
                suggestions = [n for n in known if field_name.lower() in n.lower()][:3]
                raise UnknownFieldError(field_name, node_type.name, suggestions)
            raise ValidationError("; ".join(errors), errors=errors)

        acl_dicts = _acl_entries_to_dicts(acl) if acl else None

        op: dict[str, Any] = {
            "create_node": {
                "type_id": node_type.type_id,
                "data": payload,
            }
        }

        if acl_dicts:
            op["create_node"]["acl"] = acl_dicts
        if as_:
            op["create_node"]["as"] = as_
        if fanout_to:
            op["create_node"]["fanout_to"] = fanout_to

        # Storage routing — exactly one way to pick a storage mode.
        # Defaults to Tenant() so omitting ``storage=`` is identical
        # to the previous default behavior.
        storage = storage or Tenant()
        if isinstance(storage, Mailbox):
            op["create_node"]["storage_mode"] = "USER_MAILBOX"
            op["create_node"]["target_user_id"] = storage.user_id
        elif isinstance(storage, Public):
            op["create_node"]["storage_mode"] = "PUBLIC"
        elif isinstance(storage, Tenant):
            pass  # default — no extra routing fields
        else:
            raise TypeError(
                f"storage must be Tenant(), Mailbox(user_id=...), or Public(); "
                f"got {type(storage).__name__}"
            )

        self._operations.append(op)
        return self

    def update(
        self,
        node_id: str,
        msg: Any,
    ) -> Plan:
        """Update a node from a proto message.

        Per the 2026-04-14 SDK v0.3 decision, ``update`` takes only
        the node id and a proto message. The type is read from the
        message's ``(entdb.node)`` option; the patch is exactly the
        set of fields explicitly set on the message (via
        ``ListFields()``). Unset fields are not included, so

            plan.update(node_id, Product(price_cents=1499))

        sends ``{"price_cents": 1499}`` as the patch and leaves every
        other field untouched.

        Args:
            node_id: Id of the node to update.
            msg: A proto message instance whose descriptor carries an
                ``(entdb.node)`` option.

        Returns:
            Self for chaining.
        """
        self._ensure_not_committed()

        if not _is_proto_message(msg):
            raise TypeError(
                "Plan.update requires a proto message instance — "
                f"got {type(msg).__name__}. Pass an instance like "
                "schema_pb2.Product(price_cents=1499)."
            )

        type_id = _node_type_id_from_descriptor(msg.DESCRIPTOR)
        patch = _proto_payload_from_set_fields(msg)

        op: dict[str, Any] = {
            "update_node": {
                "type_id": type_id,
                "id": node_id,
                "patch": patch,
            }
        }

        self._operations.append(op)
        return self

    def delete(
        self,
        node_type: Any,
        node_id: str,
    ) -> Plan:
        """Delete a node.

        Per the 2026-04-14 SDK v0.3 decision, ``delete`` requires a
        type witness — pass the proto message *class* (not an
        instance) so the SDK can resolve ``type_id`` without
        guessing.

        Args:
            node_type: The proto message class (e.g.
                ``schema_pb2.Product``) whose descriptor carries the
                ``(entdb.node)`` option.
            node_id: Id of the node to delete.

        Returns:
            Self for chaining.
        """
        self._ensure_not_committed()

        if not _is_proto_message_class(node_type):
            raise TypeError(
                "Plan.delete requires a proto message class as the type "
                f"witness — got {type(node_type).__name__}. Pass the class "
                "itself, e.g. schema_pb2.Product."
            )

        type_id = _node_type_id_from_descriptor(node_type.DESCRIPTOR)
        self._operations.append(
            {
                "delete_node": {
                    "type_id": type_id,
                    "id": node_id,
                }
            }
        )
        return self

    def edge_create(
        self,
        edge_type: Any,
        from_id: str | dict[str, Any],
        to_id: str | dict[str, Any],
        *,
        props: dict[str, Any] | None = None,
    ) -> Plan:
        """Create an edge.

        Per the 2026-04-14 SDK v0.3 decision, ``edge_create`` takes
        the proto message *class* (not an instance) marked with
        ``(entdb.edge)``, then the source and target node ids.

        Args:
            edge_type: Proto class with an ``(entdb.edge)`` annotation.
            from_id: Source node id (or alias ``"$alias"``).
            to_id: Target node id.
            props: Optional edge properties.

        Returns:
            Self for chaining.
        """
        self._ensure_not_committed()

        if not _is_proto_message_class(edge_type):
            raise TypeError(
                f"Plan.edge_create requires a proto message class — got {type(edge_type).__name__}."
            )

        edge_id = _node_type_id_from_descriptor(edge_type.DESCRIPTOR, kind="edge")

        op: dict[str, Any] = {
            "create_edge": {
                "edge_id": edge_id,
                "from": self._convert_ref(from_id),
                "to": self._convert_ref(to_id),
            }
        }

        if props:
            op["create_edge"]["props"] = props

        self._operations.append(op)
        return self

    def edge_delete(
        self,
        edge_type: Any,
        from_id: str | dict[str, Any],
        to_id: str | dict[str, Any],
    ) -> Plan:
        """Delete an edge.

        Args:
            edge_type: Proto class with an ``(entdb.edge)`` annotation.
            from_id: Source node id.
            to_id: Target node id.

        Returns:
            Self for chaining.
        """
        self._ensure_not_committed()

        if not _is_proto_message_class(edge_type):
            raise TypeError(
                f"Plan.edge_delete requires a proto message class — got {type(edge_type).__name__}."
            )

        edge_id = _node_type_id_from_descriptor(edge_type.DESCRIPTOR, kind="edge")
        self._operations.append(
            {
                "delete_edge": {
                    "edge_id": edge_id,
                    "from": self._convert_ref(from_id),
                    "to": self._convert_ref(to_id),
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
        schema_module: Any | None = None,
        schema_fingerprint: str | None = None,
    ) -> None:
        """Initialize client.

        Args:
            address: Server address (host:port or just host)
            secure: Whether to use TLS
            api_key: Optional API key for authentication
            max_retries: Maximum number of retries for transient gRPC failures
            registry: Optional schema registry
            schema_module: Optional generated schema module (e.g. the module
                produced by ``entdb codegen``). If provided and it exposes a
                ``SCHEMA_FINGERPRINT`` attribute, that value will be sent on
                every write for server-side version checks.
            schema_fingerprint: Optional explicit schema fingerprint. Takes
                precedence over ``schema_module`` and the registry.
        """
        # Parse address
        if ":" in address:
            host, port_str = address.rsplit(":", 1)
            port = int(port_str)
        else:
            host = address
            port = 50051  # Default gRPC port

        self.registry = registry or get_registry()
        self._grpc = GrpcClient(
            host=host,
            port=port,
            secure=secure,
            api_key=api_key,
            max_retries=max_retries,
            registry=self.registry,
        )
        self._connected = False
        self._last_offsets: dict[str, str] = {}  # tenant_id -> stream_position
        self._schema_fingerprint: str | None = schema_fingerprint
        if self._schema_fingerprint is None and schema_module is not None:
            self._schema_fingerprint = getattr(schema_module, "SCHEMA_FINGERPRINT", None)

    def _resolve_schema_fingerprint(self) -> str:
        """Resolve the schema fingerprint to send with write requests.

        Precedence:
        1. Explicit ``schema_fingerprint`` passed to ``DbClient(...)``
        2. ``SCHEMA_FINGERPRINT`` from a generated ``schema_module``
        3. ``registry.fingerprint`` if the registry has been frozen
        4. Empty string (backward-compat, server skips the check)
        """
        explicit = getattr(self, "_schema_fingerprint", None)
        if explicit:
            return explicit
        registry = getattr(self, "registry", None)
        reg_fp = getattr(registry, "fingerprint", None) if registry is not None else None
        return reg_fp or ""

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

        Example::

            plan = db.atomic("tenant_1", "user:42")
            plan.create(schema_pb2.Task(title="New Task"))
            await plan.commit()
        """
        self._ensure_connected()
        return Plan(self, tenant_id, actor, idempotency_key, trace_id=trace_id)

    def tenant(self, tenant_id: str) -> TenantScope:
        """Create a tenant-scoped handle.

        Returns a ``TenantScope`` that captures this client and tenant_id.
        Call ``.actor(actor)`` on the result to get an ``ActorScope`` with
        convenient read/write methods that no longer require tenant_id/actor.

        Args:
            tenant_id: Tenant identifier

        Returns:
            TenantScope bound to this client and tenant

        Example:
            >>> alice = db.tenant("alice").actor("user:bob")
            >>> task = await alice.get(Task, "t1")
        """
        self._ensure_connected()
        return TenantScope(self, tenant_id)

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

        return await self._grpc.get_node(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            node_id=node_id,
            after_offset=resolved_offset,
            wait_timeout_ms=30000 if resolved_offset else 0,
            trace_id=trace_id,
            timeout=timeout,
        )

    async def get_by_key(
        self,
        key: UniqueKey[Any],
        value: Any,
        tenant_id: str,
        actor: str,
        *,
        after_offset: str | None = None,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> Node | None:
        """Resolve a node via a typed unique-key token.

        Per the 2026-04-14 SDK v0.3 decision the only way to look up
        a node by a unique field is via a :class:`UniqueKey` token
        emitted by the ``protoc-gen-entdb-keys`` codegen plugin.
        Stringly-typed lookups (passing a field name) are gone — see
        ``docs/decisions/sdk_api.md`` for the rationale.

        Args:
            key: A :class:`UniqueKey` token from the generated
                ``<schema>_entdb.py`` sidecar.
            value: The scalar value to match. Type-checked against
                ``UniqueKey[T]`` at the static-analysis layer.
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())
        after_int = 0
        if after_offset:
            try:
                after_int = int(after_offset)
            except (TypeError, ValueError):
                after_int = 0

        if not isinstance(key, UniqueKey):
            raise TypeError(
                "get_by_key requires a UniqueKey token from the generated "
                f"<schema>_entdb.py sidecar — got {type(key).__name__}."
            )

        return await self._grpc.get_node_by_key(
            tenant_id=tenant_id,
            actor=actor,
            type_id=key.type_id,
            field_id=key.field_id,
            value=value,
            after_offset=after_int,
            trace_id=trace_id,
            timeout=timeout,
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

        return await self._grpc.get_nodes(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            node_ids=node_ids,
            after_offset=resolved_offset,
            wait_timeout_ms=30000 if resolved_offset else 0,
            trace_id=trace_id,
            timeout=timeout,
        )

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

        nodes, _ = await self._grpc.query_nodes(
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

        return nodes

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
        edges, _ = await self._grpc.get_edges_from(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            edge_type_id=edge_type_id,
            trace_id=trace_id,
            timeout=timeout,
        )

        return edges

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
        edges, _ = await self._grpc.get_edges_to(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            edge_type_id=edge_type_id,
            trace_id=trace_id,
            timeout=timeout,
        )

        return edges

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

    async def search_nodes(
        self,
        node_type: NodeTypeDef,
        tenant_id: str,
        actor: str,
        query: str,
        *,
        limit: int = 50,
        offset: int = 0,
        trace_id: str | None = None,
        timeout: float | None = None,
    ) -> list[Node]:
        """Full-text search across searchable fields of a node type.

        Args:
            node_type: Node type to search
            tenant_id: Tenant identifier
            actor: Actor making request
            query: FTS5 match expression
            limit: Maximum results
            offset: Pagination offset
            trace_id: Optional trace ID for distributed tracing
            timeout: Per-call timeout in seconds

        Returns:
            List of matching nodes ordered by relevance
        """
        self._ensure_connected()
        trace_id = trace_id or str(uuid.uuid4())

        return await self._grpc.search_nodes(
            tenant_id=tenant_id,
            actor=actor,
            type_id=node_type.type_id,
            query=query,
            limit=limit,
            offset=offset,
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
            schema_fingerprint=self._resolve_schema_fingerprint(),
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

        nodes, _ = await self._grpc.get_connected_nodes(
            tenant_id=tenant_id,
            actor=actor,
            node_id=node_id,
            edge_type_id=edge_type.edge_id,
            limit=limit,
            offset=offset,
            trace_id=trace_id,
            timeout=timeout,
        )

        return nodes

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

        nodes, _ = await self._grpc.list_shared_with_me(
            tenant_id=tenant_id,
            actor=actor,
            limit=limit,
            offset=offset,
            trace_id=trace_id,
            timeout=timeout,
        )

        return nodes

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
        region: str | None = None,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Create a new tenant in the global registry.

        Args:
            tenant_id: Unique tenant identifier
            name: Tenant display name
            actor: Actor performing the operation
            region: Optional geographic region pin (e.g. ``"us-east-1"``,
                ``"eu-west-1"``). When ``None`` the server defaults
                the tenant to its own served region. The resolved
                region is returned on the ``tenant`` dict.
            timeout: Per-call timeout in seconds

        Returns:
            Dict with success, tenant, and error fields
        """
        self._ensure_connected()
        return await self._grpc.create_tenant(
            actor=actor,
            tenant_id=tenant_id,
            name=name,
            region=region,
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

    # --- GDPR operations (Issue #103, ADR-004) ---

    async def delete_user(
        self,
        user_id: str,
        *,
        actor: str = "",
        grace_days: int = 30,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Right to erasure (GDPR Article 17).

        Queues the user for deletion after a grace period during which
        the user may cancel the request. After the grace period expires,
        a background worker runs the per-type ``data_policy`` rules:

            - PERSONAL/EPHEMERAL nodes are deleted
            - BUSINESS/FINANCIAL/AUDIT/HEALTHCARE nodes are anonymized
            - Personal tenants (sole-owner) are dropped entirely

        Args:
            user_id: User to delete
            actor: Actor performing the request (defaults to ``user:<user_id>``)
            grace_days: Grace period in days (defaults to 30)
            timeout: Per-call timeout in seconds

        Returns:
            Dict with ``success``, ``requested_at``, ``execute_at``,
            ``status``, and ``error``.
        """
        self._ensure_connected()
        return await self._grpc.delete_user(
            user_id=user_id,
            actor=actor,
            grace_days=grace_days,
            timeout=timeout,
        )

    async def cancel_user_deletion(
        self,
        user_id: str,
        *,
        actor: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Cancel a pending user deletion during the grace period."""
        self._ensure_connected()
        return await self._grpc.cancel_user_deletion(
            user_id=user_id,
            actor=actor,
            timeout=timeout,
        )

    async def export_user_data(
        self,
        user_id: str,
        *,
        actor: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Right to access / portability (GDPR Articles 15, 20)."""
        self._ensure_connected()
        return await self._grpc.export_user_data(
            user_id=user_id,
            actor=actor,
            timeout=timeout,
        )

    async def freeze_user(
        self,
        user_id: str,
        *,
        enabled: bool = True,
        actor: str = "",
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Right to restrict processing (GDPR Article 18)."""
        self._ensure_connected()
        return await self._grpc.freeze_user(
            user_id=user_id,
            enabled=enabled,
            actor=actor,
            timeout=timeout,
        )
