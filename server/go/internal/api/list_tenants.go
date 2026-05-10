// ListTenants RPC — Wave 2 of the Python → Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/ListTenants.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:82 (rpc), :611-619 (messages).
// Reference Python: server/python/entdb_server/api/grpc_server.py:1537-1603.
//
// Identity-driven, request-empty handler:
//
//   - Request carries no actor field — there is nothing on the wire to
//     validate. The "trusted-actor" rule has the trusted Identity
//     established by the auth interceptor as its sole input. Pinned by
//     tests/python/integration/test_privilege_escalation.py:386-417
//     (claimed admin actor in metadata MUST NOT bypass membership
//     filtering for user:eve).
//
//   - Visibility classes:
//
//   - No trusted Identity on ctx (interceptor missing) →
//     PERMISSION_DENIED, never falls open. Pinned by
//     tests/python/unit/test_listtenants_auth.py:179-192 and
//     tests/python/integration/test_grpc_contract.py:288-295.
//
//   - "__system__", "system:*", "admin:*" → all tenants this node
//     hosts (sharding still applied). Pinned by
//     test_listtenants_auth.py:99-111 and :200-230.
//
//   - "user:<id>" or any other bare id → intersection of node-local
//     tenants ∩ globalstore.GetUserTenants(<id>). Strip the "user:"
//     prefix before the lookup. Pinned by
//     test_listtenants_auth.py:119-137 and :157-171.
//
//   - Read-only. No WAL append, no canonical_store write, no
//     globalstore write.
//
//   - Swallow-as-empty. Any unhandled error path inside the handler
//     returns &ListTenantsResponse{} with grpc.OK and metric
//     status="error", matching the Python `except Exception` at
//     grpc_server.py:1600-1603. Returning codes.Internal would be a
//     contract break. PERMISSION_DENIED is the *intended* error path
//     and MUST escape the swallow.
//
// Source-of-truth: the Go port reads its tenant inventory from
// globalstore.ListTenants("active") (the registry table) rather than
// the Python directory scan over `data_dir/tenant_*.db`. The two land
// on the same set in production deployments because tenant creation
// always inserts into the registry first; the empty-globalstore branch
// from Python (test harness with no globalstore) is preserved by the
// nil-globalstore short-circuit below.

package api

import (
	"context"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const listTenantsMethod = "ListTenants"

// ListTenants implements entdb.v1.EntDBService/ListTenants. See file
// header for the identity-driven visibility rules and swallow-as-empty
// error contract.
func (s *Server) ListTenants(
	ctx context.Context,
	_ *pb.ListTenantsRequest,
) (resp *pb.ListTenantsResponse, err error) {
	start := time.Now()
	stat := "ok"
	defer func() {
		metrics.RecordGRPCRequest(listTenantsMethod, stat, time.Since(start))
	}()

	// Swallow panics → empty + OK. PERMISSION_DENIED returns through
	// the explicit `return` below, NOT via panic, so it escapes this
	// recover unchanged. Mirrors Python's outer except at
	// grpc_server.py:1600-1603.
	defer func() {
		if r := recover(); r != nil {
			stat = "error"
			resp = &pb.ListTenantsResponse{Tenants: []*pb.TenantInfo{}}
			err = nil
			_ = r
		}
	}()

	// 1. Trusted identity is required. Nil → PERMISSION_DENIED.
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id.Subject == "" {
		stat = "error"
		return nil, status.Errorf(codes.PermissionDenied,
			"ListTenants requires an authenticated caller")
	}
	trusted := id.Subject

	// 2. Admin classification — same prefix scheme the Python handler
	//    uses at grpc_server.py:1568-1572.
	isAdmin := trusted == "__system__" ||
		strings.HasPrefix(trusted, "system:") ||
		strings.HasPrefix(trusted, "admin:")

	// 3. Node-local tenant inventory. Source of truth in the Go port
	//    is the globalstore tenant_registry; if it isn't wired, the
	//    embedded-harness branch below returns empty for non-admin
	//    callers (matches test_listtenants_auth.py:238-258).
	all, lerr := s.listLocalTenantIDs(ctx)
	if lerr != nil {
		// Swallow → empty + OK. Matches Python's `except Exception`
		// at grpc_server.py:1600-1603.
		stat = "error"
		return &pb.ListTenantsResponse{Tenants: []*pb.TenantInfo{}}, nil
	}

	// 4. Sharding filter. nil sharding == single-node default; nothing
	//    to strip. Matches grpc_server.py:1575-1576.
	if s.sharding != nil && s.sharding.IsMine != nil {
		filtered := all[:0]
		for _, tid := range all {
			if s.sharding.IsMine(tid) {
				filtered = append(filtered, tid)
			}
		}
		all = filtered
	}

	// 5. Visibility intersection.
	var visible []string
	switch {
	case isAdmin:
		visible = all
	case s.global != nil:
		userID := strings.TrimPrefix(trusted, "user:")
		members, gerr := s.global.GetUserTenants(ctx, userID)
		if gerr != nil {
			stat = "error"
			return &pb.ListTenantsResponse{Tenants: []*pb.TenantInfo{}}, nil
		}
		set := make(map[string]struct{}, len(members))
		for _, m := range members {
			set[m.TenantID] = struct{}{}
		}
		visible = make([]string, 0, len(all))
		for _, tid := range all {
			if _, ok := set[tid]; ok {
				visible = append(visible, tid)
			}
		}
	default:
		// No globalstore wired (embedded harness) — empty, not all.
		// Matches grpc_server.py:1588-1591.
		visible = []string{}
	}

	// Stable ascending order. Spec note: Python returns sorted glob
	// order; explicit sort.Strings defends against future drift.
	sort.Strings(visible)

	out := make([]*pb.TenantInfo, 0, len(visible))
	for _, tid := range visible {
		out = append(out, &pb.TenantInfo{TenantId: tid})
	}
	return &pb.ListTenantsResponse{Tenants: out}, nil
}

// listLocalTenantIDs returns the node-local tenant inventory as a flat
// slice of tenant_ids. Source: globalstore.ListTenants("active"). When
// no globalstore is wired (Wave-1 bring-up / embedded harness), the
// inventory is empty — admins see [], regular users see [] via the
// nil-globalstore branch in the caller.
func (s *Server) listLocalTenantIDs(ctx context.Context) ([]string, error) {
	if s.global == nil {
		return []string{}, nil
	}
	tenants, err := s.global.ListTenants(ctx, "active")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(tenants))
	for _, t := range tenants {
		ids = append(ids, t.TenantID)
	}
	return ids, nil
}
