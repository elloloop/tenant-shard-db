// ListMailboxUsers is a deprecated stub. The legacy per-user mailbox
// SQLite store has been removed; this handler is retained for proto
// compatibility and unconditionally returns an empty user_ids list.
//
// Port spec: docs/go-port/rpcs/ListMailboxUsers.md.
//
// Contract pins (must not regress):
//   - tests/python/integration/test_grpc_contract.py:281-287 — a valid
//     tenant returns ListMailboxUsersResponse{user_ids: []}.
//   - The handler MUST NOT touch the WAL, canonical SQLite, schema, ACL,
//     audit, quota, or crypto — it is read-only and identity-independent.
//   - An unknown tenant MUST surface NOT_FOUND via tenant.CheckTenant
//     (mirrors the Python _check_tenant gate).

package api

import (
	"context"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// ListMailboxUsers implements entdb.v1.EntDBService/ListMailboxUsers as
// a deprecated stub: tenant gate, then an empty response. Resurrecting
// a real implementation is intentionally out of scope for EPIC #407 —
// see the open-questions section of the port spec.
func (s *Server) ListMailboxUsers(
	ctx context.Context,
	req *pb.ListMailboxUsersRequest,
) (*pb.ListMailboxUsersResponse, error) {
	if err := s.checkTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	// UserIds is explicitly a zero-length, non-nil slice to mirror the
	// Python `ListMailboxUsersResponse(user_ids=[])` pinned by
	// test_grpc_contract.py:286.
	return &pb.ListMailboxUsersResponse{UserIds: []string{}}, nil
}
