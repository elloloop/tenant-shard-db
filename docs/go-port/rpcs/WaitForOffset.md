# WaitForOffset — Go Port Spec (EPIC #407)

`entdb.v1.EntDBService/WaitForOffset` provides read-after-write consistency: a
client passes a WAL stream position from a prior `Receipt` and blocks until the
applier on this node has materialized at least that position into per-tenant
SQLite (or until timeout).

## Wire contract

Proto: `proto/entdb/v1/entdb.proto:88` (RPC), `:694-703` (messages).

```
rpc WaitForOffset(WaitForOffsetRequest) returns (WaitForOffsetResponse);

message WaitForOffsetRequest {
    RequestContext context = 1;     // tenant_id, actor (untrusted), trace_id
    string stream_position = 2;     // "topic:partition:offset", e.g. "entdb-wal:0:42"
    int32 timeout_ms = 3;           // 0 ⇒ server default 30s (Python: grpc_server.py:982)
}
message WaitForOffsetResponse {
    bool reached = 1;               // true iff applied_offset >= stream_position
    string current_position = 2;    // applier's latest applied offset (may be "")
}
```

Unary, idempotent, no side effects. Stream position format is opaque to the
wire but parsed by `_parse_stream_offset` (`canonical_store.py:90`): split on
last `:` and parse trailing int; non-numeric ⇒ 0. Therefore an empty
`stream_position` parses to 0 and is "already reached" once anything has been
applied — but if nothing has been applied for the tenant yet,
`get_applied_offset` returns `None` and the call blocks until timeout (see
contract test note at `tests/python/integration/test_grpc_contract.py:296`).

## Auth

- `request.context.tenant_id` is checked via `_check_tenant`
  (`grpc_server.py:362-407`): sharding ownership (UNAVAILABLE +
  `entdb-redirect-node` trailer) and region pinning (FAILED_PRECONDITION).
- `request.context.actor` is **UNTRUSTED on the wire** (per CLAUDE.md
  invariant). Note: the current Python handler does not consult the actor at
  all — there is no per-actor authorization for waiting on an offset. Do not
  add one in Go without an explicit decision; just match Python.
