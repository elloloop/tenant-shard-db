# ADR-003: Access Control Model

## Status: Accepted

## Context

EntDB needs built-in access control that works transparently — every read is automatically filtered by the requesting actor's permissions. The ACL must support inheritance (share a task, comments are automatically shared), groups, cross-tenant sharing, and deny overrides.

## Decision

### Visibility levels (checked in order)

```
1. public           — anyone, no auth required
2. owner            — node creator always has access
3. tenant_visible   — all tenant members
4. node_access      — explicit grants (direct or group)
5. acl_inherit      — inherited via propagate_share edges
6. deny             — explicit deny blocks everything (except owner)
```

If any level grants access, the actor can see the node. Deny overrides all except owner.

### Admin role does NOT grant data access

```
Tenant admin can:     manage members, groups, settings, GDPR operations
Tenant admin cannot:  read content, browse nodes, see private data

Same as Google Workspace: admin manages the org, doesn't read emails.
```

Data access is controlled entirely by ACL. Same rules for admin and member.

### Permission hierarchy

```
read      — read node payload
comment   — read + add comment-type children
write     — read + comment + update payload
share     — read + comment + write + reshare with others
delete    — read + comment + write + delete
admin     — all of the above
deny      — blocks all access (except node owner)
```

### Inheritance via propagate_share

When an edge type has `propagate_share=true`, creating that edge inserts an `acl_inherit` row. Access checks walk up the chain via recursive CTE.

```
Task ──[HasComment, propagate_share=true]──> Comment
Share Task with Bob → Bob can see Comment (via acl_inherit)
New comment added after sharing → automatically inherited
Revoke Bob from Task → Bob loses access to all comments (no cascade needed)
```

### Cycle detection

- Applier checks for cycles before inserting acl_inherit
- Recursive CTE with depth limit of 10 as query-time backstop
- Schema-level prevention is insufficient (Folder→Folder is a valid use case)

### Cross-tenant sharing

```
Alice shares task-1 with Bob (different tenants):
  1. node_access in Alice's tenant: (task-1, user:bob, read)
  2. shared_index in global store: (user:bob, alice-tenant, task-1, read)
  3. Notification in Alice's tenant notifications table for bob

Bob's shared_with_me():
  1. Read shared_index for user:bob
  2. Fetch from source tenants
  3. ACL check in source tenant confirms access
```

shared_index is a hint, not authoritative. ACL check in source tenant is the source of truth.

### Group-based sharing

Groups are per-tenant. Sharing with a group:
1. node_access entry references the group: (project-1, group:engineering, write)
2. shared_index entries expanded to individual users at share time
3. Adding member to group → add shared_index entries for their group-shared items
4. Removing member → remove shared_index entries

### Admin operations (audited)

```
transfer_user_content(tenant, from_user, to_user)    — employee offboarding
delegate_access(tenant, from_user, to_user, perm, expires)  — handover
export_user_content(tenant, user)                     — GDPR portability
revoke_user_access(tenant, user)                      — immediate termination
```

All logged in audit_log.

## Consequences

- Every read operation requires actor_id (no anonymous reads except public nodes)
- ACL checks add ~25µs per point check, ~71µs for 10 connected nodes
- propagate_share edges have write overhead (acl_inherit insertion + cycle check)
- Cross-tenant reads require routing to the owning node
- shared_index adds storage and write overhead for cross-tenant shares
