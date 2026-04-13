# Access control decisions

Frozen architectural decisions about ACL, principals, and permission scoping. Newest first.

---

## 2026-04-13: Typed capability-based permissions (core + per-type extensions)

**Status:** frozen
**Decided:** 2026-04-13
**Tags:** acl, permissions, capabilities, proto, typing, safety
**Supersedes:** none (extends the existing ACL layer without contradicting prior decisions)
**Superseded by:** none

### Decision

EntDB replaces the coarse three-level `Permission` enum (`READ`/`WRITE`/`ADMIN`) with a **typed, capability-based permission model** in two layers:

**Layer 1 — Core capabilities (built-in, universal).**

A single `CoreCapability` enum ships with EntDB and applies to every node type automatically. User schemas do not re-declare it.

```
enum CoreCapability {
  CORE_CAP_UNSPECIFIED = 0;
  CORE_CAP_READ    = 1;   // can see the node
  CORE_CAP_COMMENT = 2;   // can create child Comment nodes
  CORE_CAP_EDIT    = 3;   // can update node fields
  CORE_CAP_DELETE  = 4;   // can delete the node
  CORE_CAP_ADMIN   = 5;   // can manage ACL + everything else
}
```

**Layer 2 — Extension capabilities (per-type, typed, declared in user proto).**

Each node type that needs domain-specific actions (approve, merge, change_status, publish, etc.) declares its own proto enum and wires it to the type via an `entdb.node` option:

```
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
    capability_mappings:    [...]   // ops to capabilities
    capability_implications:[...]   // extensions → core implications
  };
}
```

**ACL entry wire format (typed on both sides):**

```
message ACLEntry {
  string grantee = 1;                      // user:, group:, service:, tenant: (see 2026-04-13 cross-tenant decision)
  int32 type_id = 2;                       // which type this grant applies to
  repeated CoreCapability core_caps = 3;   // universal, typed
  repeated int32 ext_cap_ids = 4;          // enum values from the type's extension enum
  int64 expires_at = 5;
}
```

**No capability strings anywhere.** The wire format, storage schema, SDK surface, and server authorization checks are all typed.

**Built-in defaults (not declared by the user):**

1. **Default op mappings:**
   - `GetNode`, `GetNodes`, `QueryNodes` → `CORE_CAP_READ` on the target
   - `UpdateNode` → `CORE_CAP_EDIT` on the target (overridable per field via `capability_mappings`)
   - `DeleteNode` → `CORE_CAP_DELETE` on the target
   - `ShareNode`, `RevokeNode` → `CORE_CAP_ADMIN` on the target
   - `CreateNode` → tenant membership (creation is not per-node)

2. **Default implication hierarchy:**
   - `CORE_CAP_ADMIN` → implies `{READ, COMMENT, EDIT, DELETE}`
   - `CORE_CAP_EDIT`  → implies `{READ, COMMENT}`
   - `CORE_CAP_COMMENT` → implies `{READ}`
   - `CORE_CAP_READ` → implies nothing

3. **Extension implications are user-declared** per type in `capability_implications`. Typical pattern: `PR_EXT_CAP_MERGE implies_core [CORE_CAP_EDIT]`, `PR_EXT_CAP_APPROVE implies_core [CORE_CAP_COMMENT]`.

**Field-level permission gating** is expressed via `capability_mappings` entries with `field` (and optionally `field_value`) selectors:

```
capability_mappings: [
  { op: "UpdateNode", field: "status", required_ext: TASK_EXT_CAP_CHANGE_STATUS },
  { op: "UpdateNode", field: "assignee_id", required_ext: TASK_EXT_CAP_ASSIGN },
  { op: "UpdateNode", field: "status", field_value: "merged", required_ext: PR_EXT_CAP_MERGE },
]
```

This provides field-specific authz without needing a separate field-level ACL concept.

**Child creation gating** is expressed with `op: "CreateChild"` and a `child_type`:

```
capability_mappings: [
  { op: "CreateChild", child_type: "Comment", required_core: CORE_CAP_COMMENT },
]
```

Creating a `Comment` child with an edge back to a `Task` requires `CORE_CAP_COMMENT` on the Task. The new Comment is a first-class node with its own ACL (author has `EDIT`/`DELETE` on their comment, readers who can see the parent inherit `READ` on the comment by default).

**A type with no custom actions declares nothing ACL-related.** Core caps + default op mappings cover it.

### Context

The three-level `Permission` enum cannot express:

- "Can read and comment but not edit" (needed for reviewer roles, external collaborators)
- "Can edit the status but not the title" (needed for status-limited workflows)
- "Can approve but not merge" (needed for PR review gating)
- "Can comment on a doc but not publish it"

Adding more enum values (`COMMENT`, `SUGGEST`, `APPROVE`, `MERGE`, `PUBLISH`) would require a fixed enum that covers every type. Doesn't scale: a PR's `MERGE` is meaningless for a Task, and a Doc's `PUBLISH` is meaningless for a PR.

