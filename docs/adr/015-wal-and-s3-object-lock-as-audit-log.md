# ADR-015: WAL + S3 Object Lock is the audit log

**Status:** Accepted
**Decided:** 2026-05-14
**Tags:** audit, durability, compliance, wal
**Implementation:** _this commit_ (design); #510 (admin-plane WAL routing — closed by d8d0afd); #511 (S3 Object Lock archive — open); #513 (residual shared_index cascade — open)

## Decision

EntDB has **one** audit log: the WAL (Kafka/Redpanda topic), archived
to S3 with **Object Lock in COMPLIANCE mode** for tamper evidence.
Every operation MUST flow through the WAL — tenant-data ops and
admin/control-plane ops alike. There is **no** per-tenant `audit_log`
SQLite table, no hash-chained event table, no separate audit
mechanism.

The two halves:

1. **WAL as the event stream.** Every mutation is an event on the
   Kafka topic before any state change. The applier consumes the
   topic and materializes per-tenant SQLite + globalstore. SQLite is
   a derived view; the WAL is authoritative.
2. **S3 Object Lock as tamper evidence.** An archiver continuously
   writes WAL events to S3 with `ObjectLockMode=COMPLIANCE` and a
   per-object retention. Even root cannot delete or modify the
   archive within the retention window. Legal hold escalates this
   further (deletion blocked indefinitely).

Together: every action is recorded, the recording is immutable, and
the recording can be replayed to reconstruct any past state.

## Context

The retired Python server had three audit-shaped surfaces: the
Kafka WAL, a per-tenant `audit_log` SQLite table with a SHA-256 hash
chain (`prev_hash` column), and an S3 archive of WAL segments with
Object Lock. The hash-chained SQLite table was redundant — it
recorded the same events the WAL already carried, but with weaker
tamper evidence (any operator with SQLite write access could rewrite
both the row and the hash chain; S3 Object Lock COMPLIANCE refuses
even root). The Go port deliberately omitted the SQLite table
(verified: `server/go/internal/store/schema.go:133` cites this
invariant).

Implementation status:

