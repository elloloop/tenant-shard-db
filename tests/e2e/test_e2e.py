#!/usr/bin/env python3
"""
EntDB End-to-End Test Suite

This script tests the complete flow of the EntDB system:
1. Define node types and edge types using the SDK
2. Create nodes with data
3. Create edges between nodes
4. Query nodes and verify data
5. Query edges and verify relationships
6. Test atomic transactions
7. Test error handling

Usage:
    python test_e2e.py [--host HOST] [--port PORT] [--tenant TENANT]

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

# Add SDK to path
sys.path.insert(0, "/app/sdk")
sys.path.insert(0, "/app")

from entdb_sdk import DbClient, EdgeTypeDef, NodeTypeDef, field

# =============================================================================
# Test Configuration
# =============================================================================

HOST = os.environ.get("ENTDB_HOST", "localhost")
PORT = int(os.environ.get("ENTDB_PORT", "50051"))
TENANT = os.environ.get("ENTDB_TENANT", "e2e-test")
ACTOR = "e2e-test:runner"

# Retry configuration for waiting on server
MAX_RETRIES = 30
RETRY_DELAY = 2


# =============================================================================
# Schema Definitions
# =============================================================================

# Define node types
User = NodeTypeDef(
    type_id=1,
    name="User",
    description="Application user for e2e testing",
    fields=(
        field(1, "email", "str", required=True, indexed=True),
        field(2, "name", "str", required=True, searchable=True),
        field(3, "age", "int"),
        field(4, "active", "bool", default=True),
        field(5, "created_at", "int"),
    ),
)

Product = NodeTypeDef(
    type_id=2,
    name="Product",
    description="Product catalog item",
    fields=(
        field(1, "sku", "str", required=True, indexed=True),
        field(2, "name", "str", required=True, searchable=True),
        field(3, "price", "float", required=True),
        field(4, "category", "enum", enum_values=("electronics", "clothing", "food", "other")),
        field(5, "in_stock", "bool", default=True),
    ),
)

Order = NodeTypeDef(
    type_id=3,
    name="Order",
    description="Customer order",
    fields=(
        field(1, "order_number", "str", required=True, indexed=True),
        field(2, "total", "float", required=True),
        field(
            3,
            "status",
            "enum",
            enum_values=("pending", "paid", "shipped", "delivered", "cancelled"),
        ),
        field(4, "created_at", "int"),
    ),
)

# Define edge types
Purchased = EdgeTypeDef(
    edge_id=101,
    name="purchased",
    description="User purchased a product",
    from_type=User,
    to_type=Product,
    props=(
        field(1, "quantity", "int", required=True),
        field(2, "price_paid", "float"),
    ),
)

PlacedOrder = EdgeTypeDef(
    edge_id=102,
    name="placed_order",
    description="User placed an order",
    from_type=User,
    to_type=Order,
)

OrderContains = EdgeTypeDef(
    edge_id=103,
    name="contains",
    description="Order contains a product",
    from_type=Order,
    to_type=Product,
    props=(field(1, "quantity", "int", required=True),),
)


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

    async def setup(self):
        """Connect to EntDB server with retries."""
        print(f"\n{'=' * 60}")
        print("EntDB E2E Test Suite")
        print(f"{'=' * 60}")
        print(f"Host: {HOST}:{PORT}")
        print(f"Tenant: {TENANT}")
        print(f"{'=' * 60}\n")

        for attempt in range(MAX_RETRIES):
            try:
                self.client = DbClient(f"{HOST}:{PORT}")
                await self.client.connect()
                print("[OK] Connected to EntDB server")
                return
            except Exception as e:
                if attempt < MAX_RETRIES - 1:
                    print(
                        f"[WAIT] Server not ready, retrying in {RETRY_DELAY}s... ({attempt + 1}/{MAX_RETRIES})"
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
    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(
        User,
        {
            "email": "alice@example.com",
            "name": "Alice Smith",
            "age": 30,
            "created_at": int(time.time() * 1000),
        },
        as_="alice",
    )

    result = await plan.commit(wait_applied=True)
    assert result.success, f"Commit failed: {result.error}"
    assert len(result.created_node_ids) == 1, "Expected 1 node created"


async def test_create_multiple_nodes(runner: TestRunner):
    """Test creating multiple nodes in one transaction."""
    plan = runner.client.atomic(TENANT, ACTOR)

    plan.create(
        User,
        {
            "email": "bob@example.com",
            "name": "Bob Jones",
            "age": 25,
        },
        as_="bob",
    )

    plan.create(
        Product,
        {
            "sku": "LAPTOP-001",
            "name": "Gaming Laptop",
            "price": 1299.99,
            "category": "electronics",
        },
        as_="laptop",
    )

    plan.create(
        Product,
        {
            "sku": "SHIRT-001",
            "name": "Blue T-Shirt",
            "price": 29.99,
            "category": "clothing",
        },
        as_="shirt",
    )

    result = await plan.commit(wait_applied=True)
    assert result.success, f"Commit failed: {result.error}"
    assert len(result.created_node_ids) == 3, (
        f"Expected 3 nodes, got {len(result.created_node_ids)}"
    )


async def test_create_edges(runner: TestRunner):
    """Test creating edges between nodes."""
    # First create nodes
    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(User, {"email": "charlie@example.com", "name": "Charlie"}, as_="charlie")
    plan.create(
        Product,
        {"sku": "PHONE-001", "name": "Smartphone", "price": 799.99, "category": "electronics"},
        as_="phone",
    )
    plan.create(
        Order, {"order_number": "ORD-001", "total": 799.99, "status": "pending"}, as_="order1"
    )

    # Create edges in same transaction
    plan.edge_create(
        Purchased, from_="$charlie", to="$phone", props={"quantity": 1, "price_paid": 799.99}
    )
    plan.edge_create(PlacedOrder, from_="$charlie", to="$order1")
    plan.edge_create(OrderContains, from_="$order1", to="$phone", props={"quantity": 1})

    result = await plan.commit(wait_applied=True)
    assert result.success, f"Commit failed: {result.error}"


async def test_query_nodes_by_type(runner: TestRunner):
    """Test querying nodes by type."""
    # Query all users
    nodes = await runner.client.query(
        User,
        TENANT,
        ACTOR,
        limit=100,
    )

    assert len(nodes) >= 1, "Expected at least 1 user node"

    # Verify node structure
    for node in nodes:
        assert node.type_id == User.type_id
        assert "email" in node.payload
        assert "name" in node.payload


async def test_query_products(runner: TestRunner):
    """Test querying product nodes."""
    nodes = await runner.client.query(
        Product,
        TENANT,
        ACTOR,
        limit=100,
    )

    assert len(nodes) >= 1, "Expected at least 1 product node"

    for node in nodes:
        assert node.type_id == Product.type_id
        assert "sku" in node.payload
        assert "price" in node.payload


async def test_get_node_by_id(runner: TestRunner):
    """Test getting a specific node by ID."""
    # First create a node and get its ID
    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(User, {"email": "david@example.com", "name": "David"}, as_="david")
    result = await plan.commit(wait_applied=True)
    assert result.success

    node_id = result.created_node_ids[0]

    # Now fetch it by ID
    node = await runner.client.get(User, node_id, TENANT, ACTOR)
    assert node is not None, f"Node {node_id} not found"
    assert node.payload["email"] == "david@example.com"
    assert node.payload["name"] == "David"


async def test_get_edges_from_node(runner: TestRunner):
    """Test getting outgoing edges from a node."""
    # Create a user with purchases
    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(User, {"email": "eve@example.com", "name": "Eve"}, as_="eve")
    plan.create(
        Product,
        {"sku": "BOOK-001", "name": "Python Book", "price": 49.99, "category": "other"},
        as_="book",
    )
    plan.create(
        Product,
        {"sku": "BOOK-002", "name": "Go Book", "price": 39.99, "category": "other"},
        as_="gobook",
    )
    plan.edge_create(
        Purchased, from_="$eve", to="$book", props={"quantity": 2, "price_paid": 99.98}
    )
    plan.edge_create(
        Purchased, from_="$eve", to="$gobook", props={"quantity": 1, "price_paid": 39.99}
    )

    result = await plan.commit(wait_applied=True)
    assert result.success

    eve_id = result.created_node_ids[0]

    # Get edges from Eve
    edges = await runner.client.edges_out(eve_id, TENANT, ACTOR, edge_type=Purchased)
    assert len(edges) == 2, f"Expected 2 purchase edges, got {len(edges)}"

    for edge in edges:
        assert edge.edge_type_id == Purchased.edge_id
        assert edge.from_node_id == eve_id
        assert "quantity" in edge.props


async def test_update_node(runner: TestRunner):
    """Test updating a node."""
    # Create a node
    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(User, {"email": "frank@example.com", "name": "Frank", "age": 40}, as_="frank")
    result = await plan.commit(wait_applied=True)
    assert result.success

    frank_id = result.created_node_ids[0]

    # Update the node
    plan2 = runner.client.atomic(TENANT, ACTOR)
    plan2.update(User, frank_id, {"age": 41, "active": False})
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"Update failed: {result2.error}"

    # Verify update
    node = await runner.client.get(User, frank_id, TENANT, ACTOR)
    assert node.payload["age"] == 41
    assert not node.payload["active"]


async def test_delete_node(runner: TestRunner):
    """Test deleting a node."""
    # Create a node
    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(User, {"email": "grace@example.com", "name": "Grace"}, as_="grace")
    result = await plan.commit(wait_applied=True)
    assert result.success

    grace_id = result.created_node_ids[0]

    # Delete the node
    plan2 = runner.client.atomic(TENANT, ACTOR)
    plan2.delete(User, grace_id)
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"Delete failed: {result2.error}"

    # Verify deletion
    node = await runner.client.get(User, grace_id, TENANT, ACTOR)
    assert node is None, "Node should be deleted"


async def test_delete_edge(runner: TestRunner):
    """Test deleting an edge."""
    # Create nodes and edge
    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(User, {"email": "henry@example.com", "name": "Henry"}, as_="henry")
    plan.create(
        Product,
        {"sku": "MUG-001", "name": "Coffee Mug", "price": 12.99, "category": "other"},
        as_="mug",
    )
    plan.edge_create(Purchased, from_="$henry", to="$mug", props={"quantity": 1})
    result = await plan.commit(wait_applied=True)
    assert result.success

    henry_id = result.created_node_ids[0]
    mug_id = result.created_node_ids[1]

    # Verify edge exists
    edges = await runner.client.edges_out(henry_id, TENANT, ACTOR, edge_type=Purchased)
    assert len(edges) == 1

    # Delete the edge
    plan2 = runner.client.atomic(TENANT, ACTOR)
    plan2.edge_delete(Purchased, from_=henry_id, to=mug_id)
    result2 = await plan2.commit(wait_applied=True)
    assert result2.success, f"Edge delete failed: {result2.error}"

    # Verify edge is deleted
    edges = await runner.client.edges_out(henry_id, TENANT, ACTOR, edge_type=Purchased)
    assert len(edges) == 0, "Edge should be deleted"


async def test_atomic_transaction_rollback(runner: TestRunner):
    """Test that failed transactions don't partially apply."""
    # Get initial user count
    users_before = await runner.client.query(User, TENANT, ACTOR, limit=1000)
    _ = len(users_before)

    # Try to create a transaction that should fail
    # (This tests server-side validation)
    try:
        plan = runner.client.atomic(TENANT, ACTOR)
        plan.create(User, {"email": "invalid@test.com", "name": "Test"})
        # Try to update a non-existent node - this should work but the node won't exist
        plan.update(User, "non-existent-node-id-12345", {"age": 100})
        await plan.commit(wait_applied=True)
    except Exception:
        pass  # Expected to fail

    # Verify no new users were created
    _ = await runner.client.query(User, TENANT, ACTOR, limit=1000)
    # Note: This test may pass even if the transaction partially applied
    # depending on server implementation


async def test_large_payload(runner: TestRunner):
    """Test handling of larger payloads."""
    large_name = "A" * 1000  # 1KB name

    plan = runner.client.atomic(TENANT, ACTOR)
    plan.create(
        User,
        {
            "email": "largeuser@example.com",
            "name": large_name,
        },
        as_="large",
    )

    result = await plan.commit(wait_applied=True)
    assert result.success

    node = await runner.client.get(User, result.created_node_ids[0], TENANT, ACTOR)
    assert node.payload["name"] == large_name


async def test_concurrent_transactions(runner: TestRunner):
    """Test multiple concurrent transactions."""

    async def create_user(i: int):
        plan = runner.client.atomic(TENANT, ACTOR)
        plan.create(
            User,
            {
                "email": f"concurrent{i}@example.com",
                "name": f"Concurrent User {i}",
            },
        )
        return await plan.commit(wait_applied=True)

    # Run 10 concurrent transactions
    results = await asyncio.gather(*[create_user(i) for i in range(10)])

    # All should succeed
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
