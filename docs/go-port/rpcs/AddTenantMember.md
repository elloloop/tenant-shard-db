# RPC Port Spec — `entdb.v1.EntDBService/AddTenantMember`

EPIC #407 — Python → Go server port. Source of truth: Python handler at
`server/python/entdb_server/api/grpc_server.py:2442-2490`.

## Wire contract (role enum)

Proto: `proto/entdb/v1/entdb.proto:123` (rpc), `:909-919` (messages).

`TenantMemberRequest`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `actor` | 1 | `string` | Wire-claimed actor, e.g. `"user:alice"` / `"system:admin"`. **UNTRUSTED** — replaced by interceptor identity (see Auth). |
| `tenant_id` | 2 | `string` | Required; non-empty (`grpc_server.py:2458-2459`). |
| `user_id` | 3 | `string` | Required; non-empty. The user being added (`grpc_server.py:2460-2461`). |
| `role` | 4 | `string` | Optional. Defaults to `"member"` when empty (`grpc_server.py:2476`). |

`TenantMemberResponse`: `bool success = 1; string error = 2;` Aborts MUST set
gRPC status; happy returns `success=true`. Soft failures (duplicate) return
`success=false, error=<msg>` with gRPC `OK` (`grpc_server.py:2484-2486`).

**Role is a free-form string, NOT a proto enum.** Recognised values per
authorization checks: `"owner"`, `"admin"`, `"member"`. The server does not
validate `role` against a closed set on insert — `add_member` writes whatever
string the caller supplies. The Go port MUST preserve this leniency to keep
contract tests green; harden via a separate ticket if desired (track as Open
question below). Storage column: `tenant_members.role TEXT NOT NULL DEFAULT
'member'` (`global_store.py:220-226`).

## Auth (tenant admin only; trusted-actor)

- **Authentication**: required. NOT in `AuthInterceptor.UNAUTHENTICATED_METHODS`.
- **Trusted-actor invariant** (CLAUDE.md §"trusted-actor"): handler immediately
  rebinds `trusted_actor = self._trusted_actor(request.actor)` on entry
  (`grpc_server.py:2466`). Every downstream check uses `trusted_actor`, never
  `request.actor`. Go port MUST mirror this — direct use of `req.Actor` for an
  authorization branch is a bug.
- **Authorization**: caller is allowed iff EITHER
  - `_is_admin_or_system(trusted_actor)` returns true (actor prefix
    `system:` / `admin:` / equal to `__system__`, see
    `grpc_server.py:2053-2069`), OR
  - the caller's `tenant_members.role` for `request.tenant_id` is `"owner"` or
    `"admin"` (`grpc_server.py:2468-2474`).
- **Wire-actor escalation must fail**: regression pinned by commit
  `fece3fb` ("Fix privilege escalation: ignore client-claimed actor"). A user
  sending `actor="system:admin"` with their own bearer token MUST be denied.
- **No `Permission` enum lookup, no `acl.check()`** — membership is global
  state, not per-tenant ACL.
- **Rate-limiter**: subject to default per-tenant bucket (no special bypass).

## Side effects (WAL append; global_store membership table; mailbox notification?)

**WAL append: NONE.** Tenant-membership operations are global-store admin ops
that do NOT flow through the per-tenant Kafka/Kinesis WAL. Confirmed: no
`wal.append`, no `TransactionEvent.ops` entry for membership; `wal/` and
`apply/` packages contain zero references to `add_member` or
`AddTenantMember`. This is a deliberate exception to the CLAUDE.md "all writes
go through the WAL" invariant — see `global_store.py` schema comment at
`:43-220` documenting `tenant_members` as a non-event-sourced control plane
table. **Go port MUST NOT add a WAL event for parity**: doing so would change
rebuild semantics and break the contract tests.

**Direct write to `global_store.tenant_members`** (the only side effect):
1. `INSERT INTO tenant_members (tenant_id, user_id, role, joined_at) VALUES
   (?, ?, ?, ?)` with `joined_at = now()` (`global_store.py:571-580`).
2. PRIMARY KEY `(tenant_id, user_id)` enforces uniqueness; second insert
   raises `sqlite3.IntegrityError`.

**Mailbox / notification: NONE.** No fanout, no `notifications` table write,
no `mailbox` write (legacy mailbox is removed; see
`grpc_server.py:1461-1465`). The added member is silent — discovery is via
`GetUserTenants`. If product wants a "you were added" notification, file a
new ticket; Go port MUST stay silent for parity.

**Metric**: `record_grpc_request("AddTenantMember", "ok"|"error", elapsed)`
on every exit branch (`grpc_server.py:2479,2483,2488`).

## Error contract (duplicate; unknown user)

