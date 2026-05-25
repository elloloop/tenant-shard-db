# Schema lockdown with `entdb-schema`

Audience: downstream EntDB consumers (application owners building on top
of EntDB). This guide walks through using `entdb-schema` to lock down
your runtime schema and gate your application's pull requests against
silent breaking changes.

> Companion to [ADR-032](../adr/032-schema-evolution-compat-rules.md)
> (the compat rule matrix and the customer-runnable gate) and to issue
> #488 (the original CLI design reference). `entdb-schema` ships as a
> ghcr image and a prebuilt release binary — install once, use
> independently in local dev and CI.

> **The one command.** A single buf-breaking-style invocation, run
> IDENTICALLY in local dev and in CI:
>
> ```bash
> entdb-schema breaking --baseline schema.lock.json --from-file new.json
> ```
>
> It exits non-zero on any breaking change and prints each offending
> change. `breaking` is the buf-style verb; `check` is an exact alias.
> The schema is **name-free** (ids only, [ADR-031](../adr/031-self-describing-name-free-schema.md));
> the snapshot you compare is generated from your own proto.

## What this solves

EntDB's runtime schema is a **registry of node and edge type
definitions**. Each carries a `type_id` and per-field `field_id` that
are the **on-disk keys**, plus metadata (`required`, `unique`,
`enum_values`, `data_policy`, `subject_field`, edge `on_subject_exit`,
…) that the WAL, applier, payload-translation layer, and query planner
all depend on.

The governing principle ([ADR-032](../adr/032-schema-evolution-compat-rules.md))
is: **LOOSENING is safe; TIGHTENING (or identity reuse / data
corruption) is breaking.** A few common changes look harmless at the
proto level but silently break a running cluster:

| Change                                                | Effect                                                                                       |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Reusing a reserved `field_id` (a freed id brought back to life) | Historic rows storing that id are now read back as a different field — silent data corruption. |
| Making a field `unique` or `required`                 | Historical rows may already violate; the index build / validation fails (like Postgres).      |
| Changing a field's `kind`                             | Type-coercion of stored bytes is unsafe.                                                       |
| Reusing a reserved `type_id` / `edge_id`              | History routed to the freed id is mis-interpreted or orphaned.                                 |
| Reordering enum values                                | Old WAL events carry the old integer; replay produces the wrong logical value.                |
| `data_policy` downgrade (e.g. `FINANCIAL` → `BUSINESS`) | Encryption tier weakens for historical rows.                                                 |
| Flipping `on_subject_exit: from` → `both` on an edge  | GDPR delete semantics shift for historical edges.                                            |

Conversely, **removing a field or type is SAFE** — as long as its id is
**reserved** so it can never be reused (the same discipline proto
`reserved` gives). Adding a field/type, dropping `unique`/`required`, and
adding/removing `indexed`/`searchable` are all safe (loosening). The full
matrix is in [ADR-032](../adr/032-schema-evolution-compat-rules.md).

`buf breaking` does **not** catch the runtime breaks — they live in the
runtime registry, not the proto wire contract. `entdb-schema` is the
complementary tool: **`buf breaking` defends the wire, `entdb-schema
breaking` defends the runtime.** Run both (see
[the `buf.yaml` template](../recipes/buf.yaml.template)).

## How the lock-file works

The mechanism is a single committed JSON file
(`.schema-snapshot.json`, at the root of your repo) that captures:

- `version` — snapshot envelope version (currently `1`).
- `fingerprint` — SHA256 of the canonical-encoded schema body. Used as
  a fast equality check.
- `schema` — the full registry shape: `{node_types: [...], edge_types:
  [...]}`, byte-deterministic (sorted keys, no whitespace beyond
  `","` / `":"`).

