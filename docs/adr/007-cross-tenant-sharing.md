# ADR-007: Cross-Tenant Data Sharing

## Status: Accepted

## Context

Users belong to multiple tenants. They need to share data across tenant boundaries (personal → group, org → user, user → user).

## Decision

### Three sharing operations

```
LINK (share):   item stays in source tenant, others get read/write access
MOVE:           item transfers to target tenant, ACL reset
COPY:           independent duplicate in target tenant
```

### Shared index (global store)

```sql
CREATE TABLE shared_index (
    user_id       TEXT NOT NULL,
    source_tenant TEXT NOT NULL,
    node_id       TEXT NOT NULL,
    permission    TEXT NOT NULL,
    shared_at     INTEGER NOT NULL,
    PRIMARY KEY (user_id, source_tenant, node_id)
);
```

When Alice shares task-1 with Bob:
1. `node_access(task-1, user:bob, read)` in Alice's tenant
2. `shared_index(user:bob, alice-tenant, task-1, read)` in global store

### Cross-tenant reads

Bob reads Alice's task:
```python
db.get(Task, "task-1", tenant_id="alice", actor="user:bob")
```
Server routes to the node owning Alice's tenant. ACL check passes in Alice's tenant.

### Cross-tenant writes

Bob has write on Alice's project. Bob adds a comment:
```python
plan = db.atomic(tenant_id="alice", actor="user:bob")
plan.create(Comment, {"body": "done!"})
```
Server checks: Bob has write via node_access in Alice's tenant. Comment created in Alice's tenant with owner_actor="user:bob".

### shared_with_me()

```
1. Read shared_index for user:bob → [(alice, task-1), (carol, doc-5)]
2. Fetch each node from its source tenant
3. ACL check in source tenant (shared_index is a hint, not authoritative)
```

### Group sharing across tenants

Groups are per-tenant. Cross-tenant group sharing is expanded at write time:
```
Alice shares project with group "close-friends" [bob, carol, dave]:
  → shared_index: 3 individual entries (one per user)

Alice adds eve to close-friends:
  → add shared_index entries for eve for all group-shared items

Alice removes dave:
  → remove dave's shared_index entries
```

### Consistency model

shared_index is eventually consistent:
- Write: shared_index updated by applier after processing share event
- Read: shared_index may briefly lag behind node_access
- Safety: shared_index is a hint. ACL check in source tenant is authoritative.
- Stale entries: result in "not found" (safe), cleaned up by background job
- Missing entries: result in item not appearing in shared_with_me (temporary)

### Cross-tenant reference in payloads

For relationships that span tenants (subscription → course):
```json
{ "course_ref": "easyloops:course-101" }
```
Qualified ID format: `{tenant_id}:{node_id}`. Application resolves at read time.

## Consequences

- shared_index adds write overhead on every share/revoke
- Cross-tenant reads add network hop to owning node
- shared_with_me() latency proportional to number of source tenants
- Stale shared_index entries are safe (ACL is authoritative)
- Group-based cross-tenant sharing has write amplification (expand to individual entries)
