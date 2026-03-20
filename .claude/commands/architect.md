# Architect — Design and Plan a Full Application

Design and plan features for EntDB using a structured, parallel-safe approach.

## Core Design Philosophy

EntDB follows an **event-sourced, append-only architecture**:
- gRPC API defined in `dbaas/entdb_server/api/proto/entdb.proto`
- WAL backends (Kafka/Kinesis) are pluggable via `dbaas/entdb_server/wal/`
- Per-tenant SQLite databases in `dbaas/entdb_server/apply/`
- Schema system with compatibility checking in `dbaas/entdb_server/schema/`
- Python SDK in `sdk/entdb_sdk/`
- Console (React data browser) in `console/`
- Playground (React sandbox) in `playground/`

## Four-Phase Process

**Phase 1: Gather Requirements** — Understand what needs to be built, which components are affected, and the scope.

**Phase 2: Design Architecture** — Map changes across server, SDK, console, playground. Construct a dependency graph for execution waves.

**Phase 3: Create Issue Tree** — Generate GitHub issues organized by wave, each with full context and no sibling knowledge.

**Phase 4: Report** — Present the issue tree and execution roadmap.

## EntDB Component Map

| Component | Directory | Language | Tests |
|---|---|---|---|
| gRPC Server | `dbaas/entdb_server/` | Python | `tests/unit/`, `tests/integration/` |
| Proto Definition | `dbaas/entdb_server/api/proto/` | Protobuf | Schema compat check |
| SDK | `sdk/entdb_sdk/` | Python | `tests/unit/test_sdk_*.py` |
| Console Gateway | `console/gateway/` | Python (FastAPI) | `tests/playwright/console/` |
| Console Frontend | `console/frontend/` | TypeScript (React+Vite) | `tests/playwright/console/` |
| Playground Frontend | `playground/frontend/` | TypeScript (React+Vite) | `tests/playwright/playground/` |
| E2E Tests | `tests/e2e/` | Python (pytest) | Docker Compose based |
| Playwright Tests | `tests/playwright/` | TypeScript | Playwright |

## Wave Ordering

- **Wave 0**: Proto/schema changes (must merge first)
- **Wave 1**: Server + SDK changes (depend on proto)
- **Wave 2**: Console/Playground changes (depend on server API)
- **Wave 3**: Playwright tests to verify everything works end-to-end

## Critical Safeguards

- Proto changes are backward-compatible (add-only)
- Schema evolution follows `dbaas/entdb_server/schema/compat.py` rules
- Rebase before PR — every branch must rebase on main
- All changes require corresponding tests
- Playwright tests validate frontend behavior against live backend
