# ISO/IEC 27001 Information Security Management System (ISMS)

This document is the EntDB ISMS under ISO/IEC 27001:2022. It contains the
information security policy, risk assessment methodology, risk treatment
plan, Statement of Applicability (SoA) mapped to Annex A controls, and an
asset inventory template.

This is a starting template. Before certification, it must be reviewed by
the Information Security Officer (ISO) and signed by management.

---

## 1. Information Security Policy

### 1.1 Purpose

EntDB is committed to protecting the confidentiality, integrity, and
availability of customer data and the operational systems that store,
process, and transmit it. This policy establishes the principles, roles,
and responsibilities for information security across the organization.

### 1.2 Scope

This policy applies to:

- All employees, contractors, and third parties with access to EntDB
  systems.
- All EntDB-operated environments: development, staging, production.
- All customer data processed by EntDB, regardless of classification.

### 1.3 Policy Statements

1. **Least privilege.** All access to production data and systems is
   granted on a need-to-know basis and reviewed quarterly.
2. **Encryption.** Customer data is encrypted in transit (TLS 1.2+) and at
   rest (AES-256 via KMS).
3. **Audit logging.** All administrative and data-plane operations are
   logged with principal, timestamp, and outcome.
4. **Change management.** Production changes require peer review, automated
   testing, and rollback plans.
5. **Incident response.** Security incidents are triaged within 1 hour of
   detection and escalated per `docs/compliance/incident-response.md`.
6. **Continuity.** Recovery procedures are tested annually and meet defined
   RTO/RPO targets.
7. **Supplier security.** Subprocessors are reviewed annually and listed
   in the customer-facing subprocessor page.
8. **Awareness.** All staff complete annual security training.

### 1.4 Roles

| Role | Responsibility |
|------|----------------|
| Information Security Officer (ISO) | ISMS owner, risk register, policy reviews |
| CTO | Final approval of security controls |
| Engineering Lead | Secure code review, vulnerability remediation |
| SRE Lead | Patch management, monitoring, incident response |
| Legal / DPO | Privacy, data subject requests, regulator liaison |

### 1.5 Review

This policy is reviewed annually or after any material change (new product
line, new subprocessor, significant incident).

---

## 2. Risk Assessment Methodology

EntDB follows a qualitative risk assessment aligned with ISO 27005.

### 2.1 Steps

1. **Asset identification.** Enumerate information assets (data, systems,
   people, facilities). See §5 Asset Inventory.
2. **Threat identification.** For each asset, list credible threats
   (insider misuse, external attacker, supply chain compromise, natural
   disaster, accidental loss).
3. **Vulnerability identification.** Map known weaknesses (unpatched
   dependency, missing audit log, weak ACL).
4. **Impact rating.** Score 1 (negligible) to 5 (catastrophic) for each
   of confidentiality, integrity, availability.
5. **Likelihood rating.** Score 1 (rare) to 5 (almost certain).
6. **Inherent risk.** `max(CIA impact) × likelihood`.
7. **Control mapping.** Identify applied controls from Annex A.
8. **Residual risk.** Re-score with controls applied.
9. **Treatment decision.** Accept, mitigate, transfer, or avoid.

### 2.2 Risk Matrix

| Likelihood \ Impact | 1 | 2 | 3 | 4 | 5 |
|---------------------|---|---|---|---|---|
| **5 Almost certain**| L | M | H | C | C |
| **4 Likely**        | L | M | H | H | C |
| **3 Possible**      | L | M | M | H | H |
| **2 Unlikely**      | L | L | M | M | H |
| **1 Rare**          | L | L | L | M | M |

L = Low, M = Medium, H = High, C = Critical. Critical and High require
immediate mitigation; Medium requires a treatment plan; Low may be
accepted with ISO approval.

### 2.3 Frequency

- Full assessment: annually.
- Delta assessment: on any significant change (new dependency, new tenant
  class, architecture change).

---

