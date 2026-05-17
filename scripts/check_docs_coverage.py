# SPDX-License-Identifier: AGPL-3.0-only
"""CI guard: every public RPC + SDK symbol must be documented.

Re-scoped post Python-server retirement (ADR-017). The check is:

  1. Every RPC in proto/entdb/v1/entdb.proto appears in
     docs/generated/api-reference.md.
  2. Every name in entdb_sdk.__all__ appears in
     docs/generated/sdk-python.md.
  3. Every public DbClient method appears in
     docs/generated/sdk-python.md.
  4. The generated files are not stale (delegates to
     generate_api_docs.py --check) so coverage can't be faked by
     editing the generated markdown without re-running the generator.

This is intentionally a *coverage* guard, not a *quality* guard: it
proves nothing was silently dropped from the docs surface when the
proto or SDK changed. It does not run the examples — that is a
separate CI step (`pytest examples/`) so a flaky example never blocks
a docs-only change and vice versa.

Exit codes: 0 = covered, 1 = gap found, 2 = usage/IO error.

Usage::

    python scripts/check_docs_coverage.py
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

# Reuse the extractors so the two scripts can never disagree about
# what "the surface" is.
from generate_api_docs import (  # type: ignore[import-not-found]
    OUT_DIR,
    PROTO_PATH,
    PYTHON_CLIENT,
    PYTHON_SDK_INIT,
    REPO_ROOT,
    extract_python,
    extract_rpcs,
)

SCRIPTS_DIR = Path(__file__).resolve().parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))


def _fail(msg: str) -> None:
    print(f"::error::{msg}" if _in_ci() else f"ERROR: {msg}", file=sys.stderr)


def _in_ci() -> bool:
    import os

    return os.environ.get("CI") == "true" or os.environ.get("GITHUB_ACTIONS") == "true"


def check_generated_fresh() -> bool:
    """generate_api_docs.py --check; fail loudly if stale."""
    proc = subprocess.run(
        [sys.executable, str(SCRIPTS_DIR / "generate_api_docs.py"), "--check"],
        capture_output=True,
        text=True,
    )
    sys.stdout.write(proc.stdout)
    if proc.returncode != 0:
        _fail(
            "Generated docs are stale or missing. Run "
            "`python scripts/generate_api_docs.py` and commit "
            "docs/generated/. Detail:\n" + proc.stderr.strip()
        )
        return False
    return True


def check_rpc_coverage() -> bool:
    rpcs = extract_rpcs(PROTO_PATH.read_text())
    doc = (OUT_DIR / "api-reference.md").read_text()
    missing = [r.name for r in rpcs if f"`{r.name}`" not in doc]
    if missing:
        _fail(
            f"{len(missing)} RPC(s) in entdb.proto not in "
            f"docs/generated/api-reference.md: {', '.join(sorted(missing))}"
        )
        return False
    print(f"OK: {len(rpcs)} RPCs documented.")
    return True


def check_python_coverage() -> bool:
    exports, methods = extract_python(PYTHON_CLIENT.read_text(), PYTHON_SDK_INIT.read_text())
    doc = (OUT_DIR / "sdk-python.md").read_text()

    missing_exports = [e for e in exports if f"`{e}`" not in doc]
    # Methods are documented as ``async name(...)`` / ``name(...)`` —
    # match on the bare name in backticks-with-paren to avoid false
    # positives on substrings (e.g. ``get`` inside ``get_by_key``).
    missing_methods = [m.name for m in methods if f"{m.name}(" not in doc]

    ok = True
    if missing_exports:
        _fail(
            f"{len(missing_exports)} entdb_sdk.__all__ symbol(s) "
            f"undocumented in docs/generated/sdk-python.md: "
            f"{', '.join(sorted(missing_exports))}"
        )
        ok = False
    if missing_methods:
        _fail(
            f"{len(missing_methods)} DbClient method(s) undocumented "
            f"in docs/generated/sdk-python.md: "
            f"{', '.join(sorted(missing_methods))}"
        )
        ok = False
    if ok:
        print(f"OK: {len(exports)} public symbols + {len(methods)} DbClient methods documented.")
    return ok


def main() -> int:
    if not PROTO_PATH.exists():
        _fail(f"proto not found at {PROTO_PATH}")
        return 2
    if not OUT_DIR.exists():
        _fail(
            f"{OUT_DIR.relative_to(REPO_ROOT)} missing — run "
            "`python scripts/generate_api_docs.py` first."
        )
        return 2

    results = [
        check_generated_fresh(),
        check_rpc_coverage(),
        check_python_coverage(),
    ]
    if all(results):
        print("Documentation coverage: PASS")
        return 0
    print("Documentation coverage: FAIL", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
