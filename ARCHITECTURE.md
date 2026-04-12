# EntDB System Architecture

## What EntDB Is

A multi-tenant graph database with event sourcing, built-in access control, and GDPR compliance. Each tenant gets an isolated SQLite database. Writes go through a WAL (Kafka/Kinesis/etc), reads come from SQLite.

Applications define their data model in `.proto` files. The database handles tenancy, ACL, GDPR, and cross-tenant sharing automatically.

---

## Core Primitives

```
Users       — global identities that span tenants
Tenants     — independent storage boundaries (like a company)
Groups      — per-tenant lists of people for ACL batching
Nodes       — application data (schema-defined)
Edges       — relationships between nodes (schema-defined)
ACL         — who can see what (inheritance-based)
```

---

## Storage Architecture

### Global Store

Single shared database (SQLite or Postgres for multi-node). Contains identity and cross-tenant state.

```sql
-- Who exists
CREATE TABLE user_registry (
    user_id     TEXT PRIMARY KEY,       -- "user:alice"
    email       TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',  -- active, frozen, pending_deletion, deleted
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- What tenants exist
CREATE TABLE tenant_registry (
    tenant_id   TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',  -- active, archived, legal_hold, deleted
    created_at  INTEGER NOT NULL
);

-- Who belongs to which tenant
CREATE TABLE tenant_members (
    tenant_id   TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    role        TEXT NOT NULL,  -- owner, admin, member, viewer, guest
    joined_at   INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, user_id)
);
CREATE INDEX idx_members_user ON tenant_members(user_id);

-- Cross-tenant direct shares (per-user entries)
CREATE TABLE shared_index (
    user_id       TEXT NOT NULL,
    source_tenant TEXT NOT NULL,
    node_id       TEXT NOT NULL,
    permission    TEXT NOT NULL,
    shared_at     INTEGER NOT NULL,
    PRIMARY KEY (user_id, source_tenant, node_id)
);

-- GDPR deletion queue
CREATE TABLE deletion_queue (
    user_id       TEXT PRIMARY KEY,
    requested_at  INTEGER NOT NULL,
    execute_at    INTEGER NOT NULL,   -- requested_at + 30 days
    export_path   TEXT,               -- where the export is stored
    status        TEXT NOT NULL DEFAULT 'pending'  -- pending, exporting, executing, completed
);
```

### Per-Tenant Store

One SQLite file per tenant. Contains all application data for that tenant.

```sql
-- Application data (schema-defined node types)
CREATE TABLE nodes (
    tenant_id    TEXT NOT NULL,
    node_id      TEXT NOT NULL,
    type_id      INTEGER NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    owner_actor  TEXT NOT NULL,
    acl_blob     TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (tenant_id, node_id)
);

-- Application relationships (schema-defined edge types)
CREATE TABLE edges (
    tenant_id      TEXT NOT NULL,
    edge_type_id   INTEGER NOT NULL,
    from_node_id   TEXT NOT NULL,
    to_node_id     TEXT NOT NULL,
    props_json     TEXT NOT NULL DEFAULT '{}',
    propagates_acl INTEGER NOT NULL DEFAULT 0,
    created_at     INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, edge_type_id, from_node_id, to_node_id)
);

-- Per-tenant groups (ACL batching)
CREATE TABLE groups (
    group_id    TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE group_users (
    group_id         TEXT NOT NULL,
    member_actor_id  TEXT NOT NULL,  -- can be user or group (nested)
    role             TEXT NOT NULL DEFAULT 'member',
    joined_at        INTEGER NOT NULL,
    PRIMARY KEY (group_id, member_actor_id)
);

-- ACL: direct grants
CREATE TABLE node_access (
    node_id     TEXT NOT NULL,
    actor_id    TEXT NOT NULL,
    actor_type  TEXT NOT NULL DEFAULT 'user',
    permission  TEXT NOT NULL,
    granted_by  TEXT NOT NULL,
    granted_at  INTEGER NOT NULL,
    expires_at  INTEGER DEFAULT NULL,
    PRIMARY KEY (node_id, actor_id)
);

-- ACL: inheritance pointers
CREATE TABLE acl_inherit (
    node_id      TEXT NOT NULL,
    inherit_from TEXT NOT NULL,
    PRIMARY KEY (node_id, inherit_from)
);

-- ACL: visibility index (derived from acl_blob)
CREATE TABLE node_visibility (
    tenant_id TEXT NOT NULL,
    node_id   TEXT NOT NULL,
    principal TEXT NOT NULL,
    PRIMARY KEY (tenant_id, node_id, principal)
);

-- Audit log (append-only)
CREATE TABLE audit_log (
    event_id    TEXT PRIMARY KEY,
    actor_id    TEXT NOT NULL,
    action      TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    metadata    TEXT,
    created_at  INTEGER NOT NULL
);

-- Event sourcing: idempotency
CREATE TABLE applied_events (
    tenant_id       TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    stream_pos      TEXT,
    applied_at      INTEGER NOT NULL,
    UNIQUE (tenant_id, idempotency_key)
);
```

