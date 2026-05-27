# Migrating to EntDB v2

EntDB v2 is a wire-breaking major bump anchored by [ADR-031](adr/031-self-describing-name-free-schema.md) (self-describing, name-free schema) and [ADR-032](adr/032-schema-evolution-compat-rules.md) (compat-evolution rules + customer-runnable `entdb-schema breaking` gate). This page covers the migration friction points that aren't obvious from the ADRs.

If you upgrade server and SDKs together — which is the documented requirement — you should hit none of these in production. They mostly bite during the upgrade pass itself, in CI scripts that templated against v1 conventions.

## Breaking changes you may hit while upgrading

### 1. `kind = "string"` is rejected

v1 silently accepted any free-text kind in `(entdb.field).kind = "..."` because schema validation didn't run in schemaless mode. v2 enforces the canonical FieldKind list at register-schema time and rejects anything outside it:

```
INVALID_ARGUMENT: unknown kind "string"
```

**Canonical kinds:** `str`, `int`, `float`, `bool`, `timestamp`, `json`, `bytes`, `enum`, `ref`, `list_str`, `list_int`, `list_ref`.

The most common offender is `kind = "string"` (use `str`). A one-liner to find and fix:

```bash
git grep -l 'kind: "string"' '*.proto' | xargs sed -i 's/kind: "string"/kind: "str"/g'
```

### 2. `entdb-server` requires `--data-dir`

v1.32.x defaulted `--data-dir` to a temp path; v2 fails fast if it isn't set:

```
entdb-server: --data-dir is required
```

Pass it explicitly — even for ephemeral in-memory WAL runs:

```bash
entdb-server --addr=:50051 --data-dir=/tmp/entdb --wal-backend=memory
```

CI / testcontainers configs that relied on the v1 default need a one-line addition.

### 3. Image tag spelling

The git tag is `vX.Y.Z` (with the leading `v`); the published Docker image tag historically stripped the `v` to `X.Y.Z`. As of v2.0.3 the release workflow publishes **both** forms, so you can use whichever spelling matches your `${VERSION}` variable:

```yaml
# both of these resolve to the same image after v2.0.3:
image: ghcr.io/elloloop/tenant-shard-db:v2.0.3
image: ghcr.io/elloloop/tenant-shard-db:2.0.3
```

Pre-v2.0.3 releases (v2.0.0 / v2.0.1 / v2.0.2) only have the no-`v` form (`2.0.0`, `2.0.1`, `2.0.2`). If you templated against `${VERSION}` containing the `v`, either drop the prefix in your template or bump to v2.0.3+.

## SDK behaviour changes

### Go SDK module path

`go get github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2@v2.0.3` and `import "github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2"` — the `/v2` suffix is mandatory at v2 and above (Go modules major-version rule).

### Self-describing writes

Both SDKs auto-attach a `SchemaDescriptor` on the first write per connection and omit it once the server's fingerprint matches (ADR-031). You don't need to call anything new — the SDK reads your proto via `register_proto_schema` (Python) / generated code (Go) and handles the handshake.

### Server is id-only

The server now rejects name-keyed payloads, filters, and `UpdateNodePrecondition.field` strings with `INVALID_ARGUMENT`. Official SDKs translate name → field_id client-side before the call, so app code is unchanged. Hand-rolled clients need to send id-keyed.

### `Plan.Commit(ctx, WithWaitApplied(true))` (Go SDK)

As of v2.0.3 the Go SDK's `Plan.Commit` accepts `CommitOption` values, including `WithWaitApplied(bool)` and `WithWaitTimeout(time.Duration)`. Set `WithWaitApplied(true)` on writes whose **uniqueness or precondition outcome the caller needs to react to** — without it, the loser of a unique-constraint race gets a phantom success and only discovers the failure via a follow-up read. The Python SDK has exposed `wait_applied=True` since v2.0.0.

## What is *not* breaking

- The `schema_fingerprint` on `ExecuteAtomic` still exists; the SDK manages it for you.
- Idempotency keys behave identically.
- gRPC RPC names and routes are unchanged.
- ACL, GDPR, mailbox, FTS surfaces are unchanged.

## Where to ask

If you hit a migration issue that isn't on this page, file it against the repo and tag it `v2-migration` — it likely belongs in this doc.
