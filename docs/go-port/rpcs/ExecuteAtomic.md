# RPC Port Spec — `entdb.v1.EntDBService/ExecuteAtomic`

> Implementation: `server/go/internal/api/execute_atomic.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. The central write path of EntDB and the
single most intricate RPC in the surface. Source of truth: Go handler at
`server/go/internal/api/execute_atomic.go` plus the applier at
`server/go/internal/apply/applier.go` (single-event path) and
`:575-895` (tenant-batch path).

This RPC is the **only** way mutations enter the system. It append-then-applies
a `TransactionEvent` containing one or more ops; the WAL is the source of truth,
SQLite is rebuilt from it (CLAUDE.md invariant 1).

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:49` (rpc), `:178-371` (messages).

`ExecuteAtomicRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `context` | 1 | `RequestContext` | Required. `tenant_id`, `actor`, optional `trace_id`. |
| `idempotency_key` | 2 | `string` | Optional; server generates a UUIDv4 when empty (`server/go/internal/api/execute_atomic.go`). |
| `schema_fingerprint` | 3 | `string` | Optional. When non-empty AND server has a fingerprint, mismatch → in-band failure (NOT abort). |
| `operations` | 4 | `repeated Operation` | Required, ≥1. Empty list → `INVALID_ARGUMENT`. |
| `wait_applied` | 5 | `bool` | Block return until applier writes the event's stream position to SQLite. |
| `wait_timeout_ms` | 6 | `int32` | Apply-wait timeout. Default `30_000` when zero (`server/go/internal/api/execute_atomic.go`). |

`Operation` is a `oneof op { CreateNodeOp create_node = 1; UpdateNodeOp update_node = 2; DeleteNodeOp delete_node = 3; CreateEdgeOp create_edge = 4; DeleteEdgeOp delete_edge = 5; }` (`entdb.proto:198-206`).

`CreateNodeOp` (`:230-270`): `type_id`, optional `id` (server fills with UUIDv4 when empty), `as` alias, `fanout_to`, `data google.protobuf.Struct`, `acl repeated AclEntry`, `storage_mode` enum (`TENANT|USER_MAILBOX|PUBLIC`), `target_user_id` (required iff `USER_MAILBOX`). Reserved tags: `3,4,11` and field names `data_json/acl_json/keys`.

`UpdateNodeOp` (`:272-296`): `type_id`, `id` (or alias `$alias.id` form), `field_mask`, `patch google.protobuf.Struct`. Reserved tags `3,6` (`patch_json`, `keys`).

`DeleteNodeOp` (`:298-304`): `type_id`, `id`.

`CreateEdgeOp` (`:306-322`): `edge_id`, `from NodeRef`, `to NodeRef`, `props google.protobuf.Struct`. Reserved tag `4` (`props_json`).

`DeleteEdgeOp` (`:324-333`): `edge_id`, `from NodeRef`, `to NodeRef`.

`NodeRef` (`:335-351`): `oneof ref { string id; string alias_ref; TypedNodeRef typed; }`.

**Payload encoding (CRITICAL).** On the wire `data` and `patch` Structs are
**name-keyed**. The handler translates them to **id-keyed** (`{"<field_id>":
value}`) at `_convert_operations` via `name_to_id_keys` before WAL append
(`server/go/internal/api/execute_atomic.go,882`; impl `schema/field_id_translation.py:75`). Unknown
field names are silently dropped. On the WAL, in SQLite, and on Read responses,
payloads are id-keyed. Pinned by `test_grpc_wire_format.py:172,202`.

`ExecuteAtomicResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `success` | 1 | `bool` | False on schema mismatch / WAL append exception. |
| `receipt` | 2 | `Receipt` | `tenant_id`, `idempotency_key`, `stream_position` (decimal string of WAL offset, `""` when WAL returned None). |
| `error` | 3 | `string` | Set when `success=false`. |
| `error_code` | 4 | `string` | `SCHEMA_MISMATCH` or `INTERNAL`. NB: code-level rejections (`INVALID_ARGUMENT`, `PERMISSION_DENIED`, `ALREADY_EXISTS`, `FAILED_PRECONDITION`, `NOT_FOUND`, `UNAVAILABLE`) are surfaced via `context.abort` and DO NOT populate this field. |
| `created_node_ids` | 5 | `repeated string` | One entry per `create_node` op, in op order. Server-generated UUIDs included. |
| `applied_status` | 6 | `ReceiptStatus` | `PENDING` unless `wait_applied=true` AND offset reached → `APPLIED`. Never `FAILED` from this RPC (use `GetReceiptStatus`). |

