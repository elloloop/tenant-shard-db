// SPDX-License-Identifier: AGPL-3.0-only

// TransferUserContent implements entdb.v1.EntDBService/TransferUserContent.
//
// EPIC #407 Wave 2. Spec: docs/go-port/rpcs/TransferUserContent.md.
// Source-of-truth Python: server/python/entdb_server/api/grpc_server.py:2692-2745.
//
// # Behaviour pins
//
//   - WAL-FIRST. The per-tenant ownership change is appended to the WAL
//     as one (or more, see below) `admin_transfer_content` events.
//     Direct canonical-store writes from this handler are FORBIDDEN
//     (CLAUDE.md invariant #1, pinned by
//     tests/python/unit/test_admin_ops.py:373-407).
//
//   - Trusted-actor. The wire `actor` is UNTRUSTED. We rebind to the
//     trusted actor returned by auth.Authoritative before any privilege
//     check. A claimed `actor="system:admin"` from a non-admin trusted
//     identity MUST PERMISSION_DENY *and* MUST NOT append to the WAL
//     (test_privilege_escalation.py:344-365).
//
//   - Auth: trusted-actor must be system:* / admin:*, OR have role
//     "owner" / "admin" in tenant_members for tenantID. Anything else
//     -> PERMISSION_DENIED.
//
//   - Tenant gate. s.checkTenant runs before any other side effect so
//     unknown / archived / wrong-region tenants reject cleanly.
//
//   - INVALID_ARGUMENT validation. tenant_id, from_user, to_user, and
//     the wire actor must all be non-empty BEFORE the auth substitution
//     (mirrors :2701-2706 — non-empty `actor` quirk preserved).
//
//   - Side effects, in order:
//       1. global_store.TransferUserContent — idempotent membership
//          upsert for `to_user` with role 'member'. Direct write; this
//          is the documented control-plane carve-out.
//       2. WAL append of `admin_transfer_content`. Single event for
//          small batches; chunked into multiple events for large
//          owners (CHUNK_SIZE) so a single SQLite write transaction
//          stays bounded — addresses the spec "Open question" §2
//          (large user → write-lock spike).
//       3. Pre-apply count via store.CountOwnedNodes — best-effort,
//          errors swallowed (returns 0). This count reflects nodes
//          *still* owned at the time of the call (the Applier
//          materialises the rename asynchronously). Pinned semantics.
//
//   - Visibility refresh. Performed by the Applier handler
//     (apply/ops_admin_transfer_content.go). The handler does NOT
//     touch node_visibility — that would violate WAL-first.
//
//   - Mailbox / notifications cascade. Out of scope. Existing per-node
//     ACL grants survive — only owner_actor changes.
//
//   - On global-store failure or WAL-append failure, the handler
//     returns gRPC OK with `success=false`, `error=<msg>` (matches
//     Python's :2740-2745 catch-all). gRPC-level codes are reserved
//     for tenant gate, validation, and auth.

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
	transferUserContentMethod = "TransferUserContent"

	// transferUserContentChunkSize bounds the number of node_ids attached
	// to a single `admin_transfer_content` op. The Applier rewrites
	// owner_actor inside one SQLite write transaction; without a chunk
	// cap a 10M-owned-node tenant would hold the per-tenant write lock
	// for the duration of the UPDATE, blocking other writers.
	//
	// The Python handler emits exactly ONE event regardless of size —
	// see spec "Open questions" §2 for the documented risk. Chunking is
	// the Go-side mitigation. Chunk size is conservative; the Applier
	// op handler accepts both the un-chunked form (no node_ids field —
	// rewrite all rows for `from_user`) and the chunked form (explicit
	// node_ids list) so behaviour is identical for small batches.
	transferUserContentChunkSize = 1000

	// transferUserContentTopic is the WAL topic used for tenant-scoped
	// transaction events. Matches Python's `self.topic` default
	// ("entdb-wal", grpc_server.py:2718-2724).
	transferUserContentTopic = "entdb-wal"
)

