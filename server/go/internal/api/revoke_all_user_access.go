// SPDX-License-Identifier: AGPL-3.0-only

// RevokeAllUserAccess RPC.
// Spec: docs/go-port/rpcs/RevokeAllUserAccess.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:133 (rpc), :994-1006 (messages).
//
// # Behavioural pins
//
//   - WAL-FIRST. A single `admin_revoke_access` op is appended to the
//     WAL; the applier (server/go/internal/apply/ops_admin_revoke_access.go)
//     deletes from node_access AND group_users AND node_visibility for
//     the user. PLAN.md §6.4 item 2. Pinned by
//     docs/go-port/rpcs/RevokeAllUserAccess.md "WAL invariant gap".
//
//   - Trusted-actor authz. The wire `actor` field is UNTRUSTED. We
//     rebind to the auth.Authoritative identity from ctx before any
//     privilege decision (CLAUDE.md trusted-actor invariant; see commit
//     fece3fb). Caller must be system:/admin: prefix OR carry the
//     tenant member role of "owner"/"admin"; anything else returns
//     PERMISSION_DENIED.
//
//   - Tenant gate. checkTenant runs first (sharding redirect via the
//     `entdb-redirect-node` trailer when this node does not own the
//     tenant).
//
//   - Validation order. tenant_id non-empty THEN user_id non-empty,
//     before the auth gate. Pinned by
//     tests/python/integration/test_grpc_contract.py:584-596.
//
//   - Tally semantics. The handler reads current row counts BEFORE
//     appending the WAL event, waits for the applier, and returns those
//     pre-append counts as the tallies. Re-running is idempotent (the
//     applier's per-event dedupe via applied_events) and the second call
//     will see zero rows pre-append (no-op on retry).
//
//   - Cross-tenant cleanup (shared_index). The same tenant WAL event
//     carries an `access_revoked` op, applied by the applier after the
//     tenant-scoped revoke op. The returned revoked_shared count is
//     read before append.
//
//   - Idempotency key. Synthesized from (tenant_id, user_id,
//     trusted_actor) so retries within the same admin's session
//     dedupe via the WAL backend's idempotency table. Different admins
//     issuing the same revoke produce distinct events — at worst the
//     second is a no-op on the applier side. Spec "Open questions"
//     item §2 calls this out.

package api

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const revokeAllUserAccessMethod = "RevokeAllUserAccess"

// revokeAllUserAccessTopic is the WAL topic the handler appends to.
// Matches the cmd/entdb-server/main.go default flag value ("entdb-wal").
// Hard-coded here because the Server type has no topic option;
// centralizing this constant keeps the api package self-contained until
// the cross-RPC topic wiring lands.
const revokeAllUserAccessTopic = "entdb-wal"

// revokeAllUserAccessSharedLimit caps the cross-tenant shared_index
// scan at 10000 rows. Users shared on more than 10k nodes won't be
// fully cleaned in one call — admins can re-run the RPC to drain the
// rest. Spec "Open questions" §5.
const revokeAllUserAccessSharedLimit = 10000

