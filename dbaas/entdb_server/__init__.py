"""
EntDB Server - Production-grade DBaaS for tenant-sharded graph storage.

This package implements a database-as-a-service system built on:
- Nodes and unidirectional Edges as the core data model
- WAL (Write-Ahead Log) stream as the source of truth
- SQLite as materialized views (canonical tenant store + per-user mailboxes)
- S3 for archiving and snapshots

Architecture:
    ┌─────────────┐     ┌─────────────┐     ┌─────────────────┐
    │   Client    │────▶│  gRPC/HTTP  │────▶│ TransactionEvent│
    │   (SDK)     │     │   Server    │     │   Producer      │
    └─────────────┘     └─────────────┘     └────────┬────────┘
                                                     │
                                                     ▼
                        ┌─────────────────────────────────────────┐
                        │           WAL Stream (Kafka/Kinesis)    │
                        └─────────────────────────────────────────┘
                                             │
                        ┌────────────────────┼────────────────────┐
                        │                    │                    │
                        ▼                    ▼                    ▼
                   ┌─────────┐         ┌─────────┐         ┌─────────┐
                   │ Applier │         │Archiver │         │Snapshot │
                   └────┬────┘         └────┬────┘         └────┬────┘
                        │                   │                   │
                        ▼                   ▼                   ▼
                   ┌─────────┐         ┌─────────┐         ┌─────────┐
                   │ SQLite  │         │   S3    │         │   S3    │
                   │ (views) │         │(archive)│         │(snapshots)│
                   └─────────┘         └─────────┘         └─────────┘

Invariants:
    - The WAL stream is the source of truth
    - SQLite databases are derived views that can be rebuilt
    - All operations require tenant_id and actor
    - Schema IDs (type_id, edge_id, field_id) are immutable once assigned
    - Idempotency keys ensure exactly-once semantics

How to change safely:
    - Schema changes must follow protobuf-like evolution rules
    - New fields can be added with new field_ids
    - Fields cannot be removed, only deprecated
    - IDs cannot be reused after deprecation

Version: see VERSION file at project root.
"""

from ._version import __version__

__all__ = ["__version__"]
