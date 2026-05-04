"""
Unit tests for the Python SDK's tenant-redirect cache and resolvers.

The corresponding integration test (``test_redirect_cache.py``)
exercises the full flow against a real gRPC server; here we cover
the pure-Python pieces — resolvers, cache eviction, trailer
extraction.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from sdk.entdb_sdk._redirect_cache import (
    REDIRECT_TRAILER_KEY,
    DNSTemplateResolver,
    StaticMapResolver,
    TenantEndpointCache,
    extract_redirect_node,
)

# ── Resolvers ────────────────────────────────────────────────────────


class TestDNSTemplateResolver:
    def test_default_port(self):
        r = DNSTemplateResolver(base_domain="entdb.svc.cluster.local")
        assert r.resolve("node-a") == "node-a.entdb.svc.cluster.local:50051"

    def test_custom_port(self):
        r = DNSTemplateResolver(base_domain="example.internal", port=9000)
        assert r.resolve("node-b") == "node-b.example.internal:9000"

    def test_rejects_empty_node(self):
        r = DNSTemplateResolver(base_domain="example.com")
        with pytest.raises(LookupError):
            r.resolve("")

    def test_rejects_empty_base_domain(self):
        r = DNSTemplateResolver(base_domain="")
        with pytest.raises(LookupError):
            r.resolve("node-a")


class TestStaticMapResolver:
    def test_hit(self):
        r = StaticMapResolver(endpoints={"node-a": "1.2.3.4:50051"})
        assert r.resolve("node-a") == "1.2.3.4:50051"

    def test_miss_raises(self):
        r = StaticMapResolver(endpoints={"node-a": "1.2.3.4:50051"})
        with pytest.raises(LookupError):
            r.resolve("node-z")

    def test_empty_node_raises(self):
        r = StaticMapResolver(endpoints={})
        with pytest.raises(LookupError):
            r.resolve("")


# ── Trailer extraction ──────────────────────────────────────────────


class TestExtractRedirectNode:
    def test_returns_node_id_on_unavailable_with_trailer(self):
        err = MagicMock()
        err.code.return_value = grpc.StatusCode.UNAVAILABLE
        err.trailing_metadata.return_value = [(REDIRECT_TRAILER_KEY, "node-b")]
        assert extract_redirect_node(err) == "node-b"

    def test_returns_none_when_not_unavailable(self):
        err = MagicMock()
        err.code.return_value = grpc.StatusCode.NOT_FOUND
        err.trailing_metadata.return_value = [(REDIRECT_TRAILER_KEY, "node-b")]
        assert extract_redirect_node(err) is None

    def test_returns_none_when_no_trailer(self):
        err = MagicMock()
        err.code.return_value = grpc.StatusCode.UNAVAILABLE
        err.trailing_metadata.return_value = []
        assert extract_redirect_node(err) is None

    def test_returns_none_when_trailer_missing_key(self):
        err = MagicMock()
        err.code.return_value = grpc.StatusCode.UNAVAILABLE
        err.trailing_metadata.return_value = [("some-other", "value")]
        assert extract_redirect_node(err) is None

    def test_handles_missing_trailers_method(self):
        err = MagicMock()
        err.code.return_value = grpc.StatusCode.UNAVAILABLE
        err.trailing_metadata.side_effect = AttributeError("no trailers")
        assert extract_redirect_node(err) is None


# ── Cache ───────────────────────────────────────────────────────────


@pytest.mark.asyncio
class TestTenantEndpointCache:
    async def test_store_and_get(self):
        ch = MagicMock()
        ch.close = AsyncMock()
        factory = MagicMock(return_value=ch)
        cache = TenantEndpointCache(channel_factory=factory)

        entry = await cache.store("tenant-a", "1.2.3.4:50051")
        assert entry.endpoint == "1.2.3.4:50051"
        assert entry.channel is ch
        factory.assert_called_once_with("1.2.3.4:50051")

        got = await cache.get("tenant-a")
        assert got is entry

    async def test_get_unknown_returns_none(self):
        cache = TenantEndpointCache(channel_factory=MagicMock())
        assert await cache.get("missing") is None

    async def test_reuses_channel_for_same_endpoint(self):
        ch1 = MagicMock()
        ch1.close = AsyncMock()
        ch2 = MagicMock()
        ch2.close = AsyncMock()
        factory = MagicMock(side_effect=[ch1, ch2])
        cache = TenantEndpointCache(channel_factory=factory)

        await cache.store("tenant-a", "1.2.3.4:50051")
        await cache.store("tenant-b", "1.2.3.4:50051")

        # Only one channel created — the second store reuses ch1.
        assert factory.call_count == 1
        a = await cache.get("tenant-a")
        b = await cache.get("tenant-b")
        assert a is b

    async def test_evict_closes_channel_when_last_tenant_leaves(self):
        ch = MagicMock()
        ch.close = AsyncMock()
        factory = MagicMock(return_value=ch)
        cache = TenantEndpointCache(channel_factory=factory)

        await cache.store("tenant-a", "1.2.3.4:50051")
        await cache.evict("tenant-a")

        ch.close.assert_awaited_once()
        assert await cache.get("tenant-a") is None

    async def test_evict_keeps_channel_when_other_tenants_share_it(self):
        ch = MagicMock()
        ch.close = AsyncMock()
        factory = MagicMock(return_value=ch)
        cache = TenantEndpointCache(channel_factory=factory)

        await cache.store("tenant-a", "1.2.3.4:50051")
        await cache.store("tenant-b", "1.2.3.4:50051")
        await cache.evict("tenant-a")

        ch.close.assert_not_awaited()
        assert await cache.get("tenant-b") is not None

    async def test_evict_unknown_is_no_op(self):
        cache = TenantEndpointCache(channel_factory=MagicMock())
        await cache.evict("missing")  # must not raise

    async def test_close_tears_down_all_unique_channels(self):
        ch1 = MagicMock()
        ch1.close = AsyncMock()
        ch2 = MagicMock()
        ch2.close = AsyncMock()
        # Two distinct endpoints get distinct channels.
        factory = MagicMock(side_effect=[ch1, ch2])
        cache = TenantEndpointCache(channel_factory=factory)

        await cache.store("tenant-a", "1.2.3.4:50051")
        await cache.store("tenant-b", "5.6.7.8:50051")
        await cache.close()

        ch1.close.assert_awaited_once()
        ch2.close.assert_awaited_once()
        assert await cache.get("tenant-a") is None
