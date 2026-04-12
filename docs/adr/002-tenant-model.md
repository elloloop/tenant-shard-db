# ADR-002: Tenant Model

## Status: Accepted

## Context

EntDB serves multiple products: B2B collaboration (glassa.ai), personal apps, family planners (nesta), learning platforms (easyloops), social networks (elloloop). The tenant model must support all of these.

## Decision

### Tenant is an independent entity

A tenant is like a private limited company. It exists independently. It doesn't belong to a person. It has one or more owners.

```
Tenant = storage boundary + billing entity + ownership entity
```

### Tenant types are application metadata, not database concepts

The database treats all tenants identically. The `type` field is a string the app sets:

```
tenant_registry:
  acme-corp     | type=org       | owner=carol
  alice         | type=personal  | owner=alice
  smith-family  | type=family    | owner=dave
  weekend-hikers| type=group     | owner=alice
  easyloops     | type=platform  | owner=admin
```

The database does not special-case any type.

### Roles on tenants

```
owner   — can appoint owners, dissolve tenant, full control
admin   — manage members, groups, settings, initiate GDPR operations
member  — create/read data per ACL rules
viewer  — read tenant_visible content only
guest   — explicit per-node grants only
```

### Rules

- A tenant must always have at least one owner
- Last owner cannot leave (must appoint another or dissolve)
- Admin manages the container, NOT the content (see ADR-003)
- Tenant statuses: active, archived (read-only), legal_hold (no deletes), deleted
- A user can belong to multiple tenants simultaneously

### How each product maps

| Product | Tenant usage |
|---|---|
| glassa.ai (B2B) | One tenant per org, org pays |
| Personal apps | One tenant per user, user pays |
| nesta (family) | One tenant per household, one parent pays |
| easyloops (learning) | Platform tenant + tenant per user |
| elloloop (social) | Platform tenant, sharded for scale |

### Groups

Groups live inside a tenant. Managed by tenant admins. Used for ACL batching.

- Groups are NOT tenants (groups are free, lightweight, per-tenant)
- Groups are NOT global (a group in Acme's tenant is Acme's group)
- Cross-tenant sharing with groups: expanded to individual shared_index entries at share time
- Adding someone to a group grants them access to everything shared with that group
- Removing someone from a group revokes their group-based access

## Consequences

- No "group tenant" concept — reduces proliferation of SQLite files
- Users switching contexts (personal → work → family) = switching tenant_id
- Billing is per-tenant (the app decides who pays)
- Cross-tenant sharing requires shared_index in global store
