# ADR-005: Event Sourcing and WAL Architecture

**Status:** Accepted
**Decided:** original ADR date predates current convention; revised
2026-05-16 to match shipped reality (single-topic, multi-backend)
**Tags:** event-sourcing, wal, durability, replay
**Implementation:** `server/go/internal/wal/`,
`server/go/internal/apply/`

## Decision

EntDB uses event sourcing: every write is appended to a Write-Ahead
Log before being applied to SQLite. The WAL is the source of truth;
SQLite is a materialized view rebuildable by replay. See
[ADR-016](016-handlers-append-applier-writes.md) for the
handler→WAL→applier→SQLite contract and
[ADR-015](015-wal-and-s3-object-lock-as-audit-log.md) for the
audit-log posture that this enables.

### Single topic, partitioned by tenant

All events flow through **one** WAL topic (default: `entdb-wal`).
Per-tenant ordering is preserved by keying the partition on
`tenant_id`. Control-plane events (CreateTenant, CreateUser,
AddTenantMember, …) use a global sentinel key (`__global__`) and
flow through the same topic — there is no separate
`entdb-global` / `entdb-fanout` / etc. The applier dispatches by
event scope (per-tenant vs global) when it consumes.

This is a deliberate departure from the original ADR which proposed
three topics. One topic gives:

- Total event ordering for a given key (Kafka guarantees), which is
  all the system actually needs.
- One retention policy, one archive prefix, one backup story.
- One consumer-group / partition-count knob to scale.

The original three-topic plan made the operator surface larger
without buying ordering guarantees we didn't already have.

### Partition-count guidance

| Tenants | Partitions | Applier instances |
|---|---|---|
| < 50 | 6 | 1-2 |
| 50–500 | 12 | 2-4 |
| 500–5,000 | 24 | 4-8 |
| 5,000+ | 48 | 8-16 |

Partitions can be added later (Kafka allows increase only, not
decrease). Per-tenant ordering is preserved because the partition
key is `tenant_id` — moving a tenant to a different partition
happens only on partition-count change and requires a brief drain.

### WAL backends (swappable)

