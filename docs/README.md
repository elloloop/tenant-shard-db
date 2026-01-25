# EntDB Documentation

EntDB is a production-grade Database-as-a-Service (DBaaS) system designed for multi-tenant applications with event sourcing architecture.

## Table of Contents

1. [Getting Started](getting-started.md)
2. [Architecture Overview](architecture.md)
3. [Schema Evolution](schema-evolution.md)
4. [Durability Guarantees](durability.md)
5. [Deployment Guide](deployment.md)
6. [SDK Reference](sdk-reference.md)
7. [API Reference](api-reference.md)
8. [Operations Guide](operations.md)

## Quick Start

```bash
# Start local development environment
docker-compose up -d

# Install SDK
pip install entdb-sdk

# Run your first query
python -c "
from entdb_sdk import DbClient

client = DbClient(
    endpoint='localhost:50051',
    tenant_id='my_tenant',
    actor='user:alice',
)

# Create a node
async def main():
    result = await client.atomic(lambda plan:
        plan.create(User, {'email': 'alice@example.com', 'name': 'Alice'})
    )
    print(f'Created node: {result}')

import asyncio
asyncio.run(main())
"
```

## Core Concepts

### Nodes and Edges

EntDB uses a graph data model:

- **Nodes**: Entities with typed payloads (e.g., User, Task, Message)
- **Edges**: Unidirectional relationships between nodes (e.g., AssignedTo, MemberOf)

### Event Sourcing

All writes go to a WAL (Write-Ahead Log) stream first:

1. Client sends atomic transaction
2. Server appends to WAL (Kafka/Kinesis)
3. Applier consumes WAL and updates SQLite
4. Client reads from SQLite

This provides strong durability guarantees and enables:
- Point-in-time recovery
- Event replay
- Audit logging

### Multi-Tenancy

Every operation requires:
- `tenant_id`: Isolates data between tenants
- `actor`: Identity performing the operation (for ACL)

Data is stored in separate SQLite databases per tenant.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Clients                             │
│              (gRPC / HTTP / Python SDK)                     │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                    EntDB Server                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ gRPC Server  │  │ HTTP Server  │  │ Schema       │       │
│  │              │  │              │  │ Registry     │       │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘       │
│         │                 │                                  │
│  ┌──────▼─────────────────▼─────┐                           │
│  │         WAL Stream           │                           │
│  │    (Kafka / Kinesis)         │                           │
│  └──────────────┬───────────────┘                           │
│                 │                                            │
│  ┌──────────────▼───────────────┐                           │
│  │         Applier              │                           │
│  │  (Idempotent Event Apply)    │                           │
│  └──────────────┬───────────────┘                           │
│                 │                                            │
│  ┌──────────────▼───────────────┐   ┌──────────────┐        │
│  │     Canonical Store          │   │   Mailbox    │        │
│  │    (SQLite per tenant)       │   │  (FTS5)      │        │
│  └──────────────────────────────┘   └──────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## License

MIT License - see LICENSE file for details.
