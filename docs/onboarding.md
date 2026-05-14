# Tenant & User Onboarding

EntDB does **not** auto-create tenants, users, or memberships on first
use. Every tenant and every actor that performs writes must be onboarded
explicitly via three admin RPCs before any data-plane RPC will succeed.

This is a deliberate change from pre-v1.12 behavior — the old Python
server implicitly registered tenants on first write, which was a
multi-tenant abuse vector and made auditability impossible. The Go
server (v1.12+, see [`docs/decisions/python-server-retired.md`][retired])
requires explicit onboarding.

[retired]: decisions/python-server-retired.md

## The three admin RPCs

A new actor on a new tenant takes three calls, in this order, from an
admin/system caller:

| # | RPC | What it creates | Idempotent? |
|---|-----|-----------------|-------------|
| 1 | `Admin.CreateUser` | Global `users` row keyed by `user_id` (also unique by email) | Returns `ALREADY_EXISTS` |
| 2 | `Admin.CreateTenant` | `tenant_registry` row + caller as `owner` in `tenant_members`, atomic | Returns `ALREADY_EXISTS` |
| 3 | `Admin.AddTenantMember` | `tenant_members` row binding the user to the tenant with a role | Returns `ALREADY_EXISTS` |

After all three land, the user (as `user:<user_id>`) can perform any
data-plane RPC on that tenant subject to ACL grants. Skip any one of
them and the data-plane returns `PERMISSION_DENIED` or `NOT_FOUND`.

### Why three, not one

The triple is intentional: a user can belong to multiple tenants, a
tenant can have multiple users, and the global `users` row is the place
email-uniqueness is enforced. Step 2 atomically attaches the **creator**
as the first owner (you cannot bootstrap a tenant without an owner — the
membership row is required for any subsequent admin operation on that
tenant); step 3 is for every additional user.

## Worked examples

### Python SDK

```python
import asyncio
from entdb_sdk import DbClient

async def onboard():
    # Connect as a control-plane caller. The `actor` here is the
    # default for subsequent data-plane calls; admin RPCs take their
    # own `actor=` kwarg.
    admin_client = DbClient(
        endpoint="entdb.internal:50051",
        tenant_id="_admin",          # placeholder; admin RPCs ignore it
        actor="system:admin",
    )
    await admin_client.connect()

    # 1. Register the user globally.
    await admin_client.create_user(
        user_id="alice",
        email="alice@acme.test",
        name="Alice",
        actor="system:admin",
    )

    # 2. Create the tenant. The caller (system:admin) becomes the
    #    first owner atomically — the registry row and the owner
    #    membership row land in a single transaction, so a crashed
    #    create cannot leave an orphan tenant.
    await admin_client.create_tenant(
        tenant_id="acme",
        name="Acme Corp",
        region="us-east-1",
        actor="system:admin",
    )

    # 3. Bind the user to the tenant as a member.
    await admin_client.add_tenant_member(
        tenant_id="acme",
        user_id="alice",
        role="member",          # or "admin" / "viewer"
        actor="system:admin",
    )

    await admin_client.close()

asyncio.run(onboard())
```

### Go SDK

```go
package main

import (
    "context"
    "log"

    "github.com/elloloop/tenant-shard-db/sdk/go/entdb"
)

func main() {
    ctx := context.Background()
    client, err := entdb.NewClient("entdb.internal:50051")
    if err != nil {
        log.Fatal(err)
    }
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    admin := client.Admin()

    // 1. Register the user globally.
    if _, err := admin.CreateUser(ctx,
        "system:admin", "alice", "alice@acme.test", "Alice",
    ); err != nil {
        log.Fatal(err)
    }

    // 2. Create the tenant (caller becomes owner atomically).
    if _, err := admin.CreateTenant(ctx,
        "system:admin", "acme", "Acme Corp",
        entdb.WithRegion("us-east-1"),
    ); err != nil {
        log.Fatal(err)
    }

    // 3. Add the user as a member of the tenant.
    if err := admin.AddTenantMember(ctx,
        "system:admin", "acme", "alice", "member",
    ); err != nil {
        log.Fatal(err)
    }
}
```

## Admin/system actors in production

The trusted-actor interceptor (`server/go/internal/auth/`) recognises
three privileged actor prefixes that bypass the per-tenant membership
check:

