#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""ACL sharing — owner grants read, grantee sees it, revoke removes it.

Run it::

    python examples/acl_sharing.py

The contract seed makes ``alice`` (owner) and ``bob`` (member) of
tenant ``acme``. Alice creates a private Task, shares it with Bob,
Bob sees it via ``shared_with_me()``, then Alice revokes and it's
gone. See ADR-003 for the typed-capability ACL model.
"""

from __future__ import annotations

import example_schema_pb2 as schema  # noqa: E402
from _harness import ALICE, BOB, TENANT, run_example  # noqa: E402

from entdb_sdk import Actor, DbClient, Permission, register_proto_schema  # noqa: E402


async def main(endpoint: str) -> None:
    register_proto_schema(schema)

    async with DbClient(endpoint) as db:
        alice = db.tenant(TENANT).actor(ALICE)
        bob = db.tenant(TENANT).actor(BOB)

        # Alice creates a Task only she can see.
        plan = db.atomic(TENANT, ALICE)
        plan.create(
            schema.Task(title="Private plan", description="alice only"),
            id_="acl-task-1",
        )
        await plan.commit(wait_applied=True)

        # Bob can't see it yet.
        before = await bob.shared_with_me()
        before_ids = {n.node_id for n in before}
        assert "acl-task-1" not in before_ids, "leaked before share"

        # Alice shares READ with Bob.
        ok = await alice.share("acl-task-1", Actor.user("bob"), Permission.READ)
        assert ok, "share returned False"
        print("shared acl-task-1 with bob (READ)")

        # Now Bob sees it.
        after = await bob.shared_with_me()
        after_ids = {n.node_id for n in after}
        assert "acl-task-1" in after_ids, f"bob can't see shared node: {after_ids}"
        print("bob sees acl-task-1 via shared_with_me()")

        # Alice revokes Bob's access.
        revoked = await alice.revoke("acl-task-1", "user:bob")
        assert revoked, "revoke returned False"
        gone = {n.node_id for n in await bob.shared_with_me()}
        assert "acl-task-1" not in gone, f"still visible after revoke: {gone}"
        print("revoke removed bob's access")


if __name__ == "__main__":
    run_example(main)
