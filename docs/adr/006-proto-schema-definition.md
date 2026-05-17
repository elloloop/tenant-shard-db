# ADR-006: Proto is the type system end-to-end

## Status: Accepted

## Context

Applications need to define their data model (node types, edge
types, fields). The schema definition must be strictly typed,
multi-language, and catch errors at compile time. Beyond definition,
the same schema drives the wire format, the on-disk format, the
SDK type-safety guarantees, and the server's schema registry.

## Decision

### Proto is the single source of truth for the type system

Application schemas are defined in `.proto` files using custom
EntDB options (`(entdb.node)`, `(entdb.edge)`, `(entdb.field)`).
Standard `protoc-gen-go` / `protoc-gen-go-grpc` / `protoc-gen-python`
generate typed stubs into language-specific packages. EntDB ships
no custom alternative to these — no hand-rolled enum reimplementer,
no parallel "schema-class" generator, no proto-message replacement.

The proto file is the **only** authoritative description of a type.
Wire format (gRPC), storage format (id-keyed JSON, see
[ADR-018](018-field-id-keyed-payloads.md)), SDK type signatures
(`User`, `Task`, etc.), server schema registry entries, and the
`entdb-schema` compatibility check **all derive from one proto
descriptor**.

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

Pass 2 — entdb-schema check (semantic):
  type_id / edge_id uniqueness and non-zero
  Fields 15/16 on edges reference entdb.node messages
  data_policy classification on all types
  pii marking on all string fields
  FINANCIAL/AUDIT requires legal_basis
  propagate_share target has inherit=true
  on_subject_exit required when edge references User type
  No breaking changes vs snapshot
```

### Tooling

Standard protoc plus one EntDB-specific tool:

```bash
# Generate typed stubs — standard protoc, no custom EntDB step.
protoc --python_out=. --go_out=. schema.proto

# EntDB-specific: snapshot the schema for CI baseline + diff.
entdb-schema snapshot --from-server localhost:50051 > .schema-snapshot.json
entdb-schema check --baseline .schema-snapshot.json --from-file .schema-snapshot.json
entdb-schema diff --old .schema-snapshot.json --new /tmp/new.json
entdb-schema validate --from-file .schema-snapshot.json
```

The `entdb-schema` binary lives at
`server/go/cmd/entdb-schema/`. See
`docs/guides/schema-lockdown.md` for the full CI-integration guide.

### Generated code

The standard `*_pb2.py` (Python) and `*.pb.go` (Go) outputs from
`protoc` are the only generated code. EntDB ships no extra
"NodeTypeDef" Python class or "entdb.NodeTypeDef" Go struct to
hand-build — the proto message is the type.

**Python — register the proto module with the SDK:**

```python
import my_schema_pb2
from entdb_sdk import register_proto_schema

register_proto_schema(my_schema_pb2)
# Now Task, User, etc. are usable directly:
plan.create(my_schema_pb2.Task(title="Review PR", status="todo"))
```

**Go — use the generated proto types directly:**

```go
import myschema "example.com/myapp/schema"

plan := client.NewPlan(tenantID, actor)
plan.Create(&myschema.Task{Title: "Review PR", Status: "todo"})
```

Field numbers in the proto file (e.g. `string title = 1;`) ARE the
EntDB `field_id`s on disk and on the wire — see
[ADR-018](018-field-id-keyed-payloads.md).

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

- Proto is a build-time dependency (`protoc` plus the
  `proto/entdb/v1/entdb_options.proto` extension file shipped with
  the SDK).
- Custom options live in `entdb_options.proto`. Apps `import` it
  and apply the options to their own messages.
- Fields 15/16 convention on edge types is documentation-only
  (not enforced by `protoc`); `entdb-schema validate` enforces it
  semantically.
- Schema changes require running `protoc` again — the SDK reads
  proto descriptors at runtime via `register_proto_schema()`
  (Python) or generated code (Go).
- Breaking changes blocked by CI (`entdb-schema check` against
  the committed `.schema-snapshot.json` baseline; see
  `docs/guides/schema-lockdown.md`).
- The server's own schema registry is populated by the same
  proto descriptors (loaded from `.schema-snapshot.json` at boot
  via `server/go/internal/schema/loader.go`).
