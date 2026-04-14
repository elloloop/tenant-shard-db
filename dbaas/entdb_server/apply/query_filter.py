"""
MongoDB-style query operator translation for EntDB.

Implements the query-operator half of the 2026-04-14 SDK v0.3 decision:
``canonical_store.query_nodes`` accepts a filter dict with MongoDB-style
operators on payload fields and this module translates it into a
parameterised SQL WHERE fragment.

Supported operators:

- ``$eq`` (default when value is a scalar) — equality
- ``$ne`` — inequality
- ``$gt``, ``$gte``, ``$lt``, ``$lte`` — range
- ``$in`` — membership in a list
- ``$nin`` — complement of membership
- ``$like`` — SQL LIKE pattern
- ``$between`` — inclusive ``BETWEEN lo AND hi`` (list of two)
- ``$and``, ``$or`` — boolean composition at the top level or nested

Field names are looked up against the schema registry for the given
``type_id`` so on-disk ``field_id`` keys are used in the generated SQL
(the stored payload is keyed by id, not name). Unknown field names
raise ``QueryFilterError``; invalid operator names raise the same.

Security: operator names are validated against a closed allow-list
before being mapped to SQL tokens. Field ids are integers from the
schema registry, not strings from user input, so the ``json_extract``
path literal cannot be used as a SQL-injection vector. Values are
always bound as parameters.

Invariants:
    - Operator token is validated before use.
    - Field name is resolved to ``field_id: int`` via the registry or
      the call is rejected — there is no fallthrough that silently
      scans all rows when a field is missing.
    - ``$in`` / ``$nin`` / ``$between`` require list values of the
      expected shape; wrong shapes raise ``QueryFilterError``.

Example:
    >>> sql, params = compile_query_filter(
    ...     {"status": {"$in": ["todo", "doing"]}},
    ...     101, registry)
    >>> sql
    '(json_extract(payload_json, \\'$."2"\\') IN (?, ?))'
    >>> params
    ['todo', 'doing']
"""

from __future__ import annotations

import re
from typing import Any

__all__ = ["QueryFilterError", "compile_query_filter"]


class QueryFilterError(ValueError):
    """Raised when a query filter dict is malformed."""


# Allow-list of scalar comparison operators mapped to their SQL form.
_SCALAR_OPS: dict[str, str] = {
    "$eq": "=",
    "$ne": "!=",
    "$gt": ">",
    "$gte": ">=",
    "$lt": "<",
    "$lte": "<=",
}

# Allow-list of list-valued operators.
_LIST_OPS: frozenset[str] = frozenset({"$in", "$nin"})

# Pattern / range operators.
_PATTERN_OPS: frozenset[str] = frozenset({"$like", "$between"})

# Boolean composition operators.
_BOOL_OPS: frozenset[str] = frozenset({"$and", "$or"})

_ALL_OPS: frozenset[str] = frozenset(_SCALAR_OPS.keys()) | _LIST_OPS | _PATTERN_OPS | _BOOL_OPS


_SAFE_NAME_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def _resolve_field_path(field_name: str, type_id: int, registry: Any) -> str:
    """Resolve a user-facing ``field_name`` to a JSON path literal.

    Returns a SQL string fragment of the form ``$."<key>"`` suitable
    for embedding directly into a ``json_extract(payload_json, ...)``
    call. Preference order:

    1. Digit-string names are treated as pre-translated field ids
       (e.g. callers that already converted names to ids upstream).
    2. When a schema registry is supplied, the name is resolved to
       ``field_id`` via the registered ``NodeTypeDef`` — the
       canonical path for the 2026-04-14 SDK v0.3 design where
       payloads live on disk keyed by ``field_id``.
    3. When no registry is available (legacy / test harness fallback)
       the name is used verbatim **only if it is a safe identifier**
       (``[A-Za-z_][A-Za-z0-9_]*``). This preserves the pre-v0.3
       behaviour where payloads were stored name-keyed without
       opening a SQL-injection vector through operator arguments.
    """
    if isinstance(field_name, str) and field_name.isdigit():
        return f'$."{int(field_name)}"'

    if registry is not None and hasattr(registry, "get_node_type"):
        node_type = registry.get_node_type(type_id)
        if node_type is None:
            raise QueryFilterError(
                f"Cannot resolve field '{field_name}': unknown type_id={type_id}"
            )
        for f in node_type.fields:
            if f.name == field_name:
                return f'$."{int(f.field_id)}"'
        raise QueryFilterError(
            f"Unknown field '{field_name}' on type {node_type.name} "
            f"(type_id={type_id}): declared fields are "
            f"{[f.name for f in node_type.fields]}"
        )

    # No registry — legacy fallback for callers that still write
    # name-keyed payloads. Reject anything that isn't a safe
    # identifier so user-controlled strings cannot break out of the
    # ``$."..."`` literal.
    if not isinstance(field_name, str) or not _SAFE_NAME_RE.match(field_name):
        raise QueryFilterError(
            f"Invalid field name {field_name!r}: must match "
            "[A-Za-z_][A-Za-z0-9_]* when no schema registry is provided"
        )
    return f'$."{field_name}"'


def _json_extract_expr(path_literal: str) -> str:
    """Return the ``json_extract`` SQL expression for *path_literal*.

    ``path_literal`` must already be a trusted literal — either the
    string id produced by the registry or a safe identifier vetted by
    ``_resolve_field_path``. Values are always parameterised.
    """
    return f"json_extract(payload_json, '{path_literal}')"


