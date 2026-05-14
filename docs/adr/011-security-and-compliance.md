# ADR-011: Security and Compliance from Day One

## Status: Accepted

## Context

EntDB powers products equivalent to Google Workspace and Microsoft 365. The database must have technical controls built in from day one to support SOC 2, ISO 27001, GDPR, CCPA, HIPAA, and future certifications without retrofit.

## Decision

### Encryption at rest

Every tenant SQLite file is encrypted using SQLCipher (AES-256-CBC) or an equivalent. The database never writes unencrypted data to disk.

```
tenant_acme.db     → encrypted with tenant-specific key
tenant_alice.db    → encrypted with different tenant-specific key
global.db          → encrypted with master key
```

Per-tenant encryption keys enable **crypto-shredding**: deleting a tenant's encryption key makes all data irrecoverable without touching the file. This satisfies GDPR erasure even on backup media.

Key hierarchy:
```
Master Key (HSM or KMS)
  └── Tenant Key (derived per tenant, stored encrypted in key_registry)
        └── SQLite file encrypted with tenant key
```

Key rotation: new writes use new key, background process re-encrypts existing data.

### Encryption in transit

TLS is mandatory, not optional. The server refuses plaintext gRPC connections in production.

```
gRPC:   TLS 1.3 minimum
Kafka:  TLS + SASL authentication
S3:     HTTPS only
Global: TLS between nodes
```

