"""Proto-to-EntDB schema code generator.

Reads a compiled .proto FileDescriptorSet and generates Python/Go
EntDB type definitions from messages annotated with entdb options.

Usage:
    from entdb_sdk.codegen import generate_from_proto
    code = generate_from_proto("schema.proto", lang="python")
"""

from __future__ import annotations

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

    # Custom option field numbers (from entdb_options.proto)
    NODE_OPT = 50100
    EDGE_OPT = 50101
    FIELD_OPT = 50102

    for file_desc in desc_set.file:
        for msg in file_desc.message_type:
            node_info = _extract_node(msg, NODE_OPT, FIELD_OPT)
            if node_info:
                nodes.append(node_info)
                continue

            edge_info = _extract_edge(msg, EDGE_OPT, FIELD_OPT)
            if edge_info:
                edges.append(edge_info)

    return nodes, edges


def _decode_varint(data: bytes, pos: int) -> tuple[int, int]:
    """Decode a varint from bytes at the given position."""
    result = 0
    shift = 0
    while pos < len(data):
        b = data[pos]
        result |= (b & 0x7F) << shift
        pos += 1
        if (b & 0x80) == 0:
            return result, pos
        shift += 7
    raise ValueError("Truncated varint")


def _encode_varint(value: int) -> bytes:
    """Encode an integer as a varint."""
    buf = bytearray()
    while value > 0x7F:
        buf.append((value & 0x7F) | 0x80)
        value >>= 7
    buf.append(value & 0x7F)
    return bytes(buf)


def _get_option_value(options, field_number: int) -> bytes | None:
    """Extract a custom option value from proto options by field number.

    Custom extensions are stored as unknown fields in the serialized options.
    We parse the raw wire format to find the field by number.
    """
    if not options or not options.ByteSize():
        return None

    raw = options.SerializeToString()
    if not raw:
        return None

    pos = 0
    while pos < len(raw):
        tag, pos = _decode_varint(raw, pos)
        fn = tag >> 3
        wt = tag & 0x7

        if wt == 0:  # VARINT
            val, next_pos = _decode_varint(raw, pos)
            if fn == field_number:
                return _encode_varint(val)
            pos = next_pos
        elif wt == 2:  # LENGTH_DELIMITED
            length, pos = _decode_varint(raw, pos)
            data = raw[pos : pos + length]
            if fn == field_number:
                return data
            pos += length
        elif wt == 5:  # FIXED32
            if fn == field_number:
                return raw[pos : pos + 4]
            pos += 4
        elif wt == 1:  # FIXED64
            if fn == field_number:
                return raw[pos : pos + 8]
            pos += 8
        else:
            break  # Unknown wire type

    return None


def _extract_node(msg, node_opt_num: int, field_opt_num: int) -> NodeInfo | None:
    """Extract NodeInfo from a proto message descriptor if it has entdb.node option."""
    raw_opts = _get_option_value(msg.options, node_opt_num)
    if raw_opts is None:
        return None

    node_opts = _parse_node_opts(raw_opts)
    if node_opts is None or node_opts.get("type_id", 0) == 0:
        return None

    fields = []
    for fd in msg.field:
        fi = _extract_field(fd, field_opt_num)
        fields.append(fi)

    dp_num = node_opts.get("data_policy", 0)
    data_policy = _DATA_POLICY_NAMES.get(dp_num, "PERSONAL")

    return NodeInfo(
        type_id=node_opts["type_id"],
        name=msg.name,
        fields=fields,
        acl_public=node_opts.get("public", False),
        acl_tenant_visible=node_opts.get("tenant_visible", False),
        acl_inherit=node_opts.get("inherit", False),
        is_private=node_opts.get("private", False),
        data_policy=data_policy,
        subject_field=node_opts.get("subject_field", ""),
        retention_days=node_opts.get("retention_days", 0),
        legal_basis=node_opts.get("legal_basis", ""),
        deprecated=node_opts.get("deprecated", False),
        description=node_opts.get("description", ""),
    )