## Auth

`AuthInterceptor` (`auth/auth_interceptor.py:140`) authenticates the call,
binds the verified principal into a `ContextVar`, and lets the handler call
`get_authoritative_actor(request_actor)` (`auth_interceptor.py:92`) to obtain
the **trusted** actor. The wire `request.context.actor` is UNTRUSTED.

- `_trusted_actor(ctx.actor)` is invoked at `server/go/internal/api/execute_atomic.go` and the
  handler **rebinds its actor variable** so no later code path can re-read
  the wire value. Pinned by
  `tests/python/integration/test_privilege_escalation.py:266` (rejects claimed
  admin) and `:287` (persists trusted actor in WAL event, never the claim).
  Go port: rebind the local `actor` immediately after extraction. See PR #168.
- `_check_tenant_access(tenant_id, actor=trusted_actor, require_write=true,
  op_kind="delete" if any delete else "write")` runs at `server/go/internal/api/execute_atomic.go`.
  Enforces:
  - Tenant existence (`NOT_FOUND`).
  - Tenant `status` (`active`, `archived`, `legal_hold`, `deleted`):
    - `archived` + write → `FAILED_PRECONDITION`
    - `legal_hold` + delete → `FAILED_PRECONDITION` (creates/updates allowed; pinned `test_admin_operations.py:665,686`).
    - `deleted` → `NOT_FOUND`.
  - Membership: non-member → `PERMISSION_DENIED`. `viewer`/`guest` + write →
    `PERMISSION_DENIED` (`test_tenant_roles.py:546,558`).
  - Actor-user `status` of `frozen` or `pending_deletion` (GDPR Art. 18
    restriction) + write → `PERMISSION_DENIED` (`server/go/internal/api/execute_atomic.go`).
  - System actors (`system:*` or `__system__`) bypass all (`server/go/internal/api/execute_atomic.go`).
- `_check_tenant(tenant_id)` runs FIRST at `server/go/internal/api/execute_atomic.go`. Rejects if
  this node is not the tenant's shard owner: sets trailing metadata
  `entdb-redirect-node: <owner_node_id>` and aborts `UNAVAILABLE`. Pinned by
  redirect-cache contract tests; the trailer is what SDKs use to retry
  elsewhere (`sdk/go/entdb/redirect_cache.go`). Region pin mismatch →
  `FAILED_PRECONDITION` (`server/go/internal/api/execute_atomic.go`).
- **Per-op `Permission` checks: NONE at the gRPC layer.** ACL is enforced on
  reads (`can_access`) and on share/grant ops elsewhere; ExecuteAtomic relies
  on tenant-membership + role for write authorization. There is no
  per-`type_id` write capability check today — the Go port must replicate
  this exactly to preserve behavior.
- The persisted WAL event's `actor` field is the trusted actor
  (`server/go/internal/api/execute_atomic.go`). Audit-truth invariant — do not regress.

## Side effects

End-to-end flow (CLAUDE.md invariant 1: handler → `wal.append` → applier →
SQLite; never direct):

