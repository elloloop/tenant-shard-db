# ADR-011: Security and Compliance from Day One

**Status:** Accepted
**Decided:** original ADR date predates current convention; revised
2026-05-16 to match shipped reality (v1.13.0 closed the major
encryption + TLS gaps)
**Tags:** security, compliance, encryption, gdpr, hipaa, soc2
**Implementation:** `server/go/internal/{crypto,gdpr,auth}/` + the
CLI flags on `cmd/entdb-server/`

## Context

EntDB targets workloads at the bar of Google Workspace or
Microsoft 365 — multi-tenant SaaS that must pass SOC 2, ISO 27001,
GDPR, CCPA, HIPAA. The technical controls below are designed in
from day one rather than bolted on per audit. Sections marked
**Shipped** are in v1.13.0 today; sections marked **Planned** are
design-locked but tracked in open issues.

## Decision

### Encryption at rest — **Shipped (v1.13.0)**

Every per-tenant SQLite file, every per-user mailbox file, and
`global.db` itself are encrypted using **SQLCipher v4** (AES-256,
HMAC-SHA512, PBKDF2-SHA512) via the
`github.com/mutecomm/go-sqlcipher/v4` driver. The driver is wired
into `server/go/internal/crypto/sqlcipher.go` and used by
`server/go/internal/store/pool.go` on every connection open.

```
tenant_acme.db        → encrypted with the acme-tenant key
tenant_smith.db       → encrypted with the smith-tenant key
{tenant}/user_X.db    → encrypted with the same tenant key
global.db             → encrypted with the master key
```

Key hierarchy (`server/go/internal/crypto/{key_manager,master_key,tenant_key_vault}.go`):

```
Master Key  — provisioned out-of-band (KMS / Vault / file)
  └── Tenant Key (derived per tenant, wrapped under master,
                  persisted in the tenant_key_vault table in global.db)
        └── SQLite file encrypted at PRAGMA-key open time
```

**KMS providers supported** (via `-kms-provider` flag):

- `file` — local key file (dev / single-node) or env-var lookup
- `aws` — AWS KMS
- `vault` — HashiCorp Vault
- `gcp` — Google Cloud KMS *(flag-recognized but not implemented in this binary; errors at boot)*
- `azure` — Azure Key Vault *(flag-recognized but not implemented in this binary; errors at boot)*

Operator flags (`cmd/entdb-server/main.go`):

```
-kms-provider {file|aws|gcp|azure|vault}
-kms-key-id <provider-specific identifier>
-encryption-required          # refuse to open unencrypted files
```

`-encryption-required=true` is the production posture: the server
refuses to start (or to open a tenant file) that isn't encrypted.

### Crypto-shred for GDPR erasure — **Shipped (v1.13.0)**

GDPR Article 17 erasure is implemented via crypto-shred, not file
deletion: deleting a tenant's key from `tenant_key_vault` makes
every on-disk and on-S3-archive byte for that tenant unrecoverable.
This is the **only viable erasure path** when the audit log
([ADR-015](015-wal-and-s3-object-lock-as-audit-log.md)) sits in
S3 Object Lock COMPLIANCE — Object Lock refuses content deletion
within the retention window, so the data must be cryptographically
inaccessible rather than physically removed.

```
Soft delete:      status=deletion_pending, data retained for grace period
Crypto-shred:     key wiped from tenant_key_vault → all files unreadable
File cleanup:     -crypto-shred-delete-files=true also rm's the .db/-wal/-shm
                  files after the shred (cosmetic; the bytes are already
                  garbage without the key)
```

Worker that processes the deletion queue at the configured cadence
(`server/go/internal/gdpr/processor.go`):

```
-gdpr-worker-enabled=true     # default
-gdpr-worker-interval=1m
-crypto-shred-delete-files=false
```

### Encryption in transit — **Shipped (v1.13.0)**

TLS support is implemented in `server/go/cmd/entdb-server/tls.go`
with mTLS in `server/go/internal/auth/mtls.go`.

Operator flags:

```
-tls-cert <path>              # server certificate
-tls-key <path>               # server private key
-tls-ca <path>                # client CA for mTLS verification
-tls-min-version {1.2|1.3}    # default 1.3
-require-tls=true             # refuse to start without TLS configured
-require-client-cert=true     # full mTLS — refuse clients without valid certs
```

