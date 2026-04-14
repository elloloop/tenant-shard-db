"""Typed unique-key tokens and storage descriptors for SDK v0.3.

These primitives are part of the 2026-04-14 single-shape SDK API
decision in ``docs/decisions/sdk_api.md``. The user-facing contract is:

- Unique fields are declared at the proto field site via
  ``(entdb.field).unique = true``.
- The ``protoc-gen-entdb-keys`` codegen plugin emits one
  :class:`UniqueKey` token per unique field, in a sidecar file
  alongside the standard protoc output.
- :class:`UniqueKey` has no public constructor: tokens are only
  produced by codegen. This is the mechanism that prevents
  stringly-typed lookups from creeping back into application code.
- Storage routing (tenant / mailbox / public) is selected by passing
  one of :class:`Tenant`, :class:`Mailbox`, or :class:`Public` as the
  ``storage=`` keyword to ``Plan.create``. There is exactly one way
  to create a node — pass the proto message and an optional storage
  descriptor.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Generic, Literal, TypeVar

T = TypeVar("T")


@dataclass(frozen=True)
class UniqueKey(Generic[T]):
    """A typed reference to a unique field on a node type.

    Instances of this class are produced exclusively by the
    ``protoc-gen-entdb-keys`` codegen plugin. Do not construct them
    by hand — there is no migration path back from a stringly-typed
    lookup. The generic parameter ``T`` is the field's value type, so
    passing the wrong scalar to :meth:`Scope.get_by_key` is a
    type-checker error, not a runtime one.

    Attributes:
        type_id: Numeric type id of the owning node type (from the
            proto ``(entdb.node)`` option).
        field_id: Numeric field id of the unique field (the proto
            field number).
        name: Human-readable field name. Used only for error messages
            and debugging — the wire path is keyed by ``field_id``.
    """

    type_id: int
    field_id: int
    name: str


@dataclass(frozen=True)
class Tenant:
    """Default storage descriptor.

    Nodes created with this descriptor live in the tenant's
    ``tenant.db`` SQLite file and are subject to the standard ACL
    pipeline. Equivalent to passing no ``storage=`` argument at all.
    """

    kind: Literal["tenant"] = "tenant"


@dataclass(frozen=True)
class Mailbox:
    """Per-user mailbox storage descriptor.

    Nodes created with this descriptor live in
    ``user_<user_id>.db`` and are private to exactly one user. Once
    created, a node cannot be moved out of mailbox storage — use
    :class:`Tenant` if the node might ever need to be shared.
    """

    user_id: str
    kind: Literal["mailbox"] = "mailbox"


@dataclass(frozen=True)
class Public:
    """Cross-tenant public storage descriptor.

    Nodes created with this descriptor live in the singleton
    ``public.db`` and are readable by any tenant. Use this for data
    that is universally visible (public catalogues, blog posts,
    etc.). Storage mode is immutable: a public node cannot be moved
    into a tenant or mailbox file later.
    """

    kind: Literal["public"] = "public"


# Discriminated union of storage descriptors. Used as the type of
# the ``storage=`` keyword on ``Plan.create``. The SDK targets
# Python 3.10+ (``pyproject.toml`` ``requires-python = ">=3.10"``),
# so PEP 604 ``X | Y`` is fine here.
Storage = Tenant | Mailbox | Public


__all__ = ["UniqueKey", "Tenant", "Mailbox", "Public", "Storage"]