Strings were considered and rejected on safety grounds: typos silently grant less permission than intended, wrong-type capabilities are accepted without error, refactoring rots across call sites, and security-sensitive code should never rely on string matching.

The typed, two-layer model gives compile-time safety, per-type vocabulary, IDE autocomplete, and declarative authz-in-proto without requiring the user to re-declare the base capabilities every time.

### Alternatives considered

- **Option 1: Keep 3-level `Permission` enum, add `COMMENT`.** Rejected. Doesn't express approve-vs-merge or field-specific permissions. Doesn't scale to workflows with more than 4 distinct actions.
- **Option 2: Per-type closed enum covering all capabilities (including base).** Rejected. Every type re-declares `READ`/`EDIT`/`DELETE`/`ADMIN`, boilerplate. Generic "grant read across types" helpers need to know the type at compile time.
- **Option 3: String capabilities with per-type vocabulary.** Rejected on safety grounds. Typos are silent security bugs. No compile-time check. No mechanical refactoring.
- **Option 4: Core enum + per-type extension enum, both typed.** Accepted. Best of both worlds: core caps are universal and zero-boilerplate, extensions are typed and per-type, both are compile-checked.
- **Option 5: Field-level ACL as a first-class concept.** Rejected for now. Field gating is expressed via `capability_mappings` with `field` selectors — lighter weight, same expressiveness for real use cases. A full field-level ACL system (per-field grantees, per-field visibility) would add major complexity for rare needs.

### Consequences

**What this locks in:**

- `ACLEntry` wire format includes `type_id`, `repeated CoreCapability core_caps`, `repeated int32 ext_cap_ids`. The existing `permission: string` field is deprecated; old grants are migrated by deriving core caps (`READ` → `[CORE_CAP_READ]`, `WRITE` → `[CORE_CAP_READ, CORE_CAP_COMMENT, CORE_CAP_EDIT]`, `ADMIN` → `[CORE_CAP_ADMIN]`).
- The server's authorization path is replaced by a declarative registry built at startup from proto `entdb.node` options. Op handlers no longer hardcode capability checks; they call a generic `check_capability(op_name, node_id, actor)` that resolves the required capability from the registry.
- Capability implications are computed once per type at startup (transitive closure of the implication DAG) and cached.
- `CoreCapability` values are stable forever — renumbering them is a wire-breaking change. The same caution applies to extension enum values once published.
- Extension enums are user-defined proto files; users own the evolution of their own vocabularies. Adding a new extension capability is a proto change + regen + redeploy.
- Comments are first-class nodes with their own ACLs, not a special-cased field on other nodes. The `CORE_CAP_COMMENT` capability on a parent gates the ability to create Comment children with an edge to that parent.

**What this makes easy:**

- Granting "read-only" access to any node regardless of type: `share(node, grantee, core_caps=[CORE_CAP_READ])`.
- Expressing domain-specific workflows (PR approve-without-merge, task status-change-without-edit) via a handful of proto lines per type.
- Compile-time safety at every ACL call site in both Python and Go SDKs.
- Auditing which capability was required and which grant satisfied it for every request (added to the audit event schema).

**What this makes harder:**

- Adding a new extension capability requires a proto schema change + SDK regen + server redeploy. This friction is intentional — capabilities are part of the API contract and should not churn.
- Bulk migrations of existing grants when a type's extension enum changes semantics (e.g. splitting `EDIT_ALL` into `EDIT_CONTENT` + `EDIT_METADATA`) require a backfill. Same caveat as any enum evolution.
- Generic cross-type tooling (admin dashboards that show "all grants for user X") must know each type's extension enum to render extension capability names, not just integers. The server exposes a `GetCapabilityMetadata(type_id)` RPC for this.

### References

