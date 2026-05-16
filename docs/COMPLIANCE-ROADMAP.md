# Compliance Status & Roadmap

EntDB's compliance posture is documented in [ADR-011](adr/011-security-and-compliance.md). This page tracks what's shipped vs what's still needed for each common certification, with pointers to the code or open issues.

Legend:

- ✅ **Shipped** — feature is in v1.13.0 (or earlier)
- ⏳ **Planned** — designed, tracked in an issue
- 🔧 **Deployment** — provided by your infrastructure, not by EntDB
- 📋 **Process** — organizational, not a code feature

## Foundations (all ✅)

These controls are the floor for all certifications below.

| Control | Status | Where |
|---|---|---|
| Encryption at rest (SQLCipher AES-256 + HMAC-SHA512 + PBKDF2-SHA512) | ✅ | `server/go/internal/crypto/` |
| Per-tenant key derivation + KMS-backed master | ✅ | `crypto/key_manager.go`, `crypto/master_key.go` |
| Encryption in transit (TLS 1.3 + optional mTLS) | ✅ | `server/go/cmd/entdb-server/tls.go`, `server/go/internal/auth/mtls.go` |
| Cert reload-on-SIGHUP | ✅ | `tls.go` |
| Tamper-evident audit log (WAL + S3 Object Lock COMPLIANCE) | ✅ | ADR-015; `server/go/internal/audit/` |
| Crypto-shred for GDPR erasure | ✅ | `server/go/internal/gdpr/processor.go`, `crypto/sqlcipher.go` |
| Schema validation with data classification | ✅ | ADR-006; `(entdb.field).data_policy`, `(entdb.field).pii` proto options |
| Per-tenant SQLite isolation | ✅ | ADR-014 |
| Multi-tenant ACL (typed-capability) | ✅ | ADR-003 |
| Region pin per tenant | ✅ | `tenant_registry.region` column |

❌ **Not in EntDB** (by design, per ADR-015):

- Per-tenant `audit_log` SQLite table with hash chain — rejected. The WAL + S3 Object Lock is the audit log. Hash-chained tables in SQLite are weaker tamper evidence (any operator with file access can rewrite both the row and the chain) and redundant with the WAL.

## GDPR / CCPA