| gRPC code | Trigger | Source |
|-----------|---------|--------|
| `UNIMPLEMENTED` | `global_store` not configured (tenant registry disabled). | `grpc_server.py:2450-2454` |
| `INVALID_ARGUMENT` | `actor`, `tenant_id`, or `user_id` empty. | `:2456-2461` |
| `PERMISSION_DENIED` | Caller is not system/admin AND not `owner`/`admin` member of `tenant_id`. Includes the case where the caller has no membership at all (`_get_member_role` returns `None`). | `:2468-2474` |
| `OK` + `success=false, error="Member already exists in this tenant"` | UNIQUE constraint violation on `(tenant_id, user_id)`. Soft failure, NOT a gRPC error. | `:2482-2487` |
| `OK` + `success=false, error=<exception str>` | Any other exception. | `:2488-2490` |
| `OK` + `success=true` | Insert succeeded. | `:2480` |

**Unknown `user_id`**: NOT validated. There is no `users` table FK — the
handler will happily insert a `tenant_members` row for a user that has never
authenticated. This matches the "user identity is opaque to the registry"
design. Go port MUST preserve this; pinned implicitly by
`test_grpc_contract.py:512-518` (adds `"charlie"` with no prior `CreateUser`).

**Unknown `tenant_id`**: NOT validated either — `tenant_members` has no FK
to `tenant_registry`. Insert succeeds even if the tenant was never created.
Same reason: keep parity, file a hardening ticket separately.

## Shared Go package deps

- `pb` (`server/go/internal/pb/entdbv1`) — `TenantMemberRequest`,
  `TenantMemberResponse`. Required.
- `globalstore` (new, mirrors `global_store.py`) — `AddMember(ctx, tenantID,
  userID, role string) error`, `GetMembers(ctx, tenantID) ([]Member, error)`.
  Required. Must surface unique-constraint violations as a sentinel error
  (e.g. `globalstore.ErrMemberExists`) so the handler can map to the
  soft-failure path.
- `auth` — `TrustedActor(ctx, wireActor string) string` and
  `IsAdminOrSystem(trusted string) bool`. Required. Must read the
  `ContextVar`-equivalent set by the auth interceptor (Go port: `context.Value`
  keyed by an unexported type).
- `metrics` — `RecordGRPCRequest(method, status string, dur time.Duration)`.
  Required.
- `errs` — helpers for `status.Errorf(codes.X, ...)` with consistent message
  shape across handlers.

NOT used and MUST NOT be imported here: `wal`, `apply`, `canonicalstore`,
`schema`, `acl`, `quota`, `crypto`, `audit`. Importing any signals a bug.

## Other-RPC deps (RemoveTenantMember, ChangeMemberRole)

Three handlers form one cohesive unit and SHOULD be ported in a single PR:

- `RemoveTenantMember` (`grpc_server.py:2492-2541`) — same auth model with
  added "last-owner cannot leave" guard (`:2527-2532`). Reuses
  `globalstore.GetMembers` + `RemoveMember`.
- `ChangeMemberRole` (`grpc_server.py:2601+`) — same auth model, calls
  `globalstore.ChangeRole` (`global_store.py:630-644`). Same role-string
  leniency.
- `GetTenantMembers` / `GetUserTenants` — read-only siblings; auth model is
  weaker (any member can read). Out of scope for this spec.
- `CreateTenant` — typically the FIRST writer, since it implicitly seeds the
  caller as `owner` (verify in handler before porting). `AddTenantMember`
  cannot bootstrap a tenant: the first member must come from `CreateTenant`
  or a `system:` actor.

Shared helper `_get_member_role(tenant_id, user_id) -> str | None`
(`grpc_server.py:2290-2296`) lives on the servicer and is reused by all three
write handlers — port once into `globalstore` (`MemberRole(ctx, tenantID,
userID) (string, bool, error)`).

## Contract tests pinning behavior (file:line)

- `tests/python/integration/test_grpc_contract.py:511-518` — happy path:
  `actor=ALICE` (owner), `user_id="charlie"`, `role="member"` → success.
- `tests/python/integration/test_grpc_contract.py:519-523` — `actor=""` →
  `INVALID_ARGUMENT`.
- `tests/python/integration/test_grpc_contract.py:524-530` — `actor=BOB`
  (regular member) → `PERMISSION_DENIED`.
- `tests/python/unit/test_tenant_registry.py:404-419` — owner can add member;
  resulting `get_members` returns 2 rows.
- `tests/python/unit/test_tenant_registry.py:421-434` — admin can add member
  (default role `"member"` when empty).
- `tests/python/unit/test_tenant_registry.py:436-449` — regular member is
  denied with `PERMISSION_DENIED`.
