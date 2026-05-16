# Error / gRPC Status Code Mapping (Go Port)

EPIC: #407 (Python -> Go server port). The Go server MUST emit byte-for-byte
identical `(grpc.StatusCode, message-prefix, trailers)` triples for every
error path observable from `tests/python/integration/test_grpc_contract.py`,
`tests/python/integration/test_privilege_escalation.py`, and
`(legacy Python unit test, removed in Phase 4D)`.

This is the canonical source for the mapping. Per-RPC specs in
`docs/go-port/rpcs/*.md` reference this file for code semantics and only
list the per-RPC trigger conditions.

## Code catalogue

Every status code emitted by the Python server today, with where it
fires. RPC names are the proto methods on `entdb.v1.EntDBService`.

| Code | Trigger | Where (file:line) | RPCs |
|---|---|---|---|
| `OK` | Success path; also the fallback when broad `except Exception` swallows handler-internal failures and returns an empty/default response | `api/grpc_server.py:1600`, `:1603`, `:826` (ExecuteAtomic outer), and many handler `except Exception` blocks (see Swallow Patterns) | most happy paths; ListTenants, ExecuteAtomic outer, schema/legacy paths |
| `INVALID_ARGUMENT` | Missing required field on the request (`tenant_id`, `actor`, `user_id`, `email`, `name`, `from_user`, `to_user`, `new_role`, `query`, empty `operations`); also surfaced by `QueryFilterError` at `api/grpc_server.py:1154-1155` | `api/grpc_server.py:633,635,637,1151,2114,2121,2123,2125,2164,2166,2199,2201,2246,2319,2321,2323,2377,2379,2412,2414,2457,2459,2461,2507,2509,2511,2558,2560,2587,2589,2616,2618,2620,2622,2675,2702,2704,2706,2757,2759,2761,2822,2874,2876,2941,2943,2994,2996,3025,3027,3083,3085,3128` | every write/admin RPC with required args |
| `NOT_FOUND` | Tenant does not exist on this node, OR the request actor is not a member of the tenant (treated as "not found" to avoid existence leak) | `api/grpc_server.py:505-516` (`_check_tenant`) | every RPC that calls `_check_tenant` |
| `ALREADY_EXISTS` | Idempotency key replay with conflicting payload; unique constraint or composite-unique constraint violation during apply | `api/grpc_server.py:697,746` (raised from `IdempotencyViolationError` / `UniqueConstraintError` / `CompositeUniqueConstraintError`) | `ExecuteAtomic` |
| `PERMISSION_DENIED` | Caller is not admin/owner; cross-tenant read denied; ACL `AccessDeniedError`; client-claimed actor differs from authenticated identity (privilege escalation guard) | `api/grpc_server.py:535,542,553,615-616,1042-1043,1563-1564,2117,2204,2425,2472,2633,2680,2687,2946,2999,3030,3088` | `GetNode`, `ExecuteAtomic`, admin RPCs (`CreateUser`, `UpdateUser`, `DeleteUser`, `AddTenantMember`, `RemoveTenantMember`, `ChangeMemberRole`, `TransferOwnership`, `TransferUserContent`, `DelegateAccess`, `ExportUserData`, `EraseUser`, `RevokeAccess`, etc.) |
| `FAILED_PRECONDITION` | Tenant pinned to a region this node does not serve (permanent, must NOT be retried); also some compatibility/state-machine guards | `api/grpc_server.py:520,526,406` | every RPC behind `_check_tenant` when region-pinned |
| `RESOURCE_EXHAUSTED` | Per-tenant rate-limit bucket empty; quota interceptor (ops/sec, monthly cap); transport-layer cap exceeded (`grpc.max_receive_message_length`, default 4 MiB on a stock gRPC server, 50 MiB on configured server -- see `api/grpc_server.py:3265-3266`) | `api/rate_limiter.py:94`, `auth/quota_interceptor.py:237`, gRPC core (no Python line) | any RPC; default-config server returns it for oversized payloads |
| `UNAUTHENTICATED` | Missing / malformed / expired bearer token; failed JWT or session validation; API-key validation failure | `auth/auth_interceptor.py:196,202`, `api/auth.py:81,131,139,152,187` | every RPC outside `UNAUTHENTICATED_METHODS` (the bypass set is `Health`, JWT login flows) |
| `UNAVAILABLE` | Tenant is not owned by this node (sharding mismatch). Trailers carry `entdb-redirect-node` so SDK redirect cache can update | `api/grpc_server.py:392-395` | every RPC that calls `_check_tenant` |
| `UNIMPLEMENTED` | Optional dependency not configured (no `global_store`, no user registry, no compliance/audit subsystem); also the proto-generated default for any RPC the servicer does not override (`api/generated/entdb_pb2_grpc.py:268-554`) | `api/grpc_server.py:2108,2158,2193,2240,2313,2371,2406,2451,2501,2552,2581,2610,2817,2939,2992,3023,3081,3123` | `CreateTenant`, `ArchiveTenant`, `AddTenantMember`, `RemoveTenantMember`, `ChangeMemberRole`, `GetTenant`, `GetTenantMembers`, `GetUser*`, `*UserContent`, legal-hold, etc., when the corresponding store is `None` |
| `INTERNAL` | Currently NOT raised as a gRPC code by handlers. Returned as a string field `error_code = "INTERNAL"` inside `ExecuteAtomicResponse` on inner-handler exceptions (the RPC itself stays `OK`) | `api/grpc_server.py:824` | `ExecuteAtomic` only (in-band, not status-coded) |
| `DEADLINE_EXCEEDED` | Not raised by the server; surfaces only when the client deadline elapses (transport-level) | n/a | any |
| `UNKNOWN` | Not raised by handlers. Will surface only if a handler `raise`s a non-`AbortError`, non-`RpcError` exception that escapes the broad `except Exception` blocks — currently impossible because each handler has a fallback | n/a | n/a |
| `OUT_OF_RANGE`, `CANCELLED`, `ABORTED`, `DATA_LOSS` | Not used by the Python server | n/a | n/a |

