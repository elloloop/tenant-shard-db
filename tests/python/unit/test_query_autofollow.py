# SPDX-License-Identifier: MIT
"""ADR-029 keyset auto-follow in GrpcClient.query_nodes (#564, PR-4).

A fake stub serves canned pages keyed by the incoming page_token; the
client must loop next_page_token to return the COMPLETE set by default,
honour a positive `limit` as a total cap, and fall back to a single
non-cursor request when the deprecated `offset` is used.
"""

from __future__ import annotations

import pytest

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._grpc_client import GrpcClient


class _FakeStub:
    """Serves pages indexed by token: page 0 is token "", page k is "k"."""

    def __init__(self, pages: dict[int, tuple[int, str]]):
        self._pages = pages
        self.calls: list[pb.QueryNodesRequest] = []

    async def QueryNodes(self, request, timeout=None, metadata=None):
        self.calls.append(request)
        idx = 0 if request.page_token == "" else int(request.page_token)
        count, next_tok = self._pages[idx]
        return pb.QueryNodesResponse(
            next_page_token=next_tok,
            nodes=[pb.Node(node_id=f"n{idx}-{i}", type_id=1) for i in range(count)],
        )


def _client_with_stub(stub: _FakeStub) -> GrpcClient:
    c = GrpcClient()
    c._stub = stub  # bypass connect()
    c._registry = None
    return c


@pytest.mark.asyncio
async def test_autofollows_to_complete_set():
    # page0: 100 rows -> token "1"; page1: 50 rows -> end.
    stub = _FakeStub({0: (100, "1"), 1: (50, "")})
    nodes, has_more = await _client_with_stub(stub).query_nodes("acme", "user:alice", 1)
    assert len(nodes) == 150, "auto-follow must return the complete set"
    assert has_more is False
    assert len(stub.calls) == 2
    # Cursor advances; first page fences read-after-write, later pages do not.
    assert stub.calls[0].page_token == ""
    assert stub.calls[1].page_token == "1"


@pytest.mark.asyncio
async def test_limit_caps_total_and_stops_early():
    stub = _FakeStub({0: (100, "1"), 1: (100, "2"), 2: (100, "")})
    nodes, has_more = await _client_with_stub(stub).query_nodes("acme", "user:alice", 1, limit=120)
    assert len(nodes) == 120
    assert has_more is True
    assert len(stub.calls) == 2, "should stop once the cap is reached"
    # Second page only needs the remaining 20 rows.
    assert stub.calls[1].page_size == 20


@pytest.mark.asyncio
async def test_offset_uses_legacy_single_request():
    # A token is offered but the legacy offset path must NOT follow it.
    stub = _FakeStub({0: (10, "should-not-follow")})
    nodes, has_more = await _client_with_stub(stub).query_nodes("acme", "user:alice", 1, offset=5)
    assert len(nodes) == 10
    assert len(stub.calls) == 1
    assert stub.calls[0].offset == 5
    assert stub.calls[0].page_token == ""


@pytest.mark.asyncio
async def test_single_page_no_token_returns_once():
    stub = _FakeStub({0: (7, "")})
    nodes, has_more = await _client_with_stub(stub).query_nodes("acme", "user:alice", 1)
    assert len(nodes) == 7
    assert has_more is False
    assert len(stub.calls) == 1
