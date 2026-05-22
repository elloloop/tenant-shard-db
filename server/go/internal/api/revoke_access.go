// SPDX-License-Identifier: AGPL-3.0-only

// RevokeAccess RPC.
// Spec: docs/go-port/rpcs/RevokeAccess.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:97 (rpc), :746-755 (messages).
//
// # WAL-first restoration (PLAN.md §6.1, fixed on port)
//
// The Go port appends a "revoke_access" op to the WAL; the applier
// handler in apply/ops_revoke_access.go performs the DELETE (and the
// cross-tenant shared_index cleanup hook). See spec §"Side effects" and
// §"Open questions" item 1.
//
// # Auth model — trusted-actor, ADMIN capability
//
//  1. Authentication is required (handler is NOT in
//     AuthInterceptor.UNAUTHENTICATED_METHODS).
//  2. The wire-claimed actor is UNTRUSTED — auth.Authoritative rebinds
//     to the interceptor-attested identity. Privilege-escalation
//     regression pinned by commit fece3fb.
//  3. Authorization: trusted actor must be either
//      a. system: / admin: prefixed (server-side actor), OR
//      b. an "owner" or "admin" member of the tenant per
//         globalstore.tenant_members.
//     The Go port narrows the per-node ADMIN-grant satisfaction to
//     membership-based admin for now; the per-node grant satisfaction
//     can land alongside the full capability registry port.
//
// # Tenant gate
//
// Calls s.checkTenant before authorization to surface NOT_FOUND /
// FAILED_PRECONDITION / UNAVAILABLE-with-redirect on missing or
// mis-routed tenants — same gate every handler uses.
//
// # Error contract
//
// Errors come back as RevokeAccessResponse{Found:false, Error:<string>}
// on the response, NOT as a gRPC status error. Three observable paths:
//
//	PermissionDenied Found=false, Error="permission denied: ..."
//	WAL/append failure Found=false, Error=<err>
//	Happy path Found=true, Error=""
//
// On the happy path, Found=true means "the revoke intent has been
// durably recorded in the WAL". The applier will perform the DELETE
// asynchronously; idempotent revoke-not-granted is therefore a no-op
// at apply time but still success on the wire. This matches the spec's
// "fix on port" direction (PLAN.md §6.1) — the wire contract returns
// success for both first-revoke and revoke-not-granted, and the
// applier is responsible for making both converge to the same SQLite
// state.
//
// Tenant-gate errors (NOT_FOUND / FAILED_PRECONDITION / UNAVAILABLE)
// propagate as gRPC status errors. Required-arg validation is
// INVALID_ARGUMENT. These are the only paths that surface as gRPC
// status errors; all others use the response-error convention.
//
// # Metrics
//
// Emits entdb_grpc_requests_total{method="RevokeAccess",
// status="ok"|"denied"|"error"} via the shared metrics chokepoint.
// Labels: "denied" specifically for PermissionDenied, "error" for any
// other failure, "ok" for the happy path including idempotent
// revoke-not-granted.

package api

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	revokeAccessMethod = "RevokeAccess"

	// revokeAccessTopic is the WAL topic the handler appends to. Mirrors
	// the default in cmd/entdb-server/main.go (--wal-topic). Hardcoded
	// here so the handler does not need a server-level Option for the
	// topic; the applier reads from the same topic per main.go wiring.
	revokeAccessTopic = "entdb-wal"
)

