# EntDB Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/elloloop/tenant-shard-db/sdk/go/entdb.svg)](https://pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb)

Official Go client for **EntDB** — a multi-tenant, event-sourced graph database with built-in ACL, GDPR, and compliance features.

## Install

```bash
go get github.com/elloloop/tenant-shard-db/sdk/go/entdb@latest
```

Requires Go 1.22+.

## Quickstart

```go
package main

import (
    "context"
    "log"

    "github.com/elloloop/tenant-shard-db/sdk/go/entdb"
)

func main() {
    ctx := context.Background()

    client, err := entdb.NewClient("localhost:50051",
        entdb.WithAPIKey("sk-..."),
        entdb.WithMaxRetries(3),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }

    // Typed actor — no more raw "user:bob" strings
    bob := entdb.UserActor("bob")
    alice := client.Tenant("acme").Actor(bob)

    // Atomic plan — create + edge in one commit
    plan := alice.Plan()
    taskAlias := plan.Create(101, map[string]any{
        "title":  "Ship Go SDK v0.2.0",
        "status": "todo",
    })
    plan.CreateEdge(201, taskAlias, "user:charlie")

    result, err := plan.Commit(ctx)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("created: %v", result.CreatedNodeIDs)
}
```

## Core Concepts

### Client

`entdb.NewClient(address, opts...)` creates a client. The client is not connected until `Connect()` is called.

```go
client, _ := entdb.NewClient("api.example.com:443",
    entdb.WithSecure(),               // TLS
    entdb.WithAPIKey("sk-..."),       // API key auth
    entdb.WithMaxRetries(5),          // retry transient failures
    entdb.WithTimeout(30*time.Second),// per-call timeout
)
defer client.Close()

if err := client.Connect(ctx); err != nil {
    log.Fatal(err)
}
```

### Typed Actor

All principals use the `Actor` type instead of raw strings. This prevents typos and makes it clear what kind of principal you're dealing with.

```go
bob := entdb.UserActor("bob")           // user:bob
admins := entdb.GroupActor("admins")    // group:admins
api := entdb.ServiceActor("ingestion")  // service:ingestion

// Parse a wire-format string back into an Actor
a, err := entdb.ParseActor("user:alice")

// Inspect
fmt.Println(a.Kind())   // "user"
fmt.Println(a.ID())     // "alice"
fmt.Println(a.String()) // "user:alice"
```

### Permissions

`Permission` is a string-based enum — use the constants, not raw strings.

```go
entdb.PermissionRead
entdb.PermissionWrite
entdb.PermissionAdmin
```

### Hierarchical Scope API

Binding tenant + actor once eliminates boilerplate on every call:

```go
// Instead of this:
client.Get(ctx, "acme", "user:bob", 101, "task-1")
client.Query(ctx, "acme", "user:bob", 101, filter)
client.EdgesFrom(ctx, "acme", "user:bob", "task-1", 201)

// Do this:
scope := client.Tenant("acme").Actor(entdb.UserActor("bob"))
scope.Get(ctx, 101, "task-1")
scope.Query(ctx, 101, filter)
scope.EdgesFrom(ctx, "task-1", 201)
scope.Share(ctx, "task-1", entdb.UserActor("charlie"), entdb.PermissionWrite)
```

## Reading Data

### Get a single node

```go
task, err := alice.Get(ctx, 101, "task-1")
if err != nil {
    var notFound *entdb.NotFoundError
    if errors.As(err, &notFound) {
        log.Printf("not found: %s", notFound.ResourceID)
        return
    }
    log.Fatal(err)
}
log.Printf("title=%v", task.Payload["title"])
```

### Query nodes by filter

```go
tasks, err := alice.Query(ctx, 101, map[string]any{
    "status": "todo",
})
for _, t := range tasks {
    fmt.Println(t.NodeID, t.Payload)
}
```

### Edge traversal

