// FreezeUser RPC — Wave 2 of the Python -> Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/FreezeUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:138 (rpc), :1039-1050
// (request/response). Reference Python:
// server/python/entdb_server/api/grpc_server.py:3072-3104 (handler) and
// server/python/entdb_server/global_store.py:434-447
// (`set_user_status`).
//
// GDPR Article 18 ("restrict processing"): toggle a user between
// `active` and `frozen`. Freeze blocks user-initiated mutations but
// preserves all data (vs `DeleteUser` which tombstones, vs
// `RevokeAllUserAccess` which removes ACL grants).
//
// Semantics (deliberate carve-out from the CLAUDE.md "all writes go
// through the WAL" invariant — user_registry is global_store
// control-plane state, not per-tenant event-sourced data; same shape as
// CreateUser / UpdateUser):
//
//   - Globalstore must be configured. If not, abort with
//     codes.Unimplemented "User registry not configured" — mirrors
//     grpc_server.py:3080-3081.
//   - actor and user_id are required (codes.InvalidArgument).
//   - Trusted-actor pattern: the privilege decision uses
//     auth.Authoritative(ctx, claimed) — when the interceptor has
//     installed an Identity on ctx, the request-payload actor is
//     ignored. Self-or-admin gate matches Python's _is_self_or_admin
//     at grpc_server.py:2071-2086 (admin/system, or the user
//     themselves).
//   - No tenant scope. FreezeUser is a global-store operation;
//     CheckTenant is NOT called. Mirrors the Python handler which
//     delegates only to global_store.set_user_status.
//   - Direct globalstore write (NO WAL append). Wave 2 preserves
//     bug-for-bug parity with Python — the user-registry status flag
//     is control-plane state, written directly. Same carve-out as
//     CreateUser, UpdateUser, RevokeAccess. Future fix-on-port (per
//     spec "Side effects" section) will route through wal.append +
//     applier; out of scope here.
//   - User-not-found is reported in-band: success=false,
//     error="User not found" (no NOT_FOUND status; mirrors
//     grpc_server.py:3094-3096).
//   - Idempotency: freezing an already-frozen user (or unfreezing an
//     already-active one) returns success=true with the new status,
//     because SetUserStatus UPDATEs unconditionally and rowcount > 0
//     iff a row matched. Pinned by the Python
//     test_unfreeze_user_handler contract test
//     (test_gdpr_engine.py:721-731).
//
// The freeze flag is consulted by the freeze gate
// (auth.CheckUserNotFrozen) on every subsequent mutating RPC; this
// handler only sets the bit. Reads remain allowed under freeze
// (test_frozen_user_can_still_read at test_gdpr_engine.py:754-767).

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const freezeUserMethod = "FreezeUser"

// FreezeUser implements entdb.v1.EntDBService/FreezeUser. See file
// header for the full semantic contract.
func (s *Server) FreezeUser(ctx context.Context, req *pb.FreezeUserRequest) (*pb.FreezeUserResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(freezeUserMethod, outcome, time.Since(start))
	}()

	// Configuration gate. Mirrors Python grpc_server.py:3080-3081.
	if s.global == nil {
		outcome = "error"
		return nil, status.Error(codes.Unimplemented, "User registry not configured")
	}

	// Required-field aborts. Mirrors grpc_server.py:3082-3085.
	if req.GetActor() == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "actor is required")
	}
	userID := req.GetUserId()
	if userID == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Trusted-actor resolution. The request payload is UNTRUSTED — the
	// interceptor installs the verified Identity on ctx and
	// auth.Authoritative ignores the wire claim when one is present.
	// Self-or-admin gate mirrors grpc_server.py:3086-3090.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))
	if !isSelfOrAdmin(trusted, userID) {
		outcome = "error"
		return nil, status.Error(codes.PermissionDenied,
			"FreezeUser requires the user themselves or an admin actor")
	}

	// Translate enabled bool → status string. Mirrors
	// grpc_server.py:3092 ("frozen" if request.enabled else "active").
	newStatus := "active"
	if req.GetEnabled() {
		newStatus = "frozen"
	}

	// Direct globalstore write (control-plane carve-out). Mirrors
	// grpc_server.py:3093 — global_store.set_user_status(user_id,
	// new_status). Returns true iff a row matched.
	updated, err := s.global.SetUserStatus(ctx, userID, newStatus)
	if err != nil {
		// Catch-all in-band failure mirroring grpc_server.py:3099-3104.
		// Python writes the metric label as "error" on this arm; we
		// match. Spec note: do NOT promote to codes.Internal — clients
		// pin success/error on the response body, not status codes.
		outcome = "error"
		return &pb.FreezeUserResponse{Success: false, Error: err.Error()}, nil
	}
	if !updated {
		// In-band not-found. Same metric label ("ok") as Python — no
		// abort fires. Mirrors grpc_server.py:3094-3096.
		return &pb.FreezeUserResponse{Success: false, Error: "User not found"}, nil
	}
	return &pb.FreezeUserResponse{Success: true, Status: newStatus}, nil
}
