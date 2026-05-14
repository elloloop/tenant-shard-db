# ADR-001: Storage Architecture

## Status: Accepted

## Supersedure note

ADR-014 supersedes this document's physical file map, scale envelope, and
tenant-mobility assumptions. This ADR remains the historical decision for
tenant isolation and colocating tenant data, ACLs, notifications, and
apply metadata in the tenant file.

## Context

EntDB is a multi-tenant graph database. We need to decide how data is physically stored, partitioned, and accessed.

## Decision

### Tenant file as the primary data boundary

Every tenant gets a SQLite file containing that tenant's application data
and tenant-local derived state:

```
/data/
  tenant_acme.db        — Acme data, ACL, notifications, apply metadata
  tenant_smith.db       — Smith tenant data, ACL, notifications, apply metadata
  tenant_acme.db        — all of Acme's data, ACL, notifications
  tenant_alice.db       — all of Alice's personal data
  tenant_smith.db       — all of Smith family data
```

ADR-014 owns the full physical file strategy, including global control-
plane state, reserved mailbox/public modes, scale limits, and tenant
mobility.

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
applied_events      — idempotency tracking
schema_version      — migration tracking
```

### Why single file per tenant

1. **Atomic operations**: create node + notify in one transaction
2. **Simple backup**: one file = complete tenant state
3. **Simple recovery**: replay WAL into one file
4. **No cross-file consistency**: notifications and data always in sync
5. **Tenant isolation**: one tenant's SQLite cannot affect another's
6. **GDPR deletion**: tenant-local application data is bounded by one file

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
- Maximum tenant size and sharding/migration strategy are governed by ADR-014.
