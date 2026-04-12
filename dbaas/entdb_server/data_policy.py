"""
Data policy definitions for EntDB.

This module defines the data classification policies that control how
node types are handled at runtime. Policies affect retention, auditing,
and compliance behavior.

Invariants:
    - Every node type must have a data_policy (defaults to PERSONAL)
    - FINANCIAL, AUDIT, and HEALTHCARE policies require a legal_basis
    - Policy is set at schema definition time and cached in type_metadata

How to change safely:
    - New policies can be added to the enum
    - Existing policies must not be removed or renamed
    - PERSONAL is the strictest default for unclassified types
"""

from __future__ import annotations

from enum import Enum

# Policies that require a legal_basis to be specified on the type definition
REQUIRES_LEGAL_BASIS = frozenset({"financial", "audit", "healthcare"})


class DataPolicy(str, Enum):
    """Classification policy for a node type's data.

    Each policy implies different handling for retention, encryption,
    audit logging, and compliance controls.

    Values:
        PERSONAL: Contains personal/user data (strictest default)
        BUSINESS: General business data
        FINANCIAL: Financial records (requires legal_basis)
        AUDIT: Audit trail data (requires legal_basis)
        EPHEMERAL: Temporary/session data (may be auto-purged)
        HEALTHCARE: Protected health information (requires legal_basis)
    """

    PERSONAL = "personal"
    BUSINESS = "business"
    FINANCIAL = "financial"
    AUDIT = "audit"
    EPHEMERAL = "ephemeral"
    HEALTHCARE = "healthcare"
