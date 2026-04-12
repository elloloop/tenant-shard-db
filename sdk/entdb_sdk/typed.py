"""Strongly-typed primitives for the EntDB SDK.

All user-facing types live here: ``Actor``, ``Permission``,
``ACLEntry``, ``TypedNode``, ``TypedEdge``, and reference types.

With proto-first workflow, standard ``protoc`` output replaces custom
codegen — proto messages ARE the typed classes.  These primitives
complement the proto types with SDK-specific concepts (permissions,
ACL grants, actor identifiers).
"""

from __future__ import annotations

import dataclasses
import enum
from typing import Any, ClassVar, Self

# ── Actor ─────────────────────────────────────────────────────────────


class Actor:
    """Typed actor identifier — replaces raw ``"user:bob"`` strings.

    Factory methods enforce the ``kind:id`` format and prevent typos.

    Example::

        bob = Actor.user("bob")
        admins = Actor.group("admins")
        api = Actor.service("ingestion-pipeline")
    """

    __slots__ = ("_value",)

    def __init__(self, value: str) -> None:
        if ":" not in value:
            raise ValueError(f"Actor must be 'kind:id' (e.g. 'user:bob'), got {value!r}")
        self._value = value

    @classmethod
    def user(cls, user_id: str) -> Actor:
        return cls(f"user:{user_id}")

    @classmethod
    def group(cls, group_id: str) -> Actor:
        return cls(f"group:{group_id}")

    @classmethod
    def service(cls, service_id: str) -> Actor:
        return cls(f"service:{service_id}")

    @property
    def kind(self) -> str:
        return self._value.split(":", 1)[0]

    @property
    def id(self) -> str:
        return self._value.split(":", 1)[1]

    def __str__(self) -> str:
        return self._value

    def __repr__(self) -> str:
        return f"Actor({self._value!r})"

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Actor):
            return self._value == other._value
        if isinstance(other, str):
            return self._value == other
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self._value)


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
