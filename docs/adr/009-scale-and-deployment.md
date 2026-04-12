# ADR-009: Scale Targets and Deployment Architecture

## Status: Accepted

## Context

EntDB must scale from a single-node development setup to multi-node production serving thousands of tenants with up to 1000 users each.

## Decision

### Scale target

```
Per tenant:     up to 1,000 users, up to 1M nodes, up to 100GB SQLite
Per deployment: up to 50,000 tenants
Total users:    up to 50M (across all tenants)
Write rate:     up to 100k events/second (across all tenants)
Read rate:      up to 1M reads/second (from SQLite, local disk)
```

### Single-node deployment (development / small production)

```
One EntDB server process:
  - gRPC server (port 50051)
  - Tenant applier (consumes from Kafka)
  - Global applier (consumes from entdb-global)
  - Fanout workers (consumes from entdb-fanout)
  - All tenant SQLite files on local disk
  - Global store as SQLite file

External dependencies:
  - Kafka (or local WAL for development)
  - S3-compatible storage (or local filesystem for development)

Runs on: 0.25 vCPU / 256 MB (free tier machines)
Handles: ~100 tenants, ~10k users
```

### Multi-node deployment (production)

```
Node A: owns tenants [acme, alice, bob, ...]
Node B: owns tenants [globex, carol, dave, ...]
Node C: owns tenants [...]

Shared:
  - Kafka cluster (3 brokers)
  - Global store (Postgres or distributed SQLite)
  - S3 for archives and snapshots

Routing:
  - ASSIGNED_TENANTS env var per node
  - gRPC server rejects requests for tenants not owned by this node
  - Client-side routing or load balancer with tenant-aware routing
```

### Capacity planning (1000 users/tenant, collaboration workload)

```
Per tenant:
  Writes:     50k events/day = 0.6/second sustained, 50/second burst
  Storage:    25 MB/day, ~9 GB/year
  SQLite:     one file, WAL mode
  
  Per node (handling 100 tenants):
  Writes:     5M events/day = 58/second sustained
  Storage:    2.5 GB/day, ~900 GB/year
  SQLite:     100 files, connection pool with LRU eviction
  
  Kafka (12 partitions, 3 brokers):
  Throughput: 58 events/second (trivial for Kafka)
  Disk:       7 days × 2.5 GB = 17.5 GB
  
  S3:
  Archive:    ~900 GB/year, ~$18/month
  Snapshots:  ~5 GB (100 tenants × 50 MB avg), daily
```

### Recovery

```
Tier 1: SQLite exists           → read directly (0 downtime)
Tier 2: SQLite lost, Kafka ok   → replay from Kafka (seconds-minutes)
Tier 3: Kafka lost too          → restore snapshot from S3 + replay archive (minutes-hours)
```

### High availability

```
- Kafka replication factor 3, min.insync.replicas 2
- S3 for durable archive (11 nines durability)
- Tenant SQLite files: daily snapshots to S3
- Global store: replicated Postgres (or multi-region SQLite with Litestream)
- Multi-node: if one node fails, reassign its tenants to other nodes
```

## Consequences

- 0.25 vCPU machines are viable for small deployments
- Multi-node requires shared Kafka + global store
- Tenant reassignment on node failure requires coordination
- Very large tenants (>100GB) need per-tenant sharding (future work)
- Connection pool size limits concurrent active tenants per node