### Per-User Store

One SQLite file per user per tenant. For notifications/inbox.

```
mailbox_{tenant}_{user}.db — per-user inbox, FTS5 search
```

---

## Tenant Model

A tenant is an independent entity — like a private limited company. It doesn't belong to a person. It has one or more owners.

```
Roles:
  owner   — can appoint owners, dissolve tenant, full control
  admin   — manage members, groups, settings, initiate GDPR operations
  member  — create/read data per ACL rules
  viewer  — read tenant_visible content only
  guest   — explicit per-node grants only

Rules:
  - Last owner cannot leave (must appoint another or dissolve)
  - Admin can manage the tenant but CANNOT read all content
  - Admin manages the container, ACL controls the content
  - Tenant status: active, archived (read-only), legal_hold (no deletes), deleted
```

How each product uses tenants:

```
glassa.ai (B2B)        — one tenant per organization, org pays
Personal apps (B2C)    — one tenant per user, user pays
nesta (family)         — one tenant per household, one parent pays
easyloops (learning)   — platform tenant for content + tenant per user for progress
elloloop (social)      — platform tenant for public content, sharding for scale
```

---

## Groups

Groups live inside a tenant. Managed by tenant admins. Used for ACL batching.

```
Tenant "acme-corp":
  group "engineering": [bob, dave, eve]
  group "design": [carol, frank]
  
  node_access(project-1, group:engineering, write)
  → all engineering members get write on project-1
  
  Add grace to engineering:
  → grace automatically gets access to everything shared with engineering
  
  Remove dave from engineering:
  → dave loses access to everything shared with engineering
```

Groups are NOT tenants. Groups are free, lightweight, per-tenant. Tenants are storage boundaries with billing.

Cross-tenant sharing with groups: expanded to individual entries at share time.

---

## ACL Model

### Visibility Levels

```
public            — anyone, no auth required
tenant_visible    — all tenant members (members, viewers)
node_access       — explicit grants (direct user or group)
acl_inherit       — inherited via propagate_share edges
owner-only        — default if none of the above
```

### Access Check

```
can_access(actor, node):
  1. Is node public?                              → yes
  2. Is actor the owner of node?                   → yes
  3. Is node tenant_visible AND actor is member?   → yes
  4. Does actor have node_access (direct)?         → yes
  5. Does actor inherit access via acl_inherit?    → yes
     (walk up the chain, check at each ancestor)
  6. Is actor explicitly denied?                   → no, blocked
  7. None of the above                             → no
```

### Deny Precedence

```
Explicit DENY on the node > any ALLOW (direct, inherited, group, tenant)
Owner is NEVER denied (owner always has access)
```

### Inheritance via propagate_share

```protobuf
message HasComment {
    option (entdb.edge_id) = 1;
    option (entdb.propagate_share) = true;
    Task from = 15;
    Comment to = 16;
}
```

When this edge is created, `acl_inherit(comment, task)` is inserted. Access to task implies access to comment. New comments added after sharing automatically inherit.

Cycle detection: applier checks for cycles before inserting acl_inherit. CTE depth limit of 10 as backstop.

### Permission Hierarchy

```
read      — read node payload
comment   — read + add comment-type children
write     — read + comment + update payload
share     — read + comment + write + reshare with others
delete    — read + comment + write + delete
admin     — all of the above
deny      — blocks all access (except owner)
```

### Tenant Roles vs Data Access

```
Tenant admin role  →  manage members, groups, settings
                      does NOT grant read access to content

Content access     →  controlled entirely by ACL
                      same rules for admin and member
                      admin has NO special data visibility
```

---

## Cross-Tenant Sharing

### Direct Share (person-to-person)

