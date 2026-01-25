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

from typing import Any, Dict, List, Optional


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
        code: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
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
        address: Optional[str] = None,
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
        field_name: Optional[str] = None,
        errors: Optional[List[str]] = None,
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
        expected_fingerprint: Optional[str] = None,
        actual_fingerprint: Optional[str] = None,
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
        suggestions: Optional[List[str]] = None,
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
        required_permission: Optional[str] = None,
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
        idempotency_key: Optional[str] = None,
    ) -> None:
        super().__init__(
            message,
            code="TRANSACTION_ERROR",
            details={"idempotency_key": idempotency_key},
        )
        self.idempotency_key = idempotency_key
