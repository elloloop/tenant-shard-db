# ADR-001: Storage Architecture

## Status: Accepted

## Context

EntDB is a multi-tenant graph database. We need to decide how data is physically stored, partitioned, and accessed.

## Decision

### One SQLite file per tenant

Every tenant gets a single SQLite file containing ALL of that tenant's state:

```
/data/
  tenant_acme.db        — all of Acme's data, ACL, notifications, audit
  tenant_alice.db       — all of Alice's personal data
  tenant_smith.db       — all of Smith family data
```

### Tables per tenant file

```sql
-- Application data (schema-defined)
nodes               — all node types in one table
edges               — all edge types in one table

-- ACL
node_access         — direct grants
acl_inherit         — inheritance pointers
groups              — per-tenant groups
group_users         — group membership
node_visibility     — derived visibility index

-- Notifications
notifications       — all users' notifications in this tenant
read_cursors        — per-user read position per channel/thread

-- System
audit_log           — all actions in this tenant
applied_events      — idempotency tracking
schema_version      — migration tracking
```

### Global store (separate from tenant files)

Single database shared across all tenants:

```sql
user_registry       — global user identities
tenant_registry     — what tenants exist
tenant_members      — who belongs where
shared_index        — cross-tenant direct shares
deletion_queue      — GDPR deletion processing
```

### Why single file per tenant

1. **Atomic operations**: create node + notify + audit in one transaction
2. **Simple backup**: one file = complete tenant state
3. **Simple recovery**: replay WAL into one file
4. **No cross-file consistency**: notifications and data always in sync
5. **Tenant isolation**: one tenant's SQLite cannot affect another's
6. **GDPR deletion**: delete one file = tenant gone

### Why NOT per-user mailbox files

Original design had `mailbox_{tenant}_{user}.db` per user. With 1000 users per tenant, posting in #general required opening 1000 files. This was the bottleneck.

Moving notifications into the tenant file:
- 1 file open instead of 1000
- Batch insert 1000 notifications in one transaction (~5-10ms)
- Atomic with the message creation

## Consequences

- Connection pool manages one connection per active tenant
- SQLite WAL mode required for concurrent reads during writes
- Single-threaded executor per tenant (SQLite is not thread-safe)
- Maximum tenant size limited by single-file SQLite (~281 TB theoretical, ~100 GB practical)
- Tenants with >100GB data need sharding strategy (future ADR)
