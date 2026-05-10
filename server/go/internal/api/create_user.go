// CreateUser RPC — Wave 2 of the Python -> Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/CreateUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:112 (rpc), :802-814 (messages),
// :792-800 (UserInfo). Reference Python:
// server/python/entdb_server/api/grpc_server.py:2099-2147 and
// server/python/entdb_server/global_store.py:336-369.
//
// Semantics (this is a deliberate carve-out from the CLAUDE.md "all
// writes go through the WAL" invariant — user_registry is global_store
// control-plane state, not per-tenant event-sourced data):
//
//   - Admin-only. Caller must resolve to a system:* / admin:* trusted
//     actor via auth.Authoritative. The request.actor field is UNTRUSTED
//     and is fed in only as the fallback when no interceptor ran (unit
//     tests). Privilege-escalation pin:
//     tests/python/integration/test_privilege_escalation.py:321-341.
//
//   - Single SQLite write to global_store.user_registry. No WAL append,
//     no canonical-store touch, no audit hook. The Python handler calls
//     global_store.create_user(...) directly; the Go port does the same
//     via globalstore.CreateUser.
//
//   - user_id boundary: the wire / storage form is bare ("alice"); the
//     "user:alice" tenant_principal form is added by ACL / membership
//     code only. We pass req.UserId through verbatim.
//
// Error contract:
//
//	UNIMPLEMENTED       — global_store not configured.
//	INVALID_ARGUMENT    — empty actor / user_id / email / name.
//	PERMISSION_DENIED   — trusted actor is not system:* / admin:*.
//	ALREADY_EXISTS      — duplicate user_id (PK) or duplicate email
//	                      (UNIQUE). The Go port surfaces this as a
//	                      proper gRPC status (the globalstore layer
//	                      already returns errs.Errorf(AlreadyExists, …)
//	                      via its sqlite-driver sentinel detection).
//	INTERNAL            — any other store failure.

package api

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const createUserMethod = "CreateUser"

// CreateUser implements entdb.v1.EntDBService/CreateUser. See file header
// for the full semantics + error contract.
func (s *Server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(createUserMethod, outcome, time.Since(start))
	}()

	if s.global == nil {
		outcome = "error"
		return nil, errs.Errorf(codes.Unimplemented, "User registry not configured")
	}

	// Validate actor presence first (matches Python ordering at
	// grpc_server.py:2113). We do this BEFORE the trusted-actor lookup so
	// callers that forget to set the field get a precise INVALID_ARGUMENT
	// instead of a confusing PERMISSION_DENIED.
	if req.GetActor() == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}

	// Trusted-actor chokepoint. The wire-claimed actor is fed in as the
	// fallback for the no-interceptor path (unit tests); when a real
	// AuthInterceptor populated ctx, that identity wins regardless of
	// what req.Actor claims. See auth/authoritative.go for the full
	// privilege-escalation argument.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))
	if !(trusted.IsAdmin() || trusted.IsSystem()) {
		outcome = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"CreateUser requires admin or system actor")
	}

	if req.GetUserId() == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "user_id is required")
	}
	if req.GetEmail() == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "email is required")
	}
	if req.GetName() == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "name is required")
	}

	user, err := s.global.CreateUser(ctx, req.GetUserId(), req.GetEmail(), req.GetName())
	if err != nil {
		outcome = "error"
		// globalstore.CreateUser already wraps duplicate-key collisions
		// as errs.Errorf(codes.AlreadyExists, …) using a typed sqlite
		// driver sentinel — propagate that verbatim.
		if errors.Is(err, errs.ErrAlreadyExists) {
			return nil, err
		}
		// Anything else is an unexpected backend failure: surface it as
		// INTERNAL rather than the Python OK+success=false swallow path.
		// The spec flags this as a deliberate Go-port improvement.
		return nil, errs.Errorf(codes.Internal, "create user: %v", err)
	}

	return &pb.CreateUserResponse{
		Success: true,
		User: &pb.UserInfo{
			UserId:    user.UserID,
			Email:     user.Email,
			Name:      user.Name,
			Status:    user.Status,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
	}, nil
}
