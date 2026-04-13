"""
Typed capability registry for the EntDB ACL v2 engine.

This implements the 2026-04-13 "typed capabilities" ACL decision:

- ``CoreCapability`` is a universal, built-in enum
  (``READ`` / ``COMMENT`` / ``EDIT`` / ``DELETE`` / ``ADMIN``) shipped
  with EntDB. Every node type gets these for free.

- Users declare per-type extension capabilities via the
  ``extension_capability_enum`` option on their proto ``entdb.node``
  annotation. Values are plain ``int32`` ids scoped to the owning
  ``type_id`` — so the same integer can mean
  ``CHANGE_STATUS`` on ``Task`` and ``RESOLVE`` on ``Issue`` without
  collision.

- The registry is built once at server startup from all
  ``NodeTypeDef`` entries in the schema registry. After that it is
  pure-Python and lock-free.

- Default ``op → capability`` mappings are baked in so every known
  op has a requirement even for types that do not declare any
  custom mappings. Type-specific mappings override defaults; the
  most specific (field + value + child_type) match wins.

- The built-in implication hierarchy is
  ``ADMIN > {EDIT, DELETE} > COMMENT > READ``. Per-type proto-declared
  implications are merged into the transitive closure at build time.

No strings ever appear on the hot authz path. All lookups are on
integer ``type_id`` / enum int values / cached op-name strings.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import IntEnum
from typing import Any


class CoreCapability(IntEnum):
    """Universal, built-in capabilities.

    The integer values MUST match
    ``entdb.v1.CoreCapability`` and ``entdb.CoreCapability`` on the
    wire. Tests pin this mapping.
    """

    UNSPECIFIED = 0
    READ = 1
    COMMENT = 2
    EDIT = 3
    DELETE = 4
    ADMIN = 5


# Default op → core-capability mappings applied when a type has no
# explicit override. ``CreateNode`` is intentionally absent — it is a
# tenant-level check, not a per-node capability check.
DEFAULT_OP_REQUIREMENTS: dict[str, CoreCapability] = {
    "GetNode": CoreCapability.READ,
    "GetNodes": CoreCapability.READ,
    "GetNodeByKey": CoreCapability.READ,
    "QueryNodes": CoreCapability.READ,
    "GetEdgesFrom": CoreCapability.READ,
    "GetEdgesTo": CoreCapability.READ,
    "GetConnectedNodes": CoreCapability.READ,
    "UpdateNode": CoreCapability.EDIT,
    "DeleteNode": CoreCapability.DELETE,
    "CreateEdge": CoreCapability.EDIT,
    "DeleteEdge": CoreCapability.EDIT,
    "ShareNode": CoreCapability.ADMIN,
    "RevokeAccess": CoreCapability.ADMIN,
    "TransferOwnership": CoreCapability.ADMIN,
}


# Built-in core implication hierarchy. ``ADMIN`` is the supremum.
CORE_IMPLICATIONS: dict[CoreCapability, set[CoreCapability]] = {
    CoreCapability.ADMIN: {
        CoreCapability.READ,
        CoreCapability.COMMENT,
        CoreCapability.EDIT,
        CoreCapability.DELETE,
    },
    CoreCapability.EDIT: {
        CoreCapability.READ,
        CoreCapability.COMMENT,
    },
    CoreCapability.COMMENT: {
        CoreCapability.READ,
    },
}


@dataclass
class CapabilityMapping:
    """One entry of a type's op → capability table.

    Exactly one of ``required_core`` / ``required_ext`` is non-None.
    ``field``, ``field_value`` and ``child_type`` narrow the match;
    ``None`` means "any".
    """

    op: str
    required_core: CoreCapability | None = None
    required_ext: int | None = None
    field: str | None = None
    field_value: str | None = None
    child_type: str | None = None

    def specificity(self) -> int:
        """Higher = more specific. Used to pick the best match."""
        return (
            (1 if self.field is not None else 0)
            + (1 if self.field_value is not None else 0)
            + (1 if self.child_type is not None else 0)
        )


@dataclass
class CapabilityImplication:
    """A single implication rule for a type.

    Either ``core`` or ``ext`` is set (exclusive). The implied set may
    mix core and extension capabilities.
    """

    core: CoreCapability | None = None
    ext: int | None = None
    implies_core: set[CoreCapability] = field(default_factory=set)
    implies_ext: set[int] = field(default_factory=set)


@dataclass
class TypeCapabilities:
    """Per-type capability metadata after build."""

    type_id: int
    extension_enum_name: str | None = None
    ext_cap_names: dict[int, str] = field(default_factory=dict)
    mappings: list[CapabilityMapping] = field(default_factory=list)
    implications: list[CapabilityImplication] = field(default_factory=list)
    # Precomputed transitive closures, including built-in core rules.
    implication_closure_core: dict[CoreCapability, set[CoreCapability]] = field(
        default_factory=dict
    )
    implication_closure_ext: dict[int, set[int]] = field(default_factory=dict)
    # Ext → core closure: holding ext implies these core caps.
    ext_implies_core: dict[int, set[CoreCapability]] = field(default_factory=dict)


class CapabilityRegistry:
    """Built once at server startup; read-only afterwards."""

    def __init__(self) -> None:
        self._types: dict[int, TypeCapabilities] = {}
        # Core-only type: used when no explicit type is registered.
        self._default_type = TypeCapabilities(type_id=0)
        _build_closures(self._default_type)

    # ------------------------------------------------------------------
    # Build-time API
    # ------------------------------------------------------------------

    def register_type(
        self,
        type_id: int,
        *,
        extension_enum_name: str | None = None,
        ext_cap_names: dict[int, str] | None = None,
        mappings: list[CapabilityMapping] | None = None,
        implications: list[CapabilityImplication] | None = None,
    ) -> TypeCapabilities:
        """Register a node type's capability metadata."""
        tc = TypeCapabilities(
            type_id=type_id,
            extension_enum_name=extension_enum_name,
            ext_cap_names=dict(ext_cap_names or {}),
            mappings=list(mappings or []),
            implications=list(implications or []),
        )
        _build_closures(tc)
        self._types[type_id] = tc
        return tc

    def register_from_node_opts(self, type_id: int, node_opts: Any) -> TypeCapabilities:
        """Build a ``TypeCapabilities`` from a proto ``NodeOpts`` value.

        ``node_opts`` is the resolved ``entdb.NodeOpts`` extension
        message for a single node type. Unknown / empty fields are
        ignored so this also works on legacy schemas.
        """
        ext_name: str | None = getattr(node_opts, "extension_capability_enum", None) or None
        mappings: list[CapabilityMapping] = []
        for m in getattr(node_opts, "capability_mappings", []) or []:
            req_core: CoreCapability | None = None
            req_ext: int | None = None
            raw_core = int(getattr(m, "required_core", 0) or 0)
            raw_ext = int(getattr(m, "required_ext", 0) or 0)
            if raw_core and raw_ext:
                raise ValueError(
                    f"CapabilityMapping on type_id={type_id} op={m.op!r} sets "
                    "both required_core and required_ext; exactly one is allowed"
                )
            if raw_core:
                req_core = CoreCapability(raw_core)
            elif raw_ext:
                req_ext = raw_ext
            mappings.append(
                CapabilityMapping(
                    op=m.op,
                    required_core=req_core,
                    required_ext=req_ext,
                    field=(getattr(m, "field", "") or None),
                    field_value=(getattr(m, "field_value", "") or None),
                    child_type=(getattr(m, "child_type", "") or None),
                )
            )
        implications: list[CapabilityImplication] = []
        for imp in getattr(node_opts, "capability_implications", []) or []:
            core = int(getattr(imp, "core", 0) or 0)
            ext = int(getattr(imp, "ext", 0) or 0)
            implications.append(
                CapabilityImplication(
                    core=CoreCapability(core) if core else None,
                    ext=ext if ext else None,
                    implies_core={CoreCapability(int(c)) for c in imp.implies_core if int(c)},
                    implies_ext={int(e) for e in imp.implies_ext if int(e)},
                )
            )
        return self.register_type(
            type_id,
            extension_enum_name=ext_name,
            mappings=mappings,
            implications=implications,
        )

    # ------------------------------------------------------------------
    # Query API
    # ------------------------------------------------------------------

    def get_type(self, type_id: int) -> TypeCapabilities:
        return self._types.get(type_id, self._default_type)

    def known_type_ids(self) -> list[int]:
        return sorted(self._types.keys())

    def required_for_op(
        self,
        type_id: int,
        op_name: str,
        *,
        field: str | None = None,
        field_value: str | None = None,
        child_type: str | None = None,
    ) -> tuple[CoreCapability | None, int | None]:
        """Return ``(core, ext)`` — exactly one non-None, or ``(None, None)``.

        Lookup order:

        1. Most-specific type-level mapping matching op +
           optional ``field`` / ``field_value`` / ``child_type``.
        2. Default op requirement baked into the server.
        3. ``(None, None)`` meaning "no requirement — allow".
        """
        tc = self.get_type(type_id)
        best: CapabilityMapping | None = None
        best_score = -1
        for m in tc.mappings:
            if m.op != op_name:
                continue
            if m.field is not None and m.field != field:
                continue
            if m.field_value is not None and m.field_value != field_value:
                continue
            if m.child_type is not None and m.child_type != child_type:
                continue
            score = m.specificity()
            if score > best_score:
                best = m
                best_score = score
        if best is not None:
            return (best.required_core, best.required_ext)
        default = DEFAULT_OP_REQUIREMENTS.get(op_name)
        if default is not None:
            return (default, None)
        return (None, None)

    def check_grant(
        self,
        grant_core_caps: list[int] | list[CoreCapability],
        grant_ext_cap_ids: list[int],
        required_core: CoreCapability | None,
        required_ext: int | None,
        type_id: int,
    ) -> bool:
        """Return ``True`` if ``(grant_core_caps, grant_ext_cap_ids)``
        satisfies the requirement, honouring implications.

        ``type_id`` selects which type's extension-capability space to
        use for ``required_ext`` and the type's ext → core closure.
        ``grant_ext_cap_ids`` are always interpreted in the scope of
        the SAME ``type_id`` — cross-type extension grants never
        satisfy a check (by construction of the registry).
        """
        if required_core is None and required_ext is None:
            return True

        tc = self.get_type(type_id)

        # Normalise grant cores to ``CoreCapability`` values.
        grant_core_set: set[CoreCapability] = set()
        for c in grant_core_caps:
            try:
                cc = CoreCapability(int(c))
            except ValueError:
                continue
            if cc is CoreCapability.UNSPECIFIED:
                continue
            grant_core_set.add(cc)

        grant_ext_set: set[int] = {int(e) for e in grant_ext_cap_ids if int(e)}

        # Expand grant exts via per-type ext closure AND pull in the
        # core caps that each ext implies (e.g. TASK_EXT_CAP_CHANGE_STATUS
        # may imply CORE_CAP_EDIT).
        expanded_ext: set[int] = set(grant_ext_set)
        promoted_core: set[CoreCapability] = set()
        for e in grant_ext_set:
            expanded_ext.update(tc.implication_closure_ext.get(e, set()))
            promoted_core.update(tc.ext_implies_core.get(e, set()))
        # Also walk exts we pulled in transitively.
        for e in list(expanded_ext):
            promoted_core.update(tc.ext_implies_core.get(e, set()))

        # Expand grant cores via the type's (merged with built-in)
        # core-closure, AFTER adding promoted cores from extensions so
        # ``ext=1 → EDIT → {READ, COMMENT}`` fires correctly.
        initial_core: set[CoreCapability] = set(grant_core_set) | promoted_core
        expanded_core: set[CoreCapability] = set(initial_core)
        for c in initial_core:
            expanded_core.update(tc.implication_closure_core.get(c, set()))

        if required_core is not None:
            return required_core in expanded_core
        if required_ext is not None:
            return required_ext in expanded_ext
        return False

    # ------------------------------------------------------------------
    # Backwards-compatibility helpers
    # ------------------------------------------------------------------

    @staticmethod
    def legacy_permission_to_core_caps(permission: str) -> list[CoreCapability]:
        """Map a legacy string permission to a ``CoreCapability`` list.

        Used by the wire compatibility shim on ``ShareNode`` and the
        ``_migrate_permissions_to_capabilities`` SQLite migration.
        """
        if not permission:
            return []
        p = permission.strip().lower()
        if p == "read":
            return [CoreCapability.READ]
        if p == "comment":
            return [CoreCapability.READ, CoreCapability.COMMENT]
        if p == "write":
            return [
                CoreCapability.READ,
                CoreCapability.COMMENT,
                CoreCapability.EDIT,
            ]
        if p == "delete":
            return [
                CoreCapability.READ,
                CoreCapability.COMMENT,
                CoreCapability.EDIT,
                CoreCapability.DELETE,
            ]
        if p in ("admin", "share"):
            return [CoreCapability.ADMIN]
        if p == "deny":
            # Deny grants carry no positive capabilities; the deny
            # row itself is recognised by the ACL query path.
            return []
        return []


