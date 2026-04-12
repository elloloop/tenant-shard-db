# GDPR Compliance Audit

You are a GDPR Data Protection Officer auditing EntDB. Run a thorough audit from the perspective of GDPR Articles 5-49. Create a detailed report.

## Audit Team
- **Data Protection Lead**: overall GDPR compliance, lawful basis
- **Technical Privacy Engineer**: data flows, encryption, anonymization
- **Rights Fulfillment Analyst**: data subject rights (access, erasure, portability)
- **Documentation Analyst**: records of processing, DPIAs, policies

## Audit Checklist

### Article 5: Principles
- [ ] Lawfulness: Is there a documented lawful basis for processing?
- [ ] Purpose limitation: Is data used only for stated purposes?
- [ ] Data minimization: Does the schema collect only necessary fields?
- [ ] Accuracy: Is there a rectification mechanism (update_user)?
- [ ] Storage limitation: Are retention periods defined per node type?
- [ ] Integrity/confidentiality: Encryption at rest and in transit?
- [ ] Accountability: Is processing documented?

### Article 6: Lawful Basis
- [ ] Check: Does every node type with data_policy=PERSONAL or BUSINESS have a lawful basis documented?
- [ ] Check: Is consent tracked where needed?

### Article 13-14: Information to Data Subject
- [ ] Check: Is there a privacy policy template?
- [ ] Check: Does the export include processing purposes?

### Article 15: Right of Access
- [ ] Check: Does export_user_data exist?
- [ ] Check: Does it scan ALL tenants the user belongs to?
- [ ] Check: Does it respect subject_field (data ABOUT the user, not just BY)?
- [ ] Check: Is the output structured and machine-readable?
- [ ] Check: Does it include metadata (when collected, why, who has access)?

### Article 16: Right to Rectification
- [ ] Check: Does update_user exist?
- [ ] Check: Does it propagate changes across tenants?

### Article 17: Right to Erasure
- [ ] Check: Does delete_user exist?
- [ ] Check: Does it handle each data_policy correctly?
  - PERSONAL → DELETE
  - BUSINESS → ANONYMIZE
  - FINANCIAL → RETAIN with anonymized PII
  - AUDIT → RETAIN with anonymized PII
- [ ] Check: Is there a 30-day grace period?
- [ ] Check: Is crypto-shredding implemented for personal tenants?
- [ ] Check: Are backups addressed? (crypto-shred makes backup data unrecoverable)
- [ ] Check: Is the shared_index cleaned up?
- [ ] Check: Are group memberships removed?

### Article 18: Right to Restrict Processing
- [ ] Check: Does freeze_user exist?
- [ ] Check: Does it block writes but allow reads?

### Article 20: Right to Data Portability
- [ ] Check: Is export in a structured, commonly used format (JSON)?
- [ ] Check: Can it be imported into another system?

### Article 25: Data Protection by Design
- [ ] Check: Default data_policy is PERSONAL (strictest)?
- [ ] Check: Unmarked string fields produce warnings?
- [ ] Check: entdb lint enforces classification?
- [ ] Check: entdb check blocks unclassified deployments?

### Article 28: Processor
- [ ] Check: Is EntDB's role as processor documented?
- [ ] Check: Is there a Data Processing Agreement template?

### Article 30: Records of Processing
- [ ] Check: Does the audit_log record all processing activities?
- [ ] Check: Can processing records be exported?
- [ ] Check: Are records per-tenant (matching controller scope)?

### Article 32: Security of Processing
- [ ] Check: Encryption at rest?
- [ ] Check: Encryption in transit?
- [ ] Check: Access control (ACL)?
- [ ] Check: Regular testing (penetration test, vulnerability scans)?

### Article 33-34: Breach Notification
- [ ] Check: Is there a breach detection mechanism (audit log anomalies)?
- [ ] Check: Is there a breach notification procedure (72 hours)?
- [ ] Check: Can affected data subjects be identified?

### Article 35: DPIA
- [ ] Check: Is there a Data Protection Impact Assessment template?
- [ ] Check: Is it pre-filled with EntDB's processing characteristics?

### Article 44-49: International Transfers
- [ ] Check: Is data residency configurable per tenant?
- [ ] Check: Can data be restricted to specific regions?
- [ ] Check: Are cross-region transfers documented?

### PII Field Audit
- [ ] Scan ALL proto schema files for string fields without (entdb.pii) marking
- [ ] Scan ALL node types for missing data_policy
- [ ] Scan ALL FINANCIAL/AUDIT types for missing legal_basis
- [ ] Scan ALL edge types referencing User for missing on_subject_exit
- [ ] Report any unclassified data

### Anonymization Quality
- [ ] Check: Is anonymization irreversible (not just pseudonymization)?
- [ ] Check: Are indirect identifiers addressed (email hashes, writing patterns)?
- [ ] Check: Is the anonymous ID deterministic within one deletion (conversation context)?
- [ ] Check: Can the anonymous ID be reversed? (it must NOT be reversible)

## Output Format
Create the report as `audit-reports/gdpr-audit-{date}.md`
Include: executive summary, article-by-article findings, PII field inventory, remediation priorities.
