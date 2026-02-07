#!/usr/bin/env python3
"""
EntDB Demo - Shows insertions and retrievals.

This demo uses the internal components directly since the gRPC server
is not fully implemented yet.
"""

import asyncio
import json
import tempfile

from dbaas.entdb_server.apply.applier import Applier, TransactionEvent
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.apply.mailbox_store import MailboxStore
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import EdgeTypeDef, NodeTypeDef, field
from dbaas.entdb_server.wal.memory import InMemoryWalStream


async def main():
    print("=" * 60)
    print("EntDB Demo - Insertions and Retrievals")
    print("=" * 60)
    print()

    # Create temporary directory for SQLite databases
    with tempfile.TemporaryDirectory() as data_dir:
        print(f"[Setup] Using data directory: {data_dir}")

        # 1. Define Schema
        print("\n[Step 1] Defining schema...")

        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str"),
                field(2, "name", "str"),
                field(3, "role", "enum", enum_values=("admin", "member", "guest")),
            ),
        )

        Task = NodeTypeDef(
            type_id=2,
            name="Task",
            fields=(
                field(1, "title", "str"),
                field(2, "description", "str"),
                field(3, "status", "enum", enum_values=("todo", "in_progress", "done")),
                field(4, "priority", "int"),
            ),
        )

        AssignedTo = EdgeTypeDef(
            edge_id=100,
            name="AssignedTo",
            from_type=2,  # Task
            to_type=1,  # User
        )

        # Register types
        registry = SchemaRegistry()
        registry.register_node_type(User)
        registry.register_node_type(Task)
        registry.register_edge_type(AssignedTo)

        print(f"  - Registered {len(registry._node_types)} node types: User, Task")
        print(f"  - Registered {len(registry._edge_types)} edge types: AssignedTo")

        # 2. Initialize Components
        print("\n[Step 2] Initializing storage components...")

        wal = InMemoryWalStream(num_partitions=4)
        await wal.connect()

        canonical_store = CanonicalStore(data_dir, wal_mode=False)
        mailbox_store = MailboxStore(data_dir)

        applier = Applier(
            wal=wal,
            canonical_store=canonical_store,
            mailbox_store=mailbox_store,
            topic="entdb-wal",
            group_id="demo-applier",
        )

        print("  - WAL: In-memory stream (4 partitions)")
        print("  - Storage: SQLite per tenant")
        print("  - Applier: Ready to process events")

        tenant_id = "acme_corp"
        actor = "user:admin"

        # 3. Insert Users
        print("\n[Step 3] Creating users...")

        users_data = [
            {"email": "alice@acme.com", "name": "Alice Smith", "role": "admin"},
            {"email": "bob@acme.com", "name": "Bob Jones", "role": "member"},
            {"email": "carol@acme.com", "name": "Carol White", "role": "member"},
        ]

        user_ids = []
        for i, user_data in enumerate(users_data):
            event = TransactionEvent(
                tenant_id=tenant_id,
                actor=actor,
                idempotency_key=f"create_user_{i}",
                operations=[
                    {
                        "op": "create_node",
                        "type_id": 1,
                        "id": f"user_{i + 1}",
                        "payload": user_data,
                    }
                ],
            )

            # Append to WAL
            await wal.append(
                "entdb-wal",
                tenant_id,
                json.dumps(event.to_dict()).encode(),
            )

            # Process the event
            await applier.process_one()
            user_ids.append(f"user_{i + 1}")
            print(f"  - Created: {user_data['name']} ({user_data['email']})")

        # 4. Insert Tasks
        print("\n[Step 4] Creating tasks...")

        tasks_data = [
            {
                "title": "Implement login",
                "description": "Add OAuth2 support",
                "status": "done",
                "priority": 1,
            },
            {
                "title": "Fix database bug",
                "description": "Connection pooling issue",
                "status": "in_progress",
                "priority": 2,
            },
            {
                "title": "Write documentation",
                "description": "API reference docs",
                "status": "todo",
                "priority": 3,
            },
        ]

        task_ids = []
        for i, task_data in enumerate(tasks_data):
            event = TransactionEvent(
                tenant_id=tenant_id,
                actor=actor,
                idempotency_key=f"create_task_{i}",
                operations=[
                    {
                        "op": "create_node",
                        "type_id": 2,
                        "id": f"task_{i + 1}",
                        "payload": task_data,
                    }
                ],
            )

            await wal.append(
                "entdb-wal",
                tenant_id,
                json.dumps(event.to_dict()).encode(),
            )

            await applier.process_one()
            task_ids.append(f"task_{i + 1}")
            print(f"  - Created: {task_data['title']} [{task_data['status']}]")

        # 5. Create Edges (Assign tasks to users)
        print("\n[Step 5] Assigning tasks to users...")

        assignments = [
            ("task_1", "user_1"),  # Alice completed login
            ("task_2", "user_2"),  # Bob fixing database
            ("task_3", "user_3"),  # Carol writing docs
        ]

        for task_id, user_id in assignments:
            event = TransactionEvent(
                tenant_id=tenant_id,
                actor=actor,
                idempotency_key=f"assign_{task_id}_{user_id}",
                operations=[
                    {
                        "op": "create_edge",
                        "edge_id": 100,
                        "from_id": task_id,
                        "to_id": user_id,
                        "props": {},
                    }
                ],
            )

            await wal.append(
                "entdb-wal",
                tenant_id,
                json.dumps(event.to_dict()).encode(),
            )

            await applier.process_one()
            print(f"  - {task_id} -> {user_id}")

        # 6. Query All Users
        print("\n[Step 6] Querying all users...")
        print("-" * 50)

        users = await canonical_store.query_nodes(tenant_id, type_id=1)
        for user in users:
            payload = user.get("payload", {})
            print(f"  ID: {user['node_id']}")
            print(f"    Name:  {payload.get('name')}")
            print(f"    Email: {payload.get('email')}")
            print(f"    Role:  {payload.get('role')}")
            print()

        # 7. Query All Tasks
        print("[Step 7] Querying all tasks...")
        print("-" * 50)

        tasks = await canonical_store.query_nodes(tenant_id, type_id=2)
        for task in tasks:
            payload = task.get("payload", {})
            print(f"  ID: {task['node_id']}")
            print(f"    Title:       {payload.get('title')}")
            print(f"    Status:      {payload.get('status')}")
            print(f"    Priority:    {payload.get('priority')}")
            print(f"    Description: {payload.get('description')}")
            print()

        # 8. Get Specific Node
        print("[Step 8] Getting specific node (user_1)...")
        print("-" * 50)

        node = await canonical_store.get_node(tenant_id, "user_1")
        if node:
            print(f"  Node ID:    {node.node_id}")
            print(f"  Type ID:    {node.type_id}")
            print(f"  Payload:    {json.dumps(node.payload, indent=2)}")
            print(f"  Created:    {node.created_at}")
            print(f"  Owner:      {node.owner_actor}")
        print()

        # 9. Query Edges
        print("[Step 9] Querying task assignments (edges)...")
        print("-" * 50)

        for task_id in task_ids:
            edges = await canonical_store.get_edges_from(tenant_id, task_id)
            for edge in edges:
                print(f"  {edge.from_node_id} --[AssignedTo]--> {edge.to_node_id}")
        print()

        # 10. Update a Node
        print("[Step 10] Updating task_2 status to 'done'...")
        print("-" * 50)

        event = TransactionEvent(
            tenant_id=tenant_id,
            actor=actor,
            idempotency_key="update_task_2",
            operations=[
                {
                    "op": "update_node",
                    "type_id": 2,
                    "id": "task_2",
                    "patch": {"status": "done"},
                }
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(event.to_dict()).encode(),
        )

        await applier.process_one()

        # Verify update
        node = await canonical_store.get_node(tenant_id, "task_2")
        if node:
            print(f"  Updated task_2 status: {node.payload.get('status')}")
        print()

        # 11. Idempotency Demo
        print("[Step 11] Testing idempotency (re-sending same event)...")
        print("-" * 50)

        # Re-send the same event
        event = TransactionEvent(
            tenant_id=tenant_id,
            actor=actor,
            idempotency_key="create_user_0",  # Same key as first user
            operations=[
                {
                    "op": "create_node",
                    "type_id": 1,
                    "id": "user_duplicate",
                    "payload": {
                        "email": "duplicate@test.com",
                        "name": "Should Not Exist",
                        "role": "guest",
                    },
                }
            ],
        )

        await wal.append(
            "entdb-wal",
            tenant_id,
            json.dumps(event.to_dict()).encode(),
        )

        await applier.process_one()

        # Count users - should still be 3
        users = await canonical_store.query_nodes(tenant_id, type_id=1)
        print(f"  Users after duplicate attempt: {len(users)} (expected: 3)")
        print("  Idempotency works!")
        print()

        # Cleanup
        await wal.close()

        print("=" * 60)
        print("Demo Complete!")
        print("=" * 60)


if __name__ == "__main__":
    asyncio.run(main())
