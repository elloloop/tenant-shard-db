"""
Payload validation for EntDB SDK.

This module provides thin wrappers around ``FieldDef.validate_value`` so
that callers can validate payloads against a ``NodeTypeDef`` without
needing to call into the schema dataclasses directly.

All per-field type checking logic lives in ``FieldDef.validate_value``
in ``schema.py``. This module is intentionally the only place that adds
friendly error decoration on top — unknown-field suggestions, raising
exceptions, etc.

Invariants:
    - Validation errors are deterministic
    - Error messages include context for fixing
    - Unknown fields suggest similar valid fields
    - Per-kind type checking is delegated to FieldDef (single source of truth)
"""

from __future__ import annotations

from difflib import get_close_matches
from typing import Any

from .errors import UnknownFieldError, ValidationError
from .schema import NodeTypeDef


def validate_payload(
    node_type: NodeTypeDef,
    payload: dict[str, Any],
) -> tuple[bool, list[str]]:
    """Validate payload against node type.

    Delegates per-field type checking to ``FieldDef.validate_value`` so
    that ``validate_payload`` and ``NodeTypeDef.validate_payload`` cannot
    drift apart.

    Args:
        node_type: Node type to validate against
        payload: Payload to validate

    Returns:
        Tuple of (is_valid, list_of_errors)
    """
    errors: list[str] = []

    # Check for unknown fields (with friendly suggestions)
    known_fields = {f.name for f in node_type.fields}
    unknown = set(payload.keys()) - known_fields
    if unknown:
        for field_name in unknown:
            suggestions = get_close_matches(field_name, list(known_fields), n=3)
            if suggestions:
                errors.append(f"Unknown field '{field_name}'. Did you mean: {suggestions}?")
            else:
                errors.append(f"Unknown field '{field_name}'")

    # Validate each field using FieldDef.validate_value (single source of truth)
    for field_def in node_type.fields:
        value = payload.get(field_def.name, field_def.default)
        is_valid, error = field_def.validate_value(value)
        if not is_valid and error:
            errors.append(error)

    return len(errors) == 0, errors


def validate_or_raise(
    node_type: NodeTypeDef,
    payload: dict[str, Any],
) -> None:
    """Validate payload and raise if invalid.

    Args:
        node_type: Node type to validate against
        payload: Payload to validate

    Raises:
        UnknownFieldError: If unknown field is provided
        ValidationError: If validation fails
    """
    # Check unknown fields first (for better error messages)
    known_fields = {f.name for f in node_type.fields}
    unknown = set(payload.keys()) - known_fields

    if unknown:
        field_name = list(unknown)[0]
        suggestions = get_close_matches(field_name, list(known_fields), n=3)
        raise UnknownFieldError(field_name, node_type.name, suggestions)

    is_valid, errors = validate_payload(node_type, payload)
    if not is_valid:
        raise ValidationError(
            f"Validation failed for {node_type.name}: {'; '.join(errors)}",
            errors=errors,
        )


def suggest_fields(
    partial: str,
    node_type: NodeTypeDef,
    limit: int = 5,
) -> list[str]:
    """Suggest field names based on partial input.

    Args:
        partial: Partial field name
        node_type: Node type to suggest from
        limit: Maximum suggestions

    Returns:
        List of suggested field names
    """
    known = [f.name for f in node_type.fields if not f.deprecated]
    matches = get_close_matches(partial, known, n=limit)

    # Also include prefix matches
    prefix_matches = [n for n in known if n.lower().startswith(partial.lower())]

    # Combine and deduplicate
    all_matches = list(dict.fromkeys(matches + prefix_matches))
    return all_matches[:limit]
