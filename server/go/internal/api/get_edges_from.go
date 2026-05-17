// GetEdgesFrom RPC.
//
// Spec: docs/go-port/rpcs/GetEdgesFrom.md. Pure read of the per-tenant
// `edges` table indexed by (tenant_id, from_node_id), translated into
// the wire `Edge` shape.
//
// Semantics:
//
//   - Tenant gate (s.checkTenant) is the only ingress check. Its
//     UNAVAILABLE / FAILED_PRECONDITION / NotFound / InvalidArgument
//     errors propagate to the SDK so the redirect trailer
//     (`entdb-redirect-node`) lands before the status closes.
//   - request.context.actor is UNTRUSTED; resolve trusted identity via
//     auth.Authoritative (CLAUDE.md / commit fece3fb). The trusted
//     actor is NOT used for any authorization decision today — see the
//     ACL parity gap below.
//   - PARITY GAP (deliberate, pinned): no per-destination ACL filter.
//     Python's handler does NOT call the visibility check before
//     fan-out, so callers can enumerate `to_node_id`s they would not
//     have READ on through GetNode. The privilege-escalation test at
//     tests/python/integration/test_privilege_escalation.py:421-447
//     pins this weaker property; we match byte-for-byte. Tracked as a
//     follow-up: tightening this requires a contract change because
//     existing SDK clients depend on the full edge fan-out shape.
//   - `offset` is declared on GetEdgesRequest (proto field 5) but the
//     Python handler ignores it; we ignore it too. Switching to honour
//     it must update the contract in lock-step.
//   - `edge_type_id == 0` means "no filter" (parity with the falsy
//     check at grpc_server.py:1393).
//   - `limit <= 0` defaults to 100. We pass 0 (no limit) to the store
//     so the slice + has_more accounting happens in this handler — the
//     Python side does the same in-process slice
//     (grpc_server.py:1410-1414). NOT in SQL: ordering is not
//     contractually pinned and adding ORDER BY here would diverge from
//     Python.
//   - Side effects: none. No WAL append, no SQLite write, no metric
//     other than the standard request counter.
//   - Error contract: any failure below the tenant gate is swallowed
//     into an empty GetEdgesResponse with codes.OK
//     (grpc_server.py:1415-1418). The metric outcome is "error" so
//     swallowed faults don't inflate the ok counter.

package api

import (
	"context"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const grpcMethodGetEdgesFrom = "GetEdgesFrom"

// defaultEdgesLimit mirrors grpc_server.py:1400 where `limit or 100`
// substitutes 100 for any falsy value (0 / unset / negative).
const defaultEdgesLimit = 100

// GetEdgesFrom implements entdb.v1.EntDBService/GetEdgesFrom. See file
// header for the contract; the body is intentionally a thin translation
// over CanonicalStore.GetEdgesFrom.
func (s *Server) GetEdgesFrom(ctx context.Context, req *pb.GetEdgesRequest) (*pb.GetEdgesResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(grpcMethodGetEdgesFrom, outcome, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()

	// Tenant gate — sharding ownership + region pinning. The redirect
	// trailer is set inside CheckTenant; the Go SDK redirect cache reads
	// it (sdk/go/entdb/redirect_cache.go).
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// Resolve trusted actor. The result is intentionally unused: this
	// RPC has no authorization decision today (see file-header parity
	// gap). The call exists so that the future tightening to ACL-filter
	// destinations has a single chokepoint to bind against and so we
	// honour the trusted-actor invariant explicitly.
	_ = auth.Authoritative(ctx, auth.ParseActor(req.GetContext().GetActor()))

	// edge_type_id filter: falsy (== 0) means "no filter" per
	// grpc_server.py:1393.
	var edgeTypeFilter *int32
	if etid := req.GetEdgeTypeId(); etid != 0 {
		v := etid
		edgeTypeFilter = &v
	}

	// Pass limit=0 to the store so it returns the full result set; we
	// slice + compute has_more here for parity with Python.
	edges, err := s.store.GetEdgesFrom(ctx, tenantID, req.GetNodeId(), edgeTypeFilter, 0)
	if err != nil {
		// Mirror Python's bare `except` swallow at grpc_server.py:1415.
		outcome = "error"
		return &pb.GetEdgesResponse{Edges: []*pb.Edge{}}, nil
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = defaultEdgesLimit
	}
	hasMore := len(edges) > limit
	if hasMore {
		edges = edges[:limit]
	}

	out := make([]*pb.Edge, 0, len(edges))
	for _, e := range edges {
		out = append(out, edgeToProto(e))
	}
	return &pb.GetEdgesResponse{Edges: out, HasMore: hasMore}, nil
}

// edgeToProto is shared with GetEdgesTo and lives in helpers.go (after
// the round-3 dedupe). The defensive empty-Struct shape is
// preserved there; behaviour is otherwise identical.
