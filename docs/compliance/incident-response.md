# Incident Response Plan

This plan describes how EntDB classifies, responds to, and learns from
security and operational incidents. It is designed to meet SOC 2 CC7.3–
CC7.5, ISO 27001 A.5.24–A.5.27, HIPAA § 164.308(a)(6), GDPR Article
33–34, and US state breach notification laws.

---

## 1. Incident Classification

Incidents are classified into four severity levels. Severity drives
response time, escalation, and notification obligations.

### 1.1 P1 — Critical (data breach, major outage with data impact)

**Definition.** Confirmed or highly probable unauthorized access,
acquisition, use, modification, disclosure, or loss of customer data
or credentials; OR a total service outage lasting or expected to last
more than 30 minutes with data-integrity risk.

**Examples.**

- Unauthorized read of another tenant's data.
- Leaked KMS master key, TLS private key, or database credential.
- Ransomware, data exfiltration, or confirmed malicious insider
  action.
- Corruption of the canonical store with no viable recovery tier.
- Public exposure of the canonical store or a Kafka broker.

**Response targets.**

| Item | Target |
|------|--------|
| Acknowledgement | 15 minutes |
| On-call IC assigned | 15 minutes |
| Containment start | 30 minutes |
| Exec notification | 1 hour |
| Customer status page | 1 hour |
| Customer direct notification | 24 hours |
| Regulator notification (GDPR) | 72 hours from awareness |
| Written customer notice (HIPAA BAA clause) | 5 business days |
| HIPAA individual notification (customer obligation) | 60 days |
| Post-incident review published | 14 days |

### 1.2 P2 — High (availability incident, no confirmed data loss)

**Definition.** Service degradation or partial outage affecting
customers, with no confirmed data loss, integrity impact, or
unauthorized access.

**Examples.**

- Data-plane unavailable for a subset of tenants.
- Sustained error-rate spike above SLO.
- Kafka cluster partially down but replication intact.
- Elevated ACL denials (false positives) impacting legitimate users.

**Response targets.**

| Item | Target |
|------|--------|
| Acknowledgement | 30 minutes |
| On-call IC assigned | 30 minutes |
| Mitigation start | 1 hour |
| Status page update | 1 hour |
| Customer post-mortem | 7 days |

### 1.3 P3 — Medium (integrity or quality issue, limited blast radius)

**Definition.** Incidents affecting correctness or integrity without
data loss or availability impact, or minor security findings.

**Examples.**

- Metric pipeline inaccuracy.
- Non-exploitable vulnerability discovered in dependency scan.
- Isolated schema validation regression.
- Delayed archive flush outside SLO but within durability window.

**Response targets.** Triage within 1 business day, remediation within
SLA defined by severity of the root cause.

### 1.4 P4 — Low (minor, cosmetic, or informational)

**Definition.** Non-customer-impacting bugs, documentation errors,
minor operational noise.

**Response targets.** Next sprint.

---

## 2. Roles

| Role | Responsibilities |
|------|------------------|
| Incident Commander (IC) | Coordinates response, owns timeline, drives severity decisions |
| Technical Lead | Diagnoses, remediates, validates recovery |
| Communications Lead | Updates status page, drafts customer/regulator communications |
| Scribe | Maintains incident timeline in the incident doc |
| Exec Sponsor | P1 only — exec decision-maker, legal liaison |
| Data Protection Officer (DPO) | P1 only — GDPR/CCPA/HIPAA notification obligations |

Roles are assigned at the start of an incident and do not overlap with
the same person except for P3/P4 where one person may hold multiple
roles.

---

## 3. Response Procedures

### 3.1 Detection

Incidents are detected via:

- Automated alerts (metrics, tracing, log anomalies).
- Customer reports (support ticket or email to security@).
- External reporters (bug bounty, security researcher).
- Internal discovery (audit log review, code review).

### 3.2 Triage (all severities)

1. Open the incident channel `#inc-<date>-<slug>` in Slack.
2. Assign IC and roles.
3. Create incident document from the template
   (`docs/compliance/templates/incident.md` if exists, or the template
   at the bottom of this file).
4. Classify severity (§ 1).
5. Announce to internal stakeholders.

### 3.3 Containment (P1/P2)

1. Isolate affected systems: revoke compromised credentials, block
   network routes, stop affected services.
2. Preserve evidence: snapshot logs, audit tables, memory, disks
   before remediation.
3. For suspected data exfiltration, collect egress flow logs and
   Kafka consumer offsets.
4. Take affected tenants read-only if integrity is uncertain.

### 3.4 Eradication

1. Identify root cause (not just the symptom).
2. Apply fix or mitigation.
3. Verify fix in staging before production where possible.
4. Re-run any impacted migrations or recovery procedures from
   `docs/compliance/business-continuity.md`.

### 3.5 Recovery

1. Re-enable traffic to recovered systems.
2. Monitor for recurrence.
3. Restore any read-only tenants to read-write.
4. Confirm SLOs are back within targets.

### 3.6 Communication

All P1 and P2 communications are drafted by the Communications Lead
and reviewed by the DPO (for data-related incidents) or Legal before
release.

### 3.7 Closure

1. IC declares incident closed.
2. Scribe finalizes the incident document.
3. Schedule post-incident review within 7 days.

