"""
Error types for EntDB SDK.

This module defines all exception types raised by the SDK:
- EntDbError: Base exception
- ConnectionError: Server connection issues
- ValidationError: Payload validation failures
- SchemaError: Schema-related errors
- UnknownFieldError: Unknown field in payload

Invariants:
    - All errors inherit from EntDbError
    - Errors include context for debugging
    - Error messages are actionable
"""

from __future__ import annotations

from typing import Any


class EntDbError(Exception):
    """Base exception for all EntDB SDK errors.

    Attributes:
        message: Error message
        code: Error code for programmatic handling
        details: Additional error context
    """

    def __init__(
        self,
        message: str,
        code: str | None = None,
        details: dict[str, Any] | None = None,
    ) -> None:
        super().__init__(message)
        self.message = message
        self.code = code or "ENTDB_ERROR"
        self.details = details or {}


class ConnectionError(EntDbError):
    """Failed to connect to EntDB server.

    Raised when:
    - Server is unreachable
    - Connection times out
    - Authentication fails
    """

    def __init__(
        self,
        message: str,
        address: str | None = None,
    ) -> None:
        super().__init__(
            message,
            code="CONNECTION_ERROR",
            details={"address": address},
        )
        self.address = address


class ValidationError(EntDbError):
    """Payload validation failed.

    Raised when:
    - Required field is missing
    - Field value has wrong type
    - Enum value is invalid
    """

    def __init__(
        self,
        message: str,
        field_name: str | None = None,
        errors: list[str] | None = None,
    ) -> None:
        super().__init__(
            message,
            code="VALIDATION_ERROR",
            details={"field": field_name, "errors": errors or []},
        )
        self.field_name = field_name
        self.errors = errors or []


class SchemaError(EntDbError):
    """Schema-related error.

    Raised when:
    - Schema fingerprint mismatch
    - Unknown type ID
    - Schema compatibility violation
    """

    def __init__(
        self,
        message: str,
        expected_fingerprint: str | None = None,
        actual_fingerprint: str | None = None,
    ) -> None:
        super().__init__(
            message,
            code="SCHEMA_ERROR",
            details={
                "expected_fingerprint": expected_fingerprint,
                "actual_fingerprint": actual_fingerprint,
            },
        )
        self.expected_fingerprint = expected_fingerprint
        self.actual_fingerprint = actual_fingerprint


class UnknownFieldError(EntDbError):
    """Unknown field in payload.

    Includes suggestions for similar field names.

    Attributes:
        field_name: The unknown field
        type_name: The type being created
        suggestions: Similar field names
    """

    def __init__(
        self,
        field_name: str,
        type_name: str,
        suggestions: list[str] | None = None,
    ) -> None:
        suggestions = suggestions or []
        msg = f"Unknown field '{field_name}' in type '{type_name}'"
        if suggestions:
            msg += f". Did you mean: {', '.join(suggestions)}?"

        super().__init__(
            msg,
            code="UNKNOWN_FIELD",
            details={
                "field_name": field_name,
                "type_name": type_name,
                "suggestions": suggestions,
            },
        )
        self.field_name = field_name
        self.type_name = type_name
        self.suggestions = suggestions


class NotFoundError(EntDbError):
    """Resource not found.

    Raised when:
    - Node doesn't exist
    - Edge doesn't exist
    - Tenant doesn't exist
    """

    def __init__(
        self,
        message: str,
        resource_type: str,
        resource_id: str,
    ) -> None:
        super().__init__(
            message,
            code="NOT_FOUND",
            details={
                "resource_type": resource_type,
                "resource_id": resource_id,
            },
        )
        self.resource_type = resource_type
        self.resource_id = resource_id


class AccessDeniedError(EntDbError):
    """Access denied.

    Raised when:
    - Actor lacks required permission
    - Node visibility doesn't include actor
    """

    def __init__(
        self,
        message: str,
        actor: str,
        resource_id: str,
        required_permission: str | None = None,
    ) -> None:
        super().__init__(
            message,
            code="ACCESS_DENIED",
            details={
                "actor": actor,
                "resource_id": resource_id,
                "required_permission": required_permission,
            },
        )
        self.actor = actor
        self.resource_id = resource_id
        self.required_permission = required_permission