## 3. Risk Treatment Plan

Below is a starter register. Populate and review with the ISO before
certification.

| ID | Risk | Asset | Inherent | Control(s) | Residual | Decision | Owner |
|----|------|-------|----------|------------|----------|----------|-------|
| R1 | Unauthorized access to tenant data | Canonical store | Critical | A.5.15, A.5.16, ACL v2, mTLS | Medium | Mitigate | Eng Lead |
| R2 | Accidental deletion of customer records | SQLite | High | A.8.13, append-only design, S3 archive | Low | Mitigate | SRE |
| R3 | Kafka broker outage | Broker fleet | High | A.5.29, multi-AZ, consumer replay | Low | Mitigate | SRE |
| R4 | KMS key compromise | Encryption keys | Critical | A.8.24, rotation, audit logs | Medium | Mitigate | SRE |
| R5 | Dependency supply chain attack | Python deps | High | A.8.28, pinned versions, pip-audit | Medium | Mitigate | Eng Lead |
| R6 | Insider data exfiltration | Production DB | High | A.5.15, audit log, JIT access | Medium | Mitigate | ISO |
| R7 | DDoS on data plane | gRPC endpoint | Medium | A.8.6, rate limiter, autoscaling | Low | Mitigate | SRE |
| R8 | Subprocessor breach (cloud provider) | Infra | High | A.5.19, SOC 2 from provider, DPA | Medium | Transfer | ISO |
| R9 | Schema migration corrupts data | Canonical store | High | A.8.32, migration tests, backups | Low | Mitigate | Eng Lead |
| R10 | Loss of a key engineer | Human capital | Medium | A.6.1, documentation, pair programming | Low | Accept | CTO |

---

## 4. Statement of Applicability (SoA)

ISO 27001:2022 Annex A has 93 controls across four themes: Organizational
(A.5), People (A.6), Physical (A.7), Technological (A.8). The table below
states applicability and EntDB-specific implementation.

### 4.1 Organizational Controls (A.5)

| Control | Title | Applicable | Implementation |
|---------|-------|------------|----------------|
| A.5.1 | Policies for information security | Yes | This document |
| A.5.2 | Information security roles and responsibilities | Yes | §1.4 |
| A.5.3 | Segregation of duties | Yes | PR review, deploy gating |
| A.5.4 | Management responsibilities | Yes | CTO signoff on ISMS |
| A.5.5 | Contact with authorities | Yes | DPO maintains regulator contacts |
| A.5.6 | Contact with special interest groups | Yes | Eng team participates in OSS security forums |
| A.5.7 | Threat intelligence | Yes | Dependabot, CVE feeds |
| A.5.8 | Information security in project management | Yes | ADR process |
| A.5.9 | Inventory of information and assets | Yes | §5 |
| A.5.10 | Acceptable use | Yes | Employee handbook |
| A.5.11 | Return of assets | Yes | Offboarding checklist |
| A.5.12 | Classification of information | Yes | §5.2 |
| A.5.13 | Labelling of information | Yes | Data classification tags in canonical store |
| A.5.14 | Information transfer | Yes | TLS in transit, mTLS between services |
| A.5.15 | Access control | Yes | `dbaas/entdb_server/acl.py` |
| A.5.16 | Identity management | Yes | `registry.py`, principal store |
| A.5.17 | Authentication information | Yes | `auth.py`, bearer + mTLS |
| A.5.18 | Access rights | Yes | Quarterly access review |
| A.5.19 | Supplier relationships | Yes | Subprocessor list |
| A.5.20 | Addressing security in supplier agreements | Yes | DPA template |
| A.5.21 | ICT supply chain | Yes | Pinned deps, pip-audit |
| A.5.22 | Monitoring supplier services | Yes | SOC 2 review of providers |
| A.5.23 | Cloud services security | Yes | AWS/GCP hardened baseline |
| A.5.24 | Incident management planning | Yes | `incident-response.md` |
| A.5.25 | Assessment of events | Yes | Metrics alerting |
| A.5.26 | Response to incidents | Yes | `incident-response.md` |
| A.5.27 | Learning from incidents | Yes | Post-incident review template |
| A.5.28 | Collection of evidence | Yes | `soc2-evidence.md` |
| A.5.29 | Information security during disruption | Yes | `business-continuity.md` |
| A.5.30 | ICT readiness for business continuity | Yes | Multi-AZ, DR tests |
| A.5.31 | Legal, statutory, regulatory requirements | Yes | DPO maintains register |
| A.5.32 | Intellectual property rights | Yes | OSS license scanning |
| A.5.33 | Protection of records | Yes | Audit log, append-only |
| A.5.34 | Privacy and PII protection | Yes | `privacy-policy.md` |
| A.5.35 | Independent review | Yes | Annual external audit |
| A.5.36 | Compliance with policies | Yes | Internal audits |
| A.5.37 | Documented operating procedures | Yes | `docs/operations.md` |

