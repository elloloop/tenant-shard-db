# ADR-020: Immutable storage mode; no built-in drafts primitive

**Status:** Accepted
**Decided:** 2026-04-13 (post-ADR-014 scope reduction migrated here per ADR-019)
**Tags:** storage, isolation, drafts, privacy
**Implementation:** `server/go/internal/store/` (storage-mode enforcement); SDK `Plan.Create` storage-mode descriptors

## Decision

Every node has a `storage_mode` chosen at creation time. Three values:

- `TENANT` — default tenant scope, shareable via ACL.
- `USER_MAILBOX` — private user scope, reserved for mailbox data with
  an independent lifecycle.
- `PUBLIC` — public logical scope, reserved until ADR-014's
  ownership, delete, billing, moderation, region, and WAL-keying
  rules are satisfied.

**Storage mode is immutable.** It cannot be changed by `update_node`.
Data cannot be moved between files after creation. This is enforced
server-side.

**No built-in draft/publish primitive.** EntDB does not provide a
`publish_node` op that moves data between files. Applications
implement "draft → published" in one of two ways:

1. **ACL-flip pattern (recommended for collaborative content):**
   create a node with `storage_mode=TENANT` and `acl=[owner only]`.
   "Publish" by updating the ACL to grant broader access. Single
   file, single transaction, zero cross-file complexity.
2. **Mailbox-only pattern (for inherently private data):** create a
   node with `storage_mode=USER_MAILBOX`. The node stays in the
   user's mailbox forever. Good for email drafts, personal notes,
   private state.

Applications that need a "move from mailbox to tenant on publish"
workflow must delete the mailbox node and create a new tenant node
themselves. EntDB does not assist with this — it is intentional,
because moving content across storage modes undermines the isolation
guarantees of `USER_MAILBOX`.

[ADR-014](014-physical-storage-layout.md) owns the physical file
mapping for the logical modes above. Do not infer a new SQLite file
type from a new logical scope unless ADR-014's lifecycle-boundary
rule is met.

## Context

Early design considered a built-in drafts primitive where nodes could
start in the user mailbox and be "published" into the tenant via a
dedicated `publish_node` WAL op. This required cross-file atomic
writes and introduced complex idempotency semantics under WAL replay.

The alternative — making storage mode immutable and letting apps
handle drafts via ACL — turns out to cover every use case with
significantly less mechanism:

- Team task drafts → `storage_mode=TENANT` + `acl=[owner]` + flip ACL
  on publish.
- Email drafts → `storage_mode=USER_MAILBOX` forever (no publish,
  just send).
- Document drafts → same as task drafts.
- Collaborative drafts (Google Docs pattern) →
  `storage_mode=TENANT` + `acl=[drafting group]`.

None of these need a cross-file move. The built-in primitive would
have added substantial Applier logic for zero use cases it uniquely
enables.

ADR-014 later constrained the physical file strategy (file types are
lifecycle boundaries, `USER_MAILBOX` files and `public.db` deferred
until concrete product requirements justify them). The logical
storage-mode contract here — immutable mode, no publish primitive —
survives ADR-014 unchanged: it is the SDK-/server-facing rule, while
ADR-014 owns the on-disk routing.

## Alternatives considered

- **Built-in `publish_node` primitive.** Rejected. Added DB
  complexity, cross-file atomicity concerns, and an opinionated
  workflow that doesn't fit collaborative drafts or email. Delivered
  value for only one specific flow (personal draft → team-published
  task) that can be expressed just as cleanly via ACL flips.
- **Mutable storage mode (allow move between files).** Rejected.
  Undermines the durability and reference guarantees of per-file
  storage. A node that was `USER_MAILBOX` at creation and gets moved
  to `TENANT` later would break any edges that pointed to the
  mailbox copy and would require GDPR delete to chase the node
  across files. Not worth the complexity.
- **Per-user mailbox file proliferation enabled by default.**
  Rejected (and superseded by ADR-014). `USER_MAILBOX` as a logical
  mode is reserved here; physical routing to dedicated user files is
  ADR-014's call once the lifecycle-boundary criteria are met.

## Consequences

**What this locks in:**

- The server validates `storage_mode` as immutable. Any `update_node`
  that changes it is rejected.
- Edges respect the privacy hierarchy:
  `USER_MAILBOX → TENANT → PUBLIC`. References can only point to
  equal-or-less-private data. Tenant → mailbox edges are rejected
  at write time. Public → anything-private is rejected. (This is
  the unidirectional-edge invariant cited by ADR-003 to prevent
  PUBLIC nodes from leaking private data via edge traversal.)
- GDPR delete on a user follows the physical storage strategy in
  ADR-014: mailbox data gets a dedicated file only when that
  lifecycle boundary is implemented; tenant-stored drafts are
  cleaned up in tenant files.
- SDK exposes mailbox creation as a **separate option** (e.g.
  `InMailbox` / `Mailbox(user_id)`) so the immutable choice is
  visible at the call site, not a parameter the reviewer might miss
  (see [ADR-025](025-single-shape-sdk-api.md)).
- Loud SDK documentation warns: "Storage mode is immutable. If you
  might ever share this data, use TENANT with ACL instead."

**What this makes easy:**

- Adding new storage modes later (e.g. `REGION_SHARED` for geo-
  partitioned data) is additive — existing modes don't need to
  change.
- Apps pick the privacy/sharing tradeoff per node at creation time,
  explicitly.
- Fanout, audit, and replication all benefit from each node having a
  single, stable physical home (subject to ADR-014's file-type
  rules).

**What this makes harder:**

- "I want to take this draft and publish it" requires the app to
  `create_node` in tenant and `delete_node` in mailbox as two
  separate ops. Not atomic. Apps that care about atomicity here
  should use the ACL-flip pattern instead (never put the draft in
  mailbox in the first place).

## References

- [ADR-001](001-storage-architecture.md) — tenant file as the
  primary data boundary.
- [ADR-014](014-physical-storage-layout.md) — physical file mapping,
  scale envelope, tenant mobility, and `public.db` semantics.
- [ADR-003](003-acl-model.md) — the unidirectional edge invariant
  (`USER_MAILBOX → TENANT → PUBLIC`) prevents PUBLIC nodes from
  referencing private data and leaking it through edge traversal.
- [ADR-025](025-single-shape-sdk-api.md) — storage modes are options
  to `Create`, not separate methods.
- Files this commit removes content from:
  - `docs/decisions/storage.md` — the immutable-storage-mode and
    no-built-in-drafts decision is captured here; the file is
    deleted in the same commit. ADR-014 already records this file
    in its "Supersedes" list for the physical-mapping half.
