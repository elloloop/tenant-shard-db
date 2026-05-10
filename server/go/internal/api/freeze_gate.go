// Freeze gate — load-bearing enforcement for FreezeUser
// (docs/go-port/rpcs/FreezeUser.md). FreezeUser only sets the
// `user_registry.status` flag; this gate is what actually blocks the
// frozen user's mutating RPCs.
//
// Mirrors the Python check at api/grpc_server.py:548-557 (the
// `_check_tenant_access(..., require_write=True)` arm) and the contract
// pin at tests/python/unit/test_gdpr_engine.py:734-751
// (`test_freeze_user_rejects_writes`). Reads remain allowed
// (test_frozen_user_can_still_read at test_gdpr_engine.py:754-767).
//
// The gate consults `GlobalStore.GetUser(userID).Status` and rejects
// any mutating RPC when the status is `frozen` or `pending_deletion`.
// `pending_deletion` is treated identically to `frozen` here because
// the Python gate at grpc_server.py:552 uses the same predicate; this
// preserves bug-for-bug parity with DeleteUser scheduling.
//
// Wiring: cmd/entdb-server/main.go registers this interceptor in the
// chain AFTER the auth interceptor (so a trusted Identity is on ctx)
// and BEFORE the per-handler logic. A nil globalstore means
// "freeze-gate disabled" — every request passes (matches the Python
// guard at grpc_server.py:548 which short-circuits when
// `self.global_store is None`).

package api

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
)

// mutatingMethods is the allow-list of gRPC full method names that
// trigger the freeze gate. Reads (Get*, List*, Search*, Query*,
// Health, GetSchema, GetReceiptStatus, WaitForOffset, ExportUserData)
// are intentionally absent — Python's
// `_check_tenant_access(require_write=False)` returns the role even
// for frozen users (test_frozen_user_can_still_read).
//
// Self-targeted admin RPCs (FreezeUser unfreeze, CancelUserDeletion,
// DeleteUser) are also absent because they MUST remain callable
// against a frozen user — the Python handlers gate those through
// `_is_self_or_admin`, not `_check_tenant_access`. See spec
// "Read-still-allowed scope".
var mutatingMethods = map[string]struct{}{
	"/entdb.v1.EntDBService/ExecuteAtomic":      {},
	"/entdb.v1.EntDBService/ShareNode":          {},
	"/entdb.v1.EntDBService/RevokeAccess":       {},
	"/entdb.v1.EntDBService/AddGroupMember":     {},
	"/entdb.v1.EntDBService/RemoveGroupMember":  {},
	"/entdb.v1.EntDBService/TransferOwnership":  {},
	"/entdb.v1.EntDBService/AddTenantMember":    {},
	"/entdb.v1.EntDBService/RemoveTenantMember": {},
	"/entdb.v1.EntDBService/ChangeMemberRole":   {},
	"/entdb.v1.EntDBService/CreateTenant":       {},
	"/entdb.v1.EntDBService/ArchiveTenant":      {},
	"/entdb.v1.EntDBService/UpdateUser":         {},
}

// FreezeGateInterceptor returns a grpc.UnaryServerInterceptor that
// rejects mutating RPCs from frozen users with codes.PermissionDenied.
// It depends on:
//
//   - the auth interceptor having installed an Identity on ctx (so
//     the trusted user_id is available), and
//   - a non-nil GlobalStore on the Server (so user_registry.status is
//     reachable).
//
// When either is missing the gate is a no-op pass-through — same as
// the Python guard which short-circuits when global_store is unset.
//
// The error message mirrors the Python one at grpc_server.py:553-556
// ("User '<id>' is frozen and cannot write") so existing client tests
// pinning the substring keep passing under the Go port.
func (s *Server) FreezeGateInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := s.checkFreezeGate(ctx, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// checkFreezeGate is the testable inner of the interceptor. Returns
// nil to admit the request, or a codes.PermissionDenied status to
// abort. Exported via the interceptor wrapper above; kept as a method
// on Server so tests can drive it without standing up a full gRPC
// pipeline.
func (s *Server) checkFreezeGate(ctx context.Context, fullMethod string) error {
	// Disabled when no globalstore is wired (single-node bring-up,
	// unit tests that don't exercise the user registry).
	if s.global == nil {
		return nil
	}
	// Reads, health, schema, and self-targeted admin RPCs bypass the
	// gate — only listed mutating methods consult it.
	if _, gated := mutatingMethods[fullMethod]; !gated {
		return nil
	}
	// Need a trusted user identity to look up. If no interceptor
	// installed one, fall through — auth itself will have rejected the
	// request before reaching here in production.
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil
	}
	trusted := auth.Authoritative(ctx, auth.ParseActor(id.Subject))
	// Only user identities can be frozen; system / admin / api-key
	// callers bypass the registry lookup. Mirrors Python's
	// `_check_tenant_access` short-circuit at grpc_server.py:496-499.
	if !trusted.IsUser() {
		return nil
	}
	user, err := s.global.GetUser(ctx, trusted.ID())
	if err != nil {
		// Globalstore IO error — surface as Internal so the operator
		// can detect it. Do NOT swallow as "passed" — that would let
		// a flaky DB silently disable the gate.
		return status.Errorf(codes.Internal, "freeze-gate: lookup user %q: %v", trusted.ID(), err)
	}
	if user == nil {
		// Unknown user — fail closed (Python does the same: an
		// unknown user has no membership, so `_check_tenant_access`
		// aborts PERMISSION_DENIED). The exact message differs from
		// Python; keep it precise.
		return status.Errorf(codes.PermissionDenied, "freeze-gate: user %q not found", trusted.ID())
	}
	if isFrozenStatus(user.Status) {
		return status.Errorf(codes.PermissionDenied,
			"User '%s' is frozen and cannot write", trusted.ID())
	}
	return nil
}

// isFrozenStatus reports whether a user_registry.status value blocks
// mutations. Mirrors the Python predicate at grpc_server.py:552 which
// rejects both `frozen` and `pending_deletion`.
func isFrozenStatus(status string) bool {
	return status == "frozen" || status == "pending_deletion"
}

// Compile-time guard: avoid unused-import warnings if globalstore is
// trimmed in a future refactor. globalstore.User.Status is what we
// actually read above.
var _ = (*globalstore.User)(nil)
