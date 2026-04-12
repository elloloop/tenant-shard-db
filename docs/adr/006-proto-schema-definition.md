# ADR-006: Proto-Based Schema Definition

## Status: Accepted

## Context

Applications need to define their data model (node types, edge types, fields). The schema definition must be strictly typed, multi-language, and catch errors at compile time.

## Decision

### Proto as the single source of truth

Application schemas are defined in `.proto` files using custom EntDB options. The `entdb generate` command produces typed code for Python, Go, and future languages.

### Node types as proto messages

```protobuf
enum TaskStatus { TODO = 0; IN_PROGRESS = 1; DONE = 2; }

message Task {
    option (entdb.type_id) = 2;
    option (entdb.tenant_visible) = true;
    option (entdb.data_policy) = BUSINESS;

    string title = 1       [(entdb.required) = true, (entdb.searchable) = true, (entdb.pii) = false];
    TaskStatus status = 2;
    string assignee_id = 3 [(entdb.pii) = true];
    google.protobuf.Timestamp due_date = 4;
}
```

### Edge types as proto messages with typed from/to

```protobuf
message HasComment {
    option (entdb.edge_id) = 1;
    option (entdb.propagate_share) = true;
    option (entdb.data_policy) = BUSINESS;

    Task from = 15;       // protoc VALIDATES Task exists
    Comment to = 16;      // protoc VALIDATES Comment exists
}

message AssignedTo {
    option (entdb.edge_id) = 2;
    option (entdb.data_policy) = BUSINESS;
    option (entdb.on_subject_exit) = BOTH;

    Task from = 15;
    User to = 16;

    string role = 1 [(entdb.pii) = false];  // edge property
}
```

Convention: fields 15/16 are the edge from/to type references. Fields 1-14 are edge properties.

### Compile-time safety

| Mistake | Caught by |
|---|---|
| Typo in edge from/to type | protoc (type reference) |
| Invalid enum value | protoc (enum validation) |
| Duplicate field number | protoc |
| Duplicate type_id | entdb lint |
| Duplicate edge_id | entdb lint |
| Missing data_policy | entdb lint (warning, defaults to PERSONAL) |
| Unmarked PII field | entdb lint (warning) |
| propagate_share on private type | entdb lint (error) |
| Breaking schema change | entdb check (vs snapshot) |

### Two-pass validation gate

```
Pass 1 — protoc (structural):
  Type references, enum types, field numbers, syntax

Pass 2 — entdb lint (semantic):
  type_id / edge_id uniqueness and non-zero
  Fields 15/16 on edges reference entdb.node messages
  data_policy classification on all types
  pii marking on all string fields
  FINANCIAL/AUDIT requires legal_basis
  propagate_share target has inherit=true
  on_subject_exit required when edge references User type
  No breaking changes vs snapshot
```

### CLI

```bash
entdb generate schema.proto --python schema.py --go types.go
entdb lint schema.proto
entdb check schema.proto --baseline .entdb/snapshot.json
entdb init schema.proto
```

### Generated code

Python:
```python
# Auto-generated — do not edit
from entdb_sdk import NodeTypeDef, EdgeTypeDef, AclDefaults, field
Task = NodeTypeDef(type_id=2, name="Task", ...)
HasComment = EdgeTypeDef(edge_id=1, from_type=2, to_type=3, propagate_share=True)
```

Go:
```go
var Task = entdb.NodeTypeDef{TypeID: 2, Name: "Task", ...}
var HasComment = entdb.EdgeTypeDef{EdgeID: 1, FromType: 2, ToType: 3, PropagateShare: true}
```

### What the schema declares per node/edge type

```
Structure:       type_id/edge_id, fields, field types
ACL:             tenant_visible, inherit, private, public
Data policy:     PERSONAL / BUSINESS / FINANCIAL / AUDIT / EPHEMERAL
GDPR:            pii per field, subject_field, on_subject_exit (edges)
Retention:       retention_days, legal_basis
Behavior:        propagate_share (edges), required, searchable, indexed (fields)
```

## Consequences

- Proto is a build-time dependency (protoc or grpc_tools.protoc)
- Custom options require entdb/schema.proto shipped with the SDK
- Fields 15/16 convention needs documentation (not enforced by protoc itself)
- Two-pass validation catches what protoc cannot
- Schema changes require regeneration of typed code
- Breaking changes blocked by CI (entdb check)
