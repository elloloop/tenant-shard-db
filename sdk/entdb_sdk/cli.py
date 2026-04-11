"""EntDB CLI — schema generation and validation.

Commands:
    entdb generate <schema.proto> [--python out.py] [--go out.go]
    entdb check [--baseline .entdb/snapshot.json]
    entdb init
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from .codegen import generate_from_proto, parse_proto


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

    # Build snapshot
    snapshot = {
        "version": 1,
        "source": str(proto_path),
        "node_types": [
            {
                "type_id": n.type_id,
                "name": n.name,
                "fields": [
                    {"field_id": f.field_id, "name": f.name, "kind": f.kind, "required": f.required}
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
            }
            for e in edges
        ],
    }

    if snapshot_path.exists():
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
    propagating = sum(1 for e in edges if e.propagate_share)
    print(f"Snapshot written to {snapshot_path}")
    print(f"Schema: {len(nodes)} node types, {len(edges)} edge types ({propagating} propagate ACL)")
    print("Add .entdb/snapshot.json to version control.")
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

    current = {
        "node_types": [
            {
                "type_id": n.type_id,
                "name": n.name,
                "fields": [
                    {"field_id": f.field_id, "name": f.name, "kind": f.kind, "required": f.required}
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
            }
            for e in edges
        ],
    }

    old = json.loads(baseline_path.read_text())
    issues = _check_compat(old, current)
    breaking = [i for i in issues if i["breaking"]]
    non_breaking = [i for i in issues if not i["breaking"]]

    if breaking:
        print(f"BREAKING: {len(breaking)} breaking change(s):")
        for i in breaking:
            print(f"  [BREAKING] {i['message']}")
        if non_breaking:
            print(f"Plus {len(non_breaking)} non-breaking change(s)")
        return 1

    if non_breaking:
        print(f"Schema compatible: {len(non_breaking)} non-breaking change(s)")
        for i in non_breaking:
            print(f"  [OK] {i['message']}")
    else:
        print("Schema unchanged.")

    return 0


def _check_compat(old: dict, new: dict) -> list[dict]:
    """Compare two schema snapshots for compatibility."""
    issues = []

    old_nodes = {n["type_id"]: n for n in old.get("node_types", [])}
    new_nodes = {n["type_id"]: n for n in new.get("node_types", [])}
    old_edges = {e["edge_id"]: e for e in old.get("edge_types", [])}
    new_edges = {e["edge_id"]: e for e in new.get("edge_types", [])}

    # Removed node types
    for tid, n in old_nodes.items():
        if tid not in new_nodes:
            issues.append(
                {"breaking": True, "message": f"Node type '{n['name']}' (type_id={tid}) removed"}
            )

    # Added node types
    for tid, n in new_nodes.items():
        if tid not in old_nodes:
            issues.append(
                {"breaking": False, "message": f"Node type '{n['name']}' (type_id={tid}) added"}
            )

    # Modified node types
    for tid in set(old_nodes) & set(new_nodes):
        old_n, new_n = old_nodes[tid], new_nodes[tid]
        old_fields = {f["field_id"]: f for f in old_n.get("fields", [])}
        new_fields = {f["field_id"]: f for f in new_n.get("fields", [])}

        for fid, f in old_fields.items():
            if fid not in new_fields:
                issues.append(
                    {
                        "breaking": True,
                        "message": f"{old_n['name']}.{f['name']} (field_id={fid}) removed",
                    }
                )
            elif old_fields[fid].get("kind") != new_fields[fid].get("kind"):
                issues.append(
                    {"breaking": True, "message": f"{old_n['name']}.{f['name']} kind changed"}
                )

        for fid, f in new_fields.items():
            if fid not in old_fields:
                issues.append(
                    {
                        "breaking": False,
                        "message": f"{new_n['name']}.{f['name']} (field_id={fid}) added",
                    }
                )

    # Removed edge types
    for eid, e in old_edges.items():
        if eid not in new_edges:
            issues.append(
                {"breaking": True, "message": f"Edge type '{e['name']}' (edge_id={eid}) removed"}
            )

    # Added edge types
    for eid, e in new_edges.items():
        if eid not in old_edges:
            issues.append(
                {"breaking": False, "message": f"Edge type '{e['name']}' (edge_id={eid}) added"}
            )

    # Modified edge types
    for eid in set(old_edges) & set(new_edges):
        old_e, new_e = old_edges[eid], new_edges[eid]
        if old_e.get("propagate_share") != new_e.get("propagate_share"):
            issues.append(
                {"breaking": True, "message": f"Edge '{old_e['name']}' propagate_share changed"}
            )
        if old_e.get("from_type") != new_e.get("from_type"):
            issues.append(
                {"breaking": True, "message": f"Edge '{old_e['name']}' from_type changed"}
            )
        if old_e.get("to_type") != new_e.get("to_type"):
            issues.append({"breaking": True, "message": f"Edge '{old_e['name']}' to_type changed"})

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

    args = parser.parse_args()

    if args.command == "generate":
        sys.exit(cmd_generate(args))
    elif args.command == "init":
        sys.exit(cmd_init(args))
    elif args.command == "check":
        sys.exit(cmd_check(args))
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
