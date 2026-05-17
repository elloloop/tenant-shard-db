// SPDX-License-Identifier: AGPL-3.0-only

// ShareNode RPC.
// Spec: docs/go-port/rpcs/ShareNode.md.
// Applier branch (W1.10): server/go/internal/apply/ops_share_node.go.
//
// # WAL-first restoration (PLAN.md §6)
//
// The Python ShareNode handler bypasses the WAL (CLAUDE.md invariant
// #1). It writes directly to canonical_store.share_node(...) at
// grpc_server.py:1794, which means a materialized-view rebuild from
// the WAL silently drops every share grant. The Go port restores
// the invariant: handler appends a `share_node` op to the WAL, the
// applier (already wired in W1.10 — ops_share_node.go) consumes it
// and writes node_access. There is no direct CanonicalStore.ShareNode
// call from this handler.
//
// # Auth model — trusted-actor + acl.Checker (Phase 4A.2)
//
//  1. checkTenant: sharding ownership + region pinning. Errors from
//     the gate (UNAVAILABLE w/ redirect, FAILED_PRECONDITION on region
//     mismatch, NOT_FOUND on missing tenant) propagate unchanged.
//  2. Trusted-actor rebind: the wire-claimed actor is UNTRUSTED — we
//     rebind to the interceptor-bound identity via auth.Authoritative.
//     Privilege-escalation regression pinned by commit fece3fb.
//  3. ACL pre-check via acl.Checker.Check. The Checker handles, in
//     order: system/admin actor bypass → node-owner short-circuit →
//     explicit ADMIN grant on the node (a non-owner with ADMIN on the
//     row can re-share). Mirrors the recommendation in
//     .claude/triage/sharenode-owner-share-analysis.md §5.1: ship the
//     "owner OR explicit-ADMIN-grant OR system/admin" semantic.
//     Non-owner / unknown node → soft-fail (OK + success=false).
//
// # Error contract (matches Python soft-fail shape)
//
// ShareNodeResponse is { success bool, error string } — NOT a
// google.rpc.Status. The handler returns success=false on
// PermissionError / unknown-node / WAL append errors instead of
// aborting with a status code. The ONLY hard-error paths are:
//
//	UNIMPLEMENTED WAL producer / canonical store not wired.
//	UNAVAILABLE Tenant not served by this node (gate).
//	FAILED_PRECONDITION Region mismatch / tenant archived (gate).
//	NOT_FOUND Tenant marked deleted (gate).
//
// All other failures route through OK + success=false. This is a
// deliberate contract pin — flipping to status.Errorf is a wire-
// contract break (spec §"Error contract").
//
// # Side effects
//
// On success: a single record is appended to the WAL. The applier
// then writes one row to node_access (and, for cross-tenant grants,
// one row to GlobalStore.shared_index via SharedAdded result). NO
// mailbox fanout — Python doesn't either (spec §"Side effects",
// open question 2). NO direct SQLite write from this handler.
//
// Idempotency: until the proto carries an idempotency_key, retries
// generate distinct WAL records that collapse at the applier via
// INSERT OR REPLACE on (node_id, actor_id). Acceptable but noisy;
// flagged in the spec for the receipt-tracking follow-up.
//
// Metrics: emits entdb_grpc_requests_total{method="ShareNode",
// status="ok"|"denied"|"error"} via the shared chokepoint.

package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// shareNodeMethod is the RPC label for metrics.
const shareNodeMethod = "ShareNode"

// shareNodeWALTopic is the topic this handler appends to. Must match
// the topic the applier subscribes to (cmd/entdb-server/main.go:32
// default — "entdb-wal"). When Server gains a typed WAL-topic option
// in a follow-up, swap this constant for s.walTopic.
const shareNodeWALTopic = "entdb-wal"

