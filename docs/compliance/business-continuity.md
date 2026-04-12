# Business Continuity and Disaster Recovery Plan

This plan describes how EntDB maintains service during disruption and
recovers from outages, data corruption, and disasters. It supports SOC
2 A1.2, ISO 27001 A.5.29–A.5.30, HIPAA § 164.308(a)(7), and tenant
contractual SLAs.

---

## 1. Objectives

### 1.1 Recovery Tiers

EntDB assigns each tenant to one of three recovery tiers based on the
customer contract. The tier determines RTO, RPO, and recovery
procedure.

| Tier | Description | RTO | RPO |
|------|-------------|-----|-----|
| **Tier 1** | Mission-critical, premium customers | **5 minutes** | **< 5 minutes** |
| **Tier 2** | Standard business customers | **1 hour** | **< 5 minutes** |
| **Tier 3** | Archive / compliance-only | **4 hours** | **< 5 minutes** |

- **RTO (Recovery Time Objective)** — maximum tolerable time from
  incident declaration to restored service.
- **RPO (Recovery Point Objective)** — maximum tolerable data loss
  measured in time. Across all tiers, RPO is bounded by Kafka
  durability, which is configured for `acks=all` and
  `min.insync.replicas >= 2` across multiple availability zones.

### 1.2 Service Level

Tier 1 targets **99.95%** monthly availability (≈ 22 minutes of
downtime/month). Tier 2 targets **99.9%** (≈ 43 minutes/month). Tier 3
is best-effort with contractual repair obligations.

---

## 2. Architecture Summary

EntDB has three durable layers. Each can reconstruct state if the
layer above fails.

```
           +--------------------------+
Tier A     | SQLite canonical store   |  hot, fast, read-write
           +-----------+--------------+
                       |
Tier B     | Kafka durable log        |  replicated, replayable
           +-----------+--------------+
                       |
Tier C     | S3 archive (sealed segs) |  cold, durable, cheap
           +--------------------------+
```

**Recovery relationships.**

- Tier A can be rebuilt from Tier B (Kafka replay) in minutes to tens
  of minutes, depending on data volume.
- Tier B can be rebuilt from Tier C (S3 segment replay) in hours.
- Tier C is protected by cross-region replication where contractually
  required.

See `dbaas/entdb_server/recovery_strategy.py` for the code path and
`tests/unit/test_recovery_strategy.py` for automated coverage.

---

## 3. Recovery Procedures

### 3.1 Tier 1 — SQLite restart / failover (RTO 5 min)

**When to use.** Process crash, node reboot, AZ evacuation. Canonical
store is intact and reachable.

**Procedure.**

1. Confirm the canonical store is intact via health check.
2. Restart the server process or fail over to a warm standby.
3. Server re-attaches to the Kafka consumer group and resumes from the
   committed offset.
4. Verify no missed events by comparing last SQLite commit offset vs
   Kafka high-watermark; the plan-commit guard rejects duplicates.
5. Run `pytest tests/unit/test_wait_for_offset.py` style smoke tests
   against the recovered shard.
6. Remove the shard from the "degraded" status list.

**Target time:** 1–5 minutes, bounded by container restart + warm
state rehydration.

### 3.2 Tier 2 — Kafka replay to rebuild SQLite (RTO 1 hr)

**When to use.** Canonical store is corrupted, lost, or inconsistent,
but the Kafka log is intact.

**Procedure.**

1. Declare the affected shards read-only.
2. Provision a new SQLite database file (empty schema).
3. Start the server in "rebuild from offset 0" mode.
4. The server consumes from the earliest Kafka offset and replays all
   events through the applier, rebuilding the canonical store.
5. Plan-commit idempotency ensures exactly-once semantics even if the
   process restarts mid-replay.
6. Validate totals against a golden count (event count per tenant,
   checksum of canonical store).
7. Re-enable writes.

**Target time:** depends on event count. For a shard holding 100M
events, empirical replay at ~5k events/sec is ≈ 5–6 hours; for Tier 2
tenants this SLA requires either smaller shards or parallelized
replay. Partition the replay across shards to meet the 1 hour target.

### 3.3 Tier 3 — S3 archive replay (RTO 4 hr)

**When to use.** Kafka retention has aged out and canonical store is
also lost. Only sealed archive segments exist.

**Procedure.**

1. Stage the affected archive segments from S3 to the recovery host.
2. Decrypt with the appropriate KMS key.
3. Feed segments through the archive replay tool, which writes events
   back into Kafka (or directly into a fresh canonical store in
   offline mode).