- No `_trusted_actor()` translation needed (the handler doesn't read actor).
- No additional ACL checks. This is a side-effect-free synchronization
  primitive scoped by tenant.

## Side effects

None on storage. Read-only with respect to WAL and SQLite. **Blocks the
calling RPC** on `CanonicalStore.wait_for_offset`
(`canonical_store.py:478-512`), which is event-driven (no polling):

1. Per-tenant `asyncio.Condition` lazily created in `_offset_conditions`
   (`:455-461`).
2. Predicate reads `_applied_offsets[tenant_id]` and returns
   `_compare_stream_pos(current, target) >= 0` (`:499-503`).
3. The applier (`apply/applier.py:447`, `:599`) calls
   `update_applied_offset` after each committed batch, which `notify_all()`s
   the Condition (`canonical_store.py:463-476`).
4. Waiter sleeps inside `cond.wait_for(...)` wrapped in `asyncio.wait_for`.

`current_position` is read AFTER the wait via `get_applied_offset`
(`grpc_server.py:986`); `""` if applier has never updated this tenant.

## Error contract

Python handler swallows all exceptions and returns
`WaitForOffsetResponse(reached=False, current_position="")` with `OK` status
(`grpc_server.py:989-992`). Go MUST mirror this swallow-and-return-false
behavior **except** for aborts raised by `_check_tenant`, which emit gRPC
status codes via `context.abort()` and bypass the try/except:

| Condition | Status | Notes |
|-----------|--------|-------|
| Tenant not owned by node + known owner | `UNAVAILABLE` | trailer `entdb-redirect-node: <owner>`; SDK retry hint (see `sdk/go/entdb/redirect_cache.go`). |
| Tenant not owned by node + unknown owner | `UNAVAILABLE` | no trailer. |
| Tenant region-pinned to other region | `FAILED_PRECONDITION` | permanent; do not retry. |
| Target offset reached within timeout | `OK`, `reached=true` | |
| Timeout elapsed before reached | `OK`, `reached=false`, `current_position=<latest or "">` | **Not** `DEADLINE_EXCEEDED`. The deadline is server-internal (driven by `timeout_ms`). |
| Internal error during wait | `OK`, `reached=false`, `current_position=""` | logged; metric `WaitForOffset/error`. |
| Client-side gRPC deadline expires before timeout | `DEADLINE_EXCEEDED` | grpc-go cancels the stream itself; Go handler should respect `ctx.Done()` and unblock the waiter. |

Note: client-side cancellation via `ctx.Done()` is NOT modeled by the Python
handler explicitly, but `asyncio.wait_for` will unwind on RPC cancellation.
Go must wire `ctx` into the wait so cancellation is honored without leaking
goroutines.

## Shared Go package deps

Suggested layout under `server/go/`:

- `internal/applystate` — owns `AppliedOffsets`: per-tenant
  `map[tenantID]string` + per-tenant condition variable. Mirror Python's
  `_applied_offsets` + `_offset_conditions` (`canonical_store.py:429-461`).
- `internal/streampos` — `Parse(s string) int64` and `Compare(a, b string)
  int` mirroring `_parse_stream_offset` / `_compare_stream_pos`
  (`canonical_store.py:90-109`). Pin to identical edge cases (split on last
  `:`, non-numeric → 0).
- `internal/sharding` — `IsMine(tenantID)` / `GetOwner(tenantID)` for the
  redirect logic in `_check_tenant`.
- `internal/region` — region pinning lookup against `global_store`.
- `internal/metrics` — `recordGRPCRequest("WaitForOffset", "ok"|"error",
  duration)` mirroring `record_grpc_request` calls at
  `grpc_server.py:987,990`.

## Other-RPC deps

- `ExecuteAtomic` returns `Receipt.stream_position`
  (`proto/entdb/v1/entdb.proto`, message `Receipt`, field 3) — clients feed
  that exact string back here. Go port of `ExecuteAtomic` MUST preserve the
  `topic:partition:offset` format produced by the WAL backend.
- The Go `Applier` (port of `apply/applier.py`) MUST call
  `applystate.UpdateAppliedOffset(tenantID, pos)` after every committed batch
  (mirror `applier.py:447,599`). Without this, all `WaitForOffset` calls
  time out.
- Read RPCs that accept `after_offset` (`GetNode`, `GetNodes`, `QueryNodes`,
  `GetNodeByKey`) call the same `wait_for_offset` helper internally
  (`grpc_server.py:806,1018,1238,1316`). Share the implementation.

## Contract tests pinning behavior

Behavior pinned by tests (Go port MUST keep these green via cross-language
contract harness once it exists, and the Go SDK transport tests already
exist):

- `tests/python/unit/test_wait_for_offset.py:25-52` — `_parse_stream_offset`
  and `_compare_stream_pos` semantics (full format, plain int,
  `partition:offset`, non-numeric → 0).
- `tests/python/unit/test_wait_for_offset.py:65-84` — per-tenant offset
  isolation; `get_applied_offset` returns `None` initially.
- `tests/python/unit/test_wait_for_offset.py:97-114` — already-reached and
  exactly-reached return immediately True; never-set returns False on
  timeout.
- `tests/python/unit/test_wait_for_offset.py:117-127` — waiter wakes on
  applier `update_applied_offset` (event-driven, no polling).
- `tests/python/unit/test_wait_for_offset.py:130-146` — multiple concurrent
  waiters at different positions all resolve from a single applier update.
- `tests/python/unit/test_wait_for_offset.py:149-163` — incremental progress
  wakes waiter exactly when target is crossed.
- `tests/python/unit/test_wait_for_offset.py:166-176` — applier sets a
  position below target → False on timeout, response carries the
  below-target current position.
- `tests/python/integration/test_grpc_contract.py:296-311` — gRPC-level
  contract: `stream_position="entdb-wal:0:0"` with `timeout_ms=500` returns
  a well-formed response (no abort) regardless of `reached`.
- `sdk/go/entdb/transport_extras_test.go:80-125` — Go SDK already exercises
  the wire contract; server port must satisfy these client expectations.

## Implementation outline (Go handler)

grpc-go handlers run on a goroutine per RPC. Blocking is acceptable, but
avoid busy loops and goroutine leaks on cancel. `sync.Cond` does not
compose with `ctx.Done()`; use a per-tenant broadcast channel instead.

```go
// internal/applystate/state.go
type State struct {
    mu     sync.Mutex
    pos    map[string]string         // tenantID -> applied stream position
    bcast  map[string]chan struct{}  // tenantID -> closed-on-update channel
}

func (s *State) Update(tenant, pos string) {
    s.mu.Lock()
    s.pos[tenant] = pos
    ch := s.bcast[tenant]
    s.bcast[tenant] = make(chan struct{})
    s.mu.Unlock()
    if ch != nil { close(ch) }
}

func (s *State) WaitFor(ctx context.Context, tenant, target string, timeout time.Duration) (reached bool, current string) {
    deadline := time.NewTimer(timeout)
    defer deadline.Stop()
    for {
        s.mu.Lock()
        cur := s.pos[tenant]
        if _, ok := s.pos[tenant]; ok && streampos.Compare(cur, target) >= 0 {
            s.mu.Unlock()
            return true, cur
        }
        ch := s.bcast[tenant]
        if ch == nil { ch = make(chan struct{}); s.bcast[tenant] = ch }
        s.mu.Unlock()
        select {
        case <-ch:           // applier updated; loop and re-check
        case <-deadline.C:   // server-side timeout_ms
            return false, s.snapshot(tenant)
        case <-ctx.Done():   // client cancel / grpc deadline
            return false, s.snapshot(tenant)
        }
    }
}
```

Handler skeleton:

```go
func (s *Server) WaitForOffset(ctx context.Context, req *pb.WaitForOffsetRequest) (*pb.WaitForOffsetResponse, error) {
    start := time.Now()
    if err := s.checkTenant(ctx, req.GetContext().GetTenantId()); err != nil {
        // Aborts (UNAVAILABLE / FAILED_PRECONDITION) returned as status errors,
        // matching context.abort() in Python. Trailer set via grpc.SetTrailer.
        return nil, err
    }
    timeout := 30 * time.Second
    if req.GetTimeoutMs() > 0 { timeout = time.Duration(req.GetTimeoutMs()) * time.Millisecond }
    reached, current := s.applyState.WaitFor(ctx, req.GetContext().GetTenantId(), req.GetStreamPosition(), timeout)
    metrics.RecordGRPC("WaitForOffset", "ok", time.Since(start))
    return &pb.WaitForOffsetResponse{Reached: reached, CurrentPosition: current}, nil
}
```

Notes:
- One waiter goroutine per call; parks on `select` (no CPU burn).
- `Update` is O(1) and broadcasts via `close(ch)` — identical wakeup
  semantics to Python's `cond.notify_all()`.
- No `time.Sleep` polling. No worker pools.
- `current_position` is read after timeout, matching Python at
  `grpc_server.py:986`.
- Client cancel: Python swallows and returns OK+reached=false. Go should
  return `ctx.Err()` (Canceled/DeadlineExceeded) — see open question 4.

## Open questions / risks

1. **Empty `stream_position`**: parses to 0. If applier has applied
   anything, returns immediately; otherwise blocks until timeout (footgun
   noted at `test_grpc_contract.py:296`). Match Python; do not validate.
2. **Topic/partition ignored**: `_compare_stream_pos` only compares the
   trailing int. `"other-topic:9:5"` ≡ `"entdb-wal:0:5"`. Preserve quirk
   for parity; flag as follow-up.
3. **Unbounded per-tenant condition map** (`canonical_store.py:457-461`).
   Same trade-off in Go; consider LRU eviction later.
4. **Client cancel semantics**: Python returns OK+reached=false on cancel
   (broad `except`). Go idiom is `ctx.Err()`. Pick and document — SDK
   tests only cover the timeout path
   (`sdk/go/entdb/transport_extras_test.go:106`).
5. **Server-side timeout never yields `DEADLINE_EXCEEDED`** — it is
   encoded in `reached=false`. Only client-side gRPC deadline does.
6. **Actor is unused** by the handler. Confirm via EPIC #407 whether to
   add a tenant-membership check; today any caller past `_check_tenant`
   can wait on any tenant.
7. **Region/sharding lookups** in `_check_tenant` hit `global_store` and
   `_sharding`. Cache hot path in Go (sync.Map / TTL).
