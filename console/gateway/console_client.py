"""
gRPC client for EntDB Console.

This is a simple client that talks directly to the EntDB server
using gRPC. Unlike the SDK, this doesn't require schema definitions -
it works with raw type IDs and JSON payloads.

This is intentionally separate from the SDK to keep the Console
decoupled and able to browse any EntDB instance without schema code.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any

from google.protobuf import json_format
from grpc import aio as grpc_aio

# Import generated stubs - Console has its own copy
from entdb_sdk._generated import (
    EntDBServiceStub,
    GetEdgesRequest,
    GetMailboxRequest,
    GetNodeRequest,
    GetSchemaRequest,
    HealthRequest,
    ListMailboxUsersRequest,
    ListTenantsRequest,
    QueryNodesRequest,
    RequestContext,
    SearchMailboxRequest,
)

logger = logging.getLogger(__name__)


def _struct_to_dict(s: Any) -> dict[str, Any]:
    """Convert a protobuf Struct to a Python dict."""
    if not s or not getattr(s, "fields", None):
        return {}
    return json_format.MessageToDict(s)


@dataclass
class ConsoleNode:
    """Node data for Console display."""

    node_id: str
    type_id: int
    tenant_id: str
    payload: dict[str, Any]
    owner_actor: str
    created_at: int
    updated_at: int


@dataclass
class ConsoleEdge:
    """Edge data for Console display."""

    edge_type_id: int
    from_node_id: str
    to_node_id: str
    tenant_id: str
    props: dict[str, Any]
    created_at: int


class ConsoleClient:
    """
    Read-only gRPC client for EntDB Console.

    Provides direct access to EntDB for browsing data without
    requiring schema definitions in code.
    """

    def __init__(self, host: str, port: int) -> None:
        self._host = host
        self._port = port
        self._channel: grpc_aio.Channel | None = None
        self._stub: EntDBServiceStub | None = None
        # Cache of {tenant_id: {type_id: {field_id_int: field_name}}}.
        # Populated lazily by ``_get_id_to_name_map`` when translating
        # id-keyed payloads back to names for the Console UI.
        self._field_name_cache: dict[str, dict[int, dict[int, str]]] = {}

    async def _get_id_to_name_map(self, tenant_id: str, type_id: int) -> dict[int, str]:
        """Return a ``{field_id: field_name}`` map for ``type_id``.

        Fetches the tenant's schema on first use and caches it. The
        wire payload is field-id-keyed (per CLAUDE.md invariant #6) so
        the Console must translate ids to names locally for display.
        """
        per_tenant = self._field_name_cache.get(tenant_id)
        if per_tenant is None:
            per_tenant = {}
            self._field_name_cache[tenant_id] = per_tenant
        cached = per_tenant.get(type_id)
        if cached is not None:
            return cached
        try:
            schema = await self.get_schema(tenant_id)
        except Exception:
            return {}
        node_types = schema.get("schema", {}).get("node_types", []) or []
        for nt in node_types:
            try:
                tid = int(nt.get("type_id", 0))
            except (TypeError, ValueError):
                continue
            mapping: dict[int, str] = {}
            for f in nt.get("fields", []) or []:
                fid_raw = f.get("field_id")
                fname = f.get("name")
                if fid_raw is None or fname is None:
                    continue
                try:
                    mapping[int(fid_raw)] = str(fname)
                except (TypeError, ValueError):
                    continue
            per_tenant[tid] = mapping
        return per_tenant.get(type_id, {})

    async def _translate_payload(
        self, tenant_id: str, type_id: int, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Translate an id-keyed payload to a name-keyed payload.

        Unknown id keys are kept verbatim so partial/forward-compat
        schemas don't drop data on display.
        """
        if not payload:
            return {}
        id_to_name = await self._get_id_to_name_map(tenant_id, type_id)
        if not id_to_name:
            return dict(payload)
        out: dict[str, Any] = {}
        for key, value in payload.items():
            if isinstance(key, str) and key.isdigit():
                out[id_to_name.get(int(key), key)] = value
            else:
                out[key] = value
        return out

    async def connect(self) -> None:
        """Connect to EntDB server."""
        if self._channel:
            return

        address = f"{self._host}:{self._port}"
        self._channel = grpc_aio.insecure_channel(
            address,
            options=[
                ("grpc.max_receive_message_length", 50 * 1024 * 1024),
            ],
        )
        self._stub = EntDBServiceStub(self._channel)
        logger.info(f"Console connected to EntDB at {address}")

    async def close(self) -> None:
        """Close connection."""
        if self._channel:
            await self._channel.close()
            self._channel = None
            self._stub = None

    def _context(self, tenant_id: str, actor: str) -> RequestContext:
        return RequestContext(tenant_id=tenant_id, actor=actor)

    async def health(self) -> dict[str, Any]:
        """Check server health."""
        if not self._stub:
            raise RuntimeError("Not connected")

        response = await self._stub.Health(HealthRequest())
        return {
            "healthy": response.healthy,
            "version": response.version,
            "components": dict(response.components),
        }

    async def get_schema(self, tenant_id: str | None = None) -> dict[str, Any]:
        """Get full schema from server.

        Args:
            tenant_id: Optional tenant ID for observed schema fallback.
        """
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetSchemaRequest(tenant_id=tenant_id or "")
        response = await self._stub.GetSchema(request)
        return {
            "schema": _struct_to_dict(response.schema),
            "fingerprint": response.fingerprint,
        }

    async def get_node(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        type_id: int = 0,
    ) -> ConsoleNode | None:
        """Get a node by ID."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetNodeRequest(
            context=self._context(tenant_id, actor),
            type_id=type_id,
            node_id=node_id,
        )

        response = await self._stub.GetNode(request)
        if not response.found:
            return None

        n = response.node
        raw_payload = _struct_to_dict(n.payload)
        translated = await self._translate_payload(tenant_id, n.type_id, raw_payload)
        return ConsoleNode(
            node_id=n.node_id,
            type_id=n.type_id,
            tenant_id=n.tenant_id,
            payload=translated,
            owner_actor=n.owner_actor,
            created_at=n.created_at,
            updated_at=n.updated_at,
        )

    async def query_nodes(
        self,
        tenant_id: str,
        actor: str,
        type_id: int,
        limit: int = 100,
        offset: int = 0,
    ) -> tuple[list[ConsoleNode], bool]:
        """Query nodes by type. Returns (nodes, has_more)."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = QueryNodesRequest(
            context=self._context(tenant_id, actor),
            type_id=type_id,
            limit=limit,
            offset=offset,
        )

        response = await self._stub.QueryNodes(request)

        nodes = []
        for n in response.nodes:
            raw_payload = _struct_to_dict(n.payload)
            translated = await self._translate_payload(tenant_id, n.type_id, raw_payload)
            nodes.append(
                ConsoleNode(
                    node_id=n.node_id,
                    type_id=n.type_id,
                    tenant_id=n.tenant_id,
                    payload=translated,
                    owner_actor=n.owner_actor,
                    created_at=n.created_at,
                    updated_at=n.updated_at,
                )
            )

        return nodes, response.has_more

    async def get_edges_from(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int | None = None,
        limit: int = 100,
    ) -> list[ConsoleEdge]:
        """Get outgoing edges from a node."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetEdgesRequest(
            context=self._context(tenant_id, actor),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await self._stub.GetEdgesFrom(request)

        return [
            ConsoleEdge(
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                tenant_id=e.tenant_id,
                props=_struct_to_dict(e.props),
                created_at=e.created_at,
            )
            for e in response.edges
        ]

    async def get_edges_to(
        self,
        tenant_id: str,
        actor: str,
        node_id: str,
        edge_type_id: int | None = None,
        limit: int = 100,
    ) -> list[ConsoleEdge]:
        """Get incoming edges to a node."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetEdgesRequest(
            context=self._context(tenant_id, actor),
            node_id=node_id,
            edge_type_id=edge_type_id or 0,
            limit=limit,
        )

        response = await self._stub.GetEdgesTo(request)

        return [
            ConsoleEdge(
                edge_type_id=e.edge_type_id,
                from_node_id=e.from_node_id,
                to_node_id=e.to_node_id,
                tenant_id=e.tenant_id,
                props=_struct_to_dict(e.props),
                created_at=e.created_at,
            )
            for e in response.edges
        ]

    async def search(
        self,
        tenant_id: str,
        actor: str,
        user_id: str,
        query: str,
        limit: int = 20,
    ) -> list[dict[str, Any]]:
        """Search mailbox."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = SearchMailboxRequest(
            context=self._context(tenant_id, actor),
            user_id=user_id,
            query=query,
            limit=limit,
        )

        response = await self._stub.SearchMailbox(request)

        return [
            {
                "item": {
                    "item_id": r.item.item_id,
                    "ref_id": r.item.ref_id,
                    "source_type_id": r.item.source_type_id,
                    "source_node_id": r.item.source_node_id,
                    "snippet": r.item.snippet,
                },
                "rank": r.rank,
                "highlights": r.highlights,
            }
            for r in response.results
        ]

    async def list_tenants(self) -> list[str]:
        """List all tenants with data."""
        if not self._stub:
            raise RuntimeError("Not connected")

        response = await self._stub.ListTenants(ListTenantsRequest())
        return [t.tenant_id for t in response.tenants]

    async def list_mailbox_users(self, tenant_id: str) -> list[str]:
        """List mailbox users for a tenant."""
        if not self._stub:
            raise RuntimeError("Not connected")

        response = await self._stub.ListMailboxUsers(ListMailboxUsersRequest(tenant_id=tenant_id))
        return list(response.user_ids)

    async def get_mailbox(
        self,
        tenant_id: str,
        actor: str,
        user_id: str,
        limit: int = 50,
        offset: int = 0,
    ) -> list[dict[str, Any]]:
        """Get mailbox items for a user."""
        if not self._stub:
            raise RuntimeError("Not connected")

        request = GetMailboxRequest(
            context=self._context(tenant_id, actor),
            user_id=user_id,
            limit=limit,
            offset=offset,
        )

        response = await self._stub.GetMailbox(request)

        return [
            {
                "item_id": item.item_id,
                "ref_id": item.ref_id,
                "source_type_id": item.source_type_id,
                "source_node_id": item.source_node_id,
                "thread_id": item.thread_id,
                "ts": item.ts,
                "state": _struct_to_dict(item.state),
                "snippet": item.snippet,
            }
            for item in response.items
        ]
