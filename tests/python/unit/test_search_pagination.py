# SPDX-License-Identifier: MIT
"""ADR-029 FTS carve-out for GrpcClient.search_nodes (#580).

Search is relevance-ranked top-N and OFFSET-paged, NOT cursor-paged. The
client must make exactly ONE request (no auto-follow), forward
page_size + offset (page_size taking precedence over the legacy limit),
and surface has_more verbatim.
"""

from __future__ import annotations

import pytest

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._grpc_client import GrpcClient


class _FakeStub:
    def __init__(self, count: int, has_more: bool):
        self._count = count
        self._has_more = has_more
        self.calls: list[pb.SearchNodesRequest] = []

    async def SearchNodes(self, request, timeout=None, metadata=None):
        self.calls.append(request)
        return pb.SearchNodesResponse(
            has_more=self._has_more,
            nodes=[pb.Node(node_id=f"n{i}", type_id=1) for i in range(self._count)],
        )


def _client_with_stub(stub: _FakeStub) -> GrpcClient:
    c = GrpcClient()
    c._stub = stub  # bypass connect()
    c._registry = None
    return c


@pytest.mark.asyncio
async def test_search_does_not_auto_follow_and_surfaces_has_more():
    stub = _FakeStub(count=2, has_more=True)
    nodes, has_more = await _client_with_stub(stub).search_nodes(
        "acme", "user:alice", 1, "widget", page_size=2, offset=4
    )
    assert len(nodes) == 2
    assert has_more is True
    # Exactly one request — search is top-N, NOT auto-followed.
    assert len(stub.calls) == 1
    assert stub.calls[0].page_size == 2
    assert stub.calls[0].offset == 4


@pytest.mark.asyncio
async def test_search_page_size_forwarded_with_legacy_limit():
    # Both limit and page_size set: both go on the wire; the server
    # applies page_size precedence. The client just forwards them.
    stub = _FakeStub(count=1, has_more=False)
    nodes, has_more = await _client_with_stub(stub).search_nodes(
        "acme", "user:alice", 1, "widget", limit=100, page_size=1
    )
    assert len(nodes) == 1
    assert has_more is False
    assert stub.calls[0].page_size == 1
    assert stub.calls[0].limit == 100
