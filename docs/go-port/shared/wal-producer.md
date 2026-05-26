# WAL Producer — Go Port Spec

Tracks EPIC #407 (Python → Go server port). Scope: the **producer** half of
the WAL stream abstraction. The consumer/applier is a separate spec.

The WAL is the source of truth for every mutation (CLAUDE.md invariant #1).
SQLite is a materialized view rebuilt by replaying the WAL. The Go server
must reproduce the Python producer's durability + ordering contract
exactly, otherwise replay determinism breaks.

Reference Python sources (port targets):
- `server/go/internal/wal/` — protocol + types
- `server/python/entdb_server/wal/{memory,kafka,kinesis,sqs,pubsub,servicebus,eventhubs}.py`
- `server/go/internal/apply/applier.go` — `TransactionEvent`
- `server/go/internal/api/` — handler-side `wal.append` call site
- `tests/python/integration/test_wal_replay_determinism.py` — bedrock contract
- `(legacy Python unit test, removed in Phase 4D)`, `test_wal_backends.py`, `test_wal_backends_config.py`

## TransactionEvent shape

Defined in `apply/applier.py:204-269`. Wire-level it is JSON; on disk it
is whatever the backend stores (Kafka record value, Kinesis blob, etc).
Required fields (validated in `from_dict`, `applier.py:256`):

| Field                | Type            | Notes                                                              |
| -------------------- | --------------- | ------------------------------------------------------------------ |
| `tenant_id`          | string          | Partition key. Per-tenant isolation invariant.                     |
| `actor`              | string          | Trusted actor stamped by handler (`grpc_server.py:780`), not client-supplied. Audit-truth. |
| `idempotency_key`    | string          | Producer-supplied dedupe key. Applier checks `applied_events` table (`applier.py:767,922`). |
| `schema_fingerprint` | string \| null  | Schema registry hash at write time.                                |
| `ts_ms`              | int64           | Wall-clock ms; defaults to `time.time()*1000` if missing.          |
| `ops`                | list[op-dict]   | One or more atomic ops (see below).                                |
| `stream_pos`         | StreamPos       | NOT serialized; populated by applier from record metadata.         |

`ops[i]` is an open-ended dict keyed by `"op"`. Op types handled by the
applier today: `create_node`, `update_node`, `delete_node`, `create_edge`,
`delete_edge`, `delete_where`, `register_schema` (the self-describing
schema op prepended by the ExecuteAtomic handler when client/server
fingerprints differ — see [ADR-031](../../adr/031-self-describing-name-free-schema.md)),
plus admin/GDPR/transfer/revocation ops registered in
`Applier.apply_event` (`applier.py:1327`). The Go event struct must keep
`ops` as a free-form list (`[]map[string]any` or, preferably, a
`[]*pb.Op` once op types are protobufed) — handlers will mutate ops
in-place to assign IDs (`grpc_server.py:768-772`).

`StreamPos` (`base.py:65`): `{topic, partition, offset, timestamp_ms}`,
`String()` form `topic:partition:offset`. This is the receipt the SDK
returns to clients (`grpc_server.py:799`) and the cursor the applier
commits.

## Producer contract

Single method, `append(topic, key, value, headers) → StreamPos`
(`base.py:209`). Semantics:

1. **Durability**: returns *only* after the backend has durably stored
   the record. Kafka uses `acks=all` + idempotent producer
   (`kafka.py:55-76`); Kinesis blocks on `PutRecord` post-replication
   (`kinesis.py:9-13`); SQS FIFO uses content-based dedupe
   (`sqs.py:21-25`). A returned `StreamPos` is a promise that a future
   consumer subscribed at offset 0 will see this record.
2. **Ordering**: total order *within a partition*. The partition is
   chosen by `key`. Handlers always use `tenant_id` as the key
   (`grpc_server.py:792`), so the contract reduces to: **per-tenant
   total order**. No cross-tenant ordering is guaranteed and the
   applier does not depend on it.
3. **Idempotency on retry**: two layers.
   - Backend layer (Kafka idempotent producer, SQS
     `MessageDeduplicationId`, Kinesis client retry): prevents the same
     producer call from writing twice on transient failure.
   - Application layer: the `idempotency_key` field. The applier checks
     `applied_events` before applying (`applier.py:765-768`). If the
     producer retries at a higher level (e.g. handler caller retries
     ExecuteAtomic) it MUST reuse the same `idempotency_key` — the WAL
     may then contain two copies but the applier dedupes.
4. **Failure → no partial write**: if `append` raises, the caller must
   assume nothing was written *or* it was written exactly once. The
   producer never leaves "half a record" visible to consumers
   (`wal/__init__.py:17`).
5. **Headers**: optional `map[string]bytes`. Currently unused by handlers
   but reserved for tracing / encryption metadata.

## Backend inventory

| Backend                                  | File                       | Intent / when chosen                                                                |
| ---------------------------------------- | -------------------------- | ----------------------------------------------------------------------------------- |
| Kafka / Redpanda / MSK                   | `wal/kafka.py`             | Production default. High throughput, multi-partition, mature ecosystem.            |
| AWS Kinesis Data Streams                 | `wal/kinesis.py`           | AWS-native deployments avoiding MSK. Shard-based ordering.                          |
| AWS SQS FIFO                             | `wal/sqs.py`               | Low-to-medium throughput AWS deployments; cheaper than MSK; FIFO + dedupe.          |
| GCP Pub/Sub (with ordering keys)         | `wal/pubsub.py`            | GCP-native deployments.                                                             |
| Azure Service Bus (sessions)             | `wal/servicebus.py`        | Azure-native, session-ordered.                                                      |
| Azure Event Hubs (Kafka-compatible)      | `wal/eventhubs.py`         | Azure-native high throughput.                                                       |
| In-memory                                | `wal/memory.py`            | Tests + local dev. Same ordering guarantees, lost on exit.                          |

Selection happens at startup in `wal/base.py:342` (`create_wal_stream`)
based on `config.wal_backend` (enum at `config.py:70`,
values: `kafka|kinesis|pubsub|sqs|servicebus|eventhubs|local`).

S3 is **not** a producer backend. S3 enters the picture downstream:
archived WAL segments land in S3 with Object Lock (COMPLIANCE mode) for
tamper-evident audit (`audit/__init__.py:4-8`, `audit/s3_lock.py`,
`audit/compliance.py`). The Go port keeps that split — producers never
write to S3 directly.

## Multi-tenancy & ordering

- **Partition key = `tenant_id`** at every call site
  (`grpc_server.py:792`). All backends honor this:
  Kafka → record key, Kinesis → `PartitionKey`, SQS → `MessageGroupId`,
  Pub/Sub → ordering key, Service Bus → `SessionId`, Event Hubs →
  `partition_key`.
- **Per-tenant total order**: yes (required for replay determinism).
- **Global total order**: no, and not needed. The applier indexes
  `applied_events` by `(tenant_id, idempotency_key)`.
- Memory backend hashes the key into `num_partitions` (default 4) to
  simulate the same property (`memory.py:74-88,135`).
- The applier may shard work by tenant (`_assigned_tenants`,
  `applier.py:483`); the producer does not need to know.

## Failure modes

- **Partial write / mid-flight crash**: producer must surface an error
  *or* ensure exactly-once. With Kafka idempotent producer + `acks=all`
  this is the broker's job; `append` either returns `StreamPos` or
  raises `WalError`/`WalTimeoutError` (`base.py:41-62`).
- **Leader loss / broker bounce**: producer reconnects with
  exponential backoff (`wal/__init__.py:20`); in-flight `append` calls
  surface `WalConnectionError` so the handler returns a non-OK gRPC
  status. The client is expected to retry with the same
  `idempotency_key`.
- **Slow consumer / WAL backpressure**: producer is decoupled from
  consumer. Backpressure surfaces as backend-side queue limits (Kafka
  buffer-memory, SQS quota). Producers must NOT block waiting for the
  applier — `wait_applied` is a separate, optional, post-append step
  (`grpc_server.py:804`).
- **Schema mismatch**: producer does not validate `ops`. The applier
  is the gate; a poison event halts the applier
  (`applier.py:430-441,529-533`) but the WAL keeps it (advance-on-poison
  is forbidden — `test_wal_replay_determinism.py:10-19`). Producers
  therefore must not retry "unknown op" errors that came from the
  applier — they're terminal at append time only if validation is added
  client-side later.
- **Idempotency-key reuse with different payload**: undefined. Today
  the applier silently skips on duplicate key (`applier.py:920-924`).
  The Go port should preserve this and document it as a producer
  responsibility.

## Go interface

Target package: `server/go/internal/wal/` (does not yet exist).

```go
package wal

type Event struct {
    TenantID         string
    Actor            string
    IdempotencyKey   string
    SchemaFingerprint string // empty == nil
    TsMs             int64
    Ops              []Op // []*pb.Op once protobufed; []map[string]any meanwhile
}

type StreamPos struct {
    Topic       string
    Partition   int32
    Offset      int64
    TimestampMs int64
}

func (p StreamPos) String() string // "topic:partition:offset"

type Producer interface {
    Connect(ctx context.Context) error
    Close(ctx context.Context) error
    Append(ctx context.Context, topic, key string, value []byte, headers map[string][]byte) (StreamPos, error)
}
```

Notes:
- `Append` takes raw bytes, not `Event`. Encoding (JSON today, proto
  later) lives in a thin wrapper so the producer stays transport-only.
  Mirrors `grpc_server.py:789` where the handler json-encodes before
  calling `wal.append`.
- `ctx` carries deadlines; Python uses `WalTimeoutError`, Go uses
  `ctx.Err()` + a typed `*WalTimeout` wrapper.
- Errors: `WalError`, `WalConnectionError`, `WalTimeoutError`,
  `WalSerializationError` (`base.py:41-62`) → Go sentinel errors with
  `errors.Is` support.
- **Phase 1**: only `InMemoryProducer` (port `memory.py`). Sufficient
  to bring up handlers + applier behind it and run the integration
  suite.
- **Phase 2+**: Kafka first (production critical), then Kinesis, then
  the rest. Each backend lives in its own subpackage
  (`internal/wal/kafka`, etc.) so build tags / optional deps mirror
  Python's `try: import aiokafka` pattern (`kafka.py:42-52`).

## Dependencies

- `context` — replaces Python's implicit asyncio cancellation.
- `time` — `TsMs` and `TimestampMs` generation.
- Future: generated `pb` package for typed `Op` once the proto is
  defined (`proto/entdb/v1/entdb.proto`). Until then, JSON via
  `encoding/json`.
- Per-backend SDKs (Phase 2+): `confluent-kafka-go` or `segmentio/kafka-go`,
  `aws-sdk-go-v2` (kinesis, sqs), `cloud.google.com/go/pubsub`,
  `azservicebus`, `azeventhubs`. Pick at build-tag granularity to keep
  a slim default binary.
- No dependency on the applier package — producer is upstream-only.

## Test surface

Mirror the Python suites:

- `internal/wal/memory_test.go` — port `(legacy Python unit test, removed in Phase 4D)`:
  append/subscribe roundtrip, partition determinism by key, offset
  monotonicity, num_partitions validation, `wait_for_records`.
- `internal/wal/contract_test.go` — port `tests/python/unit/test_wal_backends.go`:
  shared producer-contract tests run against every backend impl
  (durability ack, ordering within key, error types).
- `internal/wal/config_test.go` — `test_wal_backends_config.py`:
  factory selection from config.
- Integration parity: a Go version of
  `tests/python/integration/test_wal_replay_determinism.py` once the
  Go applier exists. Producer-only fixtures should expose the same
  helpers `InMemoryProducer` provides today: `GetAllRecords(topic)`,
  `GetRecordCount(topic)`, `ClearTopic(topic)`, `WaitForRecords(...)`
  (`memory.py:343-403`). These are the bedrock of cross-implementation
  contract tests under `tests/contract/`.

Cross-impl contract tests (`tests/contract/`, currently empty) should
drive the same byte-level WAL through both Python and Go applier
builds and assert identical SQLite output. The producer half of those
tests just needs deterministic JSON encoding — keep field ordering
stable.

## Open questions / risks

- **Backend selection per environment**: today config picks one global
  backend. Multi-region or multi-cloud deployments may want different
  backends per tenant shard. Out of scope for the producer port; flag
  for a future config-routing layer.
- **WAL compaction**: not implemented. Replay-from-zero is the only
  recovery path. If compaction is added, the producer may need to
  emit `tombstone`-style markers — design TBD with the applier team.
- **Replay determinism vs halt-on-poison**: the applier halts on a
  poison event and does NOT advance the WAL offset
  (`applier.py:427-441`, `test_wal_replay_determinism.py:17-19`). The
  producer is unaware of this — but it means the producer MUST NEVER
  retry an `append` by writing a *new* event with a new
  `idempotency_key`; client retries reuse the same key so the applier
  can dedupe. Document in the SDK contract.
- **Op typing**: Python keeps `ops` as `list[dict[str, Any]]`. Go
  benefits from generated proto types but until ops are protobufed,
  the Go side stores them as `json.RawMessage` or
  `[]map[string]any`. Decide before the Kafka backend lands — the
  encoded byte layout becomes the cross-impl contract surface.
- **Header semantics**: currently unused. If we later add tracing
  (W3C traceparent) or per-record encryption metadata, headers
  become load-bearing — producers must propagate them and consumers
  must not strip them.
- **Exactly-once across rebalance**: Kafka idempotent producer is
  per-session; on producer restart with a different `transactional_id`
  duplicates can re-enter the WAL. Today applier dedupes on
  `idempotency_key`; verify the Go Kafka client preserves the same
  semantics before promoting to production.
