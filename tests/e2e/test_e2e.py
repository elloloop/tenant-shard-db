#!/usr/bin/env python3
"""
EntDB End-to-End Test Suite

This script tests the complete flow of the EntDB system using the
single-shape SDK v0.3 API (proto messages everywhere — no
``NodeTypeDef + dict`` shape):

1. Register a proto schema (User / Product / Order + edges)
2. Create nodes with proto messages
3. Create edges between nodes via proto edge classes
4. Query / get / list / update / delete
5. Atomic transactions
6. Concurrent transactions

Usage:
    python test_e2e.py

Environment:
    ENTDB_HOST: Server host (default: localhost)
    ENTDB_PORT: Server port (default: 50051)
    ENTDB_TENANT: Tenant ID (default: e2e-test)
"""

import asyncio
import os
import sys
import time
from dataclasses import dataclass

# Add SDK + repo root to path so the generated proto module's
# ``from sdk.entdb_sdk._generated import entdb_options_pb2`` resolves.
sys.path.insert(0, "/app")
sys.path.insert(0, "/app/sdk")
# The schema module lives next to this script; importing it directly
# (rather than ``from tests.e2e import ...``) avoids needing the
# ``tests/__init__.py`` package marker inside the container.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import e2e_schema_pb2 as pb  # noqa: E402

from entdb_sdk import DbClient, register_proto_schema  # noqa: E402

# =============================================================================
# Test Configuration
# =============================================================================

HOST = os.environ.get("ENTDB_HOST", "localhost")
PORT = int(os.environ.get("ENTDB_PORT", "50051"))
TENANT = os.environ.get("ENTDB_TENANT", "e2e-test")
ACTOR = "user:e2e-runner"

MAX_RETRIES = 30
RETRY_DELAY = 2


# =============================================================================
# Test Results
# =============================================================================


@dataclass
class TestResult:
    name: str
    passed: bool
    duration_ms: float
    error: str | None = None


class TestRunner:
    def __init__(self):
        self.results: list[TestResult] = []
        self.client: DbClient | None = None
        self.scope = None  # ActorScope, set after connect

    async def setup(self):
        """Connect to EntDB server with retries; register schema."""
        print(f"\n{'=' * 60}")
        print("EntDB E2E Test Suite")
        print(f"{'=' * 60}")
        print(f"Host: {HOST}:{PORT}")
        print(f"Tenant: {TENANT}")
        print(f"{'=' * 60}\n")

        # Register the proto schema with the SDK registry. This makes
        # type_ids resolvable from proto-message DESCRIPTOR options.
        node_count, edge_count = register_proto_schema(pb)
        print(f"[OK] Registered schema: {node_count} node types, {edge_count} edge types")

        for attempt in range(MAX_RETRIES):
            try:
                self.client = DbClient(f"{HOST}:{PORT}")
                await self.client.connect()
                self.scope = self.client.tenant(TENANT).actor(ACTOR)
                print("[OK] Connected to EntDB server")
                return
            except Exception as e:
                if attempt < MAX_RETRIES - 1:
                    print(
                        f"[WAIT] Server not ready, retrying in {RETRY_DELAY}s... "
                        f"({attempt + 1}/{MAX_RETRIES})"
                    )
                    await asyncio.sleep(RETRY_DELAY)
                else:
                    print(f"[FAIL] Could not connect to server: {e}")
                    sys.exit(1)

    async def teardown(self):
        """Disconnect from server."""
        if self.client:
            await self.client.close()
            print("\n[OK] Disconnected from server")

    async def run_test(self, name: str, test_fn):
        """Run a single test and record result."""
        start = time.time()
        try:
            await test_fn()
            duration = (time.time() - start) * 1000
            self.results.append(TestResult(name, True, duration))
            print(f"  [PASS] {name} ({duration:.1f}ms)")
        except Exception as e:
            duration = (time.time() - start) * 1000
            self.results.append(TestResult(name, False, duration, str(e)))
            print(f"  [FAIL] {name}: {e}")

    def print_summary(self):
        """Print test summary."""
        passed = sum(1 for r in self.results if r.passed)
        failed = sum(1 for r in self.results if not r.passed)
        total_time = sum(r.duration_ms for r in self.results)

        print(f"\n{'=' * 60}")
        print("Test Summary")
        print(f"{'=' * 60}")
        print(f"Passed: {passed}")
        print(f"Failed: {failed}")
        print(f"Total:  {len(self.results)}")
        print(f"Time:   {total_time:.1f}ms")
        print(f"{'=' * 60}")

        if failed > 0:
            print("\nFailed Tests:")
            for r in self.results:
                if not r.passed:
                    print(f"  - {r.name}: {r.error}")

        return failed == 0


