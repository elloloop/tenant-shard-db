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
# Wave 2 consolidation: all 44 EntDBService RPCs now have Go-side
# handlers merged on main. Membership here flips the contract suite
# to exercise the Go binary for every RPC when ENTDB_SERVER_TARGET=go.
# Wire-shape parity is best-effort at this point; any behavioural
# divergences from the Python reference implementation become visible
# as test failures and are tracked as follow-ups (see EPIC #407).
GO_IMPLEMENTED: set[str] = {
    "AddGroupMember",
    "AddTenantMember",
    "ArchiveTenant",
    "CancelUserDeletion",
    "ChangeMemberRole",
    "CreateTenant",
    "CreateUser",
    "DelegateAccess",
    "DeleteUser",
    "ExecuteAtomic",
    "ExportUserData",
    "FreezeUser",
    "GetConnectedNodes",
    "GetEdgesFrom",
    "GetEdgesTo",
    "GetMailbox",
    "GetNode",
    "GetNodeByKey",
    "GetNodes",
    "GetReceiptStatus",
    "GetSchema",
    "GetTenant",
    "GetTenantMembers",
    "GetTenantQuota",
    "GetUser",
    "GetUserTenants",
    "Health",
    "ListMailboxUsers",
    "ListSharedWithMe",
    "ListTenants",
    "ListUsers",
    "QueryNodes",
    "RemoveGroupMember",
    "RemoveTenantMember",
    "RevokeAccess",
    "RevokeAllUserAccess",
    "SearchMailbox",
    "SearchNodes",
    "SetLegalHold",
    "ShareNode",
    "TransferOwnership",
    "TransferUserContent",
    "UpdateUser",
    "WaitForOffset",
}


def is_go_implemented(rpc_name: str) -> bool:
    """Return True iff ``rpc_name`` is served by the Go binary at HEAD."""
    return rpc_name in GO_IMPLEMENTED
