# ACL / Permission Model — Go Port Spec

EPIC: #407 (Python -> Go server port).
Frozen decision: `docs/decisions/acl.md` — 2026-04-13 typed capability-based permissions (CoreCapability + per-type ExtensionCapability) and 2026-04-13 cross-tenant ACL via `tenant:<id>` grantee.
Source of truth (Python): `server/go/internal/acl/`, `server/go/internal/acl/`, `server/go/internal/store/`.

## Model

### Permission enum (legacy, still on the wire)

`Permission` is the legacy coarse enum. New writes carry typed capabilities; the legacy string is retained for backwards compatibility and is mechanically derived for old grants.

Defined at `server/go/internal/acl/`:

| Value     | Notes                                                       |
|-----------|-------------------------------------------------------------|
| `READ`    | implied by every higher tier                                |
| `COMMENT` | implies `READ`                                              |
| `WRITE`   | implies `READ`, `COMMENT`                                   |
| `SHARE`   | implies `READ`, `COMMENT`, `WRITE`                          |
| `DELETE`  | implies `READ`, `COMMENT`, `WRITE`, `DELETE` (no `SHARE`)   |
| `ADMIN`   | implies all positive permissions                            |
| `DENY`    | grants nothing — explicit negative override                 |

Hierarchy table at `server/go/internal/acl/`.

### CoreCapability + ExtensionCapability (typed, current model)

CoreCapability values (universal, stable, wire-frozen — see `docs/decisions/acl.md:24-31`):
`CORE_CAP_UNSPECIFIED=0`, `CORE_CAP_READ=1`, `CORE_CAP_COMMENT=2`, `CORE_CAP_EDIT=3`, `CORE_CAP_DELETE=4`, `CORE_CAP_ADMIN=5`.

ExtensionCapability is a per-type proto enum declared by the user via `option (entdb.node).extension_capability_enum` (decision doc `docs/decisions/acl.md:36-54`). Stored on the wire as `repeated int32 ext_cap_ids` keyed by `type_id`.

### Actor / Principal variants

Two parallel representations (today):

- `Actor` — SDK-facing typed identifier in `sdk/python/entdb_sdk/typed.py:22-75`. Factories: `Actor.user(id)`, `Actor.group(id)`, `Actor.service(id)`. Internal storage is the canonical `kind:id` string.
- `Principal` — server-internal parsed form in `server/go/internal/acl/`. Valid kinds: `user`, `role`, `group`, `tenant`, `system`. Cross-tenant decision adds `tenant:<id>` (`docs/decisions/acl.md:177-185`).

Wildcards:
- `tenant:*` — every authenticated user in the current tenant (`acl.py:127-129`, `canonical_store.py:2887`).
- `tenant:<id>` — every authenticated user in tenant `<id>` (cross-tenant grantee).

### Storage schema (per-tenant SQLite)

All in `server/go/internal/store/`:

- `nodes.acl_blob` (`canonical_store.py:1055`) — JSON list of `{principal, permission}` snapshot kept on the row for round-trip.
- `node_access` (`canonical_store.py:1103-1115`) — direct grants. Columns: `node_id`, `actor_id`, `actor_type`, `permission`, `granted_by`, `granted_at`, `expires_at` (nullable). Extended at `canonical_store.py:1211-1222` with `type_id INTEGER`, `core_caps_json TEXT`, `ext_cap_ids_json TEXT`. PK `(node_id, actor_id)`.
- `node_visibility` (`canonical_store.py:1080-1088`) — denormalized projection of all principals (owner + grantees + `tenant:*`) keyed by `(tenant_id, principal, node_id)`. Used as the index for `ListSharedWithMe` and ACL post-filter joins.
- `group_users` (`canonical_store.py:1118-1127`) — group membership; PK `(group_id, member_actor_id)`. Drives group expansion.
- `acl_inherit` (`canonical_store.py:1130-1137`) — node->parent pointers used to walk the inheritance chain. Edges with `propagates_acl=1` (`canonical_store.py:1070`) write into this table.

