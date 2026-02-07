"""
CLI tools for EntDB administration.

This module provides command-line tools for:
- restore: Rebuild tenant database from snapshot + archive
- schema: Manage and validate schema definitions

Invariants:
    - Tools work offline (no running server required)
    - Operations are idempotent where possible
    - All operations are logged for audit
"""

from .restore import RestoreTool
from .schema_cli import SchemaCLI

__all__ = ["RestoreTool", "SchemaCLI"]
