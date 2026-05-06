# Frozen Decisions — Index

One line per decision. Newest within each topic first. Rebuilt on each freeze.

## Topics

### storage
- **2026-04-13** — [Immutable storage mode, no built-in drafts primitive](storage.md#2026-04-13-immutable-storage-mode-no-built-in-drafts-primitive) — frozen

### acl
- **2026-04-13** — [Typed capability-based permissions (core + per-type extensions)](acl.md#2026-04-13-typed-capability-based-permissions-core--per-type-extensions) — frozen
- **2026-04-13** — [Cross-tenant ACL via `tenant:<id>` grantee and PUBLIC write role](acl.md#2026-04-13-cross-tenant-acl-via-tenantid-grantee-and-public-write-role) — frozen

### quotas
- **2026-04-13** — [Three-layer rate limit model, implement Phase 1 (monthly quota) first](quotas.md#2026-04-13-three-layer-rate-limit-model-implement-phase-1-monthly-quota-first) — frozen

### unique_keys
- **2026-04-13** — [Client-asserted unique keys + secondary lookup keys via one `node_keys` table](unique_keys.md#2026-04-13-client-asserted-unique-keys--secondary-lookup-keys-via-one-node_keys-table) — superseded by [sdk_api.md 2026-04-14](sdk_api.md#2026-04-14-single-shape-sdk-api-typed-unique-key-tokens-via-codegen-expression-index-unique-enforcement)

### indexing
- **2026-04-19** — [Declarative non-unique expression indexes via (entdb.field).indexed](query_indexes.md#2026-04-19-declarative-non-unique-expression-indexes-via-entdbfieldindexed) — frozen

### sdk
- **2026-04-14** — [Single-shape SDK API, typed unique-key tokens via codegen, expression-index unique enforcement](sdk_api.md#2026-04-14-single-shape-sdk-api-typed-unique-key-tokens-via-codegen-expression-index-unique-enforcement) — frozen

### search
- **2026-04-19** — [FTS5-backed full-text search via (entdb.field).searchable](fts.md#2026-04-19-fts5-backed-full-text-search-via-entdbfieldsearchable) — frozen

### console
- **2026-05-06** — [Single Go binary `entdb-console` with curated Connect proto, replacing FastAPI console + playground](console.md#2026-05-06-single-go-binary-entdb-console-with-curated-connect-proto-replacing-fastapi-console--playground) — frozen

---

_Powered by the [`freeze-decision` skill](../../.claude/skills/freeze-decision/SKILL.md)._
