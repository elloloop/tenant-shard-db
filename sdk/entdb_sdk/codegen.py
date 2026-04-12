"""Proto-to-EntDB schema code generator.

Reads a compiled .proto FileDescriptorSet and generates Python/Go
EntDB type definitions from messages annotated with entdb options.

Usage:
    from entdb_sdk.codegen import generate_from_proto
    code = generate_from_proto("schema.proto", lang="python")
"""

from __future__ import annotations

import hashlib
import json
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any


@dataclass
class FieldInfo:
    field_id: int
    name: str
    kind: str
    required: bool = False
    searchable: bool = False
    indexed: bool = False
    pii: bool = False
    phi: bool = False
    pii_false: bool = False
    enum_values: tuple[str, ...] | None = None
    ref_type_id: int | None = None
    deprecated: bool = False
    description: str = ""
    default_value: str | None = None


# DataPolicy enum values from entdb_options.proto
_DATA_POLICY_NAMES = {
    0: "PERSONAL",
    1: "BUSINESS",
    2: "FINANCIAL",
    3: "AUDIT",
    4: "EPHEMERAL",
    5: "HEALTHCARE",
}

# SubjectExitPolicy enum values from entdb_options.proto
_SUBJECT_EXIT_NAMES = {
    0: "BOTH",
    1: "FROM",
    2: "TO",
}


@dataclass
class NodeInfo:
    type_id: int
    name: str
    fields: list[FieldInfo]
    acl_public: bool = False
    acl_tenant_visible: bool = True
    acl_inherit: bool = True
    is_private: bool = False
    data_policy: str = "PERSONAL"
    subject_field: str = ""
    retention_days: int = 0
    legal_basis: str = ""
    deprecated: bool = False
    description: str = ""


@dataclass
class EdgeInfo:
    edge_id: int
    name: str
    from_type: int
    to_type: int
    props: list[FieldInfo]
    propagate_share: bool = False
    unique_per_from: bool = False
    data_policy: str = "PERSONAL"
    on_subject_exit: str = "BOTH"
    retention_days: int = 0
    legal_basis: str = ""
    deprecated: bool = False
    description: str = ""


# Proto type number → EntDB FieldKind
_PROTO_TYPE_MAP = {
    1: "float",  # TYPE_DOUBLE
    2: "float",  # TYPE_FLOAT
    3: "int",  # TYPE_INT64
    4: "int",  # TYPE_UINT64
    5: "int",  # TYPE_INT32
    8: "bool",  # TYPE_BOOL
    9: "str",  # TYPE_STRING
    12: "bytes",  # TYPE_BYTES
    13: "int",  # TYPE_UINT32
    17: "int",  # TYPE_SINT32
    18: "int",  # TYPE_SINT64
}


def _resolve_kind(proto_type: int, label: int, kind_override: str) -> str:
    """Resolve the EntDB FieldKind from proto type + optional override."""
    if kind_override:
        return kind_override

    # Repeated fields → list types
    if label == 3:  # LABEL_REPEATED
        base = _PROTO_TYPE_MAP.get(proto_type, "str")
        if base == "str":
            return "list_str"
        if base == "int":
            return "list_int"
        return "json"  # fallback for repeated complex types

    return _PROTO_TYPE_MAP.get(proto_type, "str")


def parse_proto(
    proto_path: str, include_dirs: list[str] | None = None
) -> tuple[list[NodeInfo], list[EdgeInfo]]:
    """Parse a .proto file and extract EntDB schema info.

    Uses protoc to compile the .proto to a FileDescriptorSet, then
    reads the descriptors to extract entdb options.

    Args:
        proto_path: Path to the .proto file
        include_dirs: Additional proto include directories

    Returns:
        Tuple of (node_types, edge_types)
    """
    from google.protobuf import descriptor_pb2

    from ._generated import entdb_options_pb2  # noqa: F401 — registers extensions

    proto_path = Path(proto_path).resolve()
    if not proto_path.exists():
        raise FileNotFoundError(f"Proto file not found: {proto_path}")

    # Find the entdb options proto
    sdk_proto_dir = Path(__file__).parent / "proto"

    # Build include paths (all absolute)
    includes = [str(proto_path.parent), str(sdk_proto_dir.resolve())]
    if include_dirs:
        includes.extend(str(Path(d).resolve()) for d in include_dirs)

    # Compile proto to descriptor set
    with tempfile.NamedTemporaryFile(suffix=".pb", delete=False) as tmp:
        desc_path = tmp.name

    cmd = [
        sys.executable,
        "-m",
        "grpc_tools.protoc",
        f"--descriptor_set_out={desc_path}",
        "--include_imports",
    ]
    for inc in includes:
        cmd.append(f"-I{inc}")
    cmd.append(str(proto_path))

    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        raise RuntimeError(f"protoc failed: {result.stderr}")

    # Parse the descriptor set
    with open(desc_path, "rb") as f:
        desc_set = descriptor_pb2.FileDescriptorSet()
        desc_set.ParseFromString(f.read())

    Path(desc_path).unlink(missing_ok=True)

    nodes: list[NodeInfo] = []
    edges: list[EdgeInfo] = []

    for file_desc in desc_set.file:
        for msg in file_desc.message_type:
            node_info = _extract_node(msg)
            if node_info:
                nodes.append(node_info)
                continue

            edge_info = _extract_edge(msg)
            if edge_info:
                edges.append(edge_info)

    return nodes, edges