A common Go interface (`wal.Producer` + `wal.Consumer` in
`server/go/internal/wal/`) abstracts the backend. The Python source
(retired in Phase 4D) shipped 7 backends; the Go port currently
ships 2. **EPIC [#518](https://github.com/elloloop/tenant-shard-db/issues/518)
tracks porting the remaining 5.**

| Backend | Go status | Python source | Use case |
|---|---|---|---|
| In-memory | ✅ `wal/memory.go` | `wal/memory.py` | Tests, dev, ephemeral |
| Kafka / Redpanda / MSK | ✅ `wal/kafka.go` | `wal/kafka.py` | Production default |
| AWS Kinesis | ⏳ unported | `wal/kinesis.py` | AWS-native streaming |
| GCP Pub/Sub | ⏳ unported | `wal/pubsub.py` | GCP-native |
| AWS SQS | ⏳ unported | `wal/sqs.py` | AWS simple / low-throughput |
| Azure Service Bus | ⏳ unported | `wal/servicebus.py` | Azure-native (transactional) |
| Azure Event Hubs | ⏳ unported | `wal/eventhubs.py` | Azure-native (streaming) |

Backend selection is a CLI flag on `cmd/entdb-server/` (today:
`-wal-backend=memory|kafka`). Once #518 lands, additional values
(`kinesis`, `pubsub`, `sqs`, `servicebus`, `eventhubs`) become
available. The interface is stable; switching backends is a
deployment-config change, not a code change.

### Event structure

```protobuf
message WalEvent {
  string tenant_id           = 1;   // or "__global__" for control plane
  string actor               = 2;
  string idempotency_key     = 3;
  string schema_fingerprint  = 4;
  int64  timestamp_ms        = 5;
  repeated Operation operations = 6;
  Scope scope                = 7;   // TENANT | GLOBAL
}
```

### Write path

```
Client → Plan.commit() → gRPC ExecuteAtomic (or admin RPC)
  → server validates schema fingerprint
  → server serializes the event (proto)
  → server appends to the WAL via wal.Producer.Append
  → server returns Receipt with stream_position
  → applier consumes, applies to SQLite (per-tenant) or globalstore
  → if wait_applied=true, server blocks the response until the
    applier reaches the offset
```

The applier itself is described in ADR-016. Idempotency is keyed
on `(tenant_id, idempotency_key)`; replays of an already-applied
event are no-ops.

### Read-after-write consistency

Two opt-in mechanisms, both backed by event-driven notification on
applier offset advancement (not polling):

```
Option 1: plan.commit(wait_applied=True)
  Server blocks the RPC response until the applier reaches the offset.

Option 2: db.get(..., after_offset=receipt.stream_position)
  Server blocks the read until the applier reaches that offset before executing.
```

Default (no option): the response returns once the WAL acks. Reads
issued immediately after may not see the write yet — typical
event-sourcing eventual-consistency window.

### Retention and recovery

```
Kafka retention:    configurable; recommend ≥ snapshot cadence + headroom
S3 archive:         WAL events archived to S3 with Object Lock (ADR-015)
SQLite snapshots:   per-tenant, deferred — replay from WAL is the default DR

Recovery tiers:
  Tier 1: SQLite exists           → read directly
  Tier 2: SQLite lost, WAL exists → replay from WAL (no snapshot needed)
  Tier 3: SQLite + WAL lost       → restore S3 archive → replay
```

Per-tenant SQLite snapshots are an optional optimization (skip the
"replay from beginning" cost on tier-2 recovery). They are not the
durability mechanism — the WAL is.

### Recommended Kafka configuration (production)

```yaml
replication.factor: 3
min.insync.replicas: 2
compression.type: lz4
retention.ms: 604800000          # 7 days minimum; longer for slow archive lag
max.message.bytes: 1048576       # 1 MB
enable.auto.commit: false        # applier commits manually after applying
max.poll.records: 500            # batch size
acks: all                        # producer waits for all in-sync replicas
```

For other backends (Kinesis, Pub/Sub, Service Bus, Event Hubs, SQS),
analogous settings live in each backend's CLI flags / config. The
contract is the same: at-least-once delivery, in-order per partition
key.

## Alternatives considered

- **Three topics (`entdb-events`, `entdb-global`, `entdb-fanout`)
  per the original ADR-005.** Rejected on revision. One topic with
  scope-tagged events handles the same workloads with simpler
  operations. The original split conflated "events for the data
  plane vs control plane" with "topic-level isolation" — the former
  is a field on the event, not a topic boundary. The notification
  fanout topic is moot in any case: the current Go server doesn't
  have a notifications system.

- **Per-tenant topic (one topic per tenant).** Rejected. Kafka
  partition-count scales to thousands per topic; topic-count to
  thousands of topics has higher metadata overhead. Per-tenant
  ordering via partition key is the standard pattern.

- **Single global ordering (one partition).** Rejected. Caps
  throughput to single-partition limits and serializes unrelated
  tenants. Per-tenant ordering is all the system needs.

- **One backend only (Kafka).** Rejected. Real deployments span
  AWS / GCP / Azure; cost-optimized deployments prefer
  cloud-native managed offerings. Keeping the interface and
  shipping backends parallel to demand is cheaper than mandating
  Kafka everywhere. (See EPIC #518 for the remaining ports.)

## Consequences

**What this locks in:**

- One topic (`entdb-wal`), partitioned by `tenant_id`.
- A common Go interface (`wal.Producer` + `wal.Consumer`) for all
  backends. Adding a backend = implementing the interface, not
  refactoring callers.
- Recovery is always "replay the WAL." Snapshots are optimization,
  not the durability mechanism.
- Backend-specific CLI flags follow the existing Kafka pattern
  (`-wal-brokers`, `-wal-topic`, `-wal-group`). New backends get
  their own analogous flags.

**What this makes easy:**

- Operator surface: one topic to scale, one retention to tune, one
  archive prefix.
- Cross-cloud deployment: pick the backend that matches your
  provider; the server doesn't care.
- Disaster recovery: replay from WAL works regardless of which
  backend you're on (the events are the events).

**What this makes harder:**

- Future scenarios that *would* benefit from topic-level isolation
  (e.g. quarantine bad events without affecting good ones) would
  need to reintroduce multi-topic. Today there's no compelling use
  case.
- Multi-backend port effort is non-trivial (#518). Until the 5
  unported backends ship, single-cloud deployments are
  Kafka/Redpanda-only.

**Failure modes:**

- Producer can't reach the WAL: handler returns `Unavailable`. The
  request didn't land in the WAL, so it didn't happen — no
  partial state.
- Applier crashes mid-batch: idempotency keyed on
  `(tenant_id, idempotency_key)`. On restart, the applier resumes
  from the last-committed offset; replayed events are skipped via
  the `applied_events` table.
- Backend offset/checkpoint divergence between Go ports and the
  Python source semantics: covered by the e2e crash-recovery tests
  in `tests/python/e2e/` (Kafka backend today; other backends gain
  coverage as they port via #518).

## References

- [ADR-015](015-wal-and-s3-object-lock-as-audit-log.md) — WAL +
  S3 Object Lock as the audit log.
- [ADR-016](016-handlers-append-applier-writes.md) — the
  handler→WAL→applier→SQLite write-path contract.
- [ADR-014](014-physical-storage-layout.md) — per-tenant SQLite
  files are the materialized view; WAL is the source of truth.
- EPIC [#518](https://github.com/elloloop/tenant-shard-db/issues/518)
  — port the 5 remaining WAL backends from the Python source.
- Source: `server/go/internal/wal/` (interface + memory + kafka),
  `server/go/internal/apply/applier.go` (consumer + dispatch).
- Python history (deleted in commit `8d07f5f`):
  `server/python/entdb_server/wal/{base,memory,kafka,kinesis,pubsub,sqs,servicebus,eventhubs}.py`.
