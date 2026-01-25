# SDK Reference

Complete reference for the EntDB Python SDK.

## Installation

```bash
pip install entdb-sdk
```

## Quick Start

```python
from entdb_sdk import DbClient, NodeTypeDef, EdgeTypeDef, field

# Define schema
User = NodeTypeDef(
    type_id=1,
    name="User",
    fields=(
        field(1, "email", "str", required=True),
        field(2, "name", "str"),
    ),
)

# Connect
client = DbClient(
    endpoint="localhost:50051",
    tenant_id="my_tenant",
    actor="user:alice",
)

# Use
await client.connect()
result = await client.atomic(lambda plan:
    plan.create(User, {"email": "alice@example.com"})
)
await client.close()
```

## Schema Definitions

### NodeTypeDef

Define a node type:

```python
from entdb_sdk import NodeTypeDef, field

User = NodeTypeDef(
    type_id=1,                    # Unique, stable numeric ID
    name="User",                  # Display name (can be renamed)
    fields=(                      # Tuple of field definitions
        field(1, "email", "str", required=True),
        field(2, "name", "str"),
    ),
    deprecated=False,             # Mark type as deprecated
    description="User account",   # Documentation
    default_acl=(                 # Default visibility
        ("tenant:*", "read"),
    ),
)
```

### EdgeTypeDef

Define an edge type:

```python
from entdb_sdk import EdgeTypeDef

AssignedTo = EdgeTypeDef(
    edge_id=100,                  # Unique, stable numeric ID
    name="AssignedTo",            # Display name
    from_type=2,                  # Source node type_id
    to_type=1,                    # Target node type_id
    deprecated=False,
    description="Task assignment",
)
```

### field() Helper

Create field definitions:

```python
from entdb_sdk import field

# Basic string field
email = field(1, "email", "str")

# Required field
name = field(2, "name", "str", required=True)

# Field with default
status = field(3, "status", "str", default="active")

# Enum field
priority = field(4, "priority", "enum",
    enum_values=("low", "medium", "high"),
    default="medium"
)

# Deprecated field
legacy = field(5, "old_field", "str", deprecated=True)
```

### Field Types

| Kind | Description | Python Type |
|------|-------------|-------------|
| `"str"` | UTF-8 string | `str` |
| `"int"` | 64-bit integer | `int` |
| `"float"` | 64-bit float | `float` |
| `"bool"` | Boolean | `bool` |
| `"timestamp"` | Unix ms timestamp | `int` |
| `"enum"` | String enum | `str` |
| `"list_str"` | String list | `List[str]` |
| `"list_int"` | Integer list | `List[int]` |
| `"ref"` | Node reference | `{"type_id": int, "id": str}` |

## DbClient

### Connection

```python
from entdb_sdk import DbClient

# Create client
client = DbClient(
    endpoint="localhost:50051",   # gRPC endpoint
    tenant_id="my_tenant",        # Tenant isolation
    actor="user:alice",           # Identity for ACL
    use_tls=False,                # Enable TLS
    timeout=30.0,                 # Request timeout
)

# Connect
await client.connect()

# Check connection
if client.is_connected:
    print("Connected!")

# Close
await client.close()
```

### Context Manager

```python
async with DbClient(endpoint="...", tenant_id="...", actor="...") as client:
    # Auto-connects and closes
    result = await client.get("node_123")
```

### Atomic Transactions

Execute atomic operations:

```python
# Single operation
result = await client.atomic(lambda plan:
    plan.create(User, {"email": "alice@example.com"})
)

# Multiple operations
result = await client.atomic(lambda plan: (
    plan.create(User, {"email": "alice@example.com"}, alias="alice"),
    plan.create(Task, {"title": "Review PR"}, alias="task"),
    plan.link(AssignedTo, "$task.id", "$alice.id"),
))

# With idempotency key
result = await client.atomic(
    lambda plan: plan.create(User, {...}),
    idempotency_key="create_user_abc123",
)

# Wait for SQLite apply
result = await client.atomic(
    lambda plan: plan.create(User, {...}),
    wait_for_applied=True,
)
```

### Read Operations

```python
# Get single node
node = await client.get("node_123")
print(node.payload["email"])

# Get with options
node = await client.get(
    "node_123",
    include_deleted=False,
    consistency="eventual",  # or "strong"
)

# Query nodes by type
users = await client.query(type_id=User.type_id)
for user in users:
    print(user.payload["name"])

# Query with filters
active_users = await client.query(
    type_id=User.type_id,
    filters={"status": "active"},
    limit=100,
    offset=0,
)

# Get outgoing edges
edges = await client.edge_out(
    node_id="task_123",
    edge_type=AssignedTo.edge_id,
)

# Get incoming edges
edges = await client.edge_in(
    node_id="user_123",
    edge_type=AssignedTo.edge_id,
)
```

