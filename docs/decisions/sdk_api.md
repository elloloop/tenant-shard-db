# SDK surface

Frozen decisions about the public SDK API shape (Python + Go) and the codegen toolchain that backs it. Newest first.

---

## 2026-04-14: Single-shape SDK API, typed unique-key tokens via codegen, expression-index unique enforcement

**Status:** frozen
**Decided:** 2026-04-14
**Tags:** sdk, api, proto, codegen, indexing, schema
**Supersedes:** parts of [unique_keys.md — 2026-04-13](unique_keys.md#2026-04-13-client-asserted-unique-keys--secondary-lookup-keys-via-one-node_keys-table) (the `node_keys` table mechanism and the message-level `keys: [...]` proto option are replaced; the user-facing concept of "client declares unique fields, server enforces" stays)
**Superseded by:** none

### Decision

The Python and Go SDKs ship a **single-shape API**: there is exactly one way to perform any given operation. Alternate forms — untyped maps, "raw" escape hatches, parallel `*WithKeys` / `*WithACL` / `*InMailbox` methods, stringly-typed key lookups — are removed. Every public surface is typed end-to-end.

#### The principle

> Any PR that adds a second way to do something an existing API already does is rejected.
> If a new use case appears that the current shape can't express, we change the existing shape — we don't add a parallel one.

This is a load-bearing rule, not a style preference. Two ways to do one thing means agents (and humans) will pick the wrong one 50% of the time, and the documentation has to either show both or lie about which one is canonical. We don't ship that.

#### The one way per operation

| Operation | Python | Go |
|---|---|---|
| Create a node | `plan.create(msg, *, acl=None, storage=Tenant())` | `plan.Create(msg, opts...)` with `WithACL`, `InMailbox`, `InPublic`, `InTenant` option fns |
| Update a node | `plan.update(node_id, msg)` (set fields = patch) | `plan.Update(nodeID string, msg proto.Message)` |
| Delete a node | `plan.delete(NodeType, node_id)` | `plan.Delete[T proto.Message](nodeID string)` |
| Create an edge | `plan.edge_create(EdgeType, from_id, to_id)` | `plan.EdgeCreate(EdgeType, from, to string)` |
| Delete an edge | `plan.edge_delete(EdgeType, from_id, to_id)` | `plan.EdgeDelete(EdgeType, from, to string)` |
| Get by node_id | `scope.get(NodeType, node_id)` | `scope.Get[T proto.Message](ctx, nodeID)` |
| Get by unique key | `scope.get_by_key(KeyToken, value)` | `scope.GetByKey(ctx, key UniqueKey[T], value T)` |
| Query by filter | `scope.query(NodeType, filter=...)` | `scope.Query[T proto.Message](ctx, filter)` |
| Share | `scope.share(node_id, actor, permission)` | `scope.Share(ctx, nodeID string, actor Actor, perm Permission)` |
| Commit | `await plan.commit()` | `plan.Commit(ctx)` |

`msg` is a generated proto message — `*shop.Product` in Go, `schema_pb2.Product(...)` in Python. Field IDs and type IDs are extracted from the message's proto descriptor at SDK boundary. The user never types a field-id or type-id literal in their code.

#### Unique keys: declared at the field, looked up via typed token

Unique constraints are declared at the **field** site, not in a message-level list:

```proto
message Product {
  option (entdb.node) = { type_id: 201 };
  string sku     = 1 [(entdb.field).unique = true];
  string barcode = 2 [(entdb.field).unique = true];
  string name    = 3;
}
```

The `--entdb-keys_out` codegen plugin reads `(entdb.field).unique = true` and emits a sidecar file alongside the protoc output:

**Python** (`schema_entdb.py`):
```python
from entdb_sdk import UniqueKey

class ProductKeys:
    sku     = UniqueKey[str](type_id=201, field_id=1, name="sku")
    barcode = UniqueKey[str](type_id=201, field_id=2, name="barcode")
```

**Go** (`schema_entdb.go`):
```go
var (
    ProductSKU     = entdb.UniqueKey[string]{TypeID: 201, FieldID: 1, Name: "sku"}
    ProductBarcode = entdb.UniqueKey[string]{TypeID: 201, FieldID: 2, Name: "barcode"}
)
```

`UniqueKey[T]` is generic over the field's value type. `scope.get_by_key(ProductKeys.sku, 12345)` (passing an int instead of a string) is a compile-time / type-checker error.

`UniqueKey` has **no public constructor**. Users cannot build one by hand — the only source is the generated sidecar. This is the mechanism that prevents stringly-typed lookups from creeping back in.

The user never passes a `keys={...}` dict at create time. The proto field IS the key. The SDK reads the field value off the proto message and the server enforces uniqueness on the indexed expression.

#### Server-side enforcement: unique expression index, no `node_keys` table

The `node_keys` lookup table from the 2026-04-13 unique_keys decision is **deleted**. It's replaced by SQLite unique expression indexes on the `nodes` table:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_t201_f1
  ON nodes(tenant_id, json_extract(payload_json, '$."1"'))
  WHERE type_id = 201;
```

One unique index per `(type_id, unique_field_id)` pair. Indexes are created lazily on the first `CreateNode` op for a type whose schema declares unique fields — the applier looks up the type's unique field_ids in the schema registry and runs `CREATE UNIQUE INDEX IF NOT EXISTS` (idempotent) before applying the create. Subsequent writes skip the check via a per-`(tenant_db, type_id)` cache.

Uniqueness is enforced by the index itself: `INSERT OR FAIL` on a duplicate raises `sqlite3.IntegrityError`, which the applier catches and converts to a typed `UniqueConstraintError` that propagates back to the SDK.

**`GetNodeByKey` RPC** takes `(tenant_id, actor, type_id, field_id, value)` — `field_id` is an integer, not a string. The server runs:
```sql
SELECT * FROM nodes
 WHERE tenant_id = ?
   AND type_id = ?
   AND json_extract(payload_json, '$."<field_id>"') = ?
 LIMIT 1
```
which uses the same expression index automatically (SQLite's planner matches the indexed expression).

#### Query operators

`query_nodes` accepts MongoDB-style operators on payload fields, translated to parameterised SQL inside `canonical_store.py`:

| Operator | SQL | Example |
|---|---|---|
| `$eq` (default) | `json_extract(...) = ?` | `{"status": "todo"}` |
| `$ne` | `json_extract(...) != ?` | `{"status": {"$ne": "done"}}` |
| `$gt`, `$gte`, `$lt`, `$lte` | `json_extract(...) > ?` etc. | `{"price_cents": {"$gte": 1000}}` |
| `$in` | `json_extract(...) IN (?,?,?)` | `{"status": {"$in": ["todo", "doing"]}}` |
| `$nin` | `json_extract(...) NOT IN (...)` | `{"status": {"$nin": ["done"]}}` |
| `$like` | `json_extract(...) LIKE ?` | `{"name": {"$like": "Widget%"}}` |
| `$between` | `json_extract(...) BETWEEN ? AND ?` | `{"price_cents": {"$between": [100, 999]}}` |
| `$and` | `(...) AND (...)` | top-level `{"$and": [{...}, {...}]}` |
| `$or` | `(...) OR (...)` | top-level `{"$or": [{...}, {...}]}` |

Indexes for query-time filtering are **explicitly out of scope** for this ADR. Without an index, every filtered query is a tenant+type scan — no worse than today, but no better either. Adding declarative `(entdb.field).indexed = true` and lazy non-unique expression-index creation is a separate decision for later if the scan cost actually bites.

`json_extract` returns SQL-typed values (`INTEGER`, `REAL`, `TEXT`, `NULL`), not JSON strings, so range and `LIKE` comparisons work natively without casting hazards.

### Context

The 2026-04-13 unique_keys decision built a separate `node_keys` table because we (correctly) wanted client-asserted normalization — store `"Alice@Example.COM"` in the payload, index `"alice@example.com"` as the lookup key, app does the lowercasing. In practice that flexibility is rarely used: every serious app stores normalized values in the payload field directly, then renders display capitalization elsewhere if needed.

What we got in exchange for that unused flexibility was:

- A second table to keep consistent on every write
- Two reads on every `get_by_key` (key → node_id, then node_id → row)
- Duplicate storage (~50 B per key per node, on top of the payload bytes)
- A stringly-typed `keys={"email": ...}` argument the developer has to hand-pass at every create site, with zero compile-time link to the proto field it's supposedly mirroring
- A stringly-typed `get_by_key(Type, "email", value)` lookup, where the `"email"` literal can drift from the proto without anything noticing until production blows up

Switching to a unique expression index on the payload field removes all four costs. The "no normalization" tradeoff is documented in this ADR as a deliberate non-feature: apps normalize before writing, full stop. This is what every app does anyway.

The single-shape principle is the second half of the same decision. Once we admit there is one way to declare a unique field (in proto, on the field), there should also be one way to create a node, one way to update it, one way to look it up. The previous SDKs accumulated parallel forms (`Create`, `CreateWithKeys`, `CreateWithACL`, `CreateInMailbox`, `CreateInPublic`, plus map-based and proto-based variants) because each new feature added a method instead of extending the existing one. That's how stringly-typed APIs and runtime-only invariants accumulate. Stop now, before the public surface ships.

The codegen sidecar is the load-bearing piece. Without it, the only way to get a `UniqueKey` token is to construct one by hand from a string field name — and we'd be back to where we started. The sidecar makes typed tokens the path of least resistance: developer runs `protoc`, gets generated proto messages AND typed key tokens AND has no other option for performing a key lookup. The `UniqueKey` constructor is package-private (Go: lowercase init field, Python: leading underscore + `__init_subclass__` guard).

### Alternatives considered

- **Keep `keys={}` as an "advanced" parameter alongside the proto option.** Rejected. Any second way invites the wrong choice.
- **Generate `UniqueKey` tokens at Python import time via a metaclass / decorator that scans the proto descriptor.** Rejected for two reasons: (1) Python-only — Go would still need a separate codegen step, breaking parity. (2) Implicit metaclass magic is harder to read and to debug than a generated file you can `cat`. The sidecar is one extra build step in exchange for reading the generated code in your IDE.
- **Use proto `FieldDescriptor` directly instead of generating typed tokens** (`scope.get_by_key(User.DESCRIPTOR.fields_by_name["email"], v)`). Rejected. Still has the magic string, just buried one layer deeper. No type safety on the value parameter. Not significantly better than the current state.
- **Index on `LOWER(json_extract(...))` to support case-insensitive lookups.** Rejected for v0.3. App normalizes before write; index expression matches payload exactly. Adding a normalization function to the index couples the schema to a specific function and forces every lookup to go through the same function — which would have to be specified somewhere. Out of scope.
- **Composite unique constraints (unique on `(field_a, field_b)`).** Rejected for v0.3. Can be modelled today by deriving a single field at the application layer (`combined = a + ":" + b`) and marking that field unique. Native support is a future ADR if the use case shows up.
- **Keep the map-based `Plan.Create(typeID int, map[string]any)` in Go for "bulk import" use cases.** Rejected. `dynamicpb.NewMessage(descriptor)` from the standard `google.golang.org/protobuf/types/dynamicpb` package gives you a `proto.Message` you can populate generically from CSV/JSON/whatever — exact same SDK entry point, no map required. The CLI uses this path.
- **Phase 2 of MongoDB operators (full `$elemMatch`, `$exists`, regex).** Rejected for v0.3. The eight operators in this ADR cover ~95% of real query needs. More can be added per the principle: extend the existing shape, not a new one.

### Consequences

#### What this locks in

- **`entdb_options.proto`** gains `bool unique = 13` on `FieldOpts`. The `repeated NodeKeySpec keys = 23` field on `NodeOpts` is removed (pre-release, free to delete).
- **`canonical_store.py`** loses the `node_keys` table, all `_sync_*_node_keys` helpers, `_sync_get_node_by_key`, `_sync_check_key_exists`, and the `node_keys` cleanup branch in `_sync_delete_node`. Gains: `_ensure_unique_indexes(conn, tenant_id, type_id)` which reads the schema registry's per-type unique field_ids and runs `CREATE UNIQUE INDEX IF NOT EXISTS` once per `(tenant_db, type_id)`, cached per-process. `_sync_get_node_by_key` is rewritten to query against `nodes` directly using the expression. `_sync_query_nodes` is rewritten to support the eight operators above plus top-level `$and`/`$or`.
- **`applier.py`** drops all `node_keys` insert/update/delete branches. Calls `_ensure_unique_indexes` before each `CreateNodeOp` (one per type per process, then no-ops). Catches `sqlite3.IntegrityError` from a unique-index violation and converts it to a typed apply failure that the gRPC layer translates to `ALREADY_EXISTS` with a structured `UniqueConstraintError` payload.
- **`grpc_server.py`** loses the `keys` field from `CreateNodeOp` wire format. `GetNodeByKey` RPC takes `(type_id, field_id, value)` — `field_id` is an `int32`, not a `key_name` string. `ExecuteAtomic` no longer pre-validates uniqueness — the index is the only enforcement point.
- **`SchemaRegistry`** indexes types by their list of unique field_ids (read once from proto on register).
- **`tools/protoc-gen-entdb-keys/`** is a new Go module — a `protoc-gen-*` plugin that reads the FileDescriptorProto, finds messages with `(entdb.node)` and fields with `(entdb.field).unique = true`, and emits two output files per input proto: `<name>_entdb.py` and `<name>_entdb.go`. Both go in the same output directory as the standard protoc outputs. The plugin is invoked as `protoc --entdb-keys_out=. schema.proto`.
- **Python SDK (`sdk/entdb_sdk/`)**: deletes `Plan.create_with_acl`, `Plan.create_in_mailbox`, `Plan.create_in_public`, the `keys=` parameter on `Plan.create`, the string form of `Scope.get_by_key`, every method that takes `type_id: int` from a public surface. Adds `UniqueKey` class, `Tenant()`/`Mailbox(user_id)`/`Public()` storage descriptors, single-shape `Plan.create(msg, *, acl=None, storage=Tenant())`, `Scope.get_by_key(token: UniqueKey[T], value: T)`. `Plan.update(node_id, msg)` extracts the type from the message — no separate `type_id` arg. `Plan.delete(NodeType, node_id)` takes the proto class as the type witness.
- **Go SDK (`sdk/go/entdb/`)**: deletes `Plan.Create(int, map[string]any)`, `CreateWithKeys`, `UpdateWithKeys`, `CreateWithACL`, `CreateInMailbox`, `CreateInPublic`, every method that takes `typeID int`. Adds `UniqueKey[T any]` generic struct, `CreateOption` function type with `WithACL`, `InMailbox`, `InPublic`, `InTenant` constructors. Single-shape `Plan.Create(msg proto.Message, opts ...CreateOption) string`. `Plan.Update(nodeID string, msg proto.Message)` — type comes from `msg.ProtoReflect().Descriptor()`. `Plan.Delete[T proto.Message](nodeID string)` — type witness via generic. `Scope.GetByKey[T](ctx, key UniqueKey[T], value T)` — generic over value type, uses Go 1.22+ generics. `Scope.Get[T proto.Message](ctx, nodeID)`, `Scope.Query[T proto.Message](ctx, filter)` similarly generic.
- **CLI (`entdb-sdk/cli.py`, `sdk/go/entdb/cmd/entdb/`)**: `entdb get`, `entdb put`, `entdb query` rewritten to use `dynamicpb` (Go) / runtime descriptor lookup (Python) so they don't need compile-time imports. Same SDK entry points as user code.

#### What this makes easy

- **Zero stringly-typed lookups in user code.** The IDE shows every available unique key under `ProductKeys.` / `shop.Product*`. Renaming a proto field, regenerating, then rebuilding surfaces every stale call site as a compile error (Go) or `AttributeError` at import time (Python).
- **Schema migrations are mechanical.** Add a unique field to a proto, regenerate, the SDK has the new typed token, the server lazy-creates the new index on first write of an updated node. No data migration required for new fields.
- **One SDK example covers everything.** Documentation has one canonical `Plan.create(msg, ...)` shape. No "see also `CreateWithACL`" footnotes.
- **Bulk import via dynamicpb is identical to compile-time-typed code.** The CLI eats its own dogfood: same entry points, same error types.

#### What this makes harder

- **Adding a feature that doesn't fit the existing shape requires changing the existing shape.** This is by design — it forces a deliberate decision instead of letting a parallel method slip in. If `Plan.create` needs a new option, it goes through the `CreateOption` mechanism (Go) or as a kwarg (Python), not as a new method.
- **Removing a unique field requires manual index cleanup.** Dropping `(entdb.field).unique = true` in proto won't drop the existing `idx_unique_t<type>_f<field>` index — the applier never sees a "remove constraint" signal. Documented as a manual `DROP INDEX` migration step. Acceptable because removing a unique constraint is a rare, deliberate operation that should require human confirmation anyway.
- **`LIKE` queries cannot use the unique index for prefix matching** (since the unique index is on the bare value). For lookups by `name LIKE 'Widget%'`, a separate index would be needed — out of scope per the "no query-time indexes in v0.3" rule.

#### Failure modes

- **Race between two creates of the same unique value:** SQLite's unique index serialises them at INSERT time. One succeeds, the other gets `IntegrityError` → `UniqueConstraintError` on the wire. No window.
- **Schema registry doesn't know about a unique field at apply time:** `_ensure_unique_indexes` is a no-op and the duplicate slips through as a normal write. Mitigation: schema is registered before any apply (already an existing invariant — applier reads the schema registry on startup), and `_ensure_unique_indexes` is called fresh on every `CreateNodeOp` if the cache miss flag is set, which it is until first success.
- **Codegen sidecar not run:** developer can't construct a `UniqueKey` (no public constructor) → `scope.get_by_key(...)` won't compile. The build error points at the missing import. Documented as the first thing the quickstart shows.
- **`dynamicpb` payload field types don't match the proto's declared types** (e.g., string passed where int32 is declared): server-side validation rejects with `INVALID_ARGUMENT` containing the bad field name and expected type. Same as today.

### References

- Conversation: 2026-04-14 — "we should not give way for agent to make mistakes" → "there should always be only one way to do anything"
- Supersedes parts of: [unique_keys.md — 2026-04-13](unique_keys.md#2026-04-13-client-asserted-unique-keys--secondary-lookup-keys-via-one-node_keys-table)
- Related: [acl.md — 2026-04-13 Typed capability-based permissions](acl.md#2026-04-13-typed-capability-based-permissions-core--per-type-extensions) (proto-first schema authority)
- Related: [storage.md — 2026-04-13 Immutable storage mode](storage.md#2026-04-13-immutable-storage-mode-no-built-in-drafts-primitive) (storage modes are now options to `Create`, not separate methods)
- Implementation: SDK v0.3 single-shape API PR (in progress)
