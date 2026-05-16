# EntDB Go server

The canonical EntDB gRPC server. All 44 RPCs in `proto/entdb/v1/entdb.proto`
are implemented here. The historical Python server (`server/python/`) was
retired in EPIC #407 Phase 4D (commit `8d07f5f`); see
`docs/decisions/python-server-retired.md` for the retirement evidence.

## Layout

```
server/go/
  cmd/entdb-server/        — main package (binary entry point)
  internal/
    api/                   — gRPC service implementation (44 RPCs)
    apply/                 — WAL consumer ("Applier")
    auth/                  — auth interceptor + trusted-actor plumbing
    acl/                   — typed-capability ACL
    globalstore/           — cross-tenant SQLite (global.db)
    store/                 — per-tenant SQLite (canonical store)
    payload/               — id-keyed payload translation
    pb/                    — generated protobuf + gRPC stubs for entdb.v1 (checked in)
      consolev1/           — generated stubs for console.v1 (checked in)
    schema/                — node/edge type registry
    wal/                   — WAL producer/consumer (in-memory + Kafka backends)
    errs/                  — gRPC status mapping
    testseed/              — test fixture seeding
  buf.gen.yaml             — proto codegen template (used by `buf generate`)
  go.mod                   — module: github.com/elloloop/tenant-shard-db/server/go
```

The server module is intentionally **separate from `sdk/go/entdb`**
so SDK consumers don't transitively pull server-only dependencies.
The two modules each carry their own checked-in copy of the proto
stubs; the wire format (`proto/entdb/v1/entdb.proto`) is the contract
they share.

## Build / run

```bash
cd server/go
go build ./...
go run ./cmd/entdb-server --data-dir /tmp/entdb --addr :50051
```

## Test

```bash
cd server/go
go vet ./...
go test ./...
```

CI runs the same as the `go-server-build` job.

## Regenerating proto stubs

```bash
# from server/go
go generate ./...
```

This runs both buf templates from the repository root:

```bash
buf generate --template server/go/buf.gen.yaml          # entdb.v1
buf generate --template server/go/buf.gen.console.yaml  # console.v1
```

`entdb.proto` lands in `server/go/internal/pb/` (package `pb`);
`console.proto` lands in `server/go/internal/pb/consolev1/` (package
`consolev1`) and re-uses the entdb message types via the
`Mentdb.proto=…` plugin opt. The generated files are checked in —
regenerate only when the proto changes, and commit the diff. CI's
`Go Server Proto Drift Guard` job re-runs the same templates on
every PR and fails if the working tree differs from what's committed.

## What lives here vs. what doesn't

- **Lives here:** the gRPC server, WAL producer + consumer (applier),
  per-tenant SQLite store, global store, auth, audit, encryption,
  snapshot/recovery.
- **Doesn't live here:** the Go SDK (`sdk/go/entdb/`), the console
  service (`proto/console/v1/`).
