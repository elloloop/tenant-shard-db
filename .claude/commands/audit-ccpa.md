# CCPA Compliance Audit

You are a CCPA compliance analyst auditing EntDB for California Consumer Privacy Act compliance. Create a detailed report.

## Audit Team
- **Privacy Analyst**: consumer rights, data categories
- **Technical Analyst**: implementation of rights, data flows
- **Legal Analyst**: disclosures, opt-out mechanisms, service provider obligations

## Audit Checklist

### §1798.100 — Right to Know (Categories)
- [ ] Can the system identify what CATEGORIES of personal information are collected per tenant?
- [ ] Is data classification (data_policy) mapped to CCPA categories?
  - Identifiers (name, email, user_id) → PII fields
  - Commercial information (orders, subscriptions) → BUSINESS/FINANCIAL types
  - Internet activity (logs, usage) → AUDIT types
  - Professional information → BUSINESS types

### §1798.105 — Right to Delete
- [ ] Does delete_user exist?
- [ ] Does it cover all personal information across all tenants?
- [ ] Are there exceptions documented (legal retention)?
- [ ] Is the 45-day response window supported?
- [ ] Can deletion be verified (crypto-shred)?

### §1798.110 — Right to Know (Specific Pieces)
- [ ] Does export_user_data return specific pieces of data?
- [ ] Is the export format usable (JSON, structured)?
- [ ] Does it include all tenants the user belongs to?
- [ ] Does it respect subject_field (data about the user)?

### §1798.115 — Right to Know (Sale/Sharing)
- [ ] Is there a mechanism to track if data is "sold" or "shared" (as defined by CCPA)?
- [ ] Note: EntDB as a database processor doesn't sell data, but must support the app's disclosure

### §1798.120 — Right to Opt-Out of Sale
- [ ] Is there an opt-out flag per user?
- [ ] If user opts out, is processing restricted?
- [ ] Can the opt-out be propagated to all tenants?

### §1798.125 — Non-Discrimination
- [ ] Does the system ensure opt-out users get the same service level?
- [ ] No different rate limiting, access restrictions for privacy-exercising users?

### §1798.140 — Definitions
- [ ] "Personal information" properly scoped in data_policy classifications?
- [ ] "Service provider" obligations documented (EntDB as processor)?
- [ ] "Business purpose" documented for each data_policy type?

### §1798.150 — Private Right of Action (Data Breaches)
- [ ] Is data encrypted at rest (defense against breach claims)?
- [ ] Is there breach detection (audit log anomalies)?
- [ ] Is there breach notification procedure?

### Service Provider Assessment
- [ ] EntDB's role as "service provider" documented?
- [ ] Data processing agreement template includes CCPA clauses?
- [ ] Subprocessor list maintained?
- [ ] Annual assessment of subprocessors?

## Output Format
Create the report as `audit-reports/ccpa-audit-{date}.md`
Include: rights implementation status, gap analysis, remediation recommendations.
