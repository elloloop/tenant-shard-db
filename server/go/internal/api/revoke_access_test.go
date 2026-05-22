// Tests for RevokeAccess. Behavioural parity with the Python handler
// is pinned by the cross-language contract suite at
// tests/python/integration/test_grpc_contract.py:351-361. This file covers
// the three branches the spec calls out:
//
//  1. Happy revoke -> OK + Found=true; WAL event durably appended.
//  2. Idempotent revoke-not-granted -> OK + Found=true; WAL event
//     appended (apply is a no-op DELETE — applier-side concern).
//  3. Non-admin caller -> response.Error="permission denied: ..."
//     (response-error convention; no gRPC status error).

package api_test

import (
	"context"
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// revokeAccessTestTopic must match the topic the handler appends to
// (see revoke_access.go:revokeAccessTopic). Hardcoded mirror is fine —
// the test file is colocated with the handler, so a topic-rename will
// be caught by the constant going out of sync at the same review step.
const revokeAccessTestTopic = "entdb-wal"

// newRevokeAccessServer wires a server with a globalstore + in-memory
// WAL producer + sharding gate. The returned wal.InMemory lets tests
// assert on the appended events.
func newRevokeAccessServer(t *testing.T) (*api.Server, *wal.InMemory) {
	t.Helper()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	w := wal.NewInMemory(0)
	if err := w.Connect(ctx); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(context.Background()) })

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithWALProducer(w),
		api.WithSharding(&tenant.Sharding{NodeID: "node-a"}),
	)
	return srv, w
}

// TestRevokeAccess_HappyRevoke: a trusted admin: actor revokes a grant
// on a node; handler returns OK + Found=true and a "revoke_access" op
// is durably appended to the WAL.
func TestRevokeAccess_HappyRevoke(t *testing.T) {
	t.Parallel()

	srv, w := newRevokeAccessServer(t)
	authedCtx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodAPIKey,
		Subject: "admin:root",
	})

	resp, err := srv.RevokeAccess(authedCtx, &pb.RevokeAccessRequest{
		Context: &pb.RequestContext{
			TenantId: "acme",
			Actor:    "admin:root",
		},
		NodeId:  "node-1",
		ActorId: "user:bob",
	})
	if err != nil {
		t.Fatalf("RevokeAccess: unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("RevokeAccess: nil response")
	}
	if !resp.GetFound() {
		t.Fatalf("RevokeAccess: Found=false, Error=%q; want Found=true", resp.GetError())
	}
	if resp.GetError() != "" {
		t.Fatalf("RevokeAccess: Error=%q; want empty", resp.GetError())
	}

	// One WAL record must be present, encoding the revoke_access op.
	recs := w.GetAllRecords(revokeAccessTestTopic)
	if len(recs) != 1 {
		t.Fatalf("WAL: got %d records, want 1", len(recs))
	}
	ev, derr := wal.DecodeEvent(recs[0].Value)
	if derr != nil {
		t.Fatalf("DecodeEvent: %v", derr)
	}
	if ev.TenantID != "acme" {
		t.Fatalf("event TenantID = %q; want acme", ev.TenantID)
	}
	if len(ev.Ops) != 1 {
		t.Fatalf("event Ops len = %d; want 1", len(ev.Ops))
	}
	op := ev.Ops[0]
	if op["op"] != "revoke_access" {
		t.Fatalf("op[op] = %v; want revoke_access", op["op"])
	}
	if op["node_id"] != "node-1" {
		t.Fatalf("op[node_id] = %v; want node-1", op["node_id"])
	}
	if op["actor_id"] != "user:bob" {
		t.Fatalf("op[actor_id] = %v; want user:bob", op["actor_id"])
	}
}

