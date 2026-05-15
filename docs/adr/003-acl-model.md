# ADR-003: ACL model — typed capabilities, inheritance, cross-tenant grants

**Status:** Accepted
**Decided:** 2026-04-13 (original ADR-003 dates earlier; superseded body
merged from `docs/decisions/acl.md` per the ADR-019 single-home policy)
**Tags:** acl, capabilities, cross-tenant, permissions, proto
**Implementation:** `server/go/internal/acl/`

## Decision

EntDB has built-in access control. Every read is automatically filtered
by the requesting actor's permissions; every mutating call checks the
caller's capability against the target node's ACL.

### Visibility check order

```
1. public           — anyone, no auth required
2. owner            — node creator always has access
3. tenant_visible   — all tenant members
4. node_access      — explicit grants (direct or group)
5. acl_inherit      — inherited via propagate_share edges
6. deny             — explicit deny blocks everything except owner
```

If any level grants access, the actor can see the node. `deny`
overrides all except `owner`.

### Typed capability model (two layers)

The wire format and storage schema for `ACLEntry` carry **typed
capability identifiers**, not strings. Two layers:

**Layer 1 — CoreCapability (universal, built-in).**

```proto
enum CoreCapability {
  CORE_CAP_UNSPECIFIED = 0;
  CORE_CAP_READ    = 1;   // can see the node
  CORE_CAP_COMMENT = 2;   // can create child Comment nodes
  CORE_CAP_EDIT    = 3;   // can update node fields
  CORE_CAP_DELETE  = 4;   // can delete the node
  CORE_CAP_ADMIN   = 5;   // can manage ACL + everything else
}
```

These apply to every node type automatically. User schemas don't
re-declare them.

**Layer 2 — ExtensionCapability (per-type, declared in user proto).**

Each node type that needs domain-specific actions (approve, merge,
publish, change_status, etc.) declares its own proto enum and wires
it via `option (entdb.node)`:

```proto
enum PRExtCapability {
  PR_EXT_CAP_UNSPECIFIED     = 0;
  PR_EXT_CAP_APPROVE         = 1;
  PR_EXT_CAP_REQUEST_CHANGES = 2;
  PR_EXT_CAP_MERGE           = 3;
}

message PR {
  option (entdb.node) = {
    type_id: 301
    extension_capability_enum: "PRExtCapability"
    capability_mappings:    [...]   // ops → caps
    capability_implications:[...]   // extensions → core implications
  };
}
```

### ACL entry wire format

```proto
message ACLEntry {
  string grantee = 1;                      // user:, group:, service:, tenant:
  int32 type_id = 2;                       // which type the grant applies to
  repeated CoreCapability core_caps = 3;
  repeated int32 ext_cap_ids = 4;          // values from the type's ExtensionCapability enum
  int64 expires_at = 5;
}
```

The legacy `permission: string` field is wire-deprecated. Old grants
are mapped on read:

- `READ`  → `[CORE_CAP_READ]`
- `WRITE` → `[CORE_CAP_READ, CORE_CAP_COMMENT, CORE_CAP_EDIT]`
- `ADMIN` → `[CORE_CAP_ADMIN]`

### Default op mappings (no user declaration needed)

- `GetNode`, `GetNodes`, `QueryNodes` → `CORE_CAP_READ` on the target
- `UpdateNode` → `CORE_CAP_EDIT` on the target (overridable per field
  via `capability_mappings`)