1. **Pre-validation** (handler, before WAL):
   - Tenant ownership / region (`_check_tenant`, `server/go/internal/api/execute_atomic.go`).
   - Required-field validation (`server/go/internal/api/execute_atomic.go`).
   - Schema fingerprint check against `self.schema_registry.fingerprint`
     (`:651`). Mismatch returns `success=false, error_code="SCHEMA_MISMATCH"`
     in-band. **Do not abort.** Pinned `test_grpc_schema_mismatch_metric.py:61`.
   - `_convert_operations` — proto → internal dicts; **field-id translation
     via `name_to_id_keys`** for `create_node.data` and `update_node.patch`
     (`server/go/internal/api/execute_atomic.go,882`); enum `storage_mode` → string name; node refs
     normalized (`{"ref":"alias"}` or `{"type_id":N,"id":"X"}` or bare id).
   - Fast-fail unique pre-check (single-field): for each `create_node`, look
     up declared unique fields via `schema_registry.get_unique_field_ids`,
     then `canonical_store.get_node_by_key(tenant, type_id, fid, value)`. Hit
     → abort `ALREADY_EXISTS` (`server/go/internal/api/execute_atomic.go`). Pinned
     `test_unique_keys.py:569`.
   - Fast-fail composite-unique pre-check
     (`schema_registry.get_composite_unique_constraints`,
     `canonical_store.get_node_by_composite_key`, `server/go/internal/api/execute_atomic.go`).
     Both pre-checks are **best-effort only**; the unique expression index at
     apply time is the authoritative race winner (2026-04-14 SDK v0.3 ADR).
   - Tenant access + role + status (`_check_tenant_access`, `:758`).
   - Server-fill empty `create_node.id` with UUIDv4; collect
     `created_node_ids` in op order (`:768-772`).

2. **Build event** (`server/go/internal/api/execute_atomic.go`): `{tenant_id, actor=trusted,
   idempotency_key, schema_fingerprint=server's, ts_ms=int(time.time()*1000),
   ops=[...]}` JSON-encoded UTF-8.

3. **WAL append** (`:790`): `await self.wal.append(self.topic, tenant_id,
   event_bytes)`. Partition key = `tenant_id` so all of a tenant's events
   land on one partition (in-order replay). Returns `stream_pos` (string-ish;
   may be None for in-memory backend → empty `stream_position`).

4. **Build receipt** (`:796-800`).

5. **Optional apply-wait** (`:804`): `await
   canonical_store.wait_for_offset(tenant_id, str(stream_pos), timeout)` —
   event-driven, no polling (`server/go/internal/store/`). Sets
   `applied_status=APPLIED` on success; otherwise leaves `PENDING`.