### Mailbox Operations

```python
# Get mailbox items
items = await client.mailbox(
    user_id="alice",
    limit=20,
    offset=0,
)

# Search mailbox
results = await client.search(
    user_id="alice",
    query="meeting tomorrow",
)

# Mark read
await client.mark_read(
    user_id="alice",
    node_id="message_123",
)

# Get unread count
count = await client.unread_count(user_id="alice")
```

## Plan Builder

Build atomic transaction plans:

### create()

Create a new node:

```python
plan.create(
    node_type=User,                    # NodeTypeDef
    payload={"email": "..."},          # Field values
    alias="user1",                     # Optional alias for references
    principals=["user:alice"],         # ACL principals
    recipients=["user:bob"],           # Mailbox recipients
)
```

### update()

Update existing node:

```python
plan.update(
    node_id="node_123",                # Or "$alias.id"
    payload={"name": "New Name"},      # Fields to update
)
```

### delete()

Delete node (soft delete):

```python
plan.delete(node_id="node_123")
```

### link()

Create edge between nodes:

```python
plan.link(
    edge_type=AssignedTo,              # EdgeTypeDef
    from_id="$task.id",                # Source (alias or ID)
    to_id="$user.id",                  # Target (alias or ID)
)
```

### unlink()

Delete edge:

```python
plan.unlink(edge_id="edge_123")
```

### set_visibility()

Update node ACL:

```python
plan.set_visibility(
    node_id="node_123",
    principals=["user:alice", "user:bob", "role:admin"],
)
```

## Node Object

Returned from get/query:

```python
node = await client.get("node_123")

node.id           # Unique node ID
node.type_id      # Node type ID
node.tenant_id    # Tenant ID
node.payload      # Dict of field values
node.owner_actor  # Creator identity
node.created_at   # Creation timestamp
node.updated_at   # Last update timestamp
node.deleted      # Soft delete flag
node.version      # Optimistic lock version

# Get field with default
email = node.get("email")
name = node.get("name", "Unknown")
```

## Edge Object

Returned from edge queries:

```python
edge = edges[0]

edge.id            # Unique edge ID
edge.edge_type_id  # Edge type ID
edge.from_id       # Source node ID
edge.to_id         # Target node ID
edge.tenant_id     # Tenant ID
edge.owner_actor   # Creator identity
edge.created_at    # Creation timestamp
edge.payload       # Optional edge payload
```

## Validation

Validate payloads before sending:

```python
from entdb_sdk import validate_payload, validate_or_raise
from entdb_sdk.errors import ValidationError, UnknownFieldError

# Check validity
is_valid, errors = validate_payload(User, {"email": "test@example.com"})
if not is_valid:
    print(errors)

# Raise on invalid
try:
    validate_or_raise(User, {"emial": "typo@example.com"})  # typo
except UnknownFieldError as e:
    print(f"Unknown field: {e.field_name}")
    print(f"Did you mean: {e.suggestions}")
except ValidationError as e:
    print(f"Validation failed: {e.errors}")
```

## Schema Registry

Manage type registrations:

```python
from entdb_sdk import SchemaRegistry

registry = SchemaRegistry()

# Register types
registry.register(User)
registry.register(Task)
registry.register(AssignedTo)

# Freeze for production use
registry.freeze()

# Get fingerprint for compatibility check
print(registry.fingerprint)

# Lookup types
user_type = registry.get_node_type(1)
user_type = registry.get_node_type_by_name("User")
```

## Error Handling

```python
from entdb_sdk.errors import (
    EntDbError,          # Base error
    ConnectionError,     # Connection failed
    TimeoutError,        # Request timeout
    ValidationError,     # Invalid payload
    UnknownFieldError,   # Unknown field name
    NotFoundError,       # Node not found
    PermissionError,     # ACL denied
    ConflictError,       # Optimistic lock conflict
)

try:
    node = await client.get("node_123")
except NotFoundError:
    print("Node not found")
except PermissionError:
    print("Access denied")
except ConnectionError:
    print("Connection failed")
except EntDbError as e:
    print(f"EntDB error: {e}")
```

## Async Utilities

```python
import asyncio
from entdb_sdk import DbClient

async def batch_create(client, users):
    """Create users in batches."""
    batch_size = 100

    for i in range(0, len(users), batch_size):
        batch = users[i:i+batch_size]

        result = await client.atomic(lambda plan: [
            plan.create(User, user, alias=f"user_{j}")
            for j, user in enumerate(batch)
        ])

        await asyncio.sleep(0.1)  # Rate limiting

# Run
asyncio.run(batch_create(client, users))
```