**Production posture:** `-require-tls=true` + `-require-client-cert=true`
when service-to-service authentication is via certificates. The
cert/key files are reloaded on SIGHUP so rotation needs no restart.

The mTLS client cert subject DN / SAN is extracted into the request
context and flows through the trusted-actor interceptor (see
`server/go/internal/auth/`) — a future enhancement can map client
cert identity onto an EntDB actor (`service:<cn>`), bridging
PKI identity to EntDB capability checks.

Default (no `-tls-*` flags) is plaintext gRPC, **intended for local
dev only**. Production deployments MUST set `-require-tls=true`.

```
gRPC:   TLS 1.3 default, 1.2 minimum, mTLS available
Kafka:  TLS + SASL configured at the broker connection (operator-owned)
S3:     HTTPS only (Go SDK default)
```

### Audit logging — **Shipped (see ADR-015)**

Audit posture is owned by [ADR-015](015-wal-and-s3-object-lock-as-audit-log.md):
WAL + S3 Object Lock is the single audit log. Every operation
flows through the WAL ([ADR-016](016-handlers-append-applier-writes.md)),
the WAL is archived to S3 with Object Lock COMPLIANCE mode for
tamper evidence. This ADR does not duplicate that decision.

### Authentication — **Shipped**

Three credential carriers, implemented at
`server/go/internal/auth/`:

```
API keys  (apikey.go)
  - Stored hashed (bcrypt/argon2; never plaintext)
  - Per-tenant scoped, rotatable, revocable
  - Identity becomes the key's name field

OAuth / OIDC bearer (oauth.go)
  - JWKS-backed JWT validation (RS256/ES256)
  - Identity = sub claim, falling back to email

Session tokens (session.go)
  - SessionManager + per-user expiry

mTLS client cert (mtls.go, v1.13.0)
  - Subject DN / SAN extractable from the request context
  - Mapped to actor in a future enhancement
```

Order is fixed in `server/go/internal/auth/interceptor.go`: bearer →
api-key → session. First match wins; subsequent are not tried.

`Health` and `grpc.health.v1.Health/Check` bypass auth (orchestrator probes).

### Data residency — **Shipped (region pin), Planned (multi-region routing)**

Tenants carry a `region` column in `tenant_registry` (`global.db`),
defaulting to `us-east-1`. Set at `CreateTenant` time. The serving
region is configured on the server via `-region`. A request for a
tenant pinned to a different region returns an
`UNAVAILABLE` with the `entdb-redirect-node` trailer (handled by
the SDK; see `server/go/internal/tenant/check.go`).

Multi-region deployment (running multiple EntDB clusters with
region-specific tenants) is a deployment topology, not a single-
binary concern. Cross-region replication of the WAL is out of
scope.

### Secure deletion — **Shipped**

```
Soft delete:      status flag on the tenant_registry row; reads keep working
Hard delete:      SQLite file removed from disk (-crypto-shred-delete-files)
Crypto-shred:     key wiped from tenant_key_vault → unrecoverable on disk
                  and on the S3 archive (per ADR-015)
```

GDPR Article 17 uses crypto-shred for tenant data with the grace
period configured via `Admin.DeleteUser`'s `graceDays` argument.

### Input validation — **Shipped**

```
Schema validation  proto + entdb-schema check at CI; SDK rejects invalid
                   payloads before sending
SQL injection      parameterized queries only (`database/sql.QueryContext`
                   throughout server/go/internal/store/)
Payload size       configurable max per node (default 1 MB)
Field validation   type checking, enum validation, required fields
FTS5 injection     sanitized query input (existing fix carried over from Python)
Path traversal     tenant_id sanitized for filesystem paths in store/pool.go
```

### Network security — **Partial; deployment-owned**

```
TLS / mTLS              shipped (see "Encryption in transit")
Rate limiting           planned (no Go implementation yet; Python had a
                        QuotaInterceptor, port pending)
IP allowlisting         deployment-owned (cloud LB / WAF / SG rules)
DDoS protection         deployment-owned (Cloudflare, AWS Shield, GCP Cloud Armor)
Private networking      deployment-owned (VPC peering, private endpoints)
```

