"""Tamper-evident audit log subsystem for EntDB.

Provides hash-chained audit logging, middleware for event capture,
and export capabilities for compliance.
"""

from .audit_middleware import log_audit_event
from .audit_store import AuditStore

__all__ = ["AuditStore", "log_audit_event"]
