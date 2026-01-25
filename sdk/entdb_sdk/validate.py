"""
Payload validation for EntDB SDK.

This module provides validation utilities:
- Field-level validation
- Payload validation against types
- Helpful error messages with suggestions

Invariants:
    - Validation errors are deterministic
    - Error messages include context for fixing
    - Unknown fields suggest similar valid fields
"""

from __future__ import annotations

from difflib import get_close_matches
from typing import Any, Dict, List, Optional, Tuple

from .schema import NodeTypeDef, EdgeTypeDef, FieldKind
from .errors import ValidationError, UnknownFieldError


def validate_payload(
    node_type: NodeTypeDef,
    payload: Dict[str, Any],
) -> Tuple[bool, List[str]]:
    """Validate payload against node type.

    Args:
        node_type: Node type to validate against
        payload: Payload to validate

    Returns:
        Tuple of (is_valid, list_of_errors)
    """
    errors: List[str] = []

    # Check for unknown fields
    known_fields = {f.name for f in node_type.fields}
    unknown = set(payload.keys()) - known_fields
    if unknown:
        for field_name in unknown:
            suggestions = get_close_matches(field_name, list(known_fields), n=3)
            if suggestions:
                errors.append(f"Unknown field '{field_name}'. Did you mean: {suggestions}?")
            else:
                errors.append(f"Unknown field '{field_name}'")

    # Validate each field
    for field_def in node_type.fields:
        value = payload.get(field_def.name, field_def.default)

        # Check required
        if value is None and field_def.required:
            errors.append(f"Field '{field_def.name}' is required")
            continue

        if value is None:
            continue

        # Type-specific validation
        error = _validate_field_value(field_def.name, field_def.kind, value, field_def.enum_values)
        if error:
            errors.append(error)

    return len(errors) == 0, errors


def _validate_field_value(
    name: str,
    kind: FieldKind,
    value: Any,
    enum_values: Optional[Tuple[str, ...]] = None,
) -> Optional[str]:
    """Validate a single field value.

    Returns error message if invalid, None if valid.
    """
    if kind == FieldKind.STRING:
        if not isinstance(value, str):
            return f"Field '{name}' must be a string, got {type(value).__name__}"

    elif kind == FieldKind.INTEGER:
        if not isinstance(value, int) or isinstance(value, bool):
            return f"Field '{name}' must be an integer, got {type(value).__name__}"

    elif kind == FieldKind.FLOAT:
        if not isinstance(value, (int, float)) or isinstance(value, bool):
            return f"Field '{name}' must be a number, got {type(value).__name__}"

    elif kind == FieldKind.BOOLEAN:
        if not isinstance(value, bool):
            return f"Field '{name}' must be a boolean, got {type(value).__name__}"

    elif kind == FieldKind.TIMESTAMP:
        if not isinstance(value, int) or value < 0:
            return f"Field '{name}' must be a positive integer timestamp"

    elif kind == FieldKind.ENUM:
        if not isinstance(value, str):
            return f"Field '{name}' must be a string, got {type(value).__name__}"
        if enum_values and value not in enum_values:
            return f"Field '{name}' must be one of {enum_values}, got '{value}'"

    elif kind == FieldKind.LIST_STRING:
        if not isinstance(value, list):
            return f"Field '{name}' must be a list, got {type(value).__name__}"
        for i, item in enumerate(value):
            if not isinstance(item, str):
                return f"Field '{name}[{i}]' must be a string"

    elif kind == FieldKind.LIST_INT:
        if not isinstance(value, list):
            return f"Field '{name}' must be a list, got {type(value).__name__}"
        for i, item in enumerate(value):
            if not isinstance(item, int) or isinstance(item, bool):
                return f"Field '{name}[{i}]' must be an integer"

    elif kind == FieldKind.REFERENCE:
        if not isinstance(value, dict):
            return f"Field '{name}' must be a reference object"
        if "type_id" not in value or "id" not in value:
            return f"Field '{name}' must have 'type_id' and 'id'"

    return None


def validate_or_raise(
    node_type: NodeTypeDef,
    payload: Dict[str, Any],
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
) -> List[str]:
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
