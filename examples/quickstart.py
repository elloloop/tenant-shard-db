#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Quickstart — connect, create, read, query.

Run it::

    python examples/quickstart.py

or as a test::

    pytest examples/quickstart.py

The single-shape SDK API (ADR-025): you pass proto *message
instances*; ``type_id`` and the payload come from the proto
descriptor. No numeric ids in app code.
"""

from __future__ import annotations

import example_schema_pb2 as schema  # noqa: E402  (sys.path set in _harness)
from _harness import ALICE, TENANT, run_example  # noqa: E402

from entdb_sdk import DbClient, register_proto_schema  # noqa: E402


async def main(endpoint: str) -> None:
    # Register the proto module so the SDK knows User/Task/AssignedTo.
    register_proto_schema(schema)

    async with DbClient(endpoint) as db:
        scope = db.tenant(TENANT).actor(ALICE)

        # --- Create two nodes atomically -----------------------------
        plan = db.atomic(TENANT, ALICE)
        plan.create(
            schema.User(email="quickstart@example.com", name="Quick Start"),
            id_="qs-user-1",
        )
        plan.create(
            schema.Task(title="Read the docs", description="quickstart"),
            id_="qs-task-1",
        )
        # wait_applied=True => read-your-write: block until the applier
        # has materialized the rows before commit() returns.
        result = await plan.commit(wait_applied=True)
        assert result.success, f"commit failed: {result.error}"

        # --- Read one back by id -------------------------------------
        user = await scope.get(schema.User, "qs-user-1")
        assert user is not None, "user not found after wait_applied commit"
        assert user.payload["email"] == "quickstart@example.com"
        assert user.payload["name"] == "Quick Start"
        print(f"got user: {user.payload['email']}")

        # --- Query by type -------------------------------------------
        tasks = await scope.query(schema.Task, limit=50, order_by="node_id")
        titles = {t.payload.get("title") for t in tasks}
        assert "Read the docs" in titles, f"task missing from query: {titles}"
        print(f"query returned {len(tasks)} task(s)")


if __name__ == "__main__":
    run_example(main)