### 4.2 People Controls (A.6)

| Control | Title | Applicable | Implementation |
|---------|-------|------------|----------------|
| A.6.1 | Screening | Yes | Background checks on hires |
| A.6.2 | Terms and conditions of employment | Yes | NDA in offer letter |
| A.6.3 | Information security awareness, training | Yes | Annual training |
| A.6.4 | Disciplinary process | Yes | HR policy |
| A.6.5 | Responsibilities after termination | Yes | Offboarding checklist |
| A.6.6 | Confidentiality agreements | Yes | NDA |
| A.6.7 | Remote working | Yes | VPN + MDM |
| A.6.8 | Information security event reporting | Yes | Slack #security-alerts |

### 4.3 Physical Controls (A.7)

Most physical controls are inherited from cloud providers. All are
applicable via provider attestation (AWS SOC 2, GCP ISO 27001).

| Control | Title | Applicable | Implementation |
|---------|-------|------------|----------------|
| A.7.1 | Physical security perimeters | Yes (inherited) | Cloud provider |
| A.7.2 | Physical entry | Yes (inherited) | Cloud provider |
| A.7.3 | Securing offices | Yes | Office access control |
| A.7.4 | Physical security monitoring | Yes (inherited) | Cloud provider |
| A.7.5 | Physical and environmental threats | Yes (inherited) | Cloud provider |
| A.7.6 | Working in secure areas | Yes | Clean desk policy |
| A.7.7 | Clear desk and clear screen | Yes | Policy |
| A.7.8 | Equipment siting | Yes (inherited) | Cloud provider |
| A.7.9 | Security of assets off-premises | Yes | MDM-managed laptops |
| A.7.10 | Storage media | Yes | No removable media |
| A.7.11 | Supporting utilities | Yes (inherited) | Cloud provider |
| A.7.12 | Cabling security | Yes (inherited) | Cloud provider |
| A.7.13 | Equipment maintenance | Yes (inherited) | Cloud provider |
| A.7.14 | Secure disposal | Yes (inherited) | Cloud provider |

### 4.4 Technological Controls (A.8)

