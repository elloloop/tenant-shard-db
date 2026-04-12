#!/usr/bin/env python
"""Collect SOC 2 evidence from the codebase.

Outputs: evidence/{date}/ directory with structured JSON reports.

Each report corresponds to a SOC 2 Trust Service Criteria family and
captures a point-in-time snapshot of controls visible in the repository
(governance records, access control code, encryption config, audit log
schema, monitoring, change management, dependencies).

Usage:
    python scripts/collect_soc2_evidence.py [--repo PATH] [--out DIR]

The script is non-destructive; it only reads files and shells out to
``git``. It writes one JSON file per control family under
``evidence/<YYYY-MM-DD>/``.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

# ---------------------------------------------------------------------------
# Utilities
# ---------------------------------------------------------------------------


def _run(cmd: list[str], cwd: Path) -> str:
    """Run a command and return stdout. Return empty string on failure."""
    try:
        result = subprocess.run(
            cmd,
            cwd=str(cwd),
            check=False,
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            return ""
        return result.stdout
    except (OSError, subprocess.SubprocessError):
        return ""


def _read(path: Path, limit: int = 4000) -> str:
    """Read a file and return up to ``limit`` characters."""
    try:
        return path.read_text(encoding="utf-8", errors="replace")[:limit]
    except OSError:
        return ""


def _exists(path: Path) -> bool:
    return path.exists()


def _write(out_dir: Path, name: str, payload: dict[str, Any]) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    path = out_dir / f"{name}.json"
    path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
    return path


# ---------------------------------------------------------------------------
# Evidence collectors
# ---------------------------------------------------------------------------


def collect_cc1_governance(repo: Path) -> dict[str, Any]:
    """CC1 — governance, policies, code of conduct, contribution records."""
    files = {
        "cla": _exists(repo / "CLA.md"),
        "contributing": _exists(repo / "CONTRIBUTING.md"),
        "license": _exists(repo / "LICENSE"),
        "readme": _exists(repo / "README.md"),
        "compliance_dir": _exists(repo / "docs" / "compliance"),
        "adr_dir": _exists(repo / "docs" / "adr"),
    }
    recent_commits = _run(["git", "log", "--format=%H|%aI|%an|%s", "-n", "50"], repo)
    commits: list[dict[str, str]] = []
    for line in recent_commits.splitlines():
        parts = line.split("|", 3)
        if len(parts) == 4:
            commits.append(
                {
                    "sha": parts[0],
                    "date": parts[1],
                    "author": parts[2],
                    "subject": parts[3],
                }
            )
    return {
        "control_family": "CC1",
        "description": "Governance, integrity, accountability",
        "files_present": files,
        "recent_commits": commits,
        "commit_count": len(commits),
    }


def collect_cc6_access_control(repo: Path) -> dict[str, Any]:
    """CC6.1/6.2/6.3 — logical access control code evidence."""
    acl_dir = repo / "dbaas" / "entdb_server"
    files_of_interest = [
        "acl.py",
        "auth.py",
        "registry.py",
        "server.py",
        "rate_limiter.py",
    ]
    artifacts: dict[str, dict[str, Any]] = {}
    for name in files_of_interest:
        path = acl_dir / name
        if path.exists():
            content = _read(path, limit=200_000)
            artifacts[name] = {
                "path": str(path.relative_to(repo)),
                "size_bytes": path.stat().st_size,
                "line_count": content.count("\n"),
                "has_authorize_check": "authorize" in content
                or "check_access" in content
                or "permit" in content,
            }
    tests = (
        sorted(p.name for p in (repo / "tests" / "unit").glob("test_acl*.py") if p.is_file())
        if (repo / "tests" / "unit").exists()
        else []
    )
    return {
        "control_family": "CC6.1/CC6.2/CC6.3",
        "description": "Logical access control implementation",
        "artifacts": artifacts,
        "test_files": tests,
        "test_count": len(tests),
    }


def collect_cc6_encryption(repo: Path) -> dict[str, Any]:
    """CC6.1/6.7 — encryption in transit and at rest."""
    config_path = repo / "dbaas" / "entdb_server" / "config.py"
    config_text = _read(config_path, limit=200_000) if config_path.exists() else ""
    # Look for mentions of TLS, KMS, encryption
    signals = {
        "tls_mentioned": bool(re.search(r"\btls\b|ssl|certificate", config_text, re.IGNORECASE)),
        "kms_mentioned": bool(
            re.search(r"\bkms\b|aes[-_]?256|encrypt", config_text, re.IGNORECASE)
        ),
        "token_auth_mentioned": bool(re.search(r"bearer|token|mtls", config_text, re.IGNORECASE)),
    }
    return {
        "control_family": "CC6.1/CC6.7",
        "description": "Encryption controls",
        "config_file": str(config_path.relative_to(repo)) if config_path.exists() else None,
        "signals": signals,
    }


def collect_cc7_monitoring(repo: Path) -> dict[str, Any]:
    """CC7.1/7.2 — monitoring, metrics, tracing, audit logging."""
    server_dir = repo / "dbaas" / "entdb_server"
    candidates = ["metrics.py", "tracing.py", "audit_log.py", "audit.py"]
    found: dict[str, dict[str, Any]] = {}
    for name in candidates:
        path = server_dir / name
        if path.exists():
            found[name] = {
                "path": str(path.relative_to(repo)),
                "size_bytes": path.stat().st_size,
            }
    # Grep for metric registrations
    metrics_tests = []
    unit_dir = repo / "tests" / "unit"
    if unit_dir.exists():
        metrics_tests = sorted(p.name for p in unit_dir.glob("test_metrics*.py") if p.is_file())
    return {
        "control_family": "CC7.1/CC7.2",
        "description": "Monitoring, metrics, tracing, audit logging",
        "artifacts": found,
        "metrics_test_files": metrics_tests,
    }


def collect_cc8_change_management(repo: Path) -> dict[str, Any]:
    """CC8.1 — change management records from git and CI."""
    workflows_dir = repo / ".github" / "workflows"
    workflows: list[str] = []
    if workflows_dir.exists():
        workflows = sorted(p.name for p in workflows_dir.iterdir() if p.is_file())
    commit_count_raw = _run(["git", "rev-list", "--count", "HEAD"], repo).strip()
    try:
        commit_count = int(commit_count_raw) if commit_count_raw else 0
    except ValueError:
        commit_count = 0
    branches = _run(["git", "branch", "-a"], repo).splitlines()
    tags = _run(["git", "tag", "--list"], repo).splitlines()
    return {
        "control_family": "CC8.1",
        "description": "Change management: CI workflows, git history, branches, releases",
        "workflow_files": workflows,
        "workflow_count": len(workflows),
        "total_commits": commit_count,
        "branch_count": len(branches),
        "tag_count": len(tags),
        "tags_sample": tags[:20],
    }


def collect_dependencies(repo: Path) -> dict[str, Any]:
    """CC7.1/A.8.28 — pinned dependencies as evidence of supply chain controls."""
    manifests = {}
    candidates = [
        "pyproject.toml",
        "requirements.txt",
        "requirements-dev.txt",
        "sdk/pyproject.toml",
        "sdk/requirements.txt",
    ]
    for rel in candidates:
        path = repo / rel
        if path.exists():
            content = _read(path, limit=200_000)
            manifests[rel] = {
                "size_bytes": path.stat().st_size,
                "line_count": content.count("\n"),
                "content_preview": content[:2000],
            }
    return {
        "control_family": "CC7.1",
        "description": "Dependency manifests for supply chain controls",
        "manifests": manifests,
    }


def collect_audit_log_schema(repo: Path) -> dict[str, Any]:
    """CC7.2 — audit log schema as evidence of log completeness."""
    candidates = [
        repo / "dbaas" / "entdb_server" / "audit_log.py",
        repo / "dbaas" / "entdb_server" / "audit.py",
        repo / "dbaas" / "entdb_server" / "canonical_store.py",
    ]
    artifacts: dict[str, Any] = {}
    for path in candidates:
        if path.exists():
            text = _read(path, limit=200_000)
            schema_hits = re.findall(
                r"CREATE\s+TABLE[\s\S]*?;",
                text,
                flags=re.IGNORECASE,
            )
            artifacts[path.name] = {
                "path": str(path.relative_to(repo)),
                "schema_fragments": schema_hits[:5],
            }
    return {
        "control_family": "CC7.2",
        "description": "Audit log schema evidence",
        "artifacts": artifacts,
    }


def collect_repo_summary(repo: Path) -> dict[str, Any]:
    """Top-level repo snapshot."""
    version = ""
    version_file = repo / "VERSION"
    if version_file.exists():
        version = version_file.read_text(encoding="utf-8").strip()
    head = _run(["git", "rev-parse", "HEAD"], repo).strip()
    head_date = _run(["git", "log", "-1", "--format=%aI"], repo).strip()
    branch = _run(["git", "rev-parse", "--abbrev-ref", "HEAD"], repo).strip()
    return {
        "repo_root": str(repo),
        "head_sha": head,
        "head_date": head_date,
        "branch": branch,
        "version": version,
        "python_version": sys.version,
        "collection_timestamp": dt.datetime.now(dt.timezone.utc).isoformat(),
    }


# ---------------------------------------------------------------------------
# Orchestration
# ---------------------------------------------------------------------------


COLLECTORS = [
    ("repo_summary", collect_repo_summary),
    ("cc1_governance", collect_cc1_governance),
    ("cc6_access_control", collect_cc6_access_control),
    ("cc6_encryption", collect_cc6_encryption),
    ("cc7_monitoring", collect_cc7_monitoring),
    ("cc8_change_management", collect_cc8_change_management),
    ("dependencies", collect_dependencies),
    ("audit_log_schema", collect_audit_log_schema),
]


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--repo",
        default=os.environ.get("ENTDB_REPO") or str(Path(__file__).resolve().parents[1]),
        help="Path to repository root (default: parent of scripts/)",
    )
    parser.add_argument(
        "--out",
        default=None,
        help="Output directory (default: <repo>/evidence/<YYYY-MM-DD>)",
    )
    args = parser.parse_args(argv)

    repo = Path(args.repo).resolve()
    if not repo.is_dir():
        print(f"error: repo not found: {repo}", file=sys.stderr)
        return 2

    date_str = dt.date.today().isoformat()
    out_dir = Path(args.out).resolve() if args.out else (repo / "evidence" / date_str)

    written: list[str] = []
    for name, collector in COLLECTORS:
        try:
            payload = collector(repo)
        except Exception as exc:  # noqa: BLE001 — collectors must not crash the run
            payload = {"error": type(exc).__name__, "message": str(exc)}
        path = _write(out_dir, name, payload)
        written.append(str(path))

    manifest = {
        "generated_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "repo": str(repo),
        "files": [Path(p).name for p in written],
    }
    _write(out_dir, "_manifest", manifest)

    print(f"Wrote {len(written) + 1} evidence files to {out_dir}")
    for path in written:
        print(f"  {path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
