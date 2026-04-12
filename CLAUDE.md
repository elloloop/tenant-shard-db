# EntDB — Agent Instructions

## Workflow (MUST follow)

### Always run CI locally before pushing
Before every `git push`, run the full local CI suite and fix any failures:

```bash
python -m pytest tests/unit/ tests/integration/ -q   # all tests must pass
uvx ruff@0.15.7 check .                              # lint must be clean
uvx ruff@0.15.7 format --check .                     # format must be clean
cd sdk/go/entdb && go vet ./... && go test ./...      # Go SDK (if modified)
```

Do NOT push code that you haven't verified locally. Do NOT rely on GitHub CI to catch failures — fix them before push.

## Architecture Invariants (MUST NOT violate)

### 1. All writes go through the WAL
EntDB is event-sourced. The WAL (Kafka/Kinesis/S3) is the **source of truth**. SQLite is a materialized view rebuilt by replaying the WAL.

**Every mutation** — including admin ops, GDPR, transfers, revocations — MUST be appended to the WAL via `wal.append()` and applied by the `Applier`. Direct SQLite writes from handlers bypass the event log and will be **silently lost** on rebuild.

```
CORRECT:   handler → wal.append(event) → Applier.apply_event() → SQLite
WRONG:     handler → global_store.update_something() → SQLite (direct)
```

If you need a new operation type, add it to `TransactionEvent.ops` and handle it in `Applier.apply_event()`.

### 2. The WAL is the audit log
S3 Object Lock (COMPLIANCE mode) provides tamper-evident, immutable audit trails. Do NOT build separate audit log tables with hash chains — they're redundant and weaker than S3 Object Lock. Use `export_audit_trail()` from `dbaas/entdb_server/audit/compliance.py` for compliance exports.

### 3. Async-only on the gRPC server
The gRPC server uses `grpc.aio`. All interceptors, handlers, and middleware MUST be async (`grpc.aio.ServerInterceptor`, not `grpc.ServerInterceptor`). Never use `asyncio.run()` or `ThreadPoolExecutor` to bridge sync/async in request handlers — it destroys throughput.

### 4. Per-tenant SQLite isolation
Each tenant has its own SQLite file. Never read/write across tenant boundaries in a single SQLite transaction. Cross-tenant operations go through `global_store` (which has its own SQLite).

### 5. Proto is the type system
Standard `protoc --python_out` generates typed classes. Do NOT build custom codegen that reimplements what protobuf provides (enums, typed fields, message classes). Use `register_proto_schema()` to register proto types with the SDK registry.

### 6. Field IDs, not field names, on disk
Payloads are stored keyed by `field_id` (e.g. `{"1": "value"}`), not by name. Translation happens at the gRPC boundary only. This makes field renames free.

## Project Structure

```
dbaas/entdb_server/
  api/grpc_server.py     — gRPC handlers (ingress/egress boundary)
  apply/applier.py       — WAL consumer, applies events to SQLite
  apply/canonical_store.py — per-tenant SQLite operations
  global_store.py        — cross-tenant state (users, tenants, memberships)
  wal/                   — WAL backends (Kafka, Kinesis, SQS, memory)
  audit/                 — S3-based compliance audit export
  auth/                  — OAuth, API keys, sessions
  crypto/                — encryption at rest, key management

sdk/entdb_sdk/           — Python SDK
sdk/go/entdb/            — Go SDK

tests/unit/              — unit tests (fast, no external deps)
tests/integration/       — integration tests (in-memory WAL + SQLite)
```

## Testing

```bash
python -m pytest tests/unit/ -q          # ~1300 tests, <10s
python -m pytest tests/integration/ -q   # ~700 tests, <6s
cd sdk/go/entdb && go test ./...         # Go SDK tests
uvx ruff@0.15.7 check .                 # lint
uvx ruff@0.15.7 format --check .        # format
```

## Key Patterns

- `canonical_store` methods that write to SQLite are `_sync_*` prefixed, wrapped by async methods via `_run_sync` (thread pool)
- The `SchemaRegistry` holds `NodeTypeDef`/`EdgeTypeDef` — register via `register_node_type()`
- GDPR: `user_id` (e.g. "alice") vs `tenant_principal` (e.g. "user:alice") — translate at the boundary
- ACL grants use `Permission` enum, not raw strings
- Actors use `Actor.user("bob")` / `Actor.group("admins")`, not `"user:bob"` strings