Cross-tenant grants are recorded in the global `shared_index` (in `global_store.py`, populated from `ShareNode` and `RemoveGroupMember` cascades — `grpc_server.py:1807-1820`, `1958-1978`, `2015-2022`).

## Capability semantics

### What each Permission allows

- `READ` — `GetNode`, `GetNodes`, `QueryNodes`, edge traversal in `GetConnectedNodes` post-filter, visibility in `ListSharedWithMe`.
- `COMMENT` — create `Comment` child nodes via the `op:"CreateChild" child_type:"Comment"` mapping (`docs/decisions/acl.md:99-107`). Implies `READ`.
- `WRITE` / `EDIT` (CoreCapability) — `UpdateNode` on the target. Field-level gating via `capability_mappings` with `field`/`field_value` selectors (`docs/decisions/acl.md:87-96`).
- `SHARE` — legacy permission allowing further sharing without full ADMIN. Not represented in CoreCapability; collapses to ADMIN in the typed model.
- `DELETE` — `DeleteNode`. Implies `EDIT` chain.
- `ADMIN` — `ShareNode`, `RevokeAccess`, `TransferOwnership`. Implies every positive cap.
- `DENY` — explicit deny row blocks every check on this node for the matching actor (handled before allow-scan in `_sync_can_access` at `canonical_store.py:2893-2904` and in the `AclManager.check_permission` two-pass at `acl.py:243-264`).

### Owner short-circuit

`canonical_store._sync_can_access` and `AclManager.check_permission` both treat the row's `owner_actor` as having every permission (`acl.py:240-241`). System actors (`system:*`, `admin:*`, `__system__`) bypass capability checks at the handler boundary in `_check_capability` (`grpc_server.py:322-323`) and at `_sync_can_access` (`canonical_store.py:2883-2884`).

### Group expansion

Resolved at **check time**, not at grant time. Grants reference the group principal (`group:<id>`); membership is looked up on every check via `resolve_actor_groups` (`canonical_store.py:2828-2862`) which walks `group_users` recursively. The handler calls it once per request, hands the resolved `actor_ids` list to canonical-store reads, and the SQL filter uses `IN (?, ?, ...)` against `node_access.actor_id` and `node_visibility.principal`.

`shared_index` is the **eager** mirror: when a group is granted access to a node, every member is fanned out to `shared_index` rows (`grpc_server.py:1809-1814`, `1967-1976`). When a member is added/removed, the rows are recomputed (`grpc_server.py:1964-1976`, `2015-2022`). This is the only place where membership is materialized; same-tenant ACL checks resolve groups dynamically.

### Extension implications

User-declared per type, computed once at startup as the transitive closure of the implication DAG (`docs/decisions/acl.md:141`). Held in the `CapabilityRegistry` in `server/go/internal/acl/`; `_check_capability` consults `required_for_op(type_id, op_name, field, field_value, child_type)` (`grpc_server.py:325-331`) and `check_grant(core_cap_ids, ext_cap_ids, required_core, required_ext, type_id)` (`grpc_server.py:348-355`).

### Legacy migration

Old `permission` strings are mapped on read: `READ -> [CORE_CAP_READ]`, `WRITE -> [READ, COMMENT, EDIT]`, `ADMIN -> [CORE_CAP_ADMIN]` (`docs/decisions/acl.md:138`). Back-fill happens lazily at `canonical_store.py:3054-3081`.

## Check sites

### Pre-write authorization (handler ingress)

Every mutating handler calls `_check_capability(tenant_id, trusted_actor, node_id, type_id, op_name, ctx)` (`grpc_server.py:299-360`). On failure raises `PermissionError`; the handler turns it into a denied response and emits `record_grpc_request("<op>", "denied", ...)`.

Trusted actor comes from `auth_interceptor.get_authoritative_actor` (`grpc_server.py:1721`, `1763`) — the client-supplied `request.context.actor` is **never** used as the authorization principal (recent fix, commit `fece3fb`).

