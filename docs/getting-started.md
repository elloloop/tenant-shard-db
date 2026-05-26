# Getting Started with EntDB

This guide takes you from `git clone` to a running EntDB stack and your first atomic transaction.

## Prerequisites

- Docker and Docker Compose
- Python 3.10+ (for the Python SDK) **or** Go 1.22+ (for the Go SDK)
- `protoc` (Protocol Buffers compiler) — required for generating the typed stubs your app uses
- (Optional) `grpcurl` for poking the wire directly during debugging

## 1. Start the local stack

```bash
git clone https://github.com/elloloop/tenant-shard-db.git
cd tenant-shard-db

docker compose up -d
docker compose ps
```

`docker compose up -d` brings up:

| Service | Port | Notes |
|---------|------|-------|
| `server` | `50051` (gRPC) | The EntDB server. **gRPC only — no HTTP on this port.** Seeded with a `playground` tenant. |
| `entdb-console` | `8080` (HTTP) | Browser UI: data browser + sandbox writes against the `playground` tenant. |
| `redpanda` | `9092` (Kafka) | WAL backend (Kafka-compatible). |
| `redpanda-console` | `8083` (HTTP) | Kafka UI for inspecting WAL events. |
| `minio` | `9001` (HTTP) | S3-compatible object store for snapshots/archive. Login `minioadmin` / `minioadmin`. |

Verify the server is healthy via gRPC:

```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
# {"status": "SERVING"}
```

Browser tools:

- EntDB Console — <http://localhost:8080>
- Redpanda Console (Kafka UI) — <http://localhost:8083>
- MinIO Console (S3 UI) — <http://localhost:9001>

## 2. Install an SDK

```bash
pip install entdb-sdk                                              # Python
go get github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2@latest     # Go
```

Both SDKs talk to `localhost:50051` over gRPC.

## 3. Define a schema in `.proto`

EntDB types are proto messages decorated with EntDB-specific options ([ADR-006](adr/006-proto-schema-definition.md)). The proto field number IS the on-disk `field_id` ([ADR-018](adr/018-field-id-keyed-payloads.md)) — renames are free, removals retire the number.

`schema.proto`:

```protobuf
syntax = "proto3";

import "entdb/v1/entdb_options.proto";

package myapp;

message User {
  option (entdb.node) = { type_id: 1 };

  string email = 1 [(entdb.field) = { required: true, unique: true }];
  string name  = 2;
  int64  created_at = 3 [(entdb.field) = { kind: "timestamp" }];
}

message Task {
  option (entdb.node) = { type_id: 2 };

  string title = 1 [(entdb.field) = { required: true }];
  string description = 2;
  string status = 3 [(entdb.field) = { enum_values: "todo,doing,done" }];
  int64  due_date = 4 [(entdb.field) = { kind: "timestamp" }];
}

message AssignedTo {
  option (entdb.edge) = { edge_id: 100 };

  Task from = 15;   // edge from
  User to   = 16;   // edge to
}
```

Generate stubs:

```bash
protoc --python_out=. --go_out=. -I. -I/path/to/entdb-protos schema.proto
```

(The `-I/path/to/entdb-protos` is where `entdb/v1/entdb_options.proto` is found — shipped with the SDK; see `proto/entdb/v1/`.)

## 4. Onboard a tenant and user

> **The compose stack pre-creates a `playground` tenant** for the quick-start. If you only want to drive the playground, skip to step 5.

For any other tenant, EntDB requires explicit onboarding: the server does not auto-create tenants, users, or memberships. See [Onboarding](onboarding.md) for the full contract. The minimum is three admin RPCs:

```python
import asyncio
from entdb_sdk import DbClient

async def onboard():
    admin = DbClient(
        endpoint="localhost:50051",
        tenant_id="_admin",       # placeholder; admin RPCs ignore it
        actor="system:admin",
    )
    await admin.connect()

    await admin.create_user(
        user_id="alice", email="alice@example.com", name="Alice",
        actor="system:admin",
    )
    await admin.create_tenant(
        tenant_id="my_company", name="My Company", actor="system:admin",
    )
    await admin.add_tenant_member(
        tenant_id="my_company", user_id="alice", role="member",
        actor="system:admin",
    )
    await admin.close()

asyncio.run(onboard())
```

