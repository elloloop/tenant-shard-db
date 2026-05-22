// FreezeUser RPC.
// Spec: docs/go-port/rpcs/FreezeUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:138 (rpc), :1039-1050
// (request/response).
//
// GDPR Article 18 ("restrict processing"): toggle a user between
// `active` and `frozen`. Freeze blocks user-initiated mutations but
// preserves all data (vs `DeleteUser` which tombstones, vs
// `RevokeAllUserAccess` which removes ACL grants).
//
// Semantics:
//
//   - Globalstore must be configured. If not, abort with
//     codes.Unimplemented "User registry not configured".
//   - actor and user_id are required (codes.InvalidArgument).
//   - Trusted-actor pattern: the privilege decision uses
//     auth.Authoritative(ctx, claimed) — when the interceptor has
//     installed an Identity on ctx, the request-payload actor is
//     ignored. Self-or-admin gate (admin/system, or the user
//     themselves).
//   - No tenant scope. FreezeUser is a global-store operation;
//     CheckTenant is NOT called.
//   - WAL-first global mutation. The handler appends a global
//     `user_frozen` op and waits for the applier to set the
//     user-registry status flag.
//   - User-not-found is reported in-band: success=false,
//     error="User not found" (no NOT_FOUND status).
//   - Idempotency: freezing an already-frozen user (or unfreezing an
//     already-active one) returns success=true with the new status,
//     because SetUserStatus UPDATEs unconditionally and rowcount > 0
//     iff a row matched.
//
// The freeze flag is consulted by the freeze gate
// (auth.CheckUserNotFrozen) on every subsequent mutating RPC; this
// handler only sets the bit. Reads remain allowed under freeze.

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
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

	// Configuration gate.
	if s.global == nil {
		outcome = "error"
		return nil, status.Error(codes.Unimplemented, "User registry not configured")
	}

	// Required-field aborts.
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
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))
	if !isSelfOrAdmin(trusted, userID) {
		outcome = "error"
		return nil, status.Error(codes.PermissionDenied,
			"FreezeUser requires the user themselves or an admin actor")
	}

	// Translate enabled bool → status string: "frozen" if enabled else "active".
	newStatus := "active"
	if req.GetEnabled() {
		newStatus = "frozen"
	}

	user, err := s.global.GetUser(ctx, userID)
	if err != nil {
		outcome = "error"
		return &pb.FreezeUserResponse{Success: false, Error: err.Error()}, nil
	}
	if user == nil {
		// In-band not-found. Same metric label ("ok") — no abort fires.
		return &pb.FreezeUserResponse{Success: false, Error: "User not found"}, nil
	}
	_, _, err = s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":         string(apply.OpUserFrozen),
		"user_id":    userID,
		"status":     newStatus,
		"updated_at": time.Now().Unix(),
	})
	if err != nil {
		outcome = "error"
		return nil, err
	}
	return &pb.FreezeUserResponse{Success: true, Status: newStatus}, nil
}
