# ADR-016: Handlers append to the WAL; only the applier writes SQLite

**Status:** Accepted
**Decided:** 2026-05-14
**Tags:** event-sourcing, wal, write-path, durability
**Implementation:** _this commit_ (design); #510 (closes the residual admin-op carve-out)

## Decision

Every state change in EntDB follows one path:

```
gRPC handler  ──►  wal.Append(event)  ──►  Kafka/Redpanda topic
                                                      │
                                                      ▼
                                          Applier (single consumer goroutine)
                                                      │
                                                      ▼
                                          Per-tenant SQLite + globalstore
```

The rule, restated:

- **Handlers append.** A gRPC handler's only persistence action is
  `wal.Append(ctx, event)`. Handlers MUST NOT call
  `store.*` or `globalstore.*` write methods directly.
- **The applier writes.** `server/go/internal/apply/applier.go` is
  the only code path that writes per-tenant SQLite or globalstore
  rows. It runs as a single consumer goroutine per server (ADR-017,
  forthcoming).
- **SQLite is derived.** Per-tenant SQLite files and `global.db` are
  materialized views of the WAL. They can be deleted and rebuilt by
  replaying the WAL from the earliest retained offset.

This rule applies to **every** mutation — tenant data, admin
operations, GDPR flows, transfers, revocations. No carve-outs.

## Context

EntDB is event-sourced. The WAL is the durable record of "what
happened"; SQLite is the optimized way to answer "what is the
current state." Keeping the two cleanly separated unlocks four
properties the project depends on:

1. **Audit log (ADR-015).** WAL + S3 Object Lock is the audit log.
   If handlers wrote SQLite directly, those writes wouldn't be in
   the audit log — they'd vanish on a replay and the audit posture
   would be a fiction.
2. **Disaster recovery.** Lose every SQLite file? Replay the WAL.
   This is only possible if every state change is in the WAL.
3. **Replay debugging.** Reproducing a production bug locally is
   "replay the WAL into an empty SQLite." Direct writes break this.
4. **Future scaling.** Multiple servers can run multiple appliers
   against the same WAL (one per partition). This is only possible
   if the WAL is the only producer of SQLite state.

The event-sourcing architecture is described in ADR-005 at a
high level. ADR-016 is the executable rule that makes that
architecture actually work.

### Implementation status (and known gap)

- **Tenant-data RPCs** (`ExecuteAtomic`, `ShareNode`, `RevokeAccess`,
  `TransferOwnership`, `AddGroupMember`, `RemoveGroupMember`,
  `SetLegalHold` on nodes, GDPR ops on nodes): correctly append to
  the WAL. The applier writes SQLite. The rule holds.

- **Admin RPCs** (`CreateTenant`, `CreateUser`, `AddTenantMember`,
  `RemoveTenantMember`, `ChangeMemberRole`, `ArchiveTenant`,
  `UpdateUser`, `SetLegalHold` on tenants, `DeleteUser`,
  `CancelUserDeletion`, `FreezeUser`, `TransferUserContent`,
  `RevokeAllUserAccess`): **today these write directly to
  globalstore SQLite without a WAL event.** This is a deliberate
  carve-out inherited from the Python source, preserved during
  the Phase 4D port. It contradicts this ADR.

  EPIC #510 closes the carve-out: it adds new WAL op types
  (`TenantCreated`, `UserCreated`, `MemberAdded`, ...), routes
  every admin handler through `wal.Append`, and makes the applier
  the sole writer of `globalstore`. After #510 lands, this ADR
  matches reality with no edits needed.

  Until #510 ships, the carve-out is documented as a known
  exception. New admin RPCs MUST follow the rule (route through
  WAL); the only exceptions are the existing 13 enumerated above.

## Alternatives considered

- **Direct SQLite writes from handlers (the current admin-op
  carve-out, generalized).** Rejected. Breaks the audit log (ADR-015),
  breaks disaster recovery, breaks replay debugging. The current
  carve-out exists because the Python port prioritized parity over
  correctness; we're closing it, not endorsing it.

- **Two-phase commit between WAL append and SQLite write inside the
  handler.** Rejected. Doesn't buy anything: the applier already
  guarantees SQLite reflects the WAL eventually. 2PC adds latency
  and a distributed-coordination dependency for no gain. Caller
  ordering (read-your-write) is handled via the optional
  `wait_applied=true` flag on the RPC.

