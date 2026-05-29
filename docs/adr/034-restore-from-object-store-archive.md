# ADR-034: Opt-in restore from the object-store WAL archive (disaster recovery)

**Status:** Accepted
**Decided:** 2026-05-29
**Tags:** recovery, durability, wal, s3, object-lock, disaster-recovery, applier

**Implementation:** archive reader + restore orchestrator
(`server/go/internal/audit/archive_reader.go`,
`server/go/internal/restore/`); WAL `Seek` primitive
(`server/go/internal/wal/wal.go`, `memory.go`, `kafka.go`); boot wiring
(`server/go/cmd/entdb-server/main.go`, `--restore-from-archive`).

## Context

On a normal restart EntDB reads the durable on-disk per-tenant SQLite and
resumes the live WAL consumer from its committed offset (see
[ADR-016](016-handlers-append-applier-writes.md)). That local-disk fast
path is sufficient for ordinary restarts and stays the default.

But if a node **loses its local SQLite** — fresh node, ephemeral disk,
region failover, disaster — there was **no way to rebuild it**. The
object store already holds the full, immutable history: per
[ADR-015](015-wal-and-s3-object-lock-as-audit-log.md) the archiver ships
every WAL record to S3/Azure as gzip-JSONL under Object Lock COMPLIANCE
retention. But that archive was **write-only** — nothing read it back —
and `Applier.Replay`'s `fromPos` was a documented no-op (no backend
implemented the `Seek` primitive it needs). So the durable backup existed
but could not be used to recover.

## Decision

Add an **opt-in** restore path that rebuilds local SQLite from the
object-store WAL archive, then hands off to the live queue. Local-disk
recovery stays the default; restore is never automatic.

This is the **read side of ADR-015** realized as recovery, and a concrete
implementation of the deterministic-replay rebuild ADR-016 describes.

### Trigger

A boot flag `--restore-from-archive` (off by default) runs the restore
**before** the live applier loop starts. To avoid clobbering good data it
**refuses to run against a non-empty data-dir** unless `--restore-force`
is also set (restore is idempotent — see below — so force is safe but
must be deliberate). It reuses the existing `--archive-*` config (bucket,
endpoint, credentials) as the source. An optional `--restore-tenant`
filter restores a single tenant; default is all.

### Mechanism (reuses the applier, no new write path)

The archive *is* the WAL, so restore replays it through the **same
applier** that the live path uses — honoring ADR-016 ("only the applier
writes SQLite") and inheriting idempotency, per-tenant ordering, and
schema establishment (the leading `register_schema` op per ADR-031, so
the registry is rebuilt too) for free.

1. **Read.** An `ArchiveReader` lists objects under `wal/{topic}/`,
   reads the authoritative `entdb-partition` / `entdb-start-offset` /
   `entdb-end-offset` object metadata, and orders them by
   `(partition, start-offset)` **numerically** (not by lexical S3 list
   order — `10-20` sorts before `9-9` lexically). It downloads, gunzips,
   verifies the `entdb-sha256`, and decodes each gzip-JSONL line back
   into a `wal.Record` (the `value` field is the original WAL `Event`
   JSON; `value_base64` for non-JSON).
2. **Replay.** The decoded records are served to an applier through an
   in-memory, archive-backed `wal.Consumer` adapter (so all existing
   apply machinery applies unchanged) writing into the fresh per-tenant
   SQLite under `--data-dir`. Replay drains when the archive is
   exhausted.
3. **Track.** The orchestrator records the max applied `StreamPos` per
   `(topic, partition)`.
4. **Hand off.** For each partition it `Seek`s the **live** consumer
   group (`--wal-group`) to `lastRestoredOffset + 1`, so when the live
   applier starts it resumes **exactly after** the restored history — no
   gap, no full re-replay of the topic, no dependence on broker retention
   covering the restored range.

### The `Seek` primitive

`wal.Consumer` gains:

```go
// Seek sets the committed offset for (topic, groupID, partition) to
// offset, so the next PollBatch/Subscribe for that group resumes at
// offset. Used by restore to hand off to the live tail. Returns
// ErrSeekUnsupported on backends that cannot reposition a group.
Seek(ctx context.Context, topic, groupID string, partition int32, offset int64) error
```

- **In-memory:** sets the `committed` map entry.
- **Kafka:** commits the group offset via a sarama `OffsetManager`
  (a group offset commit, no consumption).
- **Other cloud backends:** return `ErrSeekUnsupported` for now; restore
  is supported on the backends that can reposition a group
  (in-memory + Kafka), and the flag errors clearly on unsupported
  backends. This finally implements the primitive `Applier.Replay`'s
  comment anticipated.

### Idempotent + resumable

Apply is idempotent (the in-txn `applied_events` probe, ADR-016) and the
restore commits per record, so a restore interrupted midway can simply be
re-run: already-applied records take the SKIP path and the resume point
advances. This is why `--restore-force` against a partially-restored
data-dir is safe.

## Alternatives considered

- **Auto-restore when the data-dir is empty.** Rejected as the default:
  surprising and risky (a misconfigured volume mount would silently pull
  the whole history). Restore is an explicit operator action.
- **A separate `entdb-restore` binary.** Rejected for now: the restore
  needs the exact applier/store/registry wiring `entdb-server` already
  builds; a flag reuses it without duplicating bootstrap. A standalone
  verb can wrap this later if needed.
- **Restore by copying SQLite snapshots to S3** (physical backup) instead
  of replaying the WAL archive. Rejected: the WAL archive is already the
  immutable source of truth (ADR-015); physical snapshots would add a
  second backup mechanism and lose the per-record audit granularity and
  Object Lock guarantees.
- **Read S3 directly into the live consumer position without replay.**
  Not possible — SQLite is a materialized view; it must be rebuilt by
  applying the events.

## Consequences

- A node that lost its local SQLite can be rebuilt from the object-store
  archive with one flag, then seamlessly resume the live queue.
- The object-store archive gains a read side; the `Seek` primitive is now
  real for in-memory + Kafka.
- Restore is bounded by what the archiver has flushed to the object store
  (archive lag); records appended to the live WAL but not yet archived
  are recovered from the live queue tail after the `Seek` handoff,
  provided broker retention still covers them — the archive + live-tail
  union is the recoverable set.
- Default behavior is unchanged: no flag, no restore.