### Post-read filtering

Two paths:

1. **SQL JOIN at canonical-store.** Read RPCs that scan many nodes (`QueryNodes`, `ListSharedWithMe`, `GetConnectedNodes`) use a SQL `LEFT JOIN node_visibility` (e.g. `canonical_store.py:2728`) plus an `IN (actor_ids, "tenant:*")` filter. Cheaper than per-row Python callbacks.
2. **Per-row check via `can_access`.** `_sync_can_access` (`canonical_store.py:2868-2946`) walks the `acl_inherit` chain with a recursive CTE (depth-bounded by `_ACL_MAX_DEPTH`) to support inherited grants. Used by single-node reads and edge expansion when the join would explode.

### Cascade behaviors

- `TransferOwnership` (`grpc_server.py:2030-2049`, `canonical_store.py:3631-3674`) — rewrites `nodes.owner_actor` and updates `node_visibility`. Existing ACL grants are kept.
- `RevokeAccess` (`grpc_server.py:1828-1875`) — deletes the `node_access` row, deletes the principal from `node_visibility`, and on `group:` revokes recomputes `shared_index` for every member.
- `AddGroupMember` / `RemoveGroupMember` (`grpc_server.py:1946-2028`) — fan out `shared_index` add/remove for every node the group has access to (`canonical_store.list_node_access_for_group`).
- `RevokeAllUserAccess` (GDPR path) — deletes every `node_access` row for the actor across the tenant; visibility is reprojected.

## Go design

Module: `server/go/internal/acl/`. Stays internal (no exported package) until the wire types stabilize; SDK types live in `sdk/go/entdb` and are independent.

### Types

```go
// permission.go
type Permission uint8
const (
    PermissionUnspecified Permission = iota
    PermissionRead
    PermissionComment
    PermissionWrite
    PermissionShare
    PermissionDelete
    PermissionAdmin
    PermissionDeny
)

// capability.go — wire-stable, must match proto enum values exactly
type CoreCapability uint8
const (
    CoreCapUnspecified CoreCapability = 0
    CoreCapRead        CoreCapability = 1
    CoreCapComment     CoreCapability = 2
    CoreCapEdit        CoreCapability = 3
    CoreCapDelete      CoreCapability = 4
    CoreCapAdmin       CoreCapability = 5
)

type ExtCapID int32  // opaque; meaning is type-scoped

// actor.go
type ActorKind uint8
const (
    ActorKindUser ActorKind = iota
    ActorKindGroup
    ActorKindService
    ActorKindRole
    ActorKindTenant
    ActorKindSystem
)

type Actor struct {
    Kind ActorKind
    ID   string  // bare id, no "kind:" prefix
}

func (a Actor) String() string  // "user:bob"
func ParseActor(s string) (Actor, error)
func UserActor(id string) Actor
func GroupActor(id string) Actor

// grant.go
type Grant struct {
    SubjectActor Actor
    NodeID       string
    TypeID       int32
    Permission   Permission         // legacy
    CoreCaps     []CoreCapability
    ExtCapIDs    []ExtCapID
    GrantedBy    Actor
    GrantedAt    int64              // unix millis
    ExpiresAt    *int64             // nullable
}
```

### Interfaces

