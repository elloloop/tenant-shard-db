// GetUser RPC.
// Spec: docs/go-port/rpcs/GetUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:113 (rpc), :816-824
// (request/response), :793-800 (UserInfo).
//
// Semantics (preserved from the Python handler):
//
//   - Authenticated, but unrestricted. Any authenticated actor may
//     read any user's profile. NO self/admin gate — the handler
//     intentionally omits any self/admin check (spec "Auth").
//   - Wire `actor` is UNTRUSTED. The trusted-actor invariant
//     (CLAUDE.md, commit fece3fb) means we resolve identity via
//     auth.Authoritative and ignore the wire claim for any auth
//     decision. We still validate non-empty so the contract suite
//     (which runs without an interceptor) can drive the validation
//     branch -- pinned by tests/python/integration/test_grpc_contract.py:407-411.
//   - Missing user is reported IN-BAND as `found=false` with codes.OK,
//     NOT codes.NotFound. This is a deliberate asymmetry to mirror
//     GetNode's "successful absence" shape (spec "Error contract"). Do
//     NOT upgrade to NOT_FOUND -- existing SDK clients branch on
//     `response.Found`.
//   - When the global store is not wired, return codes.Unimplemented
//     ("User registry not configured").
//   - Catch-all internal errors are swallowed into `found=false` with
//     codes.OK. The status metric is recorded as "error" so the
//     swallow doesn't inflate the ok counter.
//
// No WAL append, no SQLite write, no ACL check. Pure read off
// global_store.

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const getUserMethod = "GetUser"

// GetUser implements entdb.v1.EntDBService/GetUser. See file header
// for the full contract; the body is intentionally a thin translation
// layer over globalstore.GetUser.
func (s *Server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(getUserMethod, resultStatus, time.Since(start))
	}()

	// Guard the optional global_store dep. Returns
	// UNIMPLEMENTED("User registry not configured").
	if s.global == nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Unimplemented, "User registry not configured")
	}

	// Validate non-empty wire actor. The wire field is UNTRUSTED for
	// auth decisions but still required for argument validation
	// (spec "Wire contract" / contract pin :407-411).
	if req.GetActor() == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}

	// Validate non-empty user_id (contract pin :412-416).
	if req.GetUserId() == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "user_id is required")
	}

	// Resolve trusted identity from ctx. The result is intentionally
	// unused for any authorization decision (this RPC has none) -- the
	// call exists so a future tightening to self-or-admin has a single
	// chokepoint to bind against, and so we explicitly honour the
	// trusted-actor pattern documented in CLAUDE.md / commit fece3fb.
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// Read globalstore.user_registry. (nil, nil) on miss mirrors
	// Python's `dict | None` contract.
	user, err := s.global.GetUser(ctx, req.GetUserId())
	if err != nil {
		// Internal errors are swallowed into found=false with codes.OK
		// for parity, but the metric is recorded as "error" so the
		// swallow doesn't inflate the ok counter.
		resultStatus = "error"
		return &pb.GetUserResponse{Found: false}, nil
	}
	if user == nil {
		// In-band missing. NOT_FOUND would break SDK clients that
		// branch on `response.Found`.
		return &pb.GetUserResponse{Found: false}, nil
	}

	return &pb.GetUserResponse{
		Found: true,
		User:  userToProto(user),
	}, nil
}
