// ExportUserData RPC.
//
// Spec: docs/go-port/rpcs/ExportUserData.md (GDPR Article 20 portability).
//
// Behavioral contract:
//
//   - global_store == nil → codes.Unimplemented "User registry not configured".
//   - actor / user_id required (codes.InvalidArgument), in that
//     validation order.
//   - Trusted-actor pattern: the privilege decision uses
//     auth.Authoritative(ctx, claimed) — the wire-supplied actor is
//     IGNORED for the self-or-admin gate. Allowed identities: the user
//     themselves, or an admin: / system: caller. Anyone else →
//     codes.PermissionDenied.
//   - Cross-tenant walk: enumerate via globalstore.GetUserTenants and
//     query each tenant's SQLite with its own connection — never a
//     cross-tenant transaction (CLAUDE.md invariant #4).
//   - Per-tenant collector failures are SWALLOWED (logged, tenant
//     omitted from `tenants[]`) so one corrupt tenant cannot block the
//     bundle.
//   - Bundle shape: {"user_id", "generated_at", "tenants": [{"tenant_id",
//     "nodes": [{"tenant_id","node_id","type_id","payload","created_at",
//     "updated_at","owner_actor","data_policy"}]}]}.
//   - `payload` stays field-id-keyed (CLAUDE.md invariant #6) — the
//     handler does not translate to field names.
//   - Read-only RPC: no WAL append, no SQLite writes. Python's open
//     question #3 (audit-record the export itself via WAL) is left as
//     a follow-up — preserving current behavior keeps parity tight.
//   - Streaming export is deferred to Phase 2 (open question #1 in the
//     spec): the bundle is built in-memory and returned inline as
//     `export_json`. A 10M-node user will OOM the server today; the
//     port matches that pathology rather than silently changing the
//     wire shape.
//   - Go-port delta vs Python: aborts use real status.Errorf returns
//     rather than the Python try/except envelope. The
//     `success=false, error=…` in-band branch only fires for genuine
//     internal errors (json.Marshal of the bundle), matching the spec's
//     "real aborts" note.
//
// Metrics: emits entdb_grpc_requests_total{method="ExportUserData",
// status="ok"|"error"} via the shared chokepoint, on every code path
// (including aborts) — matches the Python handler which always records
// before re-raising.

package api

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const exportUserDataMethod = "ExportUserData"

// ExportUserData implements entdb.v1.EntDBService/ExportUserData.
//
// See file header for the full semantic contract. The handler is a
// fan-in over per-tenant SQLite reads — each tenant is queried with its
// own connection so the cross-tenant invariant (#4) is never violated.
func (s *Server) ExportUserData(
	ctx context.Context,
	req *pb.ExportUserDataRequest,
) (*pb.ExportUserDataResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(exportUserDataMethod, outcome, time.Since(start))
	}()

	// Configuration gate.
	if s.global == nil {
		outcome = "error"
		return nil, status.Error(codes.Unimplemented, "User registry not configured")
	}
	// Required-field aborts.
	if req.GetActor() == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "actor is required")
	}
	userID := req.GetUserId()
	if userID == "" {
		outcome = "error"
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	// Trusted-actor resolution. The wire `actor` is UNTRUSTED — the
	// interceptor installs the verified Identity on ctx and
	// auth.Authoritative collapses to the trusted Actor when one is
	// present.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))
	if !isSelfOrAdmin(trusted, userID) {
		outcome = "error"
		return nil, status.Error(codes.PermissionDenied,
			"ExportUserData requires the user themselves or an admin actor")
	}

	memberships, err := s.global.GetUserTenants(ctx, userID)
	if err != nil {
		// Returns success=false in-band on membership lookup failure.
		// Preserve the wire shape — the SDK pins it.
		outcome = "error"
		return &pb.ExportUserDataResponse{Success: false, Error: err.Error()}, nil
	}

	// Per-tenant nodes store owner_actor as the `user:<id>` principal.
	principal := userID
	if !strings.HasPrefix(principal, "user:") {
		principal = "user:" + userID
	}

	tenants := make([]store.TenantExport, 0, len(memberships))
	for _, m := range memberships {
		// Per-tenant exception swallow — one corrupted DB must not fail
		// the whole bundle.
		export, perr := s.store.ExportUserData(ctx, m.TenantID, principal, s.registry)
		if perr != nil {
			log.Printf("ExportUserData tenant %q failed: %v", m.TenantID, perr)
			continue
		}
		tenants = append(tenants, export)
	}

	// Bundle shape: `tenants` is always a list (never null) so the JSON
	// shape matches the empty-path `[]` contract.
	bundle := map[string]any{
		"user_id":      userID,
		"generated_at": time.Now().Unix(),
		"tenants":      tenants,
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		// Genuine internal error — the only Marshal failure mode that
		// can fire here is an unsupported type smuggled through a
		// payload, which the canonical store has already validated. Use
		// the in-band failure shape (success=false, error=…).
		outcome = "error"
		return &pb.ExportUserDataResponse{Success: false, Error: err.Error()}, nil
	}
	return &pb.ExportUserDataResponse{
		Success:    true,
		ExportJson: string(raw),
	}, nil
}
