// UpdateUser RPC — Wave 2 of the Python → Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/UpdateUser.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:114 (rpc), :826-838 (request/
// response). Reference Python:
// server/python/entdb_server/api/grpc_server.py:2184-2229 (handler) and
// server/python/entdb_server/global_store.py:386-409 (backing store).
//
// Semantics (preserved byte-for-byte from the Python handler):
//
//   - Globalstore must be configured. If not, abort with
//     codes.Unimplemented "User registry not configured" — the same
//     gate Python applies at grpc_server.py:2192-2196.
//   - actor and user_id are required (codes.InvalidArgument).
//   - Trusted-actor pattern: the privilege decision uses
//     auth.Authoritative(ctx, claimed) — when the interceptor has
//     installed an Identity on ctx, the request-payload actor is
//     ignored. Self-or-admin gate matches Python's _is_self_or_admin
//     at grpc_server.py:2071-2086.
//   - Truthy-only partial update: empty string == "do not update".
//     There is no FieldMask (the proto doesn't carry presence). If
//     all three mutable fields (email/name/status) are empty, the
//     handler short-circuits and returns success=false,
//     error="No fields to update" — IN-BAND, NOT codes.InvalidArgument
//     (matches grpc_server.py:2216-2218 and the contract pin at
//     test_user_registry.py:330-345).
//   - User-not-found is also reported in-band: success=false,
//     error="User not found" (no NOT_FOUND status).
//   - NO WAL append. The user registry is intentionally not
//     event-sourced today — see CLAUDE.md "Architecture Invariants" #1
//     and the spec's "Side effects" / "Open questions" sections. Do
//     NOT route this RPC through wal.append.
//   - Metrics: emits entdb_grpc_requests_total{method="UpdateUser",
//     status="ok"|"error"} via the shared chokepoint. Note "ok" is
//     recorded for in-band failures (no-fields, not-found) because
//     no abort fires — same as Python (grpc_server.py:2217).

package api

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const updateUserMethod = "UpdateUser"

// UpdateUser implements entdb.v1.EntDBService/UpdateUser.
//
// See file header for the full semantic contract. The handler is
// stateless apart from the globalstore handle: each call resolves the
// trusted actor, gates on self-or-admin, builds a partial update, and
// applies it via globalstore.UpdateUser.
func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(updateUserMethod, outcome, time.Since(start))
	}()

	// Configuration gate. Mirrors Python grpc_server.py:2192-2196.
	if s.global == nil {
		outcome = "error"
		return nil, status.Error(codes.Unimplemented, "User registry not configured")
	}

	// Required-field aborts. Mirrors grpc_server.py:2198-2201.
	if req.GetActor() == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "actor is required")
	}
	userID := req.GetUserId()
	if userID == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Trusted-actor resolution. The request payload is UNTRUSTED — the
	// interceptor installs the verified Identity on ctx and
	// auth.Authoritative ignores the wire claim when one is present.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))
	if !isSelfOrAdmin(trusted, userID) {
		outcome = "error"
		return nil, status.Error(codes.PermissionDenied,
			"UpdateUser requires the user themselves or admin actor")
	}

	// Truthy-only partial update. Empty strings are not forwarded —
	// matches grpc_server.py:2209-2214 and the unit-test pin at
	// test_user_registry.py:255-271 (only `name=` is forwarded when
	// only name is set).
	var upd globalstore.UserUpdates
	if v := req.GetEmail(); v != "" {
		upd.Email = stringPtr(v)
	}
	if v := req.GetName(); v != "" {
		upd.Name = stringPtr(v)
	}
	if v := req.GetStatus(); v != "" {
		upd.Status = stringPtr(v)
	}
	if upd.Email == nil && upd.Name == nil && upd.Status == nil {
		// In-band failure (no abort). Python returns "ok" metric here;
		// we do the same.
		return &pb.UpdateUserResponse{Success: false, Error: "No fields to update"}, nil
	}

	updated, err := s.global.UpdateUser(ctx, userID, upd)
	if err != nil {
		// Catch-all in-band failure mirroring grpc_server.py:2222-2227.
		// Python writes the metric label as "error" on this arm; we
		// match. Spec note: do NOT promote to codes.Internal until the
		// contract tests are loosened — clients pin success/error on
		// the response body, not status codes.
		outcome = "error"
		return &pb.UpdateUserResponse{Success: false, Error: err.Error()}, nil
	}
	if !updated {
		// In-band not-found. Same metric label ("ok") as Python — no
		// abort fires.
		return &pb.UpdateUserResponse{Success: false, Error: "User not found"}, nil
	}
	return &pb.UpdateUserResponse{Success: true}, nil
}

// isSelfOrAdmin mirrors _is_self_or_admin at grpc_server.py:2071-2086.
// Admin/system identities are always allowed; user identities are
// allowed only when their bare ID matches user_id (the "self" arm).
//
// The Python implementation also accepts the literal "__system__"
// trusted string. That identity is the Applier's bootstrap actor and
// never appears on the wire (auth_interceptor.py:111-112), so the Go
// port intentionally does not honour it here.
func isSelfOrAdmin(trusted auth.Actor, userID string) bool {
	if trusted.IsAdmin() || trusted.IsSystem() {
		return true
	}
	if trusted.IsUser() && trusted.ID() == userID {
		return true
	}
	// Defence in depth: Python compares against both `user:<id>` and
	// the bare `<id>`. ParseActor classifies a bare string with no
	// colon as KindUnknown carrying the raw id; honour the bare-id
	// fallback for parity with grpc_server.py:2080.
	if trusted.Kind() == auth.KindUnknown && !strings.Contains(trusted.ID(), ":") && trusted.ID() == userID {
		return true
	}
	return false
}

// stringPtr is a tiny helper so handler bodies stay readable. Using a
// dedicated helper rather than &v inline keeps the truthy-gate above
// the actual struct construction visually distinct.
func stringPtr(s string) *string { return &s }