// TransferUserContent reassigns ownership of every node owned by
// `from_user` inside `tenant_id` to `to_user`. Bulk offboarding flow.
//
// See file-level comment for the full pin list.
func (s *Server) TransferUserContent(
	ctx context.Context, req *pb.TransferUserContentRequest,
) (*pb.TransferUserContentResponse, error) {
	start := time.Now()
	statusLabel := "ok"
	defer func() {
		metrics.RecordGRPCRequest(transferUserContentMethod, statusLabel, time.Since(start))
	}()

	// Wave-2 dependency guard. A node booted without a globalstore can
	// not honour admin RPCs.
	if s.global == nil {
		statusLabel = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Tenant registry not configured")
	}

	// INVALID_ARGUMENT validation. The non-empty `actor` check runs
	// BEFORE the trusted-actor substitution to mirror the Python quirk
	// (spec "Open questions" §6, test_admin_operations.py:806-820).
	if req.GetActor() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "actor is required")
	}
	if req.GetTenantId() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetFromUser() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "from_user is required")
	}
	if req.GetToUser() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "to_user is required")
	}

	// Tenant gate. Catches unknown / archived tenants and wrong-region
	// requests before any side effect. Mirrors grpc_server.py:2700.
	if err := s.checkTenant(ctx, req.GetTenantId()); err != nil {
		statusLabel = "error"
		return nil, err
	}

	// Trusted-actor rebinding. Privilege checks below MUST consult
	// `trusted` (never `req.Actor`). Closing the privilege-escalation
	// hole is the entire point of fece3fb / auth.Authoritative.
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))

	// Auth: system / admin trusted actors bypass the membership lookup.
	// Otherwise the trusted user must be owner or admin of this tenant.
	if !(trusted.IsSystem() || trusted.IsAdmin()) {
		role, err := s.getTenantMemberRole(ctx, req.GetTenantId(), trusted.ID())
		if err != nil {
			statusLabel = "error"
			return nil, errs.Errorf(codes.Internal,
				"TransferUserContent: lookup caller role: %v", err)
		}
		if role != "owner" && role != "admin" {
			statusLabel = "error"
			return nil, errs.Errorf(codes.PermissionDenied,
				"TransferUserContent requires admin/owner")
		}
	}

	// Side effect 1: idempotent global-store membership upsert for
	// to_user. Direct write — control-plane carve-out (spec §"Side
	// effects" #1, admin_handlers.py:7-11).
	if _, err := s.global.TransferUserContent(
		ctx, req.GetTenantId(), req.GetFromUser(), req.GetToUser(),
	); err != nil {
		statusLabel = "error"
		return &pb.TransferUserContentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Side effect 2: WAL append. Source of truth for the rename. The
	// op shape mirrors apply/ops_admin_transfer_content.go (W1.10).
	//
	// Chunking: when the producer is wired AND a canonical store is
	// available we enumerate owned node_ids first and split into
	// multiple events of size <= transferUserContentChunkSize. The
	// Applier handler rewrites by `(tenant_id, owner_actor)` so each
	// chunk converges to the same end state idempotently. When the
	// canonical store is absent (Wave-1 deployments without store
	// wiring) we fall through to the un-chunked single-event shape.
	if s.producer == nil {
		statusLabel = "error"
		return &pb.TransferUserContentResponse{
			Success: false,
			Error:   "WAL producer not configured",
		}, nil
	}

	chunks := s.transferUserContentChunks(ctx, req.GetTenantId(), req.GetFromUser())
	for _, chunk := range chunks {
		op := map[string]any{
			"op":        "admin_transfer_content",
			"from_user": req.GetFromUser(),
			"to_user":   req.GetToUser(),
		}
		if len(chunk) > 0 {
			// Convert []string -> []any so JSON marshalling preserves
			// the ordering Python's json.dumps emits.
			ids := make([]any, 0, len(chunk))
			for _, id := range chunk {
				ids = append(ids, id)
			}
			op["node_ids"] = ids
		}
		event := wal.Event{
			TenantID:       req.GetTenantId(),
			Actor:          trusted.String(),
			IdempotencyKey: "admin-transfer-" + randHex16(),
			TsMs:           time.Now().UnixMilli(),
			Ops:            []map[string]any{op},
		}
		payload, err := event.Encode()
		if err != nil {
			statusLabel = "error"
			return &pb.TransferUserContentResponse{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		headers := map[string][]byte{
			wal.HeaderIdempotencyKey: []byte(event.IdempotencyKey),
		}
		if _, err := s.producer.Append(
			ctx, transferUserContentTopic, req.GetTenantId(), payload, headers,
		); err != nil {
			statusLabel = "error"
			return &pb.TransferUserContentResponse{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
	}

	// Side effect 3: best-effort pre-apply count. Errors swallowed —
	// matches grpc_server.py:2735-2736.
	var transferred int32
	if s.store != nil {
		if n, err := s.store.CountOwnedNodes(
			ctx, req.GetTenantId(), req.GetFromUser(),
		); err == nil {
			transferred = n
		}
	}

	return &pb.TransferUserContentResponse{
		Success:     true,
		Transferred: transferred,
	}, nil
}

// transferUserContentChunks enumerates the node_ids `fromUser` owns in
// `tenantID` and returns them split into chunks of size
// transferUserContentChunkSize. When the canonical store is unwired
// (or returns no rows / errors) it returns a single empty chunk so the
// caller still emits exactly one un-chunked event (parity with the
// Python single-event shape).
func (s *Server) transferUserContentChunks(
	ctx context.Context, tenantID, fromUser string,
) [][]string {
	if s.store == nil {
		return [][]string{nil}
	}
	ids, err := s.store.ListOwnedNodeIDs(ctx, tenantID, fromUser)
	if err != nil || len(ids) == 0 {
		return [][]string{nil}
	}
	chunks := make([][]string, 0, (len(ids)+transferUserContentChunkSize-1)/transferUserContentChunkSize)
	for i := 0; i < len(ids); i += transferUserContentChunkSize {
		end := i + transferUserContentChunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}

// randHex16 returns a 32-char lowercase hex string. Used as the random
// suffix on the idempotency key. The Python handler uses uuid4 hex
// (32 chars) — same alphabet, same length. Falls back to a
// nanosecond-timestamp on rand failure (defensive — crypto/rand on
// Linux has not failed in production memory).
func randHex16() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely. Compose from time so we still emit a
		// non-empty key (Event.Encode would reject "").
		ns := time.Now().UnixNano()
		out := make([]byte, 16)
		for i := range out {
			out[i] = byte(ns >> (i % 8 * 8))
		}
		return hex.EncodeToString(out)
	}
	return hex.EncodeToString(b[:])
}
