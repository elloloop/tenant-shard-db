# EPIC #407 — Coordination Plan (Wave 0 → 2)

This document synthesises the 54 Wave-0 spec files in `docs/go-port/{rpcs,shared}/`
into a single execution plan for the Python → Go server port. It is the input
to the Wave-1 and Wave-2 PR slates.

Layout assumed: `server/go/internal/<pkg>/` (Go server tree introduced by
PR #170 alongside `server/python/entdb_server/`). The Go binary entrypoint
will live at `server/go/cmd/entdb-server/`.

## 1. Wave structure

- **Wave 0 — Specs (DONE).** 44 RPC specs + 10 shared concern specs landed.
  Every privileged RPC has an `Auth` / `Side effects` / `Error contract` /
  `Shared deps` / `Other-RPC deps` / `Contract tests` / `Implementation
  outline` / `Open questions` section, cross-linked to Python source by
  file:line.
- **Phase 0 — `#408` (in flight).** Adds the test harness scaffolding from
  [test-harness.md](shared/test-harness.md) (`tests/python/integration/conftest.py`,
  `_go_parity.py`, CI matrix `server: [python, go]` with
  `continue-on-error: true` on the Go leg). Ships `server/go/cmd/entdb-server`
  stub that satisfies `--listen / --wal-backend memory / --auth-disabled /
  --seed-tenant`. No real handlers yet. **This unblocks Wave 1.**
- **Wave 1 — Shared packages.** ~10 PRs landing the Go ports of the shared
  concerns. Many run truly in parallel via worktrees; the rest form a thin
  dependency chain (see §3). Each Wave-1 PR ships interfaces + an in-memory /
  pure-Go default backend; production backends (Kafka, Kinesis, S3 archive,
  OAuth JWKS) defer to Phase 2.
- **Wave 2 — 44 RPC handlers.** One PR per RPC, each landing
  `server/go/internal/api/<rpc>.go` plus a registry-bump in
  `_go_parity.py` (the harness gate from
  [test-harness.md §Per-RPC gating](shared/test-harness.md#per-rpc-gating)).
  Batched in §4 by Wave-1 dependency footprint. Up to ~10 agents in
  worktrees can work simultaneously after Batch A lands.
- **Wave 3 — Flip the gate.** Drop `continue-on-error` on the Go matrix leg;
  add it to `all-checks.needs`. `GO_IMPLEMENTED` covers all 44 RPCs.
- **Phase 2+ (out of scope here).** Real WAL backends (Kafka first), real
  OAuth/JWKS, S3 archive replay, snapshot tier, encryption-at-rest.

## 2. Common Go packages (Wave 1 work)

Canonical list of `server/go/internal/<pkg>/` packages every RPC depends on.
Names are **normative** — Wave-0 specs sometimes used `apply` / `applier`,
`store` / `canonical` / `canonicalstore`, `global` / `globalstore`
interchangeably. The list below is the source of truth; rename freely
inside specs but keep import paths consistent.

### 2.1 `pb` — generated proto types
**Purpose:** `protoc-gen-go` + `protoc-gen-go-grpc` output for
`proto/entdb/v1/entdb.proto` and `proto/console/v1/console.proto`.
Mirrors `server/python/entdb_server/api/generated/`.
**Spec:** consumed by every spec; format frozen per CLAUDE.md invariant #5.
**Wave 1 scope:** `go generate` wiring + checked-in generated files; no
hand-written code. Add a CI guard that regen produces a clean diff.
**Deps:** none.

### 2.2 `errs` — typed errors + gRPC status mapping
**Purpose:** sentinels with `GRPCStatus()`, trailer helpers
(`SetRedirectTrailer`, `SetRetryAfter`), `PreserveStatusOrSwallow` choke
point for the Python `except Exception` swallow patterns.
**Spec:** [error-mapping.md](shared/error-mapping.md).
**Wave 1 scope:** all sentinel constants, `Errorf(codes.X, fmt, ...)`,
trailer helpers, swallow helper. **No** `google.rpc.ErrorInfo` extensions
(deliberate parity with Python).
**Deps:** `pb` (only because callers will compose errors from proto types
later — `errs` itself depends only on `grpc/codes` and `grpc/status`).

### 2.3 `schema` — registry, types, fingerprint, compat
**Purpose:** process-wide singleton built at boot, frozen before serving;
holds `NodeTypeDef`, `EdgeTypeDef`, `FieldDef`, `CompositeUniqueDef`;
serves `UniqueFieldIDs`, `IndexedFieldIDs`, `SearchableFieldIDs`,
`PIIFields`, `DataPolicy`, `SubjectField`. Plus `compat.Check(old, new)`
for the deploy-time ratchet, plus `Fingerprint()` for SDK drift detection.
**Spec:** [schema-registry.md](shared/schema-registry.md).
**Wave 1 scope:** `Registry`, `NodeTypeDef`, `EdgeTypeDef`, `FieldDef`,
`CompositeUniqueDef`, `LoadFromJSON` (consumes Python-emitted
`SchemaRegistry.to_json` shape — cross-language contract), `Freeze`,
`Fingerprint`. Compat package can be a stub (Wave 2 doesn't gate on it);
`LoadFromDescriptors` (proto reflection) deferred to Phase 2.
**Deps:** `pb`, `errs`.

### 2.4 `payload` — name↔id field key translation
**Purpose:** the *only* place name↔field_id translation runs
(CLAUDE.md invariant #6). `StructToPayload` (ingress, schema-aware,
drops unknowns), `PayloadToStruct` (egress, schema-less wrap),
`FilterNamesToIDs` (QueryNodes filters), `PayloadJSONToNames` (export
helper).
**Spec:** [payload-translation.md](shared/payload-translation.md).
**Wave 1 scope:** all four functions; `map[uint32]any` internal key
shape; bytes/timestamp/JSON kind coercion via `schema.Registry`.
**Deps:** `schema`, `pb`, `errs`.

### 2.5 `auth` — interceptor + trusted-actor
**Purpose:** unary + stream gRPC interceptors that authenticate via
OAuth bearer / API key / session, attach `Identity` to `context.Context`,
and supply `Authoritative(ctx, claimed) Actor` for handlers. Health
allow-list. `Actor` type with `user:`/`system:`/`admin:`/`group:` prefix
predicates.
**Spec:** [auth-interceptor.md](shared/auth-interceptor.md).
**Wave 1 scope:** `Interceptor.Unary()`, `Interceptor.Stream()`,
`IdentityFromContext`, `Authoritative`, `Actor` type, `OAuthValidator`
(in-memory JWKS for tests), `APIKeyManager` (in-memory map), 
`SessionManager` (in-memory map). Production OAuth/JWKS rotation deferred.
**Deps:** `errs`, `pb`. **Cross-cutting:** every handler imports it.

### 2.6 `wal` — producer interface + in-memory backend
**Purpose:** `Producer.Append(ctx, topic, key, value, headers) -> StreamPos`
contract: durable per-tenant total order, idempotent retries.
`Event{TenantID, Actor, IdempotencyKey, SchemaFingerprint, TsMs, Ops}`
shape. `StreamPos` `topic:partition:offset`. `Subscribe`/`PollBatch`/
`Commit` for the consumer side. In-memory backend with `WaitForRecords`,
`GetAllRecords` for tests.
**Spec:** [wal-producer.md](shared/wal-producer.md).
**Wave 1 scope:** types + interfaces + memory backend only. Kafka /
Kinesis / SQS / Pub/Sub / Service Bus / Event Hubs deferred.
**Deps:** `errs`, `pb`.

### 2.7 `store` — per-tenant SQLite (`CanonicalStore`)
**Purpose:** per-tenant `tenant_<id>.db` (+ `public.db` + per-user
mailbox files). Owns DDL, lazy unique/query/composite/FTS5 expression
indexes, `BatchTxn` (`BEGIN IMMEDIATE`), `WaitForOffset` /
`UpdateAppliedOffset` notification, all `_sync_*` read paths, all
applier-callable write helpers.
**Spec:** [canonical-store.md](shared/canonical-store.md).
**Wave 1 scope:** schema DDL, pool, per-tenant lock, `BatchTxn`,
`GetNode`/`GetNodes`/`QueryNodes`/`SearchNodes`/`GetEdgesFrom`/
`GetEdgesTo`/`GetVisibleNodes`/`ListSharedWithMe`/`CheckIdempotency`/
`WaitForOffset`/`UpdateAppliedOffset`. Write helpers callable from the
applier: `CreateNodeRaw`, `UpdateNode`, `DeleteNode`, `CreateEdge`,
`DeleteEdge`, `ShareNode`, `RevokeAccess`, `DelegateAccess`,
`TransferOwnership`, `RevokeUserAccess`, `AddGroupMember`,
`RemoveGroupMember`. Lazy index/FTS bootstrap. Driver: 
`modernc.org/sqlite` (no cgo) per spec recommendation; revisit if
SQLCipher returns to scope. Notifications/mailbox sub-package can stub
(deprecated stubs only need `_check_tenant`).
**Deps:** `schema`, `payload`, `errs`, `pb`.

### 2.8 `globalstore` — cross-tenant SQLite
**Purpose:** `global.db` — `user_registry`, `tenant_registry`,
`tenant_members`, `shared_index`, `deletion_queue`, `legal_holds`,
`tenant_quotas`, `tenant_usage`. The only place cross-tenant state may
be read or written. **Has no WAL** (documented carve-out).
**Spec:** [global-store.md](shared/global-store.md).
**Wave 1 scope:** schema DDL + migrations, all CRUD listed in the spec
(`CreateUser`, `UpdateUser`, `SetUserStatus`, `CreateTenant`,
`SetTenantStatus`, `SetLegalHold`, `AddMember`/`RemoveMember`/
`ChangeRole`, `AddShared`/`RemoveShared`/`CleanupStaleShared`/
`RemoveAllSharedForUser`, `QueueDeletion`/`CancelDeletion`/
`MarkDeletionCompleted`, `RevokeUserAccess`, `SetQuotaConfig`,
`IncrementUsage`, `ResetPeriod`). `MaxOpenConns(1)` to mirror Python's
single-thread executor.
**Deps:** `errs`, `pb`. Optional: `crypto` (deferred — encryption-at-rest
is Phase 2).

### 2.9 `acl` — typed-capability authorization engine
**Purpose:** `Permission` (legacy enum), `CoreCapability` (typed),
`ExtCapID`, `Actor`, `Grant`, `Resolver` (group expansion),
`Checker.Check`, `Filter.FilterReadable`, `Registry.RequiredForOp` /
`CheckGrant` / `LegacyToCoreCaps`, `Enforcer` that ties it together.
Mirrors `apply/acl.py` + `auth/capability_registry.py`.
**Spec:** [acl.md](shared/acl.md).
**Wave 1 scope:** all types + `Enforcer.Check` + `Enforcer.FilterReadable`.
The capability registry must be populated from proto descriptors via the
`schema` package; for Wave 1 a JSON-fed registry (mirror of
`SchemaRegistry.to_json`) is sufficient. Group resolution depth-bounded
(matches Python `_ACL_MAX_DEPTH=10`).
**Deps:** `schema`, `store` (read-only — `node_access`, `node_visibility`,
`acl_inherit`, `group_users`), `globalstore` (cross-tenant grant lookup
via `shared_index`), `pb`, `errs`.

### 2.10 `apply` — applier (WAL consumer + materializer)
**Purpose:** the WAL consumer. `Run(ctx)` loop subscribes to a
`(topic, group_id)` pair, dispatches by op-type into `BatchTxn` blocks
on `store`, writes `applied_events`, advances WAL offsets, fans out
notifications, maintains `shared_index`. `Replay(ctx, tenantID,
fromPos)` for recovery. Halt-on-poison semantics; per-event
idempotency check inside the txn.
**Spec:** [applier.md](shared/applier.md).
**Wave 1 scope:** `Run(ctx)`, `Replay(ctx, ...)`, every op type the
Python applier dispatches today: `create_node`, `update_node`,
`delete_node`, `create_edge`, `delete_edge`, `share_node` (NEW —
WAL-first fix, see §6), `revoke_access` (NEW), `delegate_access` (NEW —
fixes the silent-drop bug), `admin_transfer_content`,
`admin_revoke_access` (BROADENED — also delete `node_access` +
`group_users`, see §6), `transfer_ownership` (NEW), `add_group_member` /
`remove_group_member` (NEW for WAL-first), `set_legal_hold` (NEW),
`add_tenant_member` / `remove_tenant_member` / `change_member_role`
(NEW — closes global_store WAL gap, see §6 open question). Mailbox
fanout + shared-index hooks **best-effort** post-commit. Quota counter
fire-and-forget.
**Deps:** `wal`, `store`, `globalstore`, `schema`, `acl`, `payload`,
`errs`, `pb`.

### 2.11 `tenant` — tenant gate (sharding + region)
**Purpose:** `CheckTenant(ctx, tenantID)` — sharding ownership
(`UNAVAILABLE` + `entdb-redirect-node` trailer) and region pinning
(`FAILED_PRECONDITION`). Used by every tenant-scoped RPC.
**Spec:** referenced in 30+ RPC specs but no dedicated shared spec
exists; the contract is repeated in
[error-mapping.md](shared/error-mapping.md) and individual RPC specs
(see [Health.md](rpcs/Health.md), [GetMailbox.md](rpcs/GetMailbox.md)).
**Wave 1 scope:** `Sharding{NodeID, AssignedTenants, IsMine, Owner}`
struct, `CheckTenant` helper, region lookup against `globalstore`.
Single-node default config (always-mine).
**Deps:** `globalstore`, `errs`, `pb`.

### 2.12 `metrics` — Prometheus
**Purpose:** `RecordGRPCRequest(method, status, duration)` — single
chokepoint. Mirrors `metrics.py:103`.
**Wave 1 scope:** counter + histogram registration; in-process registry.
**Deps:** `prometheus/client_golang`. No spec needed.

### 2.13 `version` — build-stamped semver
**Purpose:** `var Version = "dev"` set via `-ldflags -X`. Returned by
`Health`. **Spec:** [Health.md](rpcs/Health.md).
**Wave 1 scope:** trivial; one file.
**Deps:** none.

### Packages NOT in Wave 1 (deferred to Phase 2+)

`crypto` (encryption-at-rest, KMS); `audit/compliance` (S3 Object Lock
export — WAL is the audit log per CLAUDE.md #2); `quotas` (rate-limit
interceptor — Wave-2 fixtures use `--auth-disabled`); `archive`/`snapshot`
(Tier-2/3 recovery); per-WAL-backend packages (`wal/kafka`, etc. — memory
only in Wave 1); `console` (separate proto, not in 44-RPC list); real
mailbox/`fts` backend (deprecated stubs only); `gdpr` worker (background
`deletion_queue` processor — `DeleteUser`/`CancelUserDeletion` only queue).

## 3. Wave 1 dependency graph

```
                                pb         (no deps)
                                │
                  ┌─────────────┼─────────────┐
                  ▼             ▼             ▼
                errs        version      metrics
                  │
         ┌────────┼────────┐
         ▼        ▼        ▼
       schema   wal      auth          (all depend on pb + errs;
         │      │         │             no inter-deps among these three)
         ▼      │         │
       payload │         │
         │     │         │
         └─┬───┘         │
           ▼             │
       globalstore       │            (depends on errs + pb only;
           │             │             schema/payload not needed)
           ▼             │
        tenant ──────────┘            (globalstore + errs)
           │
           ▼
         store          ←── schema, payload, errs, pb
           │
           ▼
          acl           ←── schema, store, globalstore, pb, errs
           │
           ▼
         apply          ←── wal, store, globalstore, schema, acl,
                            payload, errs, pb
```

**Zero-dep PRs (can start immediately when #408 merges):** `pb`, `errs`,
`version`, `metrics`. Run all four in parallel.

**Layer 2 (after `pb`+`errs`):** `schema`, `wal`, `auth`. Three parallel
agents.

**Layer 3:** `payload` (waits on `schema`), `globalstore` (waits on
nothing structural beyond `errs+pb`, can start with Layer 2 if a
worktree has them). In practice run after Layer 2.

**Layer 4:** `tenant` (waits on `globalstore`). Trivial; can fold into
the `globalstore` PR.

**Layer 5:** `store` (waits on `schema`+`payload`).

**Layer 6:** `acl` (waits on `store`+`globalstore`+`schema`).

**Layer 7:** `apply` (waits on everything except `auth`+`tenant`+
`metrics`+`version`).

Critical-path length: **7 PRs**, but the bottom of the chain (`store` →
`acl` → `apply`) is sequential. Effective wall-clock for one engineer ≈
3 weeks; with 4 agents in worktrees ≈ 7–10 days because Layers 1–3 run
in parallel.

## 4. Wave 2 RPC list, sorted

44 RPCs grouped by Wave-1 package footprint. Each batch can run agents
in parallel up to the listed cap (file-conflict ceiling — see §5).

### Batch A — Foundational, no auth, no WAL (parallel: ~5 agents)

**Required Wave-1 PRs:** `pb`, `errs`, `metrics`, `version`, `tenant`,
`globalstore`.
**Not required:** `auth` (most don't authenticate),
`wal`, `apply`, `store`, `acl`, `schema`, `payload`.

| RPC | Notes |
|---|---|
| `Health` | leaf; no auth (allow-list); `version`+`metrics` only. [Health.md](rpcs/Health.md) |
| `GetSchema` | reads `schema`. Adds `schema` dep — tight coupling, batch with §A but blocks on schema PR. [GetSchema.md](rpcs/GetSchema.md) |
| `GetReceiptStatus` | `tenant` + `store.CheckIdempotency`. Adds `store` dep — see Batch B. [GetReceiptStatus.md](rpcs/GetReceiptStatus.md) |
| `WaitForOffset` | `tenant` + `applystate` (lives inside `store`). Adds `store` dep. [WaitForOffset.md](rpcs/WaitForOffset.md) |

In practice only `Health` is truly footprint-A; the other three need
`store`. Re-bin: `Health` ships with Wave 1.0. The other three ship at
the start of Batch B once `store` is up.

### Batch B — Read path, single-tenant (parallel: ~10 agents)

**Required:** Batch A + `auth`, `store`, `acl`, `payload`, `schema`.

| RPC | Notes |
|---|---|
| `GetNode` | trusted actor, cross-tenant ACL fallback. [GetNode.md](rpcs/GetNode.md) |
| `GetNodes` | batch sibling of `GetNode`. [GetNodes.md](rpcs/GetNodes.md) |
| `GetNodeByKey` | unique-index lookup → delegates to `GetNode`. [GetNodeByKey.md](rpcs/GetNodeByKey.md) |
| `QueryNodes` | Mongo-style filter → SQL via `apply/query_filter.py` port. [QueryNodes.md](rpcs/QueryNodes.md) |
| `SearchNodes` | FTS5 + ACL post-filter. [SearchNodes.md](rpcs/SearchNodes.md) |
| `GetEdgesFrom` | per-tenant edge read. [GetEdgesFrom.md](rpcs/GetEdgesFrom.md) |
| `GetEdgesTo` | sibling. [GetEdgesTo.md](rpcs/GetEdgesTo.md) |
| `GetConnectedNodes` | edge-traversal + ACL. [GetConnectedNodes.md](rpcs/GetConnectedNodes.md) |
| `ListSharedWithMe` | reads `globalstore.shared_index` + per-tenant. [ListSharedWithMe.md](rpcs/ListSharedWithMe.md) |
| `GetReceiptStatus` | (relisted from A). |
| `WaitForOffset` | (relisted from A). |

Wire-format suite (`test_grpc_wire_format.py`) requires `GetNode` +
`ExecuteAtomic` to be `GO_IMPLEMENTED`. Land `GetNode` first in this
batch.

### Batch C — Write path (parallel: 1 — `ExecuteAtomic` is the lynchpin, then 0 more in this batch)

**Required:** Batch B + `wal`, `apply`.

| RPC | Notes |
|---|---|
| `ExecuteAtomic` | the single largest handler; once green, every other write path is structurally derivative. [ExecuteAtomic.md](rpcs/ExecuteAtomic.md) |

### Batch D — Cross-tenant / global_store (parallel: ~6 agents)

**Required:** Batch A + `auth`. **No** `wal` / `apply` / `store` / `acl`
needed for the user/tenant CRUD subset (writes go directly to
`globalstore` per CLAUDE.md carve-out — see §6 open question).

| RPC | Notes |
|---|---|
| `CreateUser` | direct globalstore write (NOT WAL). [CreateUser.md](rpcs/CreateUser.md) |
| `UpdateUser` | same. [UpdateUser.md](rpcs/UpdateUser.md) |
| `GetUser` | global read. [GetUser.md](rpcs/GetUser.md) |
| `ListUsers` | global read. [ListUsers.md](rpcs/ListUsers.md) |
| `GetUserTenants` | global read. [GetUserTenants.md](rpcs/GetUserTenants.md) |
| `CreateTenant` | global write + creator membership. [CreateTenant.md](rpcs/CreateTenant.md) |
| `GetTenant` | global read. [GetTenant.md](rpcs/GetTenant.md) |
| `ListTenants` | membership-filtered; refuses fall-back to wire actor. [ListTenants.md](rpcs/ListTenants.md) |
| `ArchiveTenant` | global status flip. [ArchiveTenant.md](rpcs/ArchiveTenant.md) |
| `GetTenantQuota` | global read. [GetTenantQuota.md](rpcs/GetTenantQuota.md) |
| `GetTenantMembers` | global read. [GetTenantMembers.md](rpcs/GetTenantMembers.md) |
| `AddTenantMember` | global write. [AddTenantMember.md](rpcs/AddTenantMember.md) |
| `RemoveTenantMember` | global write + cascade considerations. [RemoveTenantMember.md](rpcs/RemoveTenantMember.md) |
| `ChangeMemberRole` | global write. [ChangeMemberRole.md](rpcs/ChangeMemberRole.md) |

### Batch E — Sharing / ACL (parallel: ~5 agents)

**Required:** Batch C (needs `wal`+`apply`) + `acl`.

| RPC | Notes |
|---|---|
| `ShareNode` | WAL-first fix (§6). [ShareNode.md](rpcs/ShareNode.md) |
| `RevokeAccess` | WAL-first fix (§6). [RevokeAccess.md](rpcs/RevokeAccess.md) |
| `DelegateAccess` | WAL-first fix + applier op (§6). [DelegateAccess.md](rpcs/DelegateAccess.md) |
| `TransferOwnership` | WAL-first fix (§6). [TransferOwnership.md](rpcs/TransferOwnership.md) |
| `AddGroupMember` | WAL-first fix (§6). [AddGroupMember.md](rpcs/AddGroupMember.md) |
| `RemoveGroupMember` | WAL-first fix (§6). [RemoveGroupMember.md](rpcs/RemoveGroupMember.md) |

### Batch F — GDPR / compliance (parallel: ~4 agents)

**Required:** Batch C + `globalstore`.

| RPC | Notes |
|---|---|
| `DeleteUser` | queues only; worker out-of-scope. [DeleteUser.md](rpcs/DeleteUser.md) |
| `CancelUserDeletion` | inverse. [CancelUserDeletion.md](rpcs/CancelUserDeletion.md) |
| `ExportUserData` | read-only across tenants. [ExportUserData.md](rpcs/ExportUserData.md) |
| `FreezeUser` | flag in globalstore + applier hook. [FreezeUser.md](rpcs/FreezeUser.md) |
| `RevokeAllUserAccess` | bulk revoke; WAL-first (§6). [RevokeAllUserAccess.md](rpcs/RevokeAllUserAccess.md) |
| `TransferUserContent` | bulk transfer; WAL-first. [TransferUserContent.md](rpcs/TransferUserContent.md) |
| `SetLegalHold` | WAL-first (§6). [SetLegalHold.md](rpcs/SetLegalHold.md) |

### Batch G — Deprecated stubs (parallel: 3 agents, lowest risk)

**Required:** Batch A only.

| RPC | Notes |
|---|---|
| `SearchMailbox` | returns empty stub; `_check_tenant` only. [SearchMailbox.md](rpcs/SearchMailbox.md) |
| `GetMailbox` | same. [GetMailbox.md](rpcs/GetMailbox.md) |
| `ListMailboxUsers` | same. [ListMailboxUsers.md](rpcs/ListMailboxUsers.md) |

These three are byte-for-byte easiest. Recommend assigning to the same
agent for one tight PR.

**RPC count check:** 1 (A) + 11 (B) + 1 (C) + 14 (D) + 6 (E) + 7 (F) +
3 (G) = 43. Off by one because `GetReceiptStatus`/`WaitForOffset` are
listed in both A and B; the actual unique RPC count is **44**. ✓

## 5. Conflict surface (file-level)

The naive shape — one giant `server/go/internal/api/server.go` with 44
methods — would force every Wave-2 PR to touch the same file.

**Recommended layout: one file per RPC** under `server/go/internal/api/`.
File name is `snake_case(<rpc>).go` (e.g. `health.go`, `execute_atomic.go`,
`add_tenant_member.go`). Plus three shared files: `server.go` (`Server`
struct + `Register`), `helpers.go` (`checkTenant`, `requireAdminOrOwner`,
`getAuthoritativeActor`), `server_doc.go` (package doc).

**Conflict-prone files** even with this layout:

1. **`server.go`** — holds `type Server struct { wal *wal.Producer;
   store *store.CanonicalStore; ... }` and `func (s *Server) Register(grpc.ServiceRegistrar)`.
   *Mitigation:* freeze the struct shape in Wave-1's `apply` PR (it
   needs Server too). Wave-2 PRs only add fields when introducing a
   genuinely new dep (rare). Use a struct-zero default + functional
   options.
2. **`pb` registration** — `pb.RegisterEntDBServiceServer(s, srv)` needs
   the receiver type `Server` to satisfy the interface. The Go gRPC
   generator emits an `UnimplementedEntDBServiceServer` embed; use it
   so partial implementations compile. *Each Wave-2 PR removes one
   `Unimplemented*` method by providing its own impl — no shared edit.*
3. **`tests/python/integration/_go_parity.py`** — every Wave-2 PR adds
   a single line to `GO_IMPLEMENTED`. Conflicts are trivial (sort
   alphabetically, conflict-resolve by sorted-merge).
4. **`helpers.go`** — `checkTenant` / `requireAdminOrOwner` /
   `getAuthoritativeActor` shims. *Mitigation:* land all of them in the
   first Batch-B PR (`GetNode`); subsequent PRs reuse, never edit.
5. **`server/go/cmd/entdb-server/main.go`** — wiring. *Mitigation:* Wave 1
   freezes the wiring; Wave 2 PRs do not touch main.

**Out-of-scope conflict surface:** none of the Wave-2 PRs should edit
files under `server/python/` (CLAUDE.md: do not modify Python source
unless explicitly needed for parity migration).

## 6. Architectural drift surfaced by Wave 0

Wave-0 agents independently flagged that the current Python implementation
violates several CLAUDE.md invariants. Aggregated below.

### 6.1 WAL bypass (CLAUDE.md invariant #1 violations)

The following RPCs write directly to per-tenant SQLite (or `globalstore`)
**without** appending a `TransactionEvent` to the WAL. On a full WAL
rebuild, every one of these mutations is silently lost.

| RPC | Bypassed write | Spec section |
|---|---|---|
| `ShareNode` | `node_access` | [ShareNode.md §Side effects](rpcs/ShareNode.md) "INVARIANT VIOLATION IN PYTHON — FIX in Go port" |
| `RevokeAccess` | `node_access` DELETE | [RevokeAccess.md §Side effects](rpcs/RevokeAccess.md) |
| `DelegateAccess` | applier dispatch branch is **missing** entirely; legacy direct write only | [DelegateAccess.md §Open questions](rpcs/DelegateAccess.md) |
| `TransferOwnership` | `nodes.owner_actor` | [TransferOwnership.md §WAL append](rpcs/TransferOwnership.md) |
| `AddGroupMember` | `group_users` | [AddGroupMember.md](rpcs/AddGroupMember.md) |
| `RemoveGroupMember` | `group_users` + shared-index cascade | [RemoveGroupMember.md](rpcs/RemoveGroupMember.md) |
| `RevokeAllUserAccess` | `node_access` + `group_users` + `node_visibility` | [RevokeAllUserAccess.md §Side effects](rpcs/RevokeAllUserAccess.md) |
| `SetLegalHold` | `tenant_registry.status` (globalstore) | [SetLegalHold.md §WAL gap](rpcs/SetLegalHold.md) |
| `CreateTenant` | globalstore — but globalstore has its own carve-out from invariant #1 | [CreateTenant.md](rpcs/CreateTenant.md) |
| `ArchiveTenant` | same | [ArchiveTenant.md](rpcs/ArchiveTenant.md) |
| `AddTenantMember` / `RemoveTenantMember` / `ChangeMemberRole` | globalstore | (same carve-out applies) |

**Default policy: fix-on-port for per-tenant data; preserve carve-out
for global-store-only writes.**

- For per-tenant data (`node_access`, `group_users`, `node_visibility`,
  `nodes.owner_actor`): the Go handler MUST append a WAL event and the
  Go applier MUST grow a dispatch branch. New op types: `share_node`,
  `revoke_access`, `delegate_access`, `transfer_ownership`,
  `add_group_member`, `remove_group_member`, `set_legal_hold`. The
  `admin_revoke_access` op already exists in Python but only updates
  `node_visibility`; broaden it to also delete `node_access` and
  `group_users` per [RevokeAllUserAccess.md](rpcs/RevokeAllUserAccess.md).
- For global-store-only writes (user CRUD, tenant CRUD, membership):
  preserve the existing carve-out. Per [global-store.md §WAL coupling](shared/global-store.md),
  `GlobalStore` is its own durable substrate; CLAUDE.md invariant #1
  reads "every per-tenant mutation goes through the WAL". Document
  this explicitly in the Go port and ensure `GlobalStore` is included
  in S3 snapshots (open question in the spec; recommendation: yes).

**Exceptions:** `SetLegalHold` is a borderline case (the flag lives in
globalstore but is consulted on per-tenant writes). Recommendation:
emit an `admin_set_legal_hold` event AND keep the global-store status
column as the source-of-truth for tenant gating; the WAL event exists
purely for replay-survival of the audit history.

### 6.2 Trusted-actor enforcement gaps

PR #168 (commit `fece3fb`) closed most gaps. Wave-0 audits flagged the
following residual handlers that don't run wire-actor through
`get_authoritative_actor`:

- `TransferOwnership` (no auth check at all today beyond `_check_tenant`).
- `GetReceiptStatus` reads `request.context.actor` but **never uses it**
  for authorization — that's intentional and the spec pins it; no fix
  needed.
- `WaitForOffset` — same as above.
- `ListMailboxUsers` — has no `actor` field; vacuously safe.
- `GetMailbox` — does not call `_trusted_actor` (stub); no auth check
  needed.
- The `ExecuteAtomic` per-op `Permission` check is **deliberately
  missing** today. Preserve in Go for parity (flagged in
  [ExecuteAtomic.md §Open questions](rpcs/ExecuteAtomic.md)) — file as
  a follow-up.

**Default policy: fix-on-port (for write paths).** Every Wave-2 write
RPC MUST consult `auth.Authoritative(ctx, req.Context.Actor)` and
**rebind** the local variable before any authz decision. The Python
defence-in-depth pattern (multiple call sites re-invoking
`get_authoritative_actor`) is preserved.

### 6.3 Deprecated stubs

[GetMailbox.md](rpcs/GetMailbox.md), [SearchMailbox.md](rpcs/SearchMailbox.md),
[ListMailboxUsers.md](rpcs/ListMailboxUsers.md): the legacy per-user mailbox
SQLite store is gone. All three return an empty-but-well-formed response
plus the standard tenant gate. **Default policy: port-as-stub.** Do not
re-introduce a real implementation in Wave 2; file a follow-up if product
wants the listing.

### 6.4 Real bugs found while spec-writing

1. **`DelegateAccess` applier dispatch is missing** —
   [DelegateAccess.md §Open questions](rpcs/DelegateAccess.md). Python
   handler appends an `admin_delegate_access` event that is silently
   dropped by the applier; tests pass via the legacy direct-write path.
   **Fix in Go port** (Batch E ticket); also file a Python bug for
   parallel fix.
2. **`admin_revoke_access` applier op doesn't touch `node_access` /
   `group_users`** — [RevokeAllUserAccess.md §Side effects](rpcs/RevokeAllUserAccess.md).
   Today only deletes from `node_visibility`. Broaden in Go.
3. **`TransferUserContent` applier doesn't refresh `node_visibility`** —
   [TransferUserContent.md §Open questions point 4](rpcs/TransferUserContent.md).
   Visibility drift after WAL rebuild. Fix in Go (single-line addition).
4. **`ArchiveTenant` silently overwrites `legal_hold` status** —
   [SetLegalHold.md §Open questions point 4](rpcs/SetLegalHold.md). Add
   explicit guard in Go.
5. **`DeleteUser` doesn't check legal hold on user's tenants** —
   [SetLegalHold.md §Open questions point 3](rpcs/SetLegalHold.md). Fix
   behind a config flag in Go (preserves day-zero parity tests).

**Default policy:** fix-on-port for genuine bugs (1–4) — they have no
contract test pinning the broken behaviour. Item (5) is a behaviour
change that needs a config flag to keep parity tests green.

## 7. Test-harness gating

Per [test-harness.md](shared/test-harness.md):

- **Single env-var toggle:** `ENTDB_SERVER_TARGET=python|go`. Python is
  the default (in-process fixture, today's path). Go runs the binary as
  a subprocess.
- **Subprocess contract:** `$ENTDB_GO_BINARY` honours
  `--listen 127.0.0.1:<port>`, `--wal-backend memory`, `--data-dir <tmp>`,
  `--auth-disabled`, `--mailbox-fanout=false`, `--batch-size 1`,
  `--seed-tenant acme`, `--ready-probe`. The harness waits for
  `Health.healthy=true` (10 s, 50 ms poll) before yielding the port.
- **Per-RPC gate:** `tests/python/integration/_go_parity.py` exposes
  `GO_IMPLEMENTED: frozenset[str]`. A `pytest_collection_modifyitems`
  hook adds `pytest.mark.skip` to every contract case whose `rpc` is
  not in the set. Adding an RPC = one-line PR diff.
- **CI matrix:** `.github/workflows/ci.yml` grows
  `integration-tests: matrix: server: [python, go]` with
  `continue-on-error: true` on the Go leg until §1's Wave 3 flip.
- **Suites NOT parameterised by target:** `test_wal_replay_determinism.py`,
  `test_concurrent_applier_reads.py`, `test_privilege_escalation.py`. They
  run Python-only; equivalent Go-native suites live under
  `tests/go/` (separate work).

### Wave-2 PR template — definition of done

Every Wave-2 RPC PR MUST:

1. Add `server/go/internal/api/<rpc>.go` with the handler.
2. Wire it into `server.go` (replace the `Unimplemented` embed).
3. Add `<RPC>` to `GO_IMPLEMENTED` in `_go_parity.py` in the same diff.
4. Pass the relevant `tests/python/integration/test_grpc_contract.py`
   case under `ENTDB_SERVER_TARGET=go` (CI matrix `server: go` leg
   green for that case).
5. Run `python -m pytest tests/python/unit/ tests/python/integration/ -q`
   green under both targets locally.
6. `cd server/go && go vet ./... && go test ./...` green.
7. Lint clean: `uvx ruff@0.15.7 check . && uvx ruff@0.15.7 format
   --check .`.
8. PR description references the spec under `docs/go-port/rpcs/<RPC>.md`
   and acknowledges any architectural-drift fix from §6.

`ExecuteAtomic` and `GetNode` PRs additionally unlock
`test_grpc_wire_format.py`; before they land that suite is
`pytestmark = requires_target("ExecuteAtomic")`-skipped on the Go leg.

## 8. Open questions / unresolved decisions

Aggregated from every spec, deduplicated, sorted by which Wave they
affect.

### Wave 1 blockers (must decide before/during shared-package PRs)

1. **JSON encoding determinism in WAL events.** Python `json.dumps`
   uses insertion order; Go's `encoding/json` sorts keys. The replay
   determinism test compares `payload_json` strings byte-for-byte.
   *Recommendation:* migrate Python side to sorted keys before the Go
   port lands. [applier.md OQ #1](shared/applier.md#open-questions--risks),
   [ExecuteAtomic.md OQ §determinism](rpcs/ExecuteAtomic.md).
2. **SQLite driver choice.** `modernc.org/sqlite` (no cgo) vs
   `mattn/go-sqlite3` (cgo, faster, SQLCipher-capable).
   *Recommendation:* `modernc.org/sqlite` for parity with the
   "single static binary" Go-server posture; revisit if encryption-at-rest
   re-enters scope. [canonical-store.md OQ #1](shared/canonical-store.md),
   [applier.md OQ #2](shared/applier.md).
3. **Error categorisation in applier.** Today Python lumps all into
   `ApplyResult.error = str(e)`. The Go port should distinguish
   poisoned events / constraint violations / transient infra. Wire as
   typed-error switch in the consume loop. [applier.md OQ #5](shared/applier.md).
4. **`global.db` durability story.** No WAL today; not snapshot'd
   either. *Recommendation:* snapshot to S3 alongside per-tenant
   snapshots (Phase 2). [global-store.md OQ #1](shared/global-store.md).
5. **Cross-tenant ACL store split.** `_check_capability` uses
   `include_cross_tenant=True` against the global `shared_index`. The
   Go port needs a clean abstraction across per-tenant SQLite + global
   store. *Recommendation:* `store.UnifiedGrantReader`. [acl.md OQ
   #3](shared/acl.md).
6. **`SetMaxOpenConns(1)` per tenant** vs N. Bench before locking in.
   [canonical-store.md OQ #5](shared/canonical-store.md).
7. **Group nesting depth bound.** Mirror Python's implicit cap;
   instrument when fired. [acl.md OQ #1](shared/acl.md).
8. **Single consumer goroutine vs per-tenant pool in applier.**
   *Recommendation:* start with single (Python parity); move to
   per-tenant if benchmarks show starvation. [applier.md
   §single-vs-pool](shared/applier.md).

### Wave 2 considerations (per-RPC; address in the relevant PR)

9. **Should ShareNode add a mailbox notification?** Currently no fanout
   on share; only `create_node` fans out. *Recommendation:* preserve
   parity, file follow-up. [ShareNode.md OQ](rpcs/ShareNode.md).
10. **`success=false` vs `status.Errorf` for ACL ops.** Keep the
    soft-fail shape (SDK clients parse the response field). [ShareNode.md
    OQ](rpcs/ShareNode.md), [RevokeAccess.md OQ](rpcs/RevokeAccess.md).
11. **`type_id` ignored in `GetNode`.** Preserve bug-for-bug; file
    follow-up to either enforce or delete the field.
    [GetNode.md OQ #1](rpcs/GetNode.md), [canonical-store.md OQ
    #6](shared/canonical-store.md).
12. **Idempotency-key derivation for `RevokeAllUserAccess` /
    `TransferUserContent`.** Server synthesises since proto lacks the
    field. Decide hashing strategy. [RevokeAllUserAccess.md
    OQ](rpcs/RevokeAllUserAccess.md).
13. **`RevokeDelegation` RPC?** No symmetric inverse for
    `DelegateAccess`. Track v2. [DelegateAccess.md OQ](rpcs/DelegateAccess.md).
14. **Legal-hold gate on `DeleteUser`.** New behaviour; ship behind a
    config flag to keep parity tests green. [DeleteUser.md OQ
    #4](rpcs/DeleteUser.md), [SetLegalHold.md OQ #3](rpcs/SetLegalHold.md).
15. **Large-user `TransferUserContent` (10M+ owned nodes).** Single
    UPDATE holds the writer slot. Decide chunked vs accept-the-lock.
    [TransferUserContent.md OQ #2](rpcs/TransferUserContent.md).
16. **Pre-apply count semantics in admin RPCs.** Counts taken AFTER
    WAL append but BEFORE applier consumes — racy but documented. Keep.
17. **Client-cancel semantics in `WaitForOffset`.** Python returns
    `OK+reached=false`; Go idiom is `ctx.Err()`. Pick one.
    [WaitForOffset.md OQ #4](rpcs/WaitForOffset.md).

### Wave 3 / post-port

18. **Tenant-claim from JWT vs request.** [auth-interceptor.md OQ
    #1](shared/auth-interceptor.md).
19. **Multi-IdP `sub` collisions.** [auth-interceptor.md OQ
    #5](shared/auth-interceptor.md).
20. **Per-tenant schema variants.** Out of scope; flag for post-port.
    [schema-registry.md OQ #2](shared/schema-registry.md).
21. **Audit-log table sunset.** Keep read-only for back-compat; do not
    extend. [canonical-store.md OQ #7](shared/canonical-store.md).
22. **Receipt FAILED status.** Never produced today. Don't speculatively
    emit. [GetReceiptStatus.md OQ](rpcs/GetReceiptStatus.md).

## 9. Wave 1 PR slate

To fire as a parallel agent batch when #408 merges. Each entry:
title / scope / deps / size estimate / agent prompt outline.

| # | Title | Scope | Wave-1 deps | Diff size | Agent prompt outline |
|---|---|---|---|---|---|
| W1.0 | `feat(go): generate pb stubs for entdb.v1` | `proto/entdb/v1/*.proto` → `server/go/internal/pb/entdbv1/` via `protoc-gen-go` + `protoc-gen-go-grpc`. Mod file. CI go-generate guard. | none | ~5k LOC generated | "Add buf/protoc generation for `proto/entdb/v1/entdb.proto` and `proto/console/v1/console.proto` into `server/go/internal/pb/`. Mirror naming used in `sdk/go/entdb/internal/pb/`. Add `go generate ./...` and a CI step that re-runs and fails on diff." |
| W1.1 | `feat(go): errs package — typed sentinels + trailers` | `server/go/internal/errs/{errs.go,map.go,trailers.go}` per [error-mapping.md](shared/error-mapping.md). | W1.0 | ~400 LOC | "Implement `server/go/internal/errs/` from `docs/go-port/shared/error-mapping.md`. Sentinels with `GRPCStatus()`. Trailer helpers. `PreserveStatusOrSwallow`. No `google.rpc` extensions." |
| W1.2 | `feat(go): version + metrics packages` | `server/go/internal/version/version.go`, `server/go/internal/metrics/grpc.go` (Prometheus counter+histogram). | none | ~200 LOC | "Add `version.Version` set via `-ldflags -X`. Add `metrics.RecordGRPCRequest(method, status, dur)` mirroring `server/python/entdb_server/metrics.py:103`." |
| W1.3 | `feat(go): schema registry` | `server/go/internal/schema/{types,registry,loader,fingerprint,translate}.go` per [schema-registry.md](shared/schema-registry.md). `LoadFromJSON` only (proto reflection deferred). | W1.0, W1.1 | ~1500 LOC | "Implement `server/go/internal/schema/` from `docs/go-port/shared/schema-registry.md`. JSON loader consumes `SchemaRegistry.to_json` shape. `Freeze()`+`Fingerprint()`. Compat package can be a stub." |
| W1.4 | `feat(go): wal producer + memory backend` | `server/go/internal/wal/{wal.go,event.go,memory.go}` per [wal-producer.md](shared/wal-producer.md). | W1.0, W1.1 | ~600 LOC | "Implement the producer interface + in-memory backend from `docs/go-port/shared/wal-producer.md`. Mirror `server/python/entdb_server/wal/memory.py`. Test helpers `GetAllRecords`/`WaitForRecords`/`ClearTopic`." |
| W1.5 | `feat(go): auth interceptor + trusted-actor` | `server/go/internal/auth/{interceptor,identity,actor,oauth,apikey,session,errors}.go` per [auth-interceptor.md](shared/auth-interceptor.md). | W1.0, W1.1 | ~800 LOC | "Implement the auth interceptor from `docs/go-port/shared/auth-interceptor.md`. In-memory JWKS / API-key / session managers. `Authoritative(ctx, claimed)` is the choke point." |
| W1.6 | `feat(go): payload translation` | `server/go/internal/payload/translate.go` per [payload-translation.md](shared/payload-translation.md). | W1.3 | ~300 LOC | "Implement `payload.StructToPayload`/`PayloadToStruct`/`FilterNamesToIDs`/`PayloadJSONToNames` from `docs/go-port/shared/payload-translation.md`. Internal map shape: `map[uint32]any`. Schema-less passthrough." |
| W1.7 | `feat(go): globalstore + tenant gate` | `server/go/internal/globalstore/{globalstore,users,tenants,members,shared,deletion,legalhold,quota,schema}.go` per [global-store.md](shared/global-store.md). Plus `server/go/internal/tenant/check.go`. | W1.0, W1.1 | ~1200 LOC | "Implement `globalstore` package from `docs/go-port/shared/global-store.md`. `MaxOpenConns(1)`. Add `tenant.CheckTenant(ctx, tenantID)` + `Sharding` struct (single-node default = always-mine)." |
| W1.8 | `feat(go): canonical store (read paths)` | `server/go/internal/store/{canonical,schema,pool,txn,visibility,fts,indexes,notifications,errors}.go` per [canonical-store.md](shared/canonical-store.md). Read methods + applier-callable writers. | W1.3, W1.6 | ~3000 LOC | "Implement `store.CanonicalStore` from `docs/go-port/shared/canonical-store.md`. `modernc.org/sqlite` driver. Lazy unique/query/composite/FTS5 indexes. Read methods for every Batch-B RPC. Writer methods for every Batch-C/E op type." |
| W1.9 | `feat(go): acl engine` | `server/go/internal/acl/{permission,capability,actor,grant,enforcer,resolver,registry,filter}.go` per [acl.md](shared/acl.md). | W1.3, W1.7, W1.8 | ~1000 LOC | "Implement `acl` from `docs/go-port/shared/acl.md`. `Enforcer.Check`+`FilterReadable`. Group resolution depth-bounded. `Registry` populated from `schema` package." |
| W1.10 | `feat(go): applier (WAL consumer)` | `server/go/internal/apply/{applier,event,result,storage_route,ops_*,alias,errors,fanout,shared_index}.go` per [applier.md](shared/applier.md). All op-type dispatch branches incl. the new ones from §6. | W1.4, W1.7, W1.8, W1.9, W1.6 | ~2500 LOC | "Implement `apply.Applier` from `docs/go-port/shared/applier.md`. Single consumer goroutine. All op types: `create_node`, `update_node`, `delete_node`, `create_edge`, `delete_edge`, plus the WAL-first additions per `docs/go-port/PLAN.md §6`. Halt-on-poison + idempotency in-txn." |

Wall-clock with 4 agents in worktrees: W1.0/1.1/1.2 in parallel ≈ 1
day; W1.3/1.4/1.5 in parallel ≈ 2 days; W1.6/1.7 in parallel ≈ 1 day;
W1.8 ≈ 3 days (largest); W1.9 ≈ 2 days; W1.10 ≈ 3 days. Total ≈ 12
working days; critical path ≈ 9 days.

## 10. Wave 2 PR slate

44 PRs, one per RPC. Title format: `feat(go-port): implement <RPC> in
Go server`. Every PR pulls in only the listed Wave-1 deps; agent prompt
template at the bottom of the table.

| # | RPC | Batch | Required Wave-1 PRs |
|---|---|---|---|
| W2.01 | `Health` | A | W1.0–1.2 (pb, errs, version, metrics) |
| W2.02 | `GetSchema` | A | + W1.3 (schema) |
| W2.03 | `GetReceiptStatus` | A/B | + W1.7, W1.8 (globalstore, store) |
| W2.04 | `WaitForOffset` | A/B | + W1.7, W1.8 |
| W2.05 | `GetNode` | B | + W1.5, W1.8, W1.9 (auth, store, acl) |
| W2.06 | `GetNodes` | B | same as GetNode |
| W2.07 | `GetNodeByKey` | B | same |
| W2.08 | `QueryNodes` | B | + W1.6 (payload) |
| W2.09 | `SearchNodes` | B | same as QueryNodes |
| W2.10 | `GetEdgesFrom` | B | as GetNode |
| W2.11 | `GetEdgesTo` | B | as GetNode |
| W2.12 | `GetConnectedNodes` | B | as GetNode |
| W2.13 | `ListSharedWithMe` | B | as GetNode + globalstore |
| W2.14 | `ExecuteAtomic` | C | + W1.4 (wal), W1.10 (apply) |
| W2.15 | `CreateUser` | D | W1.0–1.2 + W1.5 + W1.7 |
| W2.16 | `UpdateUser` | D | as CreateUser |
| W2.17 | `GetUser` | D | as CreateUser |
| W2.18 | `ListUsers` | D | as CreateUser |
| W2.19 | `GetUserTenants` | D | as CreateUser |
| W2.20 | `CreateTenant` | D | as CreateUser |
| W2.21 | `GetTenant` | D | as CreateUser |
| W2.22 | `ListTenants` | D | as CreateUser |
| W2.23 | `ArchiveTenant` | D | as CreateUser |
| W2.24 | `GetTenantQuota` | D | as CreateUser |
| W2.25 | `GetTenantMembers` | D | as CreateUser |
| W2.26 | `AddTenantMember` | D | as CreateUser |
| W2.27 | `RemoveTenantMember` | D | as CreateUser |
| W2.28 | `ChangeMemberRole` | D | as CreateUser |
| W2.29 | `ShareNode` | E | full (incl. wal+apply) — adds applier op `share_node` |
| W2.30 | `RevokeAccess` | E | full — adds applier op `revoke_access` |
| W2.31 | `DelegateAccess` | E | full — adds applier op `delegate_access` (FIX) |
| W2.32 | `TransferOwnership` | E | full — adds applier op `transfer_ownership` |
| W2.33 | `AddGroupMember` | E | full — adds applier op `add_group_member` |
| W2.34 | `RemoveGroupMember` | E | full — adds applier op `remove_group_member` |
| W2.35 | `DeleteUser` | F | D-set + W1.10 only if worker hooks added (not Wave 2) |
| W2.36 | `CancelUserDeletion` | F | as DeleteUser |
| W2.37 | `ExportUserData` | F | D-set + W1.8 (per-tenant reads) |
| W2.38 | `FreezeUser` | F | full + applier flag check |
| W2.39 | `RevokeAllUserAccess` | F | full — broadens applier op `admin_revoke_access` |
| W2.40 | `TransferUserContent` | F | full — fix visibility refresh in applier |
| W2.41 | `SetLegalHold` | F | full — adds applier op `set_legal_hold` |
| W2.42 | `SearchMailbox` | G | A-set only |
| W2.43 | `GetMailbox` | G | A-set only |
| W2.44 | `ListMailboxUsers` | G | A-set only |

### Agent prompt template

For each Wave-2 PR:

> Implement `<RPC>` in the Go server. Source spec:
> `docs/go-port/rpcs/<RPC>.md`. Coordination plan:
> `docs/go-port/PLAN.md`.
>
> 1. Read the spec end-to-end. Note any architectural-drift fix
>    flagged in §6 of the plan — your PR must close it (with a contract
>    test).
> 2. Implement the handler in `server/go/internal/api/<rpc>.go`. Reuse
>    helpers from `server/go/internal/api/helpers.go`
>    (`checkTenant`, `requireAdminOrOwner`, `getAuthoritativeActor`).
> 3. Replace the embedded `Unimplemented<RPC>` method on `Server`.
> 4. Add `<RPC>` to `GO_IMPLEMENTED` in
>    `tests/python/integration/_go_parity.py`.
> 5. Run the local CI suite per `CLAUDE.md` (`pytest`, `ruff`, `go vet`,
>    `go test`).
> 6. Run the matrix locally:
>    `ENTDB_SERVER_TARGET=go pytest tests/python/integration/test_grpc_contract.py -q`
>    expecting your RPC's case to pass and others to skip.
> 7. PR description: 2-line summary; link to `docs/go-port/rpcs/<RPC>.md`;
>    explicit acknowledgement of any §6 drift you closed; "Test plan"
>    bulleted.

Wall-clock estimate (8 worktree agents, post-Wave-1): Batch A+G ≈
2 days; Batch B ≈ 5 days (sequential `GetNode` first); Batch C ≈ 3
days (`ExecuteAtomic` is the lynchpin); Batch D ≈ 4 days (high
parallelism but globalstore conflicts gate two concurrent agents per
file area); Batch E ≈ 5 days (each ports an applier op + handler);
Batch F ≈ 4 days. Total ≈ 4 calendar weeks for the Wave-2 surface.
