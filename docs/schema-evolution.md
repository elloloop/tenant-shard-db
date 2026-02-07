# Schema Evolution in EntDB

EntDB uses a protobuf-like schema evolution model that ensures backward and forward compatibility as your schema evolves.

## Key Principles

1. **Stable Numeric IDs**: Each type and field has a permanent numeric ID
2. **Never Remove or Reuse IDs**: Deleted IDs are retired forever
3. **Field Order Doesn't Matter**: Fields are identified by ID, not position
4. **Names Are Display-Only**: Renaming is safe

## Field Types

Supported field kinds:

| Kind | Description | Python Type |
|------|-------------|-------------|
| `str` | UTF-8 string | `str` |
| `int` | 64-bit integer | `int` |
| `float` | 64-bit floating point | `float` |
| `bool` | Boolean | `bool` |
| `timestamp` | Unix timestamp (ms) | `int` |
| `enum` | String from fixed set | `str` |
| `list_str` | List of strings | `List[str]` |
| `list_int` | List of integers | `List[int]` |
| `ref` | Reference to another node | `dict` |

## Safe Changes (Non-Breaking)

These changes are always safe:

### Adding New Types

```python
# Before
User = NodeTypeDef(type_id=1, name="User", ...)

# After - Add new type with new ID
User = NodeTypeDef(type_id=1, name="User", ...)
Task = NodeTypeDef(type_id=2, name="Task", ...)  # NEW
```

### Adding New Fields

```python
# Before
User = NodeTypeDef(
    type_id=1,
    name="User",
    fields=(
        field(1, "email", "str"),
    ),
)

# After - Add field with new ID
User = NodeTypeDef(
    type_id=1,
    name="User",
    fields=(
        field(1, "email", "str"),
        field(2, "name", "str"),  # NEW
    ),
)
```

### Renaming Types or Fields

```python
# Before
field(1, "email", "str")

# After - Same ID, different name
field(1, "emailAddress", "str")  # SAFE
```

### Deprecating Fields

```python
# Mark field as deprecated (still readable, warns on write)
field(1, "legacy_field", "str", deprecated=True)
```

### Adding Enum Values

```python
# Before
field(1, "status", "enum", enum_values=("todo", "done"))

# After - Add new value
field(1, "status", "enum", enum_values=("todo", "doing", "done"))
```

### Making Required Field Optional

```python
# Before
field(1, "name", "str", required=True)

# After - Remove required constraint
field(1, "name", "str")  # Now optional
```

## Breaking Changes (Forbidden)

These changes will fail schema compatibility checks:

### Removing Types

```python
# FORBIDDEN - Would break existing data
# Don't delete User type if it has data
```

### Removing Fields

```python
# FORBIDDEN - Would break existing data
# Use deprecation instead
```

### Changing Field Types

```python
# FORBIDDEN
# Before: field(1, "age", "str")
# After: field(1, "age", "int")  # Type change not allowed
```

### Removing Enum Values

```python
# FORBIDDEN - Existing data may have this value
# Before: enum_values=("a", "b", "c")
# After: enum_values=("a", "b")  # Can't remove "c"
```

### Making Optional Field Required

```python
# FORBIDDEN - Existing data may have NULL
# Before: field(1, "name", "str")
# After: field(1, "name", "str", required=True)
```

### Reusing IDs

```python
# FORBIDDEN
# If field ID 5 was ever used, it cannot be reused
# Even after deletion
```

## Schema Versioning Workflow

### 1. Create Schema Snapshot

```bash
# Generate baseline snapshot
python -m dbaas.entdb_server.tools.schema_cli snapshot \
  --module myapp.schema \
  --output .schema-snapshot.json
```

### 2. Check Compatibility in CI

```bash
# Compare current schema against baseline
python -m dbaas.entdb_server.tools.schema_cli check \
  --module myapp.schema \
  --baseline .schema-snapshot.json
```

### 3. Update Snapshot After Release

After a successful deployment:

```bash
# Update the baseline
python -m dbaas.entdb_server.tools.schema_cli snapshot \
  --module myapp.schema \
  --output .schema-snapshot.json

git add .schema-snapshot.json
git commit -m "Update schema snapshot for v1.2.0"
```

## CI Integration

Add to your CI pipeline:

```yaml
# .github/workflows/ci.yml
schema-check:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Check schema compatibility
      run: |
        pip install -e .
        python -m dbaas.entdb_server.tools.schema_cli check \
          --module myapp.schema \
          --baseline .schema-snapshot.json \
          --fail-on-breaking
```

## Handling Breaking Changes

If you must make a breaking change:

### Option 1: Data Migration

1. Create new field with new ID
2. Migrate existing data
3. Deprecate old field
4. Eventually remove (after all clients updated)

```python
# Step 1: Add new field
fields=(
    field(1, "status_old", "str", deprecated=True),
    field(2, "status", "enum", enum_values=("active", "inactive")),
)

# Step 2: Migration script
async def migrate_status():
    nodes = await store.query_nodes(tenant_id, type_id=1)
    for node in nodes:
        old_status = node.payload.get("status_old")
        if old_status and not node.payload.get("status"):
            new_status = "active" if old_status == "1" else "inactive"
            await store.update_node(node.id, {"status": new_status})
```

### Option 2: New Type Version

1. Create new type (e.g., `UserV2`)
2. Migrate data to new type
3. Update clients to use new type
4. Deprecate old type

```python
UserV1 = NodeTypeDef(type_id=1, name="User", deprecated=True, ...)
UserV2 = NodeTypeDef(type_id=10, name="UserV2", ...)
```

## Schema Fingerprinting

EntDB generates a deterministic fingerprint for schema comparison:

```python
from entdb_sdk import SchemaRegistry

registry = SchemaRegistry()
registry.register(User, Task)
registry.freeze()

print(f"Schema fingerprint: {registry.fingerprint}")
# "sha256:a1b2c3..."
```

The fingerprint changes when:
- Types are added/removed
- Fields are added/removed
- Field properties change

Use fingerprints to verify client-server schema compatibility.
