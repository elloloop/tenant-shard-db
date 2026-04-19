"""
gRPC server implementation for EntDB.

This module provides the gRPC API server that handles all client requests.
It implements the EntDBService defined in entdb.proto.

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

import json
import logging
import time
import uuid
from typing import TYPE_CHECKING, Any

import grpc
from google.protobuf import json_format
from google.protobuf.struct_pb2 import Struct
from grpc import aio as grpc_aio

from ..metrics import record_grpc_request
from ..schema.field_id_translation import (
    id_to_name_keys,
    name_to_id_keys,
    translate_payload_json_to_names,
)
from ..sharding import ShardingConfig
from .generated import (
    AclEntry,
    ArchiveTenantRequest,
    ArchiveTenantResponse,
    CancelUserDeletionRequest,
    CancelUserDeletionResponse,
    ChangeMemberRoleRequest,
    ChangeMemberRoleResponse,
    CreateTenantRequest,
    CreateTenantResponse,
    CreateUserRequest,
    CreateUserResponse,
    DelegateAccessRequest,
    DelegateAccessResponse,
    DeleteUserRequest,
    DeleteUserResponse,
    Edge,
    EntDBServiceServicer,
    # Request/Response types
    ExecuteAtomicRequest,
    ExecuteAtomicResponse,
    ExportUserDataRequest,
    ExportUserDataResponse,
    FreezeUserRequest,
    FreezeUserResponse,
    # ACL v2
    GetConnectedNodesRequest,
    GetConnectedNodesResponse,
    GetEdgesRequest,
    GetEdgesResponse,
    GetMailboxRequest,
    GetMailboxResponse,
    GetNodeByKeyRequest,
    GetNodeByKeyResponse,
    GetNodeRequest,
    GetNodeResponse,
    GetNodesRequest,
    GetNodesResponse,
    GetReceiptStatusRequest,
    GetReceiptStatusResponse,
    GetSchemaRequest,
    GetSchemaResponse,
    GetTenantMembersRequest,
    GetTenantMembersResponse,
    GetTenantQuotaRequest,
    GetTenantQuotaResponse,
    GetTenantRequest,
    GetTenantResponse,
    GetUserRequest,
    GetUserResponse,
    GetUserTenantsRequest,
    GetUserTenantsResponse,
    GroupMemberRequest,
    GroupMemberResponse,
    HealthRequest,
    HealthResponse,
    LegalHoldRequest,
    LegalHoldResponse,
    ListMailboxUsersRequest,
    ListMailboxUsersResponse,
    ListSharedWithMeRequest,
    ListSharedWithMeResponse,
    ListTenantsRequest,
    ListTenantsResponse,
    ListUsersRequest,
    ListUsersResponse,
    Node,
    QueryNodesRequest,
    QueryNodesResponse,
    # Data types
    Receipt,
    ReceiptStatus,
    RevokeAccessRequest,
    RevokeAccessResponse,
    RevokeAllUserAccessRequest,
    RevokeAllUserAccessResponse,
    SearchMailboxRequest,
    SearchMailboxResponse,
    SearchNodesRequest,
    SearchNodesResponse,
    ShareNodeRequest,
    ShareNodeResponse,
    # Tenant registry
    TenantDetail,
    TenantInfo,
    # Tenant membership
    TenantMemberInfo,
    TenantMemberRequest,
    TenantMemberResponse,
    TransferOwnershipRequest,
    TransferOwnershipResponse,
    # Admin operations (Issue #90)
    TransferUserContentRequest,
    TransferUserContentResponse,
    UpdateUserRequest,
    UpdateUserResponse,
    # User registry
    UserInfo,
    WaitForOffsetRequest,
    WaitForOffsetResponse,
    add_EntDBServiceServicer_to_server,
)

if TYPE_CHECKING:
    pass

logger = logging.getLogger(__name__)


def _struct_to_dict(s):
    """Convert protobuf Struct to Python dict."""
    return json_format.MessageToDict(s) if s and s.fields else {}


def _dict_to_struct(d):
    """Convert Python dict to protobuf Struct."""
    s = Struct()
    if d:
        s.update(d)
    return s


def _acl_list_to_proto(acl_list):
    """Convert list of dicts to repeated AclEntry."""
    return [
        AclEntry(principal=e.get("principal", ""), permission=e.get("permission", ""))
        for e in acl_list
    ]


def _acl_proto_to_list(acl_entries):
    """Convert repeated AclEntry to list of dicts."""
    return [{"principal": e.principal, "permission": e.permission} for e in acl_entries]


class EntDBServicer(EntDBServiceServicer):
    """gRPC service implementation for EntDB.

    This class implements the gRPC service methods defined in the proto.
    It coordinates between the WAL stream (for writes) and SQLite stores
    (for reads).

    Attributes:
        wal: WAL stream for durably writing events
        canonical_store: Tenant SQLite store for reads
        schema_registry: Schema registry for validation
        global_store: GlobalStore for cross-tenant shared_index
    """

    def __init__(
        self,
        wal: Any,
        canonical_store: Any,
        schema_registry: Any,
        topic: str = "entdb-wal",
        sharding: ShardingConfig | None = None,
        global_store: Any | None = None,
        capability_registry: Any | None = None,
    ) -> None:
        """Initialize the gRPC servicer.

        Args:
            wal: WAL stream instance
            canonical_store: CanonicalStore instance
            schema_registry: SchemaRegistry instance
            topic: WAL topic name
            sharding: Sharding configuration for multi-node mode
            global_store: GlobalStore instance for user registry operations
            capability_registry: Optional pre-built
                ``CapabilityRegistry``. When ``None``, a default
                (core-caps-only) registry is built from
                ``schema_registry`` on first use.
        """
        self.wal = wal
        self.canonical_store = canonical_store
        self.schema_registry = schema_registry
        self.topic = topic
        self._sharding = sharding
        self.global_store = global_store
        self._capability_registry = capability_registry

    @property
    def capability_registry(self) -> Any:
        """Lazy accessor for the typed-capability registry.

        Built from ``self.schema_registry`` on first access so that
        servers which construct ``EntDBServicer`` without explicitly
        wiring a registry still get the default core-capability
        behaviour. Tests and the production entrypoint can inject a
        pre-built registry via the constructor.
        """
        if self._capability_registry is None:
            from ..auth.capability_registry import build_registry_from_schema

            self._capability_registry = build_registry_from_schema(self.schema_registry)
        return self._capability_registry

    async def _check_capability(
        self,
        tenant_id: str,
        actor: Any,
        node_id: str,
        type_id: int,
        op_name: str,
        context: Any | None = None,
        *,
        field: str | None = None,
        field_value: str | None = None,
        child_type: str | None = None,
    ) -> None:
        """Raise PermissionError if ``actor`` lacks the required capability.

        Looks up the required capability via the registry's
        ``required_for_op``, fetches the actor's ACL grants from the
        canonical store, and runs each grant through
        ``CapabilityRegistry.check_grant`` honouring the implication
        hierarchy.

        System actors and tenant owners short-circuit the check.
        """
        if self._is_admin_or_system(actor if isinstance(actor, str) else str(actor)):
            return
        registry = self.capability_registry
        required_core, required_ext = registry.required_for_op(
            type_id,
            op_name,
            field=field,
            field_value=field_value,
            child_type=child_type,
        )
        if required_core is None and required_ext is None:
            return
        grants = await self.canonical_store.get_acl_grants(
            tenant_id,
            node_id,
            actor,
            include_cross_tenant=True,
        )
        for grant in grants:
            # Deny rows with no positive caps block everything.
            if (
                grant.get("permission") == "deny"
                and not grant.get("core_cap_ids")
                and not grant.get("ext_cap_ids")
            ):
                raise PermissionError(f"Actor denied explicit access on node {node_id}")
            if registry.check_grant(
                grant.get("core_cap_ids", []),
                grant.get("ext_cap_ids", []),
                required_core,
                required_ext,
                grant.get("type_id") or type_id,
            ):
                return
        raise PermissionError(
            f"Actor {actor} lacks capability "
            f"{required_core if required_core is not None else f'ext:{required_ext}'} "
            f"on node {node_id}"
        )

    async def _check_tenant(self, tenant_id: str, context) -> None:
        """Reject requests for tenants not owned by this node."""
        if self._sharding and not self._sharding.is_mine(tenant_id):
            owner = self._sharding.get_owner(tenant_id)
            hint = f" (try node {owner})" if owner else ""
            await context.abort(
                grpc.StatusCode.UNAVAILABLE,
                f"Tenant '{tenant_id}' is not served by this node{hint}",
            )

    def _actor_user_id(self, actor: str) -> str:
        """Extract user_id from actor string like 'user:ID'."""
        if actor.startswith("user:"):
            return actor[5:]
        return actor

    async def _check_tenant_access(
        self,
        tenant_id: str,
        actor: str,
        context,
        require_write: bool = False,
        op_kind: str = "read",
    ) -> str | None:
        """Check tenant existence, status, and actor role.

        Enforces:
            - tenant_registry.status:
                * 'active'      -> all operations allowed
                * 'archived'    -> reads ok, writes rejected (FAILED_PRECONDITION)
                * 'legal_hold'  -> reads + creates ok, deletes rejected
                                   (FAILED_PRECONDITION)
                * 'deleted'     -> all rejected (NOT_FOUND)
            - tenant_members.role:
                * owner / admin / member -> read + write
                * viewer / guest         -> read only
                * non-member             -> rejected (PERMISSION_DENIED)

        System actors (``system:*`` and ``__system__``) bypass all checks.

        If ``self.global_store`` is None, the check is skipped (backward
        compatible) and ``"system"`` is returned.

        Args:
            tenant_id:    Tenant identifier to check.
            actor:        Actor string (``user:<id>``, ``system:*`` etc).
            context:      gRPC servicer context for ``abort()``.
            require_write: If True, the caller is requesting a write op.
            op_kind:      One of ``"read"``, ``"create"``, ``"write"``,
                          ``"delete"``. Used for legal_hold semantics.
                          Defaults to ``"read"``.

        Returns:
            The actor's role string (or ``"system"`` for system actors,
            or ``"system"`` when GlobalStore is not configured). Returns
            ``None`` only when the call has been aborted (the abort raises,
            so callers will not actually receive ``None`` in practice).
        """
        if self.global_store is None:
            return "system"

        # System actors bypass everything.
        if actor.startswith("system:") or actor == "__system__":
            return "system"

        # Tenant existence + status check.
        tenant = await self.global_store.get_tenant(tenant_id)
        if tenant is None:
            await context.abort(
                grpc.StatusCode.NOT_FOUND,
                f"Tenant '{tenant_id}' does not exist",
            )
            return None

        status = tenant.get("status", "active")
        if status == "deleted":
            await context.abort(
                grpc.StatusCode.NOT_FOUND,
                f"Tenant '{tenant_id}' is deleted",
            )
            return None
        if status == "archived" and (require_write or op_kind != "read"):
            await context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                f"Tenant '{tenant_id}' is archived; writes are not allowed",
            )
            return None
        if status == "legal_hold" and op_kind == "delete":
            await context.abort(
                grpc.StatusCode.FAILED_PRECONDITION,
                f"Tenant '{tenant_id}' is on legal hold; deletes are not allowed",
            )
            return None

        # Role check.
        actor_uid = self._actor_user_id(actor)
        role = await self._get_member_role(tenant_id, actor_uid)
        if role is None:
            await context.abort(
                grpc.StatusCode.PERMISSION_DENIED,
                f"Actor '{actor}' is not a member of tenant '{tenant_id}'",
            )
            return None

        if require_write and role in ("viewer", "guest"):
            await context.abort(
                grpc.StatusCode.PERMISSION_DENIED,
                f"Role '{role}' does not have write access",
            )
            return None

        # Frozen / pending-deletion user check (GDPR Article 18: restrict
        # processing). A frozen user may read but cannot make new writes.
        if require_write:
            user = await self.global_store.get_user(actor_uid)
            if user is not None and user.get("status") in ("frozen", "pending_deletion"):
                await context.abort(
                    grpc.StatusCode.PERMISSION_DENIED,
                    f"User '{actor_uid}' is {user.get('status')} and cannot write",
                )
                return None

        return role

    async def _check_cross_tenant_read(
        self,
        tenant_id: str,
        actor: str,
        context,
    ) -> str:
        """Check read access, allowing cross-tenant actors with node_access.

        If no global_store is configured, returns "local" (backward compat).
        If the actor is a tenant member, returns "member".
        If the actor is NOT a member but has at least one node_access
        entry in the tenant, returns "cross_tenant".
        Otherwise aborts with PERMISSION_DENIED.

        Returns:
            "local" (no global_store), "member", or "cross_tenant".
        """
        if not self.global_store:
            return "local"

        # System actors bypass all checks
        if actor.startswith("system:") or actor == "__system__":
            return "member"

        # Check membership
        actor_uid = self._actor_user_id(actor)
        is_member = await self.global_store.is_member(tenant_id, actor_uid)
        if is_member:
            return "member"

        # Not a member — check for cross-tenant node_access
        actor_ids = await self.canonical_store.resolve_actor_groups(
            tenant_id,
            actor,
        )
        has_access = await self.canonical_store.has_node_access(
            tenant_id,
            actor_ids,
        )
        if has_access:
            return "cross_tenant"

        await context.abort(
            grpc.StatusCode.PERMISSION_DENIED,
            "Actor is not a member of this tenant",
        )

    async def ExecuteAtomic(
        self,
        request: ExecuteAtomicRequest,
        context: grpc_aio.ServicerContext,
    ) -> ExecuteAtomicResponse:
        """Execute an atomic transaction."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)

            # Extract context
            ctx = request.context
            if not ctx.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not ctx.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.operations:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "operations list is empty")

            # Generate idempotency key if not provided
            idempotency_key = request.idempotency_key or str(uuid.uuid4())

            # Validate schema fingerprint if provided
            if request.schema_fingerprint and self.schema_registry.fingerprint:
                if request.schema_fingerprint != self.schema_registry.fingerprint:
                    record_grpc_request("ExecuteAtomic", "error", time.perf_counter() - start)
                    return ExecuteAtomicResponse(
                        success=False,
                        error=f"Schema mismatch: server has {self.schema_registry.fingerprint}",
                        error_code="SCHEMA_MISMATCH",
                    )

            # Convert operations to internal format. Unique-key
            # enforcement is authoritative at apply time via unique
            # expression indexes (2026-04-14 SDK v0.3 decision). We
            # still run a best-effort fast-fail pre-check so that the
            # common "obvious duplicate" case gets an
            # ``ALREADY_EXISTS`` without a WAL round trip; races are
            # caught by the index itself at apply time.
            ops = self._convert_operations(request.operations)

            for op in ops:
                if op.get("op") != "create_node":
                    continue
                type_id_int = int(op.get("type_id", 0) or 0)
                if not type_id_int:
                    continue
                get_unique = getattr(self.schema_registry, "get_unique_field_ids", None)
                if not callable(get_unique):
                    continue
                try:
                    unique_fids = get_unique(type_id_int)
                except Exception:
                    unique_fids = []
                # Normalise to a concrete list — MagicMock returns a
                # Mock object when called which fails ``if not`` below.
                if not isinstance(unique_fids, (list, tuple)) or not unique_fids:
                    continue
                data = op.get("data") or {}
                for fid in unique_fids:
                    value = data.get(str(fid))
                    if value is None:
                        continue
                    existing = await self.canonical_store.get_node_by_key(
                        ctx.tenant_id, type_id_int, fid, value
                    )
                    if existing is not None:
                        record_grpc_request("ExecuteAtomic", "error", time.perf_counter() - start)
                        await context.abort(
                            grpc.StatusCode.ALREADY_EXISTS,
                            f"Unique constraint violation: type_id={type_id_int} "
                            f"field_id={fid} value={value!r} already exists",
                        )

            # Tenant role + status enforcement.
            # ExecuteAtomic is always a write. The status check is finer
            # grained: legal_hold rejects deletes (delete_node, delete_edge)
            # but still permits creates/updates.
            has_delete = any(op.get("op") in ("delete_node", "delete_edge") for op in ops)
            await self._check_tenant_access(
                tenant_id=ctx.tenant_id,
                actor=ctx.actor,
                context=context,
                require_write=True,
                op_kind="delete" if has_delete else "write",
            )

            # Assign IDs to create_node ops that don't have one
            created_node_ids = []
            for op in ops:
                if op.get("op") == "create_node":
                    if not op.get("id"):
                        op["id"] = str(uuid.uuid4())
                    created_node_ids.append(op["id"])

            # Build transaction event
            event = {
                "tenant_id": ctx.tenant_id,
                "actor": ctx.actor,
                "idempotency_key": idempotency_key,
                "schema_fingerprint": self.schema_registry.fingerprint,
                "ts_ms": int(time.time() * 1000),
                "ops": ops,
            }

            try:
                # Append to WAL
                event_bytes = json.dumps(event).encode("utf-8")
                stream_pos = await self.wal.append(
                    self.topic,
                    ctx.tenant_id,  # Partition key
                    event_bytes,
                )

                receipt = Receipt(
                    tenant_id=ctx.tenant_id,
                    idempotency_key=idempotency_key,
                    stream_position=str(stream_pos) if stream_pos else "",
                )

                # Wait for application if requested (event-driven via offset)
                applied_status = ReceiptStatus.RECEIPT_STATUS_PENDING
                if request.wait_applied and stream_pos is not None:
                    timeout = request.wait_timeout_ms / 1000.0 if request.wait_timeout_ms else 30.0
                    applied = await self._wait_for_offset(ctx.tenant_id, str(stream_pos), timeout)
                    if applied:
                        applied_status = ReceiptStatus.RECEIPT_STATUS_APPLIED

                record_grpc_request("ExecuteAtomic", "ok", time.perf_counter() - start)
                return ExecuteAtomicResponse(
                    success=True,
                    receipt=receipt,
                    created_node_ids=created_node_ids,
                    applied_status=applied_status,
                )

            except Exception as e:
                logger.error(f"ExecuteAtomic failed: {e}", exc_info=True)
                record_grpc_request("ExecuteAtomic", "error", time.perf_counter() - start)
                return ExecuteAtomicResponse(
                    success=False,
                    error=str(e),
                    error_code="INTERNAL",
                )
        except Exception:
            record_grpc_request("ExecuteAtomic", "error", time.perf_counter() - start)
            raise

    def _convert_operations(self, operations: list) -> list[dict[str, Any]]:
        """Convert protobuf operations to internal format.

        Ingress field-id translation (issue #104): any ``create_node.data``
        or ``update_node.patch`` payload is translated from name-keyed to
        id-keyed using the schema registry so that storage is keyed by the
        stable numeric ``field_id``.  Unknown field names are dropped.
        """
        result = []

        for op in operations:
            op_type = op.WhichOneof("op")

            if op_type == "create_node":
                create = op.create_node
                data = _struct_to_dict(create.data)
                data = name_to_id_keys(data, create.type_id, self.schema_registry)
                internal_op: dict[str, Any] = {
                    "op": "create_node",
                    "type_id": create.type_id,
                    "data": data,
                }
                if create.id:
                    internal_op["id"] = create.id
                if create.acl:
                    internal_op["acl"] = _acl_proto_to_list(create.acl)
                as_val = getattr(create, "as", "")
                if as_val:
                    internal_op["as"] = as_val
                if create.fanout_to:
                    internal_op["fanout_to"] = list(create.fanout_to)
                # Storage routing (2026-04-13 storage decision). The
                # proto enum values map to the internal storage_mode
                # string the Applier understands. Default / unspecified
                # = TENANT.
                _storage_mode_names = {
                    0: "TENANT",
                    1: "USER_MAILBOX",
                    2: "PUBLIC",
                }
                sm_value = int(getattr(create, "storage_mode", 0) or 0)
                sm_name = _storage_mode_names.get(sm_value, "TENANT")
                if sm_name != "TENANT":
                    internal_op["storage_mode"] = sm_name
                tgt_user = getattr(create, "target_user_id", "") or ""
                if tgt_user:
                    internal_op["target_user_id"] = tgt_user
                result.append(internal_op)

            elif op_type == "update_node":
                update = op.update_node
                patch = _struct_to_dict(update.patch)
                patch = name_to_id_keys(patch, update.type_id, self.schema_registry)
                update_internal: dict[str, Any] = {
                    "op": "update_node",
                    "type_id": update.type_id,
                    "id": update.id,
                    "patch": patch,
                }
                result.append(update_internal)

            elif op_type == "delete_node":
                delete = op.delete_node
                result.append(
                    {
                        "op": "delete_node",
                        "type_id": delete.type_id,
                        "id": delete.id,
                    }
                )

            elif op_type == "create_edge":
                create = op.create_edge
                result.append(
                    {
                        "op": "create_edge",
                        "edge_id": create.edge_id,
                        "from": self._convert_node_ref(getattr(create, "from")),
                        "to": self._convert_node_ref(create.to),
                        "props": _struct_to_dict(create.props),
                    }
                )

            elif op_type == "delete_edge":
                delete = op.delete_edge
                result.append(
                    {
                        "op": "delete_edge",
                        "edge_id": delete.edge_id,
                        "from": self._convert_node_ref(getattr(delete, "from")),
                        "to": self._convert_node_ref(delete.to),
                    }
                )

        return result

    def _payload_id_to_name_dict(self, n: Any) -> dict[str, Any]:
        """Egress helper: return the stored payload keyed by field name.

        Reads ``node.payload`` (id-keyed on disk) and uses the schema
        registry to remap keys.  See :func:`id_to_name_keys`.
        """
        return id_to_name_keys(n.payload, n.type_id, self.schema_registry)

    def _payload_id_to_name_json(self, n: Any) -> str:
        """Egress helper: return a JSON string payload keyed by field name."""
        return translate_payload_json_to_names(n.payload_json, n.type_id, self.schema_registry)

    def _convert_node_ref(self, ref: Any) -> Any:
        """Convert protobuf node reference to internal format."""
        ref_type = ref.WhichOneof("ref")
        if ref_type == "id":  # noqa: SIM116
            return ref.id
        elif ref_type == "alias_ref":
            return {"ref": ref.alias_ref}
        elif ref_type == "typed":
            return {"type_id": ref.typed.type_id, "id": ref.typed.id}
        return None

    async def _wait_for_offset(
        self,
        tenant_id: str,
        stream_position: str,
        timeout: float,
    ) -> bool:
        """Wait for a stream position to be applied (event-driven, no polling)."""
        return await self.canonical_store.wait_for_offset(tenant_id, stream_position, timeout)

    async def GetReceiptStatus(
        self,
        request: GetReceiptStatusRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetReceiptStatusResponse:
        """Get the status of a transaction receipt."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            applied = await self.canonical_store.check_idempotency(
                request.context.tenant_id,
                request.idempotency_key,
            )
            status = (
                ReceiptStatus.RECEIPT_STATUS_APPLIED
                if applied
                else ReceiptStatus.RECEIPT_STATUS_PENDING
            )
            record_grpc_request("GetReceiptStatus", "ok", time.perf_counter() - start)
            return GetReceiptStatusResponse(status=status)
        except Exception as e:
            record_grpc_request("GetReceiptStatus", "error", time.perf_counter() - start)
            return GetReceiptStatusResponse(
                status=ReceiptStatus.RECEIPT_STATUS_UNKNOWN,
                error=str(e),
            )

    async def WaitForOffset(
        self,
        request: WaitForOffsetRequest,
        context: grpc_aio.ServicerContext,
    ) -> WaitForOffsetResponse:
        """Wait for a stream position to be applied."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            timeout = request.timeout_ms / 1000.0 if request.timeout_ms else 30.0
            reached = await self._wait_for_offset(
                request.context.tenant_id, request.stream_position, timeout
            )
            current = self.canonical_store.get_applied_offset(request.context.tenant_id) or ""
            record_grpc_request("WaitForOffset", "ok", time.perf_counter() - start)
            return WaitForOffsetResponse(reached=reached, current_position=current)
        except Exception as e:
            record_grpc_request("WaitForOffset", "error", time.perf_counter() - start)
            logger.error(f"WaitForOffset failed: {e}", exc_info=True)
            return WaitForOffsetResponse(reached=False, current_position="")

    async def GetNode(
        self,
        request: GetNodeRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetNodeResponse:
        """Get a node by ID.

        Supports cross-tenant reads: if the actor is not a member of the
        tenant but has an explicit node_access grant for the requested node,
        the read is allowed.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            role = await self._check_cross_tenant_read(
                request.context.tenant_id,
                request.context.actor,
                context,
            )

            # Wait for offset if requested (read-after-write consistency)
            if request.after_offset:
                timeout = request.wait_timeout_ms / 1000.0 if request.wait_timeout_ms else 30.0
                await self._wait_for_offset(
                    request.context.tenant_id, request.after_offset, timeout
                )

            node = await self.canonical_store.get_node(
                request.context.tenant_id,
                request.node_id,
            )
            if not node:
                record_grpc_request("GetNode", "ok", time.perf_counter() - start)
                return GetNodeResponse(found=False)

            # Cross-tenant: verify access to this specific node
            if role == "cross_tenant":
                actor_ids = await self.canonical_store.resolve_actor_groups(
                    request.context.tenant_id,
                    request.context.actor,
                )
                has_access = await self.canonical_store.can_access(
                    request.context.tenant_id,
                    request.node_id,
                    actor_ids,
                )
                if not has_access:
                    await context.abort(
                        grpc.StatusCode.PERMISSION_DENIED,
                        "Actor does not have access to this node",
                    )

            record_grpc_request("GetNode", "ok", time.perf_counter() - start)
            return GetNodeResponse(
                found=True,
                node=Node(
                    tenant_id=node.tenant_id,
                    node_id=node.node_id,
                    type_id=node.type_id,
                    payload=_dict_to_struct(self._payload_id_to_name_dict(node)),
                    created_at=node.created_at,
                    updated_at=node.updated_at,
                    owner_actor=node.owner_actor,
                    acl=_acl_list_to_proto(node.acl),
                ),
            )
        except Exception as e:
            record_grpc_request("GetNode", "error", time.perf_counter() - start)
            logger.error(f"GetNode failed: {e}", exc_info=True)
            return GetNodeResponse(found=False)

    async def GetNodeByKey(
        self,
        request: GetNodeByKeyRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetNodeByKeyResponse:
        """Resolve a node by its declared unique field.

        Implements the 2026-04-14 SDK v0.3 decision. The handler:

        1. Verifies the tenant exists.
        2. Resolves ``(type_id, field_id, value)`` to a node_id via
           the unique expression index on
           ``json_extract(payload_json, '$."<field_id>"')``.
        3. Delegates to the same ``GetNode`` path so ACL, cross-tenant,
           and capability checks run identically — actors without
           ``CORE_CAP_READ`` get ``PERMISSION_DENIED`` instead of a
           silent ``found=False``.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.tenant_id, context)

            if request.after_offset:
                await self._wait_for_offset(request.tenant_id, str(request.after_offset), 30.0)

            # ``request.value`` is a ``google.protobuf.Value`` so it
            # reaches us already typed. ``MessageToDict`` unwraps it
            # to the underlying Python scalar (str/int/float/bool).
            raw_value = (
                json_format.MessageToDict(request.value) if request.HasField("value") else None
            )

            node = await self.canonical_store.get_node_by_key(
                request.tenant_id,
                int(request.type_id),
                int(request.field_id),
                raw_value,
            )
            if node is None:
                record_grpc_request("GetNodeByKey", "ok", time.perf_counter() - start)
                return GetNodeByKeyResponse(found=False)

            # Delegate to GetNode so ACL / cross-tenant / capability
            # checks stay centralised.
            from .generated import RequestContext as _RequestContext

            get_req = GetNodeRequest(
                context=_RequestContext(
                    tenant_id=request.tenant_id,
                    actor=request.actor,
                ),
                type_id=int(request.type_id),
                node_id=node.node_id,
            )
            inner = await self.GetNode(get_req, context)
            record_grpc_request("GetNodeByKey", "ok", time.perf_counter() - start)
            return GetNodeByKeyResponse(found=inner.found, node=inner.node)
        except Exception as e:
            record_grpc_request("GetNodeByKey", "error", time.perf_counter() - start)
            logger.error(f"GetNodeByKey failed: {e}", exc_info=True)
            return GetNodeByKeyResponse(found=False)

    async def SearchNodes(
        self,
        request: SearchNodesRequest,
        context: grpc_aio.ServicerContext,
    ) -> SearchNodesResponse:
        """Full-text search across searchable fields of a node type.

        Uses FTS5 virtual tables (one per node type with searchable
        fields). The query string supports FTS5 match expressions:
        AND, OR, NOT, phrase ("..."), prefix (word*), and column
        filters (col:word).

        Results are ACL-filtered the same way as QueryNodes.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.tenant_id, context)

            # Validate query
            query = request.query.strip()
            if not query:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "query must not be empty")
                return SearchNodesResponse(nodes=[])
            if len(query) > 1000:
                await context.abort(
                    grpc.StatusCode.INVALID_ARGUMENT,
                    "query must be under 1000 characters",
                )
                return SearchNodesResponse(nodes=[])

            type_id = int(request.type_id)
            searchable_fids = self.schema_registry.get_searchable_field_ids(type_id)

            nodes = await self.canonical_store.search_nodes(
                tenant_id=request.tenant_id,
                type_id=type_id,
                query=query,
                searchable_field_ids=searchable_fids,
                limit=request.limit or 50,
                offset=request.offset or 0,
            )

            # ACL filtering: same pattern as QueryNodes
            role = await self._check_cross_tenant_read(
                request.tenant_id,
                request.actor,
                context,
            )
            if role == "cross_tenant":
                actor_ids = await self.canonical_store.resolve_actor_groups(
                    request.tenant_id,
                    request.actor,
                )
                accessible = []
                for n in nodes:
                    if await self.canonical_store.can_access(
                        request.tenant_id,
                        n.node_id,
                        actor_ids,
                    ):
                        accessible.append(n)
                nodes = accessible

            proto_nodes = [
                Node(
                    tenant_id=n.tenant_id,
                    node_id=n.node_id,
                    type_id=n.type_id,
                    payload=_dict_to_struct(self._payload_id_to_name_dict(n)),
                    created_at=n.created_at,
                    updated_at=n.updated_at,
                    owner_actor=n.owner_actor,
                    acl=_acl_list_to_proto(n.acl),
                )
                for n in nodes
            ]

            record_grpc_request("SearchNodes", "ok", time.perf_counter() - start)
            return SearchNodesResponse(nodes=proto_nodes)
        except Exception as e:
            record_grpc_request("SearchNodes", "error", time.perf_counter() - start)
            logger.error(f"SearchNodes failed: {e}", exc_info=True)
            return SearchNodesResponse(nodes=[])

    async def GetNodes(
        self,
        request: GetNodesRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetNodesResponse:
        """Get multiple nodes by IDs.

        Supports cross-tenant reads: nodes the actor does not have
        node_access for are placed in missing_ids.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            role = await self._check_cross_tenant_read(
                request.context.tenant_id,
                request.context.actor,
                context,
            )

            # Wait for offset if requested (read-after-write consistency)
            if request.after_offset:
                timeout = request.wait_timeout_ms / 1000.0 if request.wait_timeout_ms else 30.0
                await self._wait_for_offset(
                    request.context.tenant_id, request.after_offset, timeout
                )

            # Resolve actor groups once for cross-tenant filtering
            actor_ids = None
            if role == "cross_tenant":
                actor_ids = await self.canonical_store.resolve_actor_groups(
                    request.context.tenant_id,
                    request.context.actor,
                )

            nodes = []
            missing_ids = []

            for node_id in request.node_ids:
                node = await self.canonical_store.get_node(
                    request.context.tenant_id,
                    node_id,
                )
                if not node:
                    missing_ids.append(node_id)
                    continue

                # Cross-tenant: verify per-node access
                if actor_ids is not None:
                    has_access = await self.canonical_store.can_access(
                        request.context.tenant_id,
                        node_id,
                        actor_ids,
                    )
                    if not has_access:
                        missing_ids.append(node_id)
                        continue

                nodes.append(
                    Node(
                        tenant_id=node.tenant_id,
                        node_id=node.node_id,
                        type_id=node.type_id,
                        payload=_dict_to_struct(self._payload_id_to_name_dict(node)),
                        created_at=node.created_at,
                        updated_at=node.updated_at,
                        owner_actor=node.owner_actor,
                        acl=_acl_list_to_proto(node.acl),
                    )
                )

            record_grpc_request("GetNodes", "ok", time.perf_counter() - start)
            return GetNodesResponse(nodes=nodes, missing_ids=missing_ids)
        except Exception as e:
            record_grpc_request("GetNodes", "error", time.perf_counter() - start)
            logger.error(f"GetNodes failed: {e}", exc_info=True)
            return GetNodesResponse(nodes=[], missing_ids=list(request.node_ids))

    async def QueryNodes(
        self,
        request: QueryNodesRequest,
        context: grpc_aio.ServicerContext,
    ) -> QueryNodesResponse:
        """Query nodes by type with optional payload filtering.

        Supports cross-tenant reads: if the actor is not a tenant member
        but has node_access, only accessible nodes are returned.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            role = await self._check_cross_tenant_read(
                request.context.tenant_id,
                request.context.actor,
                context,
            )

            # Wait for offset if requested (read-after-write consistency)
            if request.after_offset:
                timeout = request.wait_timeout_ms / 1000.0 if request.wait_timeout_ms else 30.0
                await self._wait_for_offset(
                    request.context.tenant_id, request.after_offset, timeout
                )

            filter_dict = None
            if request.filters:
                # Filter is kept name-keyed here — the new operator
                # translator inside ``canonical_store.query_nodes``
                # resolves field names to ids via the schema registry
                # so MongoDB-style operators (``$and``, ``$gt``, ...)
                # can also be wired through this path.
                filter_dict = {f.field: json_format.MessageToDict(f.value) for f in request.filters}

            nodes = await self.canonical_store.query_nodes(
                tenant_id=request.context.tenant_id,
                type_id=request.type_id,
                filter_json=filter_dict,
                limit=request.limit or 100,
                offset=request.offset or 0,
                order_by=request.order_by or "created_at",
                descending=request.descending,
                schema_registry=self.schema_registry,
            )

            # Cross-tenant: filter to only accessible nodes
            if role == "cross_tenant":
                actor_ids = await self.canonical_store.resolve_actor_groups(
                    request.context.tenant_id,
                    request.context.actor,
                )
                accessible = []
                for n in nodes:
                    if await self.canonical_store.can_access(
                        request.context.tenant_id,
                        n.node_id,
                        actor_ids,
                    ):
                        accessible.append(n)
                nodes = accessible

            proto_nodes = [
                Node(
                    tenant_id=n.tenant_id,
                    node_id=n.node_id,
                    type_id=n.type_id,
                    payload=_dict_to_struct(self._payload_id_to_name_dict(n)),
                    created_at=n.created_at,
                    updated_at=n.updated_at,
                    owner_actor=n.owner_actor,
                    acl=_acl_list_to_proto(n.acl),
                )
                for n in nodes
            ]

            record_grpc_request("QueryNodes", "ok", time.perf_counter() - start)
            return QueryNodesResponse(
                nodes=proto_nodes,
                has_more=len(nodes) == (request.limit or 100),
            )
        except Exception as e:
            record_grpc_request("QueryNodes", "error", time.perf_counter() - start)
            logger.error(f"QueryNodes failed: {e}", exc_info=True)
            return QueryNodesResponse(nodes=[])

    async def GetEdgesFrom(
        self,
        request: GetEdgesRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetEdgesResponse:
        """Get outgoing edges from a node."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            edge_type_id = request.edge_type_id if request.edge_type_id else None
            edges = await self.canonical_store.get_edges_from(
                request.context.tenant_id,
                request.node_id,
                edge_type_id,
            )

            limit = request.limit or 100
            proto_edges = [
                Edge(
                    tenant_id=e.tenant_id,
                    edge_type_id=e.edge_type_id,
                    from_node_id=e.from_node_id,
                    to_node_id=e.to_node_id,
                    props=_dict_to_struct(e.props),
                    created_at=e.created_at,
                )
                for e in edges[:limit]
            ]

            record_grpc_request("GetEdgesFrom", "ok", time.perf_counter() - start)
            return GetEdgesResponse(edges=proto_edges, has_more=len(edges) > limit)
        except Exception as e:
            record_grpc_request("GetEdgesFrom", "error", time.perf_counter() - start)
            logger.error(f"GetEdgesFrom failed: {e}", exc_info=True)
            return GetEdgesResponse(edges=[])

    async def GetEdgesTo(
        self,
        request: GetEdgesRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetEdgesResponse:
        """Get incoming edges to a node."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            edge_type_id = request.edge_type_id if request.edge_type_id else None
            edges = await self.canonical_store.get_edges_to(
                request.context.tenant_id,
                request.node_id,
                edge_type_id,
            )

            limit = request.limit or 100
            proto_edges = [
                Edge(
                    tenant_id=e.tenant_id,
                    edge_type_id=e.edge_type_id,
                    from_node_id=e.from_node_id,
                    to_node_id=e.to_node_id,
                    props=_dict_to_struct(e.props),
                    created_at=e.created_at,
                )
                for e in edges[:limit]
            ]

            record_grpc_request("GetEdgesTo", "ok", time.perf_counter() - start)
            return GetEdgesResponse(edges=proto_edges, has_more=len(edges) > limit)
        except Exception as e:
            record_grpc_request("GetEdgesTo", "error", time.perf_counter() - start)
            logger.error(f"GetEdgesTo failed: {e}", exc_info=True)
            return GetEdgesResponse(edges=[])

    async def SearchMailbox(
        self,
        request: SearchMailboxRequest,
        context: grpc_aio.ServicerContext,
    ) -> SearchMailboxResponse:
        """Search mailbox (deprecated, returns empty).

        The legacy per-user mailbox SQLite store has been removed. Fanout now
        writes to the per-tenant ``notifications`` table in the canonical
        store. This handler is retained for proto compatibility and returns
        an empty result set.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            record_grpc_request("SearchMailbox", "ok", time.perf_counter() - start)
            return SearchMailboxResponse(results=[], has_more=False)
        except Exception as e:
            record_grpc_request("SearchMailbox", "error", time.perf_counter() - start)
            logger.error(f"SearchMailbox failed: {e}", exc_info=True)
            return SearchMailboxResponse(results=[])

    async def GetMailbox(
        self,
        request: GetMailboxRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetMailboxResponse:
        """Get mailbox items for a user (deprecated, returns empty).

        The legacy per-user mailbox SQLite store has been removed. Fanout now
        writes to the per-tenant ``notifications`` table in the canonical
        store. This handler is retained for proto compatibility and returns
        an empty result set.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            record_grpc_request("GetMailbox", "ok", time.perf_counter() - start)
            return GetMailboxResponse(items=[], unread_count=0, has_more=False)
        except Exception as e:
            record_grpc_request("GetMailbox", "error", time.perf_counter() - start)
            logger.error(f"GetMailbox failed: {e}", exc_info=True)
            return GetMailboxResponse(items=[], unread_count=0)

    async def Health(
        self,
        request: HealthRequest,
        context: grpc_aio.ServicerContext,
    ) -> HealthResponse:
        """Get server health status."""
        start = time.perf_counter()
        try:
            components = {}

            # Check WAL connection
            try:
                wal_healthy = self.wal.is_connected if hasattr(self.wal, "is_connected") else True
                components["wal"] = "healthy" if wal_healthy else "unhealthy"
            except Exception:
                components["wal"] = "unknown"

            components["storage"] = "healthy"

            if self._sharding and self._sharding.is_multi_node:
                components["node_id"] = self._sharding.node_id
                components["assigned_tenants"] = ",".join(sorted(self._sharding.assigned_tenants))

            overall_healthy = all(
                v == "healthy" for k, v in components.items() if k in ("wal", "storage")
            )

            record_grpc_request("Health", "ok", time.perf_counter() - start)
            return HealthResponse(
                healthy=overall_healthy,
                version="1.0.0",
                components=components,
            )
        except Exception:
            record_grpc_request("Health", "error", time.perf_counter() - start)
            raise

    async def ListTenants(
        self,
        request: ListTenantsRequest,
        context: grpc_aio.ServicerContext,
    ) -> ListTenantsResponse:
        """List all tenants that have data."""
        start = time.perf_counter()
        try:
            tenant_ids = self.canonical_store.list_tenants()
            if self._sharding and self._sharding.is_multi_node:
                tenant_ids = [tid for tid in tenant_ids if self._sharding.is_mine(tid)]
            record_grpc_request("ListTenants", "ok", time.perf_counter() - start)
            return ListTenantsResponse(
                tenants=[TenantInfo(tenant_id=tid) for tid in tenant_ids],
            )
        except Exception as e:
            record_grpc_request("ListTenants", "error", time.perf_counter() - start)
            logger.error(f"ListTenants failed: {e}", exc_info=True)
            return ListTenantsResponse(tenants=[])

    async def ListMailboxUsers(
        self,
        request: ListMailboxUsersRequest,
        context: grpc_aio.ServicerContext,
    ) -> ListMailboxUsersResponse:
        """List mailbox users for a tenant (deprecated, returns empty).

        The legacy per-user mailbox SQLite store has been removed. This
        handler is retained for proto compatibility and returns an empty
        user list.
        """
        await self._check_tenant(request.tenant_id, context)
        return ListMailboxUsersResponse(user_ids=[])

    async def GetSchema(
        self,
        request: GetSchemaRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetSchemaResponse:
        """Get schema information.

        Returns registry schema. When registry is empty and tenant_id is provided,
        builds minimal schema from distinct type_ids in the data.
        """
        start = time.perf_counter()
        try:
            schema_dict = self.schema_registry.to_dict()
            has_registry_types = bool(
                schema_dict.get("node_types") or schema_dict.get("edge_types")
            )

            # Fallback to data-driven schema when registry is empty
            if not has_registry_types and request.tenant_id:
                try:
                    node_type_rows = await self.canonical_store.get_distinct_type_ids(
                        request.tenant_id,
                    )
                    edge_type_rows = await self.canonical_store.get_distinct_edge_type_ids(
                        request.tenant_id,
                    )
                    schema_dict = {
                        "node_types": [
                            {
                                "type_id": row["type_id"],
                                "name": f"Type_{row['type_id']}",
                                "fields": [],
                            }
                            for row in node_type_rows
                        ],
                        "edge_types": [
                            {
                                "edge_id": row["edge_type_id"],
                                "name": f"Edge_{row['edge_type_id']}",
                                "from_type_id": 0,
                                "to_type_id": 0,
                                "props": [],
                            }
                            for row in edge_type_rows
                        ],
                    }
                except Exception as e:
                    logger.debug(f"Could not build data-driven schema: {e}")

            if request.type_id:
                schema_dict["node_types"] = [
                    t
                    for t in schema_dict.get("node_types", [])
                    if t.get("type_id") == request.type_id
                ]
                schema_dict["edge_types"] = [
                    e
                    for e in schema_dict.get("edge_types", [])
                    if e.get("from_type_id") == request.type_id
                    or e.get("to_type_id") == request.type_id
                ]

            record_grpc_request("GetSchema", "ok", time.perf_counter() - start)
            return GetSchemaResponse(
                schema=_dict_to_struct(schema_dict),
                fingerprint=self.schema_registry.fingerprint or "",
            )
        except Exception as e:
            record_grpc_request("GetSchema", "error", time.perf_counter() - start)
            logger.error(f"GetSchema failed: {e}", exc_info=True)
            return GetSchemaResponse(fingerprint="")

    # --- ACL v2 handlers ---

    def _node_to_proto(self, n: Any) -> Node:
        """Convert an internal Node to a protobuf Node message.

        Egress translates the stored id-keyed payload back to name-keyed
        JSON so clients always see field names (issue #104).
        """
        return Node(
            tenant_id=n.tenant_id,
            node_id=n.node_id,
            type_id=n.type_id,
            payload_json=self._payload_id_to_name_json(n),
            created_at=n.created_at,
            updated_at=n.updated_at,
            owner_actor=n.owner_actor,
            acl_json=n.acl_json,
        )

    async def GetConnectedNodes(
        self,
        request: GetConnectedNodesRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetConnectedNodesResponse:
        """Get connected nodes via edge type with ACL filtering."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            actor_ids = await self.canonical_store.resolve_actor_groups(
                request.context.tenant_id,
                request.context.actor,
            )
            limit = request.limit or 100
            nodes = await self.canonical_store.get_connected_nodes(
                tenant_id=request.context.tenant_id,
                node_id=request.node_id,
                edge_type_id=request.edge_type_id,
                actor_ids=actor_ids,
                limit=limit,
                offset=request.offset or 0,
            )
            proto_nodes = [self._node_to_proto(n) for n in nodes]
            record_grpc_request("GetConnectedNodes", "ok", time.perf_counter() - start)
            return GetConnectedNodesResponse(
                nodes=proto_nodes,
                has_more=len(nodes) == limit,
            )
        except Exception as e:
            record_grpc_request("GetConnectedNodes", "error", time.perf_counter() - start)
            logger.error(f"GetConnectedNodes failed: {e}", exc_info=True)
            return GetConnectedNodesResponse(nodes=[])

    async def ShareNode(
        self,
        request: ShareNodeRequest,
        context: grpc_aio.ServicerContext,
    ) -> ShareNodeResponse:
        """Share a node with an actor.

        Accepts the legacy ``permission`` string and the typed
        ``core_caps`` / ``ext_cap_ids`` fields. When both are set the
        typed fields win. When only the string is set, it is still
        persisted so older consumers keep working, and the server
        derives the typed caps via
        ``CapabilityRegistry.legacy_permission_to_core_caps``.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            await self._check_tenant_access(
                request.context.tenant_id,
                request.context.actor,
                context,
                require_write=True,
            )
            tenant_id = request.context.tenant_id
            actor_id = request.actor_id
            permission = request.permission or "read"
            expires_at = request.expires_at if request.expires_at else None
            type_id = int(request.type_id or 0)
            core_caps: list[int] | None = (
                [int(c) for c in request.core_caps] if request.core_caps else None
            )
            ext_cap_ids: list[int] | None = (
                [int(e) for e in request.ext_cap_ids] if request.ext_cap_ids else None
            )
            # Authz: granting requires ADMIN on the target node.
            try:
                await self._check_capability(
                    tenant_id,
                    request.context.actor,
                    request.node_id,
                    type_id,
                    "ShareNode",
                    context,
                )
            except PermissionError as perm_err:
                record_grpc_request("ShareNode", "denied", time.perf_counter() - start)
                return ShareNodeResponse(success=False, error=str(perm_err))
            await self.canonical_store.share_node(
                tenant_id=tenant_id,
                node_id=request.node_id,
                actor_id=actor_id,
                permission=permission,
                granted_by=request.context.actor,
                actor_type=request.actor_type or "user",
                expires_at=expires_at,
                type_id=type_id,
                core_caps=core_caps,
                ext_cap_ids=ext_cap_ids,
            )
            # Update cross-tenant shared_index
            if self.global_store:
                try:
                    if actor_id.startswith("group:"):
                        members = await self.canonical_store.get_group_members(tenant_id, actor_id)
                        for member_id in members:
                            await self.global_store.add_shared(
                                member_id, tenant_id, request.node_id, permission
                            )
                    else:
                        await self.global_store.add_shared(
                            actor_id, tenant_id, request.node_id, permission
                        )
                except Exception as gs_err:
                    logger.warning(f"shared_index update failed on ShareNode: {gs_err}")
            record_grpc_request("ShareNode", "ok", time.perf_counter() - start)
            return ShareNodeResponse(success=True)
        except Exception as e:
            record_grpc_request("ShareNode", "error", time.perf_counter() - start)
            logger.error(f"ShareNode failed: {e}", exc_info=True)
            return ShareNodeResponse(success=False, error=str(e))

    async def RevokeAccess(
        self,
        request: RevokeAccessRequest,
        context: grpc_aio.ServicerContext,
    ) -> RevokeAccessResponse:
        """Revoke access from a node for an actor."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            tenant_id = request.context.tenant_id
            actor_id = request.actor_id
            try:
                await self._check_capability(
                    tenant_id,
                    request.context.actor,
                    request.node_id,
                    0,
                    "RevokeAccess",
                    context,
                )
            except PermissionError as perm_err:
                record_grpc_request("RevokeAccess", "denied", time.perf_counter() - start)
                return RevokeAccessResponse(found=False, error=str(perm_err))
            found = await self.canonical_store.revoke_access(
                tenant_id=tenant_id,
                node_id=request.node_id,
                actor_id=actor_id,
            )
            # Update cross-tenant shared_index
            if found and self.global_store:
                try:
                    if actor_id.startswith("group:"):
                        members = await self.canonical_store.get_group_members(tenant_id, actor_id)
                        for member_id in members:
                            await self.global_store.remove_shared(
                                member_id, tenant_id, request.node_id
                            )
                    else:
                        await self.global_store.remove_shared(actor_id, tenant_id, request.node_id)
                except Exception as gs_err:
                    logger.warning(f"shared_index update failed on RevokeAccess: {gs_err}")
            record_grpc_request("RevokeAccess", "ok", time.perf_counter() - start)
            return RevokeAccessResponse(found=found)
        except Exception as e:
            record_grpc_request("RevokeAccess", "error", time.perf_counter() - start)
            logger.error(f"RevokeAccess failed: {e}", exc_info=True)
            return RevokeAccessResponse(found=False, error=str(e))

    async def ListSharedWithMe(
        self,
        request: ListSharedWithMeRequest,
        context: grpc_aio.ServicerContext,
    ) -> ListSharedWithMeResponse:
        """List nodes shared with the calling actor.

        Reads from per-tenant node_access for same-tenant shares,
        and ALSO from global shared_index for cross-tenant shares.
        """
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            tenant_id = request.context.tenant_id
            actor = request.context.actor
            actor_ids = await self.canonical_store.resolve_actor_groups(
                tenant_id,
                actor,
            )
            limit = request.limit or 100
            offset = request.offset or 0

            # 1. Per-tenant shares (same tenant node_access)
            nodes = await self.canonical_store.list_shared_with_me(
                tenant_id=tenant_id,
                actor_ids=actor_ids,
                limit=limit,
                offset=offset,
            )

            # 2. Cross-tenant shares from global shared_index
            if self.global_store:
                try:
                    shared_entries = await self.global_store.get_shared_with_me(
                        user_id=actor,
                        limit=limit,
                        offset=offset,
                    )
                    # Collect node IDs already included from per-tenant results
                    seen = {(n.tenant_id, n.node_id) for n in nodes}
                    # Fetch actual nodes from source tenants
                    for entry in shared_entries:
                        key = (entry["source_tenant"], entry["node_id"])
                        if key in seen:
                            continue
                        try:
                            node = await self.canonical_store.get_node(
                                entry["source_tenant"], entry["node_id"]
                            )
                            if node:
                                nodes.append(node)
                                seen.add(key)
                        except Exception:
                            # Stale entry or tenant not loaded — skip
                            pass
                except Exception as gs_err:
                    logger.warning(f"shared_index read failed in ListSharedWithMe: {gs_err}")

            proto_nodes = [self._node_to_proto(n) for n in nodes]
            record_grpc_request("ListSharedWithMe", "ok", time.perf_counter() - start)
            return ListSharedWithMeResponse(
                nodes=proto_nodes,
                has_more=len(nodes) >= limit,
            )
        except Exception as e:
            record_grpc_request("ListSharedWithMe", "error", time.perf_counter() - start)
            logger.error(f"ListSharedWithMe failed: {e}", exc_info=True)
            return ListSharedWithMeResponse(nodes=[])

    async def AddGroupMember(
        self,
        request: GroupMemberRequest,
        context: grpc_aio.ServicerContext,
    ) -> GroupMemberResponse:
        """Add a member to a group."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            tenant_id = request.context.tenant_id
            group_id = request.group_id
            member_actor_id = request.member_actor_id
            await self.canonical_store.add_group_member(
                tenant_id=tenant_id,
                group_id=group_id,
                member_actor_id=member_actor_id,
                role=request.role or "member",
            )
            # Cascade shared_index: add entries for all nodes the group has access to
            if self.global_store:
                try:
                    access_entries = await self.canonical_store.list_node_access_for_group(
                        tenant_id, group_id
                    )
                    for entry in access_entries:
                        await self.global_store.add_shared(
                            member_actor_id,
                            tenant_id,
                            entry["node_id"],
                            entry["permission"],
                        )
                except Exception as gs_err:
                    logger.warning(f"shared_index update failed on AddGroupMember: {gs_err}")
            record_grpc_request("AddGroupMember", "ok", time.perf_counter() - start)
            return GroupMemberResponse(success=True)
        except Exception as e:
            record_grpc_request("AddGroupMember", "error", time.perf_counter() - start)
            logger.error(f"AddGroupMember failed: {e}", exc_info=True)
            return GroupMemberResponse(success=False, error=str(e))

    async def RemoveGroupMember(
        self,
        request: GroupMemberRequest,
        context: grpc_aio.ServicerContext,
    ) -> GroupMemberResponse:
        """Remove a member from a group."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            tenant_id = request.context.tenant_id
            group_id = request.group_id
            member_actor_id = request.member_actor_id
            # Collect group access entries BEFORE removing the member
            group_access_entries: list[dict] = []
            if self.global_store:
                try:
                    group_access_entries = await self.canonical_store.list_node_access_for_group(
                        tenant_id, group_id
                    )
                except Exception as gs_err:
                    logger.warning(
                        f"Failed to read group access for shared_index cleanup: {gs_err}"
                    )
            found = await self.canonical_store.remove_group_member(
                tenant_id=tenant_id,
                group_id=group_id,
                member_actor_id=member_actor_id,
            )
            # Cascade shared_index: remove entries for all nodes the group had access to
            if found and self.global_store and group_access_entries:
                try:
                    for entry in group_access_entries:
                        await self.global_store.remove_shared(
                            member_actor_id, tenant_id, entry["node_id"]
                        )
                except Exception as gs_err:
                    logger.warning(f"shared_index update failed on RemoveGroupMember: {gs_err}")
            record_grpc_request("RemoveGroupMember", "ok", time.perf_counter() - start)
            return GroupMemberResponse(success=found)
        except Exception as e:
            record_grpc_request("RemoveGroupMember", "error", time.perf_counter() - start)
            logger.error(f"RemoveGroupMember failed: {e}", exc_info=True)
            return GroupMemberResponse(success=False, error=str(e))

    async def TransferOwnership(
        self,
        request: TransferOwnershipRequest,
        context: grpc_aio.ServicerContext,
    ) -> TransferOwnershipResponse:
        """Transfer ownership of a node."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            found = await self.canonical_store.transfer_ownership(
                tenant_id=request.context.tenant_id,
                node_id=request.node_id,
                new_owner=request.new_owner,
            )
            record_grpc_request("TransferOwnership", "ok", time.perf_counter() - start)
            return TransferOwnershipResponse(found=found)
        except Exception as e:
            record_grpc_request("TransferOwnership", "error", time.perf_counter() - start)
            logger.error(f"TransferOwnership failed: {e}", exc_info=True)
            return TransferOwnershipResponse(found=False, error=str(e))

    # --- User registry handlers ---

    def _is_admin_or_system(self, actor: str) -> bool:
        """Check if actor is admin or system."""
        return actor.startswith("system:") or actor.startswith("admin:") or actor == "__system__"

    def _is_self_or_admin(self, actor: str, user_id: str) -> bool:
        """Check if actor is the user themselves or an admin/system."""
        return self._is_admin_or_system(actor) or actor == f"user:{user_id}" or actor == user_id

    def _user_dict_to_proto(self, user: dict) -> UserInfo:
        """Convert a user dict to a UserInfo proto message."""
        return UserInfo(
            user_id=user.get("user_id", ""),
            email=user.get("email", ""),
            name=user.get("name", ""),
            status=user.get("status", ""),
            created_at=user.get("created_at", 0),
            updated_at=user.get("updated_at", 0),
        )

    async def CreateUser(
        self,
        request: CreateUserRequest,
        context: grpc_aio.ServicerContext,
    ) -> CreateUserResponse:
        """Create a new user. Requires admin or system actor."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "User registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not self._is_admin_or_system(request.actor):
                await context.abort(
                    grpc.StatusCode.PERMISSION_DENIED,
                    "CreateUser requires admin or system actor",
                )
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            if not request.email:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "email is required")
            if not request.name:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "name is required")

            user = await self.global_store.create_user(
                request.user_id,
                request.email,
                request.name,
            )

            record_grpc_request("CreateUser", "ok", time.perf_counter() - start)
            return CreateUserResponse(
                success=True,
                user=self._user_dict_to_proto(user),
            )
        except Exception as e:
            if "UNIQUE constraint" in str(e) or "IntegrityError" in type(e).__name__:
                record_grpc_request("CreateUser", "error", time.perf_counter() - start)
                return CreateUserResponse(
                    success=False,
                    error=f"User already exists: {e}",
                )
            record_grpc_request("CreateUser", "error", time.perf_counter() - start)
            logger.error(f"CreateUser failed: {e}", exc_info=True)
            return CreateUserResponse(success=False, error=str(e))

    async def GetUser(
        self,
        request: GetUserRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetUserResponse:
        """Get a user by ID. Available to any authenticated actor."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "User registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")

            user = await self.global_store.get_user(request.user_id)

            if user is None:
                record_grpc_request("GetUser", "ok", time.perf_counter() - start)
                return GetUserResponse(found=False)

            record_grpc_request("GetUser", "ok", time.perf_counter() - start)
            return GetUserResponse(
                found=True,
                user=self._user_dict_to_proto(user),
            )
        except Exception as e:
            record_grpc_request("GetUser", "error", time.perf_counter() - start)
            logger.error(f"GetUser failed: {e}", exc_info=True)
            return GetUserResponse(found=False)

    async def UpdateUser(
        self,
        request: UpdateUserRequest,
        context: grpc_aio.ServicerContext,
    ) -> UpdateUserResponse:
        """Update a user. Requires the user themselves or admin."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "User registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            if not self._is_self_or_admin(request.actor, request.user_id):
                await context.abort(
                    grpc.StatusCode.PERMISSION_DENIED,
                    "UpdateUser requires the user themselves or admin actor",
                )

            kwargs = {}
            if request.email:
                kwargs["email"] = request.email
            if request.name:
                kwargs["name"] = request.name
            if request.status:
                kwargs["status"] = request.status

            if not kwargs:
                record_grpc_request("UpdateUser", "ok", time.perf_counter() - start)
                return UpdateUserResponse(success=False, error="No fields to update")

            updated = await self.global_store.update_user(request.user_id, **kwargs)

            record_grpc_request("UpdateUser", "ok", time.perf_counter() - start)
            if not updated:
                return UpdateUserResponse(success=False, error="User not found")
            return UpdateUserResponse(success=True)
        except Exception as e:
            record_grpc_request("UpdateUser", "error", time.perf_counter() - start)
            logger.error(f"UpdateUser failed: {e}", exc_info=True)
            return UpdateUserResponse(success=False, error=str(e))

    async def ListUsers(
        self,
        request: ListUsersRequest,
        context: grpc_aio.ServicerContext,
    ) -> ListUsersResponse:
        """List users. Available to any authenticated actor."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "User registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")

            status = request.status or "active"
            limit = request.limit or 100
            offset = request.offset or 0

            users = await self.global_store.list_users(
                status=status,
                limit=limit,
                offset=offset,
            )

            proto_users = [self._user_dict_to_proto(u) for u in users]

            record_grpc_request("ListUsers", "ok", time.perf_counter() - start)
            return ListUsersResponse(users=proto_users)
        except Exception as e:
            record_grpc_request("ListUsers", "error", time.perf_counter() - start)
            logger.error(f"ListUsers failed: {e}", exc_info=True)
            return ListUsersResponse(users=[])

    # --- Tenant registry handlers ---

    @staticmethod
    def _tenant_dict_to_proto(t: dict) -> TenantDetail:
        """Convert a tenant dict from GlobalStore to protobuf TenantDetail."""
        return TenantDetail(
            tenant_id=t.get("tenant_id", ""),
            name=t.get("name", ""),
            status=t.get("status", ""),
            created_at=t.get("created_at", 0),
        )

    @staticmethod
    def _member_dict_to_proto(m: dict) -> TenantMemberInfo:
        """Convert a member dict from GlobalStore to protobuf TenantMemberInfo."""
        return TenantMemberInfo(
            tenant_id=m.get("tenant_id", ""),
            user_id=m.get("user_id", ""),
            role=m.get("role", ""),
            joined_at=m.get("joined_at", 0),
        )

    async def _get_member_role(self, tenant_id: str, user_id: str) -> str | None:
        """Get the role of a user in a tenant, or None if not a member."""
        members = await self.global_store.get_members(tenant_id)
        for m in members:
            if m["user_id"] == user_id:
                return m["role"]
        return None

    def _actor_user_id(self, actor: str) -> str:
        """Extract user_id from actor string like 'user:ID'."""
        if actor.startswith("user:"):
            return actor[5:]
        return actor

    async def CreateTenant(
        self,
        request: CreateTenantRequest,
        context: grpc_aio.ServicerContext,
    ) -> CreateTenantResponse:
        """Create a new tenant. Also initializes the per-tenant SQLite file."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not request.name:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "name is required")

            tenant = await self.global_store.create_tenant(request.tenant_id, request.name)

            # Initialize the per-tenant SQLite database
            await self.canonical_store.initialize_tenant(request.tenant_id)

            # Add the creator as owner
            creator_user_id = self._actor_user_id(request.actor)
            await self.global_store.add_member(request.tenant_id, creator_user_id, role="owner")

            record_grpc_request("CreateTenant", "ok", time.perf_counter() - start)
            return CreateTenantResponse(
                success=True,
                tenant=self._tenant_dict_to_proto(tenant),
            )
        except Exception as e:
            if "UNIQUE constraint" in str(e) or "IntegrityError" in type(e).__name__:
                record_grpc_request("CreateTenant", "error", time.perf_counter() - start)
                return CreateTenantResponse(
                    success=False,
                    error=f"Tenant already exists: {e}",
                )
            record_grpc_request("CreateTenant", "error", time.perf_counter() - start)
            logger.error(f"CreateTenant failed: {e}", exc_info=True)
            return CreateTenantResponse(success=False, error=str(e))

    async def GetTenant(
        self,
        request: GetTenantRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetTenantResponse:
        """Get a tenant by ID."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")

            tenant = await self.global_store.get_tenant(request.tenant_id)

            if tenant is None:
                record_grpc_request("GetTenant", "ok", time.perf_counter() - start)
                return GetTenantResponse(found=False)

            record_grpc_request("GetTenant", "ok", time.perf_counter() - start)
            return GetTenantResponse(
                found=True,
                tenant=self._tenant_dict_to_proto(tenant),
            )
        except Exception as e:
            record_grpc_request("GetTenant", "error", time.perf_counter() - start)
            logger.error(f"GetTenant failed: {e}", exc_info=True)
            return GetTenantResponse(found=False)

    async def ArchiveTenant(
        self,
        request: ArchiveTenantRequest,
        context: grpc_aio.ServicerContext,
    ) -> ArchiveTenantResponse:
        """Archive a tenant (set status to 'archived'). Only owner can archive."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")

            # Only owner or system can archive
            actor_uid = self._actor_user_id(request.actor)
            if not self._is_admin_or_system(request.actor):
                role = await self._get_member_role(request.tenant_id, actor_uid)
                if role != "owner":
                    await context.abort(
                        grpc.StatusCode.PERMISSION_DENIED,
                        "Only tenant owner can archive a tenant",
                    )

            updated = await self.global_store.set_tenant_status(request.tenant_id, "archived")

            if not updated:
                record_grpc_request("ArchiveTenant", "ok", time.perf_counter() - start)
                return ArchiveTenantResponse(success=False, error="Tenant not found")

            record_grpc_request("ArchiveTenant", "ok", time.perf_counter() - start)
            return ArchiveTenantResponse(success=True)
        except Exception as e:
            record_grpc_request("ArchiveTenant", "error", time.perf_counter() - start)
            logger.error(f"ArchiveTenant failed: {e}", exc_info=True)
            return ArchiveTenantResponse(success=False, error=str(e))

    async def AddTenantMember(
        self,
        request: TenantMemberRequest,
        context: grpc_aio.ServicerContext,
    ) -> TenantMemberResponse:
        """Add a member to a tenant. Only owner/admin can add members."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")

            # Enforce: only owner/admin can add members
            actor_uid = self._actor_user_id(request.actor)
            if not self._is_admin_or_system(request.actor):
                role = await self._get_member_role(request.tenant_id, actor_uid)
                if role not in ("owner", "admin"):
                    await context.abort(
                        grpc.StatusCode.PERMISSION_DENIED,
                        "Only owner or admin can add members",
                    )

            member_role = request.role or "member"
            await self.global_store.add_member(request.tenant_id, request.user_id, role=member_role)

            record_grpc_request("AddTenantMember", "ok", time.perf_counter() - start)
            return TenantMemberResponse(success=True)
        except Exception as e:
            if "UNIQUE constraint" in str(e) or "IntegrityError" in type(e).__name__:
                record_grpc_request("AddTenantMember", "error", time.perf_counter() - start)
                return TenantMemberResponse(
                    success=False,
                    error="Member already exists in this tenant",
                )
            record_grpc_request("AddTenantMember", "error", time.perf_counter() - start)
            logger.error(f"AddTenantMember failed: {e}", exc_info=True)
            return TenantMemberResponse(success=False, error=str(e))

    async def RemoveTenantMember(
        self,
        request: TenantMemberRequest,
        context: grpc_aio.ServicerContext,
    ) -> TenantMemberResponse:
        """Remove a member from a tenant. Last owner cannot leave."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")

            # Enforce: last owner cannot leave
            members = await self.global_store.get_members(request.tenant_id)
            target_member = None
            owner_count = 0
            for m in members:
                if m["role"] == "owner":
                    owner_count += 1
                if m["user_id"] == request.user_id:
                    target_member = m

            if target_member is None:
                record_grpc_request("RemoveTenantMember", "ok", time.perf_counter() - start)
                return TenantMemberResponse(success=False, error="Member not found")

            if target_member["role"] == "owner" and owner_count <= 1:
                record_grpc_request("RemoveTenantMember", "error", time.perf_counter() - start)
                return TenantMemberResponse(
                    success=False,
                    error="Cannot remove the last owner of a tenant",
                )

            removed = await self.global_store.remove_member(request.tenant_id, request.user_id)

            record_grpc_request("RemoveTenantMember", "ok", time.perf_counter() - start)
            return TenantMemberResponse(success=removed)
        except Exception as e:
            record_grpc_request("RemoveTenantMember", "error", time.perf_counter() - start)
            logger.error(f"RemoveTenantMember failed: {e}", exc_info=True)
            return TenantMemberResponse(success=False, error=str(e))

    async def GetTenantMembers(
        self,
        request: GetTenantMembersRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetTenantMembersResponse:
        """List all members of a tenant."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")

            members = await self.global_store.get_members(request.tenant_id)
            proto_members = [self._member_dict_to_proto(m) for m in members]

            record_grpc_request("GetTenantMembers", "ok", time.perf_counter() - start)
            return GetTenantMembersResponse(members=proto_members)
        except Exception as e:
            record_grpc_request("GetTenantMembers", "error", time.perf_counter() - start)
            logger.error(f"GetTenantMembers failed: {e}", exc_info=True)
            return GetTenantMembersResponse(members=[])

    async def GetUserTenants(
        self,
        request: GetUserTenantsRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetUserTenantsResponse:
        """List all tenants a user belongs to."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")

            memberships = await self.global_store.get_user_tenants(request.user_id)
            proto_memberships = [self._member_dict_to_proto(m) for m in memberships]

            record_grpc_request("GetUserTenants", "ok", time.perf_counter() - start)
            return GetUserTenantsResponse(memberships=proto_memberships)
        except Exception as e:
            record_grpc_request("GetUserTenants", "error", time.perf_counter() - start)
            logger.error(f"GetUserTenants failed: {e}", exc_info=True)
            return GetUserTenantsResponse(memberships=[])

    async def ChangeMemberRole(
        self,
        request: ChangeMemberRoleRequest,
        context: grpc_aio.ServicerContext,
    ) -> ChangeMemberRoleResponse:
        """Change a member's role. Only owner can change roles."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )

            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            if not request.new_role:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "new_role is required")

            # Enforce: only owner can change roles
            actor_uid = self._actor_user_id(request.actor)
            if not self._is_admin_or_system(request.actor):
                role = await self._get_member_role(request.tenant_id, actor_uid)
                if role != "owner":
                    await context.abort(
                        grpc.StatusCode.PERMISSION_DENIED,
                        "Only owner can change member roles",
                    )

            updated = await self.global_store.change_role(
                request.tenant_id,
                request.user_id,
                request.new_role,
            )

            if not updated:
                record_grpc_request("ChangeMemberRole", "ok", time.perf_counter() - start)
                return ChangeMemberRoleResponse(success=False, error="Member not found")

            record_grpc_request("ChangeMemberRole", "ok", time.perf_counter() - start)
            return ChangeMemberRoleResponse(success=True)
        except Exception as e:
            record_grpc_request("ChangeMemberRole", "error", time.perf_counter() - start)
            logger.error(f"ChangeMemberRole failed: {e}", exc_info=True)
            return ChangeMemberRoleResponse(success=False, error=str(e))

    # --- Admin operations (Issue #90, ADR-003) -----------------------

    async def _require_admin_or_owner(
        self,
        tenant_id: str,
        actor: str,
        context: grpc_aio.ServicerContext,
        rpc_name: str,
    ) -> str:
        """Abort unless the caller is admin/owner of the tenant (or system).

        The authoritative actor is the trusted identity populated by
        :class:`AuthInterceptor`, not the ``actor`` field from the
        client payload. A client claiming ``actor: "system:admin"`` while
        authenticated as ``user:eve`` is downgraded to ``user:eve``.
        Returns the trusted actor so callers can reuse it.
        """
        from ..auth.auth_interceptor import get_authoritative_actor

        trusted_actor = get_authoritative_actor(actor)
        if not trusted_actor:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
        if self._is_admin_or_system(trusted_actor):
            return trusted_actor
        if self.global_store is None:
            await context.abort(
                grpc.StatusCode.PERMISSION_DENIED,
                f"{rpc_name} requires admin or owner role",
            )
        actor_uid = self._actor_user_id(trusted_actor)
        role = await self._get_member_role(tenant_id, actor_uid)
        if role not in ("owner", "admin"):
            await context.abort(
                grpc.StatusCode.PERMISSION_DENIED,
                f"{rpc_name} requires admin or owner role",
            )
        return trusted_actor

    async def TransferUserContent(
        self,
        request: TransferUserContentRequest,
        context: grpc_aio.ServicerContext,
    ) -> TransferUserContentResponse:
        """Reassign ownership of all nodes from one user to another."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.tenant_id, context)
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not request.from_user:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "from_user is required")
            if not request.to_user:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "to_user is required")
            trusted_actor = await self._require_admin_or_owner(
                request.tenant_id, request.actor, context, "TransferUserContent"
            )

            # WAL-first: append an admin_transfer_content event so the
            # ownership change survives a full WAL rebuild. Direct
            # canonical_store writes from this handler are forbidden by
            # the architecture invariant ("all writes go through the WAL").
            from .admin_handlers import handle_transfer_user_content

            await handle_transfer_user_content(
                self.global_store,
                self.wal,
                tenant_id=request.tenant_id,
                from_user=request.from_user,
                to_user=request.to_user,
                actor=trusted_actor,
                topic=self.topic,
            )
            transferred = 0
            try:
                with self.canonical_store._get_connection(request.tenant_id) as conn:
                    row = conn.execute(
                        "SELECT COUNT(*) AS n FROM nodes WHERE tenant_id = ? AND owner_actor = ?",
                        (request.tenant_id, request.from_user),
                    ).fetchone()
                    if row is not None:
                        transferred = int(row["n"])
            except Exception:
                pass

            record_grpc_request("TransferUserContent", "ok", time.perf_counter() - start)
            return TransferUserContentResponse(success=True, transferred=transferred)
        except Exception as e:
            record_grpc_request("TransferUserContent", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"TransferUserContent failed: {e}", exc_info=True)
            return TransferUserContentResponse(success=False, error=str(e))

    async def DelegateAccess(
        self,
        request: DelegateAccessRequest,
        context: grpc_aio.ServicerContext,
    ) -> DelegateAccessResponse:
        """Grant a user temporary access to another user's content."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.tenant_id, context)
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not request.from_user:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "from_user is required")
            if not request.to_user:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "to_user is required")
            trusted_actor = await self._require_admin_or_owner(
                request.tenant_id, request.actor, context, "DelegateAccess"
            )

            permission = request.permission or "read"
            expires_at = request.expires_at if request.expires_at else None

            # WAL-first: append an admin_delegate_access event. Direct
            # canonical_store writes from this handler are forbidden by
            # the architecture invariant ("all writes go through the WAL").
            from .admin_handlers import handle_delegate_access

            await handle_delegate_access(
                self.global_store,
                self.wal,
                tenant_id=request.tenant_id,
                from_user=request.from_user,
                to_user=request.to_user,
                permission=permission,
                expires_at=expires_at,
                actor=trusted_actor,
                topic=self.topic,
            )
            delegated = 0
            try:
                with self.canonical_store._get_connection(request.tenant_id) as conn:
                    row = conn.execute(
                        "SELECT COUNT(*) AS n FROM nodes WHERE tenant_id = ? AND owner_actor = ?",
                        (request.tenant_id, request.from_user),
                    ).fetchone()
                    if row is not None:
                        delegated = int(row["n"])
            except Exception:
                pass

            record_grpc_request("DelegateAccess", "ok", time.perf_counter() - start)
            return DelegateAccessResponse(
                success=True, delegated=delegated, expires_at=expires_at or 0
            )
        except Exception as e:
            record_grpc_request("DelegateAccess", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"DelegateAccess failed: {e}", exc_info=True)
            return DelegateAccessResponse(success=False, error=str(e))

    async def SetLegalHold(
        self,
        request: LegalHoldRequest,
        context: grpc_aio.ServicerContext,
    ) -> LegalHoldResponse:
        """Enable or disable legal hold on a tenant."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Tenant registry not configured",
                )
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            await self._require_admin_or_owner(
                request.tenant_id, request.actor, context, "SetLegalHold"
            )

            updated = await self.global_store.set_legal_hold(
                request.tenant_id, bool(request.enabled), request.actor
            )
            if not updated:
                record_grpc_request("SetLegalHold", "ok", time.perf_counter() - start)
                return LegalHoldResponse(success=False, error="Tenant not found")

            new_status = "legal_hold" if request.enabled else "active"
            # Audit log — best effort; failing to audit should not
            # undo the status change since the tenant row is the
            # authoritative record.
            try:
                await self.canonical_store.append_audit(
                    tenant_id=request.tenant_id,
                    actor_id=request.actor,
                    action="set_legal_hold",
                    target_type="tenant",
                    target_id=request.tenant_id,
                    metadata=json.dumps(
                        {
                            "enabled": bool(request.enabled),
                            "status": new_status,
                        }
                    ),
                )
            except Exception as audit_err:
                logger.warning(f"audit append failed on SetLegalHold: {audit_err}")

            record_grpc_request("SetLegalHold", "ok", time.perf_counter() - start)
            return LegalHoldResponse(success=True, status=new_status)
        except Exception as e:
            record_grpc_request("SetLegalHold", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"SetLegalHold failed: {e}", exc_info=True)
            return LegalHoldResponse(success=False, error=str(e))

    async def RevokeAllUserAccess(
        self,
        request: RevokeAllUserAccessRequest,
        context: grpc_aio.ServicerContext,
    ) -> RevokeAllUserAccessResponse:
        """Remove ALL access (grants, group memberships, shared_index) for a user."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.tenant_id, context)
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            await self._require_admin_or_owner(
                request.tenant_id, request.actor, context, "RevokeAllUserAccess"
            )

            result = await self.canonical_store.revoke_user_access(
                tenant_id=request.tenant_id,
                user_id=request.user_id,
                actor=request.actor,
            )

            # Cleanup cross-tenant shared_index entries that originated
            # from this tenant. We can only scope by source_tenant, so
            # iterate the user's shared entries and delete those that
            # point at this tenant.
            revoked_shared = 0
            if self.global_store:
                try:
                    entries = await self.global_store.get_shared_with_me(
                        user_id=request.user_id, limit=10000, offset=0
                    )
                    for e in entries:
                        if e.get("source_tenant") == request.tenant_id:
                            ok = await self.global_store.remove_shared(
                                request.user_id,
                                request.tenant_id,
                                e["node_id"],
                            )
                            if ok:
                                revoked_shared += 1
                except Exception as gs_err:
                    logger.warning(f"shared_index cleanup failed on RevokeAllUserAccess: {gs_err}")

            record_grpc_request("RevokeAllUserAccess", "ok", time.perf_counter() - start)
            return RevokeAllUserAccessResponse(
                success=True,
                revoked_grants=result["revoked_grants"],
                revoked_groups=result["revoked_groups"],
                revoked_shared=revoked_shared,
            )
        except Exception as e:
            record_grpc_request("RevokeAllUserAccess", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"RevokeAllUserAccess failed: {e}", exc_info=True)
            return RevokeAllUserAccessResponse(success=False, error=str(e))

    # ── GDPR operations (Issue #103, ADR-004) ──────────────────────────

    async def DeleteUser(
        self,
        request: DeleteUserRequest,
        context: grpc_aio.ServicerContext,
    ) -> DeleteUserResponse:
        """Queue a user for GDPR right-to-erasure.

        The user is not deleted immediately; instead an entry is added to
        ``deletion_queue`` with a ``grace_days`` grace period (default 30).
        A background worker processes the queue after the grace period.
        """
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(grpc.StatusCode.UNIMPLEMENTED, "User registry not configured")
            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            if not self._is_self_or_admin(request.actor, request.user_id):
                await context.abort(
                    grpc.StatusCode.PERMISSION_DENIED,
                    "DeleteUser requires the user themselves or an admin actor",
                )

            user = await self.global_store.get_user(request.user_id)
            if user is None:
                record_grpc_request("DeleteUser", "ok", time.perf_counter() - start)
                return DeleteUserResponse(success=False, error="User not found")

            existing = await self.global_store.get_deletion_entry(request.user_id)
            if existing and existing.get("status") == "pending":
                record_grpc_request("DeleteUser", "ok", time.perf_counter() - start)
                return DeleteUserResponse(
                    success=True,
                    requested_at=existing["requested_at"],
                    execute_at=existing["execute_at"],
                    status="pending",
                )

            grace = request.grace_days if request.grace_days > 0 else 30
            entry = await self.global_store.queue_deletion(request.user_id, grace_days=grace)
            await self.global_store.set_user_status(request.user_id, "pending_deletion")

            record_grpc_request("DeleteUser", "ok", time.perf_counter() - start)
            return DeleteUserResponse(
                success=True,
                requested_at=entry["requested_at"],
                execute_at=entry["execute_at"],
                status="pending",
            )
        except Exception as e:
            record_grpc_request("DeleteUser", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"DeleteUser failed: {e}", exc_info=True)
            return DeleteUserResponse(success=False, error=str(e))

    async def CancelUserDeletion(
        self,
        request: CancelUserDeletionRequest,
        context: grpc_aio.ServicerContext,
    ) -> CancelUserDeletionResponse:
        """Cancel a pending user deletion during the grace period."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(grpc.StatusCode.UNIMPLEMENTED, "User registry not configured")
            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            if not self._is_self_or_admin(request.actor, request.user_id):
                await context.abort(
                    grpc.StatusCode.PERMISSION_DENIED,
                    "CancelUserDeletion requires the user themselves or an admin actor",
                )
            ok = await self.global_store.cancel_deletion(request.user_id)
            if ok:
                await self.global_store.set_user_status(request.user_id, "active")
            record_grpc_request("CancelUserDeletion", "ok", time.perf_counter() - start)
            return CancelUserDeletionResponse(success=ok)
        except Exception as e:
            record_grpc_request("CancelUserDeletion", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"CancelUserDeletion failed: {e}", exc_info=True)
            return CancelUserDeletionResponse(success=False, error=str(e))

    async def ExportUserData(
        self,
        request: ExportUserDataRequest,
        context: grpc_aio.ServicerContext,
    ) -> ExportUserDataResponse:
        """Export a user's data across all tenants they belong to."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(grpc.StatusCode.UNIMPLEMENTED, "User registry not configured")
            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            if not self._is_self_or_admin(request.actor, request.user_id):
                await context.abort(
                    grpc.StatusCode.PERMISSION_DENIED,
                    "ExportUserData requires the user themselves or an admin actor",
                )

            memberships = await self.global_store.get_user_tenants(request.user_id)
            # Per-tenant nodes store owner_actor as "user:{id}" principal form.
            tenant_principal = (
                request.user_id
                if request.user_id.startswith("user:")
                else f"user:{request.user_id}"
            )
            tenants_out = []
            for m in memberships:
                try:
                    data = await self.canonical_store.export_user_data_for_tenant(
                        m["tenant_id"], tenant_principal, self.schema_registry
                    )
                    tenants_out.append(data)
                except Exception as tx_err:  # pragma: no cover - defensive
                    logger.warning(
                        "ExportUserData tenant %s failed: %s",
                        m["tenant_id"],
                        tx_err,
                    )

            bundle = {
                "user_id": request.user_id,
                "generated_at": int(time.time()),
                "tenants": tenants_out,
            }
            record_grpc_request("ExportUserData", "ok", time.perf_counter() - start)
            return ExportUserDataResponse(
                success=True,
                export_json=json.dumps(bundle),
            )
        except Exception as e:
            record_grpc_request("ExportUserData", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"ExportUserData failed: {e}", exc_info=True)
            return ExportUserDataResponse(success=False, error=str(e))

    async def FreezeUser(
        self,
        request: FreezeUserRequest,
        context: grpc_aio.ServicerContext,
    ) -> FreezeUserResponse:
        """Freeze or unfreeze a user (GDPR Article 18 — restrict processing)."""
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(grpc.StatusCode.UNIMPLEMENTED, "User registry not configured")
            if not request.actor:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "actor is required")
            if not request.user_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id is required")
            if not self._is_self_or_admin(request.actor, request.user_id):
                await context.abort(
                    grpc.StatusCode.PERMISSION_DENIED,
                    "FreezeUser requires the user themselves or an admin actor",
                )

            new_status = "frozen" if request.enabled else "active"
            ok = await self.global_store.set_user_status(request.user_id, new_status)
            if not ok:
                record_grpc_request("FreezeUser", "ok", time.perf_counter() - start)
                return FreezeUserResponse(success=False, error="User not found")
            record_grpc_request("FreezeUser", "ok", time.perf_counter() - start)
            return FreezeUserResponse(success=True, status=new_status)
        except Exception as e:
            record_grpc_request("FreezeUser", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"FreezeUser failed: {e}", exc_info=True)
            return FreezeUserResponse(success=False, error=str(e))

    async def GetTenantQuota(
        self,
        request: GetTenantQuotaRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetTenantQuotaResponse:
        """Return the quota config + current-period usage for a tenant.

        Intended for admin dashboards (the built-in console, Grafana
        panels, etc.). Requires ``admin`` or ``owner`` role on the
        tenant, as with the other admin RPCs. Unknown tenants get a
        zero-valued response (unlimited, unused) rather than an error —
        dashboards can then render an "unlimited" badge without a
        special-case code path.
        """
        start = time.perf_counter()
        try:
            if not self.global_store:
                await context.abort(
                    grpc.StatusCode.UNIMPLEMENTED,
                    "Quota registry not configured",
                )
            if not request.tenant_id:
                await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "tenant_id is required")
            await self._require_admin_or_owner(
                request.tenant_id, request.actor, context, "GetTenantQuota"
            )

            config = await self.global_store.get_quota_config(request.tenant_id)
            usage = await self.global_store.get_usage(request.tenant_id)

            # Period end = start of the next calendar month. Computing
            # it locally keeps dashboards honest even if the row's
            # ``period_start_ms`` is stale (no writes this period).
            from ..auth.quota_interceptor import _next_calendar_month_start_ms

            period_end_ms = _next_calendar_month_start_ms()

            cfg = config or {}
            resp = GetTenantQuotaResponse(
                tenant_id=request.tenant_id,
                max_writes_per_month=int(cfg.get("max_writes_per_month", 0) or 0),
                writes_used=int(usage.get("writes_count", 0) or 0),
                period_start_ms=int(usage.get("period_start_ms", 0) or 0),
                period_end_ms=period_end_ms,
                max_rps_sustained=int(cfg.get("max_rps_sustained", 0) or 0),
                max_rps_burst=int(cfg.get("max_rps_burst", 0) or 0),
                max_rps_per_user_sustained=int(cfg.get("max_rps_per_user_sustained", 0) or 0),
                max_rps_per_user_burst=int(cfg.get("max_rps_per_user_burst", 0) or 0),
                hard_enforce=bool(cfg.get("hard_enforce", False)),
            )
            record_grpc_request("GetTenantQuota", "ok", time.perf_counter() - start)
            return resp
        except Exception as e:
            record_grpc_request("GetTenantQuota", "error", time.perf_counter() - start)
            if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
                raise
            logger.error(f"GetTenantQuota failed: {e}", exc_info=True)
            raise


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
        auth_enabled: bool = False,
        auth_api_keys: frozenset[str] = frozenset(),
        tls_cert_file: str | None = None,
        tls_key_file: str | None = None,
        rate_limit_enabled: bool = False,
        rate_limit_rps: float = 100.0,
        rate_limit_burst: int = 200,
        global_store: Any | None = None,
    ) -> None:
        """Initialize the gRPC server.

        Args:
            servicer: EntDBServicer instance
            host: Host to bind to
            port: Port to listen on
            max_workers: Maximum concurrent RPCs
            reflection_enabled: Whether to enable gRPC reflection
            auth_enabled: Whether API key authentication is enabled
            auth_api_keys: Set of valid API keys
            tls_cert_file: Path to TLS certificate file (PEM)
            tls_key_file: Path to TLS private key file (PEM)
            rate_limit_enabled: Whether per-tenant rate limiting is enabled
            rate_limit_rps: Requests per second per tenant
            rate_limit_burst: Burst capacity per tenant
        """
        self.servicer = servicer
        self.host = host
        self.port = port
        self.max_workers = max_workers
        self.reflection_enabled = reflection_enabled
        self.auth_enabled = auth_enabled
        self.auth_api_keys = auth_api_keys
        self.tls_cert_file = tls_cert_file
        self.tls_key_file = tls_key_file
        self.rate_limit_enabled = rate_limit_enabled
        self.rate_limit_rps = rate_limit_rps
        self.rate_limit_burst = rate_limit_burst
        self.global_store = global_store
        self._server: grpc_aio.Server | None = None
        self._running = False

    async def start(self) -> None:
        """Start the gRPC server."""
        if self._running:
            logger.warning("Server already running")
            return

        # Build interceptor list
        interceptors = []
        if self.rate_limit_enabled:
            from .rate_limiter import RateLimitInterceptor

            interceptors.append(RateLimitInterceptor(self.rate_limit_rps, self.rate_limit_burst))
            logger.info(
                "Rate limiting enabled",
                extra={"rps": self.rate_limit_rps, "burst": self.rate_limit_burst},
            )
        if self.auth_enabled and self.auth_api_keys:
            from .auth import ApiKeyInterceptor

            interceptors.append(ApiKeyInterceptor(self.auth_api_keys))
            logger.info("API key authentication enabled")

        # QuotaInterceptor runs AFTER auth so quota checks can trust the
        # tenant identifier on the request. Conditional on global_store
        # being configured — a no-auth / no-quota deployment
        # (e.g. in-process tests) simply skips it.
        if self.global_store is not None:
            from ..auth.quota_interceptor import QuotaInterceptor

            interceptors.append(QuotaInterceptor(self.global_store))
            logger.info("Quota interceptor enabled (three-layer rate limits)")

        # Create async gRPC server
        self._server = grpc_aio.server(
            interceptors=interceptors,
            options=[
                ("grpc.max_send_message_length", 50 * 1024 * 1024),  # 50MB
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),  # 50MB
                ("grpc.max_concurrent_streams", self.max_workers),
            ],
        )

        # Register servicer
        add_EntDBServiceServicer_to_server(self.servicer, self._server)

        # Enable reflection for debugging tools like grpcurl
        if self.reflection_enabled:
            try:
                from grpc_reflection.v1alpha import reflection

                from .generated import entdb_pb2

                SERVICE_NAMES = (
                    entdb_pb2.DESCRIPTOR.services_by_name["EntDBService"].full_name,
                    reflection.SERVICE_NAME,
                )
                reflection.enable_server_reflection(SERVICE_NAMES, self._server)
            except ImportError:
                logger.warning("grpc-reflection not installed, skipping reflection")

        # Bind to address with optional TLS
        bind_address = f"{self.host}:{self.port}"
        if self.tls_cert_file and self.tls_key_file:
            with open(self.tls_cert_file, "rb") as f:
                cert = f.read()
            with open(self.tls_key_file, "rb") as f:
                key = f.read()
            creds = grpc.ssl_server_credentials([(key, cert)])
            self._server.add_secure_port(bind_address, creds)
            logger.info("gRPC server using TLS", extra={"port": self.port})
        else:
            self._server.add_insecure_port(bind_address)

        # Start server
        await self._server.start()
        self._running = True

        logger.info(
            f"gRPC server started on {bind_address}",
            extra={"host": self.host, "port": self.port},
        )

    async def stop(self, grace_period: float = 5.0) -> None:
        """Stop the gRPC server gracefully.

        Args:
            grace_period: Time to wait for pending RPCs to complete
        """
        if not self._running or not self._server:
            return

        logger.info("Stopping gRPC server...")
        await self._server.stop(grace_period)
        self._running = False
        logger.info("gRPC server stopped")

    async def wait_for_termination(self) -> None:
        """Wait for the server to terminate."""
        if self._server:
            await self._server.wait_for_termination()

    @property
    def is_running(self) -> bool:
        """Whether the server is running."""
        return self._running
