# EntDB Compliance Documentation

This directory contains the compliance documentation layer for EntDB:
policies, templates, runbooks, and evidence guides supporting SOC 2,
ISO 27001, HIPAA, GDPR, CCPA, and standard business continuity and
incident response practices.

> **Auditor quick reference.** Start with `soc2-evidence.md` for
> control-to-evidence mapping, then browse individual documents in the
> order that matches your audit scope.

---

## Contents

| Document | Purpose | Audience |
|----------|---------|----------|
| [soc2-evidence.md](./soc2-evidence.md) | SOC 2 Trust Service Criteria evidence collection guide | Auditors, ISO |
| [iso27001-isms.md](./iso27001-isms.md) | ISO 27001 ISMS: policy, risk methodology, SoA, asset inventory | Auditors, ISO, management |
| [baa-template.md](./baa-template.md) | HIPAA Business Associate Agreement template | Legal, customers |
| [privacy-policy.md](./privacy-policy.md) | GDPR/CCPA-ready privacy policy template | Legal, customers, end users |
| [incident-response.md](./incident-response.md) | Incident classification, procedures, and breach notification | SRE, DPO, IC rotation |
| [business-continuity.md](./business-continuity.md) | BCP/DR: RTO/RPO, recovery tiers, annual DR test | SRE, management |

---

## Related artifacts

- **Operations guide.** [`docs/operations.md`](../operations.md) —
  day-two operational procedures.
- **Deployment guide.** [`docs/deployment.md`](../deployment.md) —
  infrastructure and environment setup.
- **Durability guide.** [`docs/durability.md`](../durability.md) —
  Kafka and storage durability design.
- **Schema evolution.** [`docs/schema-evolution.md`](../schema-evolution.md)
  — change management for data schemas.
- **Getting started.** [`docs/getting-started.md`](../getting-started.md)
  — quickstart for operators and developers.

## Audit skill commands

The following Claude Code skills are available to run targeted audits
against the repository:

- `/audit-soc2` — SOC 2 Type I/II readiness audit.
- `/audit-iso27001` — ISO 27001 gap analysis.
- `/audit-hipaa` — HIPAA Security Rule audit.
- `/audit-gdpr` — GDPR Article 5/24/32 audit.
- `/audit-ccpa` — CCPA/CPRA readiness audit.
- `/audit-all` — Full compliance audit across all certifications.

## Automated evidence collection

```bash
python scripts/collect_soc2_evidence.py
```

Produces `evidence/<YYYY-MM-DD>/` containing JSON reports for each
control family. Upload the directory to your audit workspace (Drata,
Vanta, Tugboat Logic) or archive for internal records.

---

## Compliance roadmap

| Milestone | Target | Status |
|-----------|--------|--------|
| SOC 2 Type I | — | Ready for audit |
| SOC 2 Type II (12-month observation) | — | Starts after Type I |
| ISO 27001:2022 certification | — | Stage 1 ready |
| HIPAA BAA availability | — | Template ready |
| GDPR processor addendum | — | Template ready |
| CCPA disclosures | — | In privacy policy template |
| Annual DR test | — | Procedure documented |
| Annual penetration test | — | Vendor TBD |

## Document control

All documents in this directory are versioned in Git. Review cadence:

- **Annual** — Privacy policy, BCP/DR plan, ISMS (A.5.1 / CC5.3).
- **Quarterly** — Risk register, asset inventory.
- **On change** — Incident response after every P1 post-mortem.

Material changes require ISO review and, for customer-facing documents
(privacy policy, BAA), Legal sign-off.
