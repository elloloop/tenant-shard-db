# SPDX-License-Identifier: AGPL-3.0-only
"""Registry of RPCs implemented by the Go ``entdb-server`` binary at HEAD.

This module is the single source of truth for per-RPC gating of the
cross-implementation contract suite. Wave 2 of the Python -> Go server
port (EPIC #407) lands handlers one PR at a time; each PR adds the RPC
name to :data:`GO_IMPLEMENTED` in the same diff that ships the handler.

When ``ENTDB_SERVER_TARGET=go`` is set, the integration conftest
(``tests/python/integration/conftest.py``) automatically skips any
contract case whose RPC isn't present here. This keeps the contract
test file implementation-agnostic — adding an RPC to the Go side is a
one-line change here, not a change to ``test_grpc_contract.py``.

See ``docs/go-port/shared/test-harness.md`` (Per-RPC gating, Phase
progression) for the design.
"""

from __future__ import annotations

# RPCs whose Go-side handler returns a wire-shape-equivalent response
# to the Python reference implementation. Membership here means the
# contract case for that RPC runs against the Go subprocess; non-
# membership means it is skipped (not failed) when target=go.
#
# The registry is intentionally empty at Wave 2 start. Each subsequent
# Wave 2 / Wave 3 PR lands one or more RPC names here together with
# the Go handler.
GO_IMPLEMENTED: set[str] = set()


def is_go_implemented(rpc_name: str) -> bool:
    """Return True iff ``rpc_name`` is served by the Go binary at HEAD."""
    return rpc_name in GO_IMPLEMENTED
