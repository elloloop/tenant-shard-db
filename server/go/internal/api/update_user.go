// UpdateUser RPC.
// Spec: docs/go-port/rpcs/UpdateUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:114 (rpc), :826-838 (request/
// response).
//
// Semantics:
//
//   - Globalstore must be configured. If not, abort with
//     codes.Unimplemented "User registry not configured".
//   - actor and user_id are required (codes.InvalidArgument).
//   - Trusted-actor pattern: the privilege decision uses
//     auth.Authoritative(ctx, claimed) — when the interceptor has
//     installed an Identity on ctx, the request-payload actor is
//     ignored. Self-or-admin gate (see isSelfOrAdmin below).
//   - Truthy-only partial update: empty string == "do not update".
//     There is no FieldMask (the proto doesn't carry presence). If
//     all three mutable fields (email/name/status) are empty, the
//     handler short-circuits and returns success=false,
//     error="No fields to update" — IN-BAND, NOT codes.InvalidArgument.
//     The contract pin is at test_grpc_contract.py:448-453.
//   - User-not-found is also reported in-band: success=false,
//     error="User not found" (no NOT_FOUND status).
//   - WAL-first global mutation. The handler appends a global
//     `user_updated` op carrying the full desired user_registry row and
//     waits for the applier; it does not write globalstore directly.
//   - Metrics: emits entdb_grpc_requests_total{method="UpdateUser",
//     status="ok"|"error"} via the shared chokepoint. Note "ok" is
//     recorded for in-band failures (no-fields, not-found) because
//     no abort fires.

package api

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const updateUserMethod = "UpdateUser"

// UpdateUser implements entdb.v1.EntDBService/UpdateUser.
//
// See file header for the full semantic contract. The handler is
// stateless apart from the globalstore handle: each call resolves the
// trusted actor, gates on self-or-admin, builds the full desired row,
// and appends it to the global WAL scope.
func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(updateUserMethod, outcome, time.Since(start))
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
			"UpdateUser requires the user themselves or admin actor")
	}

	if req.GetEmail() == "" && req.GetName() == "" && req.GetStatus() == "" {
		// In-band failure (no abort); "ok" metric label.
		return &pb.UpdateUserResponse{Success: false, Error: "No fields to update"}, nil
	}

	user, err := s.global.GetUser(ctx, userID)
	if err != nil {
		outcome = "error"
		return &pb.UpdateUserResponse{Success: false, Error: err.Error()}, nil
	}
	if user == nil {
		// In-band not-found; "ok" metric label, no abort.
		return &pb.UpdateUserResponse{Success: false, Error: "User not found"}, nil
	}

	email := user.Email
	if req.GetEmail() != "" {
		email = req.GetEmail()
		if existing, err := s.global.GetUserByEmail(ctx, email); err != nil {
			outcome = "error"
			return &pb.UpdateUserResponse{Success: false, Error: err.Error()}, nil
		} else if existing != nil && existing.UserID != userID {
			outcome = "error"
			return &pb.UpdateUserResponse{Success: false, Error: "Email already exists"}, nil
		}
	}
	name := user.Name
	if req.GetName() != "" {
		name = req.GetName()
	}
	userStatus := user.Status
	if req.GetStatus() != "" {
		userStatus = req.GetStatus()
	}
	updatedAt := time.Now().Unix()
	_, _, err = s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":         string(apply.OpUserUpdated),
		"user_id":    userID,
		"email":      email,
		"name":       name,
		"status":     userStatus,
		"created_at": user.CreatedAt,
		"updated_at": updatedAt,
	})
	if err != nil {
		outcome = "error"
		return nil, err
	}
	return &pb.UpdateUserResponse{Success: true}, nil
}

// isSelfOrAdmin reports whether the trusted actor may modify the given
// user_id. Admin/system identities are always allowed; user identities
// are allowed only when their bare ID matches user_id (the "self" arm).
//
// The "__system__" bootstrap actor used by the Applier never appears on
// the wire, so it is not honoured here.
func isSelfOrAdmin(trusted auth.Actor, userID string) bool {
	if trusted.IsAdmin() || trusted.IsSystem() {
		return true
	}
	if trusted.IsUser() && trusted.ID() == userID {
		return true
	}
	// Defence in depth: also honour bare-id form (no "user:" prefix).
	// ParseActor classifies a bare string with no colon as KindUnknown
	// carrying the raw id.
	if trusted.Kind() == auth.KindUnknown && !strings.Contains(trusted.ID(), ":") && trusted.ID() == userID {
		return true
	}
	return false
}

// stringPtr is a tiny helper so handler bodies stay readable. Using a
// dedicated helper rather than &v inline keeps the truthy-gate above
// the actual struct construction visually distinct.
func stringPtr(s string) *string { return &s }