- `sdk/go/entdb/admin_test.go:188-197` — Go SDK happy-path against fake
  server; the real Go handler must satisfy the same wire shape.
- Privilege-escalation regression: commit `fece3fb` — wire `actor="system:admin"`
  from a non-admin caller MUST be ignored. Add an explicit Go test mirroring
  this; the Python suite covers it via `_trusted_actor` unit tests in
  `test_enhanced_auth.py`.

The Go server MUST pass `test_grpc_contract.py` verbatim once the Python
stubs are swapped out (CLAUDE.md release flow).

## Implementation outline

```go
// server/go/internal/api/tenant_members.go
func (s *EntDBServer) AddTenantMember(ctx context.Context, req *pb.TenantMemberRequest) (*pb.TenantMemberResponse, error) {
    start := time.Now()
    var status = "ok"
    defer func() { metrics.RecordGRPCRequest("AddTenantMember", status, time.Since(start)) }()

    if s.globalStore == nil {
        status = "error"
        return nil, grpcstatus.Error(codes.Unimplemented, "Tenant registry not configured")
    }
    if req.GetActor() == "" { status = "error"; return nil, grpcstatus.Error(codes.InvalidArgument, "actor is required") }
    if req.GetTenantId() == "" { status = "error"; return nil, grpcstatus.Error(codes.InvalidArgument, "tenant_id is required") }
    if req.GetUserId() == "" { status = "error"; return nil, grpcstatus.Error(codes.InvalidArgument, "user_id is required") }

    trusted := auth.TrustedActor(ctx, req.GetActor())   // ignore wire actor
    if !auth.IsAdminOrSystem(trusted) {
        role, _, err := s.globalStore.MemberRole(ctx, req.GetTenantId(), auth.ActorUserID(trusted))
        if err != nil { status = "error"; return nil, grpcstatus.Error(codes.Internal, err.Error()) }
        if role != "owner" && role != "admin" {
            status = "error"
            return nil, grpcstatus.Error(codes.PermissionDenied, "Only owner or admin can add members")
        }
    }

    role := req.GetRole(); if role == "" { role = "member" }
    if err := s.globalStore.AddMember(ctx, req.GetTenantId(), req.GetUserId(), role); err != nil {
        status = "error"
        if errors.Is(err, globalstore.ErrMemberExists) {
            return &pb.TenantMemberResponse{Success: false, Error: "Member already exists in this tenant"}, nil
        }
        return &pb.TenantMemberResponse{Success: false, Error: err.Error()}, nil
    }
    return &pb.TenantMemberResponse{Success: true}, nil
}
```

Notes for the implementer:
1. Trusted-actor rebind happens BEFORE any branch that consults identity.
2. `_get_member_role` is `O(N)` over members in Python; Go port should use a
   direct `SELECT role FROM tenant_members WHERE tenant_id=? AND user_id=?`
   for O(1).
3. Soft-failure (duplicate) returns `nil` error + `success=false`. Do NOT
   return a gRPC error code for this case — contract test pinned.
4. Metric label cardinality: fixed `{ok, error}`; safe.

## Open questions / risks

- **Role is free-form string**, not a proto enum. A typo (`"Admin"` vs
  `"admin"`) silently lands a non-privileged role. Recommend filing follow-up
  to introduce `enum TenantRole` in proto, with backward-compat string
  fallback for one release. Out of scope for the port.
- **No FK validation on `user_id` / `tenant_id`** lets the handler create
  orphan rows. Preserve for parity; harden in a separate ticket.
- **WAL bypass is intentional but undocumented in the proto.** When porting,
  add a comment in the Go handler citing CLAUDE.md §1 and explaining the
  global-store control-plane exception, so future readers don't "fix" it by
  adding a WAL append.
- **Last-admin guard is missing on this RPC** (only `RemoveTenantMember`
  guards last-owner). An admin can demote themselves indirectly via
  `ChangeMemberRole` — but `AddTenantMember` cannot lock anyone out. No
  action needed; flag for the ChangeMemberRole spec.
- **Idempotency**: callers retrying on a transient network error will see
  `success=false, error="Member already exists in this tenant"` on the
  second attempt. Document in the Go SDK that this is the expected
  idempotent-replay signal and SDK helpers MAY treat it as success.
- **Concurrency**: two concurrent `AddTenantMember` calls for the same
  `(tenant_id, user_id)` race on the SQLite write. PRIMARY KEY ensures only
  one wins; the loser sees the duplicate path. Go port using `database/sql`
  must surface the unique-constraint error as `globalstore.ErrMemberExists`
  on every supported driver (sqlite3 returns `SQLITE_CONSTRAINT_PRIMARYKEY`).