def _build_closures(tc: TypeCapabilities) -> None:
    """Populate ``implication_closure_core`` / ``implication_closure_ext``
    / ``ext_implies_core`` for a freshly registered type.

    The closures are unions of:

    * the built-in ``CORE_IMPLICATIONS`` hierarchy
    * the type's own ``implications`` list

    Computed with a simple fixed-point iteration — implication DAGs
    are tiny (tens of nodes at most) so this is trivially fast.
    """
    core_graph: dict[CoreCapability, set[CoreCapability]] = {
        k: set(v) for k, v in CORE_IMPLICATIONS.items()
    }
    ext_graph: dict[int, set[int]] = {}
    ext_to_core: dict[int, set[CoreCapability]] = {}

    for imp in tc.implications:
        if imp.core is not None:
            core_graph.setdefault(imp.core, set()).update(imp.implies_core)
            if imp.implies_ext:
                # Rare, but allowed: CORE_CAP_ADMIN implies some
                # type-specific exts. We don't model it in the ext
                # graph directly — instead, anyone with ``imp.core``
                # is already matched before exts are consulted.
                pass
        elif imp.ext is not None:
            ext_graph.setdefault(imp.ext, set()).update(imp.implies_ext)
            if imp.implies_core:
                ext_to_core.setdefault(imp.ext, set()).update(imp.implies_core)

    # Fixed-point transitive closure over the core graph.
    changed = True
    while changed:
        changed = False
        for src, targets in list(core_graph.items()):
            new_targets = set(targets)
            for t in list(targets):
                new_targets.update(core_graph.get(t, set()))
            if new_targets != targets:
                core_graph[src] = new_targets
                changed = True

    # Fixed-point transitive closure over the ext graph.
    changed = True
    while changed:
        changed = False
        for src, targets in list(ext_graph.items()):
            new_targets = set(targets)
            for t in list(targets):
                new_targets.update(ext_graph.get(t, set()))
            if new_targets != targets:
                ext_graph[src] = new_targets
                changed = True

    tc.implication_closure_core = core_graph
    tc.implication_closure_ext = ext_graph
    tc.ext_implies_core = ext_to_core


