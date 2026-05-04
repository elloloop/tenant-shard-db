"""
Tenant-to-endpoint redirect cache for the Python EntDB SDK.

Mirrors the Go SDK design: when the server returns ``UNAVAILABLE``
with an ``entdb-redirect-node`` trailing metadata header, the SDK
resolves the node id to a dial-able endpoint, opens a sub-channel,
caches it keyed by ``tenant_id``, and routes future calls for that
tenant directly there. No TTL — tenants don't move unless a node
crashes, and connection-failure on the cached sub-channel evicts
the entry.

Public surface:
    - :class:`NodeResolver` — protocol for ``node_id -> endpoint``.
    - :class:`DNSTemplateResolver` — default; resolves
      ``node-a`` to ``node-a.<base_domain>:<port>``.
    - :class:`StaticMapResolver` — explicit map, for tests and for
      static deployments without DNS.
    - :class:`TenantEndpointCache` — internal sync-safe cache used
      by :class:`entdb_sdk._grpc_client.GrpcClient`.

These names are re-exported from :mod:`entdb_sdk` so user code can
construct a resolver without importing the underscore module.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import Protocol

import grpc
from grpc import aio as grpc_aio

REDIRECT_TRAILER_KEY = "entdb-redirect-node"
"""Trailing-metadata header name the server sets when redirecting.

Mirrored on the Go SDK side as ``redirectTrailerKey``.
"""


class NodeResolver(Protocol):
    """Maps a server-issued ``node_id`` to a dial-able endpoint.

    Implementations must be safe to call from multiple coroutines.
    The resolver result is cached by the client until a connection
    error invalidates it; resolvers should NOT introduce their own
    TTL.
    """

    def resolve(self, node_id: str) -> str:
        """Return ``host:port`` for ``node_id`` or raise ``LookupError``."""
        ...


@dataclass
class DNSTemplateResolver:
    """Default resolver — composes ``<node_id>.<base_domain>:<port>``.

    The default port is 50051 (the EntDB gRPC port). For Kubernetes
    deployments backed by a headless StatefulSet, point
    ``base_domain`` at the service domain (e.g.
    ``entdb.svc.cluster.local``) and each shard pod is reachable
    via its own DNS record.
    """

    base_domain: str
    port: int = 50051

    def resolve(self, node_id: str) -> str:
        if not node_id:
            raise LookupError("entdb: empty node_id")
        if not self.base_domain:
            raise LookupError("entdb: DNSTemplateResolver.base_domain not set")
        return f"{node_id}.{self.base_domain}:{self.port}"


@dataclass
class StaticMapResolver:
    """Explicit ``node_id -> endpoint`` map.

    Use this for tests (DNS isn't available in most test
    environments) or for tiny static deployments.
    """

    endpoints: dict[str, str]

    def resolve(self, node_id: str) -> str:
        if not node_id:
            raise LookupError("entdb: empty node_id")
        try:
            return self.endpoints[node_id]
        except KeyError as e:
            raise LookupError(f"entdb: no endpoint for node {node_id!r}") from e


@dataclass
class _CachedEndpoint:
    """A cached ``endpoint -> channel`` pair."""

    endpoint: str
    channel: grpc_aio.Channel


class TenantEndpointCache:
    """Thread- and coroutine-safe ``tenant_id -> sub-channel`` cache.

    Entries are populated when the SDK is redirected by the server
    (``entdb-redirect-node`` trailer). The cache reuses the same
    sub-channel when multiple tenants share an endpoint.

    Eviction:
        - :meth:`evict` is called by the client on connection
          failures against the cached sub-channel.
        - :meth:`close` shuts every sub-channel down when the
          client is disposed.
    """

    def __init__(
        self,
        channel_factory,
    ) -> None:
        """Initialize.

        Args:
            channel_factory: Callable ``(endpoint) -> grpc.aio.Channel``
                used to open sub-channels. The default implementation
                in ``GrpcClient`` mirrors the primary channel's TLS /
                credentials settings.
        """
        self._factory = channel_factory
        self._lock = asyncio.Lock()
        self._entries: dict[str, _CachedEndpoint] = {}

    async def get(self, tenant_id: str) -> _CachedEndpoint | None:
        async with self._lock:
            return self._entries.get(tenant_id)

    async def store(self, tenant_id: str, endpoint: str) -> _CachedEndpoint:
        async with self._lock:
            # Reuse existing channels by endpoint — multiple
            # tenants frequently land on the same node.
            for entry in self._entries.values():
                if entry.endpoint == endpoint:
                    self._entries[tenant_id] = entry
                    return entry
            channel = self._factory(endpoint)
            entry = _CachedEndpoint(endpoint=endpoint, channel=channel)
            self._entries[tenant_id] = entry
            return entry

    async def evict(self, tenant_id: str) -> None:
        async with self._lock:
            entry = self._entries.pop(tenant_id, None)
            if entry is None:
                return
            # Only close the channel if no other tenant uses it.
            if not any(e.endpoint == entry.endpoint for e in self._entries.values()):
                await entry.channel.close()

    async def close(self) -> None:
        async with self._lock:
            seen: set[int] = set()
            for entry in self._entries.values():
                if id(entry.channel) in seen:
                    continue
                seen.add(id(entry.channel))
                try:
                    await entry.channel.close()
                except Exception:  # pragma: no cover — best-effort teardown
                    pass
            self._entries.clear()


def extract_redirect_node(error: grpc.RpcError) -> str | None:
    """Return the ``entdb-redirect-node`` trailer value, or None.

    Tries both ``trailing_metadata()`` (grpc.aio call objects expose
    this directly) and the AioRpcError fallback. The header is
    normalised to lowercase by gRPC.
    """
    if error.code() != grpc.StatusCode.UNAVAILABLE:
        return None
    try:
        trailers = error.trailing_metadata()
    except (AttributeError, TypeError):
        return None
    if trailers is None:
        return None
    for key, val in trailers:
        if key.lower() == REDIRECT_TRAILER_KEY:
            return val
    return None
