# Schema Evolution

EntDB uses protobuf for schema definition ([ADR-006](adr/006-proto-schema-definition.md)). The proto field number IS the on-disk `field_id` ([ADR-018](adr/018-field-id-keyed-payloads.md)) — renames are free, removals retire the number. This page documents which evolutions are safe, which are breaking, and how to enforce the rules at CI time via `entdb-schema`.

## Core principles

1. **Stable numeric IDs.** Every type has a `type_id`; every field has a number (the proto field tag) that IS its `field_id`. These are forever.
2. **Never remove or reuse IDs.** Once a field number is in use, it can be deprecated but never reassigned.
3. **Names are display-only.** Renaming a proto field doesn't touch the wire / disk format. Rename freely.
4. **Field order doesn't matter.** Fields are identified by number.
5. **Schema lockdown is CI-enforced.** `entdb-schema check` against a committed `.schema-snapshot.json` catches breaking changes before they merge.

## Field types

| Proto type | EntDB kind | Notes |
|---|---|---|
| `string` | `str` | UTF-8 string |
| `int32`, `int64`, `sint64`, `fixed64`, `sfixed64` | `int` | 64-bit integer on the wire |
| `float`, `double` | `float` | 64-bit float |
| `bool` | `bool` | |
| `bytes` | `bytes` | base64-encoded on the wire (structpb has no bytes type), stored as raw bytes |
| `int64` (epoch millis convention) | `timestamp` | declared with `(entdb.field) = { kind: "timestamp" }` |
| `enum` | `enum` | declared with `(entdb.field) = { enum_values: "a,b,c" }`; stored as enum-name string |
| repeated `string` / `int32` etc. | `list_str` / `list_int` | declared via the proto `repeated` keyword |
| nested `message` | `json` | nested struct stored as JSON in the parent payload |
| reference to another `(entdb.node)` message | `ref` | stored as a typed `{type_id, id}` pair |

## Safe changes (non-breaking)

### Adding new types

```protobuf
// Before
message User {
  option (entdb.node) = { type_id: 1 };
  string email = 1;
}

// After — new type with a new type_id
message User { ... }
message Task {
  option (entdb.node) = { type_id: 2 };
  string title = 1;
}
```

### Adding new fields

Pick the next unused number for the type.

```protobuf
// Before
message User {
  option (entdb.node) = { type_id: 1 };
  string email = 1;
}

// After
message User {
  option (entdb.node) = { type_id: 1 };
  string email = 1;
  string name  = 2;  // NEW
}
```

Old SDKs ignore field 2; old data has no field 2 (reads return zero-value). Forward and backward compatible.

### Renaming types or fields

The name is metadata only.

```protobuf
// Before
string email = 1;

// After — same number, different name
string email_address = 1;  // SAFE
```

Old SDKs sending `"email"` still hit field 1 because the proto descriptor maps name → number client-side ([ADR-018](adr/018-field-id-keyed-payloads.md)). New SDKs see the renamed name; everyone agrees on field 1.

### Deprecating fields

Mark deprecated; keep the number reserved.

```protobuf
string old_field = 5 [(entdb.field) = { deprecated: true }];
```

Existing data with this field is still readable; new writes get a deprecation warning. Tooling can surface usage to plan a future removal.

### Adding enum values

Append-only on enum lists. Existing values stay.

```protobuf
// Before
string status = 3 [(entdb.field) = { enum_values: "todo,doing,done" }];

// After
string status = 3 [(entdb.field) = { enum_values: "todo,doing,done,archived" }];
```

### Making a required field optional

```protobuf
// Before
string name = 1 [(entdb.field) = { required: true }];

// After
string name = 1;  // optional now
```

Old required data is still valid (it has the field). New optional data is also valid.

## Breaking changes (CI blocks)

### Changing a field's number

```protobuf
// DON'T — field number change breaks all existing data
string email = 1;   // before
string email = 7;   // after — field 1's data is now orphaned
```

### Changing a field's type

```protobuf
// DON'T — type change breaks reads of existing data
string age = 1;     // before
int64  age = 1;     // after — old "age" strings can't be read as int64
```

### Removing a type or reassigning a `type_id`

