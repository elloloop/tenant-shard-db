# RPC Port Spec — `entdb.v1.EntDBService/ListTenants`

> Implementation: `server/go/internal/api/list_tenants.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407 — Python → Go server port. Source of truth: Go handler at
`server/go/internal/api/list_tenants.go`.

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:82` (rpc), `:611-619` (messages).

`ListTenantsRequest` — **empty message**. No fields.
- No `RequestContext`, **no `actor`**, no `tenant_id`.
- No `filter`, no `page_token`, no `page_size`, no `sort`, no `status` selector.
- The handler is **identity-driven**, not request-driven. Pinned by
  `tests/python/integration/test_privilege_escalation.py:397-401` ("The actor
  field on ListTenantsRequest doesn't even exist in the proto").

`ListTenantsResponse`:
| Field | Tag | Type | Notes |
|------|-----|------|------|
| `tenants` | 1 | `repeated TenantInfo` | Visible tenants for the trusted caller. Order matches `canonical_store.list_tenants()` which returns `sorted(data_dir.glob("tenant_*.db"))` — i.e. **ascending tenant_id**. Go port MUST preserve sort order; no test pins it explicitly but admin tests use `sorted(...)` everywhere. |

`TenantInfo` (proto `:617-619`) — single field `string tenant_id = 1`. No
display name, no created_at, no member count. Adding fields is non-breaking
on the wire but is a contract change for the Go port — **do not add fields**.

**Pagination: none.** All visible tenants returned in a single response.
A node hosts O(tenants_on_node); 1k–10k is the realistic ceiling. If/when
pagination is added it must be a new RPC or a new `ListTenantsV2` request
type — not a silent extension here.

## Auth

Method **is NOT** in `AuthInterceptor.UNAUTHENTICATED_METHODS`
(`auth/auth_interceptor.py:157-184`). The interceptor MUST run first and
populate the trusted-identity ContextVar via `set_current_identity()`.

Handler reads the identity via `get_current_identity()` (`server/go/internal/api/list_tenants.go,
1561`). Visibility classes:

| Trusted identity | Visibility |
|---|---|
| `None` (interceptor missing or auth disabled) | **`PERMISSION_DENIED`**, message `"ListTenants requires an authenticated caller"` (`server/go/internal/api/list_tenants.go`). Do NOT fall open. |
| `"__system__"`, `"system:*"`, `"admin:*"` | All tenants on this node (sharding still applied). `server/go/internal/api/list_tenants.go`. |
| `"user:<id>"` | Intersection of node-local tenants ∩ `global_store.get_user_tenants(<id>)`. Strip `"user:"` prefix before membership lookup (`server/go/internal/api/list_tenants.go`). |
| Any other string (no recognised prefix) | Treated as the bare user_id (no strip). Same membership-filter path. |

**Trusted-actor invariant (CLAUDE.md §"Proto is the type system"):** the
request carries no actor field, so the wire-actor-is-untrusted rule has
nothing to validate. The Go port MUST NOT add a `request.context.actor`
read here — pinned by `test_privilege_escalation.py:386-417` (claimed admin
actors via the metadata-spoofing helper still see only Eve's tenants).

No `Permission` / capability check, no per-tenant ACL — visibility comes
purely from `tenant_members` rows.

## Side effects

**Strictly read-only.** No WAL append, no `canonical_store` write, no
`global_store` write, no quota charge.

In-order narration:
1. `start := time.Now()` for metrics timing.
2. `trusted := getCurrentIdentity(ctx)`. If nil → abort `PERMISSION_DENIED`.
3. `isAdmin := trusted=="__system__" || strings.HasPrefix(trusted,"system:") || strings.HasPrefix(trusted,"admin:")`.
4. `allIDs := canonicalStore.ListTenants()` — directory scan of `data_dir/tenant_*.db` (`server/go/internal/store/`). Synchronous; cheap; do **not** wrap in goroutine.
5. If `sharding != nil && sharding.IsMultiNode()` → keep only `sharding.IsMine(tid)` (`server/go/internal/api/list_tenants.go`). Pinned by `test_listtenants_auth.py:200-230`.
6. If `isAdmin` → `visible = allIDs`.
7. Else if `globalStore != nil` → strip `"user:"` prefix; `memberships, _ := globalStore.GetUserTenants(userID)`; `memberSet := {m.TenantID}`; `visible = filter(allIDs, in memberSet)`.
8. Else (`globalStore == nil`, embedded harness) → `visible = []`. Pinned by `test_listtenants_auth.py:238-258`.
9. `record_grpc_request("ListTenants","ok",elapsed)`.
10. Build `[]*pb.TenantInfo` and return.

No goroutine fan-out. No cache. No memoization (memberships can change
between calls; correctness > 1µs of latency).

## Error contract

| gRPC code | Trigger | Wire path |
|---|---|---|
| `OK` | Happy path (incl. zero results). | Always returns a response, never an error, on success. |
| `PERMISSION_DENIED` | Trusted identity is `nil` (interceptor missing/misconfigured). | `context.abort(...)` re-raised as `grpc.aio.AbortError`. Go: `return nil, status.Errorf(codes.PermissionDenied, ...)`. Pinned by `test_listtenants_auth.py:179-192`, `test_grpc_contract.py:288-295`. |
| `OK` + empty `tenants=[]` | Any **other** unhandled exception inside the handler. | Python `except Exception` swallows and returns empty list (`server/go/internal/api/list_tenants.go`). The Go port MUST replicate this — convert internal errors into `&pb.ListTenantsResponse{}` with `record_grpc_request(...,"error",...)` and `logger.Error(...)`. Returning `codes.Internal` is a **contract break**. |
| `UNAUTHENTICATED` | Surfaced by the auth interceptor BEFORE the handler when metadata is missing/bad. Not raised by the handler itself. | N/A inside this RPC. |

`INVALID_ARGUMENT` is impossible — request is empty. `NOT_FOUND` is not
used; missing tenants → empty list.

## Shared Go package deps

Each is a package under `server/go/internal/...` unless noted.

- `pb` (`server/go/internal/pb/entdbv1`) — generated `ListTenantsRequest`, `ListTenantsResponse`, `TenantInfo`, servicer interface. **Required.**
- `auth` — must export `GetCurrentIdentity(ctx) (string, bool)` populated by the auth interceptor (mirror of `auth_interceptor.py:66-75`). **Required.**
- `canonicalstore` — `ListTenants() []string` (mirror of `server/go/internal/store/`). Directory scan, no SQLite open. **Required.**
- `globalstore` — `GetUserTenants(ctx, userID string) ([]Membership, error)` returning rows with at least `TenantID` (mirror of `server/go/internal/globalstore/`). **Required**, but nil-safe call site (step 8).
- `sharding` — `Config{ IsMultiNode() bool; IsMine(tenantID string) bool }` (mirror of `sharding.py`). Optional; nil-safe.
- `metrics` — `RecordGRPCRequest(method, status string, dur time.Duration)`. **Required.**

NOT used and MUST NOT be imported here: `acl`, `wal`, `apply`, `schema`,
`errs` (no domain errors raised), `quota`, `crypto`, `audit`. Importing
`wal` is the smoke signal that someone added a write path — block at code review.

## Other-RPC deps

- **`GetUserTenants` overlap** (`server/go/internal/api/list_tenants.go`, proto `:126`/`:930-940`):
  same `global_store.get_user_tenants(user_id)` call. The Go port should
  share the underlying `globalstore.GetUserTenants` method — do not duplicate
  the SQL. The two RPCs differ in:
  - `GetUserTenants` takes `user_id` from the request (admin lookup of
    arbitrary user). `ListTenants` derives it from the trusted identity.
  - `GetUserTenants` returns full `MembershipInfo` (role, joined_at).
    `ListTenants` discards everything but `tenant_id`.
- **`Health`** is a sibling read-only RPC but shares no code path.
- **`ListMailboxUsers`** (proto `:621-629`) is unrelated despite the name —
  returns empty stub.

No write-side RPC depends on `ListTenants`. It is a leaf reader; safe to
port in isolation once `auth`, `canonicalstore`, `globalstore`, `sharding`
and `metrics` packages exist.

## Contract tests pinning behavior

- (legacy Python unit test, removed in Phase 4D) — admin identities (`__system__`, `system:admin`, `system:gdpr-worker`, `admin:root`) see all tenants on the node.
- (legacy Python unit test, removed in Phase 4D) — `user:alice` sees only her membership tenants; non-member tenants invisible.
- (legacy Python unit test, removed in Phase 4D) — user with zero memberships → empty list (no enumeration leak).
- (legacy Python unit test, removed in Phase 4D) — `"user:"` prefix MUST be stripped before `get_user_tenants` lookup.
- (legacy Python unit test, removed in Phase 4D) — no trusted identity → `PERMISSION_DENIED`, never falls open.
- (legacy Python unit test, removed in Phase 4D) — sharding filter still applied to admin (`is_mine` excludes non-local tenants).
- (legacy Python unit test, removed in Phase 4D) — `global_store == nil` → empty list for regular user (embedded-harness safety).
- `tests/python/integration/test_privilege_escalation.py:386-417` — claimed admin actor in metadata MUST NOT bypass membership filtering for `user:eve`.
- `tests/python/integration/test_grpc_contract.py:288-295` — over-the-wire: empty request without auth interceptor → `PERMISSION_DENIED`. **The Go server must pass this test verbatim once stubs swap to the Go binary.**

## Implementation outline (Go handler)

```go
// server/go/internal/api/listtenants.go
func (s *EntDBServer) ListTenants(ctx context.Context, _ *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
    start := time.Now()
    status := "ok"
    defer func() { metrics.RecordGRPCRequest("ListTenants", status, time.Since(start)) }()

    trusted, ok := auth.GetCurrentIdentity(ctx)
    if !ok || trusted == "" {
        status = "error"
        return nil, grpcstatus.Errorf(codes.PermissionDenied,
            "ListTenants requires an authenticated caller")
    }

    isAdmin := trusted == "__system__" ||
        strings.HasPrefix(trusted, "system:") ||
        strings.HasPrefix(trusted, "admin:")

    all := s.canonicalStore.ListTenants() // directory scan
    if s.sharding != nil && s.sharding.IsMultiNode() {
        all = filter(all, s.sharding.IsMine)
    }

    var visible []string
    switch {
    case isAdmin:
        visible = all
    case s.globalStore != nil:
        userID := strings.TrimPrefix(trusted, "user:")
        members, err := s.globalStore.GetUserTenants(ctx, userID)
        if err != nil {
            status = "error"
            log.Error("ListTenants membership lookup failed", "err", err)
            return &pb.ListTenantsResponse{}, nil // swallow → empty, matches Python
        }
        set := make(map[string]struct{}, len(members))
        for _, m := range members { set[m.TenantID] = struct{}{} }
        visible = filterIn(all, set)
    default:
        visible = nil // no global_store wired → empty, NOT all
    }

    out := make([]*pb.TenantInfo, 0, len(visible))
    for _, tid := range visible { out = append(out, &pb.TenantInfo{TenantId: tid}) }
    return &pb.ListTenantsResponse{Tenants: out}, nil
}
```

Wrap the entire handler body in a `defer recover()` that converts unexpected
panics into `&pb.ListTenantsResponse{}, nil` with `status="error"` — matches
Python's `except Exception` swallow. PERMISSION_DENIED must escape the
recover (it's the *intended* error path, returned via the explicit
`return` above, not via panic).

## Open questions / risks

- **Empty-list-on-error is a footgun.** Python silently masks bugs (DB
  corruption, `global_store` raises) as "you have no tenants". The Go port
  MUST replicate to keep the contract test green, but file a follow-up to
  surface internal errors as `codes.Internal` behind a feature flag once
  callers can handle it. Today's behavior is pinned at `server/go/internal/api/list_tenants.go`.
- **Identity prefix matching is string-based and fragile.** A future
  identity provider that issues `"sa:..."` (service account) tokens will
  silently fall through to the user-membership branch. Recommend adding
  a typed `Identity{Kind: User|System|Admin|Service; ID string}` in the
  `auth` package and routing on `Kind` rather than `strings.HasPrefix`.
- **`canonicalStore.ListTenants()` is a directory scan on every call.**
  Fine at 1k tenants, allocates at 100k. Not a port-blocker; track as
  perf follow-up. Memoization is unsafe (tenants can be created/deleted
  between calls); use `fsnotify` if it ever matters.
- **Sort order is not contract-tested.** Python returns ascending (sorted
  glob). Go port should explicitly `sort.Strings(visible)` before building
  the proto slice — defends against future map-iteration code drift.
- **No pagination** ships in the wire today. If a customer hits 50k+
  tenants per node, this RPC degrades into a multi-MB response. Not a
  port concern; flag for product.
- **`UNAUTHENTICATED_METHODS` parity:** confirm `/entdb.v1.EntDBService/ListTenants`
  is **absent** from the Go auth interceptor's bypass set. A copy-paste
  regression there silently re-introduces the SEC-3 enumeration bug.
