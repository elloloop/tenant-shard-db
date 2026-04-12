# SOC 2 Compliance Audit

You are a SOC 2 auditor examining EntDB for Trust Service Criteria compliance. Run a thorough audit simulating what a human SOC 2 auditor would check. Create a detailed report.

## Audit Team
Assign yourself these roles and check from each perspective:
- **Lead Auditor**: overall assessment, risk rating
- **Security Analyst**: encryption, access controls, vulnerability management  
- **Operations Analyst**: availability, monitoring, backup, incident response
- **Privacy Analyst**: data handling, confidentiality, processing integrity

## Audit Checklist

### CC1: Control Environment
- [ ] Check: Is there an ARCHITECTURE.md or equivalent system design document?
- [ ] Check: Are ADRs maintained and current? (docs/adr/)
- [ ] Check: Is there a CONTRIBUTING.md with security guidelines?
- [ ] Check: Is branch protection enabled on main?

### CC2: Communication and Information
- [ ] Check: Are error messages informative but not leaking internals?
- [ ] Check: Is there API documentation?
- [ ] Check: Are security-relevant configurations documented?

### CC3: Risk Assessment
- [ ] Check: Are dependencies pinned and scanned? (pyproject.toml, Dependabot)
- [ ] Check: Is there a known vulnerability in any dependency? (run `pip audit` or check GitHub Security tab)
- [ ] Check: Are there TODO/FIXME/HACK comments indicating unresolved security issues?

### CC5: Control Activities
- [ ] Check: Is authentication enforced? (grep for auth interceptor in grpc_server.py)
- [ ] Check: Is rate limiting active? (check rate_limiter.py)
- [ ] Check: Is input validation present? (schema validation, SQL parameterization)
- [ ] Check: Are there any raw SQL string concatenations? (grep for f-string SQL)
- [ ] Check: Is TLS configured? (check server.py for TLS options)

### CC6: Security
- [ ] Check: Encryption at rest — is SQLCipher or equivalent integrated?
- [ ] Check: Encryption in transit — is TLS enforced in production?
- [ ] Check: Are API keys stored as hashes, not plaintext?
- [ ] Check: Is there key rotation capability?
- [ ] Check: Access control — is ACL enforced on every read? (check canonical_store.py)
- [ ] Check: Are there any bypass paths where ACL is skipped (other than __system__)?
- [ ] Check: Is the __system__ actor properly restricted?

### CC7: Operations
- [ ] Check: Is there monitoring? (check metrics.py, Prometheus endpoints)
- [ ] Check: Is there tracing? (check tracing.py, OpenTelemetry)
- [ ] Check: Is there a health check endpoint? (check Health RPC)
- [ ] Check: Are backups configured? (check archiver, recovery)
- [ ] Check: Is there a backup restore test?
- [ ] Check: Is there an incident response procedure documented?

### CC8: Change Management
- [ ] Check: Is CI/CD configured? (.github/workflows/)
- [ ] Check: Are tests comprehensive? (count tests, check coverage)
- [ ] Check: Is schema evolution validated? (entdb check, compat.py)
- [ ] Check: Are breaking changes blocked in CI?

### CC9: Risk Mitigation  
- [ ] Check: Is there a WAL for durability? (check wal/ directory)
- [ ] Check: Is there tiered recovery? (snapshot → WAL → archive)
- [ ] Check: Is there data replication? (Kafka replication factor)

## Audit Process

1. Read ARCHITECTURE.md and all ADRs first
2. For each checklist item, examine the actual code
3. Rate each item: PASS / FAIL / PARTIAL / NOT APPLICABLE
4. For FAILs and PARTIALs, describe the gap and suggest remediation
5. Calculate overall SOC 2 readiness percentage
6. Produce a final report with:
   - Executive summary
   - Findings table (item, status, evidence, remediation)
   - Risk rating (Critical / High / Medium / Low per finding)
   - Recommended priority order for remediation

## Output Format
Create the report as `audit-reports/soc2-audit-{date}.md`