- `DeleteNode` → `CORE_CAP_DELETE` on the target
- `ShareNode`, `RevokeAccess` → `CORE_CAP_ADMIN` on the target
- `CreateNode` → tenant membership (creation isn't per-node)

### Default implication hierarchy

- `CORE_CAP_ADMIN` → implies `{READ, COMMENT, EDIT, DELETE}`
- `CORE_CAP_EDIT` → implies `{READ, COMMENT}`
- `CORE_CAP_COMMENT` → implies `{READ}`
- `CORE_CAP_READ` → implies nothing

Extension implications are user-declared per type via
`capability_implications` (e.g. `PR_EXT_CAP_MERGE implies_core
[CORE_CAP_EDIT]`).

Implications are computed once per type at startup (transitive closure
of the implication DAG) and cached.

### Field-level gating

Expressed via `capability_mappings` entries with `field` (and optionally
`field_value`) selectors. Avoids needing a separate field-level ACL:

```
capability_mappings: [
  { op: "UpdateNode", field: "status",      required_ext: TASK_EXT_CAP_CHANGE_STATUS },
  { op: "UpdateNode", field: "assignee_id", required_ext: TASK_EXT_CAP_ASSIGN },
  { op: "UpdateNode", field: "status", field_value: "merged",
    required_ext: PR_EXT_CAP_MERGE },
]
```

### Child creation gating

```
capability_mappings: [
  { op: "CreateChild", child_type: "Comment", required_core: CORE_CAP_COMMENT },
]
```

Creating a `Comment` child with an edge back to a `Task` requires
`CORE_CAP_COMMENT` on the Task. The new Comment is a first-class node
with its own ACL.

### Admin role does NOT grant data access

```
Tenant admin can:     manage members, groups, settings, GDPR operations
Tenant admin cannot:  read content, browse nodes, see private data
```

Same posture as Google Workspace: admin manages the org, doesn't read
data. Tenant data is accessed via ACL grants only; tenant admin
membership doesn't include implicit `CORE_CAP_READ` on tenant content.

The trusted-actor system actors (`__system__`, `system:*`, `admin:*`)
DO bypass per-tenant membership and ACL — they're operator/system
identities. See `server/go/internal/auth/`.

### Inheritance via `propagate_share` edges

When an edge type has `propagate_share=true`, creating that edge
inserts an `acl_inherit` row. Access checks walk up the chain via
recursive CTE.

```
Task ──[HasComment, propagate_share=true]──> Comment
Share Task with Bob → Bob can see Comment via acl_inherit.
New comment after sharing → automatically inherited.
Revoke Bob from Task → Bob loses access to all comments (no cascade).
```

Cycle detection:

- Applier rejects an `acl_inherit` insertion that would create a cycle.
- Recursive CTE has a depth limit of 10 as query-time backstop.
- Schema-level prevention is insufficient (e.g. `Folder → Folder` is
  a valid use case).

### Grantee principal types

```
user:<id>     — a specific user (tenant-local)
group:<id>    — a tenant-local group
service:<id>  — a service account
tenant:<id>   — any authenticated user in the named tenant (cross-tenant)
```

A grant to `tenant:B` on a node in tenant A's store grants read (or
whichever caps are granted) to every authenticated user in tenant B.
The node still physically lives in `tenant_a.db`; the server has
access to all tenant files, the ACL is the only gate.

### Cross-tenant sharing