# =============================================================================
# Test Cases
# =============================================================================


async def test_create_single_node(runner: TestRunner):
    """Test creating a single node."""
    plan = runner.scope.plan()
    plan.create(
        pb.User(
            email="alice@example.com",
            name="Alice Smith",
            age=30,
            created_at=int(time.time() * 1000),
        ),
        as_="alice",
    )

    result = await plan.commit(wait_applied=True)
    assert result.success, f"Commit failed: {result.error}"
    assert len(result.created_node_ids) == 1, "Expected 1 node created"


async def test_create_multiple_nodes(runner: TestRunner):
    """Test creating multiple nodes in one transaction."""
    plan = runner.scope.plan()

    plan.create(pb.User(email="bob@example.com", name="Bob Jones", age=25), as_="bob")
    plan.create(
        pb.Product(
            sku="LAPTOP-001",
            name="Gaming Laptop",
            price=1299.99,
            category="electronics",
        ),
        as_="laptop",
    )
    plan.create(
        pb.Product(
            sku="SHIRT-001",
            name="Blue T-Shirt",
            price=29.99,
            category="clothing",
        ),
        as_="shirt",
    )

    result = await plan.commit(wait_applied=True)
    assert result.success, f"Commit failed: {result.error}"
    assert len(result.created_node_ids) == 3, (
        f"Expected 3 nodes, got {len(result.created_node_ids)}"
    )


async def test_create_edges(runner: TestRunner):
    """Test creating edges between nodes."""
    plan = runner.scope.plan()
    plan.create(pb.User(email="charlie@example.com", name="Charlie"), as_="charlie")
    plan.create(
        pb.Product(sku="PHONE-001", name="Smartphone", price=799.99, category="electronics"),
        as_="phone",
    )
    plan.create(pb.Order(order_number="ORD-001", total=799.99, status="pending"), as_="order1")

    plan.edge_create(
        pb.Purchased, "$charlie", "$phone", props={"quantity": 1, "price_paid": 799.99}
    )
    plan.edge_create(pb.PlacedOrder, "$charlie", "$order1")
    plan.edge_create(pb.OrderContains, "$order1", "$phone", props={"quantity": 1})

    result = await plan.commit(wait_applied=True)
    assert result.success, f"Commit failed: {result.error}"


async def test_query_nodes_by_type(runner: TestRunner):
    """Test querying nodes by type."""
    nodes = await runner.scope.query(pb.User, limit=100)

    assert len(nodes) >= 1, "Expected at least 1 user node"

    for node in nodes:
        assert node.type_id == 8001
        assert "email" in node.payload
        assert "name" in node.payload


async def test_query_products(runner: TestRunner):
    """Test querying product nodes."""
    nodes = await runner.scope.query(pb.Product, limit=100)

    assert len(nodes) >= 1, "Expected at least 1 product node"

    for node in nodes:
        assert node.type_id == 8002
        assert "sku" in node.payload
        assert "price" in node.payload


async def test_get_node_by_id(runner: TestRunner):
    """Test getting a specific node by ID."""
    plan = runner.scope.plan()
    plan.create(pb.User(email="david@example.com", name="David"), as_="david")
    result = await plan.commit(wait_applied=True)
    assert result.success

    node_id = result.created_node_ids[0]

    node = await runner.scope.get(pb.User, node_id)
    assert node is not None, f"Node {node_id} not found"
    assert node.payload["email"] == "david@example.com"
    assert node.payload["name"] == "David"


async def test_get_edges_from_node(runner: TestRunner):
    """Test getting outgoing edges from a node."""
    plan = runner.scope.plan()
    plan.create(pb.User(email="eve@example.com", name="Eve"), as_="eve")
    plan.create(
        pb.Product(sku="BOOK-001", name="Python Book", price=49.99, category="other"),
        as_="book",
    )
    plan.create(
        pb.Product(sku="BOOK-002", name="Go Book", price=39.99, category="other"),
        as_="gobook",
    )
    plan.edge_create(pb.Purchased, "$eve", "$book", props={"quantity": 2, "price_paid": 99.98})
    plan.edge_create(pb.Purchased, "$eve", "$gobook", props={"quantity": 1, "price_paid": 39.99})

    result = await plan.commit(wait_applied=True)
    assert result.success

    eve_id = result.created_node_ids[0]

    edges = await runner.scope.edges_out(eve_id, edge_type=pb.Purchased)
    assert len(edges) == 2, f"Expected 2 purchase edges, got {len(edges)}"

    for edge in edges:
        assert edge.edge_type_id == 8101
        assert edge.from_node_id == eve_id
        assert "quantity" in edge.props


