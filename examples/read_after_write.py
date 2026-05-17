#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Read-after-write — eventual by default, strict on demand.

Run it::

    python examples/read_after_write.py

Reads hit the SQLite materialized view, which the applier updates
asynchronously after the WAL append — so a naive read right after a
write can miss it. Two ways to get read-your-write:

  1. ``commit(wait_applied=True)`` blocks until the write is applied.
  2. Capture the receipt's stream position and pass it as
     ``after_offset=`` on the next read.

See docs/durability.md for what ``wait_applied`` actually waits for.
"""

from __future__ import annotations

import example_schema_pb2 as schema  # noqa: E402
from _harness import ALICE, TENANT, run_example  # noqa: E402

from entdb_sdk import DbClient, register_proto_schema  # noqa: E402


async def main(endpoint: str) -> None:
    register_proto_schema(schema)

    async with DbClient(endpoint) as db:
        scope = db.tenant(TENANT).actor(ALICE)

        # --- Path 1: wait_applied=True -------------------------------
        plan = db.atomic(TENANT, ALICE)
        plan.create(
            schema.User(email="raw1@example.com", name="Strict Reader"),
            id_="raw-user-1",
        )
        await plan.commit(wait_applied=True)

        node = await scope.get(schema.User, "raw-user-1")
        assert node is not None, "wait_applied=True did not guarantee visibility"
        assert node.payload["email"] == "raw1@example.com"
        print("wait_applied=True: write visible immediately")

        # --- Path 2: explicit offset from the receipt ----------------
        plan2 = db.atomic(TENANT, ALICE)
        plan2.create(
            schema.User(email="raw2@example.com", name="Offset Reader"),
            id_="raw-user-2",
        )
        result = await plan2.commit()  # no wait — returns a CommitResult
        assert result.success, f"commit failed: {result.error}"
        offset = result.receipt.stream_position if result.receipt else None

        # Pass the write's offset so the read waits for at-least that
        # position to be applied. The SDK also tracks this internally,
        # but being explicit is the documented strict path.
        node2 = await scope.get(
            schema.User,
            "raw-user-2",
            after_offset=str(offset) if offset is not None else None,
        )
        assert node2 is not None, "offset-pinned read missed the write"
        assert node2.payload["email"] == "raw2@example.com"
        print(f"after_offset={offset}: write visible at pinned offset")


if __name__ == "__main__":
    run_example(main)
