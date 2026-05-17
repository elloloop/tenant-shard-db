# SPDX-License-Identifier: AGPL-3.0-only
"""CI guard: every public RPC + SDK symbol must be documented.

Re-scoped post Python-server retirement (ADR-017). The check is:

  1. Every RPC *and* every Operation-oneof member (the
     `ExecuteAtomic` ops, e.g. ``delete_where``) in
     proto/entdb/v1/entdb.proto appears in
     docs/generated/api-reference.md.
  2. Every name in entdb_sdk.__all__ appears in
     docs/generated/sdk-python.md.
  3. Every public method of the client-surface classes
     (``DbClient`` and the ``Plan`` chained-write builder, which
     owns ``delete_where``) appears in
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
    extract_operations,
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
    proto_text = PROTO_PATH.read_text()
    rpcs = extract_rpcs(proto_text)
    ops = extract_operations(proto_text)
    doc = (OUT_DIR / "api-reference.md").read_text()

    missing_rpcs = [r.name for r in rpcs if f"`{r.name}`" not in doc]
    # Operation oneof members (e.g. ``delete_where``) are wire
    # contract too — they ride inside ExecuteAtomic. Enforcing them
    # closes the blind spot that hid ``delete_where`` (#545) from the
    # generated reference.
    missing_ops = [o.field for o in ops if f"`{o.field}`" not in doc]

    ok = True
    if missing_rpcs:
        _fail(
            f"{len(missing_rpcs)} RPC(s) in entdb.proto not in "
            f"docs/generated/api-reference.md: {', '.join(sorted(missing_rpcs))}"
        )
        ok = False
    if missing_ops:
        _fail(
            f"{len(missing_ops)} ExecuteAtomic op(s) in entdb.proto "
            f"not in docs/generated/api-reference.md: "
            f"{', '.join(sorted(missing_ops))}"
        )
        ok = False
    if ok:
        print(f"OK: {len(rpcs)} RPCs + {len(ops)} ExecuteAtomic ops documented.")
    return ok


def check_python_coverage() -> bool:
    exports, methods_by_class = extract_python(
        PYTHON_CLIENT.read_text(), PYTHON_SDK_INIT.read_text()
    )
    doc = (OUT_DIR / "sdk-python.md").read_text()

    missing_exports = [e for e in exports if f"`{e}`" not in doc]
    # Methods are documented as ``async name(...)`` / ``name(...)`` —
    # match on the bare name in backticks-with-paren to avoid false
    # positives on substrings (e.g. ``get`` inside ``get_by_key``).
    # Both DbClient *and* Plan (the chained-write builder, which owns
    # ``delete_where`` — the #545 schema-less sweeper) are enforced so
    # a builder method can't silently fall out of the docs surface.
    missing_methods: list[str] = []
    total_methods = 0
    for class_name, methods in methods_by_class.items():
        total_methods += len(methods)
        missing_methods += [f"{class_name}.{m.name}" for m in methods if f"{m.name}(" not in doc]

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
            f"{len(missing_methods)} client-surface method(s) "
            f"(DbClient/Plan) undocumented in "
            f"docs/generated/sdk-python.md: "
            f"{', '.join(sorted(missing_methods))}"
        )
        ok = False
    if ok:
        print(
            f"OK: {len(exports)} public symbols + {total_methods} DbClient/Plan methods documented."
        )
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