// RevokeAccess removes a single direct grant on a node for one actor.
// See file header for the full contract.
func (s *Server) RevokeAccess(
	ctx context.Context,
	req *pb.RevokeAccessRequest,
) (*pb.RevokeAccessResponse, error) {
	start := time.Now()
	statusLabel := "ok"
	defer func() {
		metrics.RecordGRPCRequest(revokeAccessMethod, statusLabel, time.Since(start))
	}()

	// Required-arg validation. Empty actor / tenant_id / node_id /
	// actor_id surface as INVALID_ARGUMENT before any privilege or
	// tenant-gate work.
	rctx := req.GetContext()
	if rctx == nil || rctx.GetTenantId() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if rctx.GetActor() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetNodeId() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "node_id is required")
	}
	if req.GetActorId() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor_id is required")
	}

	// Tenant gate: NOT_FOUND on missing tenant, FAILED_PRECONDITION on
	// region mismatch, UNAVAILABLE-with-redirect on shard mis-route.
	// Errors propagate as gRPC status errors per the spec (the
	// response-error convention only applies to authz / apply paths).
	if cerr := s.checkTenant(ctx, rctx.GetTenantId()); cerr != nil {
		statusLabel = "error"
		return nil, cerr
	}

	// Trusted-actor rebind. Privilege checks below MUST consult
	// `trusted` (never req.Context.Actor); honouring the request payload
	// is the privilege-escalation hole fixed by commit fece3fb.
	trusted := auth.Authoritative(ctx, auth.ParseActor(rctx.GetActor()))

	// Admin / system bypass + tenant-membership admin path. Narrow to
	// membership-based admin/owner for now; the per-node ADMIN grant
	// satisfaction lands with the full capability registry port
	// (spec §"Auth" item 3).
	if !(trusted.IsAdmin() || trusted.IsSystem()) {
		role := ""
		if s.global != nil {
			r, err := s.lookupMemberRole(ctx, rctx.GetTenantId(), trusted.ID())
			if err != nil {
				statusLabel = "error"
				return &pb.RevokeAccessResponse{
					Found: false,
					Error: err.Error(),
				}, nil
			}
			role = r
		}
		if role != "owner" && role != "admin" {
			statusLabel = "denied"
			return &pb.RevokeAccessResponse{
				Found: false,
				Error: "permission denied: RevokeAccess requires ADMIN capability",
			}, nil
		}
	}

	// WAL append. The applier (apply/ops_revoke_access.go) performs the
	// per-tenant SQLite DELETE on node_access and the cross-tenant
	// shared_index cleanup. Idempotency: deterministic key per
	// (tenant, node, actor) so retries collapse to a single WAL record
	// and the applier sees one apply. Revoke-not-granted is naturally
	// no-op at apply time (0 rows affected) — which the spec's
	// idempotency contract requires (spec §"Open questions" item 2).
	if s.producer == nil {
		statusLabel = "error"
		return &pb.RevokeAccessResponse{
			Found: false,
			Error: "wal producer not configured",
		}, nil
	}

	idempKey := fmt.Sprintf("revoke_access:%s:%s:%s",
		rctx.GetTenantId(), req.GetNodeId(), req.GetActorId())
	ev := wal.Event{
		TenantID:       rctx.GetTenantId(),
		Actor:          trusted.String(),
		IdempotencyKey: idempKey,
		Ops: []map[string]any{{
			"op":       "revoke_access",
			"node_id":  req.GetNodeId(),
			"actor_id": req.GetActorId(),
		}},
	}
	value, err := ev.Encode()
	if err != nil {
		statusLabel = "error"
		return &pb.RevokeAccessResponse{
			Found: false,
			Error: fmt.Sprintf("encode event: %v", err),
		}, nil
	}
	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(idempKey),
	}
	if _, err := s.producer.Append(ctx, revokeAccessTopic, ev.TenantID, value, headers); err != nil {
		statusLabel = "error"
		return &pb.RevokeAccessResponse{
			Found: false,
			Error: fmt.Sprintf("wal append: %v", err),
		}, nil
	}

	// Happy path. Found=true means the revoke intent has been durably
	// recorded; the applier converges SQLite state asynchronously. This
	// covers both first-revoke and revoke-not-granted — the latter
	// applies as a 0-row DELETE, which is a normal no-op apply (spec
	// §"Open questions" item 2).
	return &pb.RevokeAccessResponse{Found: true}, nil
}