```
Alice shares task-1 from her tenant with Bob:
  1. node_access in alice's tenant: (task-1, user:bob, read)
  2. shared_index globally: (user:bob, alice-tenant, task-1, read)
  
Bob's shared_with_me():
  1. Read shared_index for user:bob
  2. Fetch nodes from source tenants
  
Bob reads task-1:
  1. db.get(Task, "task-1", tenant_id="alice", actor="user:bob")
  2. Server checks ACL in alice's tenant → bob has read → returns node
```

### Cross-Tenant Writes

```
Bob has write on alice's task-1. Bob adds a comment:
  1. db.atomic(tenant_id="alice", actor="user:bob")
  2. Server checks: does bob have write on task-1? → yes
  3. Comment created in alice's tenant, owner_actor="user:bob"
```

### Share with Group (expanded at write time)

```
Alice shares project with group "close-friends" in her tenant:
  1. node_access: (project-1, group:close-friends, write)
  2. Resolve group: close-friends = [bob, carol, dave]
  3. shared_index: individual entries for bob, carol, dave
  
Alice adds eve to close-friends:
  4. System adds shared_index entries for eve
  
Alice removes dave from close-friends:
  5. System removes dave's shared_index entries
```

### Move and Copy

```
Link (share):  item stays in source tenant, others can access
Move:          item transfers to target tenant, ACL reset
Copy:          independent duplicate in target tenant
```

---

## Admin Operations

All audited, all logged. Admin manages the container, not the content.

```
transfer_user_content(tenant, from_user, to_user)
  → reassign ownership of all user's nodes in this tenant
  → use case: employee offboarding

delegate_access(tenant, from_user, to_user, permission, expires_at)
  → temporary access to another user's content
  → use case: handover period

export_user_content(tenant, user)
  → export all user's data from this tenant
  → use case: GDPR data portability

revoke_user_access(tenant, user)
  → remove all access grants for this user
  → use case: immediate access termination
```

---

## GDPR Compliance (Built-In)

### Data Policy in Schema

Each node type declares its data classification in the proto schema:

```protobuf
message Task {
    option (entdb.type_id) = 1;
    option (entdb.data_policy) = BUSINESS;
    
    string title = 1;
    string assignee_id = 2 [(entdb.pii) = true];
}
```

### Data Policies

```
PERSONAL    — user's own data. Fully exportable. DELETE on exit.
BUSINESS    — business data. User's contributions exportable. ANONYMIZE on exit.
FINANCIAL   — legal retention required. Exportable. RETAIN + ANONYMIZE PII.
AUDIT       — security/compliance logs. Not user-exportable. RETAIN + ANONYMIZE PII.
EPHEMERAL   — temporary data. Not exportable. DELETE on exit.
```

### PII Fields

```protobuf
string employee_id = 1  [(entdb.pii) = true];   // scrubbed on anonymization
string reviewer_name = 2 [(entdb.pii) = true];   // scrubbed on anonymization
string review_text = 3;                           // kept (content, not identity)
int32 rating = 4;                                 // kept
```

### Subject Field

```protobuf
message PerformanceReview {
    option (entdb.subject_field) = "employee_id";
    string employee_id = 1 [(entdb.pii) = true];
}
```

Export includes nodes where subject_field matches the user, not just owner_actor.

### Legal Retention Override

```protobuf
message FinancialTransaction {
    option (entdb.data_policy) = FINANCIAL;
    option (entdb.retention_days) = 2555;
    option (entdb.legal_basis) = "Companies Act 2006 s.386";
}
```

Cannot be deleted by user request. PII is anonymized but record is retained.

### User Deletion Flow

```
db.delete_user("user:bob") — one call, database handles everything:

1. EXPORT bob's data across all tenants (GDPR portability)
   → only node types with exportable policy
   → respects subject_field (not just owner_actor)
   → stored for 30-day grace period

2. GRACE PERIOD (30 days)
   → user status = pending_deletion
   → bob can recover account
   → no data deleted yet

3. After 30 days — EXECUTE:

   Personal tenant (bob is sole owner):
     → DELETE entire tenant (SQLite file removed)

   Each other tenant bob belongs to:
     For each node type, based on data_policy:
       PERSONAL  → DELETE nodes where owner_actor = bob
       BUSINESS  → ANONYMIZE: scrub all pii=true fields, replace owner_actor
       FINANCIAL → ANONYMIZE PII, RETAIN record for retention_days
       AUDIT     → ANONYMIZE PII, RETAIN record for retention_days
       EPHEMERAL → DELETE

   Anonymization (thorough):
     → owner_actor "user:bob" → "user:anon-7f3a9c1b"
     → every field with (entdb.pii)=true: scrubbed
     → no indirect identifiers remain

4. GLOBAL CLEANUP
   → user_registry: status = deleted
   → tenant_members: removed from all tenants
   → shared_index: all entries for bob removed
   → node_access: all grants to bob removed
   → group_users: bob removed from all groups

5. AUDIT
   → deletion event logged in every affected tenant
   → audit log entries: bob's identity anonymized
```

