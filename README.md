# EntDB — Multi-Tenant, Event-Sourced Graph Database

> ## ⚠ Use an SDK — do not call gRPC directly
>
> The gRPC API is an **internal transport** between the server and the official SDKs. The wire format is not a stable public contract; field IDs, actor strings, ACL encoding, idempotency keys, schema fingerprints, and tenant routing all have non-obvious invariants the SDKs enforce.
>
> **Sanctioned clients:**
> - **Go:** `github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2` ([pkg.go.dev](https://pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2))
> - **Python:** `pip install entdb-sdk` ([PyPI](https://pypi.org/project/entdb-sdk/))
>
> Hand-rolled clients (curl, grpcurl-as-protocol, custom stubs) **will break** and are unsupported. If your language isn't covered, please [open an issue](https://github.com/elloloop/tenant-shard-db/issues) — don't reach for the proto.

EntDB is a multi-tenant, event-sourced database with a graph data model. Writes go through a Kafka/Redpanda (or Pub/Sub / SQS / Service Bus / Event Hubs / Kinesis — see [ADR-005](docs/adr/005-event-sourcing-wal.md)) WAL — the durable, tamper-evident audit log — and are materialized into per-tenant SQLite databases by a single applier. The server is written in Go ([Phase 4D retired the Python implementation](docs/adr/017-python-server-retired.md), v1.12+).

## Features

- **Event-sourced.** Every mutation is appended to the WAL before any state changes. SQLite is a derived view; it can be deleted and rebuilt by replay. See [ADR-016](docs/adr/016-handlers-append-applier-writes.md).
- **WAL + S3 Object Lock audit log.** WAL archived to S3 with Object Lock COMPLIANCE mode is the single audit log — no hash-chained `audit_log` table. See [ADR-015](docs/adr/015-wal-and-s3-object-lock-as-audit-log.md).
- **Per-tenant SQLite isolation.** Each tenant gets its own SQLite file ([ADR-001](docs/adr/001-storage-architecture.md), [ADR-014](docs/adr/014-physical-storage-layout.md)). No cross-tenant transactions.
- **Encryption-at-rest via SQLCipher.** Every per-tenant file, every per-user mailbox file, and `global.db` are AES-256 encrypted. Per-tenant keys derived from a KMS-managed master (AWS KMS, HashiCorp Vault, or local file shipped today; GCP KMS + Azure Key Vault flag-recognized but unimplemented — error at boot). Crypto-shred deletes the key → GDPR erasure even on immutable S3 archives. See [ADR-011](docs/adr/011-security-and-compliance.md).
- **TLS 1.3 + mTLS.** Production-grade transport with cert reload-on-SIGHUP, optional client-cert authentication mapped to actor identity.
- **Graph data model.** Typed nodes and unidirectional typed edges, keyed by stable numeric field IDs on disk (proto field numbers IS the field ID; renames are free). See [ADR-018](docs/adr/018-field-id-keyed-payloads.md).
- **Typed-capability ACL.** Principal-based grants (`user:X`, `group:X`, `tenant:X`) with typed `CoreCapability` + per-type extension capabilities. See [ADR-003](docs/adr/003-acl-model.md).
- **GDPR primitives.** `ExportUserData`, `DeleteUser`, `FreezeUser`, `TransferUserContent`, `CancelUserDeletion` as first-class RPCs.
- **Atomic transactions with aliases.** Multiple ops in one round-trip; `$alias.id` references let you wire newly-created nodes inside the same plan.
- **Two official SDKs.** Type-safe Go and Python; both pin the wire contract; both ship the same shape per [ADR-006](docs/adr/006-proto-schema-definition.md).

## Quick start

```bash
git clone https://github.com/elloloop/tenant-shard-db.git
cd tenant-shard-db

# Spin up the local stack: Go server, Redpanda, MinIO, EntDB Console
docker compose up -d

# Health-check the server (gRPC; server has no HTTP listener)
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```

The compose stack pre-seeds a `playground` tenant. The Console (browser data viewer + sandbox writes) is at <http://localhost:8080>.

Install an SDK:

```bash
pip install entdb-sdk                                              # Python
go get github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2@latest     # Go
```

## A 30-second taste

```python
import asyncio
from entdb_sdk import DbClient
from entdb_sdk.codegen import register_proto_schema
import schema_pb2  # generated from your schema.proto

register_proto_schema(schema_pb2)

async def main():
    async with DbClient(endpoint="localhost:50051",
                        tenant_id="playground",
                        actor="user:demo") as c:
        out = await c.atomic(lambda p: (
            p.create(schema_pb2.User(email="alice@example.com", name="Alice"), as_="alice"),
            p.create(schema_pb2.Task(title="Review PR", status="todo"), as_="task"),
            p.edge_create(schema_pb2.AssignedTo, "$task.id", "$alice.id"),
        ))
        print(out)

asyncio.run(main())
```

**First-time setup beyond the playground tenant** requires onboarding — the server does not auto-create tenants or users. See [Onboarding](docs/onboarding.md) for the three-RPC flow (`Admin.CreateUser` → `Admin.CreateTenant` → `Admin.AddTenantMember`).

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│             Clients (Python SDK / Go SDK)                   │
└─────────────────────────┬───────────────────────────────────┘
                          │ gRPC (TLS 1.3 / mTLS)
┌─────────────────────────▼───────────────────────────────────┐
│                    EntDB Server (Go)                        │
│                                                             │
│   gRPC handlers ──► WAL Producer ──► WAL (Kafka / Pub/Sub   │
│                                          / Service Bus /…)  │
│                                          │                  │
│                                          ▼                  │
│                                      Applier (single        │
│                                      consumer goroutine     │
│                                      per partition)         │
│                                          │                  │
│                                          ▼                  │
│   gRPC reads ◄── Per-tenant SQLite (SQLCipher AES-256)      │
│              ◄── Global SQLite (tenants, users, members,    │
│                                  quotas, deletion queue)    │
│                                                             │
│   Archiver ──► S3 Object Lock COMPLIANCE (immutable audit)  │
└─────────────────────────────────────────────────────────────┘
```

The [EntDB Console](sdk/go/entdb/cmd/entdb-console/) is a separate Go binary that bundles a React SPA — a read-only data browser plus a sandbox-write tab against one configured tenant.

## Documentation

- **Getting started** → [`docs/getting-started.md`](docs/getting-started.md)
- **Onboarding (must read for production)** → [`docs/onboarding.md`](docs/onboarding.md)
- **SDK reference (Go + Python)** → [`docs/sdk-reference.md`](docs/sdk-reference.md)
- **Deployment** → [`docs/deployment.md`](docs/deployment.md)
- **Operations** → [`docs/operations.md`](docs/operations.md)
- **Schema evolution** → [`docs/schema-evolution.md`](docs/schema-evolution.md)
- **Durability guarantees** → [`docs/durability.md`](docs/durability.md)
- **Design decisions (ADRs)** → [`docs/adr/`](docs/adr/)
- **API reference (wire-level, for SDK maintainers)** → [`docs/api-reference.md`](docs/api-reference.md)

## Project structure

```
proto/
  entdb/v1/entdb.proto         wire contract (44 RPCs)
  console/v1/console.proto     console-facing RPCs

server/go/                     Go server (AGPL-3.0-only)
  cmd/entdb-server/            main package
  cmd/entdb-schema/            schema-snapshot / check CLI
  internal/                    api, apply, auth, store, globalstore,
                               wal, acl, payload, schema, crypto,
                               gdpr, audit, ...

sdk/
  python/entdb_sdk/            Python SDK (MIT; PyPI: entdb-sdk)
  go/entdb/                    Go SDK (MIT)
  go/entdb/cmd/entdb-console/  Web console (Go + embedded React SPA)
  go/entdb/cmd/entdbctl/       Admin CLI

tests/python/
  integration/                 SDK contract suite (drives the Go server)
  e2e/                         Docker-stack e2e + TLS/encryption coverage
  benchmarks/                  pytest-benchmark suite

docs/                          ADRs + user-facing docs
docker-compose.yml             Local dev stack
Dockerfile                     Multi-stage build for server + console
```

## Development

```bash
# Go server: vet + tests (canonical CI gate)
cd server/go && go vet ./... && go test ./...

# Go SDK
cd sdk/go/entdb && go vet ./... && go test ./...

# SDK contract suite (Python pytest against the Go server)
python -m pytest tests/python/integration/ -q

# E2E (docker compose stack with Redpanda + MinIO; covers encryption, TLS, crash recovery)
bash tests/python/e2e/run-e2e.sh

# Lint / format
uvx ruff@0.15.7 check .
uvx ruff@0.15.7 format --check .
```

The integration and e2e suites both boot the Go server (`server/go/cmd/entdb-server` or via `docker-compose.e2e.yml`) and drive it through the Python SDK over gRPC.

## CI/CD

- **CI** (`.github/workflows/ci.yml`): lint, unit tests, integration tests, Docker build, e2e tests, security scan, schema compatibility check, proto drift guard.
- **Release** (`.github/workflows/release.yml`): on `vX.Y.Z` tag, builds and pushes the Go server Docker image (`ghcr.io/elloloop/tenant-shard-db:X.Y.Z` — note that `docker/metadata-action` strips the leading `v`), publishes the Python SDK to PyPI, and warms the Go module proxy for `sdk/go/entdb/vX.Y.Z`.

## Configuration

The Go server is configured by **CLI flags only** (no `ENTDB_*` env vars). Highlights:

| Flag | Default | Purpose |
|------|---------|---------|
| `-addr` | `:50051` | gRPC bind address |
| `-data-dir` | (unset) | Per-tenant SQLite + `global.db` directory |
| `-wal-backend` | `memory` | `memory` (tests) or `kafka` (prod); other backends in [#518](https://github.com/elloloop/tenant-shard-db/issues/518) |
| `-wal-topic` | `entdb-wal` | WAL topic |
| `-wal-brokers` | `localhost:9092` | Kafka bootstrap brokers |
| `-tls-cert`, `-tls-key`, `-tls-ca` | (unset) | TLS material |
| `-tls-min-version` | `1.3` | Minimum TLS version |
| `-require-tls`, `-require-client-cert` | `false` | Production-mode gates (mTLS) |
| `-kms-provider` | (unset) | `file` / `aws` / `vault` shipped today; `gcp` / `azure` are flag-recognized but not implemented (error at boot) |
| `-kms-key-id` | (unset) | Provider-specific identifier for the master key |
| `-encryption-required` | `false` | Refuse unencrypted files (production posture) |
| `-gdpr-worker-enabled` | `true` | Run the deletion-queue worker |
| `-gdpr-worker-interval` | `1m` | Worker scan cadence |
| `-crypto-shred-delete-files` | `false` | Remove `.db` files after key shred |
| `-archive-enabled` | `false` | S3 Object Lock WAL archive sidecar |
| `-archive-bucket`, `-archive-region`, `-archive-retention-days` | — | Archive config |

See `entdb-server -help` for the full list, and [`docs/deployment.md`](docs/deployment.md) for the production checklist.

## License

EntDB is dual-licensed. The split follows the standard open-source database pattern (MongoDB pre-2018, MariaDB): the server is copyleft so derivative servers stay open; the client SDKs are permissive so applications built on the SDKs aren't forced to be copyleft.

| Component | License | Path |
|---|---|---|
| **Server** (`entdb-server`) | **GNU AGPL v3** | `server/go/` |
| **Python SDK** (`entdb-sdk`) | **MIT** | `sdk/python/entdb_sdk/` |
| **Go SDK** | **MIT** | `sdk/go/entdb/` |
| **`entdbctl` CLI** | **MIT** | `sdk/go/entdb/cmd/entdbctl/` |
| **`entdb-console`** | **MIT** | `sdk/go/entdb/cmd/entdb-console/` |
| **`entdb-schema` CLI** | **AGPL v3** | `server/go/cmd/entdb-schema/` (built from the server) |

Practical implications:

- Running EntDB *server* in production is fine. Modifying the server and offering it as a network service (SaaS) requires you to publish your changes under AGPL.
- Building an application against `entdb-sdk` (Python) or `github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2` (Go) imposes **no** copyleft obligations on your application — it's MIT.
- Forking `entdbctl` or `entdb-console` is fine under MIT; attribution required.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). In short:

1. Fork the repository.
2. Create a feature branch.
3. Run `go vet ./... && go test ./...` (server + SDK), the Python integration suite, ruff, and `bash tests/python/e2e/run-e2e.sh` if your change touches the server or SDK boundaries.
4. Open a pull request — CI must be green before review.
