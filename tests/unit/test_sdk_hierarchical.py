"""
Unit tests for SDK hierarchical scope API (TenantScope, ActorScope, ScopedPlan).

Tests cover:
- db.tenant("t").actor("a") returns ActorScope
- Each ActorScope method delegates to the flat DbClient method
- ScopedPlan auto-fills tenant/actor from scope
- ScopedPlan chaining works
- Error when client is not connected
- TenantScope properties
- ActorScope properties
"""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from sdk.entdb_sdk._grpc_client import Edge, GrpcCommitResult, GrpcReceipt, Node
from sdk.entdb_sdk.client import DbClient
from sdk.entdb_sdk.schema import EdgeTypeDef, FieldDef, FieldKind, NodeTypeDef
from sdk.entdb_sdk.scope import ActorScope, ScopedPlan, TenantScope

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _task_type() -> NodeTypeDef:
    return NodeTypeDef(
        type_id=101,
        name="Task",
        fields=(FieldDef(field_id=1, name="title", kind=FieldKind.STRING, required=True),),
    )


def _edge_type() -> EdgeTypeDef:
    return EdgeTypeDef(edge_id=201, name="AssignedTo", from_type=101, to_type=101)


TASK = _task_type()
EDGE = _edge_type()


def _make_node(node_id: str = "n1") -> Node:
    return Node(
        tenant_id="t1",
        node_id=node_id,
        type_id=101,
        payload={"title": "Test"},
        created_at=1000,
        updated_at=1000,
        owner_actor="user:1",
        acl=[],
    )


def _make_edge() -> Edge:
    return Edge(
        tenant_id="t1",
        edge_type_id=201,
        from_node_id="n1",
        to_node_id="n2",
        props={},
        created_at=1000,
    )


def _success_result() -> GrpcCommitResult:
    return GrpcCommitResult(
        success=True,
        receipt=GrpcReceipt(
            tenant_id="t1",
            idempotency_key="key1",
            stream_position="pos_1",
        ),
        created_node_ids=["n1"],
        applied=True,
        error=None,
    )


@pytest.fixture
def client():
    """Create a DbClient with mocked gRPC transport."""
    c = DbClient("localhost:50051")
    c._connected = True

    c._grpc = AsyncMock()
    c._grpc.execute_atomic = AsyncMock(return_value=_success_result())
    c._grpc.get_node = AsyncMock(return_value=_make_node())
    c._grpc.get_nodes = AsyncMock(return_value=([_make_node()], []))
    c._grpc.query_nodes = AsyncMock(return_value=([_make_node()], False))
    c._grpc.get_edges_from = AsyncMock(return_value=([_make_edge()], False))
    c._grpc.get_edges_to = AsyncMock(return_value=([_make_edge()], False))
    c._grpc.get_connected_nodes = AsyncMock(return_value=([_make_node()], False))
    c._grpc.share_node = AsyncMock(return_value=True)
    c._grpc.revoke_access = AsyncMock(return_value=True)
    c._grpc.list_shared_with_me = AsyncMock(return_value=([_make_node()], False))
    return c


# ---------------------------------------------------------------------------
# 1. Builder chain
# ---------------------------------------------------------------------------


class TestBuilderChain:
    def test_tenant_returns_tenant_scope(self, client):
        scope = client.tenant("t1")
        assert isinstance(scope, TenantScope)

    def test_tenant_actor_returns_actor_scope(self, client):
        scope = client.tenant("t1").actor("user:bob")
        assert isinstance(scope, ActorScope)

    def test_actor_scope_stores_tenant_and_actor(self, client):
        scope = client.tenant("t1").actor("user:bob")
        assert scope.tenant_id == "t1"
        assert scope.actor == "user:bob"

    def test_tenant_raises_when_not_connected(self):
        c = DbClient("localhost:50051")
        # c._connected is False by default
        with pytest.raises(Exception, match="Not connected"):
            c.tenant("t1")


# ---------------------------------------------------------------------------
# 2. ActorScope read delegates
# ---------------------------------------------------------------------------


class TestActorScopeGet:
    async def test_get_delegates(self, client):
        scope = client.tenant("t1").actor("user:bob")
        result = await scope.get(TASK, "n1")

        assert result is not None
        assert result.node_id == "n1"
        client._grpc.get_node.assert_called_once()
        call_kwargs = client._grpc.get_node.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:bob"
        assert call_kwargs.kwargs["node_id"] == "n1"

    async def test_get_many_delegates(self, client):
        scope = client.tenant("t1").actor("user:bob")
        found, missing = await scope.get_many(TASK, ["n1", "n2"])

        assert len(found) == 1
        client._grpc.get_nodes.assert_called_once()
        call_kwargs = client._grpc.get_nodes.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:bob"
        assert call_kwargs.kwargs["node_ids"] == ["n1", "n2"]

    async def test_query_delegates(self, client):
        scope = client.tenant("t1").actor("user:bob")
        result = await scope.query(TASK, filter={"status": "todo"})

        assert len(result) == 1
        client._grpc.query_nodes.assert_called_once()
        call_kwargs = client._grpc.query_nodes.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:bob"
        assert call_kwargs.kwargs["filter"] == {"status": "todo"}


