"""
Schema validation for node payloads.

Validates that node payloads match the registered schema type definitions.
Validation is optional -- disabled when no schema is registered or when
SCHEMA_VALIDATION_ENABLED=false.
"""

from __future__ import annotations

import logging
from typing import Any

logger = logging.getLogger(__name__)


class SchemaValidationError(Exception):
    """Payload does not match the schema."""

    pass


def validate_payload(
    type_id: int,
    payload: dict[str, Any],
    registry: Any,
    strict: bool = False,
) -> list[str]:
    """Validate payload against schema for a node type.

    Args:
        type_id: Node type ID
        payload: Payload to validate
        registry: Schema registry instance
        strict: If True, reject unknown fields

    Returns:
        List of validation error strings (empty = valid)
    """
    errors: list[str] = []

    if not registry or not hasattr(registry, "get_node_type"):
        return errors  # No schema to validate against

    node_type = registry.get_node_type(type_id)
    if not node_type:
        return errors  # Unknown type, allow (schema-less mode)

    # Check required fields
    for field_def in node_type.fields:
        if field_def.required and field_def.name not in payload:
            errors.append(f"Required field '{field_def.name}' is missing")

    # Check field types
    for field_def in node_type.fields:
        if field_def.name in payload:
            value = payload[field_def.name]
            type_error = _check_field_type(field_def, value)
            if type_error:
                errors.append(type_error)

    # Check for unknown fields in strict mode
    if strict:
        known_fields = {f.name for f in node_type.fields}
        for key in payload:
            if key not in known_fields:
                errors.append(f"Unknown field '{key}' not in schema")

    return errors


def _check_field_type(field_def: Any, value: Any) -> str | None:
    """Check if a value matches the expected field type."""
    kind = field_def.kind.value if hasattr(field_def.kind, "value") else str(field_def.kind)

    type_checks: dict[str, Any] = {
        "str": lambda v: isinstance(v, str),
        "int": lambda v: isinstance(v, int) and not isinstance(v, bool),
        "float": lambda v: isinstance(v, int | float) and not isinstance(v, bool),
        "bool": lambda v: isinstance(v, bool),
        "list_str": lambda v: isinstance(v, list) and all(isinstance(i, str) for i in v),
        "list_int": lambda v: isinstance(v, list) and all(isinstance(i, int) for i in v),
        "json": lambda _: True,
        "bytes": lambda v: isinstance(v, str | bytes),
        "timestamp": lambda v: isinstance(v, int) and v >= 0,
        "enum": lambda v: isinstance(v, str),  # enum validation done separately
    }

    checker = type_checks.get(kind)
    if checker and not checker(value):
        return f"Field '{field_def.name}' expected type '{kind}', got {type(value).__name__}"

    # Check enum values
    if kind == "enum" and hasattr(field_def, "enum_values") and field_def.enum_values:
        if value not in field_def.enum_values:
            return (
                f"Field '{field_def.name}' value '{value}' "
                f"not in allowed values: {field_def.enum_values}"
            )

    return None
