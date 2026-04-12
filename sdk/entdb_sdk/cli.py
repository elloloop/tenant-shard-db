"""EntDB CLI — schema generation, validation, and linting.

Commands:
    entdb generate <schema.proto> [--python out.py] [--go out.go]
    entdb check [--baseline .entdb/snapshot.json]
    entdb init
    entdb lint <schema.proto>
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from .codegen import generate_from_proto, parse_proto
from .lint import lint_schema


def cmd_generate(args: argparse.Namespace) -> int:
    """Generate EntDB schema code from .proto file."""
    proto_path = args.proto

    if not Path(proto_path).exists():
        print(f"Error: proto file not found: {proto_path}", file=sys.stderr)
        return 1

    include_dirs = args.include or []

    try:
        if args.python:
            code = generate_from_proto(proto_path, lang="python", include_dirs=include_dirs)
            Path(args.python).parent.mkdir(parents=True, exist_ok=True)
            Path(args.python).write_text(code)
            print(f"Generated Python schema: {args.python}")

        if args.go:
            code = generate_from_proto(
                proto_path,
                lang="go",
                include_dirs=include_dirs,
                go_package=args.go_package or "schema",
            )
            Path(args.go).parent.mkdir(parents=True, exist_ok=True)
            Path(args.go).write_text(code)
            print(f"Generated Go schema: {args.go}")

        if not args.python and not args.go:
            # Default: print Python to stdout
            code = generate_from_proto(proto_path, lang="python", include_dirs=include_dirs)
            print(code)

        # Parse and show summary
        nodes, edges = parse_proto(proto_path, include_dirs=include_dirs)
        propagating = sum(1 for e in edges if e.propagate_share)
        print(
            f"Schema: {len(nodes)} node types, {len(edges)} edge types ({propagating} propagate ACL)"
        )

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1

    return 0


def _build_snapshot(proto_path: str, nodes, edges) -> dict:
    """Build a v2 snapshot dict from parsed nodes and edges."""
    return {
        "version": 2,
        "source": str(proto_path),
        "node_types": [
            {
                "type_id": n.type_id,
                "name": n.name,
                "deprecated": n.deprecated,
                "description": n.description,
                "data_policy": n.data_policy,
                "fields": [
                    {
                        "field_id": f.field_id,
                        "name": f.name,
                        "kind": f.kind,
                        "required": f.required,
                        "pii": f.pii,
                        "enum_values": list(f.enum_values) if f.enum_values else None,
                        "default_value": f.default_value,
                        "deprecated": f.deprecated,
                        "description": f.description,
                    }
                    for f in n.fields
                ],
            }
            for n in nodes
        ],
        "edge_types": [
            {
                "edge_id": e.edge_id,
                "name": e.name,
                "from_type": e.from_type,
                "to_type": e.to_type,
                "propagate_share": e.propagate_share,
                "data_policy": e.data_policy,
                "deprecated": e.deprecated,
                "description": e.description,
            }
            for e in edges
        ],
    }


def cmd_init(args: argparse.Namespace) -> int:
    """Initialize EntDB schema in the current project."""
    proto_path = args.proto
    snapshot_dir = Path(".entdb")
    snapshot_path = snapshot_dir / "snapshot.json"

    if not Path(proto_path).exists():
        print(f"Error: proto file not found: {proto_path}", file=sys.stderr)
        return 1

    try:
        nodes, edges = parse_proto(proto_path, include_dirs=args.include or [])
    except Exception as e:
        print(f"Error parsing proto: {e}", file=sys.stderr)
        return 1

    snapshot_dir.mkdir(exist_ok=True)

    snapshot = _build_snapshot(proto_path, nodes, edges)

    if snapshot_path.exists() and not getattr(args, "force", False):
        print("Snapshot already exists, checking compatibility...")
        old = json.loads(snapshot_path.read_text())
        issues = _check_compat(old, snapshot)
        breaking = [i for i in issues if i["breaking"]]
        if breaking:
            print(f"BREAKING: {len(breaking)} breaking changes detected:")
            for i in breaking:
                print(f"  {i['message']}")
            print("Run 'entdb init --force' to overwrite.")
            return 1
        print(f"Compatible: {len(issues)} non-breaking changes")

    snapshot_path.write_text(json.dumps(snapshot, indent=2) + "\n")
    print(f"Snapshot written to {snapshot_path} ({len(nodes)} node types, {len(edges)} edge types)")
    return 0


def cmd_lint(args: argparse.Namespace) -> int:
    """Lint a proto schema for semantic errors and warnings."""
    proto_path = args.proto

    if not Path(proto_path).exists():
        print(f"Error: proto file not found: {proto_path}", file=sys.stderr)
        return 1

    include_dirs = args.include or []

    try:
        result = lint_schema(proto_path, include_dirs=include_dirs)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1

    if result.errors:
        print(f"ERRORS ({len(result.errors)}):")
        for err in result.errors:
            print(f"  [ERROR] {err}")

    if result.warnings:
        print(f"WARNINGS ({len(result.warnings)}):")
        for warn in result.warnings:
            print(f"  [WARN]  {warn}")

    if not result.errors and not result.warnings:
        print("Lint passed: no errors, no warnings.")

    if result.errors:
        return 1

    if result.warnings:
        print(f"\nLint passed with {len(result.warnings)} warning(s).")

    return 0


def cmd_check(args: argparse.Namespace) -> int:
    """Check schema compatibility against baseline."""
    proto_path = args.proto
    baseline_path = Path(args.baseline or ".entdb/snapshot.json")

    if not baseline_path.exists():
        print(f"No baseline found at {baseline_path}. Run 'entdb init' first.", file=sys.stderr)
        return 1

    if not Path(proto_path).exists():
        print(f"Error: proto file not found: {proto_path}", file=sys.stderr)
        return 1

    try:
        nodes, edges = parse_proto(proto_path, include_dirs=args.include or [])
    except Exception as e:
        print(f"Error parsing proto: {e}", file=sys.stderr)
        return 1

    current = _build_snapshot(proto_path, nodes, edges)

    old = json.loads(baseline_path.read_text())
    issues = _check_compat(old, current)
    breaking = [i for i in issues if i["breaking"]]
    non_breaking = [i for i in issues if not i["breaking"]]

    if breaking:
        print(f"BREAKING: {len(breaking)} breaking change(s)")
        for i in breaking:
            print(f"  [BREAKING] {i['code']}: {i['message']}")
        if non_breaking:
            for i in non_breaking:
                print(f"  [OK] {i['code']}: {i['message']}")
        return 1

    if non_breaking:
        print(f"Schema compatible: 0 breaking, {len(non_breaking)} non-breaking changes")
        for i in non_breaking:
            print(f"  [OK] {i['code']}: {i['message']}")
    else:
        print("Schema unchanged.")

    return 0


# Data policy restrictiveness: higher index = more restrictive
_DATA_POLICY_RESTRICTIVENESS = {
    "EPHEMERAL": 0,
    "BUSINESS": 1,
    "PERSONAL": 2,
    "FINANCIAL": 3,
    "HEALTHCARE": 4,
    "AUDIT": 5,
}


def _check_compat(old: dict, new: dict) -> list[dict]:
    """Compare two schema snapshots for compatibility.

    Returns a list of issues, each with keys:
        breaking (bool): whether the change blocks CI
        code (str): machine-readable change type
        message (str): human-readable description
    """
    issues: list[dict] = []

    old_nodes = {n["type_id"]: n for n in old.get("node_types", [])}
    new_nodes = {n["type_id"]: n for n in new.get("node_types", [])}
    old_edges = {e["edge_id"]: e for e in old.get("edge_types", [])}
    new_edges = {e["edge_id"]: e for e in new.get("edge_types", [])}

    # Also index by name for type_id-change detection
    old_nodes_by_name = {n["name"]: n for n in old.get("node_types", [])}
    new_nodes_by_name = {n["name"]: n for n in new.get("node_types", [])}
    old_edges_by_name = {e["name"]: e for e in old.get("edge_types", [])}
    new_edges_by_name = {e["name"]: e for e in new.get("edge_types", [])}

    # ── Rule 1: Node type removed ────────────────────────────────────
    for tid, n in old_nodes.items():
        if tid not in new_nodes:
            # Check if same name exists with different type_id (rule 5)
            if n["name"] in new_nodes_by_name:
                new_n = new_nodes_by_name[n["name"]]
                issues.append(
                    {
                        "breaking": True,
                        "code": "TYPE_ID_CHANGED",
                        "message": f"{n['name']} type_id changed from {tid} to {new_n['type_id']}",
                    }
                )
            else:
                issues.append(
                    {
                        "breaking": True,
                        "code": "NODE_REMOVED",
                        "message": f"{n['name']} (type_id={tid}) removed",
                    }
                )

    # ── Rule 12: New node type added ─────────────────────────────────
    for tid, n in new_nodes.items():
        if tid not in old_nodes:
            # Skip if this was a type_id change (already reported above)
            if n["name"] not in old_nodes_by_name:
                issues.append(
                    {
                        "breaking": False,
                        "code": "NODE_ADDED",
                        "message": f"{n['name']} (type_id={tid}) added",
                    }
                )

    # ── Rule 2: Edge type removed ────────────────────────────────────
    for eid, e in old_edges.items():
        if eid not in new_edges:
            if e["name"] in new_edges_by_name:
                new_e = new_edges_by_name[e["name"]]
                issues.append(
                    {
                        "breaking": True,
                        "code": "EDGE_ID_CHANGED",
                        "message": f"{e['name']} edge_id changed from {eid} to {new_e['edge_id']}",
                    }
                )
            else:
                issues.append(
                    {
                        "breaking": True,
                        "code": "EDGE_REMOVED",
                        "message": f"{e['name']} (edge_id={eid}) removed",
                    }
                )

    # ── Rule 13: New edge type added ─────────────────────────────────
    for eid, e in new_edges.items():
        if eid not in old_edges:
            if e["name"] not in old_edges_by_name:
                issues.append(
                    {
                        "breaking": False,
                        "code": "EDGE_ADDED",
                        "message": f"{e['name']} (edge_id={eid}) added",
                    }
                )

    # ── Node-level checks on shared type_ids ─────────────────────────
    for tid in sorted(set(old_nodes) & set(new_nodes)):
        old_n, new_n = old_nodes[tid], new_nodes[tid]
        node_name = old_n["name"]

        old_fields = {f["field_id"]: f for f in old_n.get("fields", [])}
        new_fields = {f["field_id"]: f for f in new_n.get("fields", [])}

        # ── Rule 3: Field removed ────────────────────────────────────
        for fid, f in old_fields.items():
            if fid not in new_fields:
                issues.append(
                    {
                        "breaking": True,
                        "code": "FIELD_REMOVED",
                        "message": f"{node_name}.{f['name']} removed",
                    }
                )

        # ── Rule 4: Field kind/type changed ──────────────────────────
        for fid in sorted(set(old_fields) & set(new_fields)):
            old_f, new_f = old_fields[fid], new_fields[fid]
            if old_f.get("kind") != new_f.get("kind"):
                issues.append(
                    {
                        "breaking": True,
                        "code": "FIELD_KIND_CHANGED",
                        "message": (
                            f"{node_name}.{old_f['name']} kind changed "
                            f"from {old_f.get('kind')!r} to {new_f.get('kind')!r}"
                        ),
                    }
                )

            # ── Rule 8: Enum value removed ───────────────────────────
            old_enum = set(old_f.get("enum_values") or [])
            new_enum = set(new_f.get("enum_values") or [])
            removed_vals = old_enum - new_enum
            added_vals = new_enum - old_enum

            for val in sorted(removed_vals):
                issues.append(
                    {
                        "breaking": True,
                        "code": "ENUM_VALUE_REMOVED",
                        "message": f"{node_name}.{old_f['name']} — {val!r} removed",
                    }
                )

            # ── Rule 15: New enum value added ────────────────────────
            for val in sorted(added_vals):
                issues.append(
                    {
                        "breaking": False,
                        "code": "ENUM_VALUE_ADDED",
                        "message": f"{node_name}.{new_f['name']} — {val!r} added",
                    }
                )

            # ── Rule 16: Description changed (field-level) ──────────
            if old_f.get("description", "") != new_f.get("description", ""):
                issues.append(
                    {
                        "breaking": False,
                        "code": "DESCRIPTION_CHANGED",
                        "message": f"{node_name}.{new_f['name']} description changed",
                    }
                )

        # ── Rule 7: Required field added without default ─────────────
        # ── Rule 14: New optional field added ────────────────────────
        for fid, f in new_fields.items():
            if fid not in old_fields:
                if f.get("required") and not f.get("default_value"):
                    issues.append(
                        {
                            "breaking": True,
                            "code": "REQUIRED_FIELD_ADDED",
                            "message": (
                                f"{node_name}.{f['name']} (field_id={fid}) "
                                f"added as required without default"
                            ),
                        }
                    )
                else:
                    issues.append(
                        {
                            "breaking": False,
                            "code": "FIELD_ADDED",
                            "message": f"{node_name}.{f['name']} (field_id={fid}) added",
                        }
                    )

        # ── Rule 11: data_policy changed (node) ─────────────────────
        old_dp = old_n.get("data_policy", "PERSONAL")
        new_dp = new_n.get("data_policy", "PERSONAL")
        if old_dp != new_dp:
            old_rank = _DATA_POLICY_RESTRICTIVENESS.get(old_dp, 2)
            new_rank = _DATA_POLICY_RESTRICTIVENESS.get(new_dp, 2)
            if new_rank > old_rank:
                issues.append(
                    {
                        "breaking": True,
                        "code": "DATA_POLICY_MORE_RESTRICTIVE",
                        "message": (
                            f"{node_name} data_policy changed from {old_dp} to {new_dp} "
                            f"(more restrictive)"
                        ),
                    }
                )
            else:
                issues.append(
                    {
                        "breaking": False,
                        "code": "DATA_POLICY_LESS_RESTRICTIVE",
                        "message": (
                            f"{node_name} data_policy changed from {old_dp} to {new_dp} "
                            f"(less restrictive)"
                        ),
                    }
                )

        # ── Rule 16: Description changed (node-level) ───────────────
        if old_n.get("description", "") != new_n.get("description", ""):
            issues.append(
                {
                    "breaking": False,
                    "code": "DESCRIPTION_CHANGED",
                    "message": f"{node_name} description changed",
                }
            )

        # ── Rule 17: Node deprecated ────────────────────────────────
        if not old_n.get("deprecated") and new_n.get("deprecated"):
            issues.append(
                {
                    "breaking": False,
                    "code": "NODE_DEPRECATED",
                    "message": f"{node_name} deprecated",
                }
            )

    # ── Edge-level checks on shared edge_ids ─────────────────────────
    for eid in sorted(set(old_edges) & set(new_edges)):
        old_e, new_e = old_edges[eid], new_edges[eid]
        edge_name = old_e["name"]

        # ── Rule 9: propagate_share changed ──────────────────────────
        if old_e.get("propagate_share") != new_e.get("propagate_share"):
            issues.append(
                {
                    "breaking": True,
                    "code": "PROPAGATE_SHARE_CHANGED",
                    "message": f"{edge_name} propagate_share changed",
                }
            )

        # ── Rule 10: from/to type changed ────────────────────────────
        if old_e.get("from_type") != new_e.get("from_type"):
            issues.append(
                {
                    "breaking": True,
                    "code": "FROM_TYPE_CHANGED",
                    "message": f"{edge_name} from_type changed",
                }
            )
        if old_e.get("to_type") != new_e.get("to_type"):
            issues.append(
                {
                    "breaking": True,
                    "code": "TO_TYPE_CHANGED",
                    "message": f"{edge_name} to_type changed",
                }
            )

        # ── Rule 11: data_policy changed (edge) ─────────────────────
        old_dp = old_e.get("data_policy", "PERSONAL")
        new_dp = new_e.get("data_policy", "PERSONAL")
        if old_dp != new_dp:
            old_rank = _DATA_POLICY_RESTRICTIVENESS.get(old_dp, 2)
            new_rank = _DATA_POLICY_RESTRICTIVENESS.get(new_dp, 2)
            if new_rank > old_rank:
                issues.append(
                    {
                        "breaking": True,
                        "code": "DATA_POLICY_MORE_RESTRICTIVE",
                        "message": (
                            f"{edge_name} data_policy changed from {old_dp} to {new_dp} "
                            f"(more restrictive)"
                        ),
                    }
                )
            else:
                issues.append(
                    {
                        "breaking": False,
                        "code": "DATA_POLICY_LESS_RESTRICTIVE",
                        "message": (
                            f"{edge_name} data_policy changed from {old_dp} to {new_dp} "
                            f"(less restrictive)"
                        ),
                    }
                )

        # ── Rule 16: Description changed (edge-level) ───────────────
        if old_e.get("description", "") != new_e.get("description", ""):
            issues.append(
                {
                    "breaking": False,
                    "code": "DESCRIPTION_CHANGED",
                    "message": f"{edge_name} description changed",
                }
            )

    return issues


def main() -> None:
    parser = argparse.ArgumentParser(prog="entdb", description="EntDB schema tools")
    sub = parser.add_subparsers(dest="command")

    # generate
    gen = sub.add_parser("generate", help="Generate schema code from .proto")
    gen.add_argument("proto", help="Path to schema .proto file")
    gen.add_argument("--python", help="Output Python file path")
    gen.add_argument("--go", help="Output Go file path")
    gen.add_argument("--go-package", help="Go package name", default="schema")
    gen.add_argument("-I", "--include", action="append", help="Proto include directories")

    # init
    init = sub.add_parser("init", help="Initialize schema snapshot")
    init.add_argument("proto", help="Path to schema .proto file")
    init.add_argument("-I", "--include", action="append", help="Proto include directories")
    init.add_argument("--force", action="store_true", help="Overwrite existing snapshot")

    # check
    chk = sub.add_parser("check", help="Check schema compatibility")
    chk.add_argument("proto", help="Path to schema .proto file")
    chk.add_argument("--baseline", help="Baseline snapshot path", default=".entdb/snapshot.json")
    chk.add_argument("-I", "--include", action="append", help="Proto include directories")

    # lint
    lnt = sub.add_parser("lint", help="Lint schema for semantic errors and warnings")
    lnt.add_argument("proto", help="Path to schema .proto file")
    lnt.add_argument("-I", "--include", action="append", help="Proto include directories")

    args = parser.parse_args()

    if args.command == "generate":
        sys.exit(cmd_generate(args))
    elif args.command == "init":
        sys.exit(cmd_init(args))
    elif args.command == "check":
        sys.exit(cmd_check(args))
    elif args.command == "lint":
        sys.exit(cmd_lint(args))
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
