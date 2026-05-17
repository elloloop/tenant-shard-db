# SPDX-License-Identifier: AGPL-3.0-only
"""Pytest entry point for the runnable examples.

Each ``examples/<name>.py`` is a copy-pasteable script with an
``async def main(endpoint)``. This collector imports each module and
runs its ``main`` against ONE shared contract-seeded Go server so the
whole suite is one server boot, not one-per-example.

This is the CI guard the issue asks for: if an SDK or wire change
breaks a documented example, ``pytest examples/`` fails before a
reader ever copies the stale snippet.

The examples are intentionally NOT named ``test_*.py`` (the repo's
``python_files`` glob) — they must read as standalone scripts. This
module is the single ``test_*`` shim that drives them.
"""

from __future__ import annotations

import importlib
import sys
from pathlib import Path

import pytest

EXAMPLES_DIR = Path(__file__).resolve().parent
REPO_ROOT = EXAMPLES_DIR.parent
for extra in (str(REPO_ROOT / "sdk" / "python"), str(EXAMPLES_DIR)):
    if extra not in sys.path:
        sys.path.insert(0, extra)

from _harness import live_endpoint, reset_sdk_registry  # noqa: E402


def _discover_examples() -> list[str]:
    """Modules under examples/ that expose ``async def main`` — found
    dynamically so adding examples/foo.py needs no edits here. The
    generated ``*_pb2`` schema stub and this shim are excluded."""
    found: list[str] = []
    for p in sorted(EXAMPLES_DIR.glob("*.py")):
        if p.name.startswith(("_", "test_")) or p.stem.endswith("_pb2"):
            continue
        found.append(p.stem)
    return found


EXAMPLE_MODULES = _discover_examples()


@pytest.fixture(scope="module")
async def endpoint():
    """One contract-seeded Go server shared by every example."""
    async with live_endpoint() as ep:
        yield ep


@pytest.mark.parametrize("module_name", EXAMPLE_MODULES)
async def test_example_runs(module_name: str, endpoint: str) -> None:
    """Import examples/<module_name>.py and run its main(endpoint)."""
    # Each example calls register_proto_schema(); that is not
    # idempotent across the shared process, so reset first.
    reset_sdk_registry()
    mod = importlib.import_module(module_name)
    assert hasattr(mod, "main"), f"examples/{module_name}.py must expose `async def main(endpoint)`"
    await mod.main(endpoint)


def test_at_least_two_examples() -> None:
    """Issue #40 asks for >= 2 end-to-end examples."""
    assert len(EXAMPLE_MODULES) >= 2, f"expected >= 2 runnable examples, found {EXAMPLE_MODULES}"