On every pull request that touches your protos or your registry-loader
code, CI runs `entdb-schema check` to compare the registry encoded by
the PR against the committed `.schema-snapshot.json`. Any change is
classified `BREAKING` or non-breaking against a fixed rule table. The
job exits non-zero on any breaking change unless the PR carries an
explicit opt-in label (see [Opting in to a breaking change](#opting-in-to-a-breaking-change)).

## Install

Pick one of the three supported channels.

### Pre-built binary (CI-friendly)

```bash
# Linux amd64 — change the suffix for arm64 / darwin / windows.
VERSION=v1.0.0
curl -L "https://github.com/elloloop/tenant-shard-db/releases/download/${VERSION}/entdb-schema-${VERSION}-linux-amd64.tar.gz" \
  | tar -xz -C /usr/local/bin entdb-schema
chmod +x /usr/local/bin/entdb-schema
entdb-schema version
```

The release pipeline publishes archives for `linux-amd64`,
`linux-arm64`, `darwin-amd64`, `darwin-arm64`, and `windows-amd64`,
plus a `.sha256` file per archive.

### Docker image

```bash
docker pull ghcr.io/elloloop/tenant-shard-db-schema:v1.0.0
docker run --rm -v "$PWD:/work" -w /work \
  ghcr.io/elloloop/tenant-shard-db-schema:v1.0.0 \
  check --baseline .schema-snapshot.json --from-file .schema-snapshot.json
```

Distroless static base; no shell, no package manager — pin the
version tag, don't run `:latest` in CI.

### `go install`

```bash
go install github.com/elloloop/tenant-shard-db/server/go/cmd/entdb-schema@v1.0.0
```

Useful for local dev. For CI we recommend the pre-built binary or the
Docker image so you don't pay the Go-toolchain install cost on every
run.

## Subcommands

```
$ entdb-schema --help
entdb-schema — manage EntDB schema snapshots and check compatibility.

Commands:
  snapshot   Emit the current schema as a deterministic JSON document.
  breaking   Gate a schema change against a baseline (buf-breaking style).
  check      Alias of breaking — verify compatibility with a baseline.
  diff       Show differences between two snapshot files.
  validate   Run cross-reference validation over a registry.
  version    Print version and exit.
```

Stable exit codes (every subcommand):

| Code | Meaning                                                          |
| ---- | ---------------------------------------------------------------- |
| 0    | Compatible / success                                             |
| 1    | Breaking change detected (`check`, `diff --fail-on-breaking`)    |
| 2    | Usage error or I/O error                                         |

Machine output is on **stdout**, progress / errors on **stderr** —
safe to redirect either independently.

### `snapshot`

Emit the current schema as a deterministic JSON envelope.

```bash
# Snapshot a running server
entdb-schema snapshot --from-server localhost:50051 > .schema-snapshot.json

# Snapshot from an existing file (re-canonicalize, e.g. after a Python-era file)
entdb-schema snapshot --from-file .old-snapshot.json > .schema-snapshot.json
```

Source flags (exactly one is required):

- `--from-file PATH` — read a previously emitted snapshot. Accepts
  both the envelope shape (`{version, fingerprint, schema}`) and the
  bare body shape (`{node_types, edge_types}`) returned by `GetSchema`.
- `--from-server URL` — gRPC server URL; calls `GetSchema` and wraps
  the response in the envelope.
- `--from-descriptors PATH` — *reserved for a follow-up release.* The
  proto-options-based loader (`(entdb.node)/(entdb.edge)/(entdb.field)`
  custom options) is tracked under issue #488 open question #6. Use
  `--from-file` or `--from-server` in the meantime.

Output flags:

- `-o, --output PATH` — write to a file instead of stdout.
- `--pretty` — pretty-print with 2-space indent. **Do not** use
  `--pretty` for the committed `.schema-snapshot.json`; the file is a
  byte-deterministic artifact and `--pretty` defeats that.

### `breaking` (and `check`)

Gate the current schema against a baseline — the buf-breaking-style
verb and the primary CI command. `check` is an exact alias (same engine,
same flags, same exit codes).

```bash
entdb-schema breaking \
  --baseline schema.lock.json \
  --from-file new.json \
  --format json
```

Flags:

- `-b, --baseline PATH` — the committed lock file (required).
- `--from-file | --from-server | --from-descriptors` — source of the
  current schema (same semantics as `snapshot`).
- `--format text|json` — `text` for human PR comments, `json` for CI
  parsing.
- `--allow-breaking` — exit `0` even if breaking changes found. The
  changes are still printed. Equivalent to the `breaking-schema-change`
  label override in CI, but at the binary level.

Sample text output (breaking case — name-free, ids only per
[ADR-031](../adr/031-self-describing-name-free-schema.md)):

```
BREAKING: 3 breaking change(s) detected against schema.lock.json

  [BREAKING] FIELD_UNIQUE_ADDED   node:1.field:2 — field_id=2 on node type_id=1 became unique (historical data may already violate)
  [BREAKING] FIELD_ID_REUSED      node:1.field:9 — field_id=9 on node type_id=1 was reserved in the baseline but is re-introduced as a live field (id reuse)
  [BREAKING] TYPE_ID_REUSED       node:7 — type_id=7 was reserved in the baseline but is re-introduced as a live type (id reuse)
```

A safe (loosening) change passes with exit 0:

```
OK: 3 non-breaking change(s) detected against schema.lock.json

Non-breaking changes (informational): 3
  [OK]       FIELD_UNIQUE_REMOVED   node:1.field:1 — field_id=1 on node type_id=1 no longer unique
  [OK]       FIELD_REMOVED          node:1.field:2 — field_id=2 removed from node type_id=1
  [OK]       FIELD_ADDED            node:1.field:3 — field_id=3 (kind=str) added to node type_id=1
```

Same data shape in `--format json`:

```json
{
  "compatible": false,
  "breaking_count": 1,
  "non_breaking_count": 0,
  "changes": [
    {
      "kind": "FIELD_ID_REUSED",
      "path": "node:1.field:9",
      "new_value": 9,
      "message": "field_id=9 on node type_id=1 was reserved in the baseline but is re-introduced as a live field (id reuse)",
      "breaking": true
    }
  ]
}
```

### Reserved ids — making removal safe

Removing a field or type is safe **only because the freed id is then
reserved** so it can never be reused. Carry the tombstone in your
snapshot so the checker turns any future reuse into a `*_REUSED`
breaking change (the runtime analogue of proto `reserved`):

```json
{
  "node_types": [
    {"type_id": 1, "fields": [{"field_id": 1, "kind": "str"}],
     "reserved_field_ids": [9]}
  ],
  "reserved_type_ids": [7],
  "reserved_edge_ids": [50]
}
```

`reserved_field_ids` lives on a node/edge type; `reserved_type_ids` /
`reserved_edge_ids` live at the schema root. They are emitted only when
non-empty, so they never perturb the fingerprint of a schema that
doesn't reserve. Keep them in lockstep with the proto `reserved` your
`buf breaking` step already enforces on the wire.

### `diff`

Pure file-vs-file comparison. Same change shape as `check`. Doesn't
exit non-zero on breaking by default — pass `--fail-on-breaking` to
get the CI-friendly exit-code contract.

```bash
entdb-schema diff \
  --old .schema-snapshot.json \
  --new /tmp/new-snapshot.json \
  --format text \
  --fail-on-breaking
```

Useful for previewing what a snapshot regeneration would change.

### `validate`

Run cross-reference validation over a registry — confirms every
`ref_type_id` points at a registered node type, every composite-unique
references existing field ids, etc. Good for users who hand-edit
their snapshot.

```bash
entdb-schema validate --from-file .schema-snapshot.json
```

### `version`

Prints the binary version (the tagged release that built it). Useful
for pinning in CI logs.

## Onboarding a new project

If you maintain an EntDB consumer and haven't set up schema lockdown
yet:

### 1. Capture the initial snapshot

The snapshot is generated **from your own proto**. Under
[ADR-031](../adr/031-self-describing-name-free-schema.md) the server
boots empty and learns your schema from your writes (self-describing
writes), so the lowest-friction path is: boot a local server, let your
app (or its tests) make one write per type so the registry is
established, then snapshot it.

```bash
entdb-server -addr=:50051 -data-dir=/tmp/entdb-init -wal-backend=memory &
SERVER_PID=$!
sleep 1

# Run your app / its bootstrap once so each type is registered via a
# self-describing write, then snapshot the registry the server learned.
#   (your-app bootstrap …)
entdb-schema snapshot --from-server localhost:50051 > schema.lock.json

kill $SERVER_PID
```

Commit the lock file at the repo root:

```bash
git add schema.lock.json
git commit -m "chore: lock initial schema snapshot"
```

`schema.lock.json` is a **plain checked-in file** — not a generated
artifact, not git-LFS, not gitignored. (Any filename works;
`schema.lock.json` is the convention used throughout this guide.)

### 2. Wire the CI check

Create `.github/workflows/schema-compat.yml` in your application repo
(adjust to your CI provider as needed):

```yaml
name: Schema Compatibility Check

on:
  pull_request:
    paths:
      - 'proto/**'
      - '.schema-snapshot.json'
      - 'path/to/your/registry-loader/**'

permissions:
  contents: read
  pull-requests: write

jobs:
  schema-compat:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install entdb-schema
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          VERSION=v1.0.0   # pin your version
          curl -L "https://github.com/elloloop/tenant-shard-db/releases/download/${VERSION}/entdb-schema-${VERSION}-linux-amd64.tar.gz" \
            | tar -xz
          install -m 0755 entdb-schema /usr/local/bin/entdb-schema

      - name: Check schema compatibility
        run: |
          # The one command — IDENTICAL to your local `make schema-breaking`.
          # Replace the --from-file source with however your CI produces the
          # current snapshot (`--from-server` if you boot the server in CI,
          # or `--from-file` against a snapshot regenerated by your build).
          entdb-schema breaking \
            --baseline schema.lock.json \
            --from-file ./build/current-snapshot.json \
            --format text

      - name: Allow if 'breaking-schema-change' label present
        if: failure() && contains(github.event.pull_request.labels.*.name, 'breaking-schema-change')
        run: |
          echo "::warning::Breaking-schema-change label present — accepting on the labelled PR."
          echo "Reviewer responsibility: ensure CHANGELOG has a BREAKING CHANGE entry."
```

### 2b. Wire the local recipe (identical invocation)

The whole point of the buf-breaking ergonomics is that the local and CI
commands are the **same**. Use a `make` target so the invocation lives
in one place:

```makefile
# Makefile (in your app repo)
SCHEMA_BASELINE ?= schema.lock.json
SCHEMA_CURRENT  ?= build/current-snapshot.json

schema-breaking:
	entdb-schema breaking --baseline $(SCHEMA_BASELINE) --from-file $(SCHEMA_CURRENT)
```

```bash
make schema-breaking   # same gate, same exit code, on your laptop
```

…or as a pre-commit hook (`.pre-commit-config.yaml`):

```yaml
repos:
  - repo: local
    hooks:
      - id: entdb-schema-breaking
        name: entdb-schema breaking
        entry: entdb-schema breaking --baseline schema.lock.json --from-file build/current-snapshot.json
        language: system
        pass_filenames: false
        files: '(^proto/|^schema\.lock\.json$)'
```

> This repo's own `Makefile` ships the same `schema-breaking` (and
> `schema-snapshot`) targets as a reference.

### 3. Done

Every subsequent PR runs the check. Non-breaking changes (adding a
field, dropping `unique`/`required`, adding/removing `indexed` or
`searchable`, removing a field/type whose id you reserve, appending an
enum value, …) pass silently; breaking changes (making a field
`unique`/`required`, changing a field `kind`, reusing a reserved id,
reordering enum values, …) fail the job and post a comment summarising
the violations.

## Opting in to a breaking change

Sometimes you *do* want to make a breaking change — a major-version
release, a migration window, a data-corruption fix that requires
re-keying. The opt-in is intentionally three-way visible:

1. **Regenerate the snapshot in the same PR** that makes the change.
   The diff in `.schema-snapshot.json` is part of code review.
2. **Apply the `breaking-schema-change` label to the PR.** This is
   the CI signal. The workflow accepts the change *only* when the
   label is present.
3. **Add a `BREAKING CHANGE:` entry to `CHANGELOG.md`** for the
   affected version. This is the audit trail visible after merge.

> **Why not just `breaking-change`?** That label is overloaded across
> SDK and wire semantics in EntDB's own repo, so this tool uses the
> `breaking-schema-change` name to scope the opt-in to runtime-schema
> compatibility only. Choose any label name you like in your own repo,
> just keep it consistent with the workflow's `contains(...)` check.

Branch protection should require a code-owner approval on any PR
carrying the label.

## Migrating from the Python tool

EntDB's deleted Python `schema_cli.py`
(`server/python/entdb_server/tools/schema_cli.py` at commit `8d07f5f^`)
produced snapshots in the same envelope shape. The Go tool reads
those files verbatim — no edits required.

The migration is a one-line CI swap:

```diff
-entdb schema check --baseline .schema-snapshot.json ...
+entdb-schema check --baseline .schema-snapshot.json ...
```

The Python SDK's stub `entdb` CLI (`sdk/python/entdb_sdk/cli.py`) is
not a supported replacement for production use; it parses `.proto`
files with a custom parser and does not see runtime registrations.
Deprecate it in your tooling and use `entdb-schema` instead.

The migration-path guarantee is enforced by the cross-implementation
contract test at `tests/contract/schema_python_parity_test.go` (the
fixture `tests/contract/fixtures/python-snapshot-v1.json` is a
captured Python-era snapshot; the test asserts the Go-computed
fingerprint byte-equals the Python-era fingerprint embedded in the
file).

## Snapshot shape reference

Under [ADR-031](../adr/031-self-describing-name-free-schema.md) the
server boots with an **empty** registry and learns each app's schema
from its self-describing writes — there is no boot seed and no
`--seed-profile`. So there is no canonical committed snapshot in *this*
repo to copy; you generate yours from your own proto.

The snapshot is **name-free**: node/edge types are keyed by
`type_id` / `edge_id`, fields by `field_id`
([ADR-018](../adr/018-field-id-keyed-payloads.md)). A minimal body looks
like:

```json
{
  "node_types": [
    {"type_id": 1, "fields": [
      {"field_id": 1, "kind": "str", "required": true, "unique": true},
      {"field_id": 2, "kind": "str"}
    ]}
  ],
  "edge_types": []
}
```

Optional per-field attributes (`indexed`, `searchable`, `deprecated`,
`pii`, `ref_type_id`, `enum_values`), per-type metadata (`data_policy`,
`subject_field`, `composite_unique`), and the reserved-id tombstones
(`reserved_field_ids`, `reserved_type_ids`, `reserved_edge_ids`) ride
the same document. Capture yours via `entdb-schema snapshot
--from-server` pointed at your actual server once your full schema has
been established by writes.

## `delete_where` and `QueryNodes` on schema-less deployments

Some deployments run `entdb-server` **without** a registered schema —
no `.schema-snapshot.json`, no `--seed-profile`, the registry empty by
design (a thin keyed-blob store, or an early bootstrap stage before the
schema is locked). That mode is fully supported for predicate
operations, with one rule you must know.

EntDB payloads are keyed on the wire and on disk by numeric
`field_id`, never by name ([ADR-018](../adr/018-field-id-keyed-payloads.md)).
The SDKs translate a human field *name* to its `field_id` from the
proto descriptor before the call. The **server** only needs to do that
translation for predicate *filters* — and it can only do it if it has a
schema. So the rule is symmetric across the two predicate surfaces:

- A filter **field name** (e.g. `"expires_at"`) requires a registered
  schema. Against a schema-less server it returns
  `INVALID_ARGUMENT: "cannot translate filter key … without a schema"`.
- A **digit-only numeric payload field id** (e.g. `"4"`) works
  schema-less: the server treats a digit-only key as a raw `field_id`
  and skips the lookup entirely.

This is the **same schema-optional escape hatch `QueryNodes` filters
already accept** — `delete_where` (the single-RPC predicate sweeper,
[issue #504](https://github.com/elloloop/tenant-shard-db/issues/504))
deliberately reuses the exact `FieldFilter` / `FilterOp` types, so the
behaviour is identical to `query(where=)` / `QueryWhere`. The
schema-less numeric-field-id requirement for `delete_where` is tracked
in [issue #545](https://github.com/elloloop/tenant-shard-db/issues/545).

Against a schema-**configured** server both forms work; against a
schema-**less** server only the numeric-id form does. Pick the proto
field number from your own schema (the field number IS the `field_id`,
[ADR-006](../adr/006-proto-schema-definition.md) /
[ADR-018](../adr/018-field-id-keyed-payloads.md)) — e.g. if
`expires_at` is field `4` in your `.proto`, pass `"4"`.

### Go

```go
// Schema-configured server: a field name is fine.
entdb.DeleteWhere[*auth.WebAuthnChallenge](plan,
    []entdb.Filter{{Field: "expires_at", Op: entdb.FilterLt, Value: nowMs}},
    1000)

// Schema-LESS server: pass the numeric field id ("expires_at" is
// proto field 4 in the caller's own schema). A name key here would
// fail with INVALID_ARGUMENT: "cannot translate filter key … without
// a schema". QueryWhere filters take the exact same numeric key.
entdb.DeleteWhere[*auth.WebAuthnChallenge](plan,
    []entdb.Filter{{Field: "4", Op: entdb.FilterLt, Value: nowMs}},
    1000)
```

### Python

```python
from entdb_sdk import Filter, FilterOp

# Schema-configured server: a field name is fine.
plan.delete_where(
    schema_pb2.WebAuthnChallenge,
    [Filter(field="expires_at", op=FilterOp.LT, value=now_ms)],
    limit=1000,
)

# Schema-LESS server: pass the numeric field id ("expires_at" is
# proto field 4 in the caller's own schema). A name key here raises
# INVALID_ARGUMENT: "cannot translate filter key … without a schema".
# The same numeric key works for query(where=) / QueryNodes filters.
plan.delete_where(
    schema_pb2.WebAuthnChallenge,
    [Filter(field="4", op=FilterOp.LT, value=now_ms)],
    limit=1000,
)
```

`limit` is best-effort (Postgres `DELETE … LIMIT` semantics): the
server caps it to a hard ceiling so a runaway predicate cannot pin a
tenant's single applier goroutine. Drain a large backlog by sweeping
in a loop until a sweep deletes nothing. The full SDK signature and
narrative live in [sdk-reference.md → Plan methods](../sdk-reference.md#plan-methods);
the wire-level op shape is in [api-reference.md → `ExecuteAtomic`
operations](../api-reference.md#executeatomic-operations).

## Caveats and follow-ups

- **`--from-descriptors` is reserved.** Loading the registry from a
  `buf build -o fds.bin` descriptor set is tracked under issue #488
  open question #6. Use `--from-file` or `--from-server` in v1.
- **One snapshot per cluster, not per tenant.** EntDB's schema is
  process-wide today (see `docs/go-port/shared/schema-registry.md`
  §5). If multi-tenant schema variants ever land, this guide will
  grow a per-tenant section.
- **Boot-time check (server-side).** A complementary check that
  refuses to start a server whose registry's fingerprint doesn't
  match a configured `.schema-snapshot.json` is a planned follow-up
  (issue #488 §Design alternatives, option E). It catches drift in
  production deploys; this guide is about PR-time prevention.

## Related

- Issue #488 — design and rationale (acceptance criteria, alternative
  shapes, performance budget).
- Issues [#504](https://github.com/elloloop/tenant-shard-db/issues/504)
  / [#545](https://github.com/elloloop/tenant-shard-db/issues/545) —
  the `delete_where` sweeper and its schema-less numeric-field-id
  requirement (see [above](#delete_where-and-querynodes-on-schema-less-deployments)).
- `docs/schema-evolution.md` — the rules `entdb-schema` enforces.
- `docs/go-port/shared/schema-registry.md` — the registry's runtime
  contract that the snapshot captures.
- `tests/contract/schema_python_parity_test.go` — the migration-path
  contract test for Python-era snapshots.
