# mypy: ignore-errors
"""
Schema CLI tool for EntDB.

This tool manages schema definitions and compatibility:
- snapshot: Export current schema to JSON file
- check: Verify compatibility with baseline
- diff: Show differences between schemas

Usage:
    entdb schema snapshot > schema.lock.json
    entdb schema check --baseline schema.lock.json
    entdb schema diff --old schema.v1.json --new schema.v2.json

Invariants:
    - Breaking changes cause non-zero exit code
    - Schema files are deterministic (sorted JSON)
    - Compatibility rules match protobuf semantics

How to change safely:
    - Add new commands, don't modify existing ones
    - Test with various schema evolution scenarios
    - Keep output format stable for CI parsing
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
from typing import Any

from ..schema import (
    SchemaRegistry,
    check_compatibility,
)

logger = logging.getLogger(__name__)


class SchemaCLI:
    """CLI tool for schema management.

    Provides commands for:
    - Exporting schema to JSON
    - Checking compatibility with baseline
    - Showing schema differences

    Example:
        >>> cli = SchemaCLI()
        >>> cli.snapshot(registry)  # Prints JSON to stdout
        >>> cli.check(registry, "schema.lock.json")  # Returns exit code
    """

    def snapshot(self, registry: SchemaRegistry) -> str:
        """Export schema to JSON.

        Args:
            registry: Schema registry to export

        Returns:
            JSON string representation
        """
        schema_dict = registry.to_dict()

        # Add metadata
        output = {
            "version": 1,
            "fingerprint": registry.fingerprint or "unfrozen",
            "schema": schema_dict,
        }

        return json.dumps(output, indent=2, sort_keys=True)

    def check(
        self,
        registry: SchemaRegistry,
        baseline_path: str,
    ) -> tuple[bool, list[str]]:
        """Check compatibility with baseline.

        Args:
            registry: Current schema registry
            baseline_path: Path to baseline schema JSON

        Returns:
            Tuple of (is_compatible, list_of_issues)
        """
        # Load baseline
        with open(baseline_path) as f:
            baseline_data = json.load(f)

        # Handle both raw schema and wrapped format
        schema_data = baseline_data.get("schema", baseline_data)

        # Create registry from baseline
        baseline_registry = SchemaRegistry.from_dict(schema_data)

        # Check compatibility
        changes = check_compatibility(baseline_registry, registry)

        issues = []
        for change in changes:
            if change.is_breaking:
                issues.append(str(change))

        return len(issues) == 0, issues

    def diff(
        self,
        old_path: str,
        new_path: str,
    ) -> list[dict[str, Any]]:
        """Show differences between two schemas.

        Args:
            old_path: Path to old schema JSON
            new_path: Path to new schema JSON

        Returns:
            List of change dictionaries
        """
        # Load schemas
        with open(old_path) as f:
            old_data = json.load(f)
        with open(new_path) as f:
            new_data = json.load(f)

        # Handle wrapped format
        if "schema" in old_data:
            old_data = old_data["schema"]
        if "schema" in new_data:
            new_data = new_data["schema"]

        # Create registries
        old_registry = SchemaRegistry.from_dict(old_data)
        new_registry = SchemaRegistry.from_dict(new_data)

        # Get changes
        changes = check_compatibility(old_registry, new_registry)

        return [
            {
                "kind": change.kind.name,
                "path": change.path,
                "old_value": change.old_value,
                "new_value": change.new_value,
                "message": change.message,
                "is_breaking": change.is_breaking,
            }
            for change in changes
        ]

    def validate(self, registry: SchemaRegistry) -> list[str]:
        """Validate schema for internal consistency.

        Args:
            registry: Schema registry to validate

        Returns:
            List of validation errors
        """
        return registry.validate_all()


def main() -> None:
    """CLI entry point for schema tool."""
    parser = argparse.ArgumentParser(description="EntDB schema management tool")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # snapshot command
    snapshot_parser = subparsers.add_parser("snapshot", help="Export schema to JSON")
    snapshot_parser.add_argument("--module", help="Python module containing schema definitions")
    snapshot_parser.add_argument("--output", "-o", help="Output file (default: stdout)")

    # check command
    check_parser = subparsers.add_parser("check", help="Check compatibility with baseline")
    check_parser.add_argument(
        "--baseline", "-b", required=True, help="Path to baseline schema JSON"
    )
    check_parser.add_argument("--module", help="Python module containing schema definitions")
    check_parser.add_argument("--server", help="Server URL to fetch current schema")

    # diff command
    diff_parser = subparsers.add_parser("diff", help="Show differences between schemas")
    diff_parser.add_argument("--old", required=True, help="Path to old schema JSON")
    diff_parser.add_argument("--new", required=True, help="Path to new schema JSON")
    diff_parser.add_argument(
        "--format", choices=["text", "json"], default="text", help="Output format"
    )

    # validate command
    validate_parser = subparsers.add_parser("validate", help="Validate schema for consistency")
    validate_parser.add_argument("--module", help="Python module containing schema definitions")
    validate_parser.add_argument("--file", help="Schema JSON file to validate")

    args = parser.parse_args()
    cli = SchemaCLI()

    if args.command == "snapshot":
        registry = _load_registry(args.module)
        if registry.fingerprint is None:
            registry.freeze()

        output = cli.snapshot(registry)

        if args.output:
            with open(args.output, "w") as f:
                f.write(output)
            print(f"Schema exported to {args.output}", file=sys.stderr)
        else:
            print(output)

    elif args.command == "check":
        registry = _load_registry(args.module, args.server)
        if registry.fingerprint is None:
            registry.freeze()

        is_compatible, issues = cli.check(registry, args.baseline)

        if is_compatible:
            print("Schema is compatible with baseline")
            sys.exit(0)
        else:
            print(f"Schema compatibility check FAILED with {len(issues)} breaking change(s):")
            for issue in issues:
                print(f"  - {issue}")
            sys.exit(1)

    elif args.command == "diff":
        changes = cli.diff(args.old, args.new)

        if args.format == "json":
            print(json.dumps(changes, indent=2))
        else:
            if not changes:
                print("No changes detected")
            else:
                print(f"Found {len(changes)} change(s):")
                for change in changes:
                    status = "BREAKING" if change["is_breaking"] else "OK"
                    print(f"  [{status}] {change['kind']}: {change['path']}")
                    print(f"          {change['message']}")

        # Exit with error if breaking changes
        breaking = [c for c in changes if c["is_breaking"]]
        sys.exit(1 if breaking else 0)

    elif args.command == "validate":
        if args.file:
            with open(args.file) as f:
                data = json.load(f)
            if "schema" in data:
                data = data["schema"]
            registry = SchemaRegistry.from_dict(data)
        else:
            registry = _load_registry(args.module)

        errors = cli.validate(registry)

        if not errors:
            print("Schema is valid")
            sys.exit(0)
        else:
            print(f"Schema validation failed with {len(errors)} error(s):")
            for error in errors:
                print(f"  - {error}")
            sys.exit(1)


def _load_registry(
    module_path: str | None = None,
    server_url: str | None = None,
) -> SchemaRegistry:
    """Load schema registry from module or server.

    Args:
        module_path: Python module path containing schema
        server_url: Server URL to fetch schema from

    Returns:
        SchemaRegistry instance
    """
    if server_url:
        # Fetch from server
        import urllib.request

        url = f"{server_url.rstrip('/')}/v1/schema"
        with urllib.request.urlopen(url) as response:
            data = json.loads(response.read().decode("utf-8"))
        schema_data = json.loads(data.get("schema_json", "{}"))
        return SchemaRegistry.from_dict(schema_data)

    if module_path:
        # Import module and get registry
        import importlib

        module = importlib.import_module(module_path)
        if hasattr(module, "registry"):
            return module.registry
        if hasattr(module, "get_registry"):
            return module.get_registry()
        raise ValueError(f"Module {module_path} has no 'registry' or 'get_registry()'")

    # Return global registry
    from ..schema import get_registry

    return get_registry()


if __name__ == "__main__":
    main()
