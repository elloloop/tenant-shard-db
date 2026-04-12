# ADR-012: HIPAA Readiness

## Status: Accepted

## Context

Healthcare customers (telemedicine, EHR, health tracking apps like period trackers) require HIPAA compliance. EntDB must support Protected Health Information (PHI) with appropriate safeguards.

## Decision

### PHI data classification in schema

```protobuf
message PatientRecord {
    option (entdb.type_id) = 10;
    option (entdb.data_policy) = HEALTHCARE;  // new policy level
    option (entdb.retention_days) = 2190;     // 6 years (HIPAA requirement)
    option (entdb.legal_basis) = "HIPAA §164.530(j) — 6 year retention";

    string patient_id = 1    [(entdb.pii) = true, (entdb.phi) = true];
    string diagnosis = 2     [(entdb.phi) = true];
    string medication = 3    [(entdb.phi) = true];
    string notes = 4         [(entdb.phi) = true];
}
```

New field annotation: `(entdb.phi) = true` — marks Protected Health Information. Stricter than PII:
- PHI fields are encrypted at the field level (not just file level)
- PHI access is logged with enhanced detail (who accessed what, when, from where)
- PHI export requires additional authorization
- PHI cannot be included in analytics or aggregations

### HEALTHCARE data policy

```
HEALTHCARE:
  Export:      to patient only (via authorized request)
  On exit:     RETAIN (6 years minimum per HIPAA)
  Anonymize:   de-identify per HIPAA Safe Harbor (18 identifiers removed)
  Access log:  enhanced — every PHI access logged with IP, user agent, reason
  Encryption:  field-level for phi=true fields (in addition to file-level)
  Minimum necessary: query only returns fields the actor needs (field-level ACL)
```

### Enhanced access logging for PHI

```sql
-- Standard audit log entry
{ action: "read", target: "node-123", actor: "user:doctor" }

-- PHI-enhanced audit log entry  
{ action: "read_phi", target: "node-123", actor: "user:doctor",
  fields_accessed: ["diagnosis", "medication"],
  access_reason: "treatment",
  ip_address: "10.0.1.50",
  session_id: "sess-abc",
  phi_flag: true }
```

### Business Associate Agreement (BAA)

EntDB as a service provides a BAA template. The BAA covers:
- EntDB as data processor for PHI
- Encryption obligations
- Breach notification (within 60 days per HIPAA)
- Subcontractor obligations (Kafka provider, S3 provider)

### HIPAA minimum necessary rule

Field-level access control (future enhancement):
```protobuf
message PatientRecord {
    string patient_id = 1  [(entdb.phi) = true, (entdb.access_level) = "admin"];
    string diagnosis = 2   [(entdb.phi) = true, (entdb.access_level) = "provider"];
    string medication = 3  [(entdb.phi) = true, (entdb.access_level) = "provider"];
    string notes = 4       [(entdb.phi) = true, (entdb.access_level) = "provider"];
    string billing_code = 5 [(entdb.access_level) = "billing"];
}

// Nurse (provider role) sees: diagnosis, medication, notes
// Billing clerk sees: billing_code only
// Admin sees: everything
```

## Consequences

- HEALTHCARE policy adds field-level encryption overhead
- PHI access logging adds storage and latency (~1ms per access)
- 6-year retention means significant storage over time
- BAA requirement limits which cloud providers can be used
- De-identification must follow HIPAA Safe Harbor method (18 identifiers)
- Field-level ACL is a future enhancement (not in v1)
