// SPDX-License-Identifier: AGPL-3.0-only

// RemoveGroupMember implements entdb.v1.EntDBService/RemoveGroupMember
// (EPIC #407, Wave 2).
//
// Spec: docs/go-port/rpcs/RemoveGroupMember.md.
// Source-of-truth Python: server/python/entdb_server/api/grpc_server.py:1986-2028.
//
// # Behavioural pins
//
//   - WAL-first restoration. Python writes directly to per-tenant
//     SQLite via canonical_store.remove_group_member, bypassing the
//     event log (CLAUDE.md §1 violation flagged by the spec). The Go
//     port routes the membership delete through wal.Append as a
//     `remove_group_member` op (W1.10 applier handler at
//     server/go/internal/apply/ops_remove_group_member.go), so a
//     replay-from-empty-WAL reconstructs the same group_users state.
//     The handler still owns the cascade because the Python contract
//     pins shared_index cleanup to the synchronous response (best-
//     effort, swallowed on error).
//
//   - Auth (Go HARDENS vs Python). Python's capability registry does
//     NOT map RemoveGroupMember to a capability — any caller passing
//     `_check_tenant` can remove anyone from any group (privilege-
//     escalation gap, spec §"Open questions" item 6). The Go port
//     gates the RPC behind admin/system trusted actors OR a tenant
//     "owner"/"admin" membership row, matching AddTenantMember /
//     ChangeMemberRole / RemoveTenantMember. There is no "group
//     owner" concept in the schema today; v1 rule is tenant-admin.
//
//   - Trusted-actor rebind. The wire `actor` is UNTRUSTED. Privilege
//     decisions consult auth.Authoritative(ctx) only. The privilege-
//     escalation regression pinned by commit fece3fb applies here too
//     even though Python does not invoke `_trusted_actor` for this
//     RPC.
//
//   - Cascade scope. Per spec §"Side effects" item 6.2: only group-
//     derived shared_index entries are cleaned up. Direct grants on
//     individual nodes (acl_grants rows where grantee == member) are
//     NOT touched — those go through RevokeAccess /
//     RevokeAllUserAccess. Pre-read of node_access(group_id) MUST
//     happen before the WAL append; reading after observes the
//     already-removed membership edge and the cascade silently
//     undercounts (spec ordering invariant).
//
//   - Idempotency. Removing a non-existent (group, member) returns
//     success=false, error="" with code OK (parity with Python and
//     spec §"Error contract"). Repeats are safe — the op is a
//     DELETE with no rows-affected tracking on the apply path, the
//     pre-read is a SELECT, and global_store.RemoveShared is a
//     delete-if-exists.
//
//   - role on Remove. Proto carries `role` (shared with AddGroupMember)
//     but Python ignores it on the Remove path (grpc_server.py:1962
//     reads it on Add only). Go port: accept and discard, do NOT
//     thread it into the WAL op.

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// removeGroupMemberMethod is the metric label this handler reports
// under entdb_grpc_requests_total{method=...}.
const removeGroupMemberMethod = "RemoveGroupMember"

// removeGroupMemberWALTopic is the WAL topic this handler appends to.
// Hard-coded to the `entdb-wal` default (cmd/entdb-server/main.go:32);
// the api.Server does not currently take a topic option (Wave-2 scope
// freeze). When the topic becomes configurable, swap this for a field
// on Server populated by a WithWALTopic option.
const removeGroupMemberWALTopic = "entdb-wal"

