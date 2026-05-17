# ADR-014: Physical Storage Layout

## Status

Accepted

## Date

2026-05-14

## Context

EntDB has accumulated several physical storage concepts:

- `tenant_{tenant_id}.db` for tenant application data, ACLs, groups, and
  per-tenant apply metadata.
- `global.db` for control-plane rows such as users, tenants,
  memberships, quotas, deletion queue, legal holds, and cross-tenant
  share hints.
- `user_{user_id}.db` for the historical `USER_MAILBOX` design.
- `public.db` for the historical `PUBLIC` storage-mode design.

The earlier documents described what each file could hold, but they did
not state when EntDB should add a new physical file type, how single-
tenant and fleet-level scale are handled, how tenant mobility works, or
what `public.db` means operationally. This ADR makes those boundaries
explicit.

## Decision

### Principle: File Types Are Lifecycle Boundaries

Do not add a new SQLite file type merely because a feature has a new
logical scope. A new file type requires at least one hard lifecycle
boundary that cannot be safely represented inside an existing file:

- independent deletion or legal-retention lifecycle;
- independent encryption-key lifecycle;
- independent region-residency boundary;
- independent restore/replay boundary;
- isolation from tenant write locks that is material to product behavior.

If a feature only needs query grouping, authorization grouping, billing
metadata, or UI organization, store it in the existing tenant or global
file and model the scope in rows.

### Locked File Types

`tenant_{tenant_id}.db` is the primary application-data boundary. It
contains tenant nodes, edges, ACL grants, groups, visibility indexes,
notifications, idempotency rows, per-tenant applied offsets, and schema
metadata. Tenant writes are ordered by tenant WAL key and applied into
this file.

`global.db` is the control-plane boundary. It contains user registry,
tenant registry, tenant memberships, shared-index hints, deletion queue,
legal holds, quotas, usage counters, and other cross-tenant control-plane
state. It does not contain tenant application nodes.

`user_{user_id}.db` is not a general-purpose scope rule. It is reserved
for a future `USER_MAILBOX` implementation only when mailbox data has an
independent user deletion/encryption lifecycle that cannot be represented
inside the tenant file. Until such an implementation exists, mailbox RPCs
remain stubs or tenant-backed features.

`public.db` is reserved, not generally enabled. Public data needs a
dedicated product contract before implementation: ownership, delete
authority, moderation, billing, region replication, and WAL keying must
be specified together. Until then, public read sharing is modeled through
tenant data plus ACL/shared-index projections, not a singleton public
SQLite file.

### Single-Tenant Scale

The default strategy is one SQLite file per tenant. A tenant that nears
the practical SQLite operational limit, currently treated as roughly
100 GB for this architecture, is moved by an explicit operator workflow
to a dedicated backend or a future sharded-tenant design.

EntDB does not silently shard one tenant across multiple SQLite files.
Intra-tenant sharding breaks the local edge and ACL consistency model
because endpoints, grants, and visibility rows currently rely on one
transaction boundary. Any intra-tenant shard design requires a future ADR
that defines cross-shard edge semantics, ACL fanout, replay ordering, and
recovery.

### Fleet-Level Scale

Many tenant files are expected. The server should open tenant files
lazily and close idle files under an explicit pool policy. Boot should
not enumerate every tenant file to build correctness-critical state;
tenant registry and shard ownership come from `global.db` and the
sharding layer.

Cold tenants may be snapshotted or archived in future work, but archive
does not change the logical file type. Rehydration must restore the same
tenant file boundary and replay tenant WAL from the relevant offset.

`USER_MAILBOX` would multiply file count substantially. That mode needs
capacity limits and a lazy-open policy before it is enabled.

### Tenant Mobility

Tenants are pinned to a region at creation time. Automatic tenant moves,
splits, and merges are not supported.

A tenant region move is an explicit operator migration: quiesce writes,
copy or rebuild the tenant file in the destination region, update the
tenant registry, and resume writes with a clear WAL offset fence. A
tenant split or merge is a data-migration project, not a primitive of the
storage layer, because ownership, ACLs, unique keys, edges, and audit
history need product-specific rules.

### `public.db` Semantics

`public.db` remains deferred. If implemented later, it must have:

- a sentinel WAL key such as `__public__` or a partitioning rule that is
  not confused with creator tenant ordering;
- explicit creator tenant attribution for billing and abuse handling;
- platform-level write authorization;
- delete and moderation authority;
- region-replication rules;
- edge rules that prevent public data from referencing private data.

Without those rules, a singleton public file is a liability. The current
design should treat `PUBLIC` as a reserved logical mode rather than a
shipping physical file.

## Consequences

- The tenant file remains the default home for application data.
- `global.db` remains the only cross-tenant control-plane store.
- New scopes do not automatically create new files.
- Single-tenant scale beyond the practical SQLite envelope is a future
  explicit migration/sharding decision, not an implicit file split.
- Tenant mobility is operational and explicit, not automatic.
- `public.db` and general `USER_MAILBOX` file proliferation are deferred
  until concrete product requirements justify their lifecycle boundaries.

## Supersedes

This ADR supersedes the physical file mapping and scale/mobility notes in:

- ADR-001, "Storage Architecture";
- ADR-009, "Scale Targets and Deployment Architecture";
- [ADR-020](020-immutable-storage-mode.md), "Immutable storage mode,
  no built-in drafts primitive" (the logical-storage-mode contract
  itself stays in force in ADR-020; ADR-014 owns the physical file
  mapping).

Those documents still own their original higher-level decisions where
not contradicted here: tenant isolation, immutable logical storage mode,
and no built-in publish/move primitive.

## Alternatives Considered

### Every Scope Gets Its Own File

Rejected. This makes the system easy to describe in the small but creates
unbounded file proliferation as product scopes grow. Most scopes are
authorization or query scopes, not lifecycle boundaries.

### Put Everything In One Global SQLite File

Rejected. It loses tenant isolation, makes tenant-level recovery and
deletion harder, and turns every tenant write into contention on one
file.

### Intra-Tenant SQLite Shards At 100 GB

Deferred. It may become necessary, but it is not a safe default. Edges,
ACLs, visibility, idempotency, and replay ordering all need a dedicated
design before tenant data is split across files.

### Ship `public.db` Now

Rejected for now. The semantics are underspecified. Public storage needs
ownership, moderation, billing, delete authority, region, and WAL rules
before implementation.