```go
// Resolver expands group principals to a flat actor_ids list.
type Resolver interface {
    Resolve(ctx context.Context, tenantID string, a Actor) ([]Actor, error)
}

// Checker answers single-node authorization questions.
type Checker interface {
    Check(ctx context.Context, req CheckRequest) error  // returns errs.PermissionDenied
}

type CheckRequest struct {
    TenantID   string
    Actor      Actor
    NodeID     string
    TypeID     int32
    OpName     string
    Field      string  // optional, for field-level gating
    FieldValue string
    ChildType  string  // optional, for CreateChild
}

// Filter is the bulk-read variant — returns the subset of node_ids
// the actor can read. Implementation should prefer a single SQL join
// over per-row Check calls.
type Filter interface {
    FilterReadable(ctx context.Context, tenantID string, a Actor, nodeIDs []string) ([]string, error)
}

// Registry holds the proto-derived capability mappings and implication
// closures. Built once at server startup; immutable thereafter.
type Registry interface {
    RequiredForOp(typeID int32, op, field, fieldValue, childType string) (core *CoreCapability, ext *ExtCapID)
    CheckGrant(grantedCore []CoreCapability, grantedExt []ExtCapID, requiredCore *CoreCapability, requiredExt *ExtCapID, typeID int32) bool
    LegacyToCoreCaps(p Permission) []CoreCapability
}
```

### Enforcer wiring

A single `Enforcer` struct owns `Registry`, `Resolver`, and the `store.ACLReader`. `Enforcer.Check` corresponds 1:1 to `_check_capability` in `grpc_server.py:299-360`. Handlers in `server/go/internal/api/` call `acl.Enforcer.Check` before any state mutation; read paths call `acl.Enforcer.FilterReadable` after a non-filtered fetch, or push the predicate down into the SQL.

## Dependencies

- `server/go/internal/pb` — proto-generated types for `ACLEntry`, `CoreCapability`, `Permission` (legacy string), `ShareNodeRequest` etc. The Go enum constants must be kept synchronized with the generated proto values; consider a `go:generate` check.
- `server/go/internal/store` — `ACLReader.GetGrants(tenant, node, actor, includeCrossTenant)`, `ACLReader.ResolveGroups(tenant, actor)`, `ACLReader.NodeVisibility(tenant, principals, ...)`, `ACLReader.WalkInheritance(tenant, node)`. The ACL package consumes only; writes go through `wal.append` like every other mutation (CLAUDE.md invariant 1).
- `server/go/internal/errs` — `errs.PermissionDenied(actor, node, want)` mapping to gRPC `PERMISSION_DENIED`.
- `server/go/internal/auth` — supplies the trusted actor via the auth interceptor (port of `auth_interceptor.get_authoritative_actor`).
- No dependency on `server/go/internal/wal` or applier — ACL is a pure read-side concern at check time. Grant *writes* are events authored elsewhere.

## Test surface

Port these Python suites to Go (`server/go/internal/acl/*_test.go`):

- `(legacy Python unit test, removed in Phase 4D)` — `Permission` hierarchy, `Principal.parse`, `AclManager.check_permission`, DENY override, `tenant:*` wildcard.
- `(legacy Python unit test, removed in Phase 4D)`, `(legacy Python unit test, removed in Phase 4D)` — typed capability columns, legacy string co-existence, migration back-fill.
- `(legacy Python unit test, removed in Phase 4D)` — implication closure, field-level gating, `CreateChild` mapping.
- `(legacy Python unit test, removed in Phase 4D)` — registry build from schema, `required_for_op`, `check_grant`.
- `(legacy Python unit test, removed in Phase 4D)` — `tenant:<id>` grantee, cross-tenant capability check.
- `(legacy Python unit test, removed in Phase 4D)` — group fan-out semantics on `ShareNode`, `AddGroupMember`, `RemoveGroupMember`, `RevokeAccess`.
- `tests/python/integration/test_privilege_escalation.py` — actor-from-context vs trusted actor; must mirror in Go.

Fixture pattern: a per-test in-memory SQLite store seeded with a tiny schema, a stub `Registry` built from a fixed proto descriptor set, and an `Actor` factory matching the Python `Actor.user("bob")` ergonomics. No Kafka/Kinesis dependency — the ACL package is below the WAL layer in the dependency graph.

Contract tests: place cross-language equivalence cases in `tests/contract/acl/` so Go and Python implementations are checked against the same JSON fixtures.

## RPCs that depend on it

All in `server/go/internal/api/` (line refs are check sites in the Python today; the Go port keeps the same shape).