def _extract_edge(msg, edge_opt_num: int, field_opt_num: int) -> EdgeInfo | None:
    """Extract EdgeInfo from a proto message if it has entdb.edge option."""
    raw_opts = _get_option_value(msg.options, edge_opt_num)
    if raw_opts is None:
        return None

    edge_opts = _parse_edge_opts(raw_opts)
    if edge_opts is None or edge_opts.get("edge_id", 0) == 0:
        return None

    props = []
    for fd in msg.field:
        fi = _extract_field(fd, field_opt_num)
        props.append(fi)

    dp_num = edge_opts.get("data_policy", 0)
    data_policy = _DATA_POLICY_NAMES.get(dp_num, "PERSONAL")

    se_num = edge_opts.get("on_subject_exit", 0)
    on_subject_exit = _SUBJECT_EXIT_NAMES.get(se_num, "BOTH")

    return EdgeInfo(
        edge_id=edge_opts["edge_id"],
        name=edge_opts.get("name", msg.name),
        from_type=0,
        to_type=0,
        props=props,
        propagate_share=edge_opts.get("propagate_share", False),
        unique_per_from=edge_opts.get("unique_per_from", False),
        data_policy=data_policy,
        on_subject_exit=on_subject_exit,
        retention_days=edge_opts.get("retention_days", 0),
        legal_basis=edge_opts.get("legal_basis", ""),
        deprecated=edge_opts.get("deprecated", False),
        description=edge_opts.get("description", ""),
    )


def _extract_field(fd, field_opt_num: int) -> FieldInfo:
    """Extract FieldInfo from a proto field descriptor."""
    raw_opts = _get_option_value(fd.options, field_opt_num) if fd.options else None

    opts: dict[str, Any] = {}
    if raw_opts is not None:
        opts = _parse_field_opts(raw_opts)

    kind_override = opts.get("kind", "")
    enum_str = opts.get("enum_values", "")
    enum_values = tuple(v.strip() for v in enum_str.split(",") if v.strip()) if enum_str else None

    kind = _resolve_kind(fd.type, fd.label, kind_override)
    if enum_values and kind == "str":
        kind = "enum"

    return FieldInfo(
        field_id=fd.number,
        name=fd.name,
        kind=kind,
        required=opts.get("required", False),
        searchable=opts.get("searchable", False),
        indexed=opts.get("indexed", False),
        pii=opts.get("pii", False),
        phi=opts.get("phi", False),
        pii_false=opts.get("pii_false", False),
        enum_values=enum_values,
        ref_type_id=opts.get("ref_type_id") or None,
        deprecated=opts.get("deprecated", False),
        description=opts.get("description", ""),
        default_value=opts.get("default_value") or None,
    )


def _parse_node_opts(raw: bytes) -> dict[str, Any]:
    """Parse raw bytes into NodeOpts dict using protobuf wire format.

    Field numbers (from entdb_options.proto v2):
      1: type_id (varint)
      2: public (bool/varint)
      3: tenant_visible (bool/varint)
      4: inherit (bool/varint)
      5: private (bool/varint)
      6: data_policy (enum/varint)
      7: subject_field (string)
      8: retention_days (varint)
      9: legal_basis (string)
      10: description (string)
      11: deprecated (bool/varint)
    """
    result: dict[str, Any] = {}
    pos = 0
    while pos < len(raw):
        tag, new_pos = _decode_varint(raw, pos)
        field_number = tag >> 3
        wire_type = tag & 0x7

        if wire_type == 0:  # VARINT
            val, pos = _decode_varint(raw, new_pos)
            if field_number == 1:
                result["type_id"] = val
            elif field_number == 2:
                result["public"] = bool(val)
            elif field_number == 3:
                result["tenant_visible"] = bool(val)
            elif field_number == 4:
                result["inherit"] = bool(val)
            elif field_number == 5:
                result["private"] = bool(val)
            elif field_number == 6:
                result["data_policy"] = val
            elif field_number == 8:
                result["retention_days"] = val
            elif field_number == 11:
                result["deprecated"] = bool(val)
        elif wire_type == 2:  # LENGTH_DELIMITED
            length, pos = _decode_varint(raw, new_pos)
            val_bytes = raw[pos : pos + length]
            pos = pos + length
            if field_number == 7:
                result["subject_field"] = val_bytes.decode("utf-8")
            elif field_number == 9:
                result["legal_basis"] = val_bytes.decode("utf-8")
            elif field_number == 10:
                result["description"] = val_bytes.decode("utf-8")
        else:
            pos = new_pos
            if wire_type == 5:  # FIXED32
                pos += 4
            elif wire_type == 1:  # FIXED64
                pos += 8

    return result


