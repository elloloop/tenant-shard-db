# Full Compliance Audit — All Certifications

You are the Chief Compliance Officer running a comprehensive audit across all certification requirements. Coordinate a team of specialist auditors.

## Process

1. Create the `audit-reports/` directory if it doesn't exist

2. Run each audit IN PARALLEL by spawning agents:

   **Agent 1 — SOC 2 Auditor**
   Run /audit-soc2
   
   **Agent 2 — GDPR Data Protection Officer**
   Run /audit-gdpr
   
   **Agent 3 — HIPAA Compliance Officer**
   Run /audit-hipaa
   
   **Agent 4 — ISO 27001 Lead Auditor**
   Run /audit-iso27001
   
   **Agent 5 — CCPA Privacy Analyst**
   Run /audit-ccpa

3. After all audits complete, create a MASTER REPORT:

   `audit-reports/compliance-summary-{date}.md`

   Include:
   
   ### Executive Summary
   Overall compliance posture. Red/amber/green per certification.
   
   ### Readiness Dashboard
   ```
   Certification    Ready?   Score    Blockers
   SOC 2 Type I     ___      ___%     ___
   SOC 2 Type II    ___      ___%     ___
   ISO 27001        ___      ___%     ___
   GDPR             ___      ___%     ___
   CCPA             ___      ___%     ___
   HIPAA            ___      ___%     ___
   ```
   
   ### Cross-Certification Gaps
   Findings that affect MULTIPLE certifications (fix once, pass many).
   Prioritize these — highest ROI.
   
   ### Priority Remediation Plan
   Ordered by: (number of certifications affected × severity)
   1. ___
   2. ___
   3. ___
   ...
   
   ### Timeline to Certification
   Estimated weeks to each certification assuming remediation starts now.

4. Create GitHub issues for the top 10 remediation items, labeled "compliance".

## Schedule
This audit should be run:
- Monthly during development (track progress)
- Quarterly after initial certification (maintain compliance)
- After any major architecture change
- Before any certification audit engagement
