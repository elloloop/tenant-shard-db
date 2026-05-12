# SPDX-License-Identifier: AGPL-3.0-only
"""Dual-stack parity scenarios — Phase 4A (EPIC #407).

Each test runs the same RPC sequence against the Python and Go
servers (independent Kafka instances + private per-test tenant) and
asserts that:

  1. the per-call commit receipts are semantically equivalent
     (modulo UUIDs/timestamps/offsets — see ``conftest.normalise_*``),
  2. the post-replay SQLite state on each server is byte-identical
     after normalisation.

If either assertion fails the test reports a real divergence; the
parity suite is a transitional gate, so divergences are blockers for
Phase 4D's ``server/python/`` deletion.
"""

from __future__ import annotations

import asyncio
import uuid

import e2e_schema_pb2 as pb
from conftest import (
    assert_receipts_equivalent,
    assert_state_equivalent,
    replay_against_both,
)

# =============================================================================
# 1. Basic CRUD parity
# =============================================================================


async def test_basic_crud_parity(python_client, go_client, parity_tenant) -> None:
    """Create -> Get -> Update -> Delete a single node. After deletion
    both servers must report an empty User set for this tenant."""

    async def _seq(client, tenant_id):
        results: list = []
        scope = client.tenant(tenant_id).actor("user:e2e-runner")
        idem_create = f"crud-create-{tenant_id}"
        idem_update = f"crud-update-{tenant_id}"
        idem_delete = f"crud-delete-{tenant_id}"

        # 1. Create
        plan = scope.plan(idempotency_key=idem_create)
        plan.create(
            pb.User(email="crud@example.com", name="CRUD User", age=30),
            as_="u",
        )
        results.append(await plan.commit(wait_applied=True))
        node_id = results[-1].created_node_ids[0]

        # 2. Get (round-trip check inside the sequence)
        await scope.get(pb.User, node_id)

        # 3. Update
        plan2 = scope.plan(idempotency_key=idem_update)
        plan2.update(node_id, pb.User(age=31, active=True))
        results.append(await plan2.commit(wait_applied=True))

        # 4. Delete
        plan3 = scope.plan(idempotency_key=idem_delete)
        plan3.delete(pb.User, node_id)
        results.append(await plan3.commit(wait_applied=True))
        return results

    py_results, go_results = await replay_against_both(
        python_client, go_client, parity_tenant, _seq
    )
    assert_receipts_equivalent(py_results, go_results)
    await assert_state_equivalent(python_client, go_client, parity_tenant)


# =============================================================================
# 2. Edge creation parity
# =============================================================================


async def test_edge_creation_parity(python_client, go_client, parity_tenant) -> None:
    """Create 3 nodes (1 user, 2 products) and 2 edges between them.
    Both servers must report the same edge set."""

    sku1 = f"PARITY-A-{uuid.uuid4().hex[:6]}"
    sku2 = f"PARITY-B-{uuid.uuid4().hex[:6]}"

    async def _seq(client, tenant_id):
        scope = client.tenant(tenant_id).actor("user:e2e-runner")
        plan = scope.plan(idempotency_key=f"edges-{tenant_id}")
        plan.create(pb.User(email="edges@example.com", name="Edge User"), as_="u")
        plan.create(
            pb.Product(sku=sku1, name="Prod A", price=10.0, category="electronics"),
            as_="p1",
        )
        plan.create(
            pb.Product(sku=sku2, name="Prod B", price=20.0, category="electronics"),
            as_="p2",
        )
        plan.edge_create(pb.Purchased, "$u", "$p1", props={"quantity": 1, "price_paid": 10.0})
        plan.edge_create(pb.Purchased, "$u", "$p2", props={"quantity": 2, "price_paid": 40.0})
        return [await plan.commit(wait_applied=True)]

    py_results, go_results = await replay_against_both(
        python_client, go_client, parity_tenant, _seq
    )
    assert_receipts_equivalent(py_results, go_results)
    await assert_state_equivalent(python_client, go_client, parity_tenant)


# =============================================================================
# 3. Share / Revoke parity
# =============================================================================


async def test_share_revoke_parity(python_client, go_client, parity_tenant) -> None:
    """Create a node, share it with `user:bob`, then revoke. After
    revocation both servers' ACL state must match (which, since
    revoke removes the grant, means: identical normalised node ACL)."""

    async def _seq(client, tenant_id):
        results: list = []
        scope = client.tenant(tenant_id).actor("user:e2e-runner")

        # Create the target node.
        plan = scope.plan(idempotency_key=f"share-create-{tenant_id}")
        plan.create(pb.User(email="share@example.com", name="Share Target"), as_="u")
        cr = await plan.commit(wait_applied=True)
        results.append(cr)
        node_id = cr.created_node_ids[0]

        # Share it with bob (read permission).
        ok = await scope.share(node_id, "user:bob")
        results.append({"share_ok": bool(ok)})

        # Revoke.
        ok2 = await scope.revoke(node_id, "user:bob")
        results.append({"revoke_ok": bool(ok2)})

        return results

    py_results, go_results = await replay_against_both(
        python_client, go_client, parity_tenant, _seq
    )
    assert_receipts_equivalent(py_results, go_results)
    await assert_state_equivalent(python_client, go_client, parity_tenant)


# =============================================================================
# 4. Idempotent retry parity
# =============================================================================


