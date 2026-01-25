"""
Apply module for EntDB - event application and materialization.

This module handles:
- Canonical tenant SQLite store (nodes, edges, applied_events)
- Per-user mailbox SQLite store with FTS5
- ACL and visibility management
- Idempotent event application

The stores are materialized views derived from the WAL stream.
They can be rebuilt from scratch by replaying the stream.

Invariants:
    - Applier is idempotent (same event applied twice has no effect)
    - All operations within a TransactionEvent are atomic
    - Visibility index is always consistent with ACL data
    - SQLite uses WAL mode for concurrent reads during writes

How to change safely:
    - Test schema migrations thoroughly before deployment
    - Use transactions for all multi-statement operations
    - Verify idempotency with duplicate event injection tests
"""

from .canonical_store import CanonicalStore, TenantNotFoundError
from .mailbox_store import MailboxStore, MailboxItem
from .acl import AclManager, Principal, AccessDeniedError
from .applier import Applier, ApplierError

__all__ = [
    "CanonicalStore",
    "TenantNotFoundError",
    "MailboxStore",
    "MailboxItem",
    "AclManager",
    "Principal",
    "AccessDeniedError",
    "Applier",
    "ApplierError",
]
