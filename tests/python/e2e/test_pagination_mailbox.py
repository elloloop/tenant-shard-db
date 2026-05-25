# SPDX-License-Identifier: AGPL-3.0-only
"""End-to-end coverage (Python SDK ↔ docker-stack server) for the read
features shipped in v1.24–v1.32:

  - keyset cursor auto-follow on query() and edges_out() (ADR-029 / #564,
    #580) — a read returns the COMPLETE set across pages, not a 100-row
    prefix;
  - USER_MAILBOX reads + the tenant-read privacy exclusion (#568).

These run against the real built image via run-e2e.sh, exercising the
full Python-SDK → gRPC → applier → SQLite path that the in-process Go
integration suite (Go SDK) does not cover.
"""

import e2e_schema_pb2 as pb

from entdb_sdk.keys import Mailbox

# pytest asyncio_mode = auto → no decorators needed.


async def test_query_auto_follows_complete_set(db_client, fresh_tenant, actor) -> None:
    """query() with no explicit limit returns ALL matching rows by
    following the keyset cursor — not the 100-row page default (#564)."""
    scope = db_client.tenant(fresh_tenant).actor(actor)
    n = 150  # > the server's 100-row default page

    for batch_start in range(0, n, 50):
        plan = scope.plan()
        for i in range(batch_start, min(batch_start + 50, n)):
            plan.create(pb.User(email=f"pg-{i:04d}@x", name=f"U{i}"), as_=f"u{i}")
        result = await plan.commit(wait_applied=True)
        assert result.success, f"seed commit failed: {result.error}"

    users = await scope.query(pb.User)  # default limit=0 ⇒ complete set
    got = [u for u in users if str(u.payload.get("email", "")).startswith("pg-")]
    assert len(got) == n, f"auto-follow returned {len(got)} of {n} — truncation"
    # No duplicates across pages.
    assert len({u.node_id for u in got}) == n


async def test_edges_out_auto_follows_complete_set(db_client, fresh_tenant, actor) -> None:
    """edges_out() follows the edge keyset cursor over a high-fan-out node
    (#580) — all edges, not a prefix."""
    scope = db_client.tenant(fresh_tenant).actor(actor)
    n = 120  # > default edge page

    plan = scope.plan()
    plan.create(pb.User(email="hub@x", name="Hub"), as_="hub")
    for i in range(n):
        plan.create(
            pb.Product(sku=f"P-{i:04d}", name=f"P{i}", price=1.0, category="other"),
            as_=f"p{i}",
        )
    for i in range(n):
        plan.edge_create(pb.Purchased, "$hub", f"$p{i}", props={"quantity": 1})
    result = await plan.commit(wait_applied=True)
    assert result.success, f"seed commit failed: {result.error}"
    hub_id = result.created_node_ids[0]  # hub is the first create

    edges = await scope.edges_out(hub_id, edge_type=pb.Purchased)
    assert len(edges) == n, f"edge auto-follow returned {len(edges)} of {n} — truncation"


async def test_mailbox_read_and_tenant_privacy(db_client, fresh_tenant, actor) -> None:
    """A USER_MAILBOX node is visible via the mailbox scope and EXCLUDED
    from an ordinary tenant read (#568 privacy boundary)."""
    scope = db_client.tenant(fresh_tenant).actor(actor)
    mailbox_user = "mail-user-1"

    plan = scope.plan()
    plan.create(pb.User(email="tenant@x", name="Tenant Node"), as_="tenant_node")
    plan.create(
        pb.User(email="inbox@x", name="Mailbox Node"),
        as_="mbox",
        storage=Mailbox(user_id=mailbox_user),
    )
    result = await plan.commit(wait_applied=True)
    assert result.success, f"commit failed: {result.error}"

    # Mailbox scope sees the mailbox node.
    mbox = await scope.query_in_mailbox(pb.User, mailbox_user)
    mbox_emails = {u.payload.get("email") for u in mbox}
    assert "inbox@x" in mbox_emails, "mailbox node not visible via mailbox scope"

    # Ordinary tenant read EXCLUDES the mailbox-private node.
    tenant_nodes = await scope.query(pb.User)
    tenant_emails = {u.payload.get("email") for u in tenant_nodes}
    assert "tenant@x" in tenant_emails, "tenant node missing from tenant read"
    assert "inbox@x" not in tenant_emails, "mailbox-private node leaked into tenant read"
