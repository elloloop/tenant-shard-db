# HIPAA Compliance Audit

You are a HIPAA compliance officer auditing EntDB for healthcare readiness. Examine all Administrative, Physical, and Technical safeguards. Create a detailed report.

## Audit Team
- **HIPAA Security Officer**: overall HIPAA Security Rule compliance
- **HIPAA Privacy Officer**: Privacy Rule, minimum necessary, de-identification
- **Technical Safeguard Analyst**: encryption, access controls, audit controls
- **BAA Analyst**: business associate obligations, subcontractor chain

## Audit Checklist

### Administrative Safeguards (§164.308)

#### (a)(1) Security Management
- [ ] Risk analysis conducted?
- [ ] Risk management plan documented?
- [ ] Sanction policy for violations?

#### (a)(3) Workforce Security
- [ ] Authorization procedures for PHI access?
- [ ] Workforce clearance procedures?
- [ ] Termination procedures (delete_user, revoke_user_access)?

#### (a)(4) Information Access Management
- [ ] Access authorization policies?
- [ ] Access establishment/modification procedures (ACL, node_access)?
- [ ] Role-based access (tenant roles: owner, admin, member)?

#### (a)(5) Security Awareness Training
- [ ] Training documentation for developers using EntDB?
- [ ] PHI handling guidelines?

#### (a)(6) Security Incident Procedures
- [ ] Incident response plan?
- [ ] Incident identification mechanisms (audit log anomalies)?
- [ ] Incident reporting procedures?

#### (a)(7) Contingency Plan
- [ ] Data backup plan (snapshots, WAL archive)?
- [ ] Disaster recovery plan?
- [ ] Emergency mode operation?
- [ ] Testing procedures (restore tests)?

### Technical Safeguards (§164.312)

#### (a)(1) Access Control
- [ ] Unique user identification (user_registry, actor on every request)?
- [ ] Emergency access procedure?
- [ ] Automatic logoff (session timeout)?
- [ ] Encryption and decryption (SQLCipher)?

#### (b) Audit Controls
- [ ] Audit log captures ALL PHI access?
- [ ] PHI access logging includes: who, what fields, when, from where, why?
- [ ] Audit logs are tamper-evident (hash chain)?
- [ ] Audit log retention meets 6-year minimum?

#### (c) Integrity
- [ ] Data integrity mechanisms (idempotent apply, schema validation)?
- [ ] Mechanism to authenticate ePHI (hash verification)?

#### (d)(1) Person Authentication
- [ ] Authentication on every request (API key interceptor)?
- [ ] Multi-factor capability?

#### (e)(1) Transmission Security
- [ ] Encryption in transit (TLS)?
- [ ] Integrity controls (TLS provides this)?

### PHI-Specific Checks

#### Schema Audit
- [ ] Scan proto schemas: do node types handling PHI have data_policy=HEALTHCARE?
- [ ] Are PHI fields marked with (entdb.phi)=true?
- [ ] Is field-level encryption implemented for phi=true fields?
- [ ] Is retention_days >= 2190 (6 years) for HEALTHCARE types?
- [ ] Is legal_basis documented for all HEALTHCARE types?

#### Access Logging
- [ ] Is PHI access logged with enhanced detail?
- [ ] Does the log include: fields_accessed, access_reason, ip_address, session_id?
- [ ] Can logs answer: "who accessed patient X's data in the last 30 days?"

#### De-identification
- [ ] Is de-identification per Safe Harbor method? (18 identifiers)
- [ ] Check: names removed?
- [ ] Check: geographic data smaller than state removed?
- [ ] Check: dates (except year) removed?
- [ ] Check: phone/fax numbers removed?
- [ ] Check: email addresses removed?
- [ ] Check: SSN removed?
- [ ] Check: medical record numbers removed?
- [ ] Check: health plan numbers removed?
- [ ] Check: account numbers removed?
- [ ] Check: certificate/license numbers removed?
- [ ] Check: vehicle identifiers removed?
- [ ] Check: device identifiers removed?
- [ ] Check: URLs removed?
- [ ] Check: IP addresses removed?
- [ ] Check: biometric identifiers removed?
- [ ] Check: full-face photos removed?
- [ ] Check: any other unique identifying number removed?

#### Minimum Necessary
- [ ] Can queries be scoped to return only needed fields?
- [ ] Is field-level ACL implemented (or planned)?
- [ ] Are roles defined that limit PHI access by job function?

### Business Associate Agreement
- [ ] BAA template exists?
- [ ] BAA covers EntDB's obligations as processor?
- [ ] Subcontractor BAA chain documented (Kafka, S3, KMS providers)?
- [ ] Breach notification clause (60 days)?

## Output Format
Create the report as `audit-reports/hipaa-audit-{date}.md`
Rate each safeguard: IMPLEMENTED / PARTIAL / NOT IMPLEMENTED / NOT APPLICABLE
Include remediation timeline for each gap.
