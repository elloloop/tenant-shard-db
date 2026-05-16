# GetTenantMembers — Go Port Spec

> Implementation: `server/go/internal/api/get_tenant_members.go`. The Python-source citations
> below are historical (Python server was retired in EPIC #407 Phase 4D,
> commit `8d07f5f`). See ADR-016 for the write-path contract this RPC
> follows.

EPIC #407. Read-only listing of every membership row for a tenant. Backed by
the `global_store` SQLite (`tenant_members` table), not the per-tenant
canonical store. Python source of truth:
`server/go/internal/api/get_tenant_members.go`.

## Wire contract (pagination)

Proto: `proto/entdb/v1/entdb.proto:125` (RPC), `:921-928` (request/response),
`:902-907` (`TenantMemberInfo`).

```
rpc GetTenantMembers(GetTenantMembersRequest) returns (GetTenantMembersResponse);

GetTenantMembersRequest  { string actor = 1; string tenant_id = 2; }
GetTenantMembersResponse { repeated TenantMemberInfo members = 1; }
TenantMemberInfo         { string tenant_id = 1; string user_id = 2;
                           string role = 3; int64 joined_at = 4; }
```

**No pagination on the wire.** The proto carries no `limit`, `offset`,
`page_token`, or `has_more` field — the response is the full member list
in a single message. SQL ordering is `ORDER BY joined_at`
(`server/go/internal/globalstore/`). The Go port MUST preserve "single shot, no
pagination" — adding a page token here is a wire-incompatible change
gated by a separate proto issue.

`joined_at` is **epoch milliseconds** (proto field is `int64`,
`global_store` writes `int(time.time()*1000)`). Pinned by Go SDK test
`sdk/go/entdb/admin_test.go:240` (`JoinedAt: 200`).

`actor` is a flat string on the wire (e.g. `"user:alice"`,
`"system:admin"`) — there is no nested `RequestContext` for this RPC,
unlike data-plane RPCs. Empty string is rejected (see Error contract).

## Auth (member-only; trusted-actor)

The current Python handler implements **only** payload-level argument
validation — it does **not** call `_trusted_actor`, `_check_tenant`, or
`_check_cross_tenant_read`, and it has **no entry** in
`auth/capability_registry.py` (verified by grep). In effect, any caller
who reaches the gRPC handler with a non-empty `actor` and `tenant_id`
gets the full member list. Pinned by:

- (legacy Python unit test, removed in Phase 4D) — `actor="user:alice"`
  succeeds and returns all members; the test does NOT pre-grant alice
  any capability.
- `tests/python/integration/test_grpc_contract.py:489-494` — `ALICE`
  (regular user) gets a happy-path response.

For the Go port this means:

1. **Trusted-actor rebinding** still applies at the interceptor layer
   (CLAUDE.md invariant + privilege-escalation fix in commit fece3fb) —
   the AuthInterceptor's authoritative actor MUST override
   `request.actor` before this handler reads it. Even though the Python
   handler reads `request.actor` directly today, the Go port should
   route through the trusted-actor helper for consistency with #168 and
   to close the gap for free.
2. **No member-only gate today.** Do NOT add a `is_member(tenant, actor)`
   check in the Go port without an explicit issue — it would break the
   contract test at `test_grpc_contract.py:491` where `ALICE` is the
   tenant's owner but the test does not assert membership-gating per se,
   just a happy result. (A future tightening is captured under Open
   questions.)
3. **No capability mapping.** `GetTenantMembers` is absent from
   `auth/capability_registry.py`; the Go port should not invent one.

## Side effects (read on global_store)

**None.** Strictly read-only.

- One call to `global_store.get_members(tenant_id)`
  (`server/go/internal/globalstore/`) which executes
  `SELECT * FROM tenant_members WHERE tenant_id = ? ORDER BY joined_at`
  against the **global** SQLite (cross-tenant registry), invariant #4 —
  this is the legitimate cross-tenant path; it does NOT touch any
  per-tenant SQLite.
