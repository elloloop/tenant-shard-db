"""
Archive module for EntDB.

This module handles archiving WAL events to S3 for:
- Long-term durability beyond stream retention
- Disaster recovery
- Compliance and audit requirements

Invariants:
    - Archives are immutable once written
    - Archives contain checksums for integrity
    - Archive format is self-describing for restore
"""

from .archiver import Archiver, ArchiveSegment

__all__ = ["Archiver", "ArchiveSegment"]
