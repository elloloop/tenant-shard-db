# SPDX-License-Identifier: AGPL-3.0-only
"""Generate API + SDK reference markdown from source.

Single source of truth: the proto contract and the two SDK public
surfaces. No hand-written API docs — this script regenerates the
machine-extracted sections so they cannot drift from code.

Post Python-server retirement (ADR-017) the inputs are:

  proto/entdb/v1/entdb.proto    -> docs/generated/api-reference.md
  sdk/python/entdb_sdk          -> docs/generated/sdk-python.md
  sdk/go/entdb (go doc -all)    -> docs/generated/sdk-go.md

The hand-written narrative pages (docs/api-reference.md,
docs/sdk-reference.md) stay editorial; the generated files under
docs/generated/ are the contract surface that
scripts/check_docs_coverage.py enforces. Keeping the two apart means
prose edits never trip the coverage guard and the guard never rewrites
prose.

Usage::

    python scripts/generate_api_docs.py            # write files
    python scripts/generate_api_docs.py --check     # fail if stale
    python scripts/generate_api_docs.py --stdout X  # print one section

The Go section is best-effort: if the Go toolchain is absent the
script still emits the proto + Python sections and notes the Go gap
rather than failing (CI runs `go` separately on Go-touching PRs).
"""

from __future__ import annotations

import argparse
import ast
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
PROTO_PATH = REPO_ROOT / "proto" / "entdb" / "v1" / "entdb.proto"
PYTHON_SDK_INIT = REPO_ROOT / "sdk" / "python" / "entdb_sdk" / "__init__.py"
PYTHON_CLIENT = REPO_ROOT / "sdk" / "python" / "entdb_sdk" / "client.py"
GO_SDK_DIR = REPO_ROOT / "sdk" / "go" / "entdb"
OUT_DIR = REPO_ROOT / "docs" / "generated"

GENERATED_BANNER = (
    "<!-- GENERATED FILE — do not edit by hand.\n"
    "     Regenerate with: python scripts/generate_api_docs.py\n"
    "     Source of truth is the proto + SDK code, not this file. -->\n"
)


# ---------------------------------------------------------------------------
# Proto extraction
# ---------------------------------------------------------------------------


@dataclass
class Rpc:
    name: str
    request: str
    response: str
    doc: str


def _strip_comment(lines: list[str]) -> str:
    """Join accumulated ``// ...`` lines into one paragraph."""
    out = []
    for raw in lines:
        cleaned = raw.strip()
        cleaned = re.sub(r"^//\s?", "", cleaned)
        out.append(cleaned)
    return " ".join(s for s in out if s).strip()


def extract_rpcs(proto_text: str) -> list[Rpc]:
    """Parse the single ``service`` block, capturing leading ``//`` docs.

    Deliberately a small regex/state parser rather than a protobuf
    descriptor walk: the generated stubs drop comments, and adding a
    descriptor-set build step to a docs script is more moving parts
    than the contract surface needs.
    """
    rpcs: list[Rpc] = []
    pending: list[str] = []
    in_service = False
    rpc_re = re.compile(r"rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s*returns\s*\(\s*(\w+)\s*\)")
    for line in proto_text.splitlines():
        stripped = line.strip()
        if stripped.startswith("service "):
            in_service = True
            pending.clear()
            continue
        if not in_service:
            continue
        if stripped == "}":
            break
        if stripped.startswith("//"):
            pending.append(stripped)
            continue
        m = rpc_re.search(stripped)
        if m:
            rpcs.append(
                Rpc(
                    name=m.group(1),
                    request=m.group(2),
                    response=m.group(3),
                    doc=_strip_comment(pending),
                )
            )
            pending.clear()
            continue
        if stripped:
            pending.clear()
    return rpcs


def render_api_reference(rpcs: list[Rpc]) -> str:
    lines = [
        GENERATED_BANNER,
        "# API Reference — gRPC contract (generated)",
        "",
        "> Internal transport. Application code MUST use an SDK "
        "(`pip install entdb-sdk` / `go get .../sdk/go/entdb`). This "
        "page is the machine-extracted RPC inventory; the editorial "
        "narrative lives in [api-reference.md](../api-reference.md).",
        "",
        f"Source: [`proto/entdb/v1/entdb.proto`](../../proto/entdb/v1/entdb.proto). "
        f"**{len(rpcs)} RPCs.**",
        "",
        "| RPC | Request | Response | Description |",
        "|---|---|---|---|",
    ]
    for r in sorted(rpcs, key=lambda x: x.name):
        doc = r.doc.replace("|", "\\|") or "—"
        lines.append(f"| `{r.name}` | `{r.request}` | `{r.response}` | {doc} |")
    lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Python SDK extraction
# ---------------------------------------------------------------------------


@dataclass
class PySymbol:
    name: str
    kind: str  # "method" | "class" | "function"
    signature: str
    doc: str


def _ast_first_doc(node: ast.AST) -> str:
    doc = ast.get_docstring(node) or ""
    return doc.strip().splitlines()[0].strip() if doc.strip() else ""


