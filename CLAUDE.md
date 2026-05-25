# EntDB — Agent Instructions

> **Scope of this file (per [ADR-019](docs/adr/019-decision-records-home.md)).**
> CLAUDE.md is agent execution rules only: workflow, release process,
> directory map, testing commands, code-style hints, pointers to where
> things live. Design decisions live in `docs/adr/` and are referenced
> here by number, never restated. When CLAUDE.md and an ADR disagree,
> the ADR wins.

> Server is Go-only as of EPIC #407 Phase 4D; the historical Python server has
> been deleted. See [ADR-017](docs/adr/017-python-server-retired.md) for the
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
1. Builds + pushes the Go server Docker image. **Note the tag normalization**: the git tag is `vX.Y.Z`, but `docker/metadata-action` strips the leading `v`, so the image is pulled as `ghcr.io/elloloop/tenant-shard-db:X.Y.Z` (also `:X.Y`, `:X`, `:latest`, `:sha-<7>`). Multi-arch (linux/amd64 + linux/arm64).
2. Publishes Python SDK to PyPI (`pip install entdb-sdk==X.Y.Z`).
3. Publishes Go SDK by tagging `sdk/go/entdb/vX.Y.Z` and warming the Go module proxy. Consumers install with `go get github.com/elloloop/tenant-shard-db/sdk/go/entdb@vX.Y.Z`. Docs auto-render at `pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb`.

Go modules don't need a registry — the Go proxy pulls directly from git tags. Sub-module tags MUST be prefixed `sdk/go/entdb/vX.Y.Z` (the release workflow creates them automatically).

## Architecture decisions

CLAUDE.md does not embed design decisions — they live in `docs/adr/`
per [ADR-019](docs/adr/019-decision-records-home.md). The current
in-force ADRs are listed in `docs/adr/`; read them when you need
design context.

Quick orientation (one-liners, not normative — the ADR is normative):

- [ADR-001](docs/adr/001-storage-architecture.md) — per-tenant SQLite as the tenant data boundary (physical layout owned by ADR-014)
- [ADR-003](docs/adr/003-acl-model.md) — ACL model: typed capabilities (CoreCapability + per-type extensions), inheritance, cross-tenant grants
- [ADR-005](docs/adr/005-event-sourcing-wal.md) — event sourcing via single WAL topic, swappable backends (memory + kafka shipped; #518 ports the rest)
- [ADR-006](docs/adr/006-proto-schema-definition.md) — proto is the type system end-to-end
- [ADR-011](docs/adr/011-security-and-compliance.md) — security + compliance: SQLCipher encryption-at-rest, KMS-backed key vault, crypto-shred GDPR erasure, TLS 1.3 + mTLS (all shipped in v1.13.0)
- [ADR-014](docs/adr/014-physical-storage-layout.md) — physical file layout: per-tenant, global, mailbox, public; scale, mobility, public.db semantics
- [ADR-015](docs/adr/015-wal-and-s3-object-lock-as-audit-log.md) — WAL + S3 Object Lock is the audit log
- [ADR-016](docs/adr/016-handlers-append-applier-writes.md) — handlers append to the WAL; only the applier writes SQLite
- [ADR-017](docs/adr/017-python-server-retired.md) — Python server retired in EPIC #407 Phase 4D
- [ADR-018](docs/adr/018-field-id-keyed-payloads.md) — payloads keyed by `field_id` on wire and disk; proto field number IS the `field_id`
- [ADR-019](docs/adr/019-decision-records-home.md) — `docs/adr/` is the only home for design decisions; CLAUDE.md is execution-only
- [ADR-020](docs/adr/020-immutable-storage-mode.md) — `storage_mode` (TENANT / USER_MAILBOX / PUBLIC) is immutable; no built-in publish/move primitive
- [ADR-021](docs/adr/021-go-console-binary.md) — single `entdb-console` Go binary with embedded React SPA (replaced the FastAPI console + playground)
- [ADR-022](docs/adr/022-fts5-full-text-search.md) — SQLite FTS5 backs `(entdb.field).searchable = true`
- [ADR-023](docs/adr/023-declarative-query-indexes.md) — non-unique expression indexes declared via `(entdb.field).indexed = true`
- [ADR-024](docs/adr/024-three-layer-rate-limit-model.md) — three-layer rate-limit model; Phase 1 (monthly quotas) is the implementation start
- [ADR-025](docs/adr/025-single-shape-sdk-api.md) — single-shape SDK API (proto messages everywhere, typed unique-key tokens via codegen, expression-index uniqueness)
- [ADR-026](docs/adr/026-per-tenant-read-write-connection-split.md) — per-tenant read/write SQLite connection split (Accepted; landed dark — split off by default `--read-pool-size=1`; the post-commit broadcast fix is always active)
- [ADR-027](docs/adr/027-parallel-wal-apply-across-tenants.md) — parallel WAL apply across distinct tenants; amends ADR-016's no-fan-out clause (Accepted; landed dark — serial by default `--apply-concurrency=1`)
- [ADR-028](docs/adr/028-typed-payload-wire-values.md) — typed `field_id`-keyed payload wire values (`map<uint32, EntValue>`), retiring `google.protobuf.Struct` so int64 stops corrupting >2^53 (Accepted; design frozen, implementation pending — Bug C)
- [ADR-029](docs/adr/029-keyset-cursor-pagination.md) — keyset cursor pagination (`page_token`/`next_page_token`) with SDK auto-follow, replacing the silent 100-row read truncation (Accepted; design frozen, implementation pending — Bug A)
- [ADR-030](docs/adr/030-composite-unique-constraints.md) — composite (multi-field) unique constraints via `(entdb.node).composite_unique`; tuple expression index enforced in the applier; `ALREADY_EXISTS` structured detail parsed into the typed `UniqueConstraintError` (Accepted; implemented — issue #566)
- [ADR-031](docs/adr/031-self-describing-name-free-schema.md) — self-describing, name-free schema: schema rides ExecuteAtomic as a `register_schema` WAL op materialized by the applier (deterministic replay); ids at rest and on the wire (no field/type/constraint names); empty boot; establish-or-reject (Accepted; implemented)
- [ADR-032](docs/adr/032-schema-evolution-compat-rules.md) — schema-evolution compat rules (loosening safe / tightening breaking) + a customer-runnable buf-breaking-style gate (`entdb-schema breaking`); reserved-id tombstones make removal safe and reuse breaking; shipped as ghcr image + release binary (Accepted)

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
- The schema registry (`server/go/internal/schema/`) holds node/edge type definitions; it's populated at server boot from `.schema-snapshot.json` (loaded via `server/go/internal/schema/loader.go`). The SDK maintains its own client-side registry via `register_proto_schema(my_pb2)`. There is no `RegisterSchema` RPC. Read-only `GetSchema` exposes the server's current registry.
- GDPR: `user_id` (e.g. `"alice"`) vs `tenant_principal` (e.g. `"user:alice"`) — translate at the gRPC boundary, never deeper.
- ACL grants use the `Permission` enum, not raw strings.
- Actors use `Actor.user("bob")` / `Actor.group("admins")`, not `"user:bob"` strings.