4. Once Kafka is repopulated, follow Tier 2 procedure to rebuild
   SQLite.
5. Re-enable writes after validation.

**Target time:** 2–4 hours, dominated by S3 fetch and decompression.
Code paths: `dbaas/entdb_server/archiver.py` and
`dbaas/entdb_server/recovery_strategy.py`.

### 3.4 Full regional failover

**When to use.** Entire AWS region (or equivalent) unavailable.

**Procedure.**

1. Declare a regional incident via the Incident Response Plan.
2. Promote the DR region standby.
3. Update DNS (Route 53 / equivalent) via Terraform workflow to point
   at the DR region.
4. Rehydrate KMS key material access for the DR region accounts.
5. Resume consumer groups.
6. Notify customers via status page and email per the incident
   response communication templates.

**Target time:** 30–60 minutes, driven by DNS TTL and warm standby
readiness.

---

## 4. Backup Strategy

| Layer | Mechanism | Frequency | Retention |
|-------|-----------|-----------|-----------|
| Kafka | Replicated log across 3 AZs, acks=all | Continuous | 7 days rolling |
| SQLite | Snapshot to S3 every 6 hours | Every 6h | 30 days |
| S3 archive | Sealed segments, cross-region replication (premium tiers) | Continuous | Per customer contract |
| Config (Terraform) | Git repo | Every commit | Indefinite |
| Secrets | Secret manager with versioning | Every rotation | 90 days |

Backups are restored in the annual DR test (§ 6) to prove they work.

---

## 5. Dependencies

EntDB depends on the following external providers. Each is covered by
a contract with a defined SLA and is on the subprocessor list.

| Dependency | Function | Failover plan |
|------------|----------|---------------|
| AWS/GCP compute | Server hosting | Multi-AZ, multi-region standby |
| KMS | Encryption keys | Regional KMS with key replication |
| S3 / GCS | Archive storage | Cross-region replication |
| Kafka (managed) | Durable log | Multi-AZ broker replication |
| DNS (Route 53) | Traffic routing | Health-checked failover |
| Monitoring (Prometheus/Grafana) | Observability | Secondary instance |
| Alerting (PagerDuty) | On-call paging | Secondary channel (Slack) |

---

## 6. Annual DR Test Procedure

A full DR test is conducted **once per calendar year**. The test
validates each recovery tier against its RTO/RPO targets.

### 6.1 Preparation (T-14 days)

1. Schedule the test with the customer success team (non-production
   tenants or pre-prod environment only).
2. Nominate an IC and test observers.
3. Pre-stage golden datasets with known checksums.
4. Announce the window on the internal status page.

### 6.2 Test 1 — Tier 1 restart (RTO 5 min)

- Kill the server process in one shard.
- Measure time to recovery.
- Validate zero data loss via checksums.

### 6.3 Test 2 — Tier 2 Kafka replay (RTO 1 hr)

- Delete the SQLite file from a test shard.
- Trigger full replay from Kafka offset 0.
- Measure wall time.
- Validate event counts and checksums.

### 6.4 Test 3 — Tier 3 S3 replay (RTO 4 hr)

- Delete both SQLite and simulate Kafka retention loss.
- Stage archive segments from S3.
- Rebuild Kafka log from archive.
- Rebuild SQLite from rebuilt Kafka.
- Validate final state.

### 6.5 Test 4 — Regional failover

- Freeze primary region.
- Fail over DNS and warm standby.
- Run smoke tests against DR region.
- Fail back.

### 6.6 Reporting (T+7 days)

Publish a DR test report to the ISO including:

- Measured RTO/RPO for each test.
- Deviations from target and root causes.
- Action items with owners and due dates.
- Evidence file (logs, checksums) for auditor review.

Store the report in `evidence/dr-test-<year>/`.

---

## 7. Roles

| Role | Responsibility |
|------|----------------|
| BCP/DR Owner | Maintains this plan, schedules tests |
| SRE Lead | Executes recovery procedures |
| Engineering Lead | Root cause analysis, code changes |
| ISO | Reviews test results, reports to management |
| CTO | Declares major disasters, approves region failover |

---

## 8. Review

This plan is reviewed annually or after any significant architecture
change. The next review date is recorded below.

- Last reviewed: **[DATE]**
- Next review: **[DATE]**
- Owner: BCP/DR Owner
- Approver: CTO
