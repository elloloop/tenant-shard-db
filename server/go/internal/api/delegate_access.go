// SPDX-License-Identifier: AGPL-3.0-only

// DelegateAccess implements entdb.v1.EntDBService/DelegateAccess.
//
// DelegateAccess RPC.
//
// Port spec: docs/go-port/rpcs/DelegateAccess.md.
//
// # Closes the silent-drop bug
//
// This handler closes the gRPC-side gap by emitting a per-node
// `delegate_access` op for every node owned by `from_user`, so the
// applier (apply/ops_delegate_access.go) can materialise each grant.
// An admin's bulk delegation now survives a WAL replay.
//
// # Behavioural pins
//
//   - Tenant gate first (s.checkTenant) — sharding + region pinning.
//   - Required-field validation (tenant_id / from_user / to_user).
//   - Trusted-actor rebind via auth.Authoritative; the wire `actor` is
//     UNTRUSTED. Privilege-escalation regression pinned by commit fece3fb.
//   - Admin-or-owner gate (NOT a per-node ADMIN check). The delegator
//     does not need any per-node grant — admin/owner is sufficient.
//   - Permission default: empty string -> "read".
//   - expires_at: 0 means "no expiry" (NULL in node_access). Past values
//     are persisted but filtered at read time by the visibility joins.
//   - WAL-first (CLAUDE.md invariant #1): we emit one
//     `delegate_access` op per node owned by from_user inside a single
//     TransactionEvent. The applier materialises each into a
//     node_access row keyed (node_id, actor_id) with granted_by =
//     trusted-actor.
//   - `delegated` echo: the count of nodes the owner held at handler-call
//     time — a pre-apply estimate.
//   - Soft-fail shape: WAL backend / count failures collapse to OK +
//     {success=false, error=<msg>}. Wire-level INTERNAL is intentionally
//     avoided so SDK consumers parse the response field, not the status
//     code (sdk/go/entdb/admin.go:95-104).

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

const grpcMethodDelegateAccess = "DelegateAccess"

