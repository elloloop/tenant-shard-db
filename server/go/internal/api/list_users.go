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
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
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
		metrics.RecordGRPCRequest(ctx, "ListUsers", statusLabel, time.Since(start))
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
	// Page size: prefer page_size, fall back to the legacy limit, then the
	// default. SEC-4 (#135): a non-positive value coerces to 100 and an
	// oversized one is clamped, so no unbounded user-registry scan.
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = int(req.GetLimit())
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	pageSize = clampPageSize(pageSize)

	// Keyset cursor (ADR-029): page_token is bound to the status filter by
	// a fingerprint; mixing it with the deprecated offset is INVALID_ARGUMENT.
	fingerprint := usersFingerprint(statusFilter)
	var cursor *globalstore.UserCursor
	if tok := req.GetPageToken(); tok != "" {
		if req.GetOffset() != 0 {
			statusLabel = "error"
			return nil, errs.Errorf(codes.InvalidArgument,
				"page_token and the deprecated offset are mutually exclusive; use one")
		}
		c, derr := decodeUserPageToken(tok, fingerprint)
		if derr != nil {
			statusLabel = "error"
			return nil, derr
		}
		cursor = &globalstore.UserCursor{CreatedAt: c.CreatedAt, UserID: c.UserID}
	}

	rows, err := s.global.ListUsersPaged(ctx, statusFilter, pageSize, cursor)
	if err != nil {
		// Surface genuine faults (#573) instead of masking a DB outage as
		// an empty list with codes.OK. Preserve typed sentinels.
		statusLabel = "error"
		if c := errs.Code(err); c != codes.Unknown {
			return nil, errs.Errorf(c, "ListUsers: %v", err)
		}
		return nil, errs.Internal(ctx, "ListUsers: store", err)
	}

	// Mint a cursor from the last row when the page is full, so a response
	// that omits users always carries a token (ADR-029).
	var nextPageToken string
	if len(rows) == pageSize && pageSize > 0 {
		last := rows[len(rows)-1]
		nextPageToken = encodeUserPageToken(userPageCursor{
			Fingerprint: fingerprint, CreatedAt: last.CreatedAt, UserID: last.UserID,
		})
	}

	out := make([]*pb.UserInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, userToProto(r))
	}
	return &pb.ListUsersResponse{Users: out, NextPageToken: nextPageToken}, nil
}