// TestRevokeAccess_IdempotentRevokeNotGranted: revoking a grant that
// never existed returns OK + Found=true (success=true, no-op at apply
// time). Two back-to-back revokes for the same (node, actor) collapse
// to one WAL record via the deterministic idempotency key — matching
// the spec's idempotency contract (§"Open questions" item 2).
func TestRevokeAccess_IdempotentRevokeNotGranted(t *testing.T) {
	t.Parallel()

	srv, w := newRevokeAccessServer(t)
	authedCtx := auth.WithIdentity(context.Background(), auth.Identity{
		Method:  auth.MethodAPIKey,
		Subject: "admin:root",
	})

	req := &pb.RevokeAccessRequest{
		Context: &pb.RequestContext{
			TenantId: "acme",
			Actor:    "admin:root",
		},
		NodeId:  "ghost-node",
		ActorId: "user:nobody",
	}

	// First revoke against a non-existent grant.
	resp1, err := srv.RevokeAccess(authedCtx, req)
	if err != nil {
		t.Fatalf("RevokeAccess (first): %v", err)
	}
	if !resp1.GetFound() {
		t.Fatalf("RevokeAccess (first): Found=false, Error=%q; want Found=true (no-op success)", resp1.GetError())
	}
	if resp1.GetError() != "" {
		t.Fatalf("RevokeAccess (first): Error=%q; want empty", resp1.GetError())
	}

	// Second revoke for the same (tenant, node, actor) must dedupe at
	// the WAL via the deterministic idempotency key. Still success on
	// the wire.
	resp2, err := srv.RevokeAccess(authedCtx, req)
	if err != nil {
		t.Fatalf("RevokeAccess (second): %v", err)
	}
	if !resp2.GetFound() {
		t.Fatalf("RevokeAccess (second): Found=false, Error=%q; want Found=true", resp2.GetError())
	}
	if resp2.GetError() != "" {
		t.Fatalf("RevokeAccess (second): Error=%q; want empty", resp2.GetError())
	}

	// WAL must contain exactly one record despite two handler calls —
	// idempotent dedupe by (topic, key, idempotency-key).
	recs := w.GetAllRecords(revokeAccessTestTopic)
	if len(recs) != 1 {
		t.Fatalf("WAL: got %d records, want 1 (idempotent dedupe)", len(recs))
	}
}

// TestRevokeAccess_NonAdminPermissionDenied: a regular member without
// owner/admin role on the tenant gets a response-level error
// containing "permission denied", and NO WAL record is appended. The
// privilege-escalation regression (commit fece3fb) is pinned here by
// having bob's session attest user:bob while the wire claims
// actor=admin:root — the handler MUST ignore the wire claim.
//
// Note: the response-error convention (no gRPC status) is preserved
// for client compatibility.
func TestRevokeAccess_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()

	srv, w := newRevokeAccessServer(t)
	ctx := context.Background()

	// bob has no membership row in tenant_members at all — the default
	// "no role" state, which the handler treats as not owner / not
	// admin. We don't need to seed anything.

	// Privilege-escalation regression pin: bob's session attests
	// user:bob, but the request claims actor=admin:root. The handler
	// MUST authorise as user:bob and reject.
	authedCtx := auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodSession,
		Subject: "user:bob",
	})

	resp, err := srv.RevokeAccess(authedCtx, &pb.RevokeAccessRequest{
		Context: &pb.RequestContext{
			TenantId: "acme",
			Actor:    "admin:root", // wire-claimed escalation attempt
		},
		NodeId:  "node-1",
		ActorId: "user:carol",
	})
	if err != nil {
		t.Fatalf("RevokeAccess: expected nil gRPC error (response-error convention), got %v", err)
	}
	if resp == nil {
		t.Fatalf("RevokeAccess: nil response")
	}
	if resp.GetFound() {
		t.Fatalf("RevokeAccess: Found=true; want false on permission denied")
	}
	if !strings.Contains(strings.ToLower(resp.GetError()), "permission denied") {
		t.Fatalf("RevokeAccess: Error=%q; want it to contain 'permission denied'", resp.GetError())
	}

	// Crucially: no WAL append on the denied path. The privilege-
	// escalation guard MUST short-circuit before any WAL I/O.
	if recs := w.GetAllRecords(revokeAccessTestTopic); len(recs) != 0 {
		t.Fatalf("WAL: got %d records on denied path, want 0", len(recs))
	}
}