| Article / control | Status | Notes |
|---|---|---|
| Art. 17 — Right to erasure | ✅ | `Admin.DeleteUser` with configurable grace period; `gdpr/processor.go` runs the deletion-queue worker; crypto-shred wipes the tenant key, making data unreadable on disk AND on the S3 archive (which Object Lock won't let you delete). |
| Art. 20 — Right to data portability | ✅ | `Admin.ExportUserData` returns the user's data across all tenants |
| Art. 18 — Right to restriction of processing (freeze) | ✅ | `Admin.FreezeUser` blocks writes; reads continue |
| Art. 16 — Right to rectification | ✅ | `Admin.UpdateUser` |
| Art. 30 — Records of processing | ✅ | WAL events constitute the records (ADR-015) |
| Art. 32 — Security of processing (encryption, integrity) | ✅ | SQLCipher + WAL + Object Lock |
| Art. 33-34 — Breach notification | 📋 | Operator process — EntDB provides the data; org delivers the notice |
| Art. 44-49 — International data transfers | ✅ + 🔧 | `tenant.region` pin per tenant; multi-region deployment topology is operator-owned |
| Consent tracking | 📋 | Application-level concern; EntDB stores user data, not consent records (apps model consent as a node type if needed) |
| CCPA §1798.105 — Right to delete | ✅ | Same as GDPR Art. 17 |
| CCPA §1798.100 — Right to know | ✅ | Same as GDPR Art. 20 |
| CCPA §1798.150 — Private right of action (breach) | ✅ + 📋 | Encryption is the safe-harbor anchor; breach response is org process |

## SOC 2

| Control area | Status | Notes |
|---|---|---|
| CC6.1 — Logical access (TLS, mTLS, encryption) | ✅ | TLS 1.3 + mTLS; SQLCipher at rest |
| CC6.1-6.3 — Access control | ✅ | Typed-capability ACL (ADR-003) + admin gate on per-tenant ops |
| CC6.5 — Data classification + secure deletion | ✅ | `data_policy` per type; crypto-shred for erasure |
| CC7.1 — Vulnerability management | 📋 + 🔧 | Dependabot enabled, CodeQL recommended on forks; org-owned pen test cadence |
| CC7.2 — Logging and monitoring | ⏳ + 🔧 | WAL is the audit log (✅); Prometheus `/metrics` HTTP endpoint not yet exposed (recording works); OpenTelemetry tracing in [#517](https://github.com/elloloop/tenant-shard-db/issues/517) |
| CC7.3-7.5 — Incident response | 📋 | Operator process |
| Penetration testing | 📋 | Operator process |
| Vendor risk management | 📋 | Operator process |

A SOC 2 Type II audit is a 6–12-month observation window; the technical controls above need to be in place and observable throughout. EntDB provides the controls; the auditor reviews evidence over the window.

## ISO 27001

| Annex A control | Status |
|---|---|
| A.8 — Information classification (`data_policy`, `pii`) | ✅ |
| A.9 — Access control (typed-capability ACL) | ✅ |
| A.10.1 — Cryptographic controls | ✅ |
| A.12.3 — Backup (WAL durability + Object Lock archive) | ✅ |
| A.12.4 — Logging and monitoring | ✅ (audit) + ⏳ (metrics endpoint) |
| A.12.6 — Vulnerability management | 📋 + 🔧 |
| A.13.1 — Network security (TLS in transit) | ✅ + 🔧 (network topology) |
| A.14.2 — Secure development | 📋 |
| A.16 — Incident management | 📋 |
| A.17 — Business continuity (DR via WAL replay) | ✅ + 🔧 |

## HIPAA

For PHI / healthcare workloads:

| §164 control | Status |
|---|---|
| §164.308(a) — Security management | 📋 |
| §164.310(d) — Disposal (crypto-shred) | ✅ |
| §164.312(a) — Access control + encryption at rest | ✅ |
| §164.312(b) — Audit controls (WAL + Object Lock) | ✅ |
| §164.312(c) — Integrity (parameterized queries, schema validation) | ✅ |
| §164.312(e) — Transmission security (TLS) | ✅ |
| Business Associate Agreement (BAA) | 📋 — vendor-specific; see `docs/compliance/baa-template.md` |
| Risk assessment + workforce training | 📋 |

The `(entdb.field).data_policy = HIPAA` proto option marks PHI fields; the GDPR delete path includes them in the right-to-erasure cascade.

## Open implementation work

These EntDB features are designed but not yet shipped — they bear on compliance:

| Item | Status | Tracking |
|---|---|---|
| `/metrics` HTTP endpoint for Prometheus scrape | ⏳ | See ADR-011 monitoring status |
| OpenTelemetry instrumentation (server + SDK) | ⏳ | [#517](https://github.com/elloloop/tenant-shard-db/issues/517) |
| Rate limiting (the Python QuotaInterceptor wasn't ported) | ⏳ | Open work |
| Additional WAL backends (Kinesis, Pub/Sub, SQS, Service Bus, Event Hubs) | ⏳ | [#518](https://github.com/elloloop/tenant-shard-db/issues/518) |
| Per-tenant SQLite snapshots (faster recovery than WAL replay from offset 0) | ⏳ | Operator-owned for now; WAL replay is the durability mechanism |

## Process gaps that are NOT EntDB's job

EntDB provides technical controls. Compliance certifications also require organizational process:

- Policies and procedures (security policy, incident response, BCP, vendor management)
- Training records (annual security training, role-based)
- Risk assessments (annual or semi-annual)
- Vendor due diligence
- Auditor engagement and evidence packages
- Workforce access reviews
- Physical security of the deployment infrastructure

`docs/compliance/` carries templates for some of these — they're starting points for the auditing engagement, not turnkey deliverables.

## Quick-start: deploying EntDB for compliance-bound workloads

The production checklist in [operations.md § Production checklist](operations.md#production-checklist) covers the technical controls. In summary:

```bash
entdb-server \
  -addr=:50051 \
  -data-dir=/var/lib/entdb \
  -wal-backend=kafka -wal-brokers=... \
  -require-tls=true -require-client-cert=true \
  -tls-cert=... -tls-key=... -tls-ca=... \
  -kms-provider=aws -kms-key-id=... \
  -encryption-required=true \
  -gdpr-worker-enabled=true \
  -archive-enabled=true \
  -archive-bucket=... -archive-region=... \
  -archive-retention-days=2557
```

For HIPAA-bound deployments, additionally:

- Run the EntDB cluster in a HIPAA-eligible environment (AWS, GCP, Azure all have BAA-covered services).
- Ensure your Kafka / S3 / KMS provider is BAA-covered.
- Treat all PHI fields with `data_policy = HIPAA` in your proto schemas so the GDPR / right-to-erasure flow handles them correctly.

## Related

- [ADR-011](adr/011-security-and-compliance.md) — security + compliance design
- [ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md) — audit log posture
- [Operations](operations.md) — production checklist
- [Deployment](deployment.md) — production deployment topology
- `docs/compliance/` — process templates (BAA, BCP, SOC 2 evidence outline, etc.)
