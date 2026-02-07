# Durability Guarantees in EntDB

EntDB is designed for strong durability guarantees suitable for production workloads handling financial data, user records, and other critical information.

## Durability Model

### Write Path

```
Client → gRPC Server → WAL (Kafka) → Applier → SQLite
              │
              └── Returns after WAL ack
```

1. **Client Request**: Atomic transaction sent to server
2. **WAL Append**: Events written to Kafka with `acks=all`
3. **Server Response**: Returns after Kafka acknowledges
4. **Async Apply**: Applier consumes WAL and updates SQLite

### Durability Guarantee

**A write is durable when the server returns success.** The data is persisted in the WAL with all replicas acknowledged before the response.

SQLite is a derived view that can be rebuilt from the WAL at any time.

## Configuration

### Kafka Producer Settings

```python
# Production configuration
producer_config = {
    "acks": "all",                    # Wait for all ISR replicas
    "enable_idempotence": True,       # Prevent duplicates on retry
    "max_in_flight_requests": 5,      # Allow pipelining
    "retries": 10,                    # Retry on transient failures
    "retry_backoff_ms": 100,
}
```

### Kafka Cluster Requirements

For production durability:

| Setting | Value | Reason |
|---------|-------|--------|
| `replication.factor` | 3 | Survive 2 broker failures |
| `min.insync.replicas` | 2 | Require 2 replicas for ack |
| `unclean.leader.election.enable` | false | Prevent data loss |

### Environment Variables

```bash
# WAL configuration
ENTDB_KAFKA_BROKERS=broker1:9092,broker2:9092,broker3:9092
ENTDB_KAFKA_ACKS=all
ENTDB_KAFKA_ENABLE_IDEMPOTENCE=true

# S3 archiving
ENTDB_S3_BUCKET=entdb-archive
ENTDB_ARCHIVE_ENABLED=true
ENTDB_SNAPSHOT_ENABLED=true
```

## Failure Scenarios

### Broker Failure

**Scenario**: One Kafka broker fails during write

**Behavior**:
- Producer retries to remaining brokers
- Write succeeds if ISR >= `min.insync.replicas`
- Client sees slight latency increase

**Data Impact**: None - fully durable

### Network Partition

**Scenario**: Network split isolates some brokers

**Behavior**:
- Writes may fail if ISR < `min.insync.replicas`
- Server returns error to client
- Client retries with idempotency key

**Data Impact**: None - write either succeeds or fails atomically

### Application Crash

**Scenario**: EntDB server crashes after WAL append

**Behavior**:
- WAL has the event
- On restart, applier replays from checkpoint
- SQLite catches up to WAL

**Data Impact**: None - WAL is source of truth

### SQLite Corruption

**Scenario**: SQLite database file corrupted

**Behavior**:
- Detect via SQLite integrity check
- Restore from S3 snapshot
- Replay events from checkpoint to current

**Data Impact**: None - can rebuild from WAL

## Idempotency

Every transaction has an idempotency key:

```python
result = await client.atomic(
    lambda plan: plan.create(User, {...}),
    idempotency_key="create_user_alice_123",
)
```

**Guarantees**:
- Same key always returns same result
- Survives server restarts
- Tracked in SQLite `applied_events` table

**Implementation**:
```sql
CREATE TABLE applied_events (
    tenant_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    result_json TEXT,
    applied_at INTEGER,
    PRIMARY KEY (tenant_id, idempotency_key)
);
```

## Archiving and Snapshots

### S3 Archiving

WAL events are continuously archived to S3:

```
s3://bucket/archive/
  └── tenant_123/
      ├── 2024-01-15/
      │   ├── segment_000001.jsonl.gz
      │   ├── segment_000001.checksum
      │   └── ...
      └── 2024-01-16/
          └── ...
```

**Format**: JSONL with gzip compression

**Retention**: Configurable (default: 90 days)

### SQLite Snapshots

Periodic SQLite snapshots for fast recovery:

```
s3://bucket/snapshots/
  └── tenant_123/
      ├── snap_2024-01-15T00:00:00Z/
      │   ├── canonical.db
      │   ├── mailbox.db
      │   └── manifest.json
      └── ...
```

**Manifest**:
```json
{
  "tenant_id": "tenant_123",
  "timestamp": "2024-01-15T00:00:00Z",
  "wal_position": {
    "topic": "entdb-wal",
    "partition": 0,
    "offset": 12345
  },
  "schema_fingerprint": "sha256:abc..."
}
```

## Recovery Procedures

### Point-in-Time Recovery

Restore to any point after the oldest archive:

```bash
python -m dbaas.entdb_server.tools.restore \
  --tenant-id tenant_123 \
  --target-time "2024-01-15T14:30:00Z" \
  --output-dir /var/lib/entdb/data
```

### Disaster Recovery

Full restore from S3:

```bash
# 1. Find latest snapshot
python -m dbaas.entdb_server.tools.restore list-snapshots \
  --tenant-id tenant_123

# 2. Restore snapshot
python -m dbaas.entdb_server.tools.restore from-snapshot \
  --tenant-id tenant_123 \
  --snapshot-id snap_2024-01-15T00:00:00Z \
  --output-dir /var/lib/entdb/data

# 3. Replay events since snapshot
python -m dbaas.entdb_server.tools.restore replay-archive \
  --tenant-id tenant_123 \
  --from-position 12345 \
  --data-dir /var/lib/entdb/data
```

## Monitoring

### Key Metrics

| Metric | Alert Threshold | Description |
|--------|-----------------|-------------|
| `wal_append_latency_p99` | > 100ms | WAL write latency |
| `applier_lag_seconds` | > 60s | SQLite behind WAL |
| `archive_lag_events` | > 10000 | S3 archive behind |
| `snapshot_age_hours` | > 24h | Time since snapshot |

### Health Checks

```bash
# Check server health
curl http://localhost:8080/health

# Response
{
  "status": "healthy",
  "components": {
    "wal": "connected",
    "sqlite": "ok",
    "s3": "ok"
  },
  "applier_lag_ms": 50,
  "last_snapshot": "2024-01-15T00:00:00Z"
}
```

## Testing Durability

### Chaos Engineering

Test durability with fault injection:

```bash
# Kill random broker
docker kill kafka-1

# Verify writes still succeed
python tests/e2e/test_durability.py

# Verify no data loss
python tests/e2e/verify_consistency.py
```

### Recovery Testing

Regularly test recovery procedures:

1. Take snapshot
2. Destroy SQLite
3. Restore from snapshot
4. Verify data integrity
5. Compare against WAL events

```bash
# Run recovery test
ENTDB_E2E_TESTS=1 pytest tests/e2e/test_recovery.py -v
```
