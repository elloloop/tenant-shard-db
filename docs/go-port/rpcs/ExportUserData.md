# ExportUserData — Go Port Spec

EPIC #407. GDPR Article 20 portability: emit every node a user owns or
is the subject of, **across every tenant they're a member of**, as a
JSON bundle. Cross-tenant by construction.

Python source of truth: `server/python/entdb_server/api/grpc_server.py:3014-3070`.
Per-tenant collector: `server/python/entdb_server/apply/canonical_store.py:5174-5249`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:137` (RPC), `:1027-1037` (messages).

```
rpc ExportUserData(ExportUserDataRequest) returns (ExportUserDataResponse);
ExportUserDataRequest  { string actor = 1; string user_id = 2; }
ExportUserDataResponse { bool success = 1; string export_json = 2;
                         string error = 3; }
```

**Sync, single-call, no job ID.** Bundle built in-process, returned
inline as `export_json`. No streaming RPC, no async job, no signed URL,
no resumable download. Format:

```
{ "user_id": "...", "generated_at": <unix_seconds>,
  "tenants": [ { "tenant_id", "nodes": [ { "tenant_id", "node_id",
    "type_id", "payload", "created_at", "updated_at",
    "owner_actor", "data_policy" }, ... ] }, ... ] }
```

Pinned by `tests/python/unit/test_gdpr_engine.py:678-692`.

`payload` is **id-keyed** per invariant #6 — `_sync_export_user_data`
(`canonical_store.py:5211`) returns `json.loads(payload_json)` raw. This
bypasses `Struct` serialization (the field is `string`, not
`google.protobuf.Struct`); the Go port uses `encoding/json` to marshal
a `map[string]any`.

## Auth

1. **`actor` and `user_id` required** (`grpc_server.py:3024-3027`):
   empty → `INVALID_ARGUMENT`.
2. **Trusted-actor only.** `_is_self_or_admin`
   (`grpc_server.py:2071-2086`) calls
   `auth.auth_interceptor.get_authoritative_actor(actor)` and **ignores
   the wire-supplied actor for the decision**. Allowed:
   - `trusted == f"user:{user_id}"` or `trusted == user_id` — **self**.
   - `system:*` / `admin:*` / `__system__` — **admin / compliance**.
   - else → `PERMISSION_DENIED` `"ExportUserData requires the user
     themselves or an admin actor"` (`grpc_server.py:3028-3032`).
   Pinned by `tests/python/unit/test_gdpr_engine.py:696-704` and
   `tests/python/integration/test_grpc_contract.py:649-653`
   (actor=ALICE, user_id="bob" → permission_denied).
3. **No per-tenant capability check.** Self/admin alone authorizes the
   walk over all `get_user_tenants`. Cross-tenant invariant #4 holds
   because each tenant's SQLite is read with its own connection.
4. **No `RequestContext`.** Flat `actor` + `user_id` — no `tenant_id`,
   no sharding redirect, no region pin.
5. `global_store is None` → `UNIMPLEMENTED "User registry not configured"`
   (`grpc_server.py:3022-3023`).

## Side effects

**Read-only across tenants + global_store.** No WAL append, no SQLite
writes, no artifact persistence.

- `global_store.get_user_tenants(user_id)` — global SQLite read for
  membership list (`grpc_server.py:3034`).
- For each tenant: `canonical_store.export_user_data_for_tenant`
  → `_run_sync(_sync_export_user_data)` → per-tenant SQLite read of
  `nodes WHERE tenant_id = ?` (`canonical_store.py:5193-5198`).
  Per-tenant connections, never a cross-tenant transaction.
- Per-tenant exceptions are **caught and logged**, not propagated
  (`grpc_server.py:3048-3053`) — one bad tenant does not fail the bundle.
- `record_grpc_request("ExportUserData", …)` metric.

**No artifact written** — bundle returned inline; no encryption-at-rest
layer because there is no artifact, only TLS in transit.