def _compile_field_clause(
    field_name: str,
    spec: Any,
    type_id: int,
    registry: Any,
) -> tuple[str, list[Any]]:
    """Compile a ``{field: spec}`` clause into ``(sql_fragment, params)``.

    ``spec`` can be:
    - a scalar value (shorthand for ``$eq``),
    - ``None`` (shorthand for ``IS NULL``),
    - an operator dict ``{"$op": arg, ...}``.
    """
    path = _resolve_field_path(field_name, type_id, registry)
    expr = _json_extract_expr(path)

    # Shorthand: bare value → equality (or IS NULL for None).
    if not isinstance(spec, dict):
        if spec is None:
            return (f"{expr} IS NULL", [])
        return (f"{expr} = ?", [spec])

    # Operator dict — each key must be an allow-listed operator.
    if not spec:
        raise QueryFilterError(f"Empty operator dict on field '{field_name}'")

    frags: list[str] = []
    params: list[Any] = []
    for op, arg in spec.items():
        if not isinstance(op, str) or not op.startswith("$"):
            raise QueryFilterError(
                f"Operator must start with '$', got {op!r} on field '{field_name}'"
            )
        if op not in _ALL_OPS or op in _BOOL_OPS:
            raise QueryFilterError(
                f"Unknown operator {op!r} on field '{field_name}' "
                f"(valid: {sorted(_ALL_OPS - _BOOL_OPS)})"
            )

        if op in _SCALAR_OPS:
            sql_op = _SCALAR_OPS[op]
            if arg is None and op in ("$eq", "$ne"):
                # ``json_extract`` returns NULL for missing fields —
                # translate to ``IS NULL`` / ``IS NOT NULL`` so the
                # comparison behaves as the caller expects.
                frags.append(f"{expr} {'IS' if op == '$eq' else 'IS NOT'} NULL")
            else:
                frags.append(f"{expr} {sql_op} ?")
                params.append(arg)
            continue

        if op in _LIST_OPS:
            if not isinstance(arg, (list, tuple)) or not arg:
                raise QueryFilterError(f"{op!r} on field '{field_name}' requires a non-empty list")
            placeholders = ", ".join(["?"] * len(arg))
            sql_op = "IN" if op == "$in" else "NOT IN"
            frags.append(f"{expr} {sql_op} ({placeholders})")
            params.extend(arg)
            continue

        if op == "$like":
            if not isinstance(arg, str):
                raise QueryFilterError(f"$like on field '{field_name}' requires a string pattern")
            frags.append(f"{expr} LIKE ?")
            params.append(arg)
            continue

        if op == "$between":
            if not isinstance(arg, (list, tuple)) or len(arg) != 2:
                raise QueryFilterError(f"$between on field '{field_name}' requires [lo, hi]")
            lo, hi = arg
            frags.append(f"{expr} BETWEEN ? AND ?")
            params.append(lo)
            params.append(hi)
            continue

        # Unreachable — allow-list filter above.
        raise QueryFilterError(f"Unhandled operator {op!r}")

    if len(frags) == 1:
        return (frags[0], params)
    return (" AND ".join(frags), params)


def compile_query_filter(
    filter_dict: dict[str, Any] | None,
    type_id: int,
    registry: Any,
) -> tuple[str, list[Any]]:
    """Compile a filter dict into ``(sql_fragment, bound_params)``.

    Accepts:
    - ``{field: value}`` — equality shorthand.
    - ``{field: {"$op": arg}}`` — operator dict (see module docstring).
    - ``{"$and": [sub, ...]}`` / ``{"$or": [sub, ...]}`` — boolean
      composition at any level.

    An empty or ``None`` filter returns ``("", [])`` so the caller can
    skip the WHERE fragment entirely. Any mix of validation failures
    raises ``QueryFilterError`` with a descriptive message.
    """
    if not filter_dict:
        return ("", [])

    frags: list[str] = []
    params: list[Any] = []

    for key, value in filter_dict.items():
        if not isinstance(key, str):
            raise QueryFilterError(f"Filter keys must be strings, got {key!r}")

        if key in _BOOL_OPS:
            if not isinstance(value, list) or not value:
                raise QueryFilterError(f"{key!r} requires a non-empty list of sub-filters")
            sub_frags: list[str] = []
            for sub in value:
                if not isinstance(sub, dict):
                    raise QueryFilterError(
                        f"{key!r} sub-filters must be dicts, got {type(sub).__name__}"
                    )
                sub_sql, sub_params = compile_query_filter(sub, type_id, registry)
                if sub_sql:
                    sub_frags.append(f"({sub_sql})")
                    params.extend(sub_params)
            if not sub_frags:
                continue
            joiner = " AND " if key == "$and" else " OR "
            frags.append("(" + joiner.join(sub_frags) + ")")
            continue

        if key.startswith("$"):
            raise QueryFilterError(f"Top-level operator {key!r} is not allowed; use $and or $or")

        sub_sql, sub_params = _compile_field_clause(key, value, type_id, registry)
        if sub_sql:
            frags.append(sub_sql)
            params.extend(sub_params)

    if not frags:
        return ("", [])
    if len(frags) == 1:
        return (frags[0], params)
    return (" AND ".join(frags), params)
