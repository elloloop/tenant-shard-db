// SPDX-License-Identifier: AGPL-3.0-only

// TransferOwnership implements entdb.v1.EntDBService/TransferOwnership.
//
// Source-of-truth Python: server/python/entdb_server/api/grpc_server.py:2030-2049.
// Port spec: docs/go-port/rpcs/TransferOwnership.md.
//
// # WAL-first restoration (CLAUDE.md §1)
//
// The Python handler bypasses the WAL and writes nodes.owner_actor
// directly via canonical_store._sync_transfer_ownership — a long-
// standing violation of "every mutation goes through the WAL"
// (CLAUDE.md invariant #1, called out in PLAN.md §6.1).
//
// The Go port restores the invariant: the handler appends a
// `transfer_ownership` op to the per-tenant WAL via the shared
// wal.Producer, then synchronously materialises the change in the
// per-tenant SQLite via store.TransferOwnership. The applier already
// dispatches the same op on replay (see internal/apply/ops_transfer_ownership.go),
// so a future rebuild from the WAL will reproduce the transfer
// exactly. Idempotency-key based de-dupe in the applier keeps the
// in-flight + replay paths from double-applying.
//
// # Auth model — trusted-actor, current-owner-only
//
//  1. Authentication is required (handler is NOT in
//     AuthInterceptor.UNAUTHENTICATED_METHODS).
//  2. The wire-claimed `context.actor` field is UNTRUSTED — the
//     handler rebinds to the interceptor-attested identity via
//     auth.Authoritative on entry. Privilege-escalation regression
//     pinned by commit fece3fb ("Fix privilege escalation: ignore
//     client-claimed actor in gRPC handlers").
//  3. Tenant gate: s.checkTenant verifies the tenant exists and is
//     owned by this node (sharding) and is in the served region.
//  4. Owner-only authorization: the trusted actor MUST equal the
//     node's current owner_actor. Non-owner callers — even those who
//     hold an ADMIN ACL grant on the node — are rejected with
//     PERMISSION_DENIED. This is a Go-side hardening over the Python
//     handler, which performs no authorization beyond _check_tenant.
//
// # Side effects
//
//   - WAL append: one TransactionEvent containing a single
//     `transfer_ownership` op (node_id, new_owner). Idempotency key
//     is derived per-call via a UUID.
//   - SQLite: nodes.owner_actor flipped to new_owner; node_visibility
//     refreshed so the new owner can see the node and the old owner
//     loses implicit access (explicit ACL grants survive — by design,
//     spec §"ACL changes").
//   - No globalstore writes. No mailbox notification.
//
// # Error contract
//
//	INVALID_ARGUMENT      empty node_id / new_owner / context.tenant_id
//	                      OR new_owner is not a kind:id-prefixed actor
//	                      string (user:/group:/system:).
//	NOT_FOUND             tenant does not exist (from checkTenant) OR
//	                      node does not exist in the tenant.
//	UNAVAILABLE           tenant pinned to another node.
//	PERMISSION_DENIED     trusted actor is not the current owner.
//	OK + found=true       transfer applied.
//
// Note: the Python wire contract returns (found=false, error="") for a
// missing node (no gRPC status). The Go port hardens this to NOT_FOUND
// because:
//   - It is consistent with the rest of the Wave-2 surface (every
//     "row missing" path uses codes.NotFound), AND
//   - The contract test that pinned the soft-fail behaviour
//     (test_acl_v2.py:534-536) is being retired alongside the Python
//     server in EPIC #407. Existing Python tests targeting
//     TransferOwnership are pinned via the parity harness, not via this
//     wire shape.
//
// Metrics: emits entdb_grpc_requests_total{method="TransferOwnership",
// status="ok"|"error"} via the shared chokepoint.

package api

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const grpcMethodTransferOwnership = "TransferOwnership"

// transferOwnershipTopic is the WAL topic the handler appends to. Mirrors
// the per-tenant Kafka topic naming in the Python wal/kafka.py module —
// each tenant key partitions onto its own slot so total order is
// preserved per-tenant. The applier consumes from the same topic name.
const transferOwnershipTopic = "entdb-wal"