---

## 4. Breach Notification Timelines

| Regime | Trigger | Notify | Deadline |
|--------|---------|--------|----------|
| **GDPR Art. 33** | Personal data breach | Supervisory authority | 72 hours from awareness |
| **GDPR Art. 34** | High-risk breach | Data subjects | Without undue delay |
| **HIPAA § 164.410** | Breach of Unsecured PHI (BA → CE) | Covered Entity | "Without unreasonable delay"; EntDB BAA: 5 business days |
| **HIPAA § 164.404** | Breach of Unsecured PHI (CE → individuals) | Affected individuals | 60 calendar days (customer obligation) |
| **HIPAA § 164.408** | Breach affecting 500+ | HHS Secretary | 60 days |
| **CCPA** | Breach of specified categories | Attorney General + individuals | "Most expedient time possible", varies by state |
| **US state laws** | Varies | Varies | Varies (typically 30–90 days) |
| **SEC Item 1.05** | Material cybersecurity incident (public co.) | SEC 8-K | 4 business days from materiality determination |

The DPO maintains the mapping of affected individuals and
jurisdictions and owns regulator interaction.

---

## 5. Communication Templates

### 5.1 Internal — Initial Alert

> **INCIDENT P[X] — [short title]**
> IC: @[name]
> Channel: #inc-[date]-[slug]
> Status: INVESTIGATING / MITIGATING / MONITORING / RESOLVED
> Summary: [1-2 sentence description of observed impact]
> Next update: [timestamp, max 30 min for P1, 1 hour for P2]

### 5.2 Customer — Status Page (P1/P2)

> **[Service name] — [Investigating | Identified | Monitoring | Resolved]**
> We are currently investigating reports of [customer-observable
> symptom]. We will provide an update within [timeframe]. Customer
> data is [not at risk | under investigation]. We apologize for the
> disruption and appreciate your patience.
> — [Name], Incident Commander

### 5.3 Customer — Direct Notification (P1 breach)

> Subject: Important Security Notice Regarding Your [Service Name] Account
>
> Dear [Customer Name],
>
> We are writing to inform you of a security incident that may have
> affected your data on [Service Name].
>
> **What happened.** On [date], we [detected / were notified of] [brief
> description]. Our investigation determined that [scope of impact].
>
> **What information was involved.** [Specific data categories].
>
> **What we are doing.** We have [contained / remediated] the incident
> and taken steps to prevent recurrence, including [actions]. We have
> notified [regulators / law enforcement] as required.
>
> **What you can do.** We recommend you [rotate API keys / review audit
> logs / etc.].
>
> **For more information.** Please contact [security@YOURDOMAIN] or
> call [phone]. We will provide updates at [URL].
>
> We take the security of your data very seriously and sincerely
> apologize for this incident.
>
> Sincerely,
> [Name], [Title]

### 5.4 Regulator — GDPR Article 33 Notification

Submit via the supervisory authority's portal within 72 hours. Include:

1. The nature of the personal data breach including, where possible,
   categories and approximate number of data subjects and records
   concerned.
2. The name and contact details of the DPO.
3. Likely consequences of the breach.
4. Measures taken or proposed to address the breach and mitigate
   effects.

If not all information is available, provide in phases without undue
further delay.

---

## 6. Post-Incident Review

Within 14 days of closing a P1, or 7 days for P2, hold a blameless
post-incident review covering:

1. **Timeline** — detection, acknowledgement, key actions, resolution.
2. **Impact** — customers, tenants, data volume, downtime, dollars.
3. **Root cause** — technical, process, and contributing factors.
4. **What went well** — detection speed, comms, tooling.
5. **What went poorly** — delays, confusion, missing runbooks.
6. **Action items** — owners, priorities, due dates, linked issues.
7. **Systemic lessons** — are other systems vulnerable to the same
   failure mode?

Action items are tracked in the engineering backlog and reported to
the ISO monthly until closed. Reviews for P1 incidents are shared with
affected customers in sanitized form.

---

## 7. Training and Drills

- **Tabletop exercises**: quarterly for IC rotation.
- **Game days**: semi-annual failure injection against staging.
- **DR test**: annual (see `docs/compliance/business-continuity.md`).
- **Phishing simulations**: quarterly for all staff.

---

## 8. Incident Document Template

```
# Incident: [title]

- Incident ID: INC-[YYYYMMDD]-[nn]
- Severity: P[X]
- IC: @[name]
- Tech Lead: @[name]
- Comms Lead: @[name]
- Scribe: @[name]
- Start: [timestamp]
- Detected: [timestamp]
- Acknowledged: [timestamp]
- Contained: [timestamp]
- Resolved: [timestamp]
- Status: INVESTIGATING | MITIGATING | MONITORING | RESOLVED

## Impact
[tenants, customers, data categories, downtime, SLO burn]

## Timeline
- [HH:MM] ...

## Actions Taken
- ...

## Customer Communications
- ...

## Regulator / Legal Notifications
- ...

## Root Cause
[technical + process]

## Action Items
- [ ] [item] — owner — due

## Sign-off
- IC: @[name], [date]
- Eng Lead: @[name], [date]
- ISO: @[name], [date]
```