6. **Applier (out-of-band, `apply_event`, `server/go/internal/apply/applier.go`)** — invariants:
   - Every op + idempotency check + `applied_events` insert run in a **single
     SQLite transaction** (`_sync_apply_event_body`, `server/go/internal/apply/applier.go`). Roll
     back on any exception, including the idempotency record — so retries
     are safe (`apply_event` docstring `:1327`).
   - **Storage routing.** `_event_storage_mode` (`server/go/internal/apply/applier.go`) inspects
     `create_node` ops and picks one of `TENANT`, `USER_MAILBOX` (requires
     `target_user_id`), or `PUBLIC` for the whole event. Mixed-mode events →
     `ValidationError` (raised inside applier, surfaces only via
     `GetReceiptStatus=FAILED`; the gRPC handler will already have returned
     a successful Receipt). `_open_event_connection` (`:669`) opens
     `batch_transaction` / `mailbox_batch_transaction(target_user)` /
     `public_batch_transaction()`.
   - **Idempotency** at apply time: `SELECT 1 FROM applied_events WHERE
     tenant_id=? AND idempotency_key=?` inside the txn (`server/go/internal/apply/applier.go`,
     `:766`). Hit → skip the entire event, harmless empty COMMIT. Same key
     → same outcome, even after WAL replay. Pinned
     `test_wal_replay_determinism.py:385`.
   - **Per-op apply** (`server/go/internal/apply/applier.go`):
     - `create_node`: lazy-create field indexes
       (`canonical_store._ensure_field_indexes`); `create_node_raw` insert;
       on `sqlite3.IntegrityError` matching `_parse_unique_index_name` →
       `UniqueConstraintError`; matching composite → `CompositeUniqueConstraintError`;
       record alias in `_node_alias_map`; FTS index insert for searchable
       fields; data-policy log.
     - `update_node`: reject patch containing `storage_mode` key
       (immutable, `server/go/internal/apply/applier.go`); read-modify-write on `payload_json`
       (`UPDATE ... SET payload_json=?, updated_at=?`); same unique-violation
       parsing as create; FTS update.
     - `delete_node`: FTS delete; `DELETE FROM edges WHERE from=? OR to=?`;
       `DELETE FROM node_visibility`; `DELETE FROM nodes`. Append id to
       `deleted_node_ids` for post-commit shared-index cleanup.
     - `create_edge`: resolve `from`/`to` (alias / typed / bare id);
       `CanonicalStore._validate_edge_direction` enforces
       `USER_MAILBOX → TENANT → PUBLIC` privacy hierarchy
       (`server/go/internal/apply/applier.go`); `INSERT OR REPLACE INTO edges`; if
       `propagates_acl`, run recursive cycle check (depth 10) and
       `INSERT OR IGNORE INTO acl_inherit (to, from)` (`:1190-1212`).
     - `delete_edge`: `DELETE FROM edges` and `DELETE FROM acl_inherit`.
   - **Record applied** (same txn): `INSERT INTO applied_events
     (tenant_id, idempotency_key, stream_pos, applied_at)` (`:1252`).
   - **Post-commit (NOT in txn):**
     - Mailbox fanout for `create_node` ops when `fanout_config.enabled`
       (`server/go/internal/apply/applier.go`).
     - Shared-index cleanup for deleted nodes (`:1370`).
     - Phase-1 quota: `_increment_usage_safe(tenant_id, len(event.ops))`
       (`:1377`). **Fire-and-forget** — failures MUST NOT block the apply
       path. Billing drift is preferable to write outage
       (`docs/decisions/quotas.md`). Counter increments per *op*, not per
       event; replays do not double-count because skipped events return
       early before the increment. Tenant-batch path increments once per
       batch (`server/go/internal/apply/applier.go`).
     - `update_applied_offset(tenant_id, stream_pos)` for the wait-watchers
       (`:599`, batch path; single-event path notifies via the same store).

7. **Tenant scope.** Every `create_node`/`update_node`/`delete_node` op
   touches `tenant_<id>.db` only (or the per-user mailbox / public.db when
   routed). Cross-tenant work is forbidden in a single SQLite txn (CLAUDE.md
   invariant 4). The Go port must keep the per-tenant connection-pool +
   thread-pool model: synchronous SQLite work runs on
   `canonical_store._executor` so the gRPC event loop never blocks
   (`server/go/internal/apply/applier.go`).

## Error contract

All `context.abort` paths immediately raise — the handler's outer
`except Exception` (`server/go/internal/api/execute_atomic.go`) only re-records the metric and
re-raises. The few in-band failure paths set `success=false`.

