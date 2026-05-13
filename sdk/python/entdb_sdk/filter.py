# SPDX-License-Identifier: MIT
"""Typed comparison filters for :meth:`DbClient.query` (issue #501).

The single-shape v0.3 API exposes range queries through a list of
:class:`Filter` values, AND-ed together server-side. Example::

    from entdb_sdk import Filter, FilterOp

    expired = await scope.query(
        WebAuthnChallengeType,
        where=[Filter("expires_at", FilterOp.LT, now_ms)],
        limit=500,
    )

Issue #501 explicitly scopes the operator set to the six comparison
operators (Eq/Ne/Lt/Le/Gt/Ge). OR predicates, sorting beyond the
existing ``order_by`` knob, and cursors are out of scope.

Index usage caveat: ``FilterOp.NE`` (not-equal) cannot use a B-tree
expression index — the server falls back to a full type scan. Prefer
a positive predicate on hot paths.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
from typing import Any


class FilterOp(str, Enum):
    """Comparison operator for a :class:`Filter`.

    Values are MongoDB-style ``$op`` strings so the SDK can lower a
    typed :class:`Filter` to the existing inlined-operator wire shape
    without a second translation table.
    """

    EQ = "$eq"
    NE = "$ne"
    LT = "$lt"
    LE = "$lte"
    GT = "$gt"
    GE = "$gte"


@dataclass(frozen=True)
class Filter:
    """One AND-ed predicate passed to :meth:`DbClient.query`.

    ``field`` is the proto field name (the gRPC boundary resolves it
    to a payload field id). ``op`` selects the comparison operator.
    ``value`` is bound as a SQLite parameter and accepts the same
    scalar types as the existing equality filter (str, int, float,
    bool, ``None``).
    """

    field: str
    op: FilterOp
    value: Any


def filters_to_filter_dict(filters: list[Filter] | None) -> dict[str, Any]:
    """Lower a typed filter list to the MongoDB-style nested dict the
    gRPC client emits.

    Multiple filters on the same field merge into a single
    ``{"$op": v, "$op2": v2}`` sub-dict so the server's inlined
    operator decoder fans them out into one ``FieldFilter`` per
    operator.
    """
    if not filters:
        return {}
    out: dict[str, Any] = {}
    for f in filters:
        existing = out.get(f.field)
        if f.op is FilterOp.EQ and existing is None:
            out[f.field] = f.value
            continue
        if existing is None:
            out[f.field] = {f.op.value: f.value}
            continue
        if isinstance(existing, dict):
            existing[f.op.value] = f.value
            continue
        # Existing plain equality value: rewrite as ``$eq`` so the new
        # operator nests alongside it.
        out[f.field] = {"$eq": existing, f.op.value: f.value}
    return out