// RemoveGroupMember removes (group_id, member_actor_id) from a tenant's
// group_users table by appending a `remove_group_member` op to the WAL
// and best-effort cascading the cross-tenant shared_index projection.
func (s *Server) RemoveGroupMember(
	ctx context.Context, req *pb.GroupMemberRequest,
) (*pb.GroupMemberResponse, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		metrics.RecordGRPCRequest(removeGroupMemberMethod, status, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()

	// Tenant gate (region pin + redirect trailer). Mirrors
	// grpc_server.py:1994. On miss/redirect this surfaces the typed
	// gRPC code the SDK redirect cache reads.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		status = "error"
		return nil, err
	}

	if s.store == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "RemoveGroupMember: store not wired")
	}
	if s.producer == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "RemoveGroupMember: WAL producer not wired")
	}

	groupID := req.GetGroupId()
	memberID := req.GetMemberActorId()

	// Trusted-actor rebind. From here on, req.GetContext().GetActor()
	// is informational; every privilege branch consults `trusted`
	// (privilege-escalation invariant, commit fece3fb).
	claimed := auth.ParseActor(req.GetContext().GetActor())
	trusted := auth.Authoritative(ctx, claimed)

	// Capability check — Go HARDENS vs Python (spec §"Open questions"
	// item 6 latent privilege escalation). System / admin prefixed
	// trusted actors bypass; otherwise the caller must be a tenant
	// owner/admin, mirroring AddTenantMember / ChangeMemberRole /
	// RemoveTenantMember.
	if !(trusted.IsSystem() || trusted.IsAdmin()) {
		if s.global == nil {
			status = "error"
			return nil, errs.Errorf(codes.Unimplemented,
				"RemoveGroupMember: tenant registry not configured")
		}
		role, err := s.lookupMemberRole(ctx, tenantID, trusted.ID())
		if err != nil {
			status = "error"
			return nil, errs.Errorf(codes.Internal,
				"RemoveGroupMember: lookup caller role: %v", err)
		}
		if role != "owner" && role != "admin" {
			status = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"Only tenant owner/admin can remove group members")
		}
	}

	// Snapshot current state BEFORE the WAL append. Two pre-reads:
	//
	//   1. `found` — was the (group, member) pair a row in group_users?
	//      The WAL DELETE is rows-affected-agnostic at the applier
	//      layer (ops_remove_group_member.go just runs the DELETE);
	//      the response.success flag mirrors Python's canonical_store
	//      return value, so we read it here.
	//
	//   2. `groupAccess` — node_access rows where actor_id == groupID
	//      and actor_type == 'group'. Pre-read ordering is load-
	//      bearing: reading AFTER the delete would observe the
	//      already-removed membership edge and the cascade would
	//      undercount (spec §"Side effects" item 6.2 invariant).
	found, err := s.store.IsGroupMember(ctx, tenantID, groupID, memberID)
	if err != nil {
		status = "error"
		slog.WarnContext(ctx, "RemoveGroupMember: pre-read membership failed",
			"tenant", tenantID, "group", groupID, "member", memberID, "err", err)
		return &pb.GroupMemberResponse{Success: false, Error: err.Error()}, nil
	}

	var groupAccess []store.GroupNodeAccess
	if s.global != nil {
		entries, gerr := s.store.ListNodeAccessForGroup(ctx, tenantID, groupID)
		if gerr != nil {
			// Best-effort pre-read: log and continue without cascade
			// (matches Python grpc_server.py:1998-2008 swallow on the
			// pre-read error path). The membership delete still goes
			// through.
			slog.WarnContext(ctx, "RemoveGroupMember: shared_index pre-read failed",
				"tenant", tenantID, "group", groupID, "err", gerr)
		} else {
			groupAccess = entries
		}
	}

	// WAL append — restores CLAUDE.md §1. Op shape is the contract the
	// W1.10 applier handler reads at apply/ops_remove_group_member.go.
	idempKey, err := newIdempotencyKey()
	if err != nil {
		status = "error"
		return &pb.GroupMemberResponse{Success: false, Error: err.Error()}, nil
	}
	ev := wal.Event{
		TenantID:       tenantID,
		Actor:          trusted.String(),
		IdempotencyKey: idempKey,
		Ops: []map[string]any{
			{
				"op":              "remove_group_member",
				"group_id":        groupID,
				"member_actor_id": memberID,
			},
		},
	}
	value, err := ev.Encode()
	if err != nil {
		status = "error"
		return &pb.GroupMemberResponse{Success: false, Error: err.Error()}, nil
	}
	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(idempKey),
	}
	// Per-tenant total order: key by tenant_id so all of a tenant's
	// records land on the same partition (matches the Python applier
	// partition strategy in wal/memory.py:74).
	if _, werr := s.producer.Append(ctx, removeGroupMemberWALTopic, tenantID, value, headers); werr != nil {
		status = "error"
		return &pb.GroupMemberResponse{
			Success: false,
			Error:   fmt.Sprintf("wal append: %v", werr),
		}, nil
	}

	// Best-effort shared_index cascade — only when the pair existed
	// (parity with Python grpc_server.py:2014-2020: skipped entirely
	// on found=false). Failures are logged and swallowed; the RPC
	// returns success=found.
	if found && s.global != nil {
		for _, e := range groupAccess {
			if _, rerr := s.global.RemoveShared(ctx, memberID, tenantID, e.NodeID); rerr != nil {
				slog.WarnContext(ctx,
					"RemoveGroupMember: shared_index cascade failed",
					"tenant", tenantID, "group", groupID,
					"member", memberID, "node", e.NodeID, "err", rerr)
			}
		}
	}

	return &pb.GroupMemberResponse{Success: found}, nil
}

// newIdempotencyKey returns a 128-bit hex-encoded random idempotency
// key for the WAL Append. The producer's dedupe identity is
// (topic, key, idempotency_key); a fresh key per RPC means every
// distinct RemoveGroupMember invocation lands a distinct WAL record
// even when the (group, member) pair repeats.
func newIdempotencyKey() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("RemoveGroupMember: idempotency key: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
