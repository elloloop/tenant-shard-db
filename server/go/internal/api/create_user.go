// CreateUser RPC.
// Spec: docs/go-port/rpcs/CreateUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:112 (rpc), :802-814 (messages),
// :792-800 (UserInfo).
//
// Semantics:
//
//   - Admin-only. Caller must resolve to a system:* / admin:* trusted
//     actor via auth.Authoritative. The request.actor field is UNTRUSTED
//     and is fed in only as the fallback when no interceptor ran (unit
//     tests).
//
//   - WAL-first global mutation. The handler appends a global
//     `user_created` op and waits for the applier to insert the
//     user_registry row. It does not write globalstore directly.
//
//   - user_id boundary: the wire / storage form is bare ("alice"); the
//     "user:alice" tenant_principal form is added by ACL / membership
//     code only. We pass req.UserId through verbatim.
//
// Error contract:
//
//	UNIMPLEMENTED — global_store not configured.
//	INVALID_ARGUMENT — empty actor / user_id / email / name.
//	PERMISSION_DENIED — trusted actor is not system:* / admin:*.
//	ALREADY_EXISTS — duplicate user_id (PK) or duplicate email
//	                      (UNIQUE). The Go port surfaces this as a
//	                      proper gRPC status (the globalstore layer
//	                      already returns errs.Errorf(AlreadyExists, …)
//	                      via its sqlite-driver sentinel detection).
//	INTERNAL — any other store failure.

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
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
		metrics.RecordGRPCRequest(ctx, createUserMethod, outcome, time.Since(start))
	}()

	if s.global == nil {
		outcome = "error"
		return nil, errs.Errorf(codes.Unimplemented, "User registry not configured")
	}

	// Validate actor presence first, before the trusted-actor lookup, so
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

	if existing, err := s.global.GetUser(ctx, req.GetUserId()); err != nil {
		outcome = "error"
		return nil, errs.Internal(ctx, "get user", err)
	} else if existing != nil {
		outcome = "error"
		return nil, errs.Errorf(codes.AlreadyExists,
			"globalstore: user %q already exists", req.GetUserId())
	}
	if existing, err := s.global.GetUserByEmail(ctx, req.GetEmail()); err != nil {
		outcome = "error"
		return nil, errs.Internal(ctx, "get user by email", err)
	} else if existing != nil {
		outcome = "error"
		return nil, errs.Errorf(codes.AlreadyExists,
			"globalstore: email %q already exists", req.GetEmail())
	}

	now := time.Now().Unix()
	_, _, err := s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":         string(apply.OpUserCreated),
		"user_id":    req.GetUserId(),
		"email":      req.GetEmail(),
		"name":       req.GetName(),
		"status":     "active",
		"created_at": now,
		"updated_at": now,
	})
	if err != nil {
		outcome = "error"
		return nil, err
	}

	return &pb.CreateUserResponse{
		Success: true,
		User: &pb.UserInfo{
			UserId:    req.GetUserId(),
			Email:     req.GetEmail(),
			Name:      req.GetName(),
			Status:    "active",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}
