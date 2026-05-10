// SPDX-License-Identifier: AGPL-3.0-only

// Shared helpers for the api package. These were previously duplicated
// across the per-RPC handler files (e.g. get_user.go, list_users.go,
// add_tenant_member.go, change_member_role.go) by parallel Wave 2
// PRs that landed on main without seeing each other. Consolidating
// them here keeps `go vet ./internal/api/...` clean and ensures every
// caller sees identical semantics.
//
// Conventions:
//   - Helpers that are pure Server methods (need s.global, s.sharding,
//     etc.) live as methods on *Server.
//   - Helpers that are pure wire-shape transforms live as free
//     functions, mirror their Python counterparts in
//     server/python/entdb_server/api/grpc_server.py, and are nil-safe
//     where the Python equivalent is.

package api

import (
	"context"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// userToProto maps a globalstore.User row to its proto wire form,
// mirroring `_user_dict_to_proto` in the Python handler
// (server/python/entdb_server/api/grpc_server.py:2088-2097). NULL
// columns land as Go zero values via globalstore.User's plain-string
// fields, which marshal to the proto's empty-string defaults -- the
// same shape Python emits when it stringifies a None. A nil row
// yields a zero-value UserInfo so callers never panic on missing data
// (preserves the list_users.go pre-merge behaviour).
func userToProto(u *globalstore.User) *pb.UserInfo {
	if u == nil {
		return &pb.UserInfo{}
	}
	return &pb.UserInfo{
		UserId:    u.UserID,
		Email:     u.Email,
		Name:      u.Name,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// lookupMemberRole returns the role string for (tenantID, userID) or
// "" if no membership row exists. Empty userID short-circuits to ""
// so callers don't have to guard. O(N) in the tenant's member count
// to stay in lock-step with the Python _get_member_role implementation
// (grpc_server.py:2290-2296); a dedicated MemberRole helper in
// globalstore is tracked as a follow-up in the port spec.
//
// Errors from the globalstore are returned raw — call sites decide
// how to wrap them (e.g. as codes.Internal). This matches the
// add_tenant_member.go pre-merge contract; change_member_role.go's
// call site wraps explicitly.
func (s *Server) lookupMemberRole(ctx context.Context, tenantID, userID string) (string, error) {
	if userID == "" {
		return "", nil
	}
	members, err := s.global.GetTenantMembers(ctx, tenantID)
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if m.UserID == userID {
			return m.Role, nil
		}
	}
	return "", nil
}
