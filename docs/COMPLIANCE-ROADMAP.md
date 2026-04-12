# Compliance Implementation Roadmap

## Phase 0: Foundations (Week 1-4)
*Everything else depends on these*

```
[ ] Encryption at rest — SQLCipher integration
    - Add sqlcipher dependency
    - Per-tenant key derivation from master key
    - Key storage in encrypted key_registry table
    - All existing tests pass with encryption enabled
    
[ ] Global store — user_registry, tenant_registry, tenant_members, shared_index
    - Create global SQLite (or Postgres for multi-node)
    - User CRUD operations
    - Tenant CRUD operations
    - Tenant membership management
    - Shared index for cross-tenant sharing

[ ] Tamper-evident audit log
    - Hash-chained append-only table in each tenant
    - Every write, ACL change, admin action logged
    - Chain verification function
    - Audit log export for compliance

[ ] Proto schema codegen with data classification
    - entdb_options.proto: data_policy, pii, phi, retention, legal_basis
    - entdb generate: proto → Python/Go
    - entdb lint: enforce classification on all types
    - entdb check: backward compatibility
```

## Phase 1: GDPR + CCPA (Week 5-8)
*Highest impact, covers privacy for EU + California*

```
[ ] Data policy enforcement at runtime
    - data_policy read from schema registry
    - pii fields tracked per node type
    - subject_field resolution for exports
    
[ ] delete_user operation
    - Export user data across all tenants
    - 30-day grace period with recovery
    - Per-type handling: DELETE / ANONYMIZE / RETAIN
    - PII field scrubbing (all pii=true fields)
    - Edge handling (on_subject_exit: FROM/TO/BOTH)
    - Global cleanup (user_registry, shared_index, groups)
    - Crypto-shred for personal tenant

[ ] export_user_data operation
    - Scan all tenants user belongs to
    - Respect export policy per node type
    - Respect subject_field (data ABOUT user, not just BY user)
    - Structured output (JSON + metadata)

[ ] freeze_user operation
    - Block all writes by user
    - Data remains readable
    - Reversible by admin

[ ] update_user operation (rectification)
    - Update user_registry
    - Propagate name/email changes

[ ] Consent tracking
    - Record what user consented to and when
    - Consent withdrawal triggers appropriate action

[ ] Privacy policy template
    - Template for apps built on EntDB
    - Pre-filled with database capabilities
```

## Phase 2: SOC 2 Type I (Week 9-16)
*Most requested by enterprise customers*

```
[ ] TLS enforcement
    - Refuse plaintext in production mode
    - TLS 1.3 minimum
    - Certificate validation

[ ] Key management
    - Master key in KMS (AWS KMS, GCP KMS, HashiCorp Vault)
    - Tenant key derivation and storage
    - Key rotation (new writes use new key, background re-encryption)
    - Key revocation (crypto-shred)

[ ] Enhanced authentication
    - OAuth 2.0 / OIDC token validation
    - JWT verification at interceptor level
    - API key scoping (per-tenant, per-permission)
    - API key rotation without downtime

[ ] Session management
    - Token expiry enforcement
    - Concurrent session limits
    - Immediate revocation list

[ ] Vulnerability scanning in CI
    - Dependabot (already exists)
    - CodeQL or Semgrep SAST
    - Trivy container scanning
    - Secret scanning (truffleHog)

[ ] Backup integrity
    - Checksums on every snapshot
    - Automated monthly restore test
    - Restore time verification (< 1 hour SLA)

[ ] Monitoring completeness
    - Alert on auth failure spikes
    - Alert on applier lag > 60s
    - Alert on disk usage > 80%
    - Alert on audit chain break
    - Alert on error rate > 1%

[ ] SOC 2 evidence collection
    - Automated evidence gathering script
    - Screenshots/exports of configurations
    - Access review documentation
    - Change management records (git history)

[ ] SOC 2 Type I audit engagement
    - Select auditor (Drata, Vanta, or traditional firm)
    - Readiness assessment
    - Gap remediation
    - Type I report
```

## Phase 3: ISO 27001 + SOC 2 Type II (Week 17-30)
*Type II requires 6+ months of evidence*

```
[ ] ISMS documentation
    - Information security policy
    - Risk assessment methodology
    - Risk treatment plan
    - Statement of applicability
    - Asset inventory

[ ] Access control policy
    - Role definitions documented
    - Access review procedures (quarterly)
    - Privileged access management
    - Segregation of duties

[ ] Incident response
    - Incident classification (P1-P4)
    - Response procedures per severity
    - Communication templates
    - Post-incident review process
    - Breach notification procedures (72 hours for GDPR)

[ ] Business continuity
    - Recovery time objectives (RTO) per tier
    - Recovery point objectives (RPO) per tier
    - Disaster recovery procedures
    - Annual DR test

[ ] Change management
    - All changes via PR (already enforced)
    - Schema changes via entdb check (already designed)
    - Deployment procedures documented
    - Rollback procedures documented

[ ] Supplier management
    - Kafka provider security assessment
    - S3 provider security assessment
    - KMS provider security assessment
    - Subprocessor list for GDPR

[ ] Legal hold implementation
    - Per-tenant legal_hold flag
    - Block all deletes when enabled
    - Audit log of hold activation/deactivation
    - Hold applies to backups too

[ ] Data residency enforcement
    - Region pinning per tenant
    - Cross-region routing
    - Validation: data never leaves configured region
    - Audit: region compliance checks

[ ] SOC 2 Type II observation period (6 months)
    - Continuous evidence collection
    - Monthly access reviews
    - Quarterly risk assessments
    - Annual penetration test

[ ] ISO 27001 certification audit
```

## Phase 4: HIPAA (Week 20-30, parallel with Phase 3)
*Required for healthcare customers*

```
[ ] PHI field-level encryption
    - Fields marked phi=true encrypted individually
    - Separate from file-level encryption
    - Field-level decryption only when authorized

[ ] HEALTHCARE data policy
    - 6-year retention minimum
    - Enhanced access logging (fields accessed, reason, IP)
    - De-identification per Safe Harbor (18 identifiers)

[ ] BAA template
    - Business Associate Agreement for EntDB service
    - Subcontractor BAA chain (Kafka, S3, KMS providers)

[ ] Minimum necessary access
    - Field-level ACL (future: access_level per field)
    - Query returns only fields the actor needs

[ ] HIPAA training documentation
    - For EntDB operations team
    - For application developers using EntDB
```

## Phase 5: Additional certifications (Week 30+)

```
[ ] CSA STAR self-assessment
    - Consensus Assessments Initiative Questionnaire (CAIQ)
    - Based on SOC 2 + ISO 27001 evidence

[ ] VPAT / Section 508
    - API accessibility documentation
    - Admin console WCAG 2.1 AA audit
    - Documentation site accessibility audit

[ ] FedRAMP (if US government customers)
    - FedRAMP Ready assessment
    - 3PAO engagement
    - Authority to Operate (ATO)

[ ] FERPA (if education customers)
    - Student data protection
    - Parental consent mechanisms
    - School official exception handling

[ ] PCI DSS (if payment data)
    - Cardholder data environment scoping
    - Network segmentation
    - Quarterly vulnerability scans (ASV)
```