| Trigger | gRPC code / signal | Source |
|---|---|---|
| Tenant not on this node | `UNAVAILABLE` + `entdb-redirect-node` trailer | `server/go/internal/api/execute_atomic.go` |
| Tenant region mismatch | `FAILED_PRECONDITION` | `:404` |
| Empty `tenant_id` | `INVALID_ARGUMENT` | `:633` |
| Empty `actor` | `INVALID_ARGUMENT` | `:635` |
| Empty `operations` | `INVALID_ARGUMENT` | `:637` |
| `schema_fingerprint` mismatch | in-band: `success=false`, `error_code="SCHEMA_MISMATCH"` | `:653` |
| `create_node` single-field unique pre-collision | `ALREADY_EXISTS` | `:696` |
| `create_node` composite unique pre-collision | `ALREADY_EXISTS` | `:744` |
| Tenant `deleted` | `NOT_FOUND` | `:512` |
| Tenant unknown | `NOT_FOUND` | `:504` |
| Tenant `archived` (any write) | `FAILED_PRECONDITION` | `:518` |
| Tenant `legal_hold` + any delete op | `FAILED_PRECONDITION` | `:524` |
| Non-member of tenant | `PERMISSION_DENIED` | `:535` |
| Role `viewer`/`guest` (write) | `PERMISSION_DENIED` | `:541` |
| User `frozen` / `pending_deletion` (write) | `PERMISSION_DENIED` | `:553` |
| WAL append exception | in-band: `success=false`, `error_code="INTERNAL"` | `:818-825` |
| Apply-time unique race (single) | NOT surfaced via this RPC; `GetReceiptStatus` later returns FAILED | `server/go/internal/apply/applier.go` |
| Apply-time composite unique race | same | `server/go/internal/apply/applier.go` |
| Apply-time mixed storage modes / missing `target_user_id` | same — `ValidationError` | `server/go/internal/apply/applier.go,656` |
| Apply-time `update_node` patches `storage_mode` | same — `ValidationError` | `server/go/internal/apply/applier.go` |
| Apply-time edge-direction privacy violation | same — `ValidationError` | `server/go/internal/apply/applier.go` |

Reminder: `error_code` is populated **only** on the schema-mismatch and
WAL-append-exception paths. All other failure modes are aborts.

## Shared Go package deps

| Package | Why |
|---|---|
| `pb` (gen from `proto/entdb/v1/entdb.proto`) | Wire types: `ExecuteAtomicRequest`, `Operation`, `Receipt`, `RequestContext`, `NodeRef`, `Struct`. |
| `auth` | `AuthInterceptor` + `GetAuthoritativeActor(ctx)` to fetch the trusted actor from the request `context.Context`. Mirrors `auth_interceptor.py:92`. |
| `wal` | `WAL.Append(topic, partitionKey, payload []byte) (StreamPosition, error)`; backends Kafka / Kinesis / Memory. |
| `apply` | `Applier` (long-running consumer) — not called inline by the handler; the handler must NOT touch SQLite directly (CLAUDE.md invariant 1). |
| `store` (canonical) | `GetNodeByKey`, `GetNodeByCompositeKey`, `WaitForOffset`, `CheckIdempotency` (used by `GetReceiptStatus`). Also exposes the per-tenant SQLite executor used by the applier. |
| `schema` | `Registry.Fingerprint()`, `GetUniqueFieldIDs(typeID)`, `GetCompositeUniqueConstraints(typeID)`, `GetSearchableFieldIDs`, plus `NameToIDKeys(struct, typeID)` for the field-id translation. |
| `acl` | Imported transitively for `AclEntry` proto-to-struct conversion only; no per-op write check happens in this RPC. |
| `errs` | Map server-side errors to `status.Errorf(codes.X, ...)`; helper to set the `entdb-redirect-node` trailer. |
| `idempotency` | UUIDv4 generator (Python uses `uuid.uuid4()`); the *check* is at apply time, not here. |
| `quotas` | NOT used by the handler. Quota accounting lives in the applier post-commit (`server/go/internal/apply/applier.go`). |
| `globalstore` | Tenant + member + user lookups (`get_tenant`, `is_member`, `get_user`). Returns `nil` to skip checks (preserves no-global-store unit-test path, `test_tenant_roles.py:494`). |
| `sharding` | `IsMine(tenantID)`, `GetOwner(tenantID)` for the `_check_tenant` redirect. |
| `metrics` | `RecordGRPCRequest("ExecuteAtomic", "ok"|"error", elapsed)`. Schema mismatch records `"error"` (regression-pinned). |

## Other-RPC deps

- **`GetReceiptStatus`** (`server/go/internal/api/execute_atomic.go`) reads
  `canonical_store.check_idempotency(tenant, key)` to translate the
  `idempotency_key` from the receipt to `RECEIPT_STATUS_APPLIED` /
  `RECEIPT_STATUS_PENDING`. The Go port must preserve the same
  `applied_events` table semantics so a receipt from `ExecuteAtomic` is
  observable via `GetReceiptStatus`. Apply-time `ValidationError`s and
  unique races are how `RECEIPT_STATUS_FAILED` becomes observable.