```protobuf
// DON'T — type_id reuse silently corrupts reads
message User { option (entdb.node) = { type_id: 1 }; ... }   // before
message Tenant { option (entdb.node) = { type_id: 1 }; ... } // after — old User data is now read as Tenant
```

If you need to remove a type, delete the proto message but **never** reuse the `type_id`. `entdb-schema check` enforces this by remembering retired ids.

### Reordering enum values

```protobuf
// DON'T — old data with the integer encoding gets the wrong value back
enum Status { TODO = 0; DOING = 1; DONE = 2; }   // before
enum Status { DONE = 0; TODO = 1; DOING = 2; }   // after — silent corruption
```

Always append new enum values at the end.

### Making an optional field required

```protobuf
// DON'T — existing nodes without the field become invalid
string name = 1;   // was optional
string name = 1 [(entdb.field) = { required: true }];  // now required
```

If you need to add a required field, declare it required from the start; existing types should add fields as optional.

## The `entdb-schema` CLI

`entdb-schema` ([guide](guides/schema-lockdown.md)) is the authoritative tool for runtime schema compatibility.

### One-time setup

Snapshot the schema and commit the snapshot:

```bash
# Boot the server with your registered schema
entdb-server -addr=:50051 -data-dir=/tmp/init -wal-backend=memory &
SERVER_PID=$!
sleep 1

# Snapshot
entdb-schema snapshot --from-server localhost:50051 > .schema-snapshot.json

kill $SERVER_PID
git add .schema-snapshot.json
git commit -m "chore: lock initial schema snapshot"
```

### Every PR

Run a compatibility check in CI:

```bash
entdb-schema check \
  --baseline .schema-snapshot.json \
  --from-server localhost:50051 \
  --format json
```

Exit code 0 = compatible; non-zero = breaking change detected. The full GitHub Actions setup is in `docs/guides/schema-lockdown.md`.

### Subcommands

| Command | Purpose |
|---|---|
| `entdb-schema snapshot` | Emit the current schema as a deterministic JSON envelope |
| `entdb-schema check` | Verify the current schema is compatible with a baseline |
| `entdb-schema diff` | Show differences between two snapshots |
| `entdb-schema validate` | Run cross-reference validation over a registry |
| `entdb-schema version` | Print version |

Stable exit codes: `0` = compatible, `1` = breaking change, `2` = usage / I/O error. Machine output on stdout, progress on stderr.

## When you really need to break

Sometimes you do want to break — major-version cut, migration window, data-corruption fix.

1. **Regenerate the snapshot in the same PR** that makes the change. The diff is part of code review.
2. **Apply the `breaking-schema-change` label** to the PR; CI accepts the change only when the label is present (the workflow is shown in `docs/guides/schema-lockdown.md`).
3. **Add a `BREAKING CHANGE:` entry** to your `CHANGELOG.md` for the release.

For data-level migrations (e.g. you really need to change a field's type for a small set of rows): write a server-side migration tool. EntDB itself doesn't ship one.

## Renaming a field without breaking

```protobuf
// V1
message User { string email = 1; }

// V2 — same number, new name
message User { string email_address = 1; }
```

Both old and new SDKs work — both resolve their local name to number 1 client-side. No data migration needed.

## Deprecating a field

```protobuf
// V1
message User {
  string email = 1;
  string ssn   = 2;  // PII, shouldn't have collected it
}

// V2 — mark deprecated; new SDKs warn or hide the field
message User {
  string email = 1;
  string ssn   = 2 [(entdb.field) = { deprecated: true }];
}

// V3 (later) — stop writing the field but keep the number reserved
message User {
  string email = 1;
  // field 2 reserved (was: ssn)
}
```

Existing data with the field is still readable. New writes can skip the field. The number stays retired.

## Related

- [ADR-006](adr/006-proto-schema-definition.md) — proto-as-type-system
- [ADR-018](adr/018-field-id-keyed-payloads.md) — payloads keyed by `field_id` on wire and disk
- [`docs/guides/schema-lockdown.md`](guides/schema-lockdown.md) — CI integration guide
- [SDK reference](sdk-reference.md) — using the proto-generated types from your app
