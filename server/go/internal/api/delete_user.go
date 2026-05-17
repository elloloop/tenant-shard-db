// SPDX-License-Identifier: AGPL-3.0-only

// DeleteUser RPC.
// Spec: docs/go-port/rpcs/DeleteUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:136 (rpc), :1010-1025
// (messages).
//
// Semantics:
//
//   - Globalstore must be configured. If not, abort with
//     codes.Unimplemented "User registry not configured" (parity with
//     grpc_server.py:2939).
//   - actor and user_id are required (codes.InvalidArgument). Order of
//     validation matches Python: actor before user_id.
//   - Trusted-actor pattern: the privilege decision uses
//     auth.Authoritative(ctx, claimed). The wire `actor` field is
//     UNTRUSTED and only consulted as the fallback when no interceptor
//     ran (unit tests). Self-or-admin gate matches Python's
//     _is_self_or_admin (grpc_server.py:2071-2086) and shares the helper
//     defined in update_user.go.
//   - Idempotent re-queue: if a deletion_queue row with status='pending'
//     already exists, return the EXISTING requested_at / execute_at
//     unchanged. Matches grpc_server.py:2950-2956 and the contract pin
//     at test_gdpr_engine.py:637-643.
//   - User not found is reported IN-BAND: success=false,
//     error="User not found" (no NOT_FOUND status) — pinned by
//     test_gdpr_engine.py:658-662.
//   - grace_days <= 0 is normalized to 30 (Python grpc_server.py:2965).
//     Note that the underlying globalstore.QueueDeletion does NOT clamp
//     itself; the handler is the single source of normalization.
//   - On success returns success=true, status="pending" along with the
//     unix-second timestamps the queue row carries. The state machine is
//     scheduled (=='pending') -> executing -> completed. The handler only
//     ever returns the scheduled state; the worker advances the row.
//   - On success, the handler appends a global `user_deletion_scheduled`
//     WAL op and waits for the applier to insert the deletion_queue row
//     and flip user_registry.status to "pending_deletion" in one
//     globalstore transaction.
//
// Legal-hold gate (NEW behavior, behind a flag):
//
// The Python handler does not check legal hold at queue time. Per spec
// "Side effects" / "Open questions" §4 the Go port adds an explicit
// FAILED_PRECONDITION gate: before queueing, walk
// globalstore.GetUserTenants and reject if any tenant has a legal_holds
// row (globalstore.IsLegalHoldSet). The check is gated on
// Server.WithLegalHoldOnDelete(true) so day-zero parity tests pass with
// the gate disabled; production deployments MUST flip this on once a
// contract test has pinned the new behavior.
//
// DeleteUser is a global WAL mutation. The actual erasure events emitted
// by the GDPR worker (anonymize_user, delete_tenant, remove_membership)
// still go through per-tenant WALs; this handler schedules the global
// queue/status state through the global WAL scope.

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

const deleteUserMethod = "DeleteUser"

// defaultDeleteUserGraceDays is the Python normalization fallback for
// grace_days <= 0 (grpc_server.py:2965). Pulled out as a constant so
// tests can reference it without re-deriving.
const defaultDeleteUserGraceDays = 30

// DeleteUser implements entdb.v1.EntDBService/DeleteUser.
//
// See file header for the full semantic contract. The handler is
// stateless apart from the globalstore handle: it gates on
// trusted-actor + self-or-admin, looks up an existing queue row to
// preserve idempotency, optionally enforces a legal-hold precondition,
// then appends the global queue/status mutation.
func (s *Server) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(deleteUserMethod, outcome, time.Since(start))
	}()

	// Configuration gate. Mirrors Python grpc_server.py:2939.
	if s.global == nil {
		outcome = "error"
		return nil, status.Error(codes.Unimplemented, "User registry not configured")
	}

	// Required-field aborts. Mirrors grpc_server.py:2940-2943.
	if req.GetActor() == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "actor is required")
	}
	userID := req.GetUserId()
	if userID == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Trusted-actor resolution. The interceptor-bound identity wins over
	// the wire claim — the request payload is UNTRUSTED. In no-auth
	// deployments / unit tests with no Identity on ctx, the claimed
	// actor is returned as-is (auth.Authoritative documented fallback).
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))
	if !isSelfOrAdmin(trusted, userID) {
		outcome = "error"
		return nil, status.Error(codes.PermissionDenied,
			"DeleteUser requires the user themselves or an admin actor")
	}

	// Existence check. The Python handler returns OK + success=false on
	// a missing user — keep that asymmetric contract for SDK parity. Do
	// NOT promote to NOT_FOUND.
	user, err := s.global.GetUser(ctx, userID)
	if err != nil {
		outcome = "error"
		return &pb.DeleteUserResponse{Success: false, Error: err.Error()}, nil
	}
	if user == nil {
		return &pb.DeleteUserResponse{Success: false, Error: "User not found"}, nil
	}

	// Idempotency: if a pending entry exists, return it unchanged. The
	// Python handler short-circuits here (grpc_server.py:2950-2956) and
	// re-queueing must NOT push execute_at forward.
	if existing, eerr := s.global.GetDeletionEntry(ctx, userID); eerr == nil && existing != nil && existing.Status == "pending" {
		return &pb.DeleteUserResponse{
			Success:     true,
			RequestedAt: existing.RequestedAt,
			ExecuteAt:   existing.ExecuteAt,
			Status:      "pending",
		}, nil
	}

	// Optional legal-hold gate (Go-port addition). Walk the user's
	// tenants and reject with FAILED_PRECONDITION if any is held. The
	// gate is off by default to keep day-zero parity with Python; flip
	// via api.WithLegalHoldOnDelete(true).
	if s.legalHoldOnDelete {
		members, mErr := s.global.GetUserTenants(ctx, userID)
		if mErr != nil {
			outcome = "error"
			return &pb.DeleteUserResponse{Success: false, Error: mErr.Error()}, nil
		}
		for _, m := range members {
			held, hErr := s.global.IsLegalHoldSet(ctx, m.TenantID)
			if hErr != nil {
				outcome = "error"
				return &pb.DeleteUserResponse{Success: false, Error: hErr.Error()}, nil
			}
			if held {
				outcome = "error"
				return nil, status.Errorf(codes.FailedPrecondition,
					"DeleteUser blocked: tenant %q is under legal hold", m.TenantID)
			}
		}
	}

	// Normalize grace days (parity with grpc_server.py:2965).
	grace := int(req.GetGraceDays())
	if grace <= 0 {
		grace = defaultDeleteUserGraceDays
	}

	requestedAt := time.Now().Unix()
	executeAt := requestedAt + int64(grace)*86400
	_, _, err = s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":           string(apply.OpUserDeletionScheduled),
		"user_id":      userID,
		"requested_at": requestedAt,
		"execute_at":   executeAt,
		"status":       "pending",
	})
	if err != nil {
		outcome = "error"
		return nil, err
	}

	return &pb.DeleteUserResponse{
		Success:     true,
		RequestedAt: requestedAt,
		ExecuteAt:   executeAt,
		Status:      "pending",
	}, nil
}