- **`WaitForOffset`** (`server/go/internal/api/execute_atomic.go`) is the standalone version of
  the `wait_applied` flag's logic. Both call
  `canonical_store.wait_for_offset(tenant, pos, timeout)`. The Go port
  should expose a single `Store.WaitForOffset` shared by both.
- **`GetNode` / `GetNodeByKey`** are the read counterparts; they also call
  `_trusted_actor` and use `wait_for_offset` with `after_offset` for
  read-after-write consistency. Their port must wait on the same
  offset-notification mechanism the applier writes via
  `update_applied_offset` (`server/go/internal/apply/applier.go`).

## Contract tests pinning behavior

- Privilege-escalation, claimed-admin actor rejected:
  `tests/python/integration/test_privilege_escalation.py:266`.
- Trusted actor (not wire claim) persisted in WAL event:
  `tests/python/integration/test_privilege_escalation.py:287`.
- Wire payload is field-id-keyed:
  (legacy Python unit test, removed in Phase 4D).
- ID-keyed input round-trips:
  (legacy Python unit test, removed in Phase 4D).
- Idempotency same-key returns same content:
  (legacy Python unit test, removed in Phase 4D).
- Oversized payload rejected:
  (legacy Python unit test, removed in Phase 4D).
- Empty payload tolerated:
  (legacy Python unit test, removed in Phase 4D).
- Unknown field names dropped silently:
  (legacy Python unit test, removed in Phase 4D).
- Schema mismatch → `success=false, error_code=SCHEMA_MISMATCH`, metric
  recorded as `"error"`: (legacy Python unit test, removed in Phase 4D).
- Single-field unique pre-check returns `ALREADY_EXISTS`:
  (legacy Python unit test, removed in Phase 4D).
- Tenant-role matrix (owner/admin/member/viewer/guest/non-member, archived,
  legal_hold, deleted, system actors):
  (legacy Python unit test, removed in Phase 4D).
- `legal_hold` rejects `delete_node` but allows `create_node`:
  (legacy Python unit test, removed in Phase 4D).
- AuthInterceptor classifies `/entdb.EntDBService/ExecuteAtomic` as
  authenticated/rate-limited/needing actor:
  `(legacy Python unit test, removed),732,754,786`,
  (legacy Python unit test, removed in Phase 4D),
  (legacy Python unit test, removed in Phase 4D).
- Replay determinism + duplicate idempotency:
  `tests/python/integration/test_wal_replay_determinism.py:179,385,443`.
- Cross-RPC contract list (delegates ExecuteAtomic coverage to the dedicated
  files): `tests/python/integration/test_grpc_contract.py:758-760`.

## Implementation outline (Go)

```go
func (s *Server) ExecuteAtomic(ctx context.Context, req *pb.ExecuteAtomicRequest)
    (*pb.ExecuteAtomicResponse, error) {
    start := time.Now()
    defer func() { metrics.RecordGRPC("ExecuteAtomic", outcome, time.Since(start)) }()
```

1. **Sharding gate.** `s.checkTenant(ctx, req.Context.TenantId)` — if not
   ours, set trailer `entdb-redirect-node` with `sharding.GetOwner` and
   return `codes.Unavailable`. Region mismatch → `codes.FailedPrecondition`.
2. **Required fields.** `tenant_id`, `actor`, `operations` all non-empty,
   else `codes.InvalidArgument`.
