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

## Unique-constraint enforcement: when it actually fires (issue #601)

`(entdb.field).unique = true` and `(entdb.node).composite_unique` are enforced **only after** the server's registry knows about the type. Under v2's ADR-031 model the server **boots empty** and learns each type from a `SchemaDescriptor` the SDK auto-attaches on the first write for that connection. So in v2.x with the official SDKs:

1. App boots, opens a `DbClient` (the SDK builds a name-free `SchemaDescriptor` from its proto registry).
2. First write of a uniquely-keyed type → SDK attaches the descriptor → server prepends a `register_schema` op → applier registers the type **and creates the per-tenant unique expression index** before the data op lands.
3. From that moment on, duplicate writes for that key surface as `*UniqueConstraintError` (gRPC `ALREADY_EXISTS`).

The annotation is **not silently ignored** in v2.x. If you observe duplicates landing on a uniquely-annotated field, check one of:

- **Old SDK.** Pre-v2.0.2 the Go SDK dropped `(entdb.field).ref_type_id` from the descriptor, so `register_schema` was rejected for any node with a `kind:"ref"` field — schema never landed → unique never enforced. Upgrade to **v2.0.2+** (or **v2.0.4+** for the wider attribute coverage and the `Plan.Commit(ctx, WithWaitApplied(true))` option from #606 that surfaces the loser's `ALREADY_EXISTS` synchronously).
- **Hand-rolled client / schema not attached.** Anything that talks to the server without sending a `SchemaDescriptor` keeps the registry empty for that type, and unique stays metadata-only (the proto option ships but no SQLite expression index is ever created). Use an official SDK or call `register_schema` explicitly.
- **`Plan.Commit` without `WithWaitApplied(true)`.** Without it, the *loser* of a concurrent unique-race gets a phantom success: a UUID returned synchronously, the applier rejecting the write seconds later, no error surfaced to the caller. Use `WithWaitApplied(true)` (issue #606) on writes whose uniqueness outcome you need to react to.

If you genuinely want to run the server schemaless and rely on the annotation, you can't — the annotation is a *client-side* declaration that flows to the server via the SDK's `SchemaDescriptor`. There is no server-side path that consults proto options without the descriptor.

## Recommended Go codegen toolchain (issue #602)

Wire **`protoc-gen-entdb-keys`** into your `buf generate` pipeline so the SDK's typed `UniqueKey[T]` tokens are generated from your proto rather than hand-constructed. Hand-construction couples your code to an internal wire format the SDK explicitly disclaims as unstable.

Install the plugin once (each release tags the tools submodule too):

```bash
go install github.com/elloloop/tenant-shard-db/tools/protoc-gen-entdb-keys@v2.0.4
```

Then drop [`docs/recipes/buf.gen.yaml.template`](recipes/buf.gen.yaml.template) into your repo as `buf.gen.yaml`. It generates `protoc-gen-go` + `protoc-gen-go-grpc` + `protoc-gen-entdb-keys` sidecars in one `buf generate` pass. The generated tokens are then the only way to call `sdk.GetByKey`, and any future change to the token shape only ripples through codegen rather than every consumer.

## What is *not* breaking

- The `schema_fingerprint` on `ExecuteAtomic` still exists; the SDK manages it for you.
- Idempotency keys behave identically.
- gRPC RPC names and routes are unchanged.
- ACL, GDPR, mailbox, FTS surfaces are unchanged.

## Where to ask

If you hit a migration issue that isn't on this page, file it against the repo and tag it `v2-migration` — it likely belongs in this doc.