// RevokeAllUserAccess implements entdb.v1.EntDBService/RevokeAllUserAccess.
// See file header for the full contract.
func (s *Server) RevokeAllUserAccess(
	ctx context.Context, req *pb.RevokeAllUserAccessRequest,
) (*pb.RevokeAllUserAccessResponse, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		metrics.RecordGRPCRequest(ctx, revokeAllUserAccessMethod, status, time.Since(start))
	}()

	// Required-field validation BEFORE auth.
	if req.GetTenantId() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetUserId() == "" {
		status = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "user_id is required")
	}

	// Sharding gate. Sets `entdb-redirect-node` trailer + aborts
	// FAILED_PRECONDITION when this node does not own the tenant.
	if err := s.checkTenant(ctx, req.GetTenantId()); err != nil {
		status = "error"
		return nil, err
	}

	// Trusted-actor rebinding. Wire actor is UNTRUSTED.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// Admin-or-owner gate. system:/admin: prefixes bypass; everyone
	// else must carry the tenant member role "owner" or "admin".
	if !trusted.IsAdmin() && !trusted.IsSystem() {
		role := ""
		if s.global != nil {
			r, err := s.getTenantMemberRole(ctx, req.GetTenantId(), trusted.ID())
			if err != nil {
				status = "error"
				return nil, errs.Internal(ctx, "RevokeAllUserAccess: lookup caller role", err)
			}
			role = r
		}
		if role != "owner" && role != "admin" {
			status = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only tenant owner or admin can revoke all user access")
		}
	}

	// Required deps for the WAL-first path. Without a producer or a
	// store we can't honour the contract; surface UNIMPLEMENTED so the
	// caller sees a structural problem instead of silent success.
	if s.producer == nil || s.store == nil || s.global == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented,
			"RevokeAllUserAccess: WAL/store/globalstore not wired")
	}

	// Pre-append tallies. Reading from the per-tenant SQLite gives the
	// "rows that will be deleted by the applier" count. Idempotent
	// retries (which the applier dedupes) will see zero rows here on
	// the second call.
	revokedGrants, revokedGroups, err := s.countAccessRowsForUser(
		ctx, req.GetTenantId(), req.GetUserId(),
	)
	if err != nil {
		status = "error"
		return nil, errs.Internal(ctx, "RevokeAllUserAccess: count rows", err)
	}
	revokedShared := s.countSharedIndexRows(ctx, req.GetUserId(), req.GetTenantId())

	// WAL append: a single `admin_revoke_access` op. The applier
	// (apply/ops_admin_revoke_access.go) deletes node_access +
	// group_users + node_visibility in one BEGIN IMMEDIATE txn —
	// atomic on the tenant SQLite side.
	idempKey := fmt.Sprintf("revoke-all:%s:%s:%s",
		req.GetTenantId(), req.GetUserId(), trusted.String())
	ev := apply.Event{
		TenantID:       req.GetTenantId(),
		Actor:          trusted.String(),
		IdempotencyKey: idempKey,
		Ops: []map[string]any{
			{
				"op":      string(apply.OpAdminRevokeAccess),
				"user_id": req.GetUserId(),
			},
			{
				"op":        string(apply.OpAccessRevoked),
				"tenant_id": req.GetTenantId(),
				"user_id":   req.GetUserId(),
			},
		},
	}
	encoded, err := ev.Encode()
	if err != nil {
		status = "error"
		return nil, errs.Internal(ctx, "RevokeAllUserAccess: encode event", err)
	}
	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(idempKey),
	}
	pos, err := s.producer.Append(ctx,
		revokeAllUserAccessTopic, req.GetTenantId(), encoded, headers,
	)
	if err != nil {
		status = "error"
		return nil, errs.Internal(ctx, "RevokeAllUserAccess: wal append", err)
	}
	if err := s.waitForAdminApplied(ctx, req.GetTenantId(), pos.Offset, idempKey, "revoke all user access event"); err != nil {
		status = "error"
		return nil, err
	}

	return &pb.RevokeAllUserAccessResponse{
		Success:       true,
		RevokedGrants: int32(revokedGrants),
		RevokedGroups: int32(revokedGroups),
		RevokedShared: int32(revokedShared),
	}, nil
}

// countAccessRowsForUser reads node_access and group_users counts for
// userID from the per-tenant SQLite. Returns (grants, groups). A
// missing tenant DB is treated as "no rows" — nothing to revoke.
func (s *Server) countAccessRowsForUser(
	ctx context.Context, tenantID, userID string,
) (int64, int64, error) {
	db, err := s.store.AdminDB(tenantID)
	if err != nil || db == nil {
		// No tenant DB yet → nothing to revoke. Idempotent no-op path.
		return 0, 0, nil
	}
	var grants, groups int64
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM node_access WHERE actor_id = ?`, userID,
	).Scan(&grants); err != nil {
		return 0, 0, fmt.Errorf("count node_access: %w", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM group_users WHERE member_actor_id = ?`, userID,
	).Scan(&groups); err != nil {
		return 0, 0, fmt.Errorf("count group_users: %w", err)
	}
	return grants, groups, nil
}

// countSharedIndexRows counts shared_index rows for userID whose
// source_tenant matches tenantID. The paired access_revoked WAL op
// materializes the actual cleanup through the applier before return.
func (s *Server) countSharedIndexRows(
	ctx context.Context, userID, tenantID string,
) int {
	if s.global == nil {
		return 0
	}
	entries, err := s.global.ListSharedToUser(ctx, userID, revokeAllUserAccessSharedLimit, 0)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.SourceTenant != tenantID {
			continue
		}
		count++
	}
	return count
}
