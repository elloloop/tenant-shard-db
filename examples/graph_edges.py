#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Graph edges — create related nodes + an edge in one atomic plan,
then traverse it.

Run it::

    python examples/graph_edges.py

A ``Plan`` can create a node and reference it as an edge endpoint
*before* it has a server-assigned id, via the ``id_=`` deterministic
id (or ``as_=`` aliases resolved at apply time). Here a Task is
``AssignedTo`` a User; we then walk the edge with ``edges_out`` /
``connected``.
"""

from __future__ import annotations

import example_schema_pb2 as schema  # noqa: E402
from _harness import ALICE, TENANT, run_example  # noqa: E402

from entdb_sdk import DbClient, register_proto_schema  # noqa: E402


async def main(endpoint: str) -> None:
    register_proto_schema(schema)

    async with DbClient(endpoint) as db:
        scope = db.tenant(TENANT).actor(ALICE)

        # Create User + Task + the AssignedTo edge atomically.
        plan = db.atomic(TENANT, ALICE)
        plan.create(
            schema.User(email="assignee@example.com", name="Assignee"),
            id_="ge-user-1",
        )
        plan.create(
            schema.Task(title="Wire the graph", description="edge demo"),
            id_="ge-task-1",
        )
        plan.edge_create(schema.AssignedTo, "ge-task-1", "ge-user-1")
        await plan.commit(wait_applied=True)

        # Outgoing edges from the Task -> should hit the User.
        out = await scope.edges_out("ge-task-1", schema.AssignedTo)
        targets = {e.to_node_id for e in out}
        assert "ge-user-1" in targets, f"edge not found, out={targets}"
        print(f"edges_out(ge-task-1) -> {sorted(targets)}")

        # Incoming edges to the User -> should come from the Task.
        inc = await scope.edges_in("ge-user-1", schema.AssignedTo)
        sources = {e.from_node_id for e in inc}
        assert "ge-task-1" in sources, f"reverse edge missing, in={sources}"
        print(f"edges_in(ge-user-1)  <- {sorted(sources)}")

        # Connected-nodes traversal (ACL-filtered) from the Task.
        connected = await scope.connected("ge-task-1", schema.AssignedTo)
        connected_ids = {n.node_id for n in connected}
        assert "ge-user-1" in connected_ids, f"connected() missed the user: {connected_ids}"
        print(f"connected(ge-task-1) -> {sorted(connected_ids)}")


if __name__ == "__main__":
    run_example(main)
