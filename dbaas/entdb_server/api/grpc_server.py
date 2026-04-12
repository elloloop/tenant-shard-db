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
from ..sharding import ShardingConfig
from .generated import (
    AclEntry,
    Edge,
    EntDBServiceServicer,
    # Request/Response types
    ExecuteAtomicRequest,
    ExecuteAtomicResponse,
    # ACL v2
    GetConnectedNodesRequest,
    GetConnectedNodesResponse,
    GetEdgesRequest,
    GetEdgesResponse,
    GetMailboxRequest,
    GetMailboxResponse,
    GetNodeRequest,
    GetNodeResponse,
    GetNodesRequest,
    GetNodesResponse,
    GetReceiptStatusRequest,
    GetReceiptStatusResponse,
    GetSchemaRequest,
    GetSchemaResponse,
    GroupMemberRequest,
    GroupMemberResponse,
    HealthRequest,
    HealthResponse,
    ListMailboxUsersRequest,
    ListMailboxUsersResponse,
    ListSharedWithMeRequest,
    ListSharedWithMeResponse,
    ListTenantsRequest,
    ListTenantsResponse,
    MailboxItem,
    MailboxSearchResult,
    Node,
    QueryNodesRequest,
    QueryNodesResponse,
    # Data types
    Receipt,
    ReceiptStatus,
    RevokeAccessRequest,
    RevokeAccessResponse,
    SearchMailboxRequest,
    SearchMailboxResponse,
    ShareNodeRequest,
    ShareNodeResponse,
    TenantInfo,
    TransferOwnershipRequest,
    TransferOwnershipResponse,
    WaitForOffsetRequest,
    WaitForOffsetResponse,
    # User registry
    UserInfo,
    CreateUserRequest,
    CreateUserResponse,
    GetUserRequest,
    GetUserResponse,
    UpdateUserRequest,
    UpdateUserResponse,
    ListUsersRequest,
    ListUsersResponse,
    # Tenant registry
    TenantDetail,
    CreateTenantRequest,
    CreateTenantResponse,
    GetTenantRequest,
    GetTenantResponse,
    ArchiveTenantRequest,
    ArchiveTenantResponse,
    # Tenant membership
    TenantMemberInfo,
    TenantMemberRequest,
    TenantMemberResponse,
    GetTenantMembersRequest,
    GetTenantMembersResponse,
    GetUserTenantsRequest,
    GetUserTenantsResponse,
    ChangeMemberRoleRequest,
    ChangeMemberRoleResponse,
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
        mailbox_store: Per-user mailbox store
        schema_registry: Schema registry for validation
        global_store: GlobalStore for cross-tenant shared_index
    """

    def __init__(
        self,
        wal: Any,
        canonical_store: Any,
        mailbox_store: Any,
        schema_registry: Any,
        topic: str = "entdb-wal",
        sharding: ShardingConfig | None = None,
        global_store: Any | None = None,
    ) -> None:
        """Initialize the gRPC servicer.

        Args:
            wal: WAL stream instance
            canonical_store: CanonicalStore instance
            mailbox_store: MailboxStore instance
            schema_registry: SchemaRegistry instance
            topic: WAL topic name
            sharding: Sharding configuration for multi-node mode
            global_store: GlobalStore instance for user registry operations
        """
        self.wal = wal
        self.canonical_store = canonical_store
        self.mailbox_store = mailbox_store
        self.schema_registry = schema_registry
        self.topic = topic
        self._sharding = sharding
        self.global_store = global_store

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
            tenant_id, actor,
        )
        has_access = await self.canonical_store.has_node_access(
            tenant_id, actor_ids,
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

            # Convert operations to internal format
            ops = self._convert_operations(request.operations)

            # Tenant role + status enforcement.
            # ExecuteAtomic is always a write. The status check is finer
            # grained: legal_hold rejects deletes (delete_node, delete_edge)
            # but still permits creates/updates.
            has_delete = any(
                op.get("op") in ("delete_node", "delete_edge") for op in ops
            )
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
        """Convert protobuf operations to internal format."""
        result = []

        for op in operations:
            op_type = op.WhichOneof("op")

            if op_type == "create_node":
                create = op.create_node
                internal_op: dict[str, Any] = {
                    "op": "create_node",
                    "type_id": create.type_id,
                    "data": _struct_to_dict(create.data),
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
                result.append(internal_op)

            elif op_type == "update_node":
                update = op.update_node
                result.append(
                    {
                        "op": "update_node",
                        "type_id": update.type_id,
                        "id": update.id,
                        "patch": _struct_to_dict(update.patch),
                    }
                )

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
                request.context.tenant_id, request.context.actor, context,
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
                    request.context.tenant_id, request.context.actor,
                )
                has_access = await self.canonical_store.can_access(
                    request.context.tenant_id, request.node_id, actor_ids,
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
                    payload=_dict_to_struct(node.payload),
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
                request.context.tenant_id, request.context.actor, context,
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
                    request.context.tenant_id, request.context.actor,
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
                        request.context.tenant_id, node_id, actor_ids,
                    )
                    if not has_access:
                        missing_ids.append(node_id)
                        continue

                nodes.append(
                    Node(
                        tenant_id=node.tenant_id,
                        node_id=node.node_id,
                        type_id=node.type_id,
                        payload=_dict_to_struct(node.payload),
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
                request.context.tenant_id, request.context.actor, context,
            )

            # Wait for offset if requested (read-after-write consistency)
            if request.after_offset:
                timeout = request.wait_timeout_ms / 1000.0 if request.wait_timeout_ms else 30.0
                await self._wait_for_offset(
                    request.context.tenant_id, request.after_offset, timeout
                )

            filter_dict = None
            if request.filters:
                filter_dict = {f.field: json_format.MessageToDict(f.value) for f in request.filters}

            nodes = await self.canonical_store.query_nodes(
                tenant_id=request.context.tenant_id,
                type_id=request.type_id,
                filter_json=filter_dict,
                limit=request.limit or 100,
                offset=request.offset or 0,
                order_by=request.order_by or "created_at",
                descending=request.descending,
            )

            # Cross-tenant: filter to only accessible nodes
            if role == "cross_tenant":
                actor_ids = await self.canonical_store.resolve_actor_groups(
                    request.context.tenant_id, request.context.actor,
                )
                accessible = []
                for n in nodes:
                    if await self.canonical_store.can_access(
                        request.context.tenant_id, n.node_id, actor_ids,
                    ):
                        accessible.append(n)
                nodes = accessible

            proto_nodes = [
                Node(
                    tenant_id=n.tenant_id,
                    node_id=n.node_id,
                    type_id=n.type_id,
                    payload=_dict_to_struct(n.payload),
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
        """Search mailbox with full-text search."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            source_type_ids = list(request.source_type_ids) if request.source_type_ids else None
            results = await self.mailbox_store.search(
                request.context.tenant_id,
                request.user_id,
                request.query,
                limit=request.limit or 20,
                offset=request.offset or 0,
                source_type_ids=source_type_ids,
            )

            proto_results = [
                MailboxSearchResult(
                    item=MailboxItem(
                        item_id=r.item.item_id,
                        ref_id=r.item.ref_id,
                        source_type_id=r.item.source_type_id,
                        source_node_id=r.item.source_node_id,
                        thread_id=r.item.thread_id or "",
                        ts=r.item.ts,
                        state=_dict_to_struct(r.item.state),
                        snippet=r.item.snippet or "",
                        metadata=_dict_to_struct(r.item.metadata),
                    ),
                    rank=r.rank,
                    highlights=r.highlights or "",
                )
                for r in results
            ]

            record_grpc_request("SearchMailbox", "ok", time.perf_counter() - start)
            return SearchMailboxResponse(
                results=proto_results,
                has_more=len(results) == (request.limit or 20),
            )
        except Exception as e:
            record_grpc_request("SearchMailbox", "error", time.perf_counter() - start)
            logger.error(f"SearchMailbox failed: {e}", exc_info=True)
            return SearchMailboxResponse(results=[])

    async def GetMailbox(
        self,
        request: GetMailboxRequest,
        context: grpc_aio.ServicerContext,
    ) -> GetMailboxResponse:
        """Get mailbox items for a user."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            source_type_id = request.source_type_id if request.source_type_id else None
            thread_id = request.thread_id if request.thread_id else None

            items = await self.mailbox_store.list_items(
                request.context.tenant_id,
                request.user_id,
                limit=request.limit or 50,
                offset=request.offset or 0,
                source_type_id=source_type_id,
                thread_id=thread_id,
                unread_only=request.unread_only,
            )

            unread_count = await self.mailbox_store.get_unread_count(
                request.context.tenant_id,
                request.user_id,
            )

            proto_items = [
                MailboxItem(
                    item_id=item.item_id,
                    ref_id=item.ref_id,
                    source_type_id=item.source_type_id,
                    source_node_id=item.source_node_id,
                    thread_id=item.thread_id or "",
                    ts=item.ts,
                    state=_dict_to_struct(item.state),
                    snippet=item.snippet or "",
                    metadata=_dict_to_struct(item.metadata),
                )
                for item in items
            ]

            record_grpc_request("GetMailbox", "ok", time.perf_counter() - start)
            return GetMailboxResponse(
                items=proto_items,
                unread_count=unread_count,
                has_more=len(items) == (request.limit or 50),
            )
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
        """List mailbox users for a tenant."""
        await self._check_tenant(request.tenant_id, context)
        try:
            user_ids = self.mailbox_store.list_users(request.tenant_id)
            return ListMailboxUsersResponse(user_ids=user_ids)
        except Exception as e:
            logger.error(f"ListMailboxUsers failed: {e}", exc_info=True)
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
        """Convert an internal Node to a protobuf Node message."""
        return Node(
            tenant_id=n.tenant_id,
            node_id=n.node_id,
            type_id=n.type_id,
            payload_json=n.payload_json,
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
        """Share a node with an actor."""
        start = time.perf_counter()
        try:
            await self._check_tenant(request.context.tenant_id, context)
            tenant_id = request.context.tenant_id
            actor_id = request.actor_id
            permission = request.permission or "read"
            expires_at = request.expires_at if request.expires_at else None
            await self.canonical_store.share_node(
                tenant_id=tenant_id,
                node_id=request.node_id,
                actor_id=actor_id,
                permission=permission,
                granted_by=request.context.actor,
                actor_type=request.actor_type or "user",
                expires_at=expires_at,
            )
            # Update cross-tenant shared_index
            if self.global_store:
                try:
                    if actor_id.startswith("group:"):
                        members = await self.canonical_store.get_group_members(
                            tenant_id, actor_id
                        )
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
            found = await self.canonical_store.revoke_access(
                tenant_id=tenant_id,
                node_id=request.node_id,
                actor_id=actor_id,
            )
            # Update cross-tenant shared_index
            if found and self.global_store:
                try:
                    if actor_id.startswith("group:"):
                        members = await self.canonical_store.get_group_members(
                            tenant_id, actor_id
                        )
                        for member_id in members:
                            await self.global_store.remove_shared(
                                member_id, tenant_id, request.node_id
                            )
                    else:
                        await self.global_store.remove_shared(
                            actor_id, tenant_id, request.node_id
                        )
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
                tenant_id, actor,
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
                            member_actor_id, tenant_id,
                            entry["node_id"], entry["permission"],
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
                    logger.warning(f"Failed to read group access for shared_index cleanup: {gs_err}")
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
        return (
            self._is_admin_or_system(actor)
            or actor == f"user:{user_id}"
            or actor == user_id
        )

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
                request.user_id, request.email, request.name,
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
                status=status, limit=limit, offset=offset,
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
                request.tenant_id, request.user_id, request.new_role,
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
