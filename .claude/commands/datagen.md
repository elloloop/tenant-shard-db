# Datagen — Generate Test Data for EntDB

Generate test entities (nodes, edges, schemas) in a running EntDB instance for development and testing.

## Arguments

$ARGUMENTS: optional mode.
- `/datagen` → seed default test data
- `/datagen schema <name>` → create a custom schema
- `/datagen nodes <type_id> <count>` → generate N nodes of a type
- `/datagen full` → full dataset with multiple types, edges, and mailbox items

## Steps

### 1. Check if stack is running

```bash
python -c "
import grpc
ch = grpc.insecure_channel('localhost:50051')
grpc.channel_ready_future(ch).result(timeout=5)
print('Server: up')
"
```

If not running: `docker compose up -d --build`

### 2. Load or create schema

Check if schema exists:
```bash
cat schema.yaml
```

If the user wants a custom schema, create/update `schema.yaml` with the requested node and edge types.

### 3. Seed data using the SDK

```python
import asyncio
from entdb_sdk.client import DbClient

async def seed():
    async with DbClient('localhost:50051', tenant_id='dev') as db:
        # Create nodes
        for i in range(count):
            plan = db.plan()
            plan.create(type_id, {
                'name': f'Test Item {i}',
                # ... fields based on schema
            })
            await plan.commit(wait_applied=True)

        # Create edges between nodes
        # ...

        print(f'Seeded {count} nodes')

asyncio.run(seed())
```

### 4. Verify

```bash
curl -s http://localhost:8080/api/schema | python -m json.tool
curl -s http://localhost:8080/api/nodes/1?limit=5 | python -m json.tool
```

### 5. Report

Show what was created. Suggest opening Console at http://localhost:8080 to browse.
