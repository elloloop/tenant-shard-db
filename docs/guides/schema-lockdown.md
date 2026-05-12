# Schema lockdown with `entdb-schema`

Audience: downstream EntDB consumers (application owners building on top
of EntDB). This guide walks through using `entdb-schema` to lock down
your runtime schema and gate your application's pull requests against
silent breaking changes.

> Companion to `docs/schema-evolution.md` (which covers the rules
> themselves) and to issue #488 (which is the design reference).
> `entdb-schema` is shipped as a sibling binary to `entdb-server` —
> install once, use independently.

## What this solves

EntDB's runtime schema is a **registry of node and edge type
definitions**. Each carries a `type_id` and per-field `field_id` that
are the **on-disk keys**, plus metadata (`required`, `unique`,
`enum_values`, `data_policy`, `subject_field`, edge `on_subject_exit`,
…) that the WAL, applier, payload-translation layer, and query planner
all depend on.

A few common changes look harmless at the proto level but silently
break a running cluster:

| Change                                                | Effect                                                                                       |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Reassigning `field_id` 7 from `email` to `phone`      | Historic rows storing field 7 are now read back as "phone" — silent data corruption.         |
| Removing a node type                                  | WAL events referencing its `type_id` become unreplayable; rebuild halts.                     |
| Reordering enum values                                | Old WAL events carry the old integer; replay produces the wrong logical value.               |
| `data_policy` downgrade (e.g. `FINANCIAL` → `BUSINESS`) | Encryption tier weakens for historical rows.                                                 |
| Flipping `on_subject_exit: from` → `both` on an edge  | GDPR delete semantics shift for historical edges.                                            |

`buf breaking` does **not** catch any of these — they live in the
runtime registry, not the proto wire contract. `entdb-schema` is the
complementary tool: `buf breaking` defends the wire, `entdb-schema`
defends the runtime.

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
  check      Verify the current schema is compatible with a baseline.
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

### `check`

Compare the current schema against a baseline. This is the primary
CI command.

```bash
entdb-schema check \
  --baseline .schema-snapshot.json \
  --from-server localhost:50051 \
  --format json
```

Flags:

- `-b, --baseline PATH` — required.
- `--from-file | --from-server | --from-descriptors` — source of the
  current schema (same semantics as `snapshot`).
- `--format text|json` — `text` for human PR comments, `json` for CI
  parsing.
- `--allow-breaking` — exit `0` even if breaking changes found. The
  changes are still printed. Equivalent to the `breaking-schema-change`
  label override in CI, but at the binary level.

Sample text output (breaking case):

```
BREAKING: 2 breaking change(s) detected against .schema-snapshot.json

  [BREAKING] FIELD_ID_CHANGED
    path:    node:User.field:email
    message: field 'email' on User changed field_id from 3 to 7

  [BREAKING] ENUM_VALUE_REMOVED
    path:    node:Order.field:status.enum:CANCELLED
    message: enum value 'CANCELLED' (value 4) removed from Order.status

Non-breaking changes (informational): 1
  [OK] FIELD_ADDED        node:User.field:phone — phone (field_id=8) added
```

Same data shape in `--format json`:

```json
{
  "compatible": false,
  "breaking_count": 2,
  "non_breaking_count": 1,
  "changes": [
    {
      "kind": "FIELD_ID_CHANGED",
      "path": "node:User.field:email",
      "old_value": 3,
      "new_value": 7,
      "message": "field 'email' on User changed field_id from 3 to 7",
      "breaking": true
    },
    ...
  ]
}
```

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

Boot your `entdb-server` against an empty data dir (or your usual
local environment) and snapshot its registry:

```bash
entdb-server -addr=:50051 -data-dir=/tmp/entdb-init -wal-backend=memory &
SERVER_PID=$!

# Wait for the server to bind, then snapshot.
sleep 1
entdb-schema snapshot --from-server localhost:50051 > .schema-snapshot.json

kill $SERVER_PID
```

Commit the file at the repo root:

```bash
git add .schema-snapshot.json
git commit -m "chore: lock initial schema snapshot"
```

`.schema-snapshot.json` is a **plain checked-in file** — not a
generated artifact, not git-LFS, not gitignored.

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
          # Replace this with however your CI gets the current schema —
          # `--from-server` if you boot the server in CI, or `--from-file`
          # against a snapshot regenerated by your build step.
          entdb-schema check \
            --baseline .schema-snapshot.json \
            --from-file ./build/current-snapshot.json \
            --format text

      - name: Allow if 'breaking-schema-change' label present
        if: failure() && contains(github.event.pull_request.labels.*.name, 'breaking-schema-change')
        run: |
          echo "::warning::Breaking-schema-change label present — accepting on the labelled PR."
          echo "Reviewer responsibility: ensure CHANGELOG has a BREAKING CHANGE entry."
```

### 3. Done

Every subsequent PR runs the check. Non-breaking changes (adding a
field, marking deprecated, appending an enum value, …) pass silently;
breaking changes (reassigning field ids, removing types, reordering
enum values, …) fail the job and post a comment summarising the
violations.

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

## About this repository's `.schema-snapshot.json`

The `.schema-snapshot.json` committed at the root of *this* repo is
intentionally minimal: it captures exactly what
`server/go/internal/testseed.RegisterContractSchema` registers — the
`User` / `Task` / `AssignedTo` types with bare `field_id` / `kind` /
`name` only. No `required`, no `unique`, no `data_policy`, no
`subject_field`, no `default_acl`, no `composite_unique`, no
`enum_values`, no `pii`. That matches the contract test fixture that
the integration suite runs against and lets the CI schema-compat job
boot the server with `--seed-profile=contract` and get a byte-clean
match.

It is **not** a complete example of what a downstream consumer's
snapshot should look like. A richer reference snapshot exercising
those fields lives at
`tests/contract/fixtures/python-snapshot-v1.json` — a captured
Python-era snapshot used by the cross-implementation parity test. Use
that as the shape reference when you're building your own snapshot for
production use; capture yours via `entdb-schema snapshot --from-server`
pointed at your actual server (with your full registry registered, not
just the contract-test seed).

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
- `docs/schema-evolution.md` — the rules `entdb-schema` enforces.
- `docs/go-port/shared/schema-registry.md` — the registry's runtime
  contract that the snapshot captures.
- `tests/contract/schema_python_parity_test.go` — the migration-path
  contract test for Python-era snapshots.