def _reparse_message_options(options) -> Any:
    """Re-parse MessageOptions through the generated options module.

    ``FileDescriptorSet`` stores custom extensions as unknown fields on
    the options message because the descriptor's pool has no knowledge
    of our extensions. By serializing + re-parsing into a freshly imported
    ``MessageOptions`` (which has the entdb extensions registered via the
    ``entdb_options_pb2`` import in ``parse_proto``), the extensions
    become accessible via ``options.Extensions[entdb_options_pb2.node]``.
    """
    from google.protobuf import descriptor_pb2

    raw = options.SerializeToString() if options is not None else b""
    reparsed = descriptor_pb2.MessageOptions()
    if raw:
        reparsed.ParseFromString(raw)
    return reparsed


def _reparse_field_options(options) -> Any:
    """Re-parse FieldOptions through the generated options module.

    See ``_reparse_message_options`` for rationale.
    """
    from google.protobuf import descriptor_pb2

    raw = options.SerializeToString() if options is not None else b""
    reparsed = descriptor_pb2.FieldOptions()
    if raw:
        reparsed.ParseFromString(raw)
    return reparsed


def _extract_node(msg) -> NodeInfo | None:
    """Extract NodeInfo from a proto message descriptor if it has entdb.node option."""
    from ._generated import entdb_options_pb2

    opts = _reparse_message_options(msg.options)
    if not opts.HasExtension(entdb_options_pb2.node):
        return None

    node_opts = opts.Extensions[entdb_options_pb2.node]
    if node_opts.type_id == 0:
        return None

    fields = [_extract_field(fd) for fd in msg.field]
    data_policy = _DATA_POLICY_NAMES.get(int(node_opts.data_policy), "PERSONAL")

    return NodeInfo(
        type_id=node_opts.type_id,
        name=msg.name,
        fields=fields,
        acl_public=node_opts.public,
        acl_tenant_visible=node_opts.tenant_visible,
        acl_inherit=node_opts.inherit,
        is_private=node_opts.private,
        data_policy=data_policy,
        subject_field=node_opts.subject_field,
        retention_days=node_opts.retention_days,
        legal_basis=node_opts.legal_basis,
        deprecated=node_opts.deprecated,
        description=node_opts.description,
    )


def _extract_edge(msg) -> EdgeInfo | None:
    """Extract EdgeInfo from a proto message if it has entdb.edge option."""
    from ._generated import entdb_options_pb2

    opts = _reparse_message_options(msg.options)
    if not opts.HasExtension(entdb_options_pb2.edge):
        return None

    edge_opts = opts.Extensions[entdb_options_pb2.edge]
    if edge_opts.edge_id == 0:
        return None

    props = [_extract_field(fd) for fd in msg.field]
    data_policy = _DATA_POLICY_NAMES.get(int(edge_opts.data_policy), "PERSONAL")
    on_subject_exit = _SUBJECT_EXIT_NAMES.get(int(edge_opts.on_subject_exit), "BOTH")

    return EdgeInfo(
        edge_id=edge_opts.edge_id,
        name=edge_opts.name or msg.name,
        from_type=0,
        to_type=0,
        props=props,
        propagate_share=edge_opts.propagate_share,
        unique_per_from=edge_opts.unique_per_from,
        data_policy=data_policy,
        on_subject_exit=on_subject_exit,
        retention_days=edge_opts.retention_days,
        legal_basis=edge_opts.legal_basis,
        deprecated=edge_opts.deprecated,
        description=edge_opts.description,
    )


def _extract_field(fd) -> FieldInfo:
    """Extract FieldInfo from a proto field descriptor."""
    from ._generated import entdb_options_pb2

    opts = _reparse_field_options(fd.options) if fd.options else None
    fext = None
    if opts is not None and opts.HasExtension(entdb_options_pb2.field):
        fext = opts.Extensions[entdb_options_pb2.field]

    kind_override = fext.kind if fext is not None else ""
    enum_str = fext.enum_values if fext is not None else ""
    enum_values = tuple(v.strip() for v in enum_str.split(",") if v.strip()) if enum_str else None

    kind = _resolve_kind(fd.type, fd.label, kind_override)
    if enum_values and kind == "str":
        kind = "enum"

    return FieldInfo(
        field_id=fd.number,
        name=fd.name,
        kind=kind,
        required=bool(fext.required) if fext is not None else False,
        searchable=bool(fext.searchable) if fext is not None else False,
        indexed=bool(fext.indexed) if fext is not None else False,
        pii=bool(fext.pii) if fext is not None else False,
        phi=bool(fext.phi) if fext is not None else False,
        pii_false=bool(fext.pii_false) if fext is not None else False,
        enum_values=enum_values,
        ref_type_id=(fext.ref_type_id or None) if fext is not None else None,
        deprecated=bool(fext.deprecated) if fext is not None else False,
        description=fext.description if fext is not None else "",
        default_value=(fext.default_value or None) if fext is not None else None,
    )