| RPC                  | Required cap         | How ACL is consulted                                   |
|----------------------|----------------------|--------------------------------------------------------|
| `GetNode`            | `CoreCapRead`        | `_check_capability("GetNode", ...)` then fetch         |
| `GetNodes`           | `CoreCapRead` per id | per-id check or `FilterReadable` bulk path             |
| `QueryNodes`         | `CoreCapRead`        | SQL `JOIN node_visibility` post-filter                 |
| `UpdateNode`         | `CoreCapEdit` (+field map) | `_check_capability` with `field`/`field_value`   |
| `DeleteNode`         | `CoreCapDelete`      | `_check_capability("DeleteNode", ...)`                 |
| `CreateNode`         | tenant membership    | tenant-access check, no node-level ACL                 |
| `CreateChild`        | mapped via `child_type` | `_check_capability(..., child_type=)`               |
| `ShareNode`          | `CoreCapAdmin`       | `grpc_server.py:1783-1790` then write grant            |
| `RevokeAccess`       | `CoreCapAdmin`       | `grpc_server.py:1841-1848`                             |
| `ListSharedWithMe`   | (caller scoped)      | `resolve_actor_groups` + `node_visibility` + `shared_index` (`grpc_server.py:1892-1933`) |
| `AddGroupMember`     | tenant-admin         | fan out to `shared_index`                              |
| `RemoveGroupMember`  | tenant-admin         | fan out reverse to `shared_index`                      |
| `TransferOwnership`  | owner or admin       | rewrites `owner_actor` + visibility                    |
| `GetConnectedNodes`  | `CoreCapRead` on neighbors | `actor_ids` filter passed into traversal SQL     |

## Open questions / risks

- **Group nesting depth.** Python uses recursive expansion in `resolve_actor_groups` without an explicit cap. The Go port must bound depth (mirroring `_ACL_MAX_DEPTH` for inheritance) and add a metric/event when the cap fires; otherwise pathological group cycles freeze the request.
- **Orphan grants.** When a user or group is hard-deleted (GDPR or tenant teardown) `node_access` rows referencing them remain. Today this is handled by GDPR cascade in the applier; the Go port must keep the cascade and add a sweeper for tenant deletes (decision doc `docs/decisions/acl.md:196`).
- **Cross-tenant grant lookups.** `_check_capability` uses `include_cross_tenant=True` (`grpc_server.py:338`) which queries the global `shared_index`. The Go port needs a clean abstraction across the per-tenant SQLite and the global store so a single check can hit both — propose a `store.UnifiedGrantReader`.
- **Performance: read post-filter vs SQL JOIN.** Python sometimes filters in Python after fetch (legacy paths) and sometimes joins in SQL. Pick one in Go: SQL JOIN against `node_visibility` for any RPC that returns >1 node; per-row `Check` only for single-node RPCs. Benchmark before settling.
- **`shared_index` consistency.** Eager fan-out is best-effort and logs `warning` on failure (`grpc_server.py:1819-1820`). Decide for Go whether to make this transactional (write to `shared_index` inside the same WAL event) or keep best-effort with a reconciliation job. Recommendation: fold into the WAL event, it's the only way to guarantee `ListSharedWithMe` after a group-add is consistent.
- **DENY semantics under inheritance.** Today `_sync_can_access` checks DENY only on the target node, not on ancestors (`canonical_store.py:2893-2904`). Confirm whether ancestor DENYs should propagate; the decision doc is silent. Track as an ACL semantics bug to fix in the Go port if confirmed intentional.
- **Field-level grants.** The decision doc rejects a separate field-level ACL primitive but field gating is expressed via `capability_mappings`. Make sure the Go `Registry` can answer `RequiredForOp(..., field, fieldValue, ...)` cheaply — pre-index by `(typeID, op, field)`.
- **Actor string format on the wire.** Python serializes `Actor` as `kind:id`. The Go `Actor` should render identically and round-trip through proto fields that are still typed as `string` (`ACLEntry.grantee`).
