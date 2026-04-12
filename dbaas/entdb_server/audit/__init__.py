"""Audit compliance module.

The WAL (S3/Kafka/Kinesis) is the primary audit log — immutable,
append-only, and tamper-evident via S3 Object Lock. This module
provides helpers for:

- Configuring S3 Object Lock on the WAL bucket
- Exporting WAL events as compliance-formatted audit reports
"""

from .compliance import export_audit_trail
from .s3_lock import S3ObjectLockConfig, configure_s3_object_lock

__all__ = [
    "S3ObjectLockConfig",
    "configure_s3_object_lock",
    "export_audit_trail",
]