class TestActorScopeEdges:
    async def test_edges_out_delegates(self, client):
        scope = client.tenant("t1").actor("user:bob")
        edges = await scope.edges_out("n1", EDGE)

        assert len(edges) == 1
        client._grpc.get_edges_from.assert_called_once()
        call_kwargs = client._grpc.get_edges_from.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:bob"
        assert call_kwargs.kwargs["node_id"] == "n1"

    async def test_edges_in_delegates(self, client):
        scope = client.tenant("t1").actor("user:bob")
        edges = await scope.edges_in("n1", EDGE)

        assert len(edges) == 1
        client._grpc.get_edges_to.assert_called_once()
        call_kwargs = client._grpc.get_edges_to.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["node_id"] == "n1"

    async def test_connected_delegates(self, client):
        scope = client.tenant("t1").actor("user:bob")
        nodes = await scope.connected("n1", EDGE)

        assert isinstance(nodes, list)
        client._grpc.get_connected_nodes.assert_called_once()
        call_kwargs = client._grpc.get_connected_nodes.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:bob"
        assert call_kwargs.kwargs["node_id"] == "n1"
        assert call_kwargs.kwargs["edge_type_id"] == 201


# ---------------------------------------------------------------------------
# 3. ACL / sharing delegates
# ---------------------------------------------------------------------------


class TestActorScopeSharing:
    async def test_share_delegates(self, client):
        scope = client.tenant("t1").actor("user:alice")
        result = await scope.share("n1", "user:charlie", perm="write")

        assert result is True
        client._grpc.share_node.assert_called_once()
        call_kwargs = client._grpc.share_node.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:alice"
        assert call_kwargs.kwargs["node_id"] == "n1"
        assert call_kwargs.kwargs["actor_id"] == "user:charlie"
        assert call_kwargs.kwargs["permission"] == "write"

    async def test_revoke_delegates(self, client):
        scope = client.tenant("t1").actor("user:alice")
        result = await scope.revoke("n1", "user:charlie")

        assert result is True
        client._grpc.revoke_access.assert_called_once()
        call_kwargs = client._grpc.revoke_access.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:alice"

    async def test_shared_with_me_delegates(self, client):
        scope = client.tenant("t1").actor("user:bob")
        nodes = await scope.shared_with_me()

        assert len(nodes) == 1
        client._grpc.list_shared_with_me.assert_called_once()
        call_kwargs = client._grpc.list_shared_with_me.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:bob"


# ---------------------------------------------------------------------------
# 4. ScopedPlan
# ---------------------------------------------------------------------------


class TestScopedPlan:
    def test_plan_returns_scoped_plan(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()
        assert isinstance(plan, ScopedPlan)

    def test_scoped_plan_create_auto_fills_tenant_actor(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()

        # create should work without tenant/actor args
        plan.create(TASK, {"title": "Hello"})
        assert len(plan._plan._operations) == 1
        assert plan._plan._tenant_id == "t1"
        assert plan._plan._actor == "user:bob"

    def test_scoped_plan_update(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()
        plan.update(TASK, "n1", {"title": "Updated"})
        assert len(plan._plan._operations) == 1
        op = plan._plan._operations[0]
        assert "update_node" in op
        assert op["update_node"]["id"] == "n1"

    def test_scoped_plan_delete(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()
        plan.delete(TASK, "n1")
        assert len(plan._plan._operations) == 1
        op = plan._plan._operations[0]
        assert "delete_node" in op

    def test_scoped_plan_edge_create(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()
        plan.edge_create(EDGE, "n1", "n2")
        assert len(plan._plan._operations) == 1
        op = plan._plan._operations[0]
        assert "create_edge" in op

    def test_scoped_plan_edge_delete(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()
        plan.edge_delete(EDGE, "n1", "n2")
        assert len(plan._plan._operations) == 1
        op = plan._plan._operations[0]
        assert "delete_edge" in op

    def test_scoped_plan_chaining(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()
        result = plan.create(TASK, {"title": "A"}).create(TASK, {"title": "B"})
        assert result is plan
        assert len(plan._plan._operations) == 2

    async def test_scoped_plan_commit(self, client):
        scope = client.tenant("t1").actor("user:bob")
        plan = scope.plan()
        plan.create(TASK, {"title": "Test"})
        result = await plan.commit()

        assert result.success is True
        client._grpc.execute_atomic.assert_called_once()
        call_kwargs = client._grpc.execute_atomic.call_args
        assert call_kwargs.kwargs["tenant_id"] == "t1"
        assert call_kwargs.kwargs["actor"] == "user:bob"


# ---------------------------------------------------------------------------
# 5. Multiple scopes are independent
# ---------------------------------------------------------------------------


class TestMultipleScopes:
    def test_different_tenants(self, client):
        scope_a = client.tenant("t1").actor("user:a")
        scope_b = client.tenant("t2").actor("user:b")

        assert scope_a.tenant_id == "t1"
        assert scope_a.actor == "user:a"
        assert scope_b.tenant_id == "t2"
        assert scope_b.actor == "user:b"

    async def test_scopes_delegate_independently(self, client):
        scope_a = client.tenant("t1").actor("user:a")
        scope_b = client.tenant("t2").actor("user:b")

        await scope_a.get(TASK, "n1")
        await scope_b.get(TASK, "n2")

        assert client._grpc.get_node.call_count == 2

        first_call = client._grpc.get_node.call_args_list[0]
        assert first_call.kwargs["tenant_id"] == "t1"
        assert first_call.kwargs["actor"] == "user:a"

        second_call = client._grpc.get_node.call_args_list[1]
        assert second_call.kwargs["tenant_id"] == "t2"
        assert second_call.kwargs["actor"] == "user:b"