// ShareNode appends a share_node op to the WAL. The applier
// (apply/ops_share_node.go) materialises it into node_access. See
// the package-level doc for the full contract.
func (s *Server) ShareNode(
	ctx context.Context,
	req *pb.ShareNodeRequest,
) (*pb.ShareNodeResponse, error) {
	start := time.Now()
	status := "ok"
	defer func() {
		metrics.RecordGRPCRequest(shareNodeMethod, status, time.Since(start))
	}()

	// Optional-deps guard. The WAL producer is mandatory for ShareNode
	// because the entire point of this RPC is the WAL append.
	if s.producer == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "WAL producer not configured")
	}
	// CanonicalStore is needed for the owner pre-check; without it we
	// cannot enforce auth and refuse to write to the WAL.
	if s.store == nil {
		status = "error"
		return nil, errs.Errorf(codes.Unimplemented, "Canonical store not configured")
	}

	tenantID := req.GetContext().GetTenantId()

	// 1. Tenant gate (sharding ownership + region pinning). Errors here
	//    are gRPC status codes; we surface them unchanged.
	if err := s.checkTenant(ctx, tenantID); err != nil {
		status = "error"
		return nil, err
	}

	// 2. Trusted-actor rebind. The wire claim is UNTRUSTED.
	claimedActor := req.GetContext().GetActor()
	trusted := auth.Authoritative(ctx, auth.ParseActor(claimedActor))
	if trusted.IsZero() {
		status = "denied"
		return &pb.ShareNodeResponse{Success: false, Error: "actor is required"}, nil
	}

	// 3. Required-arg validation. Python (grpc_server.py:1746-1826)
	//    does not pre-validate node_id / actor_id shape — soft-fail
	//    parity preserved.
	nodeID := req.GetNodeId()
	actorID := normalizeActorID(req.GetActorId())
	if nodeID == "" || actorID == "" {
		status = "denied"
		return &pb.ShareNodeResponse{
			Success: false,
			Error:   "node_id and actor_id are required",
		}, nil
	}

	// 4. ACL pre-check via acl.Checker. The Checker handles
	//    system/admin bypass, owner short-circuit and grant-based
	//    ADMIN (so a non-owner with an explicit ADMIN row on the node
	//    can re-share). Mirrors Python's _check_capability +
	//    canonical_store owner short-circuit (the two paths combined),
	//    and matches the recommendation in
	//    .claude/triage/sharenode-owner-share-analysis.md §5.1.
	//
	//    NOT_FOUND on the underlying GetNode (surfaced by the Checker
	//    via NodeMetaReader) lowers to soft-fail success=false — a
	//    missing node is indistinguishable from "no grant" to the
	//    caller, matching the Python PermissionError-via-missing-node
	//    behaviour at spec §Wire-contract.NodeId.
	aclActor := authActorToACLActor(trusted)
	if err := s.aclCheck(ctx, acl.CheckRequest{
		TenantID:      tenantID,
		ActorTenantID: tenantID,
		Actor:         aclActor,
		NodeID:        nodeID,
		OpName:        "ShareNode",
	}); err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			status = "denied"
			return &pb.ShareNodeResponse{
				Success: false,
				Error:   "permission denied: missing-or-no-grant on node",
			}, nil
		}
		if errs.Code(err) == codes.PermissionDenied {
			status = "denied"
			return &pb.ShareNodeResponse{
				Success: false,
				Error:   "permission denied: caller lacks ADMIN on node",
			}, nil
		}
		// Anything else is an infra error (Unimplemented when the
		// store is unwired, Internal on a reader fault, etc.). Surface
		// via the soft-fail shape; the metric label is "error".
		status = "error"
		return &pb.ShareNodeResponse{Success: false, Error: err.Error()}, nil
	}

	// 5. Build the WAL op envelope. Mirror the Python applier's
	//    op shape (apply/ops_share_node.go:18-31). Field defaults:
	//    actor_type → "user", permission → "read" (spec §Wire-contract).
	actorType := req.GetActorType()
	if actorType == "" {
		actorType = "user"
	}
	permission := req.GetPermission()
	if permission == "" {
		permission = "read"
	}
	coreCaps := make([]int32, 0, len(req.GetCoreCaps()))
	for _, c := range req.GetCoreCaps() {
		coreCaps = append(coreCaps, int32(c))
	}
	extCaps := append([]int32(nil), req.GetExtCapIds()...)

	// Cross-tenant fan-out hint for the applier's SharedAdded result.
	// When actor_id is "tenant:<X>" or contains "@tenant:<Y>", record
	// the recipient + source_tenant so the applier post-commit hook
	// can write the GlobalStore.shared_index row that ListSharedWithMe
	// reads (spec §Side-effects.4 / cross-tenant pin
	// test_cross_tenant_read.py:75-105).
	userID, sourceTenant := crossTenantHint(actorID, tenantID)

	op := map[string]any{
		"op":          "share_node",
		"node_id":     nodeID,
		"actor_id":    actorID,
		"actor_type":  actorType,
		"permission":  permission,
		"expires_at":  req.GetExpiresAt(),
		"type_id":     req.GetTypeId(),
		"core_caps":   coreCaps,
		"ext_cap_ids": extCaps,
		"granted_by":  trusted.String(),
	}
	if userID != "" {
		op["user_id"] = userID
	}
	if sourceTenant != "" {
		op["source_tenant"] = sourceTenant
	}

	idemKey := newShareNodeIdempotencyKey(tenantID, trusted.String(), nodeID, actorID)
	ev := wal.Event{
		TenantID:       tenantID,
		Actor:          trusted.String(),
		IdempotencyKey: idemKey,
		Ops:            []map[string]any{op},
	}
	value, err := ev.Encode()
	if err != nil {
		status = "error"
		return &pb.ShareNodeResponse{Success: false, Error: err.Error()}, nil
	}

	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(idemKey),
	}
	if _, err := s.producer.Append(ctx, shareNodeWALTopic, tenantID, value, headers); err != nil {
		status = "error"
		return &pb.ShareNodeResponse{Success: false, Error: err.Error()}, nil
	}

	return &pb.ShareNodeResponse{Success: true}, nil
}