- Conversation: 2026-04-13 architecture discussion on capability scoping (read/comment/edit/delete/admin + extensions for per-type actions)
- Related decisions:
  - [acl.md — 2026-04-13 Cross-tenant ACL via `tenant:<id>`](acl.md#2026-04-13-cross-tenant-acl-via-tenantid-grantee-and-public-write-role) — grantee principal types, orthogonal to this capability decision
  - [storage.md — 2026-04-13 Immutable storage mode](storage.md#2026-04-13-immutable-storage-mode-no-built-in-drafts-primitive) — storage boundaries
- Implementation: pending. Planned as part of the permission-model migration PR after storage routing ships.

---

## 2026-04-13: Cross-tenant ACL via `tenant:<id>` grantee and PUBLIC write role

**Status:** frozen
**Decided:** 2026-04-13
**Tags:** acl, cross-tenant, permissions, billing, gdpr
**Supersedes:** none
**Superseded by:** none

### Decision

**Cross-tenant sharing uses the existing per-node ACL system with a new principal type.** Grantees on ACL entries can be:

- `user:<id>` — a specific user (existing)
- `group:<id>` — a group within a tenant (existing)
- `service:<id>` — a service account (existing)
- `tenant:<id>` — **new** — any authenticated user in the named tenant

When a user from tenant B reads a node owned by tenant A, the server consults the node's ACL and accepts a grant to `tenant:B` as sufficient. The node still physically lives in `acme.db` (A's file); cross-tenant reads simply open A's file from within B's session. The server has access to all tenant files; the ACL is the only gate.

**No separate cross-tenant grant table.** Bulk grants (e.g. "B can read all of A's shared Tasks") are expressed by granting ACL on a parent collection node and letting ACL inheritance propagate. We do not add a separate tenant-to-tenant relationship concept.

**`PUBLIC` storage writes gated by `PLATFORM_ADMIN` role.** Nodes with `storage_mode=PUBLIC` live in `public.db` and are readable by any authenticated user in any tenant. Writing requires the `PLATFORM_ADMIN` platform role, which is granted per deployment — not per tenant. This prevents arbitrary tenants from polluting the shared public namespace.

**Billing rule.** The tenant that creates a node owns it and pays for its storage. Cross-tenant reads are free (or metered against the reader as network/egress, at the platform's discretion). Public data is paid for by whichever tenant's user authored it; platform-authored public data is on the platform.

**GDPR rules:**

- Tenant A deletes a node shared with B → B loses access automatically (node is gone).
- Tenant A is fully deleted → `rm acme.db` → all cross-tenant grants from A evaporate correctly.
- A user in tenant A is GDPR-deleted → their tenant contributions are anonymized per each type's `data_policy`. Their public contributions have `owner_actor` anonymized but content remains (it's public).
- Public content "right to be forgotten" is an edit-in-place operation by `PLATFORM_ADMIN` or the original author. Full deletion of public content typically requires platform intervention.

### Context

Cross-tenant sharing is a real use case for B2B SaaS: agencies sharing reports with clients, marketplaces showing products to buyers, federations of teams. The existing ACL system already supports per-node grants but only with tenant-local principals (`user:`, `group:`, `service:`).

Public data is a separate concern: system templates, reference catalogs, shared curricula, marketplace listings. It's not owned by a single tenant and can't be modeled as a tenant-to-tenant grant.

Both concerns touch ACL semantics and deserved a single coherent decision.

### Alternatives considered

- **Option 1: Cross-tenant ACL via new `tenant:<id>` grantee + PUBLIC gated by platform role.** Accepted. Composes with existing ACL infrastructure, requires no new storage schema beyond the principal type, makes bulk grants expressible via ACL inheritance.
- **Option 2: Separate cross_tenant_grants table.** Rejected. Introduces a parallel access-control mechanism that would need its own ACL inheritance, its own audit trail, and its own GDPR handling. More code, more surface area, not much more expressive than Option 1.
- **Option 3: Anyone can write to PUBLIC.** Rejected. A shared public namespace with unrestricted writes becomes a spam target and has no clear ownership for moderation or billing. Platform-role gate keeps it controlled while still allowing tenants to publish via platform approval.
- **Option 4: Meter cross-tenant reads against both parties.** Rejected as premature. Starts with free cross-tenant reads; billing complexity is added later if needed.

### Consequences

**What this locks in:**

- The authorization helper (`get_authoritative_actor`, `_require_admin_or_owner`) resolves a grantee list including `tenant:<caller_tenant>` on every cross-tenant read, not just per-user grants.
- Cross-tenant grants appear in the standard `node_visibility` table; no separate schema.
- `PUBLIC` write path goes through a dedicated handler that verifies the `PLATFORM_ADMIN` role before routing to `public.db`.
- ACL inheritance extends to cross-tenant grants: granting `tenant:B` on a collection node grants access to all its descendants.
- The unidirectional edge invariant (from the storage decision) already forbids any node from pointing into a more-private storage mode — this means `PUBLIC` nodes cannot reference tenant or user mailbox data, so public data can never accidentally leak private data through edge traversal.

**What this makes easy:**

- B2B sharing is a single ACL grant, no new API surface.
- Platform shared data (templates, reference data) has a clear home with obvious write-gating.
- Cross-tenant reads reuse the existing authorization pipeline.

**What this makes harder:**

- Revoking a cross-tenant grant is the same as revoking any ACL entry — fine for small N, not designed for bulk `tenant:B` revocations across thousands of nodes. Bulk operations will require a future indexed revocation path.
- Cross-tenant audit (who in B read A's node?) requires that the audit trail captures both the `identity` (user in B) and the `resource_owner_tenant` (A) — the current audit schema already has actor + resource fields, so this works, but cross-tenant queries over the trail need a new index.

### References

- Conversation: 2026-04-13 architecture discussion on cross-tenant sharing and public data
- Related decisions: [storage.md — 2026-04-13 immutable storage mode](storage.md#2026-04-13-immutable-storage-mode-no-built-in-drafts-primitive)
- Implementation: pending, planned alongside the `PUBLIC` storage mode rollout