### Vulnerability management — **Process; not a code feature**

```
Dependencies         Dependabot enabled on the repo
SAST                 CodeQL via GitHub default
Container scanning   Trivy / Grype recommended on the published images
Secret scanning      GitHub default + manual sweeps
Penetration testing  recommended annually for production deployments
CVE monitoring       Dependabot + go.sum / go.mod alerts
```

### Monitoring and alerting — **Partial**

```
Prometheus metrics
  collection         ✅ shipped — server/go/internal/metrics/ records:
                        - grpc_requests_total{method,status}
                        - grpc_request_duration_seconds{method}
                        - wal_append_duration_seconds, wal_consumer_lag
                        - archive_lag_events, archive_writes_total,
                          archive_errors_total (when -archive-enabled)
  exposure           ⏳ NOT shipped — the server doesn't expose
                        a /metrics HTTP endpoint. Internal recording is in
                        place; surfacing it via promhttp.Handler() in
                        cmd/entdb-server/main.go is open work.

Tracing (OpenTelemetry)
  SDK transitive dep ✅ go.opentelemetry.io/otel pulled via dependencies
  actual spans       ⏳ NOT instrumented yet. Issue #517 tracks adding
                        OpenTelemetry hooks to the Go SDK; server-side
                        instrumentation follows.

Health checks
  gRPC                ✅ /grpc.health.v1.Health/Check + entdb.v1.EntDBService/Health
                        both bypass auth (orchestrator-friendly).
  HTTP /health        ✗ The Go server is gRPC-only. No HTTP listener.
                        Use grpc-health-probe or the standard gRPC health protocol.

Alerting (operator-owned)
  Applier lag, error rate, archive lag, encryption-required failures,
  authentication failure spikes — all observable via metrics once
  the /metrics endpoint ships.
```

### Backup integrity — **Partial**

```
WAL archive to S3                ✅ shipped (-archive-enabled, see ADR-015 / EPIC #511)
                                 Object Lock COMPLIANCE; tamper-evident.
Per-tenant SQLite snapshots      ⏳ NOT shipped. WAL replay is the default DR
                                 mechanism. Snapshots are an optimization (skip the
                                 "replay from beginning" cost on a fresh node).
Restore testing                  Operator-owned process; no automated test today.
Backup encryption                Automatic — SQLite files are SQLCipher-encrypted;
                                 deleting the tenant key (crypto-shred) makes
                                 backups irrecoverable too.
```

### Session and token management — **Shipped**

```
API keys              rotatable (multiple active keys), scopeable,
                      revocable. Hash-stored.
OAuth/OIDC            JWT validation; expiry enforced via token claims.
Session tokens        per-session TTL; revocable via session deletion.
Token revocation      immediate at the gateway via revocation checks
                      in `auth/session.go`.
```

### Compliance mapping

The controls above map to common compliance frameworks as follows.
This table is informational; specific certification audits will
require their own evidence packages.

| Control | SOC 2 | ISO 27001 | GDPR | HIPAA | CCPA |
|---|---|---|---|---|---|
| Encryption at rest | CC6.1 | A.10.1 | Art.32 | §164.312(a) | §1798.150 |
| Encryption in transit | CC6.1 | A.13.1 | Art.32 | §164.312(e) | — |
| Audit logging (ADR-015) | CC7.2 | A.12.4 | Art.30 | §164.312(b) | — |
| Access control (ADR-003) | CC6.1-6.3 | A.9 | Art.25 | §164.312(a) | — |
| Data classification (proto) | CC6.5 | A.8.2 | Art.30 | §164.312(a) | §1798.100 |
| Data residency | CC6.6 | A.11 | Art.44-49 | — | — |
| Encryption key mgmt | CC6.1 | A.10.1 | Art.32 | §164.312(a) | — |
| Secure deletion (crypto-shred) | CC6.5 | A.8.3 | Art.17 | §164.310(d) | §1798.105 |
| Backup integrity (WAL archive) | CC7.5 | A.12.3 | Art.32 | §164.308(a) | — |
| Vulnerability mgmt | CC7.1 | A.12.6 | Art.32 | §164.308(a) | — |
| Monitoring | CC7.2 | A.12.4 | Art.33 | §164.308(a) | — |
| Incident response | CC7.3-7.5 | A.16 | Art.33-34 | §164.308(a) | §1798.150 |
| Input validation | CC6.1 | A.14.2 | Art.32 | §164.312(c) | — |

