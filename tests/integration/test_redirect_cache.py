"""
End-to-end integration test for SDK-side tenant redirect caching.

Spins up two real ``EntDBServicer`` instances over bufconn-style
gRPC AIO servers — node-a and node-b. Tenant ``acme-2`` is
assigned to node-b; the SDK is initially pointed at node-a, with a
:class:`StaticMapResolver` that knows how to reach node-b.

Expected flow:

  1. SDK calls GetNode against node-a.
  2. Server's ``_check_tenant`` aborts with UNAVAILABLE and the
     ``entdb-redirect-node`` trailer.
  3. SDK's redirect cache resolves ``node-b`` via the static map,
     opens a sub-channel, caches it, and re-issues the call.
  4. node-b serves the request normally.
  5. The next call for the same tenant skips node-a entirely.
"""

from __future__ import annotations

import asyncio
import contextlib
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.api.generated import (
    add_EntDBServiceServicer_to_server,
)
from dbaas.entdb_server.api.grpc_server import EntDBServicer
from dbaas.entdb_server.sharding import ShardingConfig
from sdk.entdb_sdk._grpc_client import GrpcClient
from sdk.entdb_sdk._redirect_cache import StaticMapResolver


def _free_port() -> int:
    import socket

    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


async def _start_servicer(node_id: str, owned_tenants: set[str], registry: dict[str, str]):
    """Start a real EntDBServicer on a free TCP port and return (server, port, servicer)."""
    sharding = ShardingConfig(
        node_id=node_id,
        assigned_tenants=frozenset(owned_tenants),
        tenant_registry=registry,
    )

    canonical = MagicMock()
    canonical.initialize_tenant = AsyncMock()
    # GetNode hits canonical_store.get_node when sharding allows it.
    # Return a synthetic proto-shaped mock so the response succeeds.
    fake_node = MagicMock()
    fake_node.tenant_id = "acme-2"
    fake_node.node_id = "n-1"
    fake_node.type_id = 1
    fake_node.payload.fields = {}
    fake_node.created_at = 1000
    fake_node.updated_at = 1000
    fake_node.owner_actor = "user:alice"
    fake_node.acl = []
    canonical.get_node = AsyncMock(return_value=fake_node)

    servicer = EntDBServicer(
        wal=MagicMock(),
        canonical_store=canonical,
        schema_registry=MagicMock(),
        sharding=sharding,
    )

    server = grpc.aio.server()
    add_EntDBServiceServicer_to_server(servicer, server)
    port = _free_port()
    server.add_insecure_port(f"127.0.0.1:{port}")
    await server.start()
    return server, port, servicer


@pytest.mark.integration
@pytest.mark.asyncio
async def test_sdk_follows_redirect_and_caches_endpoint():
    registry = {"acme-2": "node-b"}
    server_a, port_a, _ = await _start_servicer("node-a", {"acme-1"}, registry)
    server_b, port_b, _ = await _start_servicer("node-b", {"acme-2"}, registry)

    try:
        resolver = StaticMapResolver(
            endpoints={"node-b": f"127.0.0.1:{port_b}", "node-a": f"127.0.0.1:{port_a}"}
        )
        client = GrpcClient(
            host="127.0.0.1",
            port=port_a,
            node_resolver=resolver,
            max_retries=0,  # one retry attempt is enough; the redirect path is separate
        )
        await client.connect()
        try:
            # First call: node-a redirects to node-b; SDK retries
            # transparently and returns the response from node-b.
            node = await client.get_node(
                tenant_id="acme-2",
                actor="user:alice",
                type_id=1,
                node_id="n-1",
            )
            assert node is not None
            assert node.tenant_id == "acme-2"
            assert node.node_id == "n-1"

            # Second call for the same tenant must skip node-a — the
            # cache routes us directly to node-b. We assert this by
            # checking the cache state.
            assert client._redirect_cache is not None
            cached = await client._redirect_cache.get("acme-2")
            assert cached is not None
            assert cached.endpoint == f"127.0.0.1:{port_b}"

            # Issue another call; it must succeed without an extra
            # redirect (otherwise we'd be hammering node-a).
            node2 = await client.get_node(
                tenant_id="acme-2",
                actor="user:alice",
                type_id=1,
                node_id="n-1",
            )
            assert node2 is not None
        finally:
            await client.close()
    finally:
        await server_a.stop(grace=None)
        await server_b.stop(grace=None)


@pytest.mark.integration
@pytest.mark.asyncio
async def test_sdk_without_resolver_propagates_unavailable():
    """Without a NodeResolver the SDK surfaces UNAVAILABLE as-is."""
    registry = {"acme-2": "node-b"}
    server_a, port_a, _ = await _start_servicer("node-a", {"acme-1"}, registry)

    try:
        client = GrpcClient(host="127.0.0.1", port=port_a, max_retries=0)
        await client.connect()
        try:
            with contextlib.suppress(asyncio.CancelledError):
                with pytest.raises(grpc.RpcError) as exc_info:
                    await client.get_node(
                        tenant_id="acme-2",
                        actor="user:alice",
                        type_id=1,
                        node_id="n-1",
                    )
                assert exc_info.value.code() == grpc.StatusCode.UNAVAILABLE
        finally:
            await client.close()
    finally:
        await server_a.stop(grace=None)