### GDPR API

```
db.delete_user(user_id)                     — right to erasure
db.export_user_data(user_id)                — right to access / portability
db.update_user(user_id, name, email)        — right to rectification
db.freeze_user(user_id)                     — right to restrict processing
db.set_legal_hold(tenant_id, enabled)       — legal hold on tenant
```

---

## Proto Schema Definition

### Format

```protobuf
syntax = "proto3";
import "entdb/schema.proto";
import "google/protobuf/timestamp.proto";

// Enums (compile-time safe)
enum TaskStatus { TODO = 0; IN_PROGRESS = 1; DONE = 2; }

// Nodes
message Task {
    // Identity
    option (entdb.type_id) = 2;
    
    // ACL defaults
    option (entdb.tenant_visible) = true;
    option (entdb.inherit) = true;
    
    // GDPR
    option (entdb.data_policy) = BUSINESS;
    
    // Fields
    string title = 1       [(entdb.required) = true, (entdb.searchable) = true];
    TaskStatus status = 2;
    string assignee_id = 3 [(entdb.pii) = true];
    google.protobuf.Timestamp due_date = 4;
}

// Edges (from/to are type references — protoc validates)
message HasComment {
    option (entdb.edge_id) = 1;
    option (entdb.propagate_share) = true;
    
    Task from = 15;       // compile-time validated
    Comment to = 16;      // compile-time validated
}
```

### Validation Gate (two-pass)

```
Pass 1 — protoc (structural):
  Type references validated
  Enum types validated
  Field number uniqueness
  Syntax errors

Pass 2 — entdb lint (semantic):
  type_id globally unique
  edge_id globally unique
  type_id / edge_id != 0
  Fields 15/16 on edges reference entdb.node messages
  entdb.private and entdb.inherit mutually exclusive
  propagate_share target must have inherit=true
  data_policy FINANCIAL/AUDIT requires legal_basis
  pii fields exist on types with ANONYMIZE policy
  No breaking changes vs snapshot
```

### CLI

```bash
entdb generate schema.proto --python schema.py --go types.go
entdb lint schema.proto
entdb check schema.proto --baseline .entdb/snapshot.json
entdb init schema.proto
```

---

## Event Sourcing

### Write Path

```
Client → Plan.commit() → gRPC ExecuteAtomic
  → Server validates schema fingerprint
  → Server appends event to WAL (Kafka/Kinesis/etc)
  → Server returns Receipt with stream_position
  → Applier consumes event from WAL
  → Applier applies to tenant SQLite (idempotent)
  → Applier updates applied_offset (notifies waiters)
```

### Read-after-Write Consistency

```
Option 1: await plan.commit(wait_applied=True)
  → blocks until applier processes the event

Option 2: await db.get(..., after_offset=receipt.stream_position)
  → server waits for offset before reading

Option 3: await db.wait_for_offset(tenant, position, timeout)
  → explicit wait, then read

All use asyncio.Condition (event-driven, no polling).
```

### WAL Backends

```
Kafka / Redpanda     — production (recommended)
AWS Kinesis          — AWS native
Google Pub/Sub       — GCP native
AWS SQS              — AWS simple queue
Azure Service Bus    — Azure native
Azure Event Hubs     — Azure streaming
Local (in-memory)    — development only
```

---

## Multi-Node Deployment

```
Node A: owns tenants [acme, alice, bob]
Node B: owns tenants [globex, carol, dave]

Global store: shared Postgres (or distributed SQLite)
WAL: shared Kafka cluster

Routing: tenant_id → node assignment via ASSIGNED_TENANTS env var
Cross-tenant reads: routed to the owning node
```

---

## Summary

```
The database provides:
  Tenants, users, groups, ACL, inheritance, cross-tenant sharing,
  GDPR compliance, audit logging, event sourcing, schema validation,
  read-after-write consistency, multi-node sharding.

The application provides:
  Proto schema (node types, edge types, data policies),
  business logic, UI.

The application does NOT implement:
  Access control, GDPR, schema evolution, data export,
  user deletion, anonymization, audit logging.
  
The database handles all of that.
```