def _format_args(fn: ast.FunctionDef | ast.AsyncFunctionDef) -> str:
    a = fn.args
    parts: list[str] = []
    posonly = list(a.posonlyargs)
    args = list(a.args)
    defaults = list(a.defaults)
    all_pos = posonly + args
    pad = len(all_pos) - len(defaults)
    for i, arg in enumerate(all_pos):
        if arg.arg == "self":
            continue
        if i >= pad:
            parts.append(f"{arg.arg}=...")
        else:
            parts.append(arg.arg)
    if a.vararg:
        parts.append(f"*{a.vararg.arg}")
    elif a.kwonlyargs:
        parts.append("*")
    for kw, kd in zip(a.kwonlyargs, a.kw_defaults, strict=False):
        parts.append(f"{kw.arg}=..." if kd is not None else kw.arg)
    if a.kwarg:
        parts.append(f"**{a.kwarg.arg}")
    return ", ".join(parts)


def _public_exports(init_text: str) -> list[str]:
    tree = ast.parse(init_text)
    for node in ast.walk(tree):
        if isinstance(node, ast.Assign):
            for tgt in node.targets:
                if isinstance(tgt, ast.Name) and tgt.id == "__all__":
                    return [
                        el.value
                        for el in node.value.elts  # type: ignore[attr-defined]
                        if isinstance(el, ast.Constant)
                    ]
    return []


def extract_python(client_text: str, init_text: str) -> tuple[list[str], list[PySymbol]]:
    """Return (public __all__ names, DbClient public methods)."""
    exports = _public_exports(init_text)
    tree = ast.parse(client_text)
    methods: list[PySymbol] = []
    for node in ast.walk(tree):
        if isinstance(node, ast.ClassDef) and node.name == "DbClient":
            for item in node.body:
                if isinstance(item, ast.FunctionDef | ast.AsyncFunctionDef):
                    if item.name.startswith("_"):
                        continue
                    is_async = isinstance(item, ast.AsyncFunctionDef)
                    sig = f"{'async ' if is_async else ''}{item.name}({_format_args(item)})"
                    methods.append(
                        PySymbol(
                            name=item.name,
                            kind="method",
                            signature=sig,
                            doc=_ast_first_doc(item),
                        )
                    )
    return exports, methods


def render_python(exports: list[str], methods: list[PySymbol]) -> str:
    lines = [
        GENERATED_BANNER,
        "# Python SDK Reference (generated)",
        "",
        "Extracted from `sdk/python/entdb_sdk` — `__all__` and "
        "`DbClient` public methods. Narrative + Go side-by-side lives "
        "in [sdk-reference.md](../sdk-reference.md).",
        "",
        "## Public API surface (`from entdb_sdk import ...`)",
        "",
    ]
    lines += [f"- `{name}`" for name in sorted(exports)]
    lines += [
        "",
        "## `DbClient` methods",
        "",
        "| Method | Description |",
        "|---|---|",
    ]
    for m in sorted(methods, key=lambda x: x.name):
        doc = (m.doc or "—").replace("|", "\\|")
        lines.append(f"| `{m.signature}` | {doc} |")
    lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Go SDK extraction (best-effort, needs `go`)
# ---------------------------------------------------------------------------


def extract_go() -> str | None:
    try:
        proc = subprocess.run(
            ["go", "doc", "-all", "."],
            cwd=GO_SDK_DIR,
            capture_output=True,
            text=True,
            timeout=120,
        )
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return None
    if proc.returncode != 0:
        return None
    return proc.stdout


def render_go(godoc: str | None) -> str:
    lines = [
        GENERATED_BANNER,
        "# Go SDK Reference (generated)",
        "",
        "Module `github.com/elloloop/tenant-shard-db/sdk/go/entdb`. "
        "Canonical rendered docs: "
        "<https://pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb>.",
        "",
    ]
    if godoc is None:
        lines += [
            "_Go toolchain unavailable when this file was generated; "
            "the Go public surface is verified by `go doc` in CI on "
            "Go-touching PRs. See pkg.go.dev for the rendered API._",
            "",
        ]
        return "\n".join(lines)
    lines += ["```text", godoc.rstrip(), "```", ""]
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Driver
# ---------------------------------------------------------------------------


def build_all() -> dict[str, str]:
    proto_text = PROTO_PATH.read_text()
    client_text = PYTHON_CLIENT.read_text()
    init_text = PYTHON_SDK_INIT.read_text()

    rpcs = extract_rpcs(proto_text)
    exports, methods = extract_python(client_text, init_text)
    godoc = extract_go()

    return {
        "api-reference.md": render_api_reference(rpcs),
        "sdk-python.md": render_python(exports, methods),
        "sdk-go.md": render_go(godoc),
    }


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument(
        "--check",
        action="store_true",
        help="exit non-zero if generated files are stale",
    )
    p.add_argument(
        "--stdout",
        metavar="NAME",
        help="print one section to stdout instead of writing files",
    )
    args = p.parse_args(argv)

    sections = build_all()

    if args.stdout:
        if args.stdout not in sections:
            print(
                f"unknown section {args.stdout!r}; known: {', '.join(sections)}",
                file=sys.stderr,
            )
            return 2
        print(sections[args.stdout])
        return 0

    OUT_DIR.mkdir(parents=True, exist_ok=True)

    if args.check:
        stale: list[str] = []
        for fname, content in sections.items():
            path = OUT_DIR / fname
            if not path.exists() or path.read_text() != content:
                stale.append(fname)
        if stale:
            print(
                "Stale generated docs: "
                + ", ".join(stale)
                + "\nRun: python scripts/generate_api_docs.py",
                file=sys.stderr,
            )
            return 1
        print("Generated docs up to date.")
        return 0

    for fname, content in sections.items():
        (OUT_DIR / fname).write_text(content)
        print(f"wrote docs/generated/{fname}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
