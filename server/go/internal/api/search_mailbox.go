// SearchMailbox — Wave 2 deprecated stub. Mirrors the Python handler at
// server/python/entdb_server/api/grpc_server.py:1456-1476 byte-for-byte:
// run the tenant gate, then return SearchMailboxResponse{Results: nil,
// HasMore: false}. The legacy per-user mailbox SQLite store was removed
// upstream; fanout writes now land in the per-tenant `notifications`
// table and a future FTS5-backed implementation will land behind this
// same RPC.
//
// Spec: docs/go-port/rpcs/SearchMailbox.md (EPIC #407).
//
// Error contract today (pinned by tests/python/integration/test_grpc_
// contract.py:267-273):
//   - Tenant not owned by this node -> codes.Unavailable +
//     `entdb-redirect-node` trailer.
//   - Tenant region-pinned elsewhere -> codes.FailedPrecondition.
//   - Tenant missing from globalstore -> codes.NotFound.
//   - Happy path -> empty response, nil error.
//
// Out of scope for this PR: FTS5 index, mailbox table reads, authz
// (trusted-actor check). Those land with the real implementation in a
// follow-up. Keeping the stub at byte-for-byte parity preserves the
// cross-implementation contract test.

package api

import (
	"context"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// SearchMailbox implements entdb.v1.EntDBService/SearchMailbox. It is
// intentionally a stub — see file header for the deprecation rationale.
func (s *Server) SearchMailbox(ctx context.Context, req *pb.SearchMailboxRequest) (*pb.SearchMailboxResponse, error) {
	if err := s.checkTenant(ctx, req.GetContext().GetTenantId()); err != nil {
		return nil, err
	}
	return &pb.SearchMailboxResponse{}, nil
}