async def test_idempotent_retry_parity(python_client, go_client, parity_tenant) -> None:
    """The same idempotency_key sent N times must produce exactly one
    write on each server. Both servers must agree on:
      * N receipts returned (success on each call),
      * exactly one node in the post-replay state."""

    key = f"idem-{uuid.uuid4().hex}"
    email = f"idem-{uuid.uuid4().hex[:8]}@example.com"

    async def _seq(client, tenant_id):
        scope = client.tenant(tenant_id).actor("user:e2e-runner")
        results: list = []
        for i in range(4):
            plan = scope.plan(idempotency_key=key)
            plan.create(pb.User(email=email, name="Idem User"), as_="u")
            # First call waits-applied; later ones don't (per the
            # e2e test_idempotent_create note — the server short-
            # circuits via the cached receipt and the offset never
            # advances on the applier, so wait would block forever).
            results.append(await plan.commit(wait_applied=(i == 0)))
        # Drop SDK offset tracking so the parity state-snapshot doesn't
        # block waiting for an offset the applier never advanced past.
        scope._client.clear_offsets()
        return results

    py_results, go_results = await replay_against_both(
        python_client, go_client, parity_tenant, _seq
    )
    # Every result.success must be True on both sides.
    assert all(r.success for r in py_results), "python: idempotent retry failed"
    assert all(r.success for r in go_results), "go: idempotent retry failed"
    assert_receipts_equivalent(py_results, go_results)
    await assert_state_equivalent(python_client, go_client, parity_tenant)


# =============================================================================
# 5. Concurrent writers parity
# =============================================================================


async def test_concurrent_writers_parity(python_client, go_client, parity_tenant) -> None:
    """N concurrent writers fire distinct creates; after settle both
    servers' User sets must be byte-identical (modulo normalisation
    order, which the sort-stable snapshot handles)."""

    N = 10
    # Stable email tags so the post-replay state is deterministic
    # across both servers — generating uuid4 per-loop would give us
    # different sets on each server.
    emails = [f"conc-{i:02d}-{uuid.uuid4().hex[:6]}@example.com" for i in range(N)]

    async def _seq(client, tenant_id):
        scope = client.tenant(tenant_id).actor("user:e2e-runner")

        async def _create(i: int):
            plan = scope.plan(idempotency_key=f"conc-{tenant_id}-{i}")
            plan.create(pb.User(email=emails[i], name=f"Conc {i}"), as_="u")
            return await plan.commit(wait_applied=True)

        results = await asyncio.gather(*[_create(i) for i in range(N)])
        return list(results)

    py_results, go_results = await replay_against_both(
        python_client, go_client, parity_tenant, _seq
    )
    # Concurrent ordering means receipt-list order is non-
    # deterministic per-server, so we don't position-compare; just
    # require both sides to be all-success and the same length.
    assert all(r.success for r in py_results), "python: a concurrent write failed"
    assert all(r.success for r in go_results), "go: a concurrent write failed"
    assert len(py_results) == len(go_results) == N
    # And the durable end-state on both servers must match.
    await assert_state_equivalent(python_client, go_client, parity_tenant)


# =============================================================================
# 6. WAL replay after restart parity
# =============================================================================


async def test_replay_after_restart_parity(
    python_client,
    go_client,
    parity_tenant,
    restart_servers,
) -> None:
    """Write data, restart BOTH servers, then assert both servers
    rebuild semantically-identical SQLite state from their WALs."""

    # IMPORTANT: precompute the sku OUTSIDE the closure. _seq runs twice
    # (once per server) and a uuid generated inside would yield different
    # sku values on each side, breaking the state-equivalence assertion.
    replay_sku = f"REPLAY-{uuid.uuid4().hex[:8]}"

    async def _seq(client, tenant_id):
        scope = client.tenant(tenant_id).actor("user:e2e-runner")
        plan = scope.plan(idempotency_key=f"replay-{tenant_id}")
        plan.create(
            pb.User(email="replay@example.com", name="Replay User", age=42),
            as_="u",
        )
        plan.create(
            pb.Product(
                sku=replay_sku,
                name="Replay Product",
                price=99.99,
                category="other",
            ),
            as_="p",
        )
        plan.edge_create(pb.Purchased, "$u", "$p", props={"quantity": 1, "price_paid": 99.99})
        return [await plan.commit(wait_applied=True)]

    py_results, go_results = await replay_against_both(
        python_client, go_client, parity_tenant, _seq
    )
    assert_receipts_equivalent(py_results, go_results)
    # Sanity: pre-restart state must already match.
    await assert_state_equivalent(python_client, go_client, parity_tenant)

    # Drop the offset cursors — after a restart the WAL position the
    # SDK remembers may no longer match the applier's cursor.
    python_client.clear_offsets()
    go_client.clear_offsets()

    await restart_servers()

    # The fixture-scoped channels were bound to a pre-restart server
    # process. Re-connect via a fresh channel; the SDK auto-reconnects
    # on next use, but an explicit connect-loop avoids transient
    # "channel closed" flakes on the first post-restart RPC.
    last_err: Exception | None = None
    for _ in range(30):
        try:
            await assert_state_equivalent(python_client, go_client, parity_tenant)
            return
        except Exception as exc:  # noqa: BLE001
            last_err = exc
            await asyncio.sleep(2)
    raise AssertionError(f"post-restart parity assertion never settled (last error: {last_err!r})")
