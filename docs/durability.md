# Durability Guarantees

EntDB is event-sourced. The WAL is the source of truth; SQLite is a materialized view. This page documents what's actually guaranteed by the write path and how to operate the system within those guarantees.

For the architectural decision, see [ADR-005](adr/005-event-sourcing-wal.md) and [ADR-016](adr/016-handlers-append-applier-writes.md).

## Write path

```
Client → gRPC handler → WAL Producer → Kafka/Redpanda topic
                                              │
                                              ▼
                                       Applier (per partition)
                                              │
                                              ▼
                                       Per-tenant SQLite
                                       (SQLCipher-encrypted)
                                              │
                                              ▼ (when -archive-enabled)
                                       S3 with Object Lock COMPLIANCE
                                       (tamper-evident archive, ADR-015)
```

1. **Handler validates** the request (admin gate, schema fingerprint).
2. **WAL append** with `acks=all` waits for all in-sync replicas.
3. **Handler returns** once the WAL acknowledges. The write is durable at this point.
4. **Applier consumes** the WAL and writes per-tenant SQLite asynchronously.
5. **Archiver (optional)** writes the same events to S3 with Object Lock — immutable, tamper-evident.

## Durability guarantee

**A write is durable when the handler returns success.** The data is persisted in the WAL with all in-sync replicas acknowledged before the response. The applier writing SQLite is a derived consequence that can be repeated on any node by replay.

The handler does not wait for SQLite by default. If you need read-your-write semantics (your *next* RPC must see the value), pass `wait_applied=true` (Python) / `entdb.WithWaitApplied(true)` (Go) on the commit — the handler then blocks until the applier has materialized the write into SQLite.

## Recommended Kafka configuration

| Setting | Value | Reason |
|---------|-------|--------|
| `replication.factor` | 3 | Survive 2 broker failures |
| `min.insync.replicas` | 2 | Require 2 replicas before ack |
| `unclean.leader.election.enable` | false | Prevent data loss on leader election |
| `retention.ms` | ≥ archive lag headroom; production default 7 days | If archive sidecar isn't running, set longer |
| `compression.type` | `lz4` | Throughput-friendly |
| Producer: `acks` | `all` | Set by EntDB; do not override |
| Producer: `enable.idempotence` | `true` | Prevent duplicate writes on retry |

