# Shared Port Spec — Auth Interceptor + Trusted-Actor Pattern

EPIC #407 — Python → Go server port. Source of truth:
`server/go/internal/auth/interceptor.go` (the gRPC interceptor
and `get_authoritative_actor` helper). Behavioural pin:
`tests/python/integration/test_privilege_escalation.py` (34 cases).

This doc is referenced by every RPC spec under `docs/go-port/rpcs/` — the
Go port MUST land this primitive before any privileged RPC is wired up.

## Wire-level mechanics

The interceptor reads gRPC metadata only — no TLS-cert or peer-address
identity. Three credential carriers are accepted, in this fixed order
(`auth_interceptor.py:222-276`):

1. **OAuth/OIDC bearer token** — header `authorization: Bearer <jwt>`.
   The leading `Bearer ` is stripped (`:245`). A value is treated as a JWT
   only if it has exactly two dots (`_is_jwt`, `:278-281`). Validation
   delegates to `OAuthValidator.validate_token` (RS256/ES256 against JWKS,
   `oauth_validator.py:41`); identity is `claims["sub"]`, falling back to
   `claims["email"]`, falling back to `"unknown"` (`:248`).
2. **API key** — header `x-api-key: <secret>`. Looked up via
   `ApiKeyManager.validate_key` (`api_key_manager.py`); identity is the
   key's `name`. Scopes are surfaced on `AuthContext.scopes` for use by
   the quota interceptor and any future scope-gated RPC.
3. **Session token** — header `x-session-token: <token>`. Validated via
   `SessionManager.validate_session` (`session_manager.py`); identity is
   the session's `user_id`.

The first method whose header is non-empty AND whose validator is
configured is used; subsequent methods are not tried. Validation failure
in any path raises (`AuthenticationError` / `ApiKeyError` / `SessionError`)
and the interceptor returns gRPC `UNAUTHENTICATED`
(`auth_interceptor.py:193-198`). If no header is present at all, the
interceptor returns `UNAUTHENTICATED` with `"No valid authentication
credentials provided."` (`:200-204`).

A small allow-list bypasses auth entirely (`UNAUTHENTICATED_METHODS`,
`:157-162`):

- `/entdb.EntDBService/Health`
- `/grpc.health.v1.Health/Check`

On success the verified identity string is parked in a Python
`ContextVar` (`_current_identity`, `:61-63`) for the duration of the
handler invocation, then reset in a `finally` block (`:216-220`). This
ContextVar is what `get_authoritative_actor` reads. **The Go port must
replace the ContextVar with `context.Context` value propagation** (see
"Go interface" below) — there is no equivalent of an async-task-local
`ContextVar` that survives across goroutine hops, and a
`grpc.UnaryServerInterceptor` is the natural place to attach a value.

## Trusted-actor contract

`get_authoritative_actor(request_actor: str) -> str`
(`auth_interceptor.py:92-115`):

- If the interceptor populated a trusted identity for this request, that
  value is returned — **even if `request_actor` claims a different (or
  more-privileged) actor**. This is the privilege-escalation fix.
- The trusted identity is normalised: if it already starts with `user:`,
  `system:`, `admin:`, or equals the literal `__system__`, it is
  returned verbatim. Otherwise it is wrapped as `user:<identity>`
  (`:108-115`). So a JWT `sub: "alice"` becomes `user:alice`; a session
  for `user_id: "user:alice"` stays `user:alice`.
- If no trusted identity is set (interceptor disabled, unit tests, or
  the no-auth deployment mode), `request_actor` is returned unchanged.
  This is the documented fallback for tests and dev-mode servers; a
  production server MUST run the interceptor.

`Actor` is just a string in this codebase — there is no struct. The
prefix encodes the kind:

- `user:<id>` — a real authenticated user.
- `system:<service>` — internal service identity (e.g.
  `system:gdpr-worker`); bypasses tenant-membership checks
  (`grpc_server.py:498-499`).
- `admin:<id>` — operator/admin identity; same bypass as `system:`.
- `__system__` — bootstrap/replay actor used by the Applier; never
  appears on the wire.
- `group:<id>` — only valid in ACL grant subjects, never as a caller
  identity.

For Go, model this as `type Actor string` (zero-cost) plus accessors,
not a sum type — every site already does prefix-string checks.

## Privilege escalation invariants