**No WAL audit record of the export itself.** This is a *read*, so
"all writes through WAL" (invariant #1) is not violated, but Article 30
record-of-processing is currently only captured by the auth
interceptor's structured logs + metrics — not by the tamper-evident
S3-Object-Lock WAL. See open questions.

## Error contract

First failure wins:

1. `global_store is None` → `UNIMPLEMENTED`.
2. `actor == ""` → `INVALID_ARGUMENT "actor is required"`.
3. `user_id == ""` → `INVALID_ARGUMENT "user_id is required"`.
4. Not self/admin/system → `PERMISSION_DENIED`.
5. Per-tenant collector failure → **swallowed** (logged, tenant omitted
   from `tenants[]`). Bundle still returns `success=true`.
6. Unexpected exception elsewhere → `success=false, error=str(e)`,
   gRPC status `OK`, metric `error` (`grpc_server.py:3065-3070`).

**Go port:** real aborts via `status.Errorf`. The Python try/except
swallow is unreachable in `grpc.aio` production paths; preserve the
`success=false, error=…` shape only for genuine internal errors.

## Shared Go package deps

(Paths proposed; replace with whatever EPIC #407 freezes.)

- `auth.AuthoritativeActor(ctx) string` — same as `GetNode` spec.
- `auth.IsSelfOrAdmin(trusted, userID) bool` — encodes the
  `user:{id}` / `system:` / `admin:` / `__system__` rules
  (`grpc_server.py:2071-2086`). **Shared with** `DeleteUser`,
  `FreezeUser`, `CancelUserDeletion`.
- `globalstore.Store.GetUserTenants(ctx, userID) ([]Membership, error)`.
- `canonical.Store.ExportUserData(ctx, tenantID, principal, registry) (TenantExport, error)`
  — port of `_sync_export_user_data`. Filter: `owner_actor == principal`
  OR `payload[subject_field] == user_id`. Drop nodes whose
  `DataPolicy == AUDIT` (`canonical_store.py:5215-5221`). Preserve
  id-keyed payload.
- `schema.Registry.GetDataPolicy(typeID)`,
  `schema.Registry.GetSubjectField(typeID)` — missing type → fall back
  to `DataPolicy.PERSONAL`, no subject field
  (`canonical_store.py:5204-5208`).
- `metrics.RecordGRPCRequest`.
- `encoding/json` for the bundle (no `Struct`).

## Other-RPC deps (DeleteUser pipeline)

- **`DeleteUser`** (`grpc_server.py:2925-2981`) — paired GDPR pipeline:
  (1) `ExportUserData` returns portability bundle; (2)
  `DeleteUser(grace_days=N)` schedules deletion; (3)
  `CancelUserDeletion` undoes (2) within grace. Go port keeps
  `IsSelfOrAdmin` shared across `Delete/Export/Freeze/CancelUserDeletion`
  — same call sites: `grpc_server.py:2944, 2997, 3028, 3086`.
- `canonical_store.export_user_data_for_tenant` is also exercised
  indirectly by `RevokeAllUserAccess` and `TransferUserContent`. Keep
  the per-tenant collector pure — no WAL appends inside it.
- Go SDK already exposes the call: `sdk/go/entdb/client.go:114-117,
  982-989`, `sdk/go/entdb/admin.go:151`. Port keeps wire shape identical
  so the SDK is unchanged.

## Contract tests pinning behavior

- `tests/python/unit/test_gdpr_engine.py:678-692` — happy: bundle has
  `user_id`, `tenants[0].nodes` length 1, owner-match.
- `tests/python/unit/test_gdpr_engine.py:696-704` — auth: foreign actor
  → `PERMISSION_DENIED`.
- `tests/python/unit/test_gdpr_engine.py:418-448` —
  `_sync_export_user_data` unit cases: owner-match, subject-field
  match, AUDIT-policy exclusion, multi-tenant scoping.
- `tests/python/integration/test_grpc_contract.py:643-648` — happy:
  `actor=ALICE, user_id="alice"` → `success=True`.
- `tests/python/integration/test_grpc_contract.py:649-653` — denied:
  `actor=ALICE, user_id="bob"` → `permission_denied`.
- `sdk/go/entdb/admin_gdpr_test.go:76-94` — SDK passes the JSON string
  through unparsed; do not double-encode.

## Implementation outline

```go
func (s *EntDBServer) ExportUserData(
    ctx context.Context, req *pb.ExportUserDataRequest,
) (*pb.ExportUserDataResponse, error) {
    start := time.Now()
    label := "ok"
    defer func() { metrics.RecordGRPCRequest("ExportUserData", label, time.Since(start)) }()

    if s.globalStore == nil {
        label = "error"
        return nil, status.Error(codes.Unimplemented, "User registry not configured")
    }
    if req.Actor == "" {
        label = "error"
        return nil, status.Error(codes.InvalidArgument, "actor is required")
    }
    if req.UserId == "" {
        label = "error"
        return nil, status.Error(codes.InvalidArgument, "user_id is required")
    }
    trusted := auth.AuthoritativeActor(ctx) // ignore req.Actor for the decision
    if !auth.IsSelfOrAdmin(trusted, req.UserId) {
        label = "error"
        return nil, status.Error(codes.PermissionDenied,
            "ExportUserData requires the user themselves or an admin actor")
    }

    memberships, err := s.globalStore.GetUserTenants(ctx, req.UserId)
    if err != nil { label = "error"; return errResp(err), nil }

    principal := req.UserId
    if !strings.HasPrefix(principal, "user:") { principal = "user:" + principal }

    tenants := make([]canonical.TenantExport, 0, len(memberships))
    for _, m := range memberships {
        data, err := s.canonical.ExportUserData(ctx, m.TenantID, principal, s.schema)
        if err != nil { log.Warn().Str("tenant", m.TenantID).Err(err).Msg("tenant failed"); continue }
        tenants = append(tenants, data)
    }

    raw, err := json.Marshal(map[string]any{
        "user_id": req.UserId, "generated_at": time.Now().Unix(), "tenants": tenants,
    })
    if err != nil { label = "error"; return errResp(err), nil }
    return &pb.ExportUserDataResponse{Success: true, ExportJson: string(raw)}, nil
}
```

## Open questions / risks

1. **Large export — no streaming.** `export_json` is a single `string`.
   A user with 10M nodes across 100 tenants will OOM the server and
   blow the default 4 MB gRPC max-message size. Python has no guard.
   Options: (a) preserve and document; (b) add server-streaming
   `ExportUserDataStream`; (c) write to S3 and return a signed URL.
   Deferred to EPIC #407; **port preserves current shape**.
2. **Encryption-at-rest of the artifact.** No artifact today, so N/A;
   if (1c) is chosen the Go port must encrypt with the tenant KEK
   (`server/python/entdb_server/crypto/`) and set S3 Object Lock
   retention ≥ the WAL retention.
3. **No WAL audit record of the export.** GDPR Article 30 record of
   processing is only in interceptor logs + metrics today. Consider
   appending an `EXPORT_USER_DATA` event to the WAL (audit-policy node
   type) on success — makes the export itself tamper-evidently
   auditable via S3 Object Lock without violating read-only response.
4. **Per-tenant exception swallow** (`grpc_server.py:3048-3053`): a
   corrupted tenant DB silently yields an *incomplete* portability
   bundle — user thinks they have everything. Add `partial: true` /
   `failed_tenants: []`? Behaviour-preserving port keeps silent
   swallow until follow-up issue filed.
5. **`DataPolicy.AUDIT` exclusion** (`canonical_store.py:5219-5221`):
   audit records are **not** subject-access-requestable. Document
   explicitly.
6. **Subject-field match by name vs id.** Python compares
   `payload.get(subject_field)` (`canonical_store.py:5216`);
   `subject_field` is a registry name, but `payload` is id-keyed
   (invariant #6). Latent bug — works only if the registry returns the
   field id as a string. Port faithfully, file follow-up.
7. **No allow-list of exportable types.** All non-AUDIT types walk —
   intentional per Article 20 (export is exhaustive).
