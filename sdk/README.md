# EntDB SDK

Python client library for [EntDB](https://github.com/elloloop/tenant-shard-db) — a tenant-sharded graph database with nodes and edges.

## Install

```bash
pip install entdb-sdk
```

## Quick Start

```python
from entdb_sdk import DbClient, NodeTypeDef, field

# Define types
Task = NodeTypeDef(
    type_id=101,
    name="Task",
    fields=(
        field(1, "title", "str", required=True),
        field(2, "status", "enum", enum_values=("todo", "done")),
    ),
)

# Connect and create
async with DbClient("localhost:50051") as db:
    plan = db.atomic("tenant_1", "user:42")
    plan.create(Task, {"title": "My Task", "status": "todo"})
    result = await plan.commit()
```

## License

MIT