def _parse_edge_opts(raw: bytes) -> dict[str, Any]:
    """Parse EdgeOpts from raw bytes.

    Field numbers (from entdb_options.proto v2):
      1: edge_id (varint)
      2: name (string)
      3: propagate_share (bool/varint)
      4: unique_per_from (bool/varint)
      5: data_policy (enum/varint)
      6: on_subject_exit (enum/varint)
      7: retention_days (varint)
      8: legal_basis (string)
      9: description (string)
      10: deprecated (bool/varint)
    """
    result: dict[str, Any] = {}
    pos = 0
    while pos < len(raw):
        tag, new_pos = _decode_varint(raw, pos)
        field_number = tag >> 3
        wire_type = tag & 0x7

        if wire_type == 0:  # VARINT
            val, pos = _decode_varint(raw, new_pos)
            if field_number == 1:
                result["edge_id"] = val
            elif field_number == 3:
                result["propagate_share"] = bool(val)
            elif field_number == 4:
                result["unique_per_from"] = bool(val)
            elif field_number == 5:
                result["data_policy"] = val
            elif field_number == 6:
                result["on_subject_exit"] = val
            elif field_number == 7:
                result["retention_days"] = val
            elif field_number == 10:
                result["deprecated"] = bool(val)
        elif wire_type == 2:  # LENGTH_DELIMITED
            length, pos = _decode_varint(raw, new_pos)
            val_bytes = raw[pos : pos + length]
            pos = pos + length
            if field_number == 2:
                result["name"] = val_bytes.decode("utf-8")
            elif field_number == 8:
                result["legal_basis"] = val_bytes.decode("utf-8")
            elif field_number == 9:
                result["description"] = val_bytes.decode("utf-8")
        else:
            pos = new_pos
            if wire_type == 5:  # FIXED32
                pos += 4
            elif wire_type == 1:  # FIXED64
                pos += 8

    return result


def _parse_field_opts(raw: bytes) -> dict[str, Any]:
    """Parse FieldOpts from raw bytes.

    Field numbers (from entdb_options.proto v2):
      1: required (bool/varint)
      2: searchable (bool/varint)
      3: indexed (bool/varint)
      4: pii (bool/varint)
      5: phi (bool/varint)
      6: enum_values (string)
      7: kind (string)
      8: ref_type_id (varint)
      9: default_value (string)
      10: description (string)
      11: deprecated (bool/varint)
      12: pii_false (bool/varint)
    """
    result: dict[str, Any] = {}
    pos = 0
    while pos < len(raw):
        tag, new_pos = _decode_varint(raw, pos)
        field_number = tag >> 3
        wire_type = tag & 0x7

        if wire_type == 0:  # VARINT
            val, pos = _decode_varint(raw, new_pos)
            if field_number == 1:
                result["required"] = bool(val)
            elif field_number == 2:
                result["searchable"] = bool(val)
            elif field_number == 3:
                result["indexed"] = bool(val)
            elif field_number == 4:
                result["pii"] = bool(val)
            elif field_number == 5:
                result["phi"] = bool(val)
            elif field_number == 8:
                result["ref_type_id"] = val
            elif field_number == 11:
                result["deprecated"] = bool(val)
            elif field_number == 12:
                result["pii_false"] = bool(val)
        elif wire_type == 2:  # LENGTH_DELIMITED
            length, pos = _decode_varint(raw, new_pos)
            val_bytes = raw[pos : pos + length]
            pos = pos + length
            if field_number == 6:
                result["enum_values"] = val_bytes.decode("utf-8")
            elif field_number == 7:
                result["kind"] = val_bytes.decode("utf-8")
            elif field_number == 9:
                result["default_value"] = val_bytes.decode("utf-8")
            elif field_number == 10:
                result["description"] = val_bytes.decode("utf-8")
        else:
            pos = new_pos
            if wire_type == 5:  # FIXED32
                pos += 4
            elif wire_type == 1:  # FIXED64
                pos += 8

    return result