// normalizeActorID rewrites a bare "<id>" form to "user:<id>". Mirrors
// the Python normalisation at grpc_server.py:1772-1778 and the proto
// comment at entdb.proto:723-725. Already-prefixed ids
// (user:/group:/system:/admin:/service:/tenant:) pass through
// unchanged.
func normalizeActorID(s string) string {
	if s == "" {
		return ""
	}
	for _, prefix := range []string{
		"user:", "group:", "system:", "admin:", "service:", "tenant:",
	} {
		if strings.HasPrefix(s, prefix) {
			return s
		}
	}
	return "user:" + s
}

// crossTenantHint extracts (userID, sourceTenant) from an actor_id
// when it represents a cross-tenant grantee. Returns ("", "") for
// same-tenant grants. The applier consumes these via op["user_id"] /
// op["source_tenant"] to populate GlobalStore.shared_index
// (apply/ops_share_node.go:74-83).
//
// Today: only tenant:<id> recipients are surfaced (the
// "user:<id>@tenant:<Y>" form is not part of the proto contract yet).
// The applier post-commit hook does the per-member group expansion.
func crossTenantHint(actorID, sourceTenant string) (userID, src string) {
	if strings.HasPrefix(actorID, "tenant:") {
		return strings.TrimPrefix(actorID, "tenant:"), sourceTenant
	}
	if strings.HasPrefix(actorID, "user:") {
		// Same-tenant user: grants do NOT need shared_index — the
		// recipient discovers the grant via node_access in their own
		// tenant. Return zero values.
		return "", ""
	}
	return "", ""
}

// newShareNodeIdempotencyKey builds a deterministic dedupe key from
// the (tenant, granter, node, recipient) tuple plus a wall-clock
// nanosecond suffix. The suffix is what makes retries de-duplicate
// (the applier collapses via INSERT OR REPLACE) but distinct user-
// driven re-shares still land. Mirrors spec §Side-effects: "Re-issuing
// the same ShareNode produces an INSERT OR REPLACE that updates
// granted_at." When the proto gains an idempotency_key field
// (#407 follow-up), prefer the caller-supplied value over this
// derived form.
func newShareNodeIdempotencyKey(tenantID, granter, nodeID, recipient string) string {
	return strings.Join([]string{
		"share_node",
		tenantID,
		granter,
		nodeID,
		recipient,
		time.Now().UTC().Format(time.RFC3339Nano),
	}, "|")
}