- **Admin-plane WAL routing — closed (#510, commit `d8d0afd`).**
  The 13 admin RPCs (CreateTenant, CreateUser, AddTenantMember, ...)
  now flow through the WAL via dedicated op types; the applier is
  the sole writer of `globalstore`. One residual cascade
  (`RemoveGroupMember` shared_index cleanup) remains and is tracked
  in #513.

- **S3 Object Lock archive — open (EPIC #511).** Python had
  `audit/s3_lock.py` and an archiver goroutine; neither has a Go
  counterpart yet. The WAL today lives only in the Kafka/Redpanda
  topic, which is durable but not tamper-evident against a
  sufficiently-privileged operator. The tamper-evidence half of
  the design is design-locked; implementation is queued.

## Alternatives considered

- **Hash-chained `audit_log` table per tenant (the Python design).**
  Rejected. (1) Redundant with the WAL — every event is already
  there. (2) Weaker tamper evidence: SQLite is writable by anyone
  with the file, and the hash chain can be rewritten end-to-end with
  a small script. S3 Object Lock COMPLIANCE refuses modification at
  the storage layer, not at the application layer — qualitatively
  different threat model. (3) Doubles write amplification:
  every operation pays one WAL append + one audit-table insert. (4)
  Breaks ADR-016 (the audit row would be a direct SQLite write
  bypassing the WAL).

- **Hash-chained table on top of WAL events (no separate row, but
  chain across WAL events).** Rejected. The Kafka topic itself
  preserves ordering and offsets per partition; a hash chain on top
  adds overhead without buying anything stronger than the broker's
  immutable log + S3 Object Lock can already provide.

- **S3 versioning instead of Object Lock.** Rejected. S3 versioning
  retains old versions when an object is overwritten, but doesn't
  prevent the delete operation; an attacker with delete permissions
  can purge versions. Object Lock COMPLIANCE refuses delete at the
  bucket policy level. Compliance frameworks (SOC2, HIPAA) demand
  the stronger guarantee.

- **External audit service (e.g. AWS CloudTrail, Datadog audit
  events).** Rejected as the *primary* audit log. The application
  emits the events; CloudTrail-style services audit *infrastructure*
  changes (who logged into AWS, who touched the bucket). The two are
  complementary, not substitutes. EntDB still emits OS-level audit
  events for AWS, but the application's own event log is in the WAL.

## Consequences

**What this locks in:**

- One audit log, one mechanism. New auditable operations add a WAL
  op type and an applier path; they do not get a side-channel.
- The Go server DDL never creates an `audit_log` table or any
  hash-chain column. CI / startup must fail if such a table appears.
- Once EPIC #511 ships, the prod S3 bucket MUST have Object Lock
  COMPLIANCE-mode enabled. The archiver verifies this at boot and
  refuses to start otherwise (defense in depth — operators can't
  accidentally turn off tamper evidence).
- Legal hold escalates via S3 Object Lock's `LegalHold=ON` flag on
  per-tenant archive object prefixes. No application-layer legal
  hold mechanism duplicates this.

**What this makes easy:**

- Replay = audit query. "What happened to tenant X between T1 and
  T2?" is a Kafka offset range or an S3 prefix scan. Same mechanism
  for debugging, compliance audit, and disaster recovery.
- The audit log is automatically consistent with the application
  state — by construction, since the application state is derived
  from the audit log.
- Crypto-shredding (deleting per-tenant encryption keys) automatically
  shreds the audit trail too, satisfying GDPR erasure even on the
  immutable S3 archive. (The events are still there, but unreadable.)
- New operations don't have to remember to emit an audit row — if
  it's a WAL op, it's audited.

**What this makes harder:**

- The audit log isn't human-readable in the moment. You look at it
  via `entdb-schema` / future tooling, not by `SELECT * FROM
  audit_log`. Acceptable cost.
- Once an event is on S3 with Object Lock COMPLIANCE, retention
  cannot be shortened. Setting retention to 7 years means a deletion
  bug landing in the archive is unremovable for 7 years. Mitigation:
  cautious retention defaults, separate dev/staging buckets without
  Object Lock for testing.
- GDPR erasure requires crypto-shredding (deleting tenant key)
  rather than deleting events, because the events are immutable.
  This is intentional but requires careful key-management design.

**Failure modes:**

- An admin RPC slips a direct globalstore write past code review.
  Detected by: grep for `globalstore.{CreateTenant,CreateUser,Add*}`
  call sites outside `server/go/internal/apply/`. EPIC #510 already
  closed the original 13-RPC carve-out; the grep currently has one
  expected hit (`RemoveGroupMember` cascade, #513). Any other hit is
  a regression.
- The S3 archiver lags or crashes, leaving a window where events
  exist in Kafka but not in tamper-evident storage. Detected by:
  `entdb_archive_lag_events` metric. Mitigated by: Kafka retention
  being a longer floor than typical archive lag.
- A bucket-policy misconfiguration disables Object Lock without
  detection. Detected by: archiver verifies
  `GetObjectLockConfiguration` at boot and on every PutObject; fails
  loudly if missing.
- The `audit_log` table reappears in a future migration. Detected
  by: `server/go/internal/store/schema.go` does not include it; any
  reintroduction needs to delete this ADR (which makes the
  contradiction visible) or amend it.

## References

- This ADR is the home of the audit-log design (lifted from
  CLAUDE.md's pre-ADR-019 "Architecture Invariants" section).
- `server/go/internal/store/schema.go:133-135` — DDL omits
  `audit_log` table, citing this invariant.
- EPIC #510 — Close admin-op WAL carve-out (correctness half).
  Closed by commit `d8d0afd`.
- Issue #513 — Residual `RemoveGroupMember` shared_index cascade
  follow-up to #510.
- EPIC #511 — S3 Object Lock archive of WAL (tamper-evidence half).
- Files this commit removes content from:
  - `docs/adr/001-storage-architecture.md` — removed `audit_log` row
    from the "Tables per tenant file" SQL list (per-tenant audit
    table no longer exists).
  - `docs/adr/011-security-and-compliance.md` — removed the entire
    "Audit logging (tamper-evident)" subsection, including the
    `CREATE TABLE audit_log` schema with `prev_hash` column. The
    rejected design is preserved here under "Alternatives
    considered".
- Audit findings on `chore/consistency-audit` 2026-05-14 —
  locked-vs-locked contradiction #2 (CLAUDE.md #2 vs ADR-001 and
  ADR-011 audit-log content) resolved by this ADR.