// TransferOwnership reassigns nodes.owner_actor and refreshes node_visibility.
// See package-level doc for the WAL-first restoration model and auth contract.
func (s *Server) TransferOwnership(
	ctx context.Context,
	req *pb.TransferOwnershipRequest,
) (*pb.TransferOwnershipResponse, error) {
	start := time.Now()
	statusLabel := "ok"
	defer func() {
		metrics.RecordGRPCRequest(grpcMethodTransferOwnership, statusLabel, time.Since(start))
	}()

	// 1. Validation. Mirror the Python required-arg checks plus the
	//    Go-side hardening called out in spec §"Wire contract".
	if req.GetContext() == nil || req.GetContext().GetTenantId() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetNodeId() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "node_id is required")
	}
	if req.GetNewOwner() == "" {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "new_owner is required")
	}
	if !isValidActorString(req.GetNewOwner()) {
		statusLabel = "error"
		return nil, errs.Errorf(codes.InvalidArgument,
			"new_owner must be a kind:id actor string (user:..., group:..., system:...)")
	}

	tenantID := req.GetContext().GetTenantId()

	// 2. Tenant gate — verify the tenant exists, is owned by this node,
	//    and matches the served region. Returns NotFound / Unavailable
	//    with the redirect trailer pre-set on sharding mismatch.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		statusLabel = "error"
		return nil, err
	}

	// 3. Trusted-actor rebind. From this point on, req.Context.Actor is
	//    strictly informational; every authorization branch consults
	//    `trusted`. Privilege-escalation regression fix in commit fece3fb.
	claimed := auth.ParseActor(req.GetContext().GetActor())
	trusted := auth.Authoritative(ctx, claimed)

	if s.store == nil {
		statusLabel = "error"
		return nil, errs.Errorf(codes.Unimplemented, "canonical store not configured")
	}

	// 4. Read the node to a) confirm existence (NotFound otherwise) and
	//    b) check current ownership for the auth gate.
	current, err := s.store.GetNode(ctx, tenantID, req.GetNodeId())
	if err != nil {
		if errors.Is(err, store.ErrNodeNotFound) {
			statusLabel = "error"
			return nil, errs.Errorf(codes.NotFound, "node %q not found", req.GetNodeId())
		}
		statusLabel = "error"
		return nil, errs.Errorf(codes.Internal, "read node: %v", err)
	}

	// 5. Current-owner-only authorization. The trusted actor's full
	//    "kind:id" form must match the stored owner_actor verbatim.
	//    system: callers are NOT bypassed here — TransferOwnership is a
	//    user-initiated handoff, not an admin override (the Python
	//    bulk-reassign handler `TransferUserContent` covers the admin
	//    path). Group ownership is technically representable on disk
	//    but a group cannot be a caller, so it cannot pass this gate.
	if trusted.IsZero() || trusted.String() != current.OwnerActor {
		statusLabel = "error"
		return nil, errs.Errorf(codes.PermissionDenied,
			"only the current owner of node %q can transfer ownership", req.GetNodeId())
	}

	// 6. WAL append. Encodes the same op shape the applier dispatches in
	//    internal/apply/ops_transfer_ownership.go.
	idempKey, err := newIdempotencyKey()
	if err != nil {
		statusLabel = "error"
		return nil, errs.Errorf(codes.Internal, "generate idempotency key: %v", err)
	}
	evt := wal.Event{
		TenantID:       tenantID,
		Actor:          trusted.String(),
		IdempotencyKey: idempKey,
		Ops: []map[string]any{
			{
				"op":        string(apply.OpTransferOwnership),
				"node_id":   req.GetNodeId(),
				"new_owner": req.GetNewOwner(),
			},
		},
	}
	if s.producer != nil {
		value, encErr := evt.Encode()
		if encErr != nil {
			statusLabel = "error"
			return nil, errs.Errorf(codes.Internal, "encode wal event: %v", encErr)
		}
		headers := map[string][]byte{
			wal.HeaderIdempotencyKey: []byte(idempKey),
		}
		if _, appendErr := s.producer.Append(ctx, transferOwnershipTopic, tenantID, value, headers); appendErr != nil {
			statusLabel = "error"
			return nil, errs.Errorf(codes.Internal, "wal append: %v", appendErr)
		}
	}

	// 7. Synchronous apply. The applier will re-dispatch the same op on
	//    replay; CheckIdempotency in applier.applyEvent dedupes via the
	//    applied_events table so the rebuild stays correct.
	if err := s.store.TransferOwnership(ctx, tenantID, req.GetNodeId(), req.GetNewOwner()); err != nil {
		if errors.Is(err, store.ErrNodeNotFound) {
			// Race window: node was deleted between GetNode and the
			// apply. Surface as NotFound for symmetry with step 4.
			statusLabel = "error"
			return nil, errs.Errorf(codes.NotFound, "node %q not found", req.GetNodeId())
		}
		statusLabel = "error"
		return nil, errs.Errorf(codes.Internal, "apply transfer_ownership: %v", err)
	}

	return &pb.TransferOwnershipResponse{Found: true}, nil
}

// isValidActorString reports whether s is a kind:id-prefixed actor string
// the spec accepts for `new_owner`. Mirrors auth.ParseActor's prefix
// table (user:/group:/system:/admin:) and rejects KindUnknown so a bare
// "alice" or a tenant-qualified "tenant_a:user:alice" is refused with
// INVALID_ARGUMENT.
//
// auth.ParseActor returns a known kind only when the string strictly
// starts with one of {"user:", "group:", "system:", "admin:"} and has a
// non-empty id portion; "tenant_a:user:alice" splits on the FIRST colon
// and yields KindUnknown, which is exactly what we want.
//
// Empty strings are rejected by the caller before this function is
// reached — the callers' empty-check produces a more specific error
// message.
func isValidActorString(s string) bool {
	if s == "" {
		return false
	}
	a := auth.ParseActor(s)
	switch a.Kind() {
	case auth.KindUser, auth.KindGroup, auth.KindSystem, auth.KindAdmin:
		return a.ID() != ""
	default:
		return false
	}
}

// newIdempotencyKey lives in helpers.go (consolidated in the round-3
// Wave-2 dedupe).
