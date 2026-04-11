# ADR-005: Event Sourcing and WAL Architecture

## Status: Accepted

## Context

EntDB uses event sourcing: all writes go to a Write-Ahead Log before being applied to SQLite. This provides durability, replay capability, and decouples writes from reads.

## Decision

### Three Kafka topics

```
entdb-events     — per-tenant data operations (high volume)
entdb-global     — user/tenant management (low volume)
entdb-fanout     — notification fanout (medium volume, async)
```

### Topic: entdb-events

All application data operations. Partitioned by tenant_id.

```
Partition key: tenant_id
Events: create_node, update_node, delete_node, create_edge, delete_edge,
        share_node, revoke_access, add_group_member, remove_group_member,
        transfer_ownership
```

Partition count by deployment size:

| Tenants | Partitions | Applier instances |
|---|---|---|
| < 50 | 6 | 1-2 |
| 50-500 | 12 | 2-4 |
| 500-5,000 | 24 | 4-8 |
| 5,000+ | 48 | 8-16 |

### Topic: entdb-global

User and tenant lifecycle. Low volume.

```
Partitions: 3
Events: create_user, update_user, delete_user, freeze_user,
        create_tenant, archive_tenant, delete_tenant,
        add_tenant_member, remove_tenant_member
```

### Topic: entdb-fanout

Notification delivery. Decoupled from the applier hot path.

```
Partitions: 6
Events: notify_mention, notify_dm, notify_share, notify_assign, notify_everyone
```

### Event structure

```protobuf
message WalEvent {
    string tenant_id = 1;
    string actor = 2;
    string idempotency_key = 3;
    string schema_fingerprint = 4;
    int64 timestamp_ms = 5;
    repeated Operation operations = 6;
    repeated GlobalSideEffect side_effects = 7;
}
```

### Write path

```
Client → Plan.commit() → gRPC ExecuteAtomic
  → Server validates schema fingerprint
  → Server serializes event as protobuf
  → Server appends to entdb-events (Kafka)
  → Server returns Receipt with stream_position
  → Tenant applier consumes, applies to SQLite
  → Tenant applier processes side effects (shared_index)
  → Tenant applier emits fanout events (if notifications needed)
  → Fanout worker writes notifications to tenant SQLite
```

### Read-after-write consistency

```
Option 1: plan.commit(wait_applied=True)
  Server waits on asyncio.Condition until applier reaches the offset.

Option 2: db.get(..., after_offset=receipt.stream_position)
  Server waits for offset before executing read.

Both use event-driven notification (asyncio.Condition), not polling.
```

### Applier processing

```
Tenant applier polls batch from Kafka (up to 500 events):
  1. Group by tenant_id
  2. For each tenant, in batch_transaction:
     - Check idempotency (skip already applied)
     - Apply operations to SQLite
     - Record applied event
  3. Process side effects (shared_index updates)
  4. Emit fanout events (notifications)
  5. Commit Kafka offset
  6. Notify offset waiters
```

### Notification fanout (separate from applier)

```
Regular message in #general:
  Applier:  1 write to tenant SQLite                    ~1ms
  Fanout:   nothing (broadcast, no individual notif)

Message with @bob:
  Applier:  1 write to tenant SQLite                    ~1ms
  Fanout:   1 notification insert                        ~1ms (async)

Message with @everyone (1000 users):
  Applier:  1 write to tenant SQLite                    ~1ms
  Fanout:   1000 notifications batch insert              ~5-10ms (async)
```

Broadcast messages (tenant_visible=true) don't need individual notifications. Unread state is tracked via read_cursors.

### Retention and archiving

```
Kafka:      7 days retention
S3 archive: all events, compressed, hourly batches, forever
Snapshots:  tenant SQLite files, daily, to S3

Recovery tiers:
  Tier 1: SQLite exists → read directly
  Tier 2: SQLite lost, Kafka has events → replay from Kafka
  Tier 3: Kafka lost too → restore snapshot + replay from S3 archive
```

### WAL backends (swappable)

```
Kafka / Redpanda     — production (recommended)
AWS Kinesis          — AWS native
Google Pub/Sub       — GCP native
AWS SQS              — AWS simple
Azure Service Bus    — Azure native
Azure Event Hubs     — Azure streaming
Local (in-memory)    — development only
```

All implement the same WalStream protocol. Swapping is a config change.

### Kafka configuration (production)

```yaml
replication.factor: 3
min.insync.replicas: 2
compression.type: lz4
retention.ms: 604800000          # 7 days
max.message.bytes: 1048576       # 1 MB
enable.auto.commit: false        # manual commit after apply
max.poll.records: 500            # batch size
```

## Consequences

- Writes are eventually consistent by default (use after_offset for immediate)
- WAL is the source of truth until applied to SQLite
- Kafka ordering per partition guarantees per-tenant event ordering
- Notification fanout is decoupled — doesn't block the applier
- Three topics to manage (events, global, fanout)
- S3 archive provides unlimited history for compliance/audit
