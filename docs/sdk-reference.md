# SDK Reference

EntDB ships two official SDKs. Both are thin wrappers over the same gRPC contract and follow the same single-shape API (see [ADR-006](adr/006-proto-schema-definition.md)). Use the proto messages your `protoc` invocation generates — there's no parallel `NodeTypeDef` class to hand-build.

This page covers both SDKs side by side. For the wire-level RPC contract, see [api-reference.md](api-reference.md).

## Installation

**Python:**

```bash
pip install entdb-sdk
```

Requires Python 3.10+.

**Go:**

```bash
go get github.com/elloloop/tenant-shard-db/sdk/go/entdb@latest
```

Requires Go 1.22+. Docs auto-render at [pkg.go.dev](https://pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb).

## Schema

EntDB types are proto messages with EntDB-specific options. The proto field number IS the on-disk `field_id` ([ADR-018](adr/018-field-id-keyed-payloads.md)).

```protobuf
syntax = "proto3";
import "entdb/v1/entdb_options.proto";
package myapp;

message User {
  option (entdb.node) = { type_id: 1 };
  string email = 1 [(entdb.field) = { required: true, unique: true }];
  string name  = 2;
}

message Task {
  option (entdb.node) = { type_id: 2 };
  string title = 1 [(entdb.field) = { required: true }];
  string status = 2 [(entdb.field) = { enum_values: "todo,doing,done" }];
}

message AssignedTo {
  option (entdb.edge) = { edge_id: 100 };
  Task from = 15;
  User to   = 16;
}
```

Generate stubs with standard `protoc`; no custom EntDB codegen step.

**Python — register the proto module with the SDK at startup:**

```python
from entdb_sdk.codegen import register_proto_schema
import schema_pb2

register_proto_schema(schema_pb2)
```

**Go — generated types are used directly:**

```go
import myschema "example.com/myapp/schema"
// No registration step; the SDK reads the descriptor at use site.
```

## Connection

### Python — `DbClient`

```python
from entdb_sdk import DbClient

client = DbClient(
    endpoint="localhost:50051",   # gRPC endpoint
    tenant_id="my_tenant",        # tenant for data-plane ops
    actor="user:alice",           # identity for ACL
    use_tls=False,                # set True in production
    timeout=30.0,                 # default per-call timeout
)

await client.connect()
# ...
await client.close()

# Or as an async context manager:
async with DbClient(endpoint=..., tenant_id=..., actor=...) as client:
    ...
```

### Go — `entdb.Client`

```go
import "github.com/elloloop/tenant-shard-db/sdk/go/entdb"

client, err := entdb.NewClient("localhost:50051",
    entdb.WithTLS(tlsConfig),         // omit for dev plaintext
    entdb.WithTimeout(30*time.Second),
)
if err != nil { /* ... */ }
if err := client.Connect(ctx); err != nil { /* ... */ }
defer client.Close()
```

### Scope (tenant + actor pre-bound)

Both SDKs offer a scope handle that pre-binds tenant and actor so you don't repeat them on every call:

```python
# Python
scope = client.tenant("acme").actor("user:alice")
node = await scope.get(SomeType, node_id)
```

```go
// Go
scope := client.Tenant("acme").Actor("user:alice")
node, err := entdb.Get[*SomeType](ctx, scope, nodeID)
```

## Atomic transactions (Plan)

A `Plan` builds one or more operations applied atomically. Use it whenever you need to create related nodes + edges in one round-trip, or when read-your-write is needed via `wait_applied`.

### Python

```python
result = await client.atomic(lambda plan: (
    plan.create(schema_pb2.User(email="alice@example.com", name="Alice"), as_="alice"),
    plan.create(schema_pb2.Task(title="Review PR", status="todo"), as_="task"),
    plan.edge_create(schema_pb2.AssignedTo, "$task.id", "$alice.id"),
))

# With idempotency
result = await client.atomic(
    lambda plan: plan.create(schema_pb2.User(...)),
    idempotency_key="create_alice_v1",
)

# Block until the applier has materialized the write (read-your-write)
result = await client.atomic(
    lambda plan: plan.create(schema_pb2.User(...)),
    wait_applied=True,
)
```

### Go

```go
plan := client.NewPlan("acme", "user:alice")
alice := plan.Create(&myschema.User{Email: "alice@example.com", Name: "Alice"})
task  := plan.Create(&myschema.Task{Title: "Review PR", Status: "todo"})
entdb.EdgeCreate[*myschema.AssignedTo](plan, task, alice)

result, err := plan.Commit(ctx,
    entdb.WithIdempotencyKey("create_alice_v1"),
    entdb.WithWaitApplied(true),
)
```

### Plan methods

| Operation | Python | Go |
|---|---|---|
| Create a node | `plan.create(msg, *, as_=, acl=, storage=)` | `plan.Create(msg, opts...) string` |
| Update a node | `plan.update(node_id, msg)` | `plan.Update(nodeID, msg)` |
| Update with precondition (CAS) | `plan.update(id, msg, precondition=("field", value))` | `plan.UpdateIf(...)` |
| Delete a node | `plan.delete(NodeType, node_id)` | `entdb.Delete[T](plan, nodeID)` |
| Delete by predicate (sweeper) | `plan.delete_where(NodeType, where, *, limit=0)` | `entdb.DeleteWhere[T](plan, where, limit)` |
| Create an edge | `plan.edge_create(EdgeType, from_id, to_id, props={})` | `entdb.EdgeCreate[T](plan, from, to, opts...)` |
| Delete an edge | `plan.edge_delete(EdgeType, from_id, to_id)` | `entdb.EdgeDelete[T](plan, from, to)` |
| Commit | `await plan.commit(...)` | `plan.Commit(ctx, ...)` |

`as_` (Python) / the alias returned from `plan.Create` (Go) lets later ops in the same plan reference the new node before it has a server-assigned ID. The server resolves `$alias.id` at apply time.

#### `delete_where` — single-RPC predicate sweeper

`delete_where` (issue [#504](https://github.com/elloloop/tenant-shard-db/issues/504)) deletes every node of a type matching an AND-ed list of `Filter` predicates in **one** `ExecuteAtomic` op, collapsing the TTL-sweeper "query for ids, then delete each" loop into a single round-trip. At least one filter is required — an unconditional bulk delete is rejected. `limit` is best-effort (Postgres `DELETE … LIMIT` semantics): `0` selects the server default, which the server clamps to its own ceiling so a runaway predicate cannot pin a tenant's single applier; drain a backlog by sweeping in a loop until a sweep deletes nothing.

```python
plan.delete_where(
    schema_pb2.WebAuthnChallenge,
    [Filter(field="expires_at", op=FilterOp.LT, value=now_ms)],
    limit=1000,
)
```

```go
entdb.DeleteWhere[*myschema.WebAuthnChallenge](plan,
    []entdb.Filter{{Field: "expires_at", Op: entdb.FilterLt, Value: nowMs}}, 1000)
```

The predicate is resolved exactly like `query(where=)` — server-side, from payload `field_id`s. **Schema-less servers** (issue [#545](https://github.com/elloloop/tenant-shard-db/issues/545)): a server started without a registered schema cannot translate a field *name* to a `field_id`. Pass the numeric payload field id as the filter field (e.g. `Filter(field="4", ...)` / `{Field: "4", ...}`); the SDK forwards a digit-only key verbatim and the server treats it as a raw `field_id` with no schema lookup — the same schema-optional escape hatch `QueryNodes` filters already accept. Against a schema-less server a *name* key returns `INVALID_ARGUMENT` (`"cannot translate filter key … without a schema"`). See [Schema lockdown → `delete_where` on schema-less deployments](guides/schema-lockdown.md#delete_where-and-querynodes-on-schema-less-deployments).

## Reads

### Get by ID

```python
node = await scope.get(schema_pb2.User, node_id)  # returns a Node[User]
```

```go
node, err := entdb.Get[*myschema.User](ctx, scope, nodeID)
```

### Get by unique key

The proto field with `(entdb.field) = { unique: true }` becomes a typed key token via codegen.

```python
from schema_entdb import UserKeys
user = await scope.get_by_key(UserKeys.email, "alice@example.com")
```

```go
var UserEmail = entdb.UniqueKey[string]{TypeID: 1, FieldID: 1, Name: "email"}
user, err := entdb.GetByKey[*myschema.User](ctx, scope, UserEmail, "alice@example.com")
```

### Query with filters

MongoDB-style operators: `$eq` (default), `$ne`, `$gt`, `$gte`, `$lt`, `$lte`, `$in`, `$nin`, `$like`, `$between`, plus top-level `$and` / `$or`. See [ADR sdk_api](adr/) for the full operator list.

```python
results = await scope.query(
    schema_pb2.Task,
    filter={"status": {"$in": ["todo", "doing"]}, "priority": {"$gte": 5}},
    limit=100,
)
```

```go
results, err := entdb.Query[*myschema.Task](ctx, scope, map[string]any{
    "status":   map[string]any{"$in": []string{"todo", "doing"}},
    "priority": map[string]any{"$gte": 5},
}, entdb.WithLimit(100))
```

### Edges

```python
# Outgoing edges from a node
edges = await scope.edges_out(node_id, edge_type=100)
# Incoming edges to a node
edges = await scope.edges_in(node_id, edge_type=100)
```

```go
out, _ := scope.EdgesOut(ctx, nodeID, 100)
in_,  _ := scope.EdgesIn(ctx, nodeID, 100)
```

## ACL and sharing

```python
# Grant
await scope.share(node_id, grantee="user:bob", caps=["CORE_CAP_READ"])
# Grant a per-type extension capability
await scope.share(node_id, grantee="user:bob",
                  caps=["CORE_CAP_READ"], ext_caps=[PRExt.APPROVE])

# Revoke
await scope.revoke(node_id, grantee="user:bob")

# Nodes shared with the current actor (cross-tenant included)
nodes = await scope.shared_with_me()

# Group membership (groups are nodes in the tenant)
await scope.group_add(group_id, member_id, role="member")
await scope.group_remove(group_id, member_id)
```

```go
err := scope.Share(ctx, nodeID, "user:bob", []entdb.CoreCap{entdb.CoreCapRead})
err = scope.Revoke(ctx, nodeID, "user:bob")
nodes, err := scope.SharedWithMe(ctx)
err = scope.GroupAdd(ctx, groupID, memberID, "member")
err = scope.GroupRemove(ctx, groupID, memberID)
```

See [ADR-003](adr/003-acl-model.md) for the full ACL model.

## Admin operations

These cross tenant boundaries and require an admin / system actor (`admin:`, `system:`, or `__system__`).

```python
await client.create_user(
    user_id="alice", email="alice@example.com", name="Alice",
    actor="system:admin",
)
await client.create_tenant(
    tenant_id="acme", name="Acme Corp",
    actor="system:admin",
)
await client.add_tenant_member(
    tenant_id="acme", user_id="alice", role="member",
    actor="system:admin",
)
```

```go
admin := client.Admin()
_, err := admin.CreateUser(ctx, "system:admin", "alice", "alice@example.com", "Alice")
_, err  = admin.CreateTenant(ctx, "system:admin", "acme", "Acme Corp")
err     = admin.AddTenantMember(ctx, "system:admin", "acme", "alice", "member")
```

See [Onboarding](onboarding.md) for the full three-RPC bootstrap and admin-actor production wiring.

## GDPR

```python
# Schedule a user deletion (grace period configurable)
await client.delete_user(user_id="alice", grace_days=30, actor="system:admin")
# Cancel a pending deletion
await client.cancel_user_deletion(user_id="alice", actor="system:admin")
# Freeze (block all writes; keep reads)
await client.freeze_user(user_id="alice", actor="system:admin")
# Export the user's data (GDPR Article 20 portability)
data = await client.export_user_data(user_id="alice", actor="system:admin")
```

Crypto-shred ([ADR-011](adr/011-security-and-compliance.md)) executes at the grace period expiry: the tenant key is wiped from `tenant_key_vault`, making the user's data unreadable on disk and on the S3 archive.

## Error handling

Both SDKs surface typed errors derived from gRPC status codes:

| Status code | Python class | Go type |
|---|---|---|
| `NOT_FOUND` | `NotFoundError` | `entdb.ErrNotFound` |
| `ALREADY_EXISTS` | `AlreadyExistsError` | `entdb.ErrAlreadyExists` |
| `PERMISSION_DENIED` | `PermissionDeniedError` | `entdb.ErrPermissionDenied` |
| `INVALID_ARGUMENT` | `InvalidArgumentError` | `entdb.ErrInvalidArgument` |
| `FAILED_PRECONDITION` | `PreconditionFailedError` | `entdb.ErrPreconditionFailed` |
| `UNAVAILABLE` | `UnavailableError` | `entdb.ErrUnavailable` |
| `UNAUTHENTICATED` | `UnauthenticatedError` | `entdb.ErrUnauthenticated` |

```python
from entdb_sdk.errors import NotFoundError, PreconditionFailedError

try:
    node = await scope.get(schema_pb2.User, node_id)
except NotFoundError:
    print("Node not found")
except PreconditionFailedError as e:
    print(f"CAS failed: {e}")
```

```go
node, err := entdb.Get[*myschema.User](ctx, scope, nodeID)
if errors.Is(err, entdb.ErrNotFound) {
    // ...
}
```

## Read-after-write

Default reads return whatever the SQLite materialized view has at request time — eventually consistent. For strict read-your-write:

**Python**: `wait_applied=True` on the `commit()` call blocks the response until the applier has materialized the write. Alternatively, capture the `stream_position` from the receipt and pass it on the next read.

**Go**: `entdb.WithWaitApplied(true)` on `Commit`, or `entdb.WithAfterOffset(receipt.StreamPosition)` on the next read.

## Connection and offset state

Both SDKs internally track the highest `stream_position` they've seen on writes so subsequent reads can wait for at-least-that-offset. This is automatic; only relevant when debugging cross-SDK-instance read-after-write issues.

## Console (operator tool, not an SDK)

For interactive data browsing or one-off writes against a tenant, use the [EntDB Console](../sdk/go/entdb/cmd/entdb-console/) at <http://localhost:8080> in the compose stack. The console is a single Go binary with an embedded React SPA; it talks to the same gRPC API the SDKs use.

## Further reading

- [Onboarding](onboarding.md) — required setup for any tenant beyond the local `playground`
- [Schema Evolution](schema-evolution.md) — adding/deprecating fields safely
- [Durability Guarantees](durability.md) — what `wait_applied` actually waits for
- [ADR-006](adr/006-proto-schema-definition.md) — proto is the type system end-to-end
- [ADR-003](adr/003-acl-model.md) — typed-capability ACL
- [ADR-016](adr/016-handlers-append-applier-writes.md) — write-path semantics
- [api-reference.md](api-reference.md) — wire-level gRPC contract (SDK internal)
