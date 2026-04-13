# Storage decisions

Frozen architectural decisions about where data lives in EntDB. Newest first.

---

## 2026-04-13: Immutable storage mode, no built-in drafts primitive

**Status:** frozen
**Decided:** 2026-04-13
**Tags:** architecture, storage, isolation, drafts, privacy
**Supersedes:** none
**Superseded by:** none

### Decision

Every node has a `storage_mode` chosen at creation time. Three values:

- `TENANT` — default. Lives in `tenant.db`, sharable via ACL.
- `USER_MAILBOX` — lives in `{tenant}/user_{user_id}.db`, private to one user.
- `PUBLIC` — lives in `public.db`, readable by any tenant (see cross-tenant decision).

**Storage mode is immutable.** It cannot be changed by `update_node`. Data cannot be moved between files after creation. This is enforced server-side.

**No built-in draft/publish primitive.** EntDB does not provide a `publish_node` op that moves data between files. Applications implement "draft → published" in one of two ways:

1. **ACL-flip pattern (recommended for collaborative content):** create a node with `storage_mode=TENANT` and `acl=[owner only]`. "Publish" by updating the ACL to grant broader access. Single file, single transaction, zero cross-file complexity.
2. **Mailbox-only pattern (for inherently private data):** create a node with `storage_mode=USER_MAILBOX`. The node stays in the user's mailbox forever. Good for email drafts, personal notes, private state.

Applications that need a "move from mailbox to tenant on publish" workflow must delete the mailbox node and create a new tenant node themselves. EntDB does not assist with this — it's intentional, because moving content across storage modes undermines the isolation guarantees of `USER_MAILBOX`.

### Context

Early design considered a built-in drafts primitive where nodes could start in the user mailbox and be "published" into the tenant via a dedicated `publish_node` WAL op. This required cross-file atomic writes and introduced complex idempotency semantics under WAL replay.

The alternative — making storage mode immutable and letting apps handle drafts via ACL — turns out to cover every use case with significantly less mechanism:

- Team task drafts → `storage_mode=TENANT` + `acl=[owner]` + flip ACL on publish
- Email drafts → `storage_mode=USER_MAILBOX` forever (no publish, just send)
- Document drafts → same as task drafts
- Collaborative drafts (Google Docs pattern) → `storage_mode=TENANT` + `acl=[drafting group]`

None of these need a cross-file move. The built-in primitive would have added ~1500 lines of Applier logic for zero use cases it uniquely enables.

### Alternatives considered

- **Option 1: Built-in `publish_node` primitive.** Rejected. Added DB complexity, cross-file atomicity concerns, and an opinionated workflow that doesn't fit collaborative drafts or email. Delivered value for only one specific flow (personal draft → team-published task) that can be expressed just as cleanly via ACL flips.
- **Option 2: Immutable storage mode, app handles drafts.** Accepted. Simpler DB, smaller SDK surface, maximum app flexibility, same privacy properties for opt-in mailbox storage.
- **Option 3: Mutable storage mode (allow move between files).** Rejected. Undermines the durability and reference guarantees of per-file storage. A node that was `USER_MAILBOX` at creation and gets moved to `TENANT` later would break any edges that pointed to the mailbox copy and would require GDPR delete to chase the node across files. Not worth the complexity.

### Consequences

**What this locks in:**

- The server validates `storage_mode` as immutable. Any `update_node` that changes it is rejected.
- Edges respect the privacy hierarchy: `USER_MAILBOX → TENANT → PUBLIC`. References can only point to equal-or-less-private data. Tenant → mailbox edges are rejected at write time. Public → anything-private is rejected.
- GDPR delete on a user is `rm {tenant}_{user}.db` for mailbox data plus a per-tenant cleanup query for tenant-stored drafts owned by the user.
- SDK exposes mailbox creation as a **separate method** (e.g. `CreateInMailbox` / `create_in_mailbox`) so the immutable choice is visible at the call site, not a parameter the reviewer might miss.
- Loud SDK documentation warns: "Storage mode is immutable. If you might ever share this data, use TENANT with ACL instead."

**What this makes easy:**

- Adding new storage modes later (e.g. `REGION_SHARED` for geo-partitioned data) is additive — existing modes don't need to change.
- Apps pick the privacy/sharing tradeoff per node at creation time, explicitly.
- Fanout, audit, and replication all benefit from each node having a single, stable physical home.

**What this makes harder:**

- "I want to take this draft and publish it" requires the app to `create_node` in tenant and `delete_node` in mailbox as two separate ops. Not atomic. Apps that care about atomicity here should use the ACL-flip pattern instead (never put the draft in mailbox in the first place).

### References

- Conversation: 2026-04-13 architecture discussion on drafts vs storage modes
- Related decisions: none yet (this is the first frozen storage decision)
- Implementation: planned as Phase 1 of storage-routing work
