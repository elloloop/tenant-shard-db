# SOC 2 Evidence Collection Guide

This document lists the evidence required to support a SOC 2 Type I or Type II
audit for EntDB and identifies where each piece of evidence lives in the
codebase, how to generate or export it, and sample auditor questions with
suggested answers.

EntDB is a multi-tenant, shard-based, append-only database with a
Kafka-durable log, SQLite canonical store, and S3 archive tier. All evidence
paths below assume the repository root is `$REPO`.

Run `python scripts/collect_soc2_evidence.py` to auto-generate a timestamped
`evidence/<YYYY-MM-DD>/` directory containing structured JSON reports for the
controls listed here.

---

## Scope

EntDB offers the following trust-relevant services:

- Durable, tenant-scoped event storage (Kafka, SQLite, S3)
- Role- and attribute-based access control (ACL v2)
- Encryption in transit (TLS) and at rest (KMS-backed)
- Audit logging for administrative and data-plane operations
- Backup, archival, and tier-based recovery

The SOC 2 scope covers the EntDB server (`dbaas/entdb_server/`), SDK
(`sdk/`), and operational controls (`terraform/`, CI workflows).

---

## Trust Service Criteria — Evidence Map

### CC1.0 — Control Environment

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC1.1 Integrity and ethical values | Code of conduct, CLA | `$REPO/CLA.md`, `$REPO/CONTRIBUTING.md` |
| CC1.2 Board oversight | Governance doc (external) | Out of repo |
| CC1.3 Management philosophy | ADRs, architecture docs | `$REPO/docs/` |
| CC1.4 Commitment to competence | Contributor guide, review requirements | `$REPO/CONTRIBUTING.md` |
| CC1.5 Accountability | Git blame, code review logs | `git log`, PR history |

**Sample question:** "How does management establish accountability for the
security of customer data?"

**Answer:** EntDB uses signed commits, mandatory PR review (see CI config),
and role-based write access to main. All production changes are traceable
via Git history and CI workflow logs. The ADRs under `docs/adr/` document
architectural decisions affecting security.

### CC2.0 — Communication and Information

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC2.1 Internal communication | Slack logs, release notes | `$REPO/docs/releases/` |
| CC2.2 External communication | Changelog, public README | `$REPO/README.md`, `$REPO/VERSION` |
| CC2.3 Issue reporting | GitHub Issues, security policy | `$REPO/SECURITY.md` |

**Sample question:** "How do customers report security issues?"

**Answer:** `SECURITY.md` documents the vulnerability disclosure process.
Customers may report issues via a dedicated email alias; triage SLA is 24
hours for P1/P2.

### CC3.0 — Risk Assessment

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC3.1 Objectives specification | Product requirements, ADRs | `$REPO/docs/adr/` |
| CC3.2 Risk identification | Threat model, risk register | `$REPO/docs/compliance/iso27001-isms.md` |
| CC3.3 Fraud risk | Access log reviews | SQLite `audit_log` table |
| CC3.4 Change risk | ADR process, migration plans | `$REPO/docs/adr/`, `$REPO/docs/schema-evolution.md` |

### CC4.0 — Monitoring Activities

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC4.1 Ongoing evaluations | CI runs, test coverage reports | `.github/workflows/`, `coverage.xml` |
| CC4.2 Deficiency communication | Issue tracker, incident log | `$REPO/docs/compliance/incident-response.md` |

### CC5.0 — Control Activities

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC5.1 Control selection | `docs/compliance/iso27001-isms.md` SoA | Statement of Applicability |
| CC5.2 Technology controls | Encryption, ACL, TLS configs | `dbaas/entdb_server/config.py`, `terraform/` |
| CC5.3 Policies and procedures | This directory | `docs/compliance/` |

### CC6.0 — Logical and Physical Access Controls

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC6.1 Access restriction | ACL v2 implementation | `dbaas/entdb_server/acl.py`, `tests/unit/test_acl_v2.py` |
| CC6.2 Account management | Registry, principal store | `dbaas/entdb_server/registry.py` |
| CC6.3 Authorization enforcement | Server-side check in every RPC | `dbaas/entdb_server/server.py` |
| CC6.4 Physical access | Cloud provider (AWS/GCP) attestation | External |
| CC6.5 Data disposal | Archive retention + delete APIs | `dbaas/entdb_server/archiver.py` |
| CC6.6 External threat protection | TLS, mTLS, rate limiter | `dbaas/entdb_server/rate_limiter.py` |
| CC6.7 Data in transit | TLS config, cert rotation | `terraform/`, deployment docs |
| CC6.8 Malware prevention | Dependency scanning | `.github/workflows/` |

**Sample question:** "How is access to tenant data restricted?"

**Answer:** EntDB uses ACL v2 (`dbaas/entdb_server/acl.py`), which enforces
principal → tenant → kind → operation checks on every RPC. Tests are in
`tests/unit/test_acl_v2.py` and `test_acl_v2_extended.py`. The `auth`
module (`dbaas/entdb_server/auth.py`) authenticates callers via mTLS or
bearer token before ACL checks run.

