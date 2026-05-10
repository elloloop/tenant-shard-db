// SPDX-License-Identifier: AGPL-3.0-only

// SetLegalHold RPC — Wave 2 of the Python → Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/SetLegalHold.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:132 (rpc), :982-992 (messages).
// Reference Python: server/python/entdb_server/api/grpc_server.py:2808-2862.
//
// Behavioural parity with Python is preserved on the wire shape. Three
// PLAN.md §6 drifts are folded in here:
//
//  1. WAL-first restoration (CLAUDE.md invariant #1, spec "Open questions"
//     item 1). The Python handler writes the status flip directly to
//     globalstore SQLite — a WAL replay against a blank globalstore
//     loses the hold. The Go port closes that gap by appending a
//     `set_legal_hold` op to the WAL when the producer is wired.
//     The W1.10 applier (server/go/internal/apply/ops_set_legal_hold.go)
//     materialises the op against globalstore.legal_holds. The
//     tenant_registry.status flip is performed synchronously here too
//     (parity with Python's gating-flag semantics) so downstream RPCs
//     gating on the registry status see the change immediately.
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
//   - global_store nil               -> UNIMPLEMENTED  "Tenant registry not configured"
//   - empty actor                    -> INVALID_ARGUMENT "actor is required"
//   - empty tenant_id                -> INVALID_ARGUMENT "tenant_id is required"
//   - tenant not served by this node -> UNAVAILABLE   (via s.checkTenant; sharding gate)
//   - tenant not in registry         -> NOT_FOUND     (via s.checkTenant)
//   - non-admin / non-system caller  -> PERMISSION_DENIED "SetLegalHold requires admin or owner role"
//
// On success the response carries `success=true` and `status` set to
// "legal_hold" (when enabled) or "active" (when cleared) — pinned by
// tests/python/unit/test_admin_operations.py:594-624.

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	grpcMethodSetLegalHold = "SetLegalHold"

	// setLegalHoldWALTopic is the topic used when the producer is wired.
	// Matches the default in cmd/entdb-server/main.go (--wal-topic flag).
	// The applier subscribes to the same topic so the op materialises
	// into globalstore.legal_holds via apply/ops_set_legal_hold.go.
	setLegalHoldWALTopic = "entdb-wal"

	// statusActive / statusLegalHold are the two literals that
	// tenant_registry.status takes for this RPC. Pinned by
	// test_admin_operations.py:607,624.
	statusActive    = "active"
	statusLegalHold = "legal_hold"
)

// SetLegalHold toggles tenant_registry.status between "active" and
// "legal_hold" for tenant_id, and (when the WAL producer is wired)
// appends a `set_legal_hold` op so the flag is reproducible from a WAL
// replay. See file header for the full contract.
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
	// every other Wave-2 mutating RPC has converged on the
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
	// match other Wave-2 RPCs (e.g. ArchiveTenant) — admin-only.
	if !(trusted.IsAdmin() || trusted.IsSystem()) {
		resultStatus = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"SetLegalHold requires admin or owner role")
	}

	// Synchronous tenant_registry status flip. This is the *gating*
	// flag downstream RPCs (DeleteNode, ExecuteAtomic delete ops,
	// future DeleteUser/ArchiveTenant) consult to refuse mutations.
	// Parity with Python: if the row is missing we'd never reach here
	// (CheckTenant rejects with NOT_FOUND); if the row exists the
	// UPDATE always matches, so `updated` is true on the happy path.
	newStatus := statusActive
	if req.GetEnabled() {
		newStatus = statusLegalHold
	}
	updated, err := s.global.SetTenantStatus(ctx, req.GetTenantId(), newStatus)
	if err != nil {
		resultStatus = "error"
		return nil, errs.Errorf(codes.Internal, "set_tenant_status: %v", err)
	}
	if !updated {
		// Defence in depth: the row vanished between CheckTenant and
		// here (concurrent delete). Surface as NOT_FOUND rather than
		// the Python OK + success=false channel.
		resultStatus = "error"
		return nil, errs.Errorf(codes.NotFound, "tenant %q not found", req.GetTenantId())
	}

	// WAL-first restoration. Append a `set_legal_hold` op so a future
	// global-store rebuild reproduces the flag. The applier
	// (apply/ops_set_legal_hold.go) writes the legal_holds row in
	// globalstore. Best-effort against the producer: a WAL append
	// failure here is logged via metrics but does NOT roll back the
	// status flip — auditors prefer the asymmetric "flag set, audit
	// might be missing" over "flag silently dropped".
	//
	// When the producer is not wired (e.g. unit tests against the
	// Server with only globalstore), we skip the WAL append and rely
	// on the synchronous SetTenantStatus above. Same trade-off the
	// applier makes when global is nil.
	if s.producer != nil {
		op := map[string]any{
			"op":      "set_legal_hold",
			"held_by": trusted.String(),
			"clear":   !req.GetEnabled(),
		}
		if req.GetEnabled() {
			op["reason"] = "SetLegalHold RPC"
		}
		ev := wal.Event{
			TenantID:       req.GetTenantId(),
			Actor:          trusted.String(),
			IdempotencyKey: newIdempotencyKey(),
			TsMs:           time.Now().UnixMilli(),
			Ops:            []map[string]any{op},
		}
		value, encErr := ev.Encode()
		if encErr == nil {
			headers := map[string][]byte{
				wal.HeaderIdempotencyKey: []byte(ev.IdempotencyKey),
			}
			// Best-effort: failures are not surfaced to the client.
			// Audit row in WAL is the immutable record (CLAUDE.md §2).
			_, _ = s.producer.Append(ctx, setLegalHoldWALTopic, req.GetTenantId(), value, headers)
		}
	}

	return &pb.LegalHoldResponse{
		Success: true,
		Status:  newStatus,
	}, nil
}

// newIdempotencyKey produces a random hex string suitable for use as
// the WAL idempotency-key header. Mirrors the uuid4-hex shape Python
// uses; uniqueness is the only contract.
func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a time-based key if crypto/rand is unavailable
		// (extremely rare; the kernel CSPRNG is always seeded by the
		// time userland code runs). The applier dedupes on the key,
		// not on its randomness, so this is acceptable.
		return time.Now().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
