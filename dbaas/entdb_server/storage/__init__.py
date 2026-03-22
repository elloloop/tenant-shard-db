"""
Pluggable object storage abstraction.

Supports S3/R2/MinIO, Azure Blob Storage, Google Cloud Storage,
and local filesystem. All interchangeable via STORAGE_BACKEND env var.
"""

from .base import ObjectMeta, ObjectStore, create_object_store

__all__ = ["ObjectStore", "ObjectMeta", "create_object_store"]
