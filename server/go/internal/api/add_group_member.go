// SPDX-License-Identifier: AGPL-3.0-only

// AddGroupMember implements entdb.v1.EntDBService/AddGroupMember.
//
// Port spec: docs/go-port/rpcs/AddGroupMember.md.
//
// # WAL-first restoration
//
// This handler restores the WAL-first invariant: it appends an
// `add_group_member` op to the per-tenant WAL and lets the Applier
// (server/go/internal/apply/ops_add_group_member.go) materialize the
// row. A rebuild-from-empty-WAL therefore reconstructs membership.
//
// # Trusted-actor substitution
//
// Privilege-escalation gap closed in commit fece3fb:
//
//  1. `auth.Authoritative(ctx, claimed)` is the single source of truth
//     for caller identity. The wire-claimed `request.context.actor` is
//     informational only.
//  2. Authorization succeeds iff the trusted actor is admin: / system:,
//     or holds the "owner" / "admin" role on the target tenant. v1
//     "group admin" === "tenant admin" since no per-group ownership
//     table exists yet (spec "Open questions" §1).
//
// # Idempotency
//
//   - The op uses `INSERT OR REPLACE INTO group_users` (apply path), so
//     a re-add with the same (group_id, member_actor_id) overwrites
//     role + joined_at.
//   - The WAL Append carries an idempotency key derived from
//     (tenant_id, group_id, member_actor_id, role) so a network retry
//     of the same logical request returns the original StreamPos
//     without writing a duplicate record.
//
// # Error contract
//
//	UNIMPLEMENTED WAL producer not configured.
//	NOT_FOUND / FAILED_PRECONDITION
//	                    tenant missing/disabled (via checkTenant).
//	INVALID_ARGUMENT tenant_id / group_id / member_actor_id empty.
//	PERMISSION_DENIED trusted actor is neither admin/system nor
//	                    owner/admin member of tenant_id.
//	OK + success=false WAL append failed (in-band error shape:
//	                    gRPC OK, response.Error set).
//	OK + success=true WAL append accepted; apply happens
//	                    asynchronously (caller fences via
//	                    WaitForOffset on subsequent reads).

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// addGroupMemberWALTopic is the per-tenant WAL topic the handler appends
// to. The entire EntDB stream lives on a single topic today;
// partitioning is by tenant_id (the Append key).
const addGroupMemberWALTopic = "entdb-wal"

// AddGroupMember adds (or re-adds, idempotently) a member to a group.
// See package-level doc for the WAL-first restoration and trusted-actor
// substitution.
func (s *Server) AddGroupMember(
	ctx context.Context,
	req *pb.GroupMemberRequest,
) (*pb.GroupMemberResponse, error) {
	start := time.Now()
	statusLabel := "ok"
	defer func() {
		metrics.RecordGRPCRequest(ctx, "AddGroupMember", statusLabel, time.Since(start))
	}()

	if s.producer == nil {
		statusLabel = "error"
		return nil, errs.Errorf(codes.Unimplemented, "WAL producer not configured")
	}

	rc := req.GetContext()
	tenantID := rc.GetTenantId()
	groupID := req.GetGroupId()
	memberActorID := req.GetMemberActorId()

	// Required-arg validation BEFORE the tenant gate so a malformed
	// request can never leak tenant existence via NOT_FOUND vs
	// INVALID_ARGUMENT timing differences.
	if tenantID == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if groupID == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "group_id is required")
	}
	if memberActorID == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "member_actor_id is required")
	}

	if err := s.checkTenant(ctx, tenantID); err != nil {
		statusLabel = "error"
		return nil, err
	}

	// Trusted-actor substitution: the wire-claimed actor is ignored.
	// Every authorization branch consults `trusted` from this point on
	// (privilege-escalation regression pinned by commit fece3fb).
	claimed := auth.ParseActor(rc.GetActor())
	trusted := auth.Authoritative(ctx, claimed)

	// v1 group-admin === tenant-admin. The schema has no per-group
	// ownership today (spec "Open questions" §1) so the only safe rule
	// is "tenant admin only". Per-group admin is a follow-up.
	if !(trusted.IsAdmin() || trusted.IsSystem()) {
		if s.global == nil {
			statusLabel = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only tenant admins can add group members")
		}
		role, err := s.lookupMemberRole(ctx, tenantID, trusted.ID())
		if err != nil {
			statusLabel = "error"
			return nil, err
		}
		if role != "owner" && role != "admin" {
			statusLabel = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only tenant admins can add group members")
		}
	}

	role := req.GetRole()
	if role == "" {
		role = "member"
	}

	// Build the WAL event. The op shape is the contract that
	// apply/ops_add_group_member.go consumes — keys MUST match exactly.
	idempKey := addGroupMemberIdempotencyKey(tenantID, groupID, memberActorID, role)
	ev := wal.Event{
		TenantID:       tenantID,
		Actor:          trusted.String(),
		IdempotencyKey: idempKey,
		Ops: []map[string]any{{
			"op":              "add_group_member",
			"group_id":        groupID,
			"member_actor_id": memberActorID,
			"role":            role,
		}},
	}
	value, err := ev.Encode()
	if err != nil {
		statusLabel = "error"
		return &pb.GroupMemberResponse{Success: false, Error: err.Error()}, nil
	}

	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(idempKey),
	}
	if _, err := s.producer.Append(ctx, addGroupMemberWALTopic, tenantID, value, headers); err != nil {
		// In-band error, gRPC OK. The spec calls this out explicitly
		// (docs/go-port/rpcs/AddGroupMember.md "Error contract"
		// status-code parity quirk).
		statusLabel = "error"
		return &pb.GroupMemberResponse{Success: false, Error: err.Error()}, nil
	}

	return &pb.GroupMemberResponse{Success: true}, nil
}

// addGroupMemberIdempotencyKey derives a stable, content-addressed
// idempotency key so two byte-identical AddGroupMember requests dedupe
// at the WAL boundary. The hash inputs are the (tenant, group, member,
// role) tuple — exactly the fields the apply path keys on. Re-adds
// with the same role return the original StreamPos; re-adds with a
// different role create a new WAL record (which the apply path will
// REPLACE the row for, matching INSERT OR REPLACE semantics).
func addGroupMemberIdempotencyKey(tenantID, groupID, memberActorID, role string) string {
	h := sha256.New()
	// NUL-separated to avoid prefix-collision ambiguity between fields.
	h.Write([]byte("add_group_member\x00"))
	h.Write([]byte(tenantID))
	h.Write([]byte{0})
	h.Write([]byte(groupID))
	h.Write([]byte{0})
	h.Write([]byte(memberActorID))
	h.Write([]byte{0})
	h.Write([]byte(role))
	return hex.EncodeToString(h.Sum(nil))
}