Certificate management: support for ACME (Let's Encrypt), custom certs, and mutual TLS (mTLS) for service-to-service.

### Audit logging

Audit posture is owned by **ADR-015** (WAL + S3 Object Lock is the
audit log). Every operation flows through the WAL; the WAL is archived
to S3 with Object Lock COMPLIANCE mode for tamper evidence. This ADR
does not duplicate that decision.

### Authentication

```
API Keys:
  - Per-tenant API keys with scoped permissions
  - Key rotation without downtime (multiple active keys)
  - Key revocation with immediate effect
  - Keys stored as bcrypt/argon2 hashes (never plaintext)

OAuth 2.0 / OIDC:
  - Support for external identity providers (Google, Microsoft, Okta)
  - JWT validation at the gRPC interceptor level
  - Token expiry enforced server-side

mTLS:
  - Mutual TLS for service-to-service communication
  - Certificate-based identity for automated systems
```

### Data residency

Tenants can be pinned to specific geographic regions.

```sql
-- Global store
CREATE TABLE tenant_registry (
    tenant_id   TEXT PRIMARY KEY,
    ...
    region      TEXT NOT NULL DEFAULT 'us-east-1',  -- where this tenant's data lives
    data_residency_policy TEXT,                       -- 'eu-only', 'us-only', 'any'
);
```

Multi-region deployment:
```
Region EU (Frankfurt):   owns tenants with region=eu-west-1
Region US (Virginia):    owns tenants with region=us-east-1
Region APAC (Singapore): owns tenants with region=ap-southeast-1
```

Data never leaves the configured region. Cross-region reads are routed to the owning region.

### Network security

```
IP allowlisting:    per-tenant IP restrictions
Rate limiting:      per-tenant, per-user, per-API-key (already exists)
DDoS protection:    connection limits, request size limits
Private networking: VPC peering, private endpoints (cloud deployments)
```

### Secure deletion

Three levels:
```
Soft delete:      status=deleted, data retained for grace period
Hard delete:      SQLite file removed from disk
Crypto-shred:     encryption key deleted, data on disk/backup is unrecoverable
```

GDPR deletion uses crypto-shred for personal tenants (key deleted after grace period).

### Input validation

```
Schema validation:  proto schema enforces field types at the SDK level
SQL injection:      parameterized queries only (no string interpolation)
Payload size:       configurable max per node (default 1MB)
Field validation:   type checking, enum validation, required field checks
FTS5 injection:     sanitized query input (already fixed)
Path traversal:     tenant_id sanitized for filesystem paths (already implemented)
```

### Vulnerability management

```
Dependencies:      Dependabot / Renovate for automated updates
SAST:              CodeQL or Semgrep in CI
Container scanning: Trivy or Grype on Docker images
Secret scanning:   git-secrets / truffleHog in CI
Penetration testing: annual third-party pentest
CVE monitoring:    automated alerts for SQLite, gRPC, protobuf, Kafka CVEs
```

### Monitoring and alerting

```
Metrics (Prometheus):
  - Request rate, error rate, latency (per RPC, per tenant)
  - SQLite connection pool utilization
  - Kafka consumer lag (applier falling behind)
  - Disk usage per tenant
  - Authentication failures
  - Rate limit hits
  - GDPR operation status

Tracing (OpenTelemetry):
  - Distributed traces across gRPC → WAL → Applier → SQLite
  - Per-tenant trace context

Health checks:
  - /health endpoint with component status
  - WAL connectivity, SQLite accessibility, disk space

Alerting:
  - Applier lag > 60 seconds
  - Authentication failure spike
  - Disk usage > 80%
  - Error rate > 1%
  - S3 Object Lock archive lag > N events (see ADR-015)
```

### Session and token management

```
API keys:       rotatable, scopeable, revocable, expirable
JWT tokens:     short-lived (15 min), refresh tokens (7 days)
Session timeout: configurable per tenant
Concurrent sessions: configurable limit per user
Token revocation: immediate via revocation list (checked on every request)
```

### Backup integrity

```
Backups:
  - Daily SQLite snapshots to S3 (encrypted)
  - Hourly WAL archive to S3 (encrypted)
  - Backup checksums verified on creation

Restore testing:
  - Automated monthly restore test (pick random tenant, restore, verify)
  - Restore time SLA: < 1 hour for any single tenant

Backup encryption:
  - Same tenant key used for backup encryption
  - Crypto-shred: deleting tenant key makes backups unrecoverable too
```

### Compliance mapping

| Control | SOC 2 | ISO 27001 | GDPR | HIPAA | CCPA |
|---|---|---|---|---|---|
| Encryption at rest | CC6.1 | A.10.1 | Art.32 | §164.312(a) | §1798.150 |
| Encryption in transit | CC6.1 | A.13.1 | Art.32 | §164.312(e) | — |
| Audit logging | CC7.2 | A.12.4 | Art.30 | §164.312(b) | — |
| Access control | CC6.1-6.3 | A.9 | Art.25 | §164.312(a) | — |
| Data classification | CC6.5 | A.8.2 | Art.30 | §164.312(a) | §1798.100 |
| Data residency | CC6.6 | A.11 | Art.44-49 | — | — |
| Encryption key mgmt | CC6.1 | A.10.1 | Art.32 | §164.312(a) | — |
| Secure deletion | CC6.5 | A.8.3 | Art.17 | §164.310(d) | §1798.105 |
| Backup integrity | CC7.5 | A.12.3 | Art.32 | §164.308(a) | — |
| Vulnerability mgmt | CC7.1 | A.12.6 | Art.32 | §164.308(a) | — |
| Monitoring | CC7.2 | A.12.4 | Art.33 | §164.308(a) | — |
| Incident response | CC7.3-7.5 | A.16 | Art.33-34 | §164.308(a) | §1798.150 |
| Input validation | CC6.1 | A.14.2 | Art.32 | §164.312(c) | — |
| Rate limiting | CC6.1 | A.13.1 | — | — | — |

### What EntDB provides vs what the deployment provides

```
EntDB provides (built into the database):
  ✅ Encryption at rest (SQLCipher)
  ✅ Encryption in transit (TLS required)
  ✅ Audit logging (WAL + S3 Object Lock; see ADR-015)
  ✅ Access control (ACL v2, role-based)
  ✅ Data classification (proto schema)
  ✅ GDPR operations (delete, export, anonymize, freeze)
  ✅ Secure deletion (crypto-shred)
  ✅ Input validation (schema, parameterized queries)
  ✅ Rate limiting
  ✅ Monitoring (Prometheus metrics, OpenTelemetry tracing)
  ✅ Backup with checksums
  ✅ Legal hold

The deployment provides (infrastructure):
  🔧 Network security (VPC, firewalls)
  🔧 Physical security (data center)
  🔧 KMS for master key (AWS KMS, GCP KMS, Vault)
  🔧 Certificate management
  🔧 DDoS protection (Cloudflare, AWS Shield)
  🔧 CI/CD pipeline security
  🔧 Employee access controls
  🔧 Incident response procedures
  🔧 Business continuity planning
  🔧 Third-party audit engagement
```

## Consequences

- SQLCipher adds ~5-10% overhead on SQLite operations
- Per-tenant encryption keys require key management infrastructure
- TLS-only means no plaintext development mode (use self-signed certs for dev)
- Data residency requires multi-region deployment for global customers
- Compliance documentation is an ongoing effort beyond technical controls
- Annual SOC 2 audit requires third-party auditor engagement