> In local dev (no TLS configured), the wire-level `actor` field is trusted. In production, the actor is taken from the verified OAuth / API-key / mTLS client identity — see [ADR-011](adr/011-security-and-compliance.md) and [Onboarding § Admin/system actors in production](onboarding.md#adminsystem-actors-in-production).

## 5. Write data

**Python:**

```python
import asyncio
from entdb_sdk import DbClient
from entdb_sdk.codegen import register_proto_schema
import schema_pb2

register_proto_schema(schema_pb2)

async def main():
    async with DbClient(
        endpoint="localhost:50051",
        tenant_id="playground",   # or "my_company" after onboarding
        actor="user:demo",        # or "user:alice"
    ) as client:
        result = await client.atomic(lambda plan: (
            plan.create(schema_pb2.User(email="alice@example.com", name="Alice"), as_="alice"),
            plan.create(schema_pb2.Task(title="Review PR #123", status="todo"), as_="task"),
            plan.edge_create(schema_pb2.AssignedTo, "$task.id", "$alice.id"),
        ))
        print(result)

        # Read back
        edges = await client.edges_in(result["alice"].id, edge_type=100)
        print(f"Tasks assigned to Alice: {len(edges)}")

asyncio.run(main())
```

**Go:**

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2"
    myschema "example.com/myapp/schema"
)

func main() {
    ctx := context.Background()
    client, err := entdb.NewClient("localhost:50051")
    if err != nil { log.Fatal(err) }
    if err := client.Connect(ctx); err != nil { log.Fatal(err) }
    defer client.Close()

    plan := client.NewPlan("playground", "user:demo")
    alice := plan.Create(&myschema.User{Email: "alice@example.com", Name: "Alice"})
    task  := plan.Create(&myschema.Task{Title: "Review PR #123", Status: "todo"})
    entdb.EdgeCreate[*myschema.AssignedTo](plan, task, alice)

    result, err := plan.Commit(ctx)
    if err != nil { log.Fatal(err) }
    fmt.Println(result)
}
```

The `$task.id` / `$alice.id` syntax (Python) and `plan.Create(...)` returning an alias-style token (Go) let a single plan reference nodes it creates in the same call — the server resolves aliases before persisting.

> **Bulk cleanup:** to delete every node matching a predicate in one
> round-trip (the TTL-sweeper pattern), use `plan.delete_where(...)` /
> `entdb.DeleteWhere[...]` instead of a query-then-delete loop. On a
> schema-less server, filter by the numeric payload field id (same
> rule as `QueryNodes` filters) — see [Plan methods](sdk-reference.md#plan-methods)
> and [Schema lockdown → `delete_where` on schema-less deployments](guides/schema-lockdown.md#delete_where-and-querynodes-on-schema-less-deployments)
> ([#545](https://github.com/elloloop/tenant-shard-db/issues/545)).

## 6. Inspect the data

- **EntDB Console** (<http://localhost:8080>): browse tenants, nodes, edges; run sandbox writes against `playground`.
- **Redpanda Console** (<http://localhost:8083>): inspect raw WAL events on the `entdb-wal` topic — this is the audit log per [ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md).
- **MinIO Console** (<http://localhost:9001>): browse archived events and snapshots when the archive sidecar is enabled.

## Troubleshooting

### `UNAVAILABLE: connection refused`

```bash
docker compose ps                          # all services Up?
docker compose logs server --tail 50       # server boot errors?
```

The Go server image is distroless; there's no shell inside the container. Use `docker compose logs server` rather than `docker exec`.

### `NOT_FOUND: tenant "<id>" not found`

You skipped onboarding. Either use the pre-seeded `playground` tenant or run the three admin RPCs from step 4. See [Onboarding](onboarding.md).

### `PERMISSION_DENIED: actor is not a member of "<tenant>"`

The wire `actor` is a *registered* user, but they aren't a member of the target tenant. Call `Admin.AddTenantMember` from a `system:` / `admin:` actor.

### Writes don't show up immediately on read

The applier is single-threaded per partition and asynchronous. For deterministic read-your-writes:

```python
await client.atomic(
    lambda plan: plan.create(schema_pb2.User(...)),
    wait_applied=True,
)
```

Equivalent in Go: pass `entdb.WithWaitApplied(true)` to `Commit`.

## Next steps

- [Onboarding](onboarding.md) — tenant, user, and membership setup
- [SDK Reference](sdk-reference.md) — full SDK surface, Go and Python
- [Schema Evolution](schema-evolution.md) — adding/deprecating fields safely
- [Durability Guarantees](durability.md) — the WAL contract
- [Deployment](deployment.md) — taking it to production (TLS, KMS, archive)
- [ADRs](adr/) — design rationale
