# SPDX-License-Identifier: MIT
"""Mailbox read surface (#568): the SDK threads target_user onto the wire
for GetNode / GetNodes / QueryNodes / SearchNodes.

A fake stub captures the outgoing request so we can assert the
target_user field is set when the mailbox-scoped methods are used, and is
empty for ordinary tenant reads (the additive, opt-in contract).
"""

from __future__ import annotations

import pytest

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._grpc_client import GrpcClient


class _FakeStub:
    def __init__(self) -> None:
        self.get_node_req: pb.GetNodeRequest | None = None
        self.get_nodes_req: pb.GetNodesRequest | None = None
        self.query_req: pb.QueryNodesRequest | None = None
        self.search_req: pb.SearchNodesRequest | None = None

    async def GetNode(self, request, timeout=None, metadata=None):
        self.get_node_req = request
        return pb.GetNodeResponse(found=False)

    async def GetNodes(self, request, timeout=None, metadata=None):
        self.get_nodes_req = request
        return pb.GetNodesResponse(nodes=[], missing_ids=list(request.node_ids))

    async def QueryNodes(self, request, timeout=None, metadata=None):
        self.query_req = request
        return pb.QueryNodesResponse(nodes=[], next_page_token="")

    async def SearchNodes(self, request, timeout=None, metadata=None):
        self.search_req = request
        return pb.SearchNodesResponse(nodes=[])


def _client(stub: _FakeStub) -> GrpcClient:
    c = GrpcClient()
    c._stub = stub
    return c


@pytest.mark.asyncio
async def test_get_node_sets_target_user():
    stub = _FakeStub()
    await _client(stub).get_node("t1", "user:alice", 7, "n1", target_user="alice")
    assert stub.get_node_req is not None
    assert stub.get_node_req.target_user == "alice"


@pytest.mark.asyncio
async def test_get_node_defaults_no_target_user():
    stub = _FakeStub()
    await _client(stub).get_node("t1", "user:alice", 7, "n1")
    assert stub.get_node_req is not None
    assert stub.get_node_req.target_user == ""


@pytest.mark.asyncio
async def test_get_nodes_sets_target_user():
    stub = _FakeStub()
    await _client(stub).get_nodes("t1", "user:alice", 7, ["n1", "n2"], target_user="alice")
    assert stub.get_nodes_req is not None
    assert stub.get_nodes_req.target_user == "alice"


@pytest.mark.asyncio
async def test_query_nodes_sets_target_user():
    stub = _FakeStub()
    await _client(stub).query_nodes("t1", "user:alice", 7, target_user="alice")
    assert stub.query_req is not None
    assert stub.query_req.target_user == "alice"


@pytest.mark.asyncio
async def test_search_nodes_sets_target_user():
    stub = _FakeStub()
    await _client(stub).search_nodes("t1", "user:alice", 7, "keyword", target_user="alice")
    assert stub.search_req is not None
    assert stub.search_req.target_user == "alice"
    assert stub.search_req.query == "keyword"
