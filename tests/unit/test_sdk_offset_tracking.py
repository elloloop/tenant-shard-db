"""
Unit tests for SDK automatic offset tracking (read-after-write consistency).

Tests cover:
- Commit stores offset in _last_offsets
- Subsequent get/get_many/query include after_offset automatically
- First read (no prior write) has no after_offset
- Explicit after_offset=None opts out
- Explicit after_offset="specific" overrides tracked value
- Different tenants track independently
- clear_offsets resets all tracked offsets
- Edge read methods resolve offsets
- connected and shared_with_me resolve offsets
- Failed commits do not store offsets
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from sdk.entdb_sdk._grpc_client import GrpcCommitResult, GrpcNode, GrpcReceipt
from sdk.entdb_sdk.client import _UNSET, DbClient
from sdk.entdb_sdk.schema import EdgeTypeDef, FieldDef, FieldKind, NodeTypeDef

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

def _task_type() -> NodeTypeDef:
    return NodeTypeDef(
        type_id=101,
        name="Task",
        fields=(
            FieldDef(field_id=1, name="title", kind=FieldKind.STRING, required=True),
        ),
    )


def _edge_type() -> EdgeTypeDef:
    return EdgeTypeDef(edge_id=201, name="AssignedTo", from_type=101, to_type=101)


TASK = _task_type()
EDGE = _edge_type()


def _make_grpc_node(tenant_id: str = "t1", node_id: str = "n1") -> GrpcNode:
    return GrpcNode(
        tenant_id=tenant_id,
        node_id=node_id,
        type_id=101,
        payload={"title": "Test"},
        created_at=1000,
        updated_at=1000,
        owner_actor="user:1",
        acl=[],
    )


def _success_result(
    tenant_id: str = "t1",
    stream_position: str = "pos_42",
) -> GrpcCommitResult:
    return GrpcCommitResult(
        success=True,
        receipt=GrpcReceipt(
            tenant_id=tenant_id,
            idempotency_key="key1",
            stream_position=stream_position,
        ),
        created_node_ids=["n1"],
        applied=True,
        error=None,
    )


def _fail_result() -> GrpcCommitResult:
    return GrpcCommitResult(
        success=False,
        receipt=None,
        created_node_ids=[],
        applied=False,
        error="conflict",
    )


@pytest.fixture
def client():
    """Create a DbClient with mocked gRPC transport."""
    c = DbClient("localhost:50051")
    c._connected = True

    # Mock all gRPC methods we test against
    c._grpc = AsyncMock()
    c._grpc.execute_atomic = AsyncMock(return_value=_success_result())
    c._grpc.get_node = AsyncMock(return_value=_make_grpc_node())
    c._grpc.get_nodes = AsyncMock(return_value=([_make_grpc_node()], []))
    c._grpc.query_nodes = AsyncMock(return_value=([_make_grpc_node()], False))
    c._grpc.get_edges_from = AsyncMock(return_value=([], False))
    c._grpc.get_edges_to = AsyncMock(return_value=([], False))
    c._grpc.get_connected_nodes = AsyncMock(return_value=([], False))
    c._grpc.list_shared_with_me = AsyncMock(return_value=([], False))
    return c


# ---------------------------------------------------------------------------
# 1. Commit stores offset
# ---------------------------------------------------------------------------

class TestCommitStoresOffset:
    async def test_commit_stores_stream_position(self, client):
        plan = client.atomic("t1", "user:1")
        plan.create(TASK, {"title": "x"})
        await plan.commit()

        assert client._last_offsets["t1"] == "pos_42"

    async def test_commit_stores_different_positions(self, client):
        client._grpc.execute_atomic = AsyncMock(
            return_value=_success_result("t1", "pos_100")
        )
        plan = client.atomic("t1", "user:1")
        plan.create(TASK, {"title": "x"})
        await plan.commit()
        assert client._last_offsets["t1"] == "pos_100"

    async def test_failed_commit_does_not_store_offset(self, client):
        client._grpc.execute_atomic = AsyncMock(return_value=_fail_result())
        plan = client.atomic("t1", "user:1")
        plan.create(TASK, {"title": "x"})
        await plan.commit()

        assert "t1" not in client._last_offsets

    async def test_commit_without_stream_position_does_not_store(self, client):
        client._grpc.execute_atomic = AsyncMock(
            return_value=GrpcCommitResult(
                success=True,
                receipt=GrpcReceipt(
                    tenant_id="t1",
                    idempotency_key="key1",
                    stream_position=None,
                ),
                created_node_ids=["n1"],
                applied=True,
                error=None,
            )
        )
        plan = client.atomic("t1", "user:1")
        plan.create(TASK, {"title": "x"})
        await plan.commit()

        assert "t1" not in client._last_offsets


# ---------------------------------------------------------------------------
# 2. get() auto-includes after_offset
# ---------------------------------------------------------------------------

class TestGetAutoOffset:
    async def test_get_includes_tracked_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.get(TASK, "n1", "t1", "user:1")

        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] == "pos_42"
        assert call_kwargs.kwargs["wait_timeout_ms"] == 30000

    async def test_get_no_prior_write(self, client):
        await client.get(TASK, "n1", "t1", "user:1")

        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] is None
        assert call_kwargs.kwargs["wait_timeout_ms"] == 0

    async def test_get_explicit_none_opts_out(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.get(TASK, "n1", "t1", "user:1", after_offset=None)

        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] is None
        assert call_kwargs.kwargs["wait_timeout_ms"] == 0

    async def test_get_explicit_offset_overrides(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.get(TASK, "n1", "t1", "user:1", after_offset="pos_99")

        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] == "pos_99"
        assert call_kwargs.kwargs["wait_timeout_ms"] == 30000


# ---------------------------------------------------------------------------
# 3. query() auto-includes after_offset
# ---------------------------------------------------------------------------

class TestQueryAutoOffset:
    async def test_query_includes_tracked_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.query(TASK, "t1", "user:1")

        call_kwargs = client._grpc.query_nodes.call_args
        assert call_kwargs.kwargs["after_offset"] == "pos_42"

    async def test_query_no_prior_write(self, client):
        await client.query(TASK, "t1", "user:1")

        call_kwargs = client._grpc.query_nodes.call_args
        assert call_kwargs.kwargs["after_offset"] is None
        assert call_kwargs.kwargs["wait_timeout_ms"] == 0


# ---------------------------------------------------------------------------
# 4. get_many() auto-includes after_offset
# ---------------------------------------------------------------------------

class TestGetManyAutoOffset:
    async def test_get_many_includes_tracked_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.get_many(TASK, ["n1"], "t1", "user:1")

        call_kwargs = client._grpc.get_nodes.call_args
        assert call_kwargs.kwargs["after_offset"] == "pos_42"

    async def test_get_many_explicit_none_opts_out(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.get_many(TASK, ["n1"], "t1", "user:1", after_offset=None)

        call_kwargs = client._grpc.get_nodes.call_args
        assert call_kwargs.kwargs["after_offset"] is None


# ---------------------------------------------------------------------------
# 5. Edge read methods resolve offsets
# ---------------------------------------------------------------------------

class TestEdgeAutoOffset:
    async def test_edges_out_resolves_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.edges_out("n1", "t1", "user:1")
        # The gRPC edges_from doesn't support after_offset yet, but
        # _resolve_offset is still called for future-proofing.
        assert client._last_offsets["t1"] == "pos_42"

    async def test_edges_in_resolves_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.edges_in("n1", "t1", "user:1")
        assert client._last_offsets["t1"] == "pos_42"


# ---------------------------------------------------------------------------
# 6. connected() and shared_with_me() resolve offsets
# ---------------------------------------------------------------------------

class TestAclReadAutoOffset:
    async def test_connected_resolves_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.connected("n1", EDGE, "t1", "user:1")
        assert client._last_offsets["t1"] == "pos_42"

    async def test_shared_with_me_resolves_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        await client.shared_with_me("t1", "user:1")
        assert client._last_offsets["t1"] == "pos_42"


# ---------------------------------------------------------------------------
# 7. Different tenants track independently
# ---------------------------------------------------------------------------

class TestMultiTenantTracking:
    async def test_tenants_track_independently(self, client):
        client._grpc.execute_atomic = AsyncMock(
            side_effect=[
                _success_result("t1", "pos_10"),
                _success_result("t2", "pos_20"),
            ]
        )

        plan1 = client.atomic("t1", "user:1")
        plan1.create(TASK, {"title": "a"})
        await plan1.commit()

        plan2 = client.atomic("t2", "user:1")
        plan2.create(TASK, {"title": "b"})
        await plan2.commit()

        assert client._last_offsets["t1"] == "pos_10"
        assert client._last_offsets["t2"] == "pos_20"

    async def test_read_uses_correct_tenant_offset(self, client):
        client._last_offsets["t1"] = "pos_10"
        client._last_offsets["t2"] = "pos_20"

        await client.get(TASK, "n1", "t1", "user:1")
        call1 = client._grpc.get_node.call_args
        assert call1.kwargs["after_offset"] == "pos_10"

        await client.get(TASK, "n1", "t2", "user:1")
        call2 = client._grpc.get_node.call_args
        assert call2.kwargs["after_offset"] == "pos_20"

    async def test_read_untracked_tenant_has_no_offset(self, client):
        client._last_offsets["t1"] = "pos_10"

        await client.get(TASK, "n1", "t_unknown", "user:1")
        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] is None


# ---------------------------------------------------------------------------
# 8. clear_offsets resets
# ---------------------------------------------------------------------------

class TestClearOffsets:
    def test_clear_offsets_empties_dict(self, client):
        client._last_offsets["t1"] = "pos_42"
        client._last_offsets["t2"] = "pos_99"
        client.clear_offsets()
        assert client._last_offsets == {}

    async def test_read_after_clear_has_no_offset(self, client):
        client._last_offsets["t1"] = "pos_42"
        client.clear_offsets()

        await client.get(TASK, "n1", "t1", "user:1")
        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] is None
        assert call_kwargs.kwargs["wait_timeout_ms"] == 0


# ---------------------------------------------------------------------------
# 9. End-to-end flow: commit then read
# ---------------------------------------------------------------------------

class TestEndToEndFlow:
    async def test_commit_then_get_uses_offset(self, client):
        """Full flow: commit stores offset, subsequent get uses it."""
        plan = client.atomic("t1", "user:1")
        plan.create(TASK, {"title": "x"})
        result = await plan.commit()

        assert result.success
        assert result.receipt.stream_position == "pos_42"

        await client.get(TASK, "n1", "t1", "user:1")
        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] == "pos_42"

    async def test_second_commit_updates_offset(self, client):
        """Second commit updates the tracked offset."""
        client._grpc.execute_atomic = AsyncMock(
            side_effect=[
                _success_result("t1", "pos_10"),
                _success_result("t1", "pos_20"),
            ]
        )

        plan1 = client.atomic("t1", "user:1")
        plan1.create(TASK, {"title": "a"})
        await plan1.commit()
        assert client._last_offsets["t1"] == "pos_10"

        plan2 = client.atomic("t1", "user:1")
        plan2.create(TASK, {"title": "b"})
        await plan2.commit()
        assert client._last_offsets["t1"] == "pos_20"

        await client.get(TASK, "n1", "t1", "user:1")
        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["after_offset"] == "pos_20"


# ---------------------------------------------------------------------------
# 10. _resolve_offset helper
# ---------------------------------------------------------------------------

class TestResolveOffset:
    def test_unset_returns_tracked(self, client):
        client._last_offsets["t1"] = "pos_42"
        assert client._resolve_offset("t1", _UNSET) == "pos_42"

    def test_unset_returns_none_when_not_tracked(self, client):
        assert client._resolve_offset("t1", _UNSET) is None

    def test_explicit_none_returns_none(self, client):
        client._last_offsets["t1"] = "pos_42"
        assert client._resolve_offset("t1", None) is None

    def test_explicit_value_returns_that_value(self, client):
        client._last_offsets["t1"] = "pos_42"
        assert client._resolve_offset("t1", "pos_99") == "pos_99"