- No `wal.append`, no SQLite writes, no Applier interaction (read-only,
  so invariant #1 is not in play).
- Metrics: `record_grpc_request("GetTenantMembers", "ok"|"error", dt)`
  at `server/go/internal/api/get_tenant_members.go` and `:2568`.

## Error contract

| Condition                               | Code              | Source line                       |
|-----------------------------------------|-------------------|-----------------------------------|
| `global_store is None`                  | `UNIMPLEMENTED`   | `server/go/internal/api/get_tenant_members.go`        |
| `actor == ""`                           | `INVALID_ARGUMENT`| `server/go/internal/api/get_tenant_members.go`        |
| `tenant_id == ""`                       | `INVALID_ARGUMENT`| `server/go/internal/api/get_tenant_members.go`        |
| Unknown tenant (no rows)                | `OK` + empty list | implicit (SELECT returns 0 rows)  |
| Any unhandled exception in `get_members`| `OK` + empty list | `server/go/internal/api/get_tenant_members.go`        |

The bare-`except` swallow at `:2567-2570` returning an empty `members`
list with `record_grpc_request(..., "error", ...)` is **load-bearing
behavior** that the Go port must replicate to keep
`test_grpc_contract.py:496-499` semantics (note: that contract row only
exercises the `actor=""` branch, which goes through the `await
context.abort(INVALID_ARGUMENT)` path; the silent-empty branch is not
directly contract-pinned but is a Python-side resilience pattern — see
Open questions about whether to keep it).

`INVALID_ARGUMENT` messages are exact strings, ordering matters
(actor-check fires before tenant-check):

- `"actor is required"`
- `"tenant_id is required"`

These should be reproduced verbatim in the Go handler so SDK callers
that string-match the error stay green.

## Shared Go package deps

Reuse only — do not duplicate:

- `internal/auth` — interceptor's authoritative-actor `context.Context`
  helper (the Go equivalent of Python's `get_authoritative_actor`).
- `internal/globalstore` — Go port of `server/go/internal/globalstore/`. Must expose
  `GetMembers(ctx, tenantID) ([]MemberRow, error)` returning rows
  ordered by `joined_at ASC`. The `MemberRow` struct mirrors the four
  columns (`tenant_id`, `user_id`, `role`, `joined_at int64 ms`).
- `internal/protoconv` — a `MemberRowToProto(MemberRow) *pb.TenantMemberInfo`
  helper, factored from the existing Python `_member_dict_to_proto`
  (`server/go/internal/api/get_tenant_members.go`). It is reused by `AddTenantMember`,
  `RemoveTenantMember`, `GetUserTenants`, `ChangeMemberRole`, so it
  belongs in `protoconv`, not in this RPC's file.
- `internal/metrics` — `RecordGRPCRequest(method, status, dur)` matching
  the Python `record_grpc_request` Prometheus labels.
- `pb` — `github.com/elloloop/tenant-shard-db/proto/gen/entdb/v1`.

Do NOT pull in `internal/canonicalstore`, `internal/wal`,
`internal/applier`, or `internal/capabilities` — none are touched by
this RPC.

## Other-RPC deps

None at runtime. The four-column `TenantMemberInfo` proto and the
`global_store.tenant_members` schema are shared with:

- `AddTenantMember` / `RemoveTenantMember` (`server/go/internal/api/get_tenant_members.go`)
- `GetUserTenants` (`server/go/internal/api/get_tenant_members.go`)
- `ChangeMemberRole` (`server/go/internal/api/get_tenant_members.go+`)

If the Go port lands `GetTenantMembers` first, the proto-conversion
helper and the `globalstore.GetMembers` accessor should be merged in
the same PR so the four siblings can land incrementally without churn.

## Contract tests pinning behavior (file:line)

- (legacy Python unit test, removed in Phase 4D) — happy path
  returns 2 members, both `user_id` values exposed, no auth gate
  required for `user:alice`.
- `tests/python/integration/test_grpc_contract.py:489-494` — happy path
  through the wire (`ALICE` actor); response contains `user_id=="alice"`.
- `tests/python/integration/test_grpc_contract.py:495-499` — empty
  `actor` → `INVALID_ARGUMENT`.
- `sdk/go/entdb/admin_test.go:235-260` — Go SDK round-trip against the
  fake server: `members[0].UserID=="alice"`, `members[0].Role=="admin"`,
  `members[1].JoinedAt==200` (epoch ms preserved).
- `sdk/go/entdb/grpc_transport_test.go:108-109,367` — fake-server
  fixture wiring for the gRPC transport.

## Implementation outline

```go
func (s *Server) GetTenantMembers(
    ctx context.Context, req *pb.GetTenantMembersRequest,
) (*pb.GetTenantMembersResponse, error) {
    start := time.Now()
    defer func() {
        // status set below; see record(...)
    }()
    record := func(status string) {
        metrics.RecordGRPCRequest("GetTenantMembers", status, time.Since(start))
    }

    if s.globalStore == nil {
        record("error")
        return nil, status.Error(codes.Unimplemented, "Tenant registry not configured")
    }
    if req.GetActor() == "" {
        record("error")
        return nil, status.Error(codes.InvalidArgument, "actor is required")
    }
    if req.GetTenantId() == "" {
        record("error")
        return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
    }

    rows, err := s.globalStore.GetMembers(ctx, req.GetTenantId())
    if err != nil {
        record("error")
        s.log.Error("GetTenantMembers failed", "err", err)
        // Python-parity silent-empty: see Open questions.
        return &pb.GetTenantMembersResponse{}, nil
    }

    out := make([]*pb.TenantMemberInfo, 0, len(rows))
    for _, r := range rows {
        out = append(out, protoconv.MemberRowToProto(r))
    }
    record("ok")
    return &pb.GetTenantMembersResponse{Members: out}, nil
}
```

Notes:
- Use `codes.Unimplemented` / `codes.InvalidArgument` from
  `google.golang.org/grpc/codes` to match Python's `grpc.StatusCode`.
- Do NOT call any per-tenant SQLite handle (invariant #4).
- Do NOT append to the WAL (read-only; invariant #1 not triggered).
- Streaming: keep unary; no streaming variant exists in proto.

## Open questions / risks

1. **Silent-empty on internal error.** `server/go/internal/api/get_tenant_members.go`
   swallows every exception and returns an empty list with status `OK`.
   This is hostile to debugging (the SDK cannot distinguish "no
   members" from "DB blew up"). Recommendation: in the Go port, return
   `codes.Internal` instead and write an ADR — but flip the contract
   test at `test_grpc_contract.py:489-494` only after EPIC #407 review.
2. **No auth gate.** As noted in §Auth, the Python handler is
   effectively unauthenticated beyond non-empty argument checks. A
   `is_member(tenant, _trusted_actor)` gate is the obvious fix and
   would mirror `ChangeMemberRole` (`test_tenant_registry.py:585-632`).
   Out of scope for the lift-and-shift port; file as a follow-up.
3. **Pagination.** A 100k-member tenant returns a single message
   bounded only by gRPC's max message size (4MB default). Add `limit`
   + `page_token` in proto v2 — track separately, do NOT bolt on in
   this port.
4. **Ordering stability.** SQL `ORDER BY joined_at` is not unique
   (two members joining in the same ms tie). The Go port should add
   `, user_id` as a deterministic tiebreaker; harmless because no
   client pins exact order across ties.
5. **`tenant_id` validation.** Empty string is rejected, but no format
   validation (e.g. `^[a-z0-9_-]+$`). Matches Python; do not tighten
   without a proto issue.