3. **Trusted actor.** `actor := auth.AuthoritativeActor(ctx,
   req.Context.Actor)` — REBIND the local var; never re-read
   `req.Context.Actor` past this line. (PR #168 invariant.)
4. **Idempotency key.** `idem := req.IdempotencyKey; if idem == "" { idem =
   uuid.NewString() }`.
5. **Schema fingerprint.** If `req.SchemaFingerprint != "" &&
   s.schema.Fingerprint() != "" && != req.SchemaFingerprint` → return
   `&pb.ExecuteAtomicResponse{Success:false, Error:..., ErrorCode:
   "SCHEMA_MISMATCH"}, nil` and record `"error"` metric. Do NOT abort.
6. **Convert operations.** For each `Operation`:
   - `create_node`: copy fields; **`data = schema.NameToIDKeys(structToMap(create.Data),
     create.TypeId)`** (id-keyed payload — CLAUDE.md invariant 6); convert
     `acl` proto → list; map storage-mode enum → string
     (`TENANT`/`USER_MAILBOX`/`PUBLIC`); preserve `as`, `fanout_to`,
     `target_user_id`.
   - `update_node`: `patch = schema.NameToIDKeys(...)`; preserve `id`, `type_id`.
   - `delete_node`: `type_id`, `id`.
   - `create_edge` / `delete_edge`: `convertNodeRef(from)`, `convertNodeRef(to)`;
     `props = structToMap` (NOT translated — edges are properties-shaped,
     not declared-fields-shaped).
7. **Fast-fail unique pre-checks** (best-effort, race winner is the unique
   index at apply time):
   - For each `create_node` op with unique fields: call
     `store.GetNodeByKey(tenant, typeID, fieldID, value)`; non-nil →
     `codes.AlreadyExists`.
   - Same for composite uniques via `store.GetNodeByCompositeKey`.
8. **Tenant access.** `s.checkTenantAccess(ctx, tenant, trustedActor,
   requireWrite=true, opKind=("delete" if hasDelete else "write"))`.
9. **Server-fill IDs.** For each `create_node` with empty `id`, set
   `op.ID = uuid.NewString()`; record into `createdNodeIDs` in op order.
10. **Build event.**
    ```go
    event := map[string]any{
        "tenant_id": tenant, "actor": trustedActor,
        "idempotency_key": idem,
        "schema_fingerprint": s.schema.Fingerprint(),
        "ts_ms": time.Now().UnixMilli(),
        "ops": ops,
    }
    payload, _ := json.Marshal(event)
    ```
    JSON encoding must be byte-stable enough that replay produces identical
    state — see Determinism section.
11. **WAL append.** `pos, err := s.wal.Append(ctx, s.topic, tenant, payload)`.
    On err → in-band `Success=false, ErrorCode="INTERNAL"`, return `nil, nil`.
    `tenant` is the partition key — keeps a tenant's events on one partition
    (in-order replay).
12. **Build receipt.** `Receipt{TenantId: tenant, IdempotencyKey: idem,
    StreamPosition: posString}` (empty string when `pos` zero/None).
13. **Optional apply-wait.** If `req.WaitApplied && pos != ""`:
    `applied := s.store.WaitForOffset(ctx, tenant, posString, timeout)`;
    set `appliedStatus = APPLIED` on success, else `PENDING`.
14. **Return.** `&pb.ExecuteAtomicResponse{Success: true, Receipt: receipt,
    CreatedNodeIds: createdNodeIDs, AppliedStatus: appliedStatus}`.

The applier (separate goroutine consuming from WAL) is the SQLite writer.
The handler MUST NOT touch SQLite for mutations (CLAUDE.md invariant 1).
The single-tenant happy-path puts handler latency at: tenant gate +
fast-fail uniques (1–2 SQLite reads) + WAL append + optional offset wait.

## Determinism + replay

The applier is replayed on rebuilds; **identical input must produce
identical SQLite state** (`test_wal_replay_determinism.py:179`).

- **`ts_ms` is captured at handler time and stored in the event**
  (`server/go/internal/api/execute_atomic.go`). The applier uses `event.ts_ms` for `created_at` /
  `updated_at`, NOT `time.Now()`. Pinned by the determinism test. Go port
  must do the same — never call `time.Now()` inside the apply path.
- **`applied_at` in `applied_events` uses `time.Now()` at apply time**
  (`server/go/internal/apply/applier.go,1260`). This is metadata, not state used for joins, so
  the determinism test does not pin it. Acceptable to differ on replay.
- **`idempotency_key` is the dedup key**: replays of the same WAL event are
  no-ops. The Go applier must keep the `applied_events` UNIQUE-keyed by
  `(tenant_id, idempotency_key)` and check inside the txn.
- **Server-generated UUIDs for empty `create_node.id`** are written into
  the WAL event before append (`server/go/internal/api/execute_atomic.go`). Replay reads the same
  ID off the log — this is what makes IDs deterministic across replays.
  Go port: generate the UUID **before** WAL append, never inside the
  applier.
- **Map iteration order risk.** Python `json.dumps` produces stable output
  for dicts because Python 3.7+ dicts are insertion-ordered. Go's
  `encoding/json` sorts map keys alphabetically. Both are stable, but the
  bytes will differ — that is fine because the WAL stores bytes and the
  applier re-parses to a map; reads of `event.ts_ms` etc. don't depend on
  key order. Document this in the Go applier so reviewers don't add a
  "match Python's exact bytes" requirement.
- **`storage_mode` resolution.** `_event_storage_mode` walks ops in order
  to detect mixed-mode (`server/go/internal/apply/applier.go`). Go port must walk in the
  same order — but since mixed-mode raises, the only observable difference
  on replay is which op the error message names. Test file does not pin
  the message, so order does not matter functionally.
- **Alias resolution** (`_node_alias_map`) must be cleared per event
  (`server/go/internal/apply/applier.go,911`). Carrying state across events is a determinism
  hazard.
- **Quota counter** (`_increment_usage_safe`) is post-commit and
  fire-and-forget. On replay, only events that actually committed (skipped
  duplicates excluded) bump the counter — this is correct because the WAL
  is the only producer of writes.

## Open questions / risks

1. **No per-op `Permission` check today.** The Go port must preserve this
   to avoid breaking integration tests, but it is a known gap (any tenant
   member with write role can create any `type_id`). Track separately —
   not in scope for the port.
2. **Schema mismatch is in-band, not an abort.** Surprising — the Go port
   should keep it for SDK compatibility but consider documenting it as a
   wart. Pinned by `test_grpc_schema_mismatch_metric.py:51`.
3. **`ts_ms` from the gRPC layer means clock skew between gRPC nodes
   becomes part of the event log.** No NTP requirement is currently
   enforced. Replay is still deterministic per-event, but ordering across
   producers depends on partition append order, not `ts_ms`.
4. **Best-effort unique pre-check is racy.** The handler calls
   `store.GetNodeByKey` BEFORE WAL append, but the applier of an
   in-flight event may not have committed yet. Two concurrent
   `ExecuteAtomic` calls with the same unique value can both pass the
   pre-check; the unique expression index at apply time then fails the
   second event, and `GetReceiptStatus` reports `FAILED`. Documented in
   `server/go/internal/api/execute_atomic.go`. Go port must replicate, including the in-band
   nature of the failure (NOT an abort).
5. **`wait_applied` blocks the gRPC call.** Default 30s timeout. Under
   slow apply or WAL backpressure, this can balloon the call duration —
   the Go port should plumb the request `context.Context` deadline through
   `Store.WaitForOffset` so client cancellation is honored.
6. **Mailbox-fanout determinism on replay** — `_fanout_node` happens
   post-commit and is NOT bounded by the WAL. If a replay re-runs fanout,
   downstream notifications can duplicate. Out of scope for ExecuteAtomic
   port; worth flagging for the applier port.
7. **Handler logs `exc_info=True` on the WAL-append exception
   (`server/go/internal/api/execute_atomic.go`).** The Go port should keep stack-trace logging
   for production triage.
8. **`error_code` is a `string`, not a proto enum.** Only two values
   today (`SCHEMA_MISMATCH`, `INTERNAL`). Resist the urge to enumify in
   the port — it would change the wire schema.
