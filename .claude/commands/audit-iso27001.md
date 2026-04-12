# ISO 27001 Compliance Audit

You are an ISO 27001 lead auditor examining EntDB's Information Security Management System (ISMS). Audit against Annex A controls. Create a detailed report.

## Audit Team
- **Lead Auditor**: ISMS effectiveness, management commitment
- **Technical Auditor**: technical controls (A.8-A.14)
- **Process Auditor**: organizational controls (A.5-A.7)
- **Operational Auditor**: operational controls (A.12-A.18)

## Audit Checklist

### A.5 Information Security Policies
- [ ] Information security policy documented?
- [ ] Policy reviewed and approved?
- [ ] Policy communicated (ARCHITECTURE.md, ADRs)?

### A.6 Organization of Information Security
- [ ] Security roles defined?
- [ ] Segregation of duties in code (who can merge to main)?
- [ ] Contact with authorities (breach notification procedure)?

### A.7 Human Resource Security
- [ ] Security responsibilities in CONTRIBUTING.md?
- [ ] Security awareness (documentation for developers)?
- [ ] Termination procedures (delete_user, revoke_user_access)?

### A.8 Asset Management
- [ ] Asset inventory (what data is stored, where, classified how)?
- [ ] Data classification scheme (data_policy: PERSONAL/BUSINESS/FINANCIAL/AUDIT)?
- [ ] Media handling (SQLite files, backups, encryption)?
- [ ] Media disposal (crypto-shred, secure deletion)?

### A.9 Access Control
- [ ] Access control policy (ADR-003)?
- [ ] User registration (user_registry)?
- [ ] Privilege management (tenant roles: owner/admin/member)?
- [ ] User access review (can list all access per user)?
- [ ] Authentication (API keys, OAuth)?
- [ ] Password/key policy (key rotation)?
- [ ] Access to source code (branch protection, PR reviews)?

### A.10 Cryptography
- [ ] Encryption policy documented?
- [ ] Encryption at rest (SQLCipher)?
- [ ] Encryption in transit (TLS)?
- [ ] Key management (hierarchy, rotation, revocation)?
- [ ] Crypto-shredding capability?

### A.11 Physical Security
- [ ] Note: physical security is deployment responsibility
- [ ] Documented that cloud provider handles physical security?

### A.12 Operations Security
- [ ] Change management (CI/CD, PR reviews, schema checks)?
- [ ] Capacity management (scale targets documented)?
- [ ] Separation of environments (dev/staging/production)?
- [ ] Malware protection (dependency scanning, SAST)?
- [ ] Backup (snapshots, WAL archive)?
- [ ] Logging (audit_log, Prometheus metrics)?
- [ ] Monitoring (alerts on failures, lag, errors)?
- [ ] Clock synchronization (timestamps in audit log)?

### A.13 Communications Security
- [ ] Network controls (TLS, rate limiting)?
- [ ] Secure transfer (TLS for gRPC, HTTPS for S3)?
- [ ] Confidentiality agreements (DPA template)?

### A.14 System Development
- [ ] Security in development (input validation, parameterized SQL)?
- [ ] Secure coding guidelines?
- [ ] System testing (1269+ tests, CI)?
- [ ] Test data protection (no production data in tests)?
- [ ] Source code protection (GitHub, branch protection)?

### A.15 Supplier Relationships
- [ ] Supplier security policy (Kafka, S3, KMS providers)?
- [ ] Supplier monitoring?
- [ ] Subprocessor list for GDPR?

### A.16 Incident Management
- [ ] Incident response procedure?
- [ ] Incident reporting mechanisms?
- [ ] Incident classification (severity levels)?
- [ ] Lessons learned process?

### A.17 Business Continuity
- [ ] Business continuity plan?
- [ ] Recovery procedures (3-tier recovery)?
- [ ] RTO/RPO defined?
- [ ] Testing (restore tests)?

### A.18 Compliance
- [ ] Legal requirements identified (GDPR, CCPA, HIPAA)?
- [ ] Privacy compliance (ADR-004)?
- [ ] Audit trail (tamper-evident logs)?

## Maturity Assessment

Rate each control area 1-5:
```
1 = Initial     — ad hoc, no process
2 = Managed     — basic process exists
3 = Defined     — standardized, documented
4 = Measured    — monitored, metrics
5 = Optimized   — continuous improvement
```

## Output Format
Create the report as `audit-reports/iso27001-audit-{date}.md`
Include: executive summary, control-by-control findings, maturity scores, certification readiness assessment.
