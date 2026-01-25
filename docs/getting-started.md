# Getting Started with EntDB

This guide walks you through setting up EntDB for local development and running your first operations.

## Prerequisites

- Docker and Docker Compose
- Python 3.10+
- (Optional) Protocol Buffers compiler for gRPC

## Local Development Setup

### 1. Clone and Start Services

```bash
# Clone the repository
git clone https://github.com/your-org/entdb.git
cd entdb

# Start all services
docker-compose up -d

# Verify services are running
docker-compose ps
```

This starts:
- EntDB server (gRPC on 50051, HTTP on 8080)
- Redpanda (Kafka-compatible WAL)
- MinIO (S3-compatible storage)
- Redpanda Console (Kafka UI on 8081)

### 2. Install the SDK

```bash
pip install entdb-sdk

# Or install from source
pip install -e ./sdk
```

### 3. Define Your Schema

Create a `schema.py` file:

```python
from entdb_sdk import NodeTypeDef, EdgeTypeDef, field

# Define node types with stable numeric IDs
User = NodeTypeDef(
    type_id=1,
    name="User",
    fields=(
        field(1, "email", "str", required=True),
        field(2, "name", "str"),
        field(3, "created_at", "timestamp"),
    ),
)

Task = NodeTypeDef(
    type_id=2,
    name="Task",
    fields=(
        field(1, "title", "str", required=True),
        field(2, "description", "str"),
        field(3, "status", "enum", enum_values=("todo", "doing", "done")),
        field(4, "due_date", "timestamp"),
    ),
)

Team = NodeTypeDef(
    type_id=3,
    name="Team",
    fields=(
        field(1, "name", "str", required=True),
        field(2, "description", "str"),
    ),
)

# Define edge types
AssignedTo = EdgeTypeDef(
    edge_id=100,
    name="AssignedTo",
    from_type=2,  # Task
    to_type=1,    # User
)

MemberOf = EdgeTypeDef(
    edge_id=101,
    name="MemberOf",
    from_type=1,  # User
    to_type=3,    # Team
)
```

### 4. Connect and Create Data

```python
import asyncio
from entdb_sdk import DbClient
from schema import User, Task, AssignedTo

async def main():
    # Connect to EntDB
    client = DbClient(
        endpoint="localhost:50051",
        tenant_id="my_company",
        actor="user:admin",
    )

    await client.connect()

    # Create user and task in single atomic transaction
    result = await client.atomic(lambda plan: (
        plan.create(User, {
            "email": "alice@example.com",
            "name": "Alice",
        }, alias="alice"),
        plan.create(Task, {
            "title": "Review PR #123",
            "status": "todo",
        }, alias="task1"),
        plan.link(AssignedTo, "$task1.id", "$alice.id"),
    ))

    print(f"Created user: {result['alice']}")
    print(f"Created task: {result['task1']}")

    # Query tasks assigned to Alice
    alice_node = await client.get(result['alice'])
    edges = await client.edge_in(alice_node.id, edge_type=AssignedTo.edge_id)

    print(f"Tasks assigned to Alice: {len(edges)}")

    await client.close()

asyncio.run(main())
```

### 5. Verify in Redpanda Console

Open http://localhost:8081 to see events in the WAL stream.

## Project Structure

Recommended project structure for EntDB applications:

```
my-app/
├── schema/
│   ├── __init__.py
│   ├── nodes.py          # Node type definitions
│   └── edges.py          # Edge type definitions
├── services/
│   ├── __init__.py
│   ├── user_service.py   # Business logic
│   └── task_service.py
├── api/
│   ├── __init__.py
│   └── routes.py         # HTTP/gRPC handlers
├── tests/
│   ├── test_user.py
│   └── test_task.py
├── docker-compose.yml
└── main.py
```

## Next Steps

- [Schema Evolution](schema-evolution.md) - Learn about safe schema changes
- [Durability Guarantees](durability.md) - Understand the durability model
- [SDK Reference](sdk-reference.md) - Complete SDK documentation
- [Deployment Guide](deployment.md) - Production deployment

## Troubleshooting

### Connection Refused

If you get connection errors:

```bash
# Check if services are running
docker-compose ps

# Check logs
docker-compose logs dbaas
```

### Schema Validation Errors

If you get schema validation errors:

1. Check field IDs are unique within a type
2. Check type IDs are unique across all types
3. Verify enum values are strings

### Data Not Appearing

The applier processes events asynchronously. Add a small delay:

```python
await asyncio.sleep(0.5)  # Wait for applier
```

For production, use the `wait_for_applied` option:

```python
result = await client.atomic(
    lambda plan: plan.create(User, {...}),
    wait_for_applied=True,
)
```