```go
// Outgoing edges (who does this task point to?)
assignees, err := alice.EdgesFrom(ctx, "task-1", 201)

// Incoming edges (what tasks point at this user?)
assigned, err := alice.EdgesTo(ctx, "user:charlie", 201)
```

## Writing Data

### Atomic plans

A `Plan` batches operations into a single atomic commit. If any operation fails, none are applied.

```go
plan := alice.Plan()

// Create a node, capture its alias for edge references
taskAlias := plan.Create(101, map[string]any{
    "title":  "Design review",
    "status": "todo",
})

// Create an edge using the alias
plan.CreateEdge(201, taskAlias, "user:charlie")

// Update another node
plan.Update("existing-task-id", 101, map[string]any{
    "status": "in_progress",
})

// Delete
plan.Delete("old-task-id")

// Commit
result, err := plan.Commit(ctx)
if err != nil {
    log.Fatal(err)
}
```

A Plan **cannot be reused** after `Commit`. Create a new Plan for subsequent operations.

### Create with ACL

```go
plan := alice.Plan()

acl := []entdb.ACLEntry{
    {Grantee: entdb.UserActor("charlie"), Permission: entdb.PermissionRead},
    {Grantee: entdb.GroupActor("reviewers"), Permission: entdb.PermissionWrite},
}

plan.CreateWithACL(101, map[string]any{
    "title": "Sensitive doc",
}, acl)

_, err := plan.Commit(ctx)
```

### Idempotency keys

Pass an idempotency key to make a commit safely retryable:

```go
plan := client.NewPlanWithKey("acme", "user:bob", "order-12345")
plan.Create(101, map[string]any{"amount": 99.99})
result, err := plan.Commit(ctx)
// Retrying with the same key returns the same result instead of
// creating a duplicate node.
```

## Sharing and ACL

### Grant access

```go
err := alice.Share(ctx, "task-1",
    entdb.UserActor("charlie"),
    entdb.PermissionWrite,
)
```

### Revoke access

Use `Plan` with a delete-ACL operation (via the raw API), or let ACL entries expire naturally.

## Error Handling

All errors implement `error`. Use `errors.As` to check for specific types:

```go
result, err := plan.Commit(ctx)
if err != nil {
    var valErr *entdb.ValidationError
    var accessErr *entdb.AccessDeniedError
    var txErr *entdb.TransactionError

    switch {
    case errors.As(err, &valErr):
        log.Printf("validation failed: %s (field=%s)", valErr.Message, valErr.Field)
    case errors.As(err, &accessErr):
        log.Printf("access denied for %s on %s", accessErr.Actor, accessErr.ResourceID)
    case errors.As(err, &txErr):
        log.Printf("transaction conflict (key=%s)", txErr.IdempotencyKey)
    default:
        log.Printf("unexpected: %v", err)
    }
}
```

Error types:

| Type | When |
|------|------|
| `ConnectionError` | Cannot reach server |
| `ValidationError` | Payload failed schema validation |
| `NotFoundError` | Node/edge/tenant doesn't exist |
| `AccessDeniedError` | ACL denied the operation |
| `TransactionError` | Idempotency conflict or atomic commit failure |
| `SchemaError` | Schema fingerprint mismatch (stale client) |

## Flat API

The hierarchical scope API (`client.Tenant().Actor()`) is the recommended entry point. For cases where you need to operate on multiple tenants from one call site, use the flat API:

```go
client.Get(ctx, tenantID, actor, typeID, nodeID)
client.Query(ctx, tenantID, actor, typeID, filter)
client.EdgesFrom(ctx, tenantID, actor, nodeID, edgeTypeID)
client.EdgesTo(ctx, tenantID, actor, nodeID, edgeTypeID)
client.NewPlan(tenantID, actor)
client.NewPlanWithKey(tenantID, actor, idempotencyKey)
```

Both APIs share the same `Plan` type once you have one.

## CLI

Install the `entdb` command-line tool:

```bash
go install github.com/elloloop/tenant-shard-db/sdk/go/entdb/cmd/entdb@latest
```