Server-side, the corresponding `entdb-server` flags are described in [`operations.md`](operations.md#configuration) and the full reference in [`README.md`](../README.md#configuration). The server does not expose Kafka producer config via flags — it uses sensible defaults wired in `server/go/internal/wal/kafka.go`.

## Failure scenarios

### Broker failure

One broker fails during a write.

- Producer retries to remaining brokers.
- Write succeeds if remaining ISR ≥ `min.insync.replicas`.
- Client sees a small latency bump.
- **Data impact:** none.

### Network partition

Network split isolates some brokers.

- Writes may fail if ISR drops below `min.insync.replicas`.
- Handler returns `UNAVAILABLE`.
- Client retries with the same idempotency key.
- **Data impact:** none — the write either succeeds or fails atomically.

### Server crash mid-write

Server crashes after the WAL append but before responding (or before the applier commits).

- WAL has the event.
- On restart, the applier resumes from the last committed offset.
- SQLite catches up to the WAL.
- The client may see `UNAVAILABLE` and retry; the idempotency key prevents double-apply.
- **Data impact:** none.

### Per-tenant SQLite corruption

Tenant SQLite file is corrupted (disk error, bug, etc.).

- Detect via SQLite integrity check or applier error logs.
- Stop the server, delete the corrupted file from `-data-dir`, restart.
- The applier replays the WAL from the configured retention floor (or from the archive if `-archive-enabled=true`).
- **Data impact:** none, provided the WAL still has the events (or the archive does).

### Loss of `-data-dir` entirely

Volume lost, region failed, etc.

- New server, empty `-data-dir`, same `-kms-provider` / `-kms-key-id`.
- Restore `global.db` from operator-managed backup (the `tenant_key_vault` rows are unique state; see [operations.md § Backup and recovery](operations.md#backup-and-recovery)).
- Applier rejoins `-wal-group` and replays.
- **Data impact:** none, provided `global.db` was backed up. Without `global.db`, the per-tenant encryption keys are lost and the data is unrecoverable — by design of crypto-shred ([ADR-011](adr/011-security-and-compliance.md)).

### Loss of the WAL

The Kafka cluster is destroyed, no archive enabled.

- All durability is gone. WAL is the source of truth.
- Mitigation: run multi-AZ Kafka, run the archive sidecar (`-archive-enabled=true`), keep `-archive-bucket` in a different region than the cluster if your DR plan requires cross-region.

### Loss of WAL **and** archive

Everything gone. No data is recoverable. This is the same disaster as losing every replica of a database — out of scope for built-in durability.

## Idempotency

Every mutating RPC carries an idempotency key. The same key always returns the same result without re-applying.

```python
result = await client.atomic(
    lambda plan: plan.create(schema_pb2.User(...)),
    idempotency_key="create_user_alice_v1",
)
```

```go
plan.Commit(ctx, entdb.WithIdempotencyKey("create_user_alice_v1"))
```

**Guarantees:**

- Same `(tenant_id, idempotency_key)` always returns the same result.
- Survives server restarts (tracked in the per-tenant `applied_events` table).
- Survives WAL replay (the applier checks the table before applying).

```sql
CREATE TABLE applied_events (
    tenant_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    result_json TEXT,
    applied_at INTEGER,
    PRIMARY KEY (tenant_id, idempotency_key)
);
```

The SDKs auto-generate idempotency keys; callers can override for deterministic deduplication of business-level operations.

## Archive and recovery

### S3 Object Lock archive (`-archive-enabled=true`)

When enabled, the archiver continuously consumes WAL events and writes them to S3 with Object Lock COMPLIANCE mode for the configured retention window (`-archive-retention-days`).

- Object Lock prevents deletion or modification, even by root, within the retention window — see [ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md).
- Operator-owned bucket policy must have Object Lock pre-enabled; the archiver verifies at boot and refuses to start otherwise.
- Optional SSE-KMS via `-archive-kms-key-id` on top of Object Lock for an additional encryption layer.

### Recovery from archive

If Kafka retention has expired beyond your recovery window but the archive has the events, the operator can re-seed Kafka from the archive (manual process; tooling for this is operator-owned today, not built-in).

The simpler/intended recovery path is "Kafka retention is your DR floor; archive is for compliance / long-term audit, not for routine restores." Set Kafka retention to span your worst-case recovery scenario plus a comfortable margin.

### Snapshots

Per-tenant SQLite snapshots are **not built in**. The WAL is the durability mechanism; replay from offset 0 (or from the last committed offset for the applier's consumer group) handles all recovery scenarios. If your recovery floor needs to be shorter than "replay from Kafka offset 0," configure longer Kafka retention or enable the archive.

## Monitoring durability

Key metrics (see [operations.md § Metrics](operations.md#metrics)):

| Metric | Suggested alert |
|---|---|
| `entdb_wal_append_duration_seconds{p99}` | > 100 ms |
| `entdb_wal_consumer_lag` | > 10000 (applier behind WAL) |
| `entdb_archive_lag_events` | > 10000 (archive behind WAL) |
| Kafka consumer-group lag for `entdb-applier` | unbounded growth |
| `entdb_archive_errors_total` | non-zero for sustained periods |

The server doesn't yet expose `/metrics` over HTTP — until that lands, operational signals come from the Kafka consumer-group lag (broker-side) plus client-side latency observations from the SDKs.

## Health checks

```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
# {"status": "SERVING"}
```

The server is gRPC-only. There is no HTTP `/health` endpoint.

## Testing durability

Chaos / fault injection is operator-owned today. Suggested approaches:

- **Kafka broker failure**: kill one of N brokers during a write campaign; verify the SDK retries succeed and idempotency keys prevent duplicates.
- **Server kill mid-flight**: SIGKILL the server while a Plan is being committed. Verify the WAL has the event (broker-side) and that the applier replays correctly on restart.
- **SQLite delete mid-flight**: stop applier, delete a tenant's SQLite file, restart. The applier should rebuild from the WAL.
- **Crypto-shred + replay**: schedule a `DeleteUser`, let the grace period expire, verify that the tenant's data is unreadable on disk and that the archive can be retrieved but is gibberish.

The e2e suite (`bash tests/python/e2e/run-e2e.sh`) covers some of these — crash recovery, encryption coverage, TLS — see `tests/python/e2e/`.

## Related

- [ADR-005](adr/005-event-sourcing-wal.md) — event-sourcing + WAL backend design
- [ADR-011](adr/011-security-and-compliance.md) — encryption, crypto-shred, key vault
- [ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md) — WAL + Object Lock as the audit log
- [ADR-016](adr/016-handlers-append-applier-writes.md) — write-path contract
- [Operations](operations.md) — production operations
- [Deployment](deployment.md) — production deployment topology