# ── Code Generators ───────────────────────────────────────────────────


def generate_python(nodes: list[NodeInfo], edges: list[EdgeInfo]) -> str:
    """Generate Python EntDB schema module from parsed proto."""
    lines = [
        '"""EntDB schema — generated from .proto. Do not edit."""',
        "",
        "from entdb_sdk import AclDefaults, DataPolicy, EdgeTypeDef, NodeTypeDef, SubjectExitPolicy, field",
        "",
    ]

    for n in nodes:
        lines.append(f"{n.name} = NodeTypeDef(")
        lines.append(f"    type_id={n.type_id},")
        lines.append(f"    name={n.name!r},")
        if n.description:
            lines.append(f"    description={n.description!r},")
        if n.deprecated:
            lines.append("    deprecated=True,")

        acl_parts = []
        if n.acl_public:
            acl_parts.append("public=True")
        if n.acl_tenant_visible:
            acl_parts.append("tenant_visible=True")
        if n.acl_inherit:
            acl_parts.append("inherit=True")
        if n.is_private:
            acl_parts.append("private=True")
        if acl_parts:
            lines.append(f"    acl_defaults=AclDefaults({', '.join(acl_parts)}),")

        lines.append(f"    data_policy=DataPolicy.{n.data_policy},")
        if n.subject_field:
            lines.append(f"    subject_field={n.subject_field!r},")
        if n.retention_days:
            lines.append(f"    retention_days={n.retention_days},")
        if n.legal_basis:
            lines.append(f"    legal_basis={n.legal_basis!r},")

        if n.fields:
            lines.append("    fields=(")
            for f in n.fields:
                parts = [str(f.field_id), repr(f.name), repr(f.kind)]
                if f.required:
                    parts.append("required=True")
                if f.enum_values:
                    parts.append(f"enum_values={f.enum_values!r}")
                if f.searchable:
                    parts.append("searchable=True")
                if f.indexed:
                    parts.append("indexed=True")
                if f.pii:
                    parts.append("pii=True")
                if f.phi:
                    parts.append("phi=True")
                if f.pii_false:
                    parts.append("pii_false=True")
                if f.ref_type_id:
                    parts.append(f"ref_type_id={f.ref_type_id}")
                if f.deprecated:
                    parts.append("deprecated=True")
                if f.default_value:
                    parts.append(f"default={f.default_value!r}")
                if f.description:
                    parts.append(f"description={f.description!r}")
                lines.append(f"        field({', '.join(parts)}),")
            lines.append("    ),")

        lines.append(")")
        lines.append("")

    for e in edges:
        lines.append(f"{e.name} = EdgeTypeDef(")
        lines.append(f"    edge_id={e.edge_id},")
        lines.append(f"    name={e.name!r},")
        lines.append(f"    from_type={e.from_type},")
        lines.append(f"    to_type={e.to_type},")
        if e.propagate_share:
            lines.append("    propagate_share=True,")
        if e.unique_per_from:
            lines.append("    unique_per_from=True,")
        lines.append(f"    data_policy=DataPolicy.{e.data_policy},")
        if e.on_subject_exit != "BOTH":
            lines.append(f"    on_subject_exit=SubjectExitPolicy.{e.on_subject_exit},")
        if e.retention_days:
            lines.append(f"    retention_days={e.retention_days},")
        if e.legal_basis:
            lines.append(f"    legal_basis={e.legal_basis!r},")
        if e.deprecated:
            lines.append("    deprecated=True,")
        if e.description:
            lines.append(f"    description={e.description!r},")

        if e.props:
            lines.append("    props=(")
            for f in e.props:
                parts = [str(f.field_id), repr(f.name), repr(f.kind)]
                if f.required:
                    parts.append("required=True")
                if f.enum_values:
                    parts.append(f"enum_values={f.enum_values!r}")
                if f.pii:
                    parts.append("pii=True")
                if f.phi:
                    parts.append("phi=True")
                if f.pii_false:
                    parts.append("pii_false=True")
                lines.append(f"        field({', '.join(parts)}),")
            lines.append("    ),")

        lines.append(")")
        lines.append("")

    return "\n".join(lines)