- **Per-RPC choice of write path (some handlers write directly,
  some go through WAL).** Rejected. Inconsistency invites mistakes
  during code review and makes the audit story impossible to
  enforce mechanically. One rule, no exceptions.

- **Direct SQLite writes for "control plane" operations only
  (formalize the current carve-out instead of closing it).**
  Rejected. The carve-out is exactly what makes the audit log a
  fiction — admin operations are precisely the operations a
  compliance auditor cares most about (who created tenants, who
  changed memberships, who deleted users). Carve-outs from "WAL
  is the source of truth" can't coexist with "WAL is the audit
  log" (ADR-015).

## Consequences

**What this locks in:**

- The applier is the only writer of per-tenant SQLite and
  globalstore. Code review rejects any direct-write call from
  handler code outside `server/go/internal/apply/`.
- Adding a new mutating RPC follows the same recipe every time:
  define the op type in `server/go/internal/wal/event.go`, add an
  `Apply*` method in the applier, write a handler that builds the
  event and calls `wal.Append`. The handler does not touch SQLite.
- WAL retention is the disaster-recovery floor. Configure Kafka
  retention longer than the worst-case recovery window plus snapshot
  cadence.
- The applier's idempotency-on-replay tests are load-bearing. Every
  `Apply*` method must accept the same event twice and produce the
  same state. Today this is enforced by the per-package test
  suites; CI keeps that surface green.

**What this makes easy:**

- Disaster recovery is "spin up a new server, point at the WAL, wait
  for the applier to catch up." No SQLite backups needed.
- Audit queries are WAL queries. "What did tenant X do between T1
  and T2?" is a Kafka offset range scan or an S3 archive prefix
  scan (per ADR-015). Same mechanism for compliance, debugging,
  and recovery.
- Reasoning about consistency is local: per-tenant ordering is the
  WAL partition's offset order. There's no parallel write path to
  reconcile against.

**What this makes harder:**

- Synchronous write semantics. A handler can't return "I wrote it"
  until either the WAL acks (fast, ~10ms) or the applier finishes
  (slower, depends on apply latency). Default behavior is "WAL ack
  is enough"; callers needing strict read-your-write pass
  `wait_applied=true`.
- Test-only in-memory WAL backend (`server/go/internal/wal/memory.go`)
  is mandatory for fast unit tests; without it the test suite would
  need Kafka.
- Bootstrap sequencing: applier MUST be running before any handler
  appends. This is fine in practice (server boot starts the applier
  before the gRPC listener) but is a constraint that operators have
  to honor.

**Failure modes:**

- A new handler writes SQLite directly. Detected by code review
  + an audit:
  ```sh
  git grep -nE 'store\.(Begin|Insert|Update|Delete)|globalstore\.(Create|Update|Delete|Add|Remove|Change)' server/go/internal/api/
  ```
  Should only return matches inside `server/go/internal/apply/`
  (when EPIC #510 closes), or inside the 13 enumerated admin
  handlers (until #510 closes).

- Applier writes from outside its consumer goroutine (breaks
  ordering). Detected by code review; the applier's `Apply`
  methods are not exported beyond the package.

- WAL drops events (Kafka misconfigured `acks` or retention too
  short). Detected by e2e crash-recovery tests
  (`tests/python/e2e/`) — these replay from offset 0 against a
  fresh data dir and assert state matches.

- The carve-out widens (new admin RPC added with direct-write
  pattern). Detected by the audit grep above. Mitigated by
  closing #510 so the pattern goes away entirely.

## References

- [ADR-015](015-wal-and-s3-object-lock-as-audit-log.md) — WAL +
  S3 Object Lock as the audit log. ADR-016 makes ADR-015 work:
  without "all writes through WAL," the audit log is incomplete.
- [ADR-005](005-event-sourcing-wal.md) — Event sourcing
  architecture overview (predates ADR-014's "one home" policy;
  some content in ADR-005 is dated and will be migrated when we
  touch it).
- EPIC [#510](https://github.com/elloloop/tenant-shard-db/issues/510)
  — Close admin-op WAL carve-out. After it lands, the
  "Implementation status: known gap" section above can be deleted.
- Source: `server/go/internal/wal/` (producer + consumer),
  `server/go/internal/apply/applier.go` (the only SQLite writer),
  `server/go/internal/wal/event.go` (op type definitions).
- Files this commit removes content from:
  - `CLAUDE.md` — invariant #1 body replaced with a one-line
    pointer to this ADR (per ADR-014).
