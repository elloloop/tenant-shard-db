# ADR-004: GDPR Compliance (Built-In)

## Status: Accepted

## Context

EntDB is a database-as-a-service. GDPR compliance must be built into the database, not left to application developers. The database has complete knowledge of users, tenants, content ownership, and data classification — enough to handle GDPR automatically.

## Decision

### Legal roles

```
Data Subject:     End user (Alice, Bob)
Data Controller:  Application developer (the company building on EntDB)
Data Processor:   EntDB (the database service)
```

The user is NEVER the controller — not even of their personal tenant. The app developer defines policies. The database enforces them.

### Data classification in proto schema

Every node and edge type must declare its data policy:

```protobuf
message Task {
    option (entdb.data_policy) = BUSINESS;
    string assignee_id = 1 [(entdb.pii) = true];
    string title = 2       [(entdb.pii) = false];
}
```

### Data policies

| Policy | Export to user | On user exit | Retention | User can request delete |
|---|---|---|---|---|
| PERSONAL | Full | DELETE | None | Yes |
| BUSINESS | User's contributions | ANONYMIZE | None | Anonymize only |
| FINANCIAL | Full (subject match) | ANONYMIZE PII, RETAIN record | Mandatory (legal_basis required) | No — legal override |
| AUDIT | Not exportable | ANONYMIZE PII, RETAIN record | Mandatory (legal_basis required) | No — legal override |
| EPHEMERAL | Not exportable | DELETE | Short | Yes |

### Defaults: strictest by default

```
No data_policy set   → defaults to PERSONAL (most restrictive)
String field no pii  → WARNING: must explicitly set true or false
FINANCIAL no basis   → ERROR: legal_basis required
```

This forces developers to classify every node type and every string field from day one. Privacy by design (GDPR Article 25).

### PII field tracking

```protobuf
string email = 1         [(entdb.pii) = true];    // scrubbed on anonymization
string ip_address = 2    [(entdb.pii) = true];    // scrubbed on anonymization
string title = 3         [(entdb.pii) = false];   // explicitly not PII, kept
int32 rating = 4;                                   // non-string, no warning
```

Unmarked string fields produce lint warnings. `pii=false` is an explicit declaration that silences the warning.

### Subject field

Data can be ABOUT a user who didn't create it:

```protobuf
message PerformanceReview {
    option (entdb.subject_field) = "employee_id";
    string employee_id = 1 [(entdb.pii) = true];
}
```

Export includes nodes where subject_field matches the user, not just owner_actor.

### Edge data policies

Edges can carry PII and need classification:

```protobuf
message AssignedTo {
    option (entdb.edge_id) = 2;
    option (entdb.data_policy) = BUSINESS;
    option (entdb.on_subject_exit) = BOTH;  // FROM, TO, or BOTH

    Task from = 15;
    User to = 16;
    string assigned_by = 1 [(entdb.pii) = true];
}
```

`on_subject_exit`: when deleted user is referenced in from or to, what happens to the edge.

### Legal retention override

```protobuf
message FinancialTransaction {
    option (entdb.data_policy) = FINANCIAL;
    option (entdb.retention_days) = 2555;
    option (entdb.legal_basis) = "Companies Act 2006 s.386";
}
```

Cannot be deleted by user request. PII is anonymized, record retained.

### User deletion flow

`db.delete_user("user:bob")` — one call, database handles everything:

```
1. EXPORT
   Scan all tenants bob belongs to.
   For each node type with exportable policy, collect:
     - Nodes where owner_actor = bob
     - Nodes where subject_field matches bob
   Store export for 30-day grace period.

2. GRACE PERIOD (30 days)
   Status: pending_deletion.
   Bob can recover account.
   No data modified yet.

3. EXECUTE (after 30 days)
   Personal tenant (bob is sole owner):
     DELETE entire tenant (SQLite file removed).
   
   Each other tenant bob belongs to:
     For each node, based on its type's data_policy:
       PERSONAL   → DELETE node
       BUSINESS   → ANONYMIZE: scrub all pii=true fields, replace owner_actor
       FINANCIAL  → ANONYMIZE PII, RETAIN record for retention_days
       AUDIT      → ANONYMIZE PII, RETAIN record for retention_days
       EPHEMERAL  → DELETE node
     
     For each edge where bob is from or to:
       Based on edge's data_policy + on_subject_exit.
   
   Anonymization:
     owner_actor "user:bob" → "user:anon-" + sha256(bob + salt)[:12]
     Every pii=true field: scrubbed
     Deterministic: same user → same anonymous ID within one deletion

4. GLOBAL CLEANUP
   user_registry: status = deleted
   tenant_members: removed from all tenants
   shared_index: all entries removed
   node_access: all grants to bob removed
   group_users: bob removed from all groups

5. AUDIT
   Deletion logged in every affected tenant.
   Audit entries: bob's identity anonymized.
```

### Lint enforcement

```
entdb lint (warnings — development):
  "Task has no data_policy — defaulting to PERSONAL"
  "Task.assignee_id not marked as PII"

entdb check (errors — CI gate):
  "Cannot deploy with unclassified node types"

Runtime default:
  Unclassified data treated as PERSONAL (strictest).
  Over-delete rather than leak.
```

### GDPR API

```
db.delete_user(user_id)                  — right to erasure (Article 17)
db.export_user_data(user_id)             — right to access/portability (Articles 15, 20)
db.update_user(user_id, name, email)     — right to rectification (Article 16)
db.freeze_user(user_id)                  — right to restrict processing (Article 18)
db.set_legal_hold(tenant_id, enabled)    — legal hold on tenant
```

## Consequences

- Every node type and edge type MUST have data_policy classification
- Every string field MUST be explicitly marked pii=true or pii=false
- Developers are forced to think about data classification at schema time
- User deletion is a single API call with automatic cascade
- Legal retention overrides are documented in the schema with legal_basis
- Audit logs are anonymized, never deleted (compliance requirement)
- 30-day grace period before permanent deletion
