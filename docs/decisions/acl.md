# Access control decisions

Frozen architectural decisions about ACL, principals, and permission scoping. Newest first.

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