// DelegateAccess bulk-grants `permission` on every node owned by
// `from_user` to `to_user`, optionally time-bounded by `expires_at`.
func (s *Server) DelegateAccess(
	ctx context.Context,
	req *pb.DelegateAccessRequest,
) (*pb.DelegateAccessResponse, error) {
	start := time.Now()

	// 1. Tenant gate (sharding + region pinning).
	if err := s.checkTenant(ctx, req.GetTenantId()); err != nil {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return nil, err
	}

	// 2. Required-field validation.
	if req.GetTenantId() == "" {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetFromUser() == "" {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "from_user is required")
	}
	if req.GetToUser() == "" {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return nil, errs.Errorf(codes.InvalidArgument, "to_user is required")
	}

	// 3. Trusted-actor rebind. The wire `actor` is UNTRUSTED — we
	//    consult auth.Authoritative so a forged actor in the request body
	//    cannot grant itself elevated privileges. fix-fece3fb.
	claimed := auth.ParseActor(req.GetActor())
	trusted := auth.Authoritative(ctx, claimed)

	// 4. Admin-or-owner gate. Non-system/non-admin trusted actors must
	//    hold tenant role "owner" or "admin".
	if !(trusted.IsSystem() || trusted.IsAdmin()) {
		if s.global == nil {
			metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
			return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
		}
		role, err := s.lookupMemberRole(ctx, req.GetTenantId(), trusted.ID())
		if err != nil {
			metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
			return nil, err
		}
		if role != "owner" && role != "admin" {
			metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only tenant owner or admin can delegate access")
		}
	}

	// 5. Resolve the writer prerequisites. Without a store + WAL the
	//    handler cannot honour invariant #1 (every mutation through the
	//    WAL); without them we soft-fail rather than wire-error so the
	//    SDK shape stays parsable.
	if s.store == nil || s.producer == nil {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return &pb.DelegateAccessResponse{
			Success: false,
			Error:   "DelegateAccess: store/wal not configured",
		}, nil
	}

	// 6. Look up the nodes owned by from_user. The count becomes the
	//    `delegated` echo AND drives the per-node ops we emit. Pre-apply
	//    estimate by design — new nodes created between SELECT and apply
	//    inflate the actual grant count.
	nodeIDs, err := listNodesByOwner(ctx, s, req.GetTenantId(), req.GetFromUser())
	if err != nil {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return &pb.DelegateAccessResponse{
			Success: false,
			Error:   fmt.Sprintf("DelegateAccess: list owner nodes: %v", err),
		}, nil
	}

	// 7. Build the WAL event. One delegate_access op per owned node;
	//    op shape mirrors apply/ops_delegate_access.go (node_id +
	//    actor_id keyed). Empty permission defaults to "read".
	permission := req.GetPermission()
	if permission == "" {
		permission = "read"
	}
	expiresAt := req.GetExpiresAt() // 0 => no expiry; the applier maps that to NULL.

	ops := make([]map[string]any, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		op := map[string]any{
			"op":         string(apply.OpDelegateAccess),
			"node_id":    nodeID,
			"actor_id":   req.GetToUser(),
			"actor_type": "user",
			"permission": permission,
		}
		if expiresAt != 0 {
			op["expires_at"] = expiresAt
		}
		ops = append(ops, op)
	}

	idemKey, err := newDelegateIdempotencyKey()
	if err != nil {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return &pb.DelegateAccessResponse{
			Success: false,
			Error:   fmt.Sprintf("DelegateAccess: idempotency key: %v", err),
		}, nil
	}

	event := wal.Event{
		TenantID:       req.GetTenantId(),
		Actor:          trusted.String(),
		IdempotencyKey: idemKey,
		TsMs:           time.Now().UnixMilli(),
		Ops:            ops,
	}
	value, err := event.Encode()
	if err != nil {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return &pb.DelegateAccessResponse{
			Success: false,
			Error:   fmt.Sprintf("DelegateAccess: encode event: %v", err),
		}, nil
	}

	headers := map[string][]byte{wal.HeaderIdempotencyKey: []byte(idemKey)}
	if _, err := s.producer.Append(ctx, s.walTopic(), req.GetTenantId(), value, headers); err != nil {
		metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "error", time.Since(start))
		return &pb.DelegateAccessResponse{
			Success: false,
			Error:   fmt.Sprintf("DelegateAccess: wal append: %v", err),
		}, nil
	}

	resp := &pb.DelegateAccessResponse{
		Success:   true,
		Delegated: int32(len(nodeIDs)),
	}
	// Echo expires_at unchanged: 0 stays 0 (permanent), positive value
	// echoes the request. Pinned by sdk/go/entdb/admin_test.go:291-333.
	resp.ExpiresAt = expiresAt

	metrics.RecordGRPCRequest(grpcMethodDelegateAccess, "ok", time.Since(start))
	return resp, nil
}

// listNodesByOwner returns every node_id whose owner_actor matches
// fromUser inside tenantID. The silent-drop bug meant this SELECT was
// not run on replay; the applier fix closed that gap.
//
// We open the tenant DB lazily so a fresh tenant with no nodes still
// returns an empty slice rather than a "tenant not open" error.
func listNodesByOwner(ctx context.Context, s *Server, tenantID, fromUser string) ([]string, error) {
	if err := s.store.OpenTenant(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("open tenant: %w", err)
	}
	db, err := s.store.AdminDB(tenantID)
	if err != nil {
		return nil, fmt.Errorf("admin db: %w", err)
	}
	rows, err := db.QueryContext(ctx,
		`SELECT node_id FROM nodes WHERE tenant_id = ? AND owner_actor = ? ORDER BY node_id`,
		tenantID, fromUser,
	)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan node_id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	return out, nil
}

// newDelegateIdempotencyKey returns "admin-delegate-<32hex>". The 16
// random bytes give us 128 bits of entropy, without pulling a
// third-party dep.
func newDelegateIdempotencyKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "admin-delegate-" + hex.EncodeToString(b[:]), nil
}