- **No separate cross-tenant grant table.** Bulk grants ("B can read
  all of A's shared Tasks") are expressed by granting ACL on a parent
  collection node and letting `acl_inherit` propagate.
- **`shared_index` in `global.db`** is a hint for cross-tenant
  discovery ("what other tenants have shared something with me?"). It
  is not authoritative — the ACL in the source tenant is the source
  of truth on every read.
- **Group-based grants** are expanded to individual `shared_index`
  entries at share time / on `AddGroupMember`. Removing a member
  removes their `shared_index` rows.

### `PUBLIC` nodes

Nodes with `storage_mode=PUBLIC` are readable by any authenticated
user in any tenant. **Writing requires the `PLATFORM_ADMIN`
deployment role** (not per-tenant). This prevents arbitrary tenants
from polluting the shared public namespace. The physical routing of
`PUBLIC` writes to `public.db` is owned by
[ADR-014](014-physical-storage-layout.md); ADR-014 defers the full
`public.db` semantics (ownership, delete authority, moderation,
billing, region replication, WAL keying) until each is specified.

### Billing

The tenant that creates a node owns it and pays for its storage.
Cross-tenant reads are free (or metered against the reader at the
platform's discretion). Public data is paid by the authoring tenant;
platform-authored public data is on the platform.

### GDPR semantics

- Tenant A deletes a node shared with B → B loses access automatically
  (the node is gone).
- Tenant A is fully deleted → tenant file is removed → cross-tenant
  grants from A evaporate correctly.
- A user in tenant A is GDPR-deleted → their tenant contributions are
  anonymized per the node type's `data_policy`. Their public
  contributions have `owner_actor` anonymized; content stays.
- "Right to be forgotten" on public content is an edit-in-place by
  `PLATFORM_ADMIN` or the original author.

### Admin operations (audited)

```
TransferUserContent(tenant, from_user, to_user)        — employee offboarding
DelegateAccess(tenant, from_user, to_user, perm, exp)  — handover
ExportUserData(tenant, user)                           — GDPR portability
RevokeAllUserAccess(tenant, user)                      — immediate termination
```

All flow through the WAL and are therefore in the audit log (see
[ADR-015](015-wal-and-s3-object-lock-as-audit-log.md)).

## Alternatives considered

- **String permissions (`"read"`, `"write"`, `"admin"`).** Rejected.
  Typos silently grant less than intended; wrong-type capabilities
  pass without error; refactoring rots across call sites. Security-
  sensitive code shouldn't rely on string matching.
- **Three-level `Permission` enum (`READ`/`WRITE`/`ADMIN`).** Rejected.
  Can't express "read + comment but not edit," "edit status but not
  title," "approve but not merge." Doesn't scale past 4 distinct
  actions per type.
- **Per-type closed enum covering all capabilities (including base).**
  Rejected. Every type re-declares `READ`/`EDIT`/`DELETE`/`ADMIN` —
  boilerplate. Generic helpers (e.g. "grant read across types") need
  to know each type's enum.
- **Field-level ACL as a first-class concept.** Rejected for now.
  Field gating via `capability_mappings` with `field`/`field_value`
  selectors covers real use cases with less mechanism. A full
  field-level ACL system (per-field grantees, per-field visibility)
  adds major complexity for rare needs.
- **Separate `cross_tenant_grants` table.** Rejected. Parallel
  access-control mechanism with its own ACL inheritance, its own
  audit, its own GDPR cascade. More code, more surface area, no more
  expressive than `tenant:<id>` grantees in the standard ACL.
- **Anyone can write to `PUBLIC`.** Rejected. Shared namespace with
  unrestricted writes is a spam target with no clear ownership for
  moderation or billing. `PLATFORM_ADMIN` gate keeps it controlled.
- **Meter cross-tenant reads against both parties.** Rejected as
  premature. Free cross-tenant reads to start; billing complexity can
  be added later.

## Consequences

**What this locks in:**

- `ACLEntry` wire format includes `type_id` + `repeated CoreCapability`
  + `repeated int32 ext_cap_ids`. Authorization is generic
  `check_capability(op_name, node_id, actor)` against a registry
  built at startup from proto descriptors.
- `CoreCapability` values are stable forever — renumbering them is a
  wire-breaking change. Same caution applies to extension enum
  values once published.
- Capability implications are computed once per type at startup
  (transitive closure of the implication DAG) and cached.
- Comments are first-class nodes with their own ACLs, not a
  special-cased field on other nodes.
- Cross-tenant grants live in the standard `node_visibility` table
  with `tenant:<id>` principals. `shared_index` in `global.db` is a
  discovery hint.
- Admin operations flow through the WAL (per ADR-016), so the audit
  log includes them automatically.

**What this makes easy:**

- "Read-only access" to any node regardless of type:
  `share(node, grantee, core_caps=[CORE_CAP_READ])`.
- Domain-specific workflows (PR approve-without-merge, task
  status-change-without-edit) via a handful of proto lines per type.
- Compile-time safety at every ACL call site in both Python and Go
  SDKs.
- B2B cross-tenant sharing is one ACL grant; no new API surface.

**What this makes harder:**

- Adding a new extension capability requires a proto change + SDK
  regen + server redeploy. Intentional — capabilities are part of
  the API contract.
- Bulk migrations of existing grants when a type's extension enum
  changes semantics (e.g. splitting `EDIT_ALL` into `EDIT_CONTENT`
  + `EDIT_METADATA`) require a backfill.
- Bulk revocation of cross-tenant grants (`tenant:B` across thousands
  of nodes) is the same as revoking any ACL entry. Not designed for
  large-N; a future indexed revocation path is tracked separately.
- Generic cross-type tooling (admin dashboards) must call
  `GetCapabilityMetadata(type_id)` to render extension capability
  names rather than integers.

**Failure modes:**

- **Orphan grants.** A user or group is hard-deleted (GDPR or tenant
  teardown), `node_access` rows referencing them remain. Handled
  today by GDPR cascade in the applier; tenant teardown gets a
  dedicated sweeper.
- **Cross-tenant audit.** Querying "who in tenant B read tenant A's
  node?" requires the audit trail to carry both `actor` and
  `resource_owner_tenant`. Audit events already include both; a
  dedicated index on `resource_owner_tenant` is the future-work
  hook.
- **Bulk public moderation.** `PLATFORM_ADMIN` can edit/redact, but
  there's no bulk-moderate primitive. Out of scope for now.

## References

- [ADR-014](014-physical-storage-layout.md) — owns the physical
  routing of `public.db` writes (PUBLIC storage mode).
- [ADR-015](015-wal-and-s3-object-lock-as-audit-log.md) — audit log
  posture; ACL operations are audited via the WAL.
- [ADR-016](016-handlers-append-applier-writes.md) — all ACL mutations
  flow through the WAL → applier → SQLite path.
- [`docs/decisions/storage.md`](../decisions/storage.md) — the
  unidirectional edge invariant (`USER_MAILBOX → TENANT → PUBLIC`)
  prevents PUBLIC nodes from referencing private data and leaking
  it through edge traversal.
- Implementation: `server/go/internal/acl/`, with the capability
  registry populated at server boot from the schema-registry's proto
  descriptors.
- Files this commit removes content from:
  - `docs/decisions/acl.md` — both 2026-04-13 decisions (typed
    capabilities + cross-tenant `tenant:<id>` grantee) are now
    captured here. The file is deleted in the same commit.
