"""Strongly-typed primitives for the EntDB SDK.

All user-facing types live here: ``TypedNode``, ``TypedEdge``,
``Permission``, ``ACLEntry``, and ``NodeRef`` / ``EdgeRef``.
Generated code (``entdb codegen``) subclasses ``TypedNode`` and
``TypedEdge`` with concrete fields.
"""

from __future__ import annotations

import dataclasses
import enum
from typing import Any, ClassVar, Self

# ── Enums ────────────────────────────────────────────────────────────


class Permission(enum.Enum):
    """ACL permission levels."""

    READ = "read"
    WRITE = "write"
    ADMIN = "admin"


# ── ACL ──────────────────────────────────────────────────────────────


@dataclasses.dataclass(frozen=True, slots=True)
class ACLEntry:
    """A single ACL grant on a node."""

    grantee: str
    permission: Permission
    actor_type: str = "user"
    expires_at: int | None = None


# ── Typed Node ───────────────────────────────────────────────────────


@dataclasses.dataclass
class TypedNode:
    """Base class for generated typed node wrappers.

    Subclasses declare their fields as regular dataclass fields and set
    ``_type_id`` and ``_type_name`` class variables.

    Example (generated)::

        @dataclass
        class Task(TypedNode):
            title: str
            status: str
            assignee_id: str | None = None

            _type_id: ClassVar[int] = 101
            _type_name: ClassVar[str] = \"Task\"
    """

    _type_id: ClassVar[int] = 0
    _type_name: ClassVar[str] = ""

    def to_payload(self) -> dict[str, Any]:
        """Convert dataclass fields to a payload dictionary.

        Only includes fields that are not ``None``.
        """
        result: dict[str, Any] = {}
        for f in dataclasses.fields(self):
            value = getattr(self, f.name)
            if value is not None:
                result[f.name] = value
        return result

    @classmethod
    def from_payload(cls, payload: dict[str, Any]) -> Self:
        """Construct a typed instance from a payload dictionary.

        Unknown keys are silently ignored for forward compatibility.
        """
        known = {f.name for f in dataclasses.fields(cls)}
        filtered = {k: v for k, v in payload.items() if k in known}
        return cls(**filtered)


# ── Typed Edge ───────────────────────────────────────────────────────


@dataclasses.dataclass
class TypedEdge:
    """Base class for generated typed edge wrappers.

    Example (generated)::

        @dataclass
        class AssignedTo(TypedEdge):
            priority: int | None = None

            _edge_type_id: ClassVar[int] = 201
            _edge_type_name: ClassVar[str] = \"AssignedTo\"
    """

    _edge_type_id: ClassVar[int] = 0
    _edge_type_name: ClassVar[str] = ""

    def to_props(self) -> dict[str, Any]:
        """Convert edge fields to a properties dictionary."""
        result: dict[str, Any] = {}
        for f in dataclasses.fields(self):
            value = getattr(self, f.name)
            if value is not None:
                result[f.name] = value
        return result

    @classmethod
    def from_props(cls, props: dict[str, Any]) -> Self:
        """Construct a typed edge from a properties dictionary."""
        known = {f.name for f in dataclasses.fields(cls)}
        filtered = {k: v for k, v in props.items() if k in known}
        return cls(**filtered)


# ── Node / Edge refs for operation building ──────────────────────────


@dataclasses.dataclass(frozen=True, slots=True)
class NodeRef:
    """Reference to a node by type and ID, or by plan alias."""

    type_id: int
    id: str

    def to_dict(self) -> dict[str, Any]:
        return {"type_id": self.type_id, "id": self.id}


@dataclasses.dataclass(frozen=True, slots=True)
class AliasRef:
    """Reference to a node created earlier in the same plan."""

    alias: str

    def to_dict(self) -> dict[str, str]:
        return {"ref": f"${self.alias}.id"}
