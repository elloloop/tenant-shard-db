// CancelUserDeletion RPC — Wave 2 of the Python → Go server port (EPIC
// #407). Spec: docs/go-port/rpcs/CancelUserDeletion.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:139 (rpc), :1052-1060
// (request/response). Reference Python:
// server/python/entdb_server/api/grpc_server.py:2983-3012 (handler) and
// server/python/entdb_server/global_store.py:842-848 (cancel_deletion)
// + :434-450 (set_user_status).
//
// Semantics (preserved byte-for-byte from the Python handler):
//
//   - Globalstore must be configured. If not, abort with
//     codes.Unimplemented "User registry not configured" — mirrors
//     grpc_server.py:2991-2992.
//   - actor and user_id are required (codes.InvalidArgument).
//   - Trusted-actor self/admin gate via auth.Authoritative + the shared
//     isSelfOrAdmin helper (defined in update_user.go). The wire
//     payload's `actor` is UNTRUSTED post-#168.
//   - "Within grace window" is enforced ONLY by globalstore.CancelDeletion
//     (DELETE … WHERE status='pending'). If the GDPR worker has already
//     swept the row to 'completed', this RPC is a silent no-op:
//     success=false, error="" — IDENTICAL to the Python parity contract
//     (see spec "Error contract" table). The Go port does NOT promote
//     past-no-return to FAILED_PRECONDITION; that's a documented v2
//     follow-up (separate proto + store + test change).
//   - On successful cancel, flip user_registry.status back to "active"
//     via globalstore.SetUserStatus. The two writes are NOT atomic —
//     same as Python (grpc_server.py:3002-3004); a crash between them
//     leaves the user in pending_deletion until retry.
//   - NO WAL append. Global-registry mutations are a deliberate carve-
//     out from CLAUDE.md invariant #1 (see spec "Side effects").
//   - Metrics: emits entdb_grpc_requests_total{method="CancelUserDeletion",
//     status="ok"|"error"} via the shared chokepoint. "ok" is recorded
//     for the in-band no-pending-row arm because no abort fires.

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

const cancelUserDeletionMethod = "CancelUserDeletion"

// CancelUserDeletion implements entdb.v1.EntDBService/CancelUserDeletion.
//
// See file header for the full semantic contract. The handler resolves
// the trusted actor, gates on self-or-admin, removes the pending
// deletion_queue row, and (only on a successful remove) flips the user
// registry status back to "active".
func (s *Server) CancelUserDeletion(ctx context.Context, req *pb.CancelUserDeletionRequest) (*pb.CancelUserDeletionResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(cancelUserDeletionMethod, outcome, time.Since(start))
	}()

	// Configuration gate. Mirrors Python grpc_server.py:2991-2992.
	if s.global == nil {
		outcome = "error"
		return nil, status.Error(codes.Unimplemented, "User registry not configured")
	}

	// Required-field aborts. Mirrors grpc_server.py:2993-2996.
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

	// Step 1: hard-delete the pending row. Returns false if the row was
	// never queued OR if the GDPR worker already swept it to 'completed'
	// (past point of no return). Per parity contract, both surface
	// IDENTICALLY as success=false, error="" — DO NOT promote to
	// FAILED_PRECONDITION (v2 follow-up).
	ok, err := s.global.CancelDeletion(ctx, userID)
	if err != nil {
		// Catch-all in-band failure mirroring grpc_server.py:3010-3012.
		outcome = "error"
		return &pb.CancelUserDeletionResponse{Success: false, Error: err.Error()}, nil
	}
	if !ok {
		// In-band no-op: row never existed or already completed. Same
		// metric label ("ok") as Python — no abort fires.
		return &pb.CancelUserDeletionResponse{Success: false}, nil
	}

	// Step 2: conditional status flip. Only fires when step 1 actually
	// removed a row. Mirrors grpc_server.py:3003-3004.
	if _, err := s.global.SetUserStatus(ctx, userID, "active"); err != nil {
		outcome = "error"
		return &pb.CancelUserDeletionResponse{Success: false, Error: err.Error()}, nil
	}
	return &pb.CancelUserDeletionResponse{Success: true}, nil
}
