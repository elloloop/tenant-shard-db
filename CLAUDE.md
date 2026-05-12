# EntDB — Agent Instructions

> Server is Go-only as of EPIC #407 Phase 4D; the historical Python server has
> been deleted. See `docs/decisions/python-server-retired.md` for the
> retirement evidence ladder (contract parity, e2e parity, perf, release-image
> swap).

## Workflow (MUST follow)

### Always run CI locally before pushing
Before every `git push`, run the full local CI suite and fix any failures:

```bash
cd server/go && go vet ./... && go test ./...           # Go server (source of truth)
cd sdk/go/entdb && go vet ./... && go test ./...        # Go SDK
python -m pytest tests/python/integration/ -q           # SDK contract suite vs Go server
uvx ruff@0.15.7 check .                                 # lint must be clean
uvx ruff@0.15.7 format --check .                        # format must be clean
```

Do NOT push code that you haven't verified locally. Do NOT rely on GitHub CI to catch failures — fix them before push.

### Releases

Tagging `vX.Y.Z` on `main` triggers `.github/workflows/release.yml` which:
1. Builds + pushes the Go server Docker image to `ghcr.io/elloloop/tenant-shard-db:vX.Y.Z` (same image name as before; Go binary inside).
2. Publishes Python SDK to PyPI (`pip install entdb-sdk==X.Y.Z`).
3. Publishes Go SDK by tagging `sdk/go/entdb/vX.Y.Z` and warming the Go module proxy. Consumers install with `go get github.com/elloloop/tenant-shard-db/sdk/go/entdb@vX.Y.Z`. Docs auto-render at `pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb`.

Go modules don't need a registry — the Go proxy pulls directly from git tags. Sub-module tags MUST be prefixed `sdk/go/entdb/vX.Y.Z` (the release workflow creates them automatically).

## Architecture Invariants (MUST NOT violate)

### 1. All writes go through the WAL
EntDB is event-sourced. The WAL (Kafka/Redpanda, in-memory for tests) is the **source of truth**. SQLite is a materialized view rebuilt by replaying the WAL.

**Every mutation** — including admin ops, GDPR, transfers, revocations — MUST be appended to the WAL via `wal.Append` and applied by the `Applier` in `server/go/internal/apply/`. Direct SQLite writes from handlers bypass the event log and will be **silently lost** on rebuild.

```
CORRECT:   handler → wal.Append(event) → Applier.Apply() → store (SQLite)
WRONG:     handler → store.WriteSomething() (direct)
```

If you need a new operation type, add it to the `Event`/`Op` types in `server/go/internal/wal/event.go` and handle it in `server/go/internal/apply/applier.go`.

### 2. The WAL is the audit log
S3 Object Lock (COMPLIANCE mode) layered over the Kafka/Redpanda backend in `server/go/internal/wal/kafka.go` provides tamper-evident, immutable audit trails. Do NOT build separate audit log tables with hash chains — they're redundant and weaker than S3 Object Lock.

### 3. Single consumer goroutine for the applier
The applier runs as a single consumer goroutine per server (Python-parity ordering guarantee). A per-tenant worker pool is deferred — do NOT add ad-hoc per-tenant goroutines that fan out applies, as it breaks per-tenant offset ordering and rebuild determinism. gRPC handlers themselves are goroutine-per-request (Go-native); the invariant is only about the apply path.

### 4. Per-tenant SQLite isolation
Each tenant has its own SQLite file (via `modernc.org/sqlite`, managed by `server/go/internal/store/pool.go`). Never read/write across tenant boundaries in a single SQLite transaction. Cross-tenant operations go through `server/go/internal/globalstore/` (which has its own SQLite).

### 5. Proto is the type system
Standard `protoc-gen-go` / `protoc-gen-go-grpc` generates typed stubs into `server/go/internal/pb/`. Do NOT build custom codegen that reimplements what protobuf provides (enums, typed fields, message classes). Use `register_proto_schema()` in the SDK to register proto types with the SDK registry.

### 6. Field IDs, not field names, on disk
Payloads are stored keyed by `field_id` (e.g. `{"1": "value"}`), not by name. Translation happens at the gRPC boundary only (`server/go/internal/payload/`). This makes field renames free.

## Project Structure

```
proto/
  entdb/v1/entdb.proto        — wire contract (44 RPCs)
  console/v1/console.proto    — browser-facing console RPCs

server/
  go/                         — Go gRPC server (AGPL-3.0-only)
    cmd/entdb-server/         — main package
    internal/
      api/                    — gRPC handlers (44 RPCs)
      apply/                  — WAL consumer (Applier)
      auth/                   — trusted-actor interceptor
      globalstore/            — cross-tenant SQLite
      pb/                     — generated protobuf stubs
      schema/                 — node/edge type registry
      store/                  — per-tenant SQLite (canonical store)
      wal/                    — WAL producer/consumer
                                (in-memory + Kafka/Redpanda backends)
      acl/                    — typed-capability ACL
      payload/                — id-keyed payload translation
      errs/                   — gRPC status mapping
      testseed/               — test fixture seeding
    go.mod                    — module: github.com/elloloop/tenant-shard-db/server/go

sdk/
  python/entdb_sdk/           — Python SDK (MIT, PyPI: entdb-sdk)
  go/entdb/                   — Go SDK (module: github.com/elloloop/tenant-shard-db/sdk/go/entdb)

tests/
  python/                     — all integration/e2e/benchmarks driven through the SDK against the Go gRPC server
    integration/              — contract suite (~70 cases), per-RPC behavior
    e2e/                      — Docker-stack tests (22 cases) including crash recovery
    benchmarks/               — pytest-benchmark suite
  go/                         — (currently sparse; Go-side unit tests live in server/go/internal/<pkg>/*_test.go)

pyproject.toml                — workspace root, dev tooling only (no [project])
```

## Testing

```bash
cd server/go && go vet ./... && go test ./...           # Go server unit tests
cd sdk/go/entdb && go test ./...                        # Go SDK tests
python -m pytest tests/python/integration/ -q           # SDK contract suite (runs vs Go server via conftest harness)
bash tests/python/e2e/run-e2e.sh                        # Docker-stack e2e (22 cases)
uvx ruff@0.15.7 check .                                 # lint
uvx ruff@0.15.7 format --check .                        # format
```

`tests/python/integration/` and `tests/python/e2e/` both target the Go server — the Python conftest harness boots `server/go/cmd/entdb-server` (or a docker-compose stack for e2e) and drives it through the Python SDK over gRPC.

## Key Patterns

- Store methods accept `context.Context` as the first argument; the per-tenant SQLite pool is keyed by `tenant_id` (`server/go/internal/store/pool.go`); writes go through `BatchTxn` (`server/go/internal/store/txn.go`) so the applier can commit a multi-op event atomically.
- The schema registry (`server/go/internal/schema/`) holds node/edge type definitions — register via the schema RPCs; the SDK mirrors them through `register_proto_schema()`.
- GDPR: `user_id` (e.g. `"alice"`) vs `tenant_principal` (e.g. `"user:alice"`) — translate at the gRPC boundary, never deeper.
- ACL grants use the `Permission` enum, not raw strings.
- Actors use `Actor.user("bob")` / `Actor.group("admins")`, not `"user:bob"` strings.