Notes:

- `ExecuteAtomic` has a *layered* error model: the outer status stays
  `OK` even on apply failures; failures are reported in
  `ExecuteAtomicResponse{success=false, error, error_code}`. The Go
  port MUST preserve this. See `api/grpc_server.py:818-825`.
- The auth interceptor and quota interceptor produce status from
  *outside* the handler -- the Go port must keep these as interceptors
  too, otherwise the codes would not be observable for streaming /
  unary calls that never enter the handler body.

## Exception -> code mapping

Python exception types and the status code they map to. Many are
mapped *manually* by handlers (caught and rethrown via `context.abort`)
rather than via a central handler -- the Go port should centralize.

| Python exception | File | gRPC code | Proposed Go (`server/go/internal/errs`) |
|---|---|---|---|
| `grpc.aio.AbortError` | (raised by `context.abort`) | (already encoded) | `errs.From(status.Error(...))` -- pass-through |
| `AccessDeniedError` | `apply/acl.py:44` | `PERMISSION_DENIED` | `errs.ErrPermission` |
| `TenantNotFoundError` | `apply/canonical_store.py:165` | `NOT_FOUND` (via `_check_tenant`) | `errs.ErrNotFound` |
| `IdempotencyViolationError` | `apply/canonical_store.py:171` | `ALREADY_EXISTS` | `errs.ErrAlreadyExists` |
| `UniqueConstraintError`, `CompositeUniqueConstraintError` | `apply/applier.py:114,146` | `ALREADY_EXISTS` | `errs.ErrAlreadyExists` |
| `ValidationError`, `SchemaFingerprintMismatch` | `apply/applier.py:53,59` | reported in-band on `ExecuteAtomicResponse` (NOT a status code) | `errs.ErrValidation` (only the in-band path) |
| `QueryFilterError` | `apply/query_filter.py:60` | `INVALID_ARGUMENT` | `errs.ErrInvalidArgument` |
| `SchemaValidationError` | `schema/validator.py:18` | in-band on `ExecuteAtomicResponse` | `errs.ErrValidation` |
| `CompatibilityError` | `schema/compat.py:128` | `FAILED_PRECONDITION` (when surfaced through admin path) | `errs.ErrFailedPrecondition` |
| `RegistryFrozenError`, `DuplicateRegistrationError` | `schema/registry.py:51,57` | `FAILED_PRECONDITION` | `errs.ErrFailedPrecondition` |
| `AuthenticationError` | `auth/oauth_validator.py:44` | `UNAUTHENTICATED` (via interceptor) | `errs.ErrUnauthenticated` |
| `SessionError` | `auth/session_manager.py:36` | `UNAUTHENTICATED` | `errs.ErrUnauthenticated` |
| `ApiKeyError` | `auth/api_key_manager.py:41` | `UNAUTHENTICATED` | `errs.ErrUnauthenticated` |
| `AuthError` | `api/jwt_auth.py:40` | `UNAUTHENTICATED` | `errs.ErrUnauthenticated` |
| `CryptoShreddedError`, `TenantShreddedError` | `crypto/key_manager.py:59`, `crypto/tenant_key_vault.py:55` | `FAILED_PRECONDITION` (tenant has been crypto-shredded; permanent) | `errs.ErrFailedPrecondition` |
| `TenantKeyAlreadyProvisionedError` | `crypto/tenant_key_vault.py:67` | `ALREADY_EXISTS` | `errs.ErrAlreadyExists` |
| `WalConnectionError`, `WalTimeoutError` | `wal/base.py:47,53` | currently swallowed -> `OK` with empty/error response (see Swallow Patterns) | `errs.ErrUnavailable` (Go port may upgrade -- behind feature flag if it breaks contract tests) |
| `WalSerializationError` | `wal/base.py:59` | swallowed | `errs.ErrInternal` (in-band only) |
| `PermissionError` (Python builtin) | `api/grpc_server.py:1791,1849` | swallowed inside admin paths | `errs.ErrPermission` |

