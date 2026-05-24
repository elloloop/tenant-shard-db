# SPDX-License-Identifier: MIT
"""ADR-029 keyset auto-follow for GrpcClient.list_users (#580).

A fake stub serves canned pages keyed by the incoming page_token; the
client must loop next_page_token to return the COMPLETE user list by
default, honour a positive `limit` cap, and fall back to a single
non-cursor request for the deprecated `offset`.
"""

from __future__ import annotations

import pytest

from entdb_sdk._generated import entdb_pb2 as pb
from entdb_sdk._grpc_client import GrpcClient


class _FakeStub:
    def __init__(self, pages: dict[int, tuple[int, str]]):
        self._pages = pages
        self.calls: list[pb.ListUsersRequest] = []

    async def ListUsers(self, request, timeout=None, metadata=None):
        self.calls.append(request)
        idx = 0 if request.page_token == "" else int(request.page_token)
        count, next_tok = self._pages[idx]
        return pb.ListUsersResponse(
            next_page_token=next_tok,
            users=[pb.UserInfo(user_id=f"u{idx}-{i}", status="active") for i in range(count)],
        )


def _client(stub: _FakeStub) -> GrpcClient:
    c = GrpcClient()
    c._stub = stub
    return c


@pytest.mark.asyncio
async def test_list_users_autofollows_to_complete_set():
    stub = _FakeStub({0: (100, "1"), 1: (40, "")})
    users = await _client(stub).list_users()
    assert len(users) == 140
    assert len(stub.calls) == 2
    assert stub.calls[0].page_token == ""
    assert stub.calls[1].page_token == "1"


@pytest.mark.asyncio
async def test_list_users_limit_caps_total():
    stub = _FakeStub({0: (100, "1"), 1: (100, "2"), 2: (100, "")})
    users = await _client(stub).list_users(limit=120)
    assert len(users) == 120
    assert len(stub.calls) == 2
    assert stub.calls[1].page_size == 20


@pytest.mark.asyncio
async def test_list_users_offset_uses_legacy_single_request():
    stub = _FakeStub({0: (10, "should-not-follow")})
    users = await _client(stub).list_users(offset=5)
    assert len(users) == 10
    assert len(stub.calls) == 1
    assert stub.calls[0].offset == 5
    assert stub.calls[0].page_token == ""