class TransactionError(EntDbError):
    """Transaction failed.

    Raised when:
    - Atomic operation partially failed
    - Idempotency violation
    - Concurrent modification
    """

    def __init__(
        self,
        message: str,
        idempotency_key: str | None = None,
    ) -> None:
        super().__init__(
            message,
            code="TRANSACTION_ERROR",
            details={"idempotency_key": idempotency_key},
        )
        self.idempotency_key = idempotency_key


class UniqueConstraintError(EntDbError):
    """Raised when a write would violate a declared unique field.

    Implements the client-facing side of the 2026-04-14 SDK v0.3
    decision (``docs/decisions/sdk_api.md``). Unique fields are
    declared on the proto field via ``(entdb.field).unique = true``
    and enforced server-side by a unique expression index on the
    payload. Duplicates surface either as a gRPC ``ALREADY_EXISTS``
    (pre-validate fast-fail in the gRPC layer) or via a failed apply
    at the unique-index INSERT (authoritative race winner). Both
    paths are mapped into this typed error.

    Composite (multi-field) unique violations from
    ``(entdb.node).composite_unique`` map onto the same exception
    type so callers only need a single ``except`` clause.
    ``constraint_name`` / ``field_ids`` / ``values`` are populated for
    the composite form; ``field_id`` / ``value`` are populated for
    the single-field form. Single-field is the legacy / common case
    so the simple attributes stay first-class.

    Attributes:
        tenant_id: Tenant where the collision occurred.
        type_id: Numeric node type id.
        field_id: Numeric field id of the unique field (single-field).
        value: The colliding scalar value (single-field).
        constraint_name: Composite-constraint name (composite only).
        field_ids: Tuple of field ids participating in the composite
            constraint (composite only).
        values: Tuple of colliding values aligned with ``field_ids``
            (composite only).
    """

    def __init__(
        self,
        message: str,
        *,
        tenant_id: str | None = None,
        type_id: int | None = None,
        field_id: int | None = None,
        value: Any = None,
        constraint_name: str | None = None,
        field_ids: tuple[int, ...] | None = None,
        values: tuple[Any, ...] | None = None,
    ) -> None:
        super().__init__(
            message,
            code="UNIQUE_CONSTRAINT",
            details={
                "tenant_id": tenant_id,
                "type_id": type_id,
                "field_id": field_id,
                "value": value,
                "constraint_name": constraint_name,
                "field_ids": list(field_ids) if field_ids is not None else None,
                "values": list(values) if values is not None else None,
            },
        )
        self.tenant_id = tenant_id
        self.type_id = type_id
        self.field_id = field_id
        self.value = value
        self.constraint_name = constraint_name
        self.field_ids = field_ids
        self.values = values

    @property
    def is_composite(self) -> bool:
        """``True`` when this is a composite (multi-field) violation."""
        return self.constraint_name is not None


class RateLimitError(EntDbError):
    """Request was rejected because a rate limit or quota was exceeded.

    The server returns this with gRPC ``RESOURCE_EXHAUSTED`` status and
    a ``retry-after`` trailing metadata field (seconds). The SDK parses
    both and surfaces a typed error so applications can back off
    cleanly.

    This error type is shared across all three rate-limit layers:

    - **Per-tenant monthly quota** — ``retry_after_ms`` is the number
      of milliseconds until the next calendar-month period starts.
    - **Per-tenant burst (token bucket)** — ``retry_after_ms`` is the
      time until the bucket refills enough for one more request.
    - **Per-user abuse protection** — ``retry_after_ms`` is the time
      until the per-user bucket has capacity.

    Applications typically catch this, log, back off for
    ``retry_after_ms``, and retry. The ``DbClient`` can be configured
    with ``auto_retry_on_throttle=True`` to do this transparently.
    """

    def __init__(
        self,
        message: str,
        *,
        retry_after_ms: int | None = None,
        limit: int | None = None,
        used: int | None = None,
    ) -> None:
        super().__init__(
            message,
            code="RATE_LIMITED",
            details={
                "retry_after_ms": retry_after_ms,
                "limit": limit,
                "used": used,
            },
        )
        self.retry_after_ms = retry_after_ms
        self.limit = limit
        self.used = used
