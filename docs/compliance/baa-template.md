# HIPAA Business Associate Agreement (BAA) — Template

This Business Associate Agreement ("Agreement") is entered into between
**EntDB, Inc.** ("Business Associate") and **[CUSTOMER NAME]** ("Covered
Entity") and is effective as of **[EFFECTIVE DATE]**.

This template reflects Business Associate obligations under 45 CFR Part
160 and Part 164, Subparts A and E ("the Privacy Rule") and Subparts A
and C ("the Security Rule"), as amended by the HITECH Act.

This document is a template. It is not legal advice. Have counsel review
before execution.

---

## 1. Definitions

Terms used in this Agreement have the meanings set forth in 45 CFR
§§ 160.103 and 164.501, including the following:

- **Protected Health Information (PHI)** means individually identifiable
  health information, as defined at 45 CFR § 160.103, that Business
  Associate creates, receives, maintains, or transmits on behalf of
  Covered Entity.
- **Electronic PHI (ePHI)** means PHI transmitted by or maintained in
  electronic media.
- **Breach** has the meaning set forth at 45 CFR § 164.402.
- **Secretary** means the Secretary of the U.S. Department of Health
  and Human Services.
- **Unsecured PHI** means PHI not rendered unusable, unreadable, or
  indecipherable per the HHS guidance under HITECH § 13402(h)(2).

---

## 2. Permitted Uses and Disclosures

### 2.1 Services

Business Associate provides EntDB: a multi-tenant, append-only database
service that stores, processes, and transmits Covered Entity's ePHI on
its behalf for the purpose of **[DESCRIBE SERVICE — e.g., clinical event
storage, audit logging, analytics]**.

### 2.2 General

Business Associate may use or disclose PHI only:

(a) as permitted or required by this Agreement;
(b) as required by law; or
(c) as otherwise permitted by the Privacy Rule for the proper
    management and administration of Business Associate, or to carry
    out the legal responsibilities of Business Associate.

### 2.3 Specific Uses

Business Associate may use PHI to:

- Provide the Services as described in the underlying Master Services
  Agreement.
- Perform data aggregation services as defined at 45 CFR § 164.501.
- Create de-identified information in accordance with 45 CFR § 164.514.
- Report violations of law to appropriate federal and state authorities
  per 45 CFR § 164.502(j)(1).

### 2.4 Prohibited Uses

Business Associate shall not:

- Sell PHI, except as permitted by 45 CFR § 164.502(a)(5)(ii).
- Use PHI for marketing except as permitted by 45 CFR § 164.508(a)(3).
- Use PHI outside the United States without Covered Entity's prior
  written consent.

---

## 3. Safeguards

### 3.1 Administrative, Physical, Technical

Business Associate shall implement administrative, physical, and
technical safeguards as required by 45 CFR §§ 164.308, 164.310, and
164.312. EntDB's current controls include:

- **Access control**: principal-based ACL enforced on every request
  (EntDB ACL v2).
- **Audit controls**: immutable audit log capturing principal,
  timestamp, tenant, resource, operation, and outcome.
- **Integrity controls**: append-only canonical store and plan-commit
  idempotency guard.
- **Transmission security**: TLS 1.2+ for all external traffic, mTLS
  between internal services.
- **Encryption at rest**: AES-256-GCM via AWS KMS / GCP KMS.
- **Workforce clearance**: background checks, role-based access, annual
  training.
- **Contingency plan**: multi-AZ deployment, Kafka + S3 recovery tiers,
  annual DR test (see `docs/compliance/business-continuity.md`).

### 3.2 Minimum Necessary

Business Associate shall make reasonable efforts to use, disclose, and
request only the minimum PHI necessary to accomplish the intended
purpose, consistent with 45 CFR § 164.502(b).

---

## 4. Subcontractors

Business Associate shall, in accordance with 45 CFR § 164.502(e)(1)(ii),
ensure that any subcontractor that creates, receives, maintains, or
transmits PHI on its behalf agrees in writing to the same restrictions
and conditions that apply to Business Associate.

### Schedule A — Approved Subcontractors

| Subcontractor | Role | Location | BAA on file |
|---------------|------|----------|-------------|
| **[CLOUD PROVIDER]** (AWS / GCP) | Infrastructure, KMS, S3 | US | Yes |
| **[KAFKA OPERATOR]** (Confluent / MSK) | Event log broker | US | Yes |
| **[MONITORING PROVIDER]** | Metrics, logging, alerting | US | Yes |
| **[SECRETS MANAGER]** | Secret storage | US | Yes |

Covered Entity consents to the subcontractors listed above. Business
Associate will provide 30 days' notice before adding or replacing a
subcontractor that processes PHI.

---

## 5. Breach Notification

### 5.1 Reporting Obligation

Business Associate shall report to Covered Entity any:

- Security Incident affecting ePHI as required by 45 CFR § 164.314(a).
- Breach of Unsecured PHI as required by 45 CFR § 164.410.
- Use or disclosure of PHI not permitted by this Agreement.

### 5.2 Timing

Business Associate shall provide initial notification to Covered Entity
**without unreasonable delay** and in no case later than **five (5)
business days** after Discovery. Full written report shall be delivered
within **thirty (30) calendar days** of Discovery.

HIPAA permits Covered Entities up to 60 days to notify affected
individuals. Business Associate's 5-business-day / 30-day timeline
ensures Covered Entity retains operational headroom within the 60-day
HIPAA breach notification window.

### 5.3 Contents of Notification

To the extent available, Business Associate shall provide:

(a) A description of the incident, including the date of the incident
    and the date of Discovery;
(b) The types of PHI involved;
(c) The identities of individuals whose PHI was or is reasonably
    believed to have been affected;
(d) The steps Business Associate has taken or will take to investigate,
    mitigate, and prevent recurrence; and
(e) Contact information for further inquiries.

### 5.4 Cooperation

Business Associate shall cooperate in good faith with Covered Entity's
investigation of any Breach, including providing logs, forensic data,
and access to personnel reasonably necessary for Covered Entity to
comply with its notification obligations.

---

## 6. Individual Rights

Business Associate shall, within **[15] business days** of Covered
Entity's request:

- **Access (§ 164.524)**: Provide access to PHI maintained in a
  Designated Record Set.
- **Amendment (§ 164.526)**: Incorporate amendments to PHI.
- **Accounting of Disclosures (§ 164.528)**: Document and provide a
  record of disclosures of PHI.

EntDB's append-only audit log supports (§ 164.528) with fields:
principal, timestamp, tenant, resource, operation, outcome, purpose.

---

## 7. Access by Secretary

Business Associate shall make its internal practices, books, and records
relating to the use and disclosure of PHI received from, created, or
received by Business Associate on behalf of Covered Entity available to
the Secretary for purposes of determining Covered Entity's compliance
with the Privacy Rule.

---

## 8. Term and Termination

### 8.1 Term

This Agreement shall be effective as of **[EFFECTIVE DATE]** and shall
terminate upon the termination of the underlying Services Agreement or
upon termination under § 8.2.

### 8.2 Termination for Cause

Either party may terminate this Agreement if it determines that the
other party has breached a material term and the breach is not cured
within **thirty (30) days** of written notice.

### 8.3 Effect of Termination

Upon termination, Business Associate shall, if feasible, return or
destroy all PHI received from Covered Entity, or created or received by
Business Associate on behalf of Covered Entity. If return or destruction
is infeasible, Business Associate shall extend the protections of this
Agreement to such PHI and limit further use and disclosure.

EntDB destruction procedure: archive-tier delete API overwrites S3
objects with a tombstone and removes KMS key grants, rendering data
cryptographically unrecoverable.

---

## 9. Miscellaneous

- **Amendment.** Parties agree to amend this Agreement to comply with
  any changes in HIPAA or HITECH.
- **Interpretation.** Ambiguities shall be resolved in favor of
  compliance with HIPAA.
- **No third-party beneficiaries.** Nothing in this Agreement confers
  rights on any third party.
- **Survival.** Sections 5, 7, and 8.3 survive termination.

---

## Signatures

**Covered Entity:** [CUSTOMER NAME]

Signed: ______________________________  Date: _______________

Name: **[SIGNATORY NAME]**

Title: **[SIGNATORY TITLE]**

**Business Associate:** EntDB, Inc.

Signed: ______________________________  Date: _______________

Name: **[ENTDB SIGNATORY]**

Title: **[ENTDB TITLE]**
