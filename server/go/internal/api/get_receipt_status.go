// W2.03 — GetReceiptStatus RPC (Python -> Go server port, EPIC #407).
//
// Spec: docs/go-port/rpcs/GetReceiptStatus.md. The handler is a pure
// read of the per-tenant applied_events table indexed by
// (tenant_id, idempotency_key). It is intentionally minimal:
//
//   - No auth / permission check (parity with
//     api/grpc_server.py:946-971: actor in RequestContext is untrusted
//     and ignored).
//   - tenant.CheckTenant is the only ingress gate; on
//     UNAVAILABLE / FAILED_PRECONDITION / NotFound it returns a gRPC
//     error directly (the SDK relies on the redirect trailer being set
//     before the status closes).
//   - Any other error — including a panic inside CheckIdempotency —
//     collapses to status=UNKNOWN, error=err.Error(), and the RPC
//     returns OK. This mirrors the Python bare `except Exception` at
//     grpc_server.py:966 and is required by the contract tests
//     pinning (see spec, "Contract tests" section).
//   - PENDING is returned both for "key issued but applier hasn't
//     caught up" and "key never issued" — the two are wire-level
//     indistinguishable; see spec, "Open questions" PENDING ambiguity.

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// grpcMethodGetReceiptStatus is the metric label. Mirrors the Python
// `record_grpc_request("GetReceiptStatus", ...)` calls in
// api/grpc_server.py:964,967.
const grpcMethodGetReceiptStatus = "GetReceiptStatus"

// GetReceiptStatus implements entdb.v1.EntDBService/GetReceiptStatus.
func (s *Server) GetReceiptStatus(ctx context.Context, req *pb.GetReceiptStatusRequest) (resp *pb.GetReceiptStatusResponse, err error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(grpcMethodGetReceiptStatus, outcome, time.Since(start))
	}()

	// Panic safety: Python's bare `except Exception` (grpc_server.py:966)
	// collapses every runtime fault — including programmer errors — to
	// status=UNKNOWN + error=<str(exc)>, with the RPC still returning
	// OK. Go's default is a goroutine crash; recover here so contract
	// tests under fault injection see the same shape.
	defer func() {
		if p := recover(); p != nil {
			outcome = "error"
			resp = &pb.GetReceiptStatusResponse{
				Status: pb.ReceiptStatus_RECEIPT_STATUS_UNKNOWN,
				Error:  fmt.Sprintf("panic: %v", p),
			}
			err = nil
		}
	}()

	tenantID := req.GetContext().GetTenantId()

	// Ingress gate — sharding ownership, tenant existence, region
	// pinning. This is the ONLY place we may return a gRPC error code
	// (UNAVAILABLE / FAILED_PRECONDITION / NotFound / InvalidArgument).
	// Match Python parity: api/grpc_server.py:946-971 calls
	// `_check_tenant` first and lets its grpc.aio.AbortError propagate.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// `req.context.actor` is read off the wire but deliberately NOT
	// used for authorization or logging — see spec, "Auth" section
	// (trusted-actor invariant).

	applied, ierr := s.store.CheckIdempotency(ctx, tenantID, req.GetIdempotencyKey())
	if ierr != nil {
		// Application-level fault. Python collapses this to UNKNOWN +
		// error (grpc_server.py:966); do NOT upgrade to codes.Internal
		// — contract tests assert OK + UNKNOWN body.
		outcome = "error"
		return &pb.GetReceiptStatusResponse{
			Status: pb.ReceiptStatus_RECEIPT_STATUS_UNKNOWN,
			Error:  ierr.Error(),
		}, nil
	}

	status := pb.ReceiptStatus_RECEIPT_STATUS_PENDING
	if applied {
		status = pb.ReceiptStatus_RECEIPT_STATUS_APPLIED
	}
	return &pb.GetReceiptStatusResponse{Status: status}, nil
}