## Detail trailers

The Python server attaches trailing metadata in two places. The Go
port MUST emit these with identical keys and value formats (header
matching is byte-for-byte after lower-casing per HTTP/2).

| Trailer key | Status | Value | Set at |
|---|---|---|---|
| `entdb-redirect-node` | `UNAVAILABLE` | The owning node id (string) the SDK should retry against | `api/grpc_server.py:387-389` |
| `retry-after` | `RESOURCE_EXHAUSTED` (quota) | Seconds until next allowed attempt (`str(int)`) | `auth/quota_interceptor.py:233` |

Other RPC paths do NOT attach trailers. No `error_details` /
`google.rpc.Status` proto encoding is used today -- error context is
purely the gRPC `message` string. The Go port should NOT introduce
`google.rpc.ErrorInfo` / `google.rpc.RetryInfo` proto details
(out-of-scope for parity).

A subtle quirk: Python's `set_trailing_metadata` on the redirect path
is *synchronous* in `grpc.aio` but the code awaits it conditionally
(`api/grpc_server.py:390`) so unit-test `AsyncMock`s work. In Go this
is just `grpc.SetTrailer(ctx, metadata.Pairs(...))` followed by
returning the `status.Error(...)`.

Order matters: trailer MUST be set BEFORE the abort/return-error,
otherwise gRPC flushes trailers along with the closing status and the
late call is a no-op.

## Swallow patterns

The Python server has several broad `except Exception` blocks that
the Go port must mirror behavior-for-behavior to avoid breaking
contract tests:

1. **Re-raise gRPC aborts, swallow the rest**
   (`api/grpc_server.py:2740-2745`, `:2801-2806`, `:2857-2860`,
   `:2916-2920`, `:2976-2978`, `:3007-3009`, `:3065-3067`,
   `:3099-3101`, `:3158-3160`):
   ```
   except Exception as e:
       if isinstance(e, grpc.RpcError) or "StatusCode" in type(e).__name__:
           raise
       logger.error(...)
       return ResponseProto(success=False, error=str(e))
   ```
   These RPCs return `OK` with `success=false` for any non-gRPC
   exception. Go equivalent: `if status.Code(err) != codes.Unknown { return err } else { return &Resp{Success: false, Error: err.Error()}, nil }`.

