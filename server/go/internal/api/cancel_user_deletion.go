// CancelUserDeletion RPC.
// Spec: docs/go-port/rpcs/CancelUserDeletion.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:139 (rpc), :1052-1060
// (request/response).
//
// Semantics:
//
//   - Globalstore must be configured. If not, abort with
//     codes.Unimplemented "User registry not configured".
//   - actor and user_id are required (codes.InvalidArgument).
//   - Trusted-actor self/admin gate via auth.Authoritative + the shared
//     isSelfOrAdmin helper (defined in update_user.go). The wire
//     payload's `actor` is UNTRUSTED post-#168.
//   - "Within grace window" is enforced by the pre-read of deletion_queue.
//     If the GDPR worker has already swept the row to 'completed', this
//     RPC is a silent no-op: success=false, error="" (see spec "Error
//     contract" table). Promoting past-no-return to FAILED_PRECONDITION
//     is a documented v2 follow-up (separate proto + store + test change).
//   - On successful cancel, append/wait for a global
//     `user_deletion_canceled` WAL op. The applier removes the pending
//     deletion_queue row and flips user_registry.status back to "active"
//     in one globalstore transaction.
//   - Metrics: emits entdb_grpc_requests_total{method="CancelUserDeletion",
//     status="ok"|"error"} via the shared chokepoint. "ok" is recorded
//     for the in-band no-pending-row arm because no abort fires.

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

const cancelUserDeletionMethod = "CancelUserDeletion"

// CancelUserDeletion implements entdb.v1.EntDBService/CancelUserDeletion.
//
// See file header for the full semantic contract. The handler resolves
// the trusted actor, gates on self-or-admin, removes the pending
// deletion_queue row, and appends the global cancel/status mutation.
func (s *Server) CancelUserDeletion(ctx context.Context, req *pb.CancelUserDeletionRequest) (*pb.CancelUserDeletionResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(ctx, cancelUserDeletionMethod, outcome, time.Since(start))
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
			"CancelUserDeletion requires the user themselves or an admin actor")
	}

	entry, err := s.global.GetDeletionEntry(ctx, userID)
	if err != nil {
		outcome = "error"
		return &pb.CancelUserDeletionResponse{Success: false, Error: err.Error()}, nil
	}
	if entry == nil || entry.Status != "pending" {
		// In-band no-op: row never existed or already completed.
		// No abort fires; metric label stays "ok".
		return &pb.CancelUserDeletionResponse{Success: false}, nil
	}

	_, _, err = s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":         string(apply.OpUserDeletionCanceled),
		"user_id":    userID,
		"updated_at": time.Now().Unix(),
	})
	if err != nil {
		outcome = "error"
		return nil, err
	}
	return &pb.CancelUserDeletionResponse{Success: true}, nil
}