# ── Schema Fingerprint ────────────────────────────────────────────────


def _field_to_canonical(f: FieldInfo) -> dict[str, Any]:
    """Canonical dict representation of a FieldInfo for fingerprinting."""
    return {
        "field_id": f.field_id,
        "name": f.name,
        "kind": f.kind,
        "required": f.required,
        "searchable": f.searchable,
        "indexed": f.indexed,
        "pii": f.pii,
        "phi": f.phi,
        "pii_false": f.pii_false,
        "enum_values": list(f.enum_values) if f.enum_values else None,
        "ref_type_id": f.ref_type_id,
        "deprecated": f.deprecated,
        "description": f.description,
        "default_value": f.default_value,
    }


def compute_schema_fingerprint(nodes: list[NodeInfo], edges: list[EdgeInfo]) -> str:
    """Compute a deterministic sha256 fingerprint of a parsed schema.

    The fingerprint is a sha256 over a canonical JSON representation of
    all NodeTypeDef + EdgeTypeDef + FieldDef tuples, sorted by type_id
    and field_id. The returned value is prefixed with ``sha256:`` and
    matches the format used by the server-side schema registry.
    """
    canonical = {
        "node_types": [
            {
                "type_id": n.type_id,
                "name": n.name,
                "acl_public": n.acl_public,
                "acl_tenant_visible": n.acl_tenant_visible,
                "acl_inherit": n.acl_inherit,
                "is_private": n.is_private,
                "data_policy": n.data_policy,
                "subject_field": n.subject_field,
                "retention_days": n.retention_days,
                "legal_basis": n.legal_basis,
                "deprecated": n.deprecated,
                "description": n.description,
                "fields": [
                    _field_to_canonical(f) for f in sorted(n.fields, key=lambda f: f.field_id)
                ],
            }
            for n in sorted(nodes, key=lambda n: n.type_id)
        ],
        "edge_types": [
            {
                "edge_id": e.edge_id,
                "name": e.name,
                "from_type": e.from_type,
                "to_type": e.to_type,
                "propagate_share": e.propagate_share,
                "unique_per_from": e.unique_per_from,
                "data_policy": e.data_policy,
                "on_subject_exit": e.on_subject_exit,
                "retention_days": e.retention_days,
                "legal_basis": e.legal_basis,
                "deprecated": e.deprecated,
                "description": e.description,
                "props": [
                    _field_to_canonical(f) for f in sorted(e.props, key=lambda f: f.field_id)
                ],
            }
            for e in sorted(edges, key=lambda e: e.edge_id)
        ],
    }
    blob = json.dumps(canonical, sort_keys=True, separators=(",", ":"))
    digest = hashlib.sha256(blob.encode("utf-8")).hexdigest()
    return f"sha256:{digest}"


# ── Code Generators ───────────────────────────────────────────────────


_FIELD_KIND_TO_PYTHON_TYPE: dict[str, str] = {
    "str": "str",
    "int": "int",
    "float": "float",
    "bool": "bool",
    "timestamp": "int",
    "json": "dict",
    "bytes": "bytes",
    "enum": "str",
    "ref": "dict",
    "list_str": "list[str]",
    "list_int": "list[int]",
    "list_ref": "list[dict]",
}


def register_proto_schema(module: Any) -> tuple[int, int]:
    """Auto-register all proto messages with entdb options from a generated module.

    Walks the module's proto message types and registers any that carry
    ``entdb.node`` or ``entdb.edge`` options with the global schema registry.
    This replaces the old ``generate_python`` codegen step — standard
    ``protoc --python_out`` is now sufficient.

    Args:
        module: A generated ``_pb2`` module (e.g. ``my_schema_pb2``).

    Returns:
        Tuple of (node_types_registered, edge_types_registered).

    Example::

        import my_schema_pb2
        from entdb_sdk.codegen import register_proto_schema

        register_proto_schema(my_schema_pb2)
        # Now Task, User, etc. are registered and can be used with Plan/DbClient.
    """
    from .registry import get_registry
    from .schema import EdgeTypeDef, NodeTypeDef

    registry = get_registry()
    nodes_registered = 0
    edges_registered = 0

    file_descriptor = getattr(module, "DESCRIPTOR", None)
    if file_descriptor is None:
        return (0, 0)

    for msg_desc in file_descriptor.message_types_by_name.values():
        opts = _reparse_message_options(msg_desc.GetOptions())
        if opts is None:
            continue

        from ._generated import entdb_options_pb2

        if opts.HasExtension(entdb_options_pb2.node):
            node_type = NodeTypeDef.from_descriptor(msg_desc)
            registry.register_node_type(node_type)
            nodes_registered += 1
        elif opts.HasExtension(entdb_options_pb2.edge):
            edge_type = EdgeTypeDef.from_descriptor(msg_desc)
            registry.register_edge_type(edge_type)
            edges_registered += 1

    return (nodes_registered, edges_registered)