2. **Pure swallow -> empty default** (`api/grpc_server.py:1600-1603`,
   `:826-828`, `:680`, `:720`, `:1514`, `:1533`, `:1929`, `:2735`,
   `:2794`, `:2852`):
   Catches everything and returns the proto's zero value (`ListTenantsResponse(tenants=[])`,
   `count=0`, etc.). Status stays `OK`. The Go port must emulate this
   for **read** paths used by the SDK happy-path test set.

3. **In-band error code on ExecuteAtomic**
   (`api/grpc_server.py:818-825`):
   Inner handler exception -> `ExecuteAtomicResponse{success=false,
   error=str(e), error_code="INTERNAL"}`, gRPC status `OK`. The string
   `"INTERNAL"` is part of the wire contract -- contract tests assert
   on it. Do NOT change to `codes.Internal`.

4. **Re-raise `AbortError`** (`api/grpc_server.py:1597-1599`):
   `grpc.aio.AbortError` is caught only to record metrics and is
   re-raised. Go: same -- record metric in deferred function, then
   return the status error.

## Go design

```
server/go/internal/errs/
    errs.go        -- typed sentinel errors + GRPCStatus()
    map.go         -- (Python-equivalent-exception) -> code translator
    trailers.go    -- helpers for redirect-node / retry-after
```

Sentinels (each implements `GRPCStatus() *status.Status` so
`status.Code(err)` works directly):

```go
var (
    ErrInvalidArgument    = newCode(codes.InvalidArgument)
    ErrNotFound           = newCode(codes.NotFound)
    ErrAlreadyExists      = newCode(codes.AlreadyExists)
    ErrPermission         = newCode(codes.PermissionDenied)
    ErrFailedPrecondition = newCode(codes.FailedPrecondition)
    ErrResourceExhausted  = newCode(codes.ResourceExhausted)
    ErrUnauthenticated    = newCode(codes.Unauthenticated)
    ErrUnavailable        = newCode(codes.Unavailable)
    ErrUnimplemented      = newCode(codes.Unimplemented)
)
```

Helper:

```go
// Errorf wraps msg in the canonical status. Use in handlers instead of
// status.Errorf so the Python -> Go contract stays in one place.
func Errorf(code codes.Code, format string, a ...any) error
```

Trailer helpers:

```go
func SetRedirectTrailer(ctx context.Context, ownerNode string) error
func SetRetryAfter(ctx context.Context, seconds int) error
```

Callers MUST invoke these before returning the error -- the request
spec for each RPC will instruct.

Single chokepoint for swallow-pattern (1):

```go
// PreserveStatusOrSwallow returns err if it already carries a gRPC
// status code; otherwise returns nil and writes the message into
// dst.Success/dst.Error. Mirrors grpc_server.py "isinstance(e, RpcError)"
// branch.
func PreserveStatusOrSwallow(err error, dst interface{ SetError(string) }) error
```

## Dependencies

- `google.golang.org/grpc/codes`
- `google.golang.org/grpc/status`
- `google.golang.org/grpc/metadata` (trailers)
- `google.golang.org/grpc` (`grpc.SetTrailer`)
- stdlib `errors` (for `errors.Is` against sentinels)

No third-party dependency is required. Do NOT pull
`google.golang.org/genproto/googleapis/rpc/errdetails` -- the Python
server does not use proto error details and adding them would
de-sync the contract.

## Test surface

Go-side mirror of Python contract assertions. Each becomes a row in
`tests/go/contract/error_codes_test.go` (or equivalent).

