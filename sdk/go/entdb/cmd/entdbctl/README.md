# entdbctl

A kubectl-style read-only CLI for inspecting a deployed [EntDB](https://github.com/elloloop/tenant-shard-db) instance.

## Install

Prebuilt static binaries for `darwin/linux/windows × amd64/arm64` are attached to every [GitHub release](https://github.com/elloloop/tenant-shard-db/releases). Or:

```bash
go install github.com/elloloop/tenant-shard-db/sdk/go/entdbctl@latest
```

## Configure

| Flag        | Env var          | Default            |
|-------------|------------------|--------------------|
| `--addr`    | `ENTDB_ADDR`     | `localhost:50051`  |
| `--api-key` | `ENTDB_API_KEY`  | (none)             |
| `--actor`   | `ENTDB_ACTOR`    | (none)             |
| `--format`  | —                | `table` (or `json`) |
| `--timeout` | —                | `30s`              |

The actor (`user:alice`, `system:admin`, …) is required by most read RPCs for ACL filtering.

## Examples

```bash
# Liveness probe
entdbctl health

# List + inspect tenants
entdbctl tenants list
entdbctl tenants get acme

# Browse a tenant's schema
entdbctl schema show acme

# List nodes (numeric type_id or registered name)
entdbctl nodes list acme --type Product --limit 20
entdbctl nodes get acme node-123 --type 201

# Edges (default --out)
entdbctl edges acme node-123 --type OWNS
entdbctl edges acme node-123 --in

# One-hop neighbours
entdbctl graph acme node-123

# Mailbox FTS
entdbctl search acme "shipping label" --user alice

# Pipe into jq
entdbctl nodes list acme --type Product --format=json | jq '.nodes[].node_id'
```

## What it doesn't do

`entdbctl` is read-only. Writes (CreateTenant, ExecuteAtomic, ShareNode, GDPR ops, …) go through the typed Go SDK or `entdb` CLI; this tool deliberately stays narrow so the binary works against any tenant without proto descriptors.
