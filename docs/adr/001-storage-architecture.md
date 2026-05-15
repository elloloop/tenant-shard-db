# ADR-001: Storage Architecture

## Status: Accepted

## Pointer to ADR-014

The physical file layout, scale envelope, and tenant-mobility design
are owned by [ADR-014](014-physical-storage-layout.md). This ADR
records the original "tenant file as the primary data boundary"
decision and the per-tenant table list (the actual Go DDL is at
`server/go/internal/store/schema.go`, which is source-of-truth).

## Context

EntDB is a multi-tenant graph database. We need to decide how data is physically stored, partitioned, and accessed.

## Decision

### Tenant file as the primary data boundary

Every tenant gets a SQLite file containing that tenant's application data
and tenant-local derived state:

```
/data/
  tenant_acme.db        — Acme tenant data, ACL, apply metadata
  tenant_smith.db       — Smith tenant data, ACL, apply metadata
```

ADR-014 owns the full physical file strategy, including the global
control-plane file (`global.db`), per-user mailbox files
(`{tenant}/user_{user_id}.db`), and `public.db` semantics.

### Tables per tenant file

The canonical DDL is `server/go/internal/store/schema.go`; this list
is informational only.

```sql
-- Application data (schema-defined)
nodes               — all node types in one table
edges               — all edge types in one table

-- ACL
node_access         — direct grants
acl_inherit         — inheritance pointers
group_users         — group membership
node_visibility     — derived visibility index

-- System
applied_events      — idempotency tracking (event_id dedup)
applied_offsets     — last applied WAL offset per partition
schema_version      — migration tracking
```

### Why single file per tenant

1. **Atomic operations**: create node + grant ACL + write visibility in one transaction
2. **Simple backup**: one file = complete tenant state
3. **Simple recovery**: replay WAL into one file
4. **Tenant isolation**: one tenant's SQLite cannot affect another's
5. **GDPR deletion**: tenant-local application data is bounded by one file (with the per-tenant encryption key shredded per ADR-011)

## Consequences

- Connection pool manages one connection per active tenant
- SQLite WAL mode required for concurrent reads during writes
- Single-threaded executor per tenant (SQLite is not thread-safe)
- Maximum tenant size and sharding/migration strategy are governed by ADR-014.