| Python assertion | Go assertion |
|---|---|
| `tests/python/integration/test_grpc_contract.py:725` -- `INVALID_ARGUMENT` for missing required field, every required-arg RPC | `status.Code(err) == codes.InvalidArgument` |
| `:731` -- `PERMISSION_DENIED` when caller is not admin/owner | `codes.PermissionDenied` |
| `:737` -- `UNIMPLEMENTED` when registry / global_store is `nil` | `codes.Unimplemented` |
| `tests/python/integration/test_privilege_escalation.py:209,228,247,284,341,363,378` -- claimed-actor mismatch -> `PERMISSION_DENIED` on `GetNode`, `ExecuteAtomic`, admin RPCs | `codes.PermissionDenied` for every per-handler privilege guard |
| `(legacy Python unit test, removed in Phase 4D)` -- 5 MiB request on default-cap server -> `RESOURCE_EXHAUSTED` | `codes.ResourceExhausted` (transport-layer) |
| Sharding redirect (when `_sharding.is_mine == false`) | `codes.Unavailable` AND trailer `entdb-redirect-node` matches owner |
| Region pin mismatch | `codes.FailedPrecondition`, no retry trailer |
| Quota exceeded | `codes.ResourceExhausted` AND trailer `retry-after` is a positive integer string |

For each existing per-RPC spec in `docs/go-port/rpcs/`, the "Errors"
section must list the codes from this catalogue rather than redefining
them.

## Open questions / risks

1. **`RESOURCE_EXHAUSTED` for oversized messages.** The Python wire-
   format test (`test_grpc_wire_format.py:293-317`) uses a server with
   the *default* 4 MiB receive cap; the production server raises the
   cap to 50 MiB (`api/grpc_server.py:3265-3266`). The Go port should:
   - Build the production server with `grpc.MaxRecvMsgSize(50<<20)`
     and `grpc.MaxSendMsgSize(50<<20)` to match Python defaults.
   - Run the contract test variant against a server built with the
     gRPC default (4 MiB). The Go gRPC default is also 4 MiB, so the
     code emitted for an oversized payload is `codes.ResourceExhausted`
     out-of-the-box -- parity is automatic provided we DO NOT call
     `grpc.MaxRecvMsgSize` in the test fixture.
   - Risk: gRPC-Go's exact error message for oversized messages is
     `"grpc: received message larger than max"`; SDK string-parses
     would break. The Python SDK only checks `code()`, so the message
     drift is acceptable -- but future SDK contributors must not
     depend on the message text.

2. **Retry trailer propagation.** `grpc.SetTrailer` only takes effect
   if called before the handler returns. If the handler is wrapped by
   an interceptor that re-classifies errors (we expect one for the
   swallow-pattern mapping), the interceptor must propagate trailers.
   Use `grpc.SetTrailer(ctx, ...)` (which writes to the call's trailer
   set, not a return value) before returning the status error.

3. **`ExecuteAtomic` in-band `error_code = "INTERNAL"`.** This is a
   string in the proto, NOT a gRPC code. Tempting to delete -- do not.
   The Python SDK keys retry behavior off the proto field, not the
   status. Keep the literal `"INTERNAL"`.

4. **`UNKNOWN` should never appear.** If contract tests start seeing
   `codes.Unknown`, it means an unwrapped Go error escaped a handler.
   Treat as a P0 bug. Add a final `recover()` / wrapping interceptor
   that maps any naked error to `errs.ErrInternal` *only* for
   `ExecuteAtomic` (which has the in-band channel) and re-raises
   elsewhere as `codes.Internal` -- after confirming no contract test
   currently exercises a `codes.Internal` path. (Today's Python
   surface emits zero `codes.Internal` from any handler.)

5. **Auth bypass set drift.** `UNAUTHENTICATED_METHODS` lives in two
   places (`auth/auth_interceptor.py:157`, `api/auth.py:51,102`) --
   the Go port should consolidate into a single
   `errs.AnonymousMethods` set referenced by the interceptor. Audit
   contract tests for any RPC that currently bypasses auth and
   document it in the per-RPC spec.

6. **Streaming RPCs.** None of the EntDB RPCs are streaming today; the
   quota interceptor (`auth/quota_interceptor.py:242-244`) explicitly
   only wraps unary-unary. If a streaming RPC is added during the
   port, this mapping must be revisited -- streaming errors arrive on
   `Send`/`Recv`, not as a single status.
