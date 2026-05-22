// SPDX-License-Identifier: AGPL-3.0-only

// ListUsers is a global-plane read of the user_registry table.
// Port spec: docs/go-port/rpcs/ListUsers.md.
//
// Parity warts preserved verbatim (file follow-up tickets, do NOT "fix"
// here):
//
//   - No admin-scope check: any authenticated caller passes once
//     request.actor is non-empty.
//   - Silent error swallow: an internal globalstore error returns
//     codes.OK with users=[].
//   - SEC-4 (#135): the historic "No upper cap on limit; negative limit
//     flows through as unlimited (SQLite LIMIT -1)" wart is closed. Any
//     non-positive limit now coerces to the 100 default (was: only ==0),
//     and a positive limit above MaxPageSize is clamped to MaxPageSize
//     before it reaches globalstore. The unbounded-scan path is gone.
//
// Trusted-actor invariant (CLAUDE.md, commit fece3fb): we still bind the
// authoritative actor from ctx via auth.Authoritative even though no
// privilege check consumes it today, so a future scope-gate cannot
// accidentally use the wire claim.

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

// ListUsers implements entdb.v1.EntDBService/ListUsers.
//
// Contract pins:
//
//   - global_store == nil -> codes.Unimplemented "User registry not configured".
//   - request.actor == "" -> codes.InvalidArgument "actor is required"
//     (test_grpc_contract.py:423-427).
//   - empty status -> coerce to "active".
//   - zero limit -> coerce to 100.
//   - any internal error -> codes.OK with users=[].
func (s *Server) ListUsers(
	ctx context.Context,
	req *pb.ListUsersRequest,
) (*pb.ListUsersResponse, error) {
	start := time.Now()
	statusLabel := "ok"
	defer func() {
		metrics.RecordGRPCRequest("ListUsers", statusLabel, time.Since(start))
	}()

	if s.global == nil {
		statusLabel = "error"
		return nil, errs.Errorf(codes.Unimplemented, "User registry not configured")
	}
	if req.GetActor() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}

	// Trusted-actor invariant: rebind from ctx even though no privilege
	// check consumes it today. Keeps the privilege-escalation guard wired
	// for a future capability gate (see commit fece3fb / CLAUDE.md).
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	statusFilter := req.GetStatus()
	if statusFilter == "" {
		statusFilter = "active"
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		// SEC-4 (#135): widened from ==0 to <=0 so a negative limit
		// (SQLite treats LIMIT -1 as unbounded) can no longer trigger
		// a full-table scan of the user registry.
		limit = 100
	}
	// SEC-4 (#135): cap oversized page requests before building protos.
	limit = clampPageSize(limit)
	offset := int(req.GetOffset())

	rows, err := s.global.ListUsers(ctx, statusFilter, limit, offset)
	if err != nil {
		// Mirror Python's outer `except Exception` swallow: log via the
		// metrics label, return an empty list with codes.OK. Tracked as
		// a parity wart for follow-up — this hides DB outages from
		// callers and breaks pagination clients silently.
		statusLabel = "error"
		return &pb.ListUsersResponse{Users: []*pb.UserInfo{}}, nil
	}

	out := make([]*pb.UserInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, userToProto(r))
	}
	return &pb.ListUsersResponse{Users: out}, nil
}