- `admin:<id>` — operator/admin identity
- `system:<service>` — internal service identity (e.g. `system:provisioner`)
- `__system__` — bootstrap/replay actor (never legitimate on the wire)

Onboarding RPCs require one of these. How a caller *obtains* such an
identity in production depends on the credential carrier configured on
the interceptor (in priority order):

1. **OAuth/OIDC bearer token** — `authorization: Bearer <jwt>`. The
   `sub` claim becomes the verified actor. To grant admin, mint a token
   with `sub: "system:provisioner"` (or `admin:root`). Any prefix the
   verifier accepts is honored verbatim — namespace your IdP-issued
   admin tokens so they cannot collide with regular user `sub`s.
2. **API key** — `x-api-key: <secret>`. The key's `name` becomes the
   verified actor. Provision API keys named `system:provisioner` or
   `admin:onboarder` and treat them as control-plane credentials.
3. **Session token** — `x-session-token: <token>`. The session's
   `user_id` becomes the verified actor. Operator consoles typically
   use this path; mint admin sessions only from a hardened admin UI.

The wire-level `actor` field in every RPC request is **untrusted**: the
trusted-actor interceptor's verified identity always wins. A user
authenticated as `user:alice` who sets `actor: "system:admin"` in the
request body is rejected with `PERMISSION_DENIED`. See
[`docs/go-port/shared/auth-interceptor.md`][auth] for the full contract.

[auth]: go-port/shared/auth-interceptor.md

### Recommended production wiring

For a control-plane integration (identity service onboarding new
customers):

- Provision an API key named `system:provisioner` and store it in your
  secrets manager.
- Have the identity service inject `x-api-key: <secret>` on every
  EntDB admin call.
- Bind EntDB to a private subnet so the admin RPCs are not reachable
  from public networks; if they must be exposed, gate them behind a
  separate listener with stricter network policy.

For local development, the no-auth fallback applies: if no interceptor
is configured (or no credentials are sent), the server trusts the wire
`actor` field. **Production servers MUST run the auth interceptor** —
without it, every caller can claim `system:admin`.

## Idempotency & retry

All three onboarding RPCs are safe to retry:

- **`CreateUser`** — duplicate `user_id` (or duplicate `email`) returns
  `ALREADY_EXISTS`. Callers can treat this as success.
- **`CreateTenant`** — duplicate `tenant_id` returns `ALREADY_EXISTS`.
  The RPC appends a global `tenant_created` WAL event and waits for the
  applier; the applier inserts the registry row and owner membership in
  a single globalstore transaction, so WAL replay reconstructs both rows.
- **`AddTenantMember`** — duplicate `(tenant_id, user_id)` returns
  `ALREADY_EXISTS`. Role changes go through `Admin.ChangeMemberRole`,
  not a re-add.

A typical onboarding driver wraps each call with "succeed-or-already-
exists" handling and proceeds to the next step on either outcome.

## Where things live

- Admin RPC handlers: `server/go/internal/api/create_user.go`,
  `create_tenant.go`, `add_tenant_member.go`.
- Global admin WAL apply path:
  `server/go/internal/apply/global.go` and
  `server/go/internal/globalstore/apply.go`.
- Tenant gate (rejects RPCs on unknown tenants):
  `server/go/internal/tenant/check.go`, wired via
  `Server.checkTenant` in `server/go/internal/api/server.go`.
- Trusted-actor resolution: `server/go/internal/auth/authoritative.go`.

## Common errors

| You see | Likely cause |
|---|---|
| `NOT_FOUND: tenant "acme" not found` on a data-plane RPC | Step 2 (`CreateTenant`) was never run for this tenant |
| `PERMISSION_DENIED: ... is not a member of "acme"` | Step 3 (`AddTenantMember`) was never run for this user/tenant |
| `PERMISSION_DENIED: CreateTenant requires admin or system actor` | Verified actor on the request is `user:<...>`, not `admin:` / `system:` — check your auth interceptor |
| `ALREADY_EXISTS: tenant "acme" already exists` | Idempotent retry of step 2; safe to swallow and proceed |
| `ALREADY_EXISTS: user "alice@acme.test" already exists` | Idempotent retry of step 1; safe to swallow and proceed |