async def test_update_node(runner: TestRunner):
    """Test updating a node."""
    plan = runner.scope.plan()
    plan.create(pb.User(email="frank@example.com", name="Frank", age=40), as_="frank")
    result = await plan.commit(wait_applied=True)
    assert result.success

    frank_id = result.created_node_ids[0]

    plan2 = runner.scope.plan()
    plan2.update(frank_id, pb.User(age=41, active=False))
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"Update failed: {result2.error}"

    node = await runner.scope.get(pb.User, frank_id)
    assert node.payload["age"] == 41
    assert node.payload["active"] is False


async def test_delete_node(runner: TestRunner):
    """Test deleting a node."""
    plan = runner.scope.plan()
    plan.create(pb.User(email="grace@example.com", name="Grace"), as_="grace")
    result = await plan.commit(wait_applied=True)
    assert result.success

    grace_id = result.created_node_ids[0]

    plan2 = runner.scope.plan()
    plan2.delete(pb.User, grace_id)
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"Delete failed: {result2.error}"

    node = await runner.scope.get(pb.User, grace_id)
    assert node is None, "Node should be deleted"


async def test_delete_edge(runner: TestRunner):
    """Test deleting an edge."""
    plan = runner.scope.plan()
    plan.create(pb.User(email="henry@example.com", name="Henry"), as_="henry")
    plan.create(
        pb.Product(sku="MUG-001", name="Coffee Mug", price=12.99, category="other"),
        as_="mug",
    )
    plan.edge_create(pb.Purchased, "$henry", "$mug", props={"quantity": 1})
    result = await plan.commit(wait_applied=True)
    assert result.success

    henry_id = result.created_node_ids[0]
    mug_id = result.created_node_ids[1]

    edges = await runner.scope.edges_out(henry_id, edge_type=pb.Purchased)
    assert len(edges) == 1

    plan2 = runner.scope.plan()
    plan2.edge_delete(pb.Purchased, henry_id, mug_id)
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"Edge delete failed: {result2.error}"

    edges = await runner.scope.edges_out(henry_id, edge_type=pb.Purchased)
    assert len(edges) == 0, "Edge should be deleted"


async def test_large_payload(runner: TestRunner):
    """Test handling of larger payloads."""
    large_name = "A" * 1000  # 1KB name

    plan = runner.scope.plan()
    plan.create(
        pb.User(email="largeuser@example.com", name=large_name),
        as_="large",
    )

    result = await plan.commit(wait_applied=True)
    assert result.success

    node = await runner.scope.get(pb.User, result.created_node_ids[0])
    assert node.payload["name"] == large_name


async def test_concurrent_transactions(runner: TestRunner):
    """Test multiple concurrent transactions."""

    async def create_user(i: int):
        plan = runner.scope.plan()
        plan.create(
            pb.User(
                email=f"concurrent{i}@example.com",
                name=f"Concurrent User {i}",
            )
        )
        return await plan.commit(wait_applied=True)

    results = await asyncio.gather(*[create_user(i) for i in range(10)])

    for i, result in enumerate(results):
        assert result.success, f"Transaction {i} failed: {result.error}"


# =============================================================================
# Main
# =============================================================================


async def main():
    runner = TestRunner()

    try:
        await runner.setup()

        print("\n[TEST] Node Operations")
        await runner.run_test("create_single_node", lambda: test_create_single_node(runner))
        await runner.run_test("create_multiple_nodes", lambda: test_create_multiple_nodes(runner))
        await runner.run_test("query_nodes_by_type", lambda: test_query_nodes_by_type(runner))
        await runner.run_test("query_products", lambda: test_query_products(runner))
        await runner.run_test("get_node_by_id", lambda: test_get_node_by_id(runner))
        await runner.run_test("update_node", lambda: test_update_node(runner))
        await runner.run_test("delete_node", lambda: test_delete_node(runner))

        print("\n[TEST] Edge Operations")
        await runner.run_test("create_edges", lambda: test_create_edges(runner))
        await runner.run_test("get_edges_from_node", lambda: test_get_edges_from_node(runner))
        await runner.run_test("delete_edge", lambda: test_delete_edge(runner))

        print("\n[TEST] Advanced Operations")
        await runner.run_test("large_payload", lambda: test_large_payload(runner))
        await runner.run_test(
            "concurrent_transactions", lambda: test_concurrent_transactions(runner)
        )

        success = runner.print_summary()
        sys.exit(0 if success else 1)

    finally:
        await runner.teardown()


if __name__ == "__main__":
    asyncio.run(main())
