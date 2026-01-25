"""
Snapshot module for EntDB.

This module handles periodic SQLite snapshots to S3 for:
- Fast database restore (avoid full replay)
- Point-in-time recovery
- Database backup

Invariants:
    - Snapshots are taken during idle periods when possible
    - Snapshots include metadata for restore validation
    - Only complete, consistent snapshots are uploaded
"""

from .snapshotter import SnapshotInfo, Snapshotter

__all__ = ["Snapshotter", "SnapshotInfo"]