def generate_go(nodes: list[NodeInfo], edges: list[EdgeInfo], package: str = "schema") -> str:
    """Generate Go EntDB schema module from parsed proto."""
    lines = [
        "// EntDB schema — generated from .proto. Do not edit.",
        f"package {package}",
        "",
        'import "github.com/elloloop/entdb/sdk/go/entdb"',
        "",
    ]

    kind_map = {
        "str": "entdb.STRING",
        "int": "entdb.INTEGER",
        "float": "entdb.FLOAT",
        "bool": "entdb.BOOLEAN",
        "timestamp": "entdb.TIMESTAMP",
        "json": "entdb.JSON",
        "bytes": "entdb.BYTES",
        "enum": "entdb.ENUM",
        "ref": "entdb.REFERENCE",
        "list_str": "entdb.LIST_STRING",
        "list_int": "entdb.LIST_INT",
        "list_ref": "entdb.LIST_REF",
    }

    for n in nodes:
        lines.append(f"var {n.name} = entdb.NodeTypeDef{{")
        lines.append(f"\tTypeID:     {n.type_id},")
        lines.append(f'\tName:       "{n.name}",')
        lines.append(f"\tDataPolicy: entdb.DataPolicy{n.data_policy.title()},")
        if n.subject_field:
            lines.append(f'\tSubjectField: "{n.subject_field}",')
        if n.retention_days:
            lines.append(f"\tRetentionDays: {n.retention_days},")
        if n.legal_basis:
            lines.append(f'\tLegalBasis: "{n.legal_basis}",')
        if n.fields:
            lines.append("\tFields: []entdb.FieldDef{")
            for f in n.fields:
                go_kind = kind_map.get(f.kind, "entdb.STRING")
                parts = [f"FieldID: {f.field_id}", f'Name: "{f.name}"', f"Kind: {go_kind}"]
                if f.required:
                    parts.append("Required: true")
                if f.enum_values:
                    vals = ", ".join(f'"{v}"' for v in f.enum_values)
                    parts.append(f"EnumValues: []string{{{vals}}}")
                if f.pii:
                    parts.append("PII: true")
                if f.phi:
                    parts.append("PHI: true")
                if f.pii_false:
                    parts.append("PIIFalse: true")
                lines.append(f"\t\t{{{', '.join(parts)}}},")
            lines.append("\t},")
        lines.append("}")
        lines.append("")

    for e in edges:
        lines.append(f"var {e.name} = entdb.EdgeTypeDef{{")
        lines.append(f"\tEdgeID:         {e.edge_id},")
        lines.append(f'\tName:           "{e.name}",')
        lines.append(f"\tFromType:       {e.from_type},")
        lines.append(f"\tToType:         {e.to_type},")
        if e.propagate_share:
            lines.append("\tPropagateShare: true,")
        lines.append(f"\tDataPolicy:     entdb.DataPolicy{e.data_policy.title()},")
        if e.on_subject_exit != "BOTH":
            lines.append(f"\tOnSubjectExit:  entdb.SubjectExit{e.on_subject_exit.title()},")
        if e.retention_days:
            lines.append(f"\tRetentionDays:  {e.retention_days},")
        if e.legal_basis:
            lines.append(f'\tLegalBasis:     "{e.legal_basis}",')
        lines.append("}")
        lines.append("")

    return "\n".join(lines)


def generate_from_proto(
    proto_path: str,
    lang: str = "python",
    include_dirs: list[str] | None = None,
    go_package: str = "schema",
) -> str:
    """Parse a .proto file and generate EntDB schema code.

    Args:
        proto_path: Path to the schema .proto file
        lang: Output language ("python" or "go")
        include_dirs: Additional proto include paths
        go_package: Go package name (for lang="go")

    Returns:
        Generated source code as string
    """
    nodes, edges = parse_proto(proto_path, include_dirs)

    if lang == "python":
        return generate_python(nodes, edges)
    elif lang == "go":
        return generate_go(nodes, edges, package=go_package)
    else:
        raise ValueError(f"Unsupported language: {lang}")