### What EntDB provides vs what the deployment provides

```
EntDB provides (built into the database):
  ✅ Encryption at rest (SQLCipher v4; AES-256 + HMAC-SHA512)
  ✅ Encryption in transit (TLS 1.3 + mTLS, cert reload on SIGHUP)
  ✅ Audit log (WAL + S3 Object Lock; ADR-015)
  ✅ Access control (typed-capability ACL; ADR-003)
  ✅ Data classification (proto data_policy)
  ✅ GDPR operations (DeleteUser, ExportUserData, FreezeUser,
                      TransferUserContent, CancelUserDeletion)
  ✅ Crypto-shred secure deletion
  ✅ Input validation (proto + parameterized queries)
  ✅ Session/token management
  ✅ Legal hold (per-tenant flag in global.db; archive escalates to
                  Object Lock LegalHold=ON per ADR-015)
  ⏳ Rate limiting (Python had QuotaInterceptor; Go port pending)
  ⏳ Prometheus metrics endpoint (recording works; HTTP exposure pending)
  ⏳ OpenTelemetry tracing (#517 tracks SDK + server instrumentation)
  ⏳ Per-tenant SQLite snapshots (WAL replay is the durability mechanism)

The deployment provides (infrastructure):
  🔧 Network security (VPC, firewalls, security groups)
  🔧 Physical security (data center)
  🔧 KMS for master key (AWS KMS or HashiCorp Vault shipped; GCP / Azure providers stubbed, not implemented)
  🔧 Certificate provisioning and rotation cadence
  🔧 DDoS protection (Cloudflare, AWS Shield, Cloud Armor)
  🔧 CI/CD pipeline security
  🔧 Employee access controls / SSO
  🔧 Incident response procedures
  🔧 Business continuity planning
  🔧 Third-party audit engagement
  🔧 IP allowlisting at the LB / WAF
```

## Consequences

- SQLCipher adds ~5–10% overhead on SQLite operations. Acceptable
  for the privacy guarantees it enables; profile your workload if
  it's an issue.
- Per-tenant encryption keys require KMS infrastructure. Single-
  binary deployments can use `-kms-provider=file` for dev / lab;
  production MUST use a real KMS.
- `-require-tls=true` means no plaintext dev mode. Local dev uses
  self-signed certs (`openssl req` + `-tls-cert/-tls-key`).
- Data residency requires running multiple EntDB clusters, one per
  region, each owning the tenants pinned to that region. Cross-
  region replication is a future concern.
- Compliance documentation is an ongoing effort beyond technical
  controls — audits require evidence packages, control narratives,
  process attestations, and risk assessments that aren't in scope
  for this ADR.

## References

- [ADR-003](003-acl-model.md) — typed-capability ACL (the access
  control control).
- [ADR-014](014-physical-storage-layout.md) — physical file layout
  is what gets encrypted.
- [ADR-015](015-wal-and-s3-object-lock-as-audit-log.md) — audit log
  posture (this ADR's "Audit logging" line refers there).
- [ADR-016](016-handlers-append-applier-writes.md) — every state
  change flows through WAL; required for ADR-015 to hold.
- EPIC [#511](https://github.com/elloloop/tenant-shard-db/issues/511)
  — S3 Object Lock archive completion (mostly shipped via the
  `eda6ba9` / `a68d8e1` work; the issue tracks remaining tuning).
- Issue [#517](https://github.com/elloloop/tenant-shard-db/issues/517)
  — OpenTelemetry / interceptor hooks (Go SDK + server tracing).
- v1.13.0 release: SQLCipher + KMS + crypto-shred + TLS/mTLS shipped
  in commit `a68d8e1`; PRs #522 (TLS flags), #523 (key vault),
  bundled in #524-equivalent.
- Source: `server/go/internal/crypto/`,
  `server/go/internal/gdpr/`, `server/go/internal/auth/`,
  `server/go/cmd/entdb-server/{tls.go,main.go}`.