`tests/python/integration/test_privilege_escalation.py` is the
behavioural pin. Every RPC that takes a wire-level `actor` (top-level
field or nested in `RequestContext`) MUST consult the trusted identity
before any authorization decision and MUST persist the trusted identity
into the WAL when the operation succeeds.

Authentication setup in every test: `with _trusted_identity("user:eve")`
(`:97-104`). Privileged claims tried in the payload: `system:admin`,
`system:gdpr-worker`, `__system__`, `admin:root` (`CLAIMED_ADMIN_ACTORS`,
`:166-171`).

The 34 cases break down by class:

| Class | Cases | RPCs | Asserted |
|-------|-------|------|----------|
| Read-path, `RequestContext.actor` | 12 | `GetNode`, `GetNodes`, `QueryNodes` × 4 claimed strings | `ctx.aborted == True` AND `abort_code == PERMISSION_DENIED` (`:204-209`, `:225-228`, `:244-247`) |
| Write-path reject (`ExecuteAtomic`, non-member) | 4 | `ExecuteAtomic` × 4 | `PERMISSION_DENIED` regardless of payload claim (`:280-284`) |
| Write-path persists trusted (`ExecuteAtomic`, member) | 1 | `ExecuteAtomic` with eve as member, claim `system:admin` | WAL event `actor == "user:eve"` (`:308-313`) — proves the persisted log is not poisoned |
| Admin-only top-level `actor` | 12 | `CreateUser`, `TransferUserContent`, `GetTenantQuota` × 4 claimed | `PERMISSION_DENIED`; for `TransferUserContent` also `wal.append.assert_not_awaited()` (`:362-365`) |
| `ListTenants` membership filter | 4 | `ListTenants` (one per claimed string) | eve sees only `["globex"]`; sanity check that a *trusted* admin caller DOES see all three tenants (`:402-417`) |
| Edge read negative | 1 | `GetEdgesFrom` with payload `system:admin` | handler does not crash and does not forward the claimed actor downstream (`:443-447`) |

Total: **34** parameterised cases across 8 RPCs. Any Go handler that
fails to substitute the trusted actor will trip at least one of these
when the contract test is ported.

## Go interface

Proposed package layout (mirrors `server/python/entdb_server/auth/`):

```
server/go/internal/auth/
  interceptor.go     // UnaryAuthInterceptor + StreamAuthInterceptor
  identity.go        // contextKey, Authoritative(), WithIdentity()
  actor.go           // Actor type, prefix predicates
  oauth.go           // JWKS-backed JWT validator
  apikey.go          // API key store
  session.go         // session manager
  errors.go          // typed errors -> codes.Unauthenticated
```

Sketch (no implementation in this doc):

```go
package auth

type contextKey struct{}
var identityKey = contextKey{}

type Identity struct {
    Method   string   // "oauth" | "api_key" | "session"
    Subject  string   // raw verified id (e.g. JWT sub)
    Scopes   []string // API-key scopes; nil for oauth/session
    Claims   map[string]any
}

type Interceptor struct {
    OAuth    *OAuthValidator
    APIKeys  *APIKeyManager
    Sessions *SessionManager
}

func (i *Interceptor) Unary() grpc.UnaryServerInterceptor
func (i *Interceptor) Stream() grpc.StreamServerInterceptor

// Identity from ctx, or zero value if anonymous.
func IdentityFromContext(ctx context.Context) (Identity, bool)

// Authoritative implements get_authoritative_actor semantics:
// trusted wins over claimed; falls back to claimed when no identity.
func Authoritative(ctx context.Context, claimed Actor) Actor
```

The `Health` allow-list is matched on `info.FullMethod` exactly as in
Python (`auth_interceptor.py:185-187`). For streaming RPCs use
`grpc.ServerStream` wrapping to keep the augmented context available
to the handler.

The `Authoritative` helper is the choke point — call it once at the top
of each handler and rebind the local variable, mirroring
`grpc_server.py:493`. Subsequent helpers (`_is_admin_or_system`,
`_check_tenant_access`, `_check_cross_tenant_read`) all re-call it as
defence-in-depth; the Go port should keep that belt-and-suspenders.

## Dependencies

- `pb` (generated proto): no runtime dependency from the interceptor —
  it operates on metadata only. `Authoritative()` is consumed by every
  handler in `pkg/server/grpc/...`.
