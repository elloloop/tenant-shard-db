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
    pb/                    — generated protobuf + gRPC stubs (checked in)
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
# from repo root
buf generate --template server/go/buf.gen.yaml proto/entdb/v1
```

This rewrites everything under `server/go/internal/pb/`. The
generated files are checked in — regenerate only when the proto
changes, and commit the diff.

## What lives here vs. what doesn't

- **Lives here:** the gRPC server, WAL producer + consumer (applier),
  per-tenant SQLite store, auth, audit, encryption, snapshot/recovery —
  everything currently under `server/python/entdb_server/`.
- **Doesn't live here:** the Go SDK (`sdk/go/entdb/`), the console
  service (`proto/console/v1/`), the Python server (`server/python/`).
