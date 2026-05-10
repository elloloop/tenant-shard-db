# EntDB Go server

Go reimplementation of the EntDB gRPC server. Tracked by [EPIC #407][epic].

This module is **Phase 0**: a runnable skeleton. Every RPC returns
`codes.Unimplemented`. Methods land one at a time as RPC sub-issues
([`label:kind/rpc`][rpc-issues]) are picked up.

[epic]: https://github.com/elloloop/tenant-shard-db/issues/407
[rpc-issues]: https://github.com/elloloop/tenant-shard-db/issues?q=is%3Aopen+label%3A%22kind%2Frpc%22

## Layout

```
server/go/
  cmd/entdb-server/        — main package (binary entry point)
  internal/
    api/                   — gRPC service implementation (EntDBService)
    apply/                 — WAL consumer ("Applier") — empty in Phase 0
    auth/                  — auth interceptor + trusted-actor plumbing — empty in Phase 0
    pb/                    — generated protobuf + gRPC stubs for entdb.v1 (checked in)
      consolev1/           — generated stubs for console.v1 (checked in)
    wal/                   — WAL producers + backends — empty in Phase 0
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
go run ./cmd/entdb-server --addr :50051
```

`grpcurl` against the server gets `Unimplemented` for every method
in Phase 0 — that's the expected baseline.

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
  per-tenant SQLite store, auth, audit, encryption, snapshot/recovery —
  everything currently under `server/python/entdb_server/`.
- **Doesn't live here:** the Go SDK (`sdk/go/entdb/`), the console
  service (`proto/console/v1/`), the Python server (`server/python/`).