def build_registry_from_schema(schema_registry: Any) -> CapabilityRegistry:
    """Build a ``CapabilityRegistry`` from an ``entdb_server.schema.SchemaRegistry``.

    Walks every registered ``NodeTypeDef`` and reads a ``capabilities``
    attribute when present. Node types that do not declare capability
    metadata fall back to the default (core-only) behaviour.

    The schema registry owns the type system; this function only
    bridges the two worlds so that the hot authz path can work off
    integer ``type_id`` → ``TypeCapabilities`` lookups without touching
    the proto/dataclass layer.
    """
    reg = CapabilityRegistry()
    node_types = getattr(schema_registry, "node_types", None)
    if node_types is None:
        return reg
    for node_type in node_types():
        caps_meta = getattr(node_type, "capabilities", None)
        if caps_meta is None:
            # Register a plain type so required_for_op still hits the
            # defaults path (rather than the default-type wildcard).
            reg.register_type(node_type.type_id)
            continue
        reg.register_type(
            node_type.type_id,
            extension_enum_name=getattr(caps_meta, "extension_enum_name", None),
            ext_cap_names=getattr(caps_meta, "ext_cap_names", {}) or {},
            mappings=list(getattr(caps_meta, "mappings", []) or []),
            implications=list(getattr(caps_meta, "implications", []) or []),
        )
    return reg


__all__ = [
    "CORE_IMPLICATIONS",
    "DEFAULT_OP_REQUIREMENTS",
    "CapabilityImplication",
    "CapabilityMapping",
    "CapabilityRegistry",
    "CoreCapability",
    "TypeCapabilities",
    "build_registry_from_schema",
]
