# API Reference (wire-level)

> ## ⚠ Internal transport — use an SDK
>
> The gRPC surface below is an **internal transport** used by the official SDKs. It is not a stable public contract; the wire format may change between releases, and the SDKs enforce non-obvious invariants (field IDs, actor strings, ACL encoding, idempotency keys, schema fingerprints, capability ids) that hand-rolled clients will get wrong.
>
> Use one of the sanctioned SDKs:
>
> - **Go:** `github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2` ([pkg.go.dev](https://pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2))
> - **Python:** `pip install entdb-sdk` ([PyPI](https://pypi.org/project/entdb-sdk/))
>
> This reference is for SDK maintainers and operators debugging the wire — not for application code. For application-level docs, see [sdk-reference.md](sdk-reference.md).

## No HTTP API

**The Go server (v1.12+) is gRPC-only.** There is no `/v1/atomic`, no `/v1/nodes/:id`, no `/health` HTTP endpoint, no REST gateway. The historical Python server had partial HTTP support; that surface was retired in EPIC #407 Phase 4D and not reintroduced.

If you need an HTTP-style interface for browser tooling, the [EntDB Console](../sdk/go/entdb/cmd/entdb-console/) provides a Connect-RPC bridge over HTTP. The console is curated — it exposes a deliberately small subset for browser access, not the full RPC surface.

## gRPC contract

The canonical contract lives in [`proto/entdb/v1/entdb.proto`](../proto/entdb/v1/entdb.proto). 44 RPCs total. To browse:

```bash
# Reflection (works in dev when no auth/TLS gate blocks it)
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 list entdb.v1.EntDBService
grpcurl -plaintext localhost:50051 describe entdb.v1.EntDBService

# Or compile the proto and inspect
protoc --descriptor_set_out=/tmp/entdb.bin proto/entdb/v1/entdb.proto
buf curl --schema proto/entdb/v1/entdb.proto --reflect ...
```

## Service / RPC families

The 44 RPCs cluster into the following groups. Source-of-truth for shapes is the proto file; the spec docs in [`docs/go-port/rpcs/`](go-port/rpcs/) carry per-RPC design notes (some still cite the retired Python source — see ADR-019).

**Data plane (per-tenant):**

- `ExecuteAtomic` — the only mutating RPC for tenant data. Builds one or more ops in a single WAL event. Op union includes the `delete_where` predicate sweeper (see [`ExecuteAtomic` operations](#executeatomic-operations)).
- `GetNode`, `GetNodes`, `GetNodeByKey`, `QueryNodes`, `SearchNodes`
- `GetEdgesFrom`, `GetEdgesTo`, `GetConnectedNodes`
- `GetMailbox`, `SearchMailbox`, `ListMailboxUsers`
- `GetReceiptStatus`, `WaitForOffset` — read-after-write helpers

**ACL / sharing:**

- `ShareNode`, `RevokeAccess`, `ListSharedWithMe`
- `AddGroupMember`, `RemoveGroupMember`
- `TransferOwnership`, `DelegateAccess`

**Schema:**

- `GetSchema` — read-only; returns the server's registered schema fingerprint + types.

**Health:**

- `Health` (EntDB-specific) — version + applier status
- `grpc.health.v1.Health/Check` — standard gRPC health protocol; both bypass auth

**Admin / control plane (cross-tenant):**

- `CreateUser`, `UpdateUser`, `GetUser`, `ListUsers`, `DeleteUser`, `FreezeUser`, `CancelUserDeletion`
- `CreateTenant`, `GetTenant`, `ArchiveTenant`, `ListTenants`
- `AddTenantMember`, `RemoveTenantMember`, `GetTenantMembers`, `GetUserTenants`, `ChangeMemberRole`
- `GetTenantQuota`
- `TransferUserContent`, `RevokeAllUserAccess`, `ExportUserData`
- `SetLegalHold`

See [Onboarding](onboarding.md) for the typical admin-RPC flow.

## Wire conventions

### Field IDs, not field names

Per [ADR-018](adr/018-field-id-keyed-payloads.md), payloads on the wire and on disk are keyed by **stringified `field_id`s** (`"1"`, `"2"`, ...), not by names. The SDKs do name↔id translation client-side from the proto descriptor (where `fd.Number()` IS the EntDB `field_id` per ADR-006).

```
WIRE:        {"1": "alice@example.com", "2": "Alice"}
APP CODE:    {"email": "alice@example.com", "name": "Alice"}
```

Schemaless ingress (server doesn't know the type's schema): only digit-string keys are accepted; name-keyed entries return `INVALID_ARGUMENT`.

### Idempotency

Every `ExecuteAtomic` request carries an `idempotency_key`. Replaying the same key returns the same result without re-applying. Keyed on `(tenant_id, idempotency_key)`.

### Authentication

Per [ADR-011](adr/011-security-and-compliance.md), the gRPC interceptor accepts three credential carriers in fixed priority order:

1. OAuth/OIDC bearer: `authorization: Bearer <jwt>` header
2. API key: `x-api-key: <secret>` header
3. Session: `x-session-token: <token>` header

Plus mTLS client cert when configured (`-require-client-cert=true`); the subject DN / SAN flows into the request context and bridges to the trusted-actor identity.

`Health` and `grpc.health.v1.Health/Check` bypass authentication.

### Actor / trusted-actor pattern

The wire `actor` field on every RPC is **untrusted**. The trusted identity comes from the verified credential (above). The wire field is rebound at the handler via `auth.Authoritative(ctx, claimed)` — see [ADR-016 trusted-actor section](adr/016-handlers-append-applier-writes.md).

Actor prefixes:

- `user:<id>` — a real authenticated user
- `system:<service>` — internal service identity; bypasses tenant-membership checks
- `admin:<id>` — operator/admin identity; same bypass as `system:`
- `__system__` — bootstrap/replay actor used by the applier; never legitimate on the wire
- `group:<id>` — only valid in ACL grant subjects, never as a caller

### gRPC status codes

| Status | When |
|---|---|
| `OK` | Success |
| `INVALID_ARGUMENT` | Malformed request, schema mismatch, name-keyed payload on schemaless type |
| `NOT_FOUND` | Node, tenant, user not found |
| `ALREADY_EXISTS` | Duplicate `tenant_id`, duplicate unique-field value, etc. |
| `PERMISSION_DENIED` | ACL check failed; actor not a tenant member; admin gate failed |
| `FAILED_PRECONDITION` | CAS precondition mismatched (`UpdateNode` with `precondition`) |
| `UNAUTHENTICATED` | Auth header missing/invalid |
| `UNAVAILABLE` | Server starting up, WAL unreachable, tenant pinned to a different region (see redirect trailer) |
| `RESOURCE_EXHAUSTED` | Quota / rate limit (when implemented; see ADR-011 status table) |
| `INTERNAL` | Server bug or unexpected backend error |

### Cross-region redirect

When a tenant is pinned to a region the current server doesn't serve, the RPC returns `UNAVAILABLE` with a trailer `entdb-redirect-node: <host:port>`. SDKs handle this automatically; hand-rolled clients have to follow it.

## Payload encoding

The `data` (create) and `patch` (update) fields of operations carry a `google.protobuf.Struct` keyed by stringified field IDs:

```json
{
  "create_node": {
    "type_id": 1,
    "data": {
      "1": "alice@example.com",
      "2": "Alice"
    },
    "as_": "alice",
    "acl": [ ... ]
  }
}
```

Special kind coercions on ingress (server-side, when the schema is known):

- **bytes**: wire carries a base64 string; server decodes to `[]byte`.
- **timestamp / int64**: wire carries `float64` (structpb's only number type); server coerces to `int64`.
- **enum**: wire carries the enum value as a string (not the proto integer); kept as string in storage.
- **json**: nested `Struct` values are unwrapped to native maps once; never recurses.

On egress, the wire shape is symmetric: `[]byte` re-encoded as base64, `int64` written as `float64` (structpb), enums as strings.

## `ExecuteAtomic` operations

`ExecuteAtomic` carries `repeated Operation operations`; each `Operation` is a `oneof op` over the six op messages. These are wire contract but are **not** standalone RPCs — the machine-extracted inventory of all six is in [`docs/generated/api-reference.md`](generated/api-reference.md).

| `op` field | Message | Purpose |
|---|---|---|
| `create_node` | `CreateNodeOp` | Insert a node. |
| `update_node` | `UpdateNodeOp` | Patch a node, optionally with a CAS precondition. |
| `delete_node` | `DeleteNodeOp` | Delete a single node by id. |
| `create_edge` | `CreateEdgeOp` | Insert an edge. |
| `delete_edge` | `DeleteEdgeOp` | Delete an edge. |
| `delete_where` | `DeleteWhereOp` | Predicate sweeper: delete every node of a type matching all `where` filters, best-effort capped by `limit`, in one op (issue [#504](https://github.com/elloloop/tenant-shard-db/issues/504)). |

`DeleteWhereOp.where` reuses the same `FieldFilter` / `FilterOp` shape as `QueryNodesRequest.filters`, so no new wire concept is introduced. As with `QueryNodes` filters, the filter `field` is normally a payload `field_id` resolved client-side from a name; against a **schema-less server** only digit-string keys translate (a name key returns `INVALID_ARGUMENT` — see [Field IDs, not field names](#field-ids-not-field-names) and the [schema-lockdown guide](guides/schema-lockdown.md#delete_where-and-querynodes-on-schema-less-deployments), issue [#545](https://github.com/elloloop/tenant-shard-db/issues/545)). `limit <= 0` selects the server default cap; the server clamps to a hard ceiling so a runaway predicate cannot pin the single applier goroutine for a tenant. Deleted ids are not returned (v1 scope, #504) — callers needing the ids keep using the `QueryNodes` + `DeleteNodeOp` loop.

## Schema RPC

`GetSchema` returns the **server's** registered schema (loaded at boot from `.schema-snapshot.json`). It's read-only — there is no `RegisterSchema` RPC. Clients use the schema for client-side validation; the SDKs maintain their own local registry via `register_proto_schema(my_pb2)` (Python) or generated code (Go).

## Where to find each handler

- gRPC handlers: `server/go/internal/api/<rpc_snake_case>.go`
- Per-RPC design notes: `docs/go-port/rpcs/<RPC>.md` (some still cite the retired Python source — those citations are historical)
- WAL op type definitions: `server/go/internal/wal/event.go`
- Applier dispatch: `server/go/internal/apply/applier.go`

## Related

- [`proto/entdb/v1/entdb.proto`](../proto/entdb/v1/entdb.proto) — wire contract (source of truth)
- [`proto/entdb/v1/entdb_options.proto`](../proto/entdb/v1/entdb_options.proto) — EntDB-specific proto options for schema declaration
- [sdk-reference.md](sdk-reference.md) — application-level SDK API
- [ADR-006](adr/006-proto-schema-definition.md) — proto-as-type-system
- [ADR-016](adr/016-handlers-append-applier-writes.md) — write-path contract
- [ADR-018](adr/018-field-id-keyed-payloads.md) — payload key contract on wire and disk
