# ADR-010: Consistency Model

## Status: Accepted

## Context

EntDB uses event sourcing: writes go to Kafka, reads come from SQLite. There's a window where reads return stale data. The SDK must handle this transparently so applications get correct behavior by default.

## Decision

### The SDK guarantees read-after-write consistency by default

The SDK, not the application, is responsible for consistency. After a `plan.commit()`, subsequent reads through the SAME SDK client instance are guaranteed to see the written data.

### How it works

```python
async with DbClient("localhost:50051") as db:
    # Write
    plan = db.atomic("acme", "user:alice")
    plan.create(Task, {"title": "Ship v2"}, as_="t")
    result = await plan.commit()
    
    # Read — SDK automatically waits for consistency
    task = await db.get(Task, result.created_node_ids[0], "acme", "user:alice")
    # task is ALWAYS found. The SDK handles it.
```

### SDK tracks the latest offset per tenant

```python
class DbClient:
    _last_offsets: dict[str, str] = {}  # tenant_id → stream_position
    
    async def commit(self, ...):
        result = await self._execute(...)
        if result.receipt and result.receipt.stream_position:
            self._last_offsets[tenant_id] = result.receipt.stream_position
        return result
    
    async def get(self, node_type, node_id, tenant_id, actor, ...):
        # Automatically include after_offset if we've written to this tenant
        after_offset = self._last_offsets.get(tenant_id)
        return await self._grpc.get_node(
            ..., after_offset=after_offset
        )
    
    async def query(self, node_type, tenant_id, actor, ...):
        after_offset = self._last_offsets.get(tenant_id)
        return await self._grpc.query_nodes(
            ..., after_offset=after_offset
        )
```

### Behavior

```
First read (no prior write):     no offset → eventually consistent (fast)
Read after write (same client):  has offset → waits for consistency (correct)
Read after write (diff client):  no offset → eventually consistent (different session)
```

### Server-side: event-driven wait

When a read arrives with `after_offset`:
1. Check: has the applier reached this offset? → yes → read immediately
2. No → wait on `asyncio.Condition` until applier catches up
3. Timeout after 5 seconds → return error (applier is stuck)

The applier notifies the Condition after each batch. No polling.

### Explicit control (opt-in)

Applications can override the default behavior:

```python
# Skip consistency (fast, eventually consistent read)
task = await db.get(Task, node_id, tenant_id, actor, after_offset=None)

# Wait for specific offset (from a different session)
task = await db.get(Task, node_id, tenant_id, actor, 
                    after_offset=some_receipt.stream_position)

# Explicit wait then read
await db.wait_for_offset(tenant_id, receipt.stream_position)
tasks = await db.query(Task, tenant_id, actor)
```

### Cross-client consistency

Different SDK instances (different processes/machines) don't share offsets. If process A writes and process B needs to see the write:

```python
# Process A writes, gets receipt
result = await plan.commit()
# Process A sends receipt.stream_position to Process B (via API response, queue, etc.)

# Process B reads with the offset
task = await db.get(Task, node_id, tenant_id, actor,
                    after_offset=received_stream_position)
```

The `stream_position` is the consistency token. Pass it across processes when needed.

## Consequences

- SDK provides read-after-write consistency by default (no application code needed)
- First reads to a tenant are eventually consistent (no prior offset known)
- Consistency adds ~10-50ms latency on reads after writes (waiting for applier)
- Cross-client consistency requires explicit offset passing
- Applications can opt out for fast reads when consistency isn't needed
- Server-side wait is event-driven (asyncio.Condition), not polling