### CC7.0 — System Operations

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC7.1 Vulnerability monitoring | Dependabot, pip-audit | `.github/workflows/`, `requirements.txt` |
| CC7.2 Anomaly detection | Metrics, tracing, audit logs | `dbaas/entdb_server/metrics.py`, `tracing.py` |
| CC7.3 Incident evaluation | Incident response procedure | `docs/compliance/incident-response.md` |
| CC7.4 Incident response | Runbooks | `docs/compliance/incident-response.md` |
| CC7.5 Recovery | Recovery strategy | `dbaas/entdb_server/recovery_strategy.py`, `docs/compliance/business-continuity.md` |

### CC8.0 — Change Management

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC8.1 Change authorization | PR approval, signed commits | Git history |
| CC8.1 Change tracking | Git log, CHANGELOG | `git log`, `$REPO/VERSION` |
| CC8.1 Testing | Unit + integration tests | `tests/`, CI workflow runs |
| CC8.1 Rollback | Deployment history | `terraform/`, release tags |

### CC9.0 — Risk Mitigation

| Criterion | Evidence | Location |
|-----------|----------|----------|
| CC9.1 Business disruption | BCP/DR plan | `docs/compliance/business-continuity.md` |
| CC9.2 Vendor management | Subprocessor list | `docs/compliance/baa-template.md` (Schedule A) |

---

## Availability (A1)

| Criterion | Evidence | Location |
|-----------|----------|----------|
| A1.1 Capacity planning | Sharding config, rate limiter | `dbaas/entdb_server/sharding.py`, `rate_limiter.py` |
| A1.2 Environmental protections | Cloud provider attestation | External |
| A1.3 Recovery testing | Annual DR test log | `docs/compliance/business-continuity.md` |

## Confidentiality (C1)

| Criterion | Evidence | Location |
|-----------|----------|----------|
| C1.1 Confidential information identified | Data classification | `docs/compliance/iso27001-isms.md` |
| C1.2 Confidential information disposed | Archive delete procedure | `archiver.py` |

## Processing Integrity (PI1)

| Criterion | Evidence | Location |
|-----------|----------|----------|
| PI1.1 Processing definition | Schema validator, canonical store | `schema_validator.py`, `canonical_store.py` |
| PI1.2 Inputs validated | Schema enforcement at write | `tests/unit/test_schema_validator.py` |
| PI1.3 Processing monitored | Metrics, tracing | `metrics.py`, `tracing.py` |
| PI1.4 Outputs reviewed | Mailbox store, replay | `mailbox_store.py` |
| PI1.5 Storage integrity | Plan commit guard, idempotency | `tests/unit/test_plan_commit_guard.py` |

## Privacy (P1–P8)

See `docs/compliance/privacy-policy.md` for consumer-facing language and
`docs/compliance/iso27001-isms.md` Annex A.18 for the control mapping.

---

## Evidence Generation

### Automated (preferred)

```bash
python scripts/collect_soc2_evidence.py
# Creates evidence/<date>/ with one JSON file per control family:
#   cc1_governance.json
#   cc6_access_control.json
#   cc6_encryption.json
#   cc7_monitoring.json
#   cc8_change_management.json
#   dependencies.json
#   audit_log_schema.json
```

Upload the generated directory to the audit workspace (e.g. Drata, Vanta,
Tugboat Logic). Retain for one year minimum (SOC 2 Type II review period).

### Manual

1. Export CI workflow run history for the audit period:
   `gh run list --limit 1000 --json databaseId,conclusion,createdAt,workflowName`.
2. Export git log with commit signatures:
   `git log --since="2024-01-01" --format="%H %aI %an %s %G?" > git_log.txt`.
3. Export the current ACL test suite output:
   `pytest tests/unit/test_acl_v2.py tests/unit/test_acl_v2_extended.py -v > acl_tests.txt`.
4. Export audit log schema: `sqlite3 canonical.db ".schema audit_log"`.

---

## Sample Auditor Walkthroughs

### Walkthrough 1 — Write Path

Auditor asks: "Walk me through how a customer write is authorized, validated,
durably stored, and logged."

1. Client sends `Append` RPC with bearer token.
2. `auth.py` verifies token and attaches `Principal`.
3. `acl.py` checks `(principal, tenant, kind, write)`.
4. `schema_validator.py` validates payload against registered schema.
5. `plan_commit_guard` ensures idempotency.
6. Server writes to Kafka (durable) and SQLite (canonical).
7. `metrics.py` increments write counters; `tracing.py` emits span.
8. Audit log row written with `(timestamp, principal, tenant, kind, op, result)`.

### Walkthrough 2 — Key Rotation

Auditor asks: "Show me how encryption keys are rotated without downtime."

1. KMS rotation policy (cloud provider).
2. `config.py` reads key alias, not key ID.
3. Archiver re-encrypts new segments with the active key.
4. Old segments remain readable via KMS key history.
5. Rotation verified via DR test (annual).

---

## Retention

Evidence files and audit reports are retained for **seven years** to cover
SOC 2 Type II annual refresh cycles and to support downstream HIPAA and
ISO 27001 audits.