Commands:

```bash
entdb version                                # print version
entdb help                                   # usage
entdb lint schema.proto                      # validate proto has (entdb.node) annotations
entdb check schema.proto                     # protoc compile + list fields
entdb ping localhost:50051 --api-key=sk-... # connectivity check
entdb get localhost:50051 --tenant=acme --actor=user:bob --type-id=101 --node-id=task-1
entdb query localhost:50051 --tenant=acme --actor=user:bob --type-id=101
```

## Full Example

```go
package main

import (
    "context"
    "errors"
    "log"
    "time"

    "github.com/elloloop/tenant-shard-db/sdk/go/entdb"
)

const (
    TaskType    = 101
    CommentType = 102
    OnEdge      = 201
    ByEdge      = 202
)

func main() {
    ctx := context.Background()

    client, err := entdb.NewClient("api.example.com:443",
        entdb.WithSecure(),
        entdb.WithAPIKey("sk-prod-..."),
        entdb.WithTimeout(10*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }

    alice := client.Tenant("acme").Actor(entdb.UserActor("alice"))

    // 1. Create a task + first comment in one atomic plan
    plan := alice.Plan()
    taskAlias := plan.Create(TaskType, map[string]any{
        "title":  "Review Go SDK",
        "status": "todo",
    })
    commentAlias := plan.Create(CommentType, map[string]any{
        "body": "Looks great — needs more examples",
    })
    plan.CreateEdge(OnEdge, commentAlias, taskAlias)
    plan.CreateEdge(ByEdge, commentAlias, "user:alice")

    result, err := plan.Commit(ctx)
    if err != nil {
        var valErr *entdb.ValidationError
        if errors.As(err, &valErr) {
            log.Fatalf("validation failed: %s", valErr.Message)
        }
        log.Fatal(err)
    }
    taskID := result.CreatedNodeIDs[0]
    log.Printf("task created: %s", taskID)

    // 2. Share with Charlie
    _ = alice.Share(ctx, taskID,
        entdb.UserActor("charlie"),
        entdb.PermissionWrite,
    )

    // 3. Query all open tasks
    tasks, _ := alice.Query(ctx, TaskType, map[string]any{"status": "todo"})
    log.Printf("open tasks: %d", len(tasks))

    // 4. Find all comments on this task
    comments, _ := alice.EdgesTo(ctx, taskID, OnEdge)
    log.Printf("comments: %d", len(comments))

    // 5. Update task status
    plan2 := alice.Plan()
    plan2.Update(taskID, TaskType, map[string]any{"status": "done"})
    _, _ = plan2.Commit(ctx)
}
```

## Compatibility with Python SDK

The Go SDK mirrors the Python SDK's API surface so teams with both can stay consistent:

| Python                            | Go                                    |
|-----------------------------------|---------------------------------------|
| `db.tenant("t").actor("u:bob")`   | `client.Tenant("t").Actor(UserActor("bob"))` |
| `Actor.user("bob")`               | `UserActor("bob")`                    |
| `Permission.WRITE`                | `PermissionWrite`                     |
| `ACLEntry(grantee=..., perm=...)` | `ACLEntry{Grantee: ..., Permission: ...}` |
| `alice.get(Task, "t1")`           | `alice.Get(ctx, 101, "t1")`           |
| `alice.query(Task, filter={...})` | `alice.Query(ctx, 101, map[string]any{...})` |
| `alice.edges_out("t1", OnEdge)`   | `alice.EdgesFrom(ctx, "t1", 201)`     |
| `alice.plan().create(task)`       | `plan := alice.Plan(); plan.Create(101, data)` |

## Links

- [Full docs](https://elloloop.github.io/tenant-shard-db/)
- [Python SDK](https://pypi.org/project/entdb-sdk/)
- [pkg.go.dev reference](https://pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb)
- [GitHub](https://github.com/elloloop/tenant-shard-db)

## License

MIT
