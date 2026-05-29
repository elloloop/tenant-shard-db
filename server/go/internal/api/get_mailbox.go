// GetMailbox is a deprecated stub. The legacy cross-tenant mailbox
// SQLite store was removed; per-tenant fanout now lives in the
// `notifications` table. The RPC is retained for proto compatibility
// and contract pinning (see docs/go-port/rpcs/GetMailbox.md).
//
// Behavior:
//
//  1. Run the tenant gate (CheckTenant): sharding + region. Errors
//     from the gate (UNAVAILABLE with redirect trailer,
//     FAILED_PRECONDITION on region mismatch, NOT_FOUND on missing
//     tenant) propagate to the client unchanged.
//  2. On success, return a well-formed empty response:
//     items = [] (non-nil), unread_count = 0, has_more = false.
//  3. Other exceptions inside the handler body are swallowed and the
//     empty response is returned.
//  4. No SQLite reads, no WAL append, no ACL check, no read of
//     request.user_id / filters / pagination — all ignored on purpose.
//
// Metrics: emits entdb_grpc_requests_total{method="GetMailbox",
// status="ok"|"error"} via the shared metrics chokepoint.

package api

import (
	"context"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// GetMailbox implements entdb.v1.EntDBService/GetMailbox.
//
// DEPRECATED STUB. Always returns an empty mailbox on the success
// path. The only real work is the tenant gate.
func (s *Server) GetMailbox(ctx context.Context, req *pb.GetMailboxRequest) (*pb.GetMailboxResponse, error) {
	start := time.Now()

	// Swallow-on-panic: log and fall through to an empty response.
	// A panic must NOT propagate as a gRPC error to the SDK.
	var (
		resp *pb.GetMailboxResponse
		err  error
	)
	defer func() {
		if r := recover(); r != nil {
			metrics.RecordGRPCRequest(ctx, "GetMailbox", "error", time.Since(start))
			resp = emptyMailboxResponse()
			err = nil
		}
	}()

	tenantID := req.GetContext().GetTenantId()
	if cerr := s.checkTenant(ctx, tenantID); cerr != nil {
		metrics.RecordGRPCRequest(ctx, "GetMailbox", "error", time.Since(start))
		return nil, cerr
	}

	metrics.RecordGRPCRequest(ctx, "GetMailbox", "ok", time.Since(start))
	resp = emptyMailboxResponse()
	return resp, err
}

// emptyMailboxResponse is the canned empty response. Items is a
// non-nil empty slice (`items=[]`); protobuf-go marshals nil and an
// empty slice identically on the wire, but explicit `[]` keeps the Go
// source readable.
func emptyMailboxResponse() *pb.GetMailboxResponse {
	return &pb.GetMailboxResponse{
		Items:       []*pb.MailboxItem{},
		UnreadCount: 0,
		HasMore:     false,
	}
}
