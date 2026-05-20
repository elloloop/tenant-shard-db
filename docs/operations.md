# Operations Guide

Running EntDB in production. The Go server is gRPC-only; operator work happens via Kafka tooling, cloud-provider dashboards, the `entdb-schema` CLI, and the SDK admin RPCs.

## Production checklist

Before running EntDB in front of real traffic:

- Set `-data-dir` to a persistent volume.
- Set `-wal-backend=kafka` (or another supported backend per [ADR-005](adr/005-event-sourcing-wal.md)) and configure `-wal-brokers` to your MSK / Redpanda cluster.
- Set `-require-tls=true` with `-tls-cert`, `-tls-key` paths. Use `-tls-min-version=1.3` unless a documented legacy client still needs TLS 1.2.
- Enable mTLS for service-to-service callers: `-require-client-cert=true` + `-tls-ca <client-ca>`.
- Set `-kms-provider` (`aws`, `vault`, or `file` — `gcp` / `azure` flags exist but error at boot; not yet implemented) and `-kms-key-id`. Set `-encryption-required=true`.
- Enable the WAL archive: `-archive-enabled=true`, `-archive-bucket`, `-archive-region`, `-archive-retention-days` matching your retention policy. The bucket must have Object Lock COMPLIANCE pre-configured at creation — see [S3 Object Lock archive](#s3-object-lock-archive) for bucket requirements, the full IAM action set, and the ops runbook ([ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md)).
- Keep `-legal-hold-lift-worker-enabled=true` (default) so a released legal hold is lifted on already-archived S3 objects; it only does work when `-archive-enabled`.
- Keep `-gdpr-worker-enabled=true` with `-gdpr-worker-interval` matched to your erasure SLA.
- Set `-metrics-addr` (e.g. `:9090`) so dashboards can scrape `entdb_archive_lag_events` and the legal-hold-lift metrics.

## Health checking

Two health endpoints, both over gRPC on the same port (default `:50051`):

| Method | Purpose |
|--------|---------|
| `/grpc.health.v1.Health/Check` | Standard gRPC health protocol — for load balancers / k8s readiness probes |
| `/entdb.v1.EntDBService/Health` | EntDB-specific: returns version + applier status |

Both bypass the auth interceptor (per [ADR-011](adr/011-security-and-compliance.md)).

```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
# {"status": "SERVING"}
```

For LB health checks, target the gRPC port with `path = "/grpc.health.v1.Health/Check"`. The Go server image is **distroless** — no `curl`, `wget`, or shell inside the container. Container-level `--health-cmd` healthchecks aren't possible; use orchestrator-level probes (ALB / NLB / k8s with `grpc_health_probe`).

## Configuration

The Go server takes **CLI flags only** (no `ENTDB_*` env vars). Pass them as the container's `command:` field (Terraform / k8s / docker compose); SIGHUP reloads the TLS cert/key files without restart. The full flag list lives in [`README.md`](../README.md#configuration).

## Observability

### Metrics

Prometheus-style counters and histograms are collected internally (`server/go/internal/metrics/`):

| Metric | Description |
|---|---|
| `entdb_grpc_requests_total{method,status}` | RPC counts |
| `entdb_grpc_request_duration_seconds{method}` | RPC latency histogram |
| `entdb_wal_append_duration_seconds` | WAL producer latency |
| `entdb_wal_consumer_lag` | Applier lag in events |
| `entdb_archive_lag_events{topic,partition}` | Archive sidecar lag (records polled but not yet on S3) — see [S3 Object Lock archive](#s3-object-lock-archive) |
| `entdb_archive_writes_total`, `entdb_archive_errors_total` | Archive activity / failures |
| `entdb_legal_hold_lift_pending` | Durable legal-hold-lift queue depth (should drain to 0) |
| `entdb_legal_hold_lift_completed_total`, `entdb_legal_hold_lift_errors_total` | Lift sweep completions / failures |

Set `-metrics-addr` (for example `-metrics-addr=:9090`) to expose the Prometheus `/metrics` endpoint on a plain HTTP listener separate from the gRPC port. It is off by default; scrape traffic is expected to be in-cluster. Metric names and labels are stable across releases.

Complementary operational signals:

- **Kafka/MSK consumer-group lag** on `entdb-wal` via `kafka-consumer-groups.sh` or CloudWatch.
- **Client-side latency / error rates** from the SDKs.
- **gRPC health probes** for service liveness.

### Tracing

OpenTelemetry is a transitive dependency but no spans are emitted by the server yet. Tracking issue: [#517](https://github.com/elloloop/tenant-shard-db/issues/517).

### Logging

Structured JSON logs to stdout. Aggregate with your container runtime's log collector (CloudWatch agent, Fluent Bit, Loki Promtail, etc.).

## S3 Object Lock archive

The WAL archive (`-archive-enabled=true`) copies WAL events to S3 with Object Lock COMPLIANCE, giving tamper-evident long-term storage that survives Kafka retention rolling off ([ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md)). The archiver runs as a sidecar goroutine that consumes the WAL topic with its own consumer group (`-archive-group`).

> **EntDB ships no Terraform / IaC module.** It is an OSS Docker image plus SDKs; how you provision the bucket (Terraform, CloudFormation, CDK, console) is your choice. The CLI snippets below are illustrative examples — adapt them to your own tooling.

### Bucket requirements

The archive bucket MUST satisfy all of:

- **Object Lock enabled at bucket creation.** S3 Object Lock can only be turned on when the bucket is *created* — it cannot be added to an existing bucket. (Enabling Object Lock at creation also turns on versioning, which Object Lock requires.)
- **A COMPLIANCE-mode default retention** with a `Days` window matching `-archive-retention-days`. The server verifies COMPLIANCE specifically and rejects GOVERNANCE (a privileged principal can bypass GOVERNANCE).
- **A dedicated bucket with all public access blocked.**

COMPLIANCE retention is true WORM: a `RetainUntil` date in the future is immutable — nobody, including the AWS account root, can shorten it, overwrite the object, or delete it before it expires. The flip side is that a long retention (e.g. `-archive-retention-days=2557`, ~7 years) is a long commitment, including for objects written by a bug. Use a separate dev/staging bucket *without* Object Lock for testing.

The archiver calls `GetObjectLockConfiguration` at boot and **refuses to start** unless the bucket is Object Lock COMPLIANCE — defence in depth so the archiver is never pointed at a non-WORM bucket.

Archive objects are keyed `wal/<topic>/<partition>/<start>-<end>.jsonl.gz` — **one object per WAL partition run, not per tenant**; a single object can carry records for many tenants.

```bash
# Example only — adapt to your own IaC/tooling. EntDB ships no module for this.
# 1. Object Lock MUST be enabled at create time (cannot be added later;
#    also turns on versioning automatically).
aws s3api create-bucket \
  --bucket entdb-audit-prod --region us-east-1 \
  --object-lock-enabled-for-bucket

# 2. Block all public access.
aws s3api put-public-access-block \
  --bucket entdb-audit-prod \
  --public-access-block-configuration \
    "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"

# 3. COMPLIANCE default retention matching -archive-retention-days.
aws s3api put-object-lock-configuration \
  --bucket entdb-audit-prod \
  --object-lock-configuration \
    '{"ObjectLockEnabled":"Enabled","Rule":{"DefaultRetention":{"Mode":"COMPLIANCE","Days":2557}}}'
```

### IAM permissions

The server role needs the archive-write actions plus the legal-hold-lift sweep actions. Every action below is exercised by a real code path in `server/go/internal/audit/` (archiver + legal-hold lift), not listed speculatively:

| IAM action | Used by | S3 call |
|---|---|---|
| `s3:GetBucketObjectLockConfiguration` | Boot verification | `GetObjectLockConfiguration` (refuses to start unless COMPLIANCE) |
| `s3:PutObject` | Archive write | `PutObject` with COMPLIANCE retain-until (+ legal-hold ON when held) |
| `s3:PutObjectLegalHold` | Archive write & lift sweep | Sets hold ON at write for a held tenant; clears OFF on release |
| `s3:GetObjectLegalHold` | Lift sweep | Skip objects already OFF |
| `s3:ListBucket` | Lift sweep | `ListObjectsV2` over the `wal/` prefix |
| `s3:GetObject` | Lift sweep | Decode object bodies to recover the authoritative tenant set |

Add `kms:GenerateDataKey` + `kms:Decrypt` on the key only when you set `-archive-kms-key-id`. Notably **absent**: `s3:DeleteObject` (the archive is append-only) and `s3:PutObjectRetention` / `s3:BypassGovernanceRetention` (retention is immutable; only the legal-hold flag is ever mutated).

### Retention vs legal hold

Two independent Object Lock controls:

- **COMPLIANCE retention** — set by the archiver on every write; immutable until expiry; cleared by nothing but time.
- **Legal hold** — stamped ON for a tenant under hold (`SetLegalHold` ON); persists even after retention expires; liftable only via an explicit `SetLegalHold` OFF.

An object is undeletable while it has either an unexpired retention **or** an active legal hold.

Releasing a hold (`SetLegalHold` OFF) flows through the WAL; the apply step that clears the hold also enqueues a row in a durable `legal_hold_lift_queue` *in the same transaction* (crash-durable). The `-legal-hold-lift-worker-enabled` worker (default on, `-legal-hold-lift-worker-interval` default `1m`) drains the queue, running an idempotent, resumable sweep of the `wal/` prefix: for each object with legal-hold ON it decodes the body to find the tenants it carries and clears the hold **only when none of them is still under hold** (a co-tenant still on hold keeps the object ON). The queue row is deleted only on full success; the COMPLIANCE retention is never touched. The worker only does work when `-archive-enabled` is on.

GDPR erasure never lifts a hold — legal hold supersedes erasure. While a tenant is held, `DeleteUser` refuses to queue erasure; only an explicit `SetLegalHold` OFF clears the hold, after which queued erasure may proceed.

### Runbook: archive lag

`entdb_archive_lag_events{topic,partition}` is the number of WAL records polled from a partition but not yet durably written to S3 and committed. A healthy archiver drives it to 0 after each cycle. A **sustained non-zero lag means the archiver is falling behind or S3 is unreachable** — events live only in Kafka/Redpanda, outside tamper-evident storage.

```promql
# Page when archive lag is stuck above zero for 10 minutes.
max(entdb_archive_lag_events) > 0 for 10m
# Corroborate with the error counter.
rate(entdb_archive_errors_total[5m]) > 0
```

When lag is sustained:

1. Check `entdb_archive_errors_total` and server logs for the underlying S3 error.
2. Confirm S3 reachability and that the role still has `s3:PutObject` + `s3:GetBucketObjectLockConfiguration` (a policy change or KMS key disablement is a common cause).
3. Confirm the bucket still has Object Lock COMPLIANCE configured.
4. You usually don't intervene on the data: keep Kafka/Redpanda retention longer than your worst-case archive lag so events are never lost before reaching S3. Once S3 recovers, the archiver resumes from its committed offset (retrying with `-archive-retry-backoff`) and lag drains to 0.

### Runbook: legal-hold lift

`entdb_legal_hold_lift_pending` is the durable lift-queue depth — tenants whose hold was released but whose archived objects have not been fully swept clear. It climbs on release and **should drain back to 0**. Sustained `entdb_legal_hold_lift_errors_total` growth means a stuck lift — almost always an S3 permission or availability problem.

```promql
# A release hasn't drained for 30 minutes (the worker ticks every minute).
entdb_legal_hold_lift_pending > 0 for 30m
# The lift sweep is erroring repeatedly.
rate(entdb_legal_hold_lift_errors_total[10m]) > 0
```

When pending stays non-zero or errors climb:

1. Check server logs for the lift sweep error.
2. Confirm the role has the lift-sweep actions: `s3:ListBucket`, `s3:GetObject`, `s3:GetObjectLegalHold`, `s3:PutObjectLegalHold`.
3. No manual replay is needed — the durable queue retries automatically every tick and the sweep is idempotent/resumable. Once the underlying problem is fixed, the next tick completes the lift and the gauge drains.
4. Objects shared with a co-tenant that is *still* legitimately held correctly stay ON; that is not an error.

## Kafka / WAL operations

The WAL is the source of truth. Most operator tasks touch Kafka, not EntDB.

### Inspect WAL events

```bash
kafka-console-consumer.sh \
  --bootstrap-server "$BROKERS" \
  --topic entdb-wal \
  --max-messages 5 \
  --property print.key=true
```

Key: `tenant_id` (or `__global__` for control-plane events per [ADR-016](adr/016-handlers-append-applier-writes.md)).

### Consumer-group lag

```bash
kafka-consumer-groups.sh \
  --bootstrap-server "$BROKERS" \
  --describe \
  --group entdb-applier
```

Unbounded growth → the applier is falling behind. Check server logs, SQLite I/O saturation, `-data-dir` free space.

### Partition scaling

The applier consumes one partition per goroutine ([ADR-005](adr/005-event-sourcing-wal.md)). Adding partitions allows more concurrent appliers across the server fleet without breaking per-tenant ordering (the partition key is `tenant_id`).

```bash
kafka-topics.sh --bootstrap-server "$BROKERS" \
  --alter --topic entdb-wal --partitions 16
```

Partition count can only increase. Adding partitions during a deployment means tenants may briefly map to different partitions; drain applier + restart is the cleanest path.

## Backup and recovery

### What's durable

| Surface | Authoritative? | Notes |
|---|---|---|
| WAL (Kafka topic `entdb-wal`) | **Yes** | Source of truth. Set retention to span your DR window. |
| S3 Object Lock archive (`-archive-enabled=true`) | **Yes, immutable** | COMPLIANCE mode; tamper-evident. See [ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md). |
| Per-tenant SQLite files in `-data-dir` | **No** | Derived view; rebuildable by WAL replay. SQLCipher-encrypted with the tenant key (stored in `global.db`'s `tenant_key_vault`). |
| Global SQLite (`global.db`) | **Hybrid** | Tenant-data ops are WAL-derived; `tenant_key_vault` rows are unique state — back up `global.db` separately if you care about key material. |

### Recovery procedure

Lost a server / SQLite files but Kafka still has the events:

1. Spin up a fresh `entdb-server` with empty `-data-dir`, same `-wal-group=entdb-applier`, same `-kms-provider` + `-kms-key-id` (so it can unwrap tenant keys from `tenant_key_vault`).
2. The applier rejoins the consumer group at the last committed offset.
3. Per-tenant SQLite files are reconstructed as events apply.
4. Service resumes once the applier catches up.

Lost everything including `global.db`:

1. Restore `global.db` from your backup (operator-owned; mechanism not built-in).
2. Then proceed as above.

If you have no `global.db` backup, every tenant's data is unrecoverable — `tenant_key_vault` was the only place the per-tenant keys were stored. This is the deliberate property of crypto-shred ([ADR-011](adr/011-security-and-compliance.md)): GDPR erasure destroys the key, so the keys must be recoverable only from `global.db`.

### Snapshots

Per-tenant SQLite snapshots are **not** implemented. The WAL is the durability mechanism; replay handles all recovery scenarios. If `-archive-enabled=true`, the S3 Object Lock archive gives tamper-evident long-term storage even after Kafka retention expires.

If your recovery floor needs to be shorter than "replay from offset 0," configure longer Kafka retention or enable the archive.

## Schema operations

The `entdb-schema` CLI ([guide](guides/schema-lockdown.md)) is the operator tool for runtime schema lockdown.

```bash
# Snapshot what the running server has registered
entdb-schema snapshot --from-server localhost:50051 > .schema-snapshot.json

# Compare against a committed baseline (CI)
entdb-schema check --baseline .schema-snapshot.json --from-server localhost:50051

# Diff two snapshots
entdb-schema diff --old .schema-snapshot.json --new /tmp/new.json
```

A schema compatibility check is the canonical CI gate. See `docs/guides/schema-lockdown.md` for the full workflow.

## Tenant onboarding

Every tenant + user + membership must be created via admin RPCs before the data plane will serve writes. See [Onboarding](onboarding.md) for the production three-RPC flow and admin-actor wiring.

## Scaling

### Horizontal

The server is stateless beyond `-data-dir`. Run N replicas behind a gRPC-aware load balancer:

- Distinct `-data-dir` per replica, OR
- Shared persistent volume with single-writer guarantees (SQLite isn't safe under concurrent writes from multiple processes).

All replicas share the same `-wal-group`; Kafka distributes partitions across them. The applier is single-threaded per partition.

### Vertical

For high-write tenants, SQLite cache and busy-timeout can be tuned via build-time pragmas (`server/go/internal/store/`); the current defaults follow standard SQLCipher + WAL-mode recommendations.

## Troubleshooting

### `UNIMPLEMENTED` from a real RPC

The server returns `Unimplemented` when `-data-dir` is unset. Set `-data-dir` for real persistence.

### High applier lag

Symptoms: writes go to the WAL but reads don't reflect them; consumer-group `LAG` climbs.

Diagnose:

- Server logs for apply errors.
- Disk space on `-data-dir`.
- SQLite contention (a single very large transaction holding the writer).
- Network latency to Kafka/MSK.

Mitigate: more replicas + more partitions (each server pulls fewer partitions), or shorter transactions on the write path.

### `UNAVAILABLE: connection refused`

```bash
# Local:
docker compose ps
docker compose logs server --tail 50

# Production:
# Orchestrator pod/task status, then:
grpcurl -plaintext "$HOST:$PORT" grpc.health.v1.Health/Check
```

The server is gRPC-only. HTTP `/health` probes will always fail.

### `NOT_FOUND: tenant "<id>" not found`

Onboarding skipped. See [Onboarding](onboarding.md).

### `PERMISSION_DENIED: actor is not a member of "<tenant>"`

The wire `actor` exists in `users` but has no row in `tenant_members` for the target tenant. Call `Admin.AddTenantMember` with a `system:` / `admin:` actor.

### `UNAUTHENTICATED: missing or invalid credentials`

Auth interceptor rejected the request. Check the credential carrier:

- Bearer token: `authorization: Bearer <jwt>`. JWKS must reach the configured OAuth validator.
- API key: `x-api-key: <secret>`. Key must be registered server-side.
- Session: `x-session-token: <token>`. Session must not have expired.
- mTLS: client cert must validate against `-tls-ca` when `-require-client-cert=true`.

### `INVALID_ARGUMENT: payload ... contains name-keyed field "..."` 

The SDK predates v1.12.2's client-side name→id translation (see [ADR-018](adr/018-field-id-keyed-payloads.md)) or is operating on an unregistered type. Upgrade the SDK, or register the schema on the server.

### Schema-compatibility CI failure

`entdb-schema check` flagged a breaking change. See [Schema Evolution](schema-evolution.md).

## Support

For issues:

1. Collect:
   - EntDB version (`grpcurl -plaintext "$HOST:$PORT" entdb.v1.EntDBService/Health`)
   - Server flags (container `docker inspect`, k8s manifest, or task definition)
   - Recent logs
   - Kafka consumer-group lag for `entdb-applier`
2. Search [existing issues](https://github.com/elloloop/tenant-shard-db/issues).
3. Open a new issue with the above.

## Related

- [Deployment](deployment.md) — production deployment topology
- [Onboarding](onboarding.md) — tenant/user/member setup
- [Durability](durability.md) — what the WAL guarantees
- [ADRs](adr/) — design rationale