- `errs` mapping: `OAuth/APIKey/SessionError` → `codes.Unauthenticated`
  with the original message; transport-level failures (JWKS fetch,
  store backend) → `codes.Unavailable`. Mirror the Python catch
  (`auth_interceptor.py:193-198`).
- Context plumbing: install via
  `grpc.ChainUnaryInterceptor(authInterceptor.Unary(), quotaInterceptor.Unary(), ...)`.
  Order matters — auth must run *before* quota so the quota interceptor
  can read the identity for per-identity rate limiting.
- `sync.Map` (or a cache library like `ristretto`) for JWKS and session
  lookups — Python uses dicts behind a single `asyncio.Lock`; Go needs
  proper concurrency primitives.

## RPCs that consume this

Every RPC in `entdb.v1.EntDBService` consumes the interceptor for
authentication. The set that additionally consumes `Authoritative` for
authorization (i.e. cannot be ported correctly without it) is at least:

- `ExecuteAtomic` (writes WAL; `grpc_server.py:435-437`, `:491-493`)
- `GetNode`, `GetNodes`, `QueryNodes`, `SearchNodes` (cross-tenant read
  check; `grpc_server.py:586-588`)
- `GetEdgesFrom`, `GetEdgesTo`
- `CreateUser`, `UpdateUser`, `DeleteUser`, `GetUser`, `ListUsers`
- `CreateTenant`, `ArchiveTenant`, `DeleteTenant`, `RestoreTenant`,
  `SetTenantLegalHold`
- `AddGroupMember`, `RemoveGroupMember`, `ChangeMemberRole`
- `TransferUserContent` (`_require_admin_or_owner`,
  `grpc_server.py:2671-2673`)
- `GetTenantQuota`, `SetTenantQuota`
- `ListTenants` (special: rejects when `get_current_identity()` is
  None, `grpc_server.py:1561-1566` — does NOT fall back to the payload)
- All GDPR / audit-export RPCs

Bypass / unauthenticated:

- `Health` and `grpc.health.v1.Health/Check` — explicit allow-list
  (`auth_interceptor.py:158-162`). The Go port MUST keep the allow-list
  exact; orchestrators (k8s, ECS) probe the standard health service
  without credentials (`grpc_server.py:3274-3275`).

Special: `ListTenants` deliberately does **not** fall back to the
payload when the interceptor is absent — it aborts with
`PERMISSION_DENIED` instead (`grpc_server.py:1562-1566`). The Go port
must preserve this; it is the one RPC where the
"no-interceptor → trust claimed" fallback is unsafe (cross-tenant
enumeration).

## Open questions / risks

- **Tenant scoping from token vs request.** Today every request carries
  `RequestContext.tenant_id` and the interceptor has no opinion on it;
  membership is checked in `_check_tenant_access`. JWTs commonly carry a
  `tenant_id` (or list) claim — should the Go port reject requests
  whose `request.context.tenant_id` does not match a token claim?
  Decision deferred; flagged in EPIC #407. Today: trust the request
  field, gate via membership.
- **Session caching strategy.** Python uses an in-memory dict
  (`session_manager.py:14-23`). For the Go port behind a load balancer
  this needs to be either (a) sticky sessions, (b) Redis, or (c)
  signed/stateless session tokens. Picking one is out of scope here but
  must be settled before the Go server is multi-replica.
- **Rate-limit-by-identity.** The quota interceptor
  (`auth/quota_interceptor.py`) currently keys off tenant. Once
  `Identity` is in `context.Context`, per-identity (per-API-key) rate
  limits become trivial — but the quota proto needs new fields. Track
  separately.
- **JWKS rotation thundering herd.** Python serialises refetch behind a
  single asyncio lock. Go's natural pattern is `singleflight.Group` —
  use that, not a plain mutex, so a kid miss does not block all
  concurrent requests on the same JWKS URL.
- **`sub` collisions across providers.** When multiple OIDC issuers are
  configured, two different users at two providers can share a `sub`.
  Python ignores this. The Go port should namespace identities by
  issuer (e.g. `user:<iss>#<sub>`) — but doing so changes every
  persisted WAL `actor` and is a breaking change. Leave Python-compat
  for now; revisit before multi-IdP support ships.
- **Constant-time API key compare.** `validate_key` should use
  `subtle.ConstantTimeCompare` on the SHA-256 hash; flagged in
  `api_key_manager.py:23` as a TODO and inherited.