| Control | Title | Applicable | Implementation |
|---------|-------|------------|----------------|
| A.8.1 | User endpoint devices | Yes | MDM |
| A.8.2 | Privileged access rights | Yes | JIT access |
| A.8.3 | Information access restriction | Yes | ACL v2 |
| A.8.4 | Access to source code | Yes | GitHub branch protection |
| A.8.5 | Secure authentication | Yes | mTLS + bearer |
| A.8.6 | Capacity management | Yes | Rate limiter, metrics |
| A.8.7 | Protection against malware | Yes | Hardened containers |
| A.8.8 | Vulnerability management | Yes | Dependabot, pip-audit |
| A.8.9 | Configuration management | Yes | Terraform, pinned configs |
| A.8.10 | Information deletion | Yes | Archiver delete API |
| A.8.11 | Data masking | Partially | Log scrubbing |
| A.8.12 | DLP | Partially | Egress monitoring |
| A.8.13 | Information backup | Yes | Kafka + S3 archive |
| A.8.14 | Redundancy | Yes | Multi-AZ |
| A.8.15 | Logging | Yes | Audit log, app logs |
| A.8.16 | Monitoring activities | Yes | `metrics.py`, alerts |
| A.8.17 | Clock synchronization | Yes | NTP on all nodes |
| A.8.18 | Privileged utilities | Yes | Restricted in prod |
| A.8.19 | Installation of software on operational systems | Yes | Immutable images |
| A.8.20 | Network security | Yes | VPC, security groups |
| A.8.21 | Security of network services | Yes | TLS everywhere |
| A.8.22 | Segregation in networks | Yes | Per-env VPCs |
| A.8.23 | Web filtering | N/A | No public browsing |
| A.8.24 | Cryptography | Yes | KMS, TLS 1.2+ |
| A.8.25 | Secure development lifecycle | Yes | PR review, ADRs |
| A.8.26 | Application security requirements | Yes | Threat modeling |
| A.8.27 | Secure system architecture | Yes | `docs/adr/` |
| A.8.28 | Secure coding | Yes | Lint, type-check, review |
| A.8.29 | Security testing | Yes | Unit + integration tests |
| A.8.30 | Outsourced development | N/A | No outsourcing |
| A.8.31 | Separation of dev/test/prod | Yes | Per-env infra |
| A.8.32 | Change management | Yes | PR + CI |
| A.8.33 | Test information | Yes | Synthetic test data only |
| A.8.34 | Protection during audit testing | Yes | Read-only audit access |

---

## 5. Asset Inventory Template

Populate this table and keep it under version control. Review quarterly.

### 5.1 Information Assets

| ID | Asset | Type | Owner | Classification | Storage | Retention |
|----|-------|------|-------|----------------|---------|-----------|
| A1 | Customer events | Data | Eng Lead | Confidential | Kafka + SQLite + S3 | Per customer DPA |
| A2 | Tenant metadata | Data | Eng Lead | Confidential | SQLite | 7 years |
| A3 | Audit logs | Data | SRE | Confidential | SQLite + S3 | 7 years |
| A4 | Source code | Code | CTO | Restricted | GitHub | Indefinite |
| A5 | KMS keys | Secret | SRE | Restricted | AWS KMS / GCP KMS | Rotated annually |
| A6 | TLS certificates | Secret | SRE | Restricted | Vault | Rotated 90 days |
| A7 | CI secrets | Secret | SRE | Restricted | GitHub Actions secrets | Rotated 90 days |
| A8 | Dependency manifests | Config | Eng Lead | Internal | Repo | Indefinite |

### 5.2 Classification Levels

- **Public** — Marketing materials, open docs.
- **Internal** — Source code, internal wikis.
- **Confidential** — Customer data, audit logs, financial data.
- **Restricted** — Secrets, keys, credentials.

### 5.3 System Assets

| ID | System | Owner | Environment | Criticality |
|----|--------|-------|-------------|-------------|
| S1 | EntDB server | Eng Lead | Production | Critical |
| S2 | Kafka cluster | SRE | Production | Critical |
| S3 | SQLite canonical store | Eng Lead | Production | Critical |
| S4 | S3 archive | SRE | Production | High |
| S5 | Console (admin UI) | Eng Lead | Production | High |
| S6 | Metrics pipeline (Prometheus) | SRE | Production | Medium |
| S7 | Alerting (PagerDuty) | SRE | Production | High |
| S8 | CI (GitHub Actions) | Eng Lead | Dev | High |

---

## 6. Document Control

| Field | Value |
|-------|-------|
| Version | 0.1 (draft) |
| Author | Information Security Officer |
| Approver | CTO |
| Effective date | TBD |
| Review cycle | Annual |
| Next review | TBD |
