# SPDX-License-Identifier: AGPL-3.0-only
"""End-to-End ACID Tests using the Python SDK.

Verifies Atomicity, Consistency, Isolation, and Durability (ACID)
behaviors from a client perspective using the high-level Python SDK client.
"""

from __future__ import annotations

import uuid

import e2e_schema_pb2 as pb
import pytest

from entdb_sdk.errors import PreconditionFailedError, UniqueConstraintError

# =============================================================================
# Atomicity Tests (All-or-Nothing)
# =============================================================================


async def test_acid_atomicity_precondition_failure(scope) -> None:
    """Verifies that a plan containing a mismatching precondition rolls back completely."""
    uid = uuid.uuid4().hex[:8]
    email = f"atom-pre-{uid}@x.com"

    plan = scope.plan()
    # Op 1: Create a valid User node
    plan.create(
        pb.User(email=email, name="Atomicity Pre user"),
        as_="u1",
    )
    # Op 2: Update a nonexistent node with a failing precondition
    plan.update(
        "nonexistent-node-id",
        pb.User(name="Failed Update"),
        precondition=("email", "impossible-expected-value"),
    )

    # Committing the plan must raise PreconditionFailedError
    with pytest.raises(PreconditionFailedError):
        await plan.commit(wait_applied=True)

    # The node created in Op 1 must NOT exist in the database (rolled back)
    nodes = await scope.query(pb.User, filter={"email": email})
    assert len(nodes) == 0, (
        f"Atomicity violation: node with email {email} was created despite plan failure!"
    )


async def test_acid_atomicity_unique_violation(scope) -> None:
    """Verifies that a plan containing a unique index violation rolls back completely."""
    uid = uuid.uuid4().hex[:8]
    shared_sku = f"shared-{uid}"
    distinct_sku = f"distinct-{uid}"

    # First, seed a product with the shared SKU
    seed_plan = scope.plan()
    seed_plan.create(
        pb.Product(sku=shared_sku, name="Seed Product", price=9.99),
        as_="seed",
    )
    seed_res = await seed_plan.commit(wait_applied=True)
    assert seed_res.success

    # Attempt to insert a valid product *and* a duplicate product in a single atomic transaction
    plan = scope.plan()
    # Op 1: Valid product
    plan.create(
        pb.Product(sku=distinct_sku, name="Valid Product", price=19.99),
        as_="ok",
    )
    # Op 2: Duplicate product violating unique constraint on 'sku'
    plan.create(
        pb.Product(sku=shared_sku, name="Duplicate Product", price=29.99),
        as_="dup",
    )

    # Commit must raise UniqueConstraintError
    with pytest.raises(UniqueConstraintError):
        await plan.commit(wait_applied=True)

    # Verify that the OK product node was NOT committed (rolled back)
    nodes = await scope.query(pb.Product, filter={"sku": distinct_sku})
    assert len(nodes) == 0, (
        f"Atomicity violation: SKU {distinct_sku} was committed despite unique violation!"
    )


# =============================================================================
# Consistency Tests (Schema Constraints)
# =============================================================================


async def test_acid_consistency_unique_key(scope) -> None:
    """Verifies that the database preserves consistency by enforcing field-level uniqueness."""
    uid = uuid.uuid4().hex[:8]
    sku = f"consis-{uid}"

    # First insert: succeeds
    p1 = scope.plan()
    p1.create(pb.Product(sku=sku, name="Product 1", price=5.0))
    res1 = await p1.commit(wait_applied=True)
    assert res1.success

    # Second insert of the same SKU: fails
    p2 = scope.plan()
    p2.create(pb.Product(sku=sku, name="Product 2", price=10.0))
    with pytest.raises(UniqueConstraintError) as exc_info:
        await p2.commit(wait_applied=True)

    assert exc_info.value.type_id == 8002
    assert exc_info.value.field_id == 1
    assert exc_info.value.value == sku


# =============================================================================
# Isolation Tests (Lost Update OCC)
# =============================================================================


async def test_acid_isolation_lost_update_precondition(scope) -> None:
    """Verifies that optimistic concurrency control prevents write-write lost updates."""
    uid = uuid.uuid4().hex[:8]
    orig_email = f"orig-{uid}@x.com"

    # Seed the node
    seed_plan = scope.plan()
    seed_plan.create(pb.User(email=orig_email, name="Orig"), as_="u")
    seed_res = await seed_plan.commit(wait_applied=True)
    assert seed_res.success
    node_id = seed_res.created_node_ids[0]

    # Client A and Client B both read the state (email == orig_email)
    node_a = await scope.get(pb.User, node_id)
    assert node_a is not None
    assert node_a.payload["email"] == orig_email

    # Client A updates email to 'new-a@x.com' with precondition email == orig_email (succeeds)
    plan_a = scope.plan()
    plan_a.update(node_id, pb.User(email="new-a@x.com"), precondition=("email", orig_email))
    res_a = await plan_a.commit(wait_applied=True)
    assert res_a.success

    # Client B attempts to update email to 'new-b@x.com' with precondition email == orig_email
    # It must fail because Client A changed the email value
    plan_b = scope.plan()
    plan_b.update(node_id, pb.User(email="new-b@x.com"), precondition=("email", orig_email))
    with pytest.raises(PreconditionFailedError) as exc_info:
        await plan_b.commit(wait_applied=True)

    assert exc_info.value.field == "1"
    assert exc_info.value.expected == orig_email
    assert exc_info.value.observed == "new-a@x.com"
