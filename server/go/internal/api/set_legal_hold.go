// SPDX-License-Identifier: AGPL-3.0-only

// SetLegalHold RPC.
// Spec: docs/go-port/rpcs/SetLegalHold.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:132 (rpc), :982-992 (messages).
//
// Behavioural parity with Python is preserved on the wire shape. Three
// PLAN.md §6 drifts are folded in here:
//
//  1. WAL-first restoration (CLAUDE.md invariant #1, spec "Open questions"
//     item 1). The Python handler writes the status flip directly to
//     globalstore SQLite — a WAL replay against a blank globalstore
//     loses the hold. The Go port closes that gap by appending a
//     `legal_hold_set` op to the global WAL scope. The applier
//     materialises the op against globalstore.legal_holds and flips
//     tenant_registry.status before the handler returns.
//
//  2. Compliance-officer trusted-actor gate (spec §"Auth"). Today only
//     admin: / system: actors may set or clear a hold. The "compliance
//     officer" role flagged in spec "Open questions" item 5 is not
//     wired yet — admin/system stand in for it. Wire `actor` is
//     UNTRUSTED and rebound via auth.Authoritative before any privilege
//     decision (privilege-escalation remediation, commit fece3fb).
//
//  3. Downstream enforcement is left to the consuming RPCs (DeleteUser
//     refuses to queue erasure when any of the user's tenants is on
//     legal_hold; ArchiveTenant must check the hold before flipping
//     status — spec §"Other-RPC deps"). This handler only sets / clears
//     the flag and writes the WAL audit record.
//
// Error contract (spec §"Error contract"):
//
//   - global_store nil -> UNIMPLEMENTED "Tenant registry not configured"
//   - empty actor -> INVALID_ARGUMENT "actor is required"
//   - empty tenant_id -> INVALID_ARGUMENT "tenant_id is required"
//   - tenant not served by this node -> UNAVAILABLE (via s.checkTenant; sharding gate)
//   - tenant not in registry -> NOT_FOUND (via s.checkTenant)
//   - non-admin / non-system caller -> PERMISSION_DENIED "SetLegalHold requires admin or owner role"
//
// On success the response carries `success=true` and `status` set to
// "legal_hold" (when enabled) or "active" (when cleared) — pinned by
// tests/python/unit/test_admin_operations.py:594-624.

package api

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const (
	grpcMethodSetLegalHold = "SetLegalHold"

	// statusActive / statusLegalHold are the two literals that
	// tenant_registry.status takes for this RPC. Pinned by
	// test_admin_operations.py:607,624.
	statusActive    = "active"
	statusLegalHold = "legal_hold"
)

// SetLegalHold toggles tenant_registry.status between "active" and
// "legal_hold" for tenant_id by appending a global `legal_hold_set`
// WAL op. See file header for the full contract.
func (s *Server) SetLegalHold(
	ctx context.Context,
	req *pb.LegalHoldRequest,
) (*pb.LegalHoldResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(grpcMethodSetLegalHold, resultStatus, time.Since(start))
	}()

	// Optional-store guard. Mirrors Python `:2816-2820` which aborts with
	// UNIMPLEMENTED when the global_store dep is not configured. This
	// runs BEFORE the auth check (spec §"Auth": ordering preserved).
	if s.global == nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// Required-arg validation. Empty actor / tenant_id surface as
	// INVALID_ARGUMENT before any privilege decision (parity with
	// `:2821-2822`). The wire `actor` is required even though we rebind
	// it below — the wire contract pin is preserved (test_grpc_contract.py:579-583).
	if req.GetActor() == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetTenantId() == "" {
		resultStatus = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}

	// Tenant gate: sharding ownership + registry existence + region pin.
	// CheckTenant returns NOT_FOUND when the tenant is missing from
	// tenant_registry. This is a Go-port tightening over the Python
	// handler (which returns OK + success=false + error="Tenant not
	// found"); the Python asymmetry is hostile to SDK callers and
	// every other mutating RPC has converged on the
	// status-code form.
	if err := s.checkTenant(ctx, req.GetTenantId()); err != nil {
		resultStatus = "error"
		return nil, err
	}

	// Trusted-actor rebind. Wire `actor` is UNTRUSTED — rebind to the
	// interceptor-bound identity before any authz decision. In
	// no-auth deployments / unit tests with no Identity on ctx, the
	// claimed actor is returned as-is (auth.Authoritative documented
	// fallback) — matching the Python ContextVar fall-through.
	claimed := auth.ParseActor(req.GetActor())
	trusted := auth.Authoritative(ctx, claimed)

	// Compliance-officer gate. Today admin: / system: stand in for
	// the not-yet-wired "compliance officer" role (spec "Open questions"
	// item 5). The owner branch in the Python handler is deferred to
	// match other RPCs (e.g. ArchiveTenant) — admin-only.
	if !(trusted.IsAdmin() || trusted.IsSystem()) {
		resultStatus = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"SetLegalHold requires admin or owner role")
	}

	newStatus := statusActive
	if req.GetEnabled() {
		newStatus = statusLegalHold
	}

	reason := ""
	if req.GetEnabled() {
		reason = "SetLegalHold RPC"
	}
	_, _, err := s.appendGlobalAdminOp(ctx, trusted.String(), map[string]any{
		"op":         string(apply.OpLegalHoldSet),
		"tenant_id":  req.GetTenantId(),
		"held_by":    trusted.String(),
		"reason":     reason,
		"enabled":    req.GetEnabled(),
		"created_at": time.Now().Unix(),
	})
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	// EPIC #511 Gap 1: on an *explicit* release (enabled=false) the S3
	// Object Lock legal hold on the tenant's already-archived objects
	// must be lifted. That is NOT done from this RPC path: the
	// appendGlobalAdminOp above has already driven the durable global
	// apply step (apply/global.go -> globalstore.ApplyLegalHoldSet) which,
	// in the SAME transaction that clears the legal_holds row + flips
	// tenant_registry.status, upserts a legal_hold_lift_queue row. A
	// crash-durable background worker (internal/audit.LiftWorker, wired
	// in cmd/entdb-server/main.go) drains that queue, running the
	// idempotent/resumable paginated sweep and retrying until complete.
	//
	// This replaces the previous detached fire-and-forget goroutine,
	// which left a released tenant's objects stuck LegalHold=ON with no
	// retry and no signal after a restart / S3 outage / timeout —
	// unacceptable for a compliance feature (GDPR
	// right-to-erasure-after-release would be silently unfulfillable).
	// The OFF RPC now just causes the durable enqueue (via the apply
	// path) and returns; the worker does the rest. GDPR erasure never
	// lifts a hold — only an explicit release emits enabled=false (legal
	// hold supersedes erasure; DeleteUser still refuses to queue while
	// held). COMPLIANCE retention is never touched by the sweep.

	return &pb.LegalHoldResponse{
		Success: true,
		Status:  newStatus,
	}, nil
}

// newIdempotencyKey lives in helpers.go (consolidated in the round-3
//  dedupe). Its signature is now (string, error); the
// time-based fallback this file previously used has been dropped — a
// crypto/rand failure is vanishingly rare and the best-effort WAL
// append above simply skips the audit row in that case.
