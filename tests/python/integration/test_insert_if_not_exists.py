# SPDX-License-Identifier: AGPL-3.0-only
"""Integration tests for ``ActorScope.insert_if_not_exists`` (issue #599).

Drives the helper end-to-end against a live entdb-server. The mock-based
unit tests (``tests/python/unit/test_insert_if_not_exists.py``) pin the
SDK's branching logic; this suite proves the round-trip wire path
through a real applier (the gap that surfaced the v2.1.0 single-field
UCE parser bug — typed coordinates were dropped on the way back).

Two cases per the v2.1.0 boundary:

  * created path  — fresh unique key → returns ``(created_id, None)``;
    node is queryable.
  * conflict path — same key again → returns ``(None, existing_id)``;
    the resolved id matches the first writer's and the original
    payload is preserved (no upsert overwrite).
"""

from __future__ import annotations

import uuid

import pytest

from entdb_sdk import DbClient, register_proto_schema
from entdb_sdk.registry import reset_registry
from tests.python._test_schemas import test_schema_pb2 as ts

TENANT = "acme"
ALICE = "user:alice"


@pytest.fixture(autouse=True)
def _registered_schema():
    reset_registry()
    register_proto_schema(ts)
    yield
    reset_registry()


async def test_insert_if_not_exists_created(grpc_endpoint) -> None:
    """Happy path: fresh unique key returns the new id, existing is None."""
    sku = f"iine-created-{uuid.uuid4().hex[:8]}"
    client = DbClient(address=grpc_endpoint)
    await client.connect()
    try:
        scope = client.tenant(TENANT).actor(ALICE)
        created, existed = await scope.insert_if_not_exists(
            ts.Product(sku=sku, name="Widget", price_cents=100),
            idempotency_key=f"iine-c-{uuid.uuid4().hex[:8]}",
        )
        assert created is not None and created != "", f"expected a created id, got {created!r}"
        assert existed is None, f"existed should be None on a fresh write, got {existed!r}"

        node = await scope.get(ts.Product, created)
        assert node is not None, "created node not visible after wait_applied write"
        assert node.payload["sku"] == sku
    finally:
        await client.close()


async def test_insert_if_not_exists_resolves_conflict(grpc_endpoint) -> None:
    """Second write of the same unique key returns the FIRST writer's id.

    Regression pin for the v2.1.0 typed-coordinate bug: without the
    single-field UCE parser branch in the SDK, the second call would
    re-raise the UCE instead of resolving to the existing id.
    """
    sku = f"iine-race-{uuid.uuid4().hex[:8]}"
    client = DbClient(address=grpc_endpoint)
    await client.connect()
    try:
        scope = client.tenant(TENANT).actor(ALICE)

        first_id, existed = await scope.insert_if_not_exists(
            ts.Product(sku=sku, name="First", price_cents=100),
            idempotency_key=f"iine-r1-{uuid.uuid4().hex[:8]}",
        )
        assert first_id is not None and existed is None

        # Different payload, same unique sku — the helper must
        # resolve to the first id, NOT create a second and NOT
        # overwrite.
        created2, resolved = await scope.insert_if_not_exists(
            ts.Product(sku=sku, name="Second", price_cents=999),
            idempotency_key=f"iine-r2-{uuid.uuid4().hex[:8]}",
        )
        assert created2 is None, f"second call should not create; got created={created2!r}"
        assert resolved == first_id, f"resolved id {resolved!r} != first writer's id {first_id!r}"

        # First writer's payload preserved (no upsert overwrite).
        node = await scope.get(ts.Product, first_id)
        assert node is not None
        assert node.payload["sku"] == sku
        assert node.payload["name"] == "First", (
            "InsertIfNotExists must not overwrite — that's a v2.2 upsert primitive"
        )
        assert node.payload["price_cents"] == 100
    finally:
        await client.close()
