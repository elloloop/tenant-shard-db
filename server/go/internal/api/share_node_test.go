// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the ShareNode RPC. Spec: docs/go-port/rpcs/ShareNode.md.
// Behavioural pins:
//
//   - Owner-as-trusted-actor happy path (test_grpc_contract.py:327-338).
//   - Non-owner / non-admin → soft-fail success=false, error contains
//     "permission denied".
//   - Unknown node id → soft-fail success=false (spec §Wire-contract.NodeId
//     "auth check carries the existence signal").
//   - Idempotent re-share: two ShareNode calls with the same shape both
//     succeed; the WAL records two events but the applier collapses
//     them via INSERT OR REPLACE.
//   - Cross-tenant recipient (tenant:<id>) lands a user_id /
//     source_tenant hint into the WAL op so the applier can populate
//     GlobalStore.shared_index (spec §Side-effects.4). This is the
//     closest analogue to a "recipient mailbox flag" — ShareNode does
//     NOT fan a notification into the recipient's mailbox today (spec
//     §Side-effects "Mailbox fanout: ShareNode does NOT currently fan a
//     notification into the recipient's mailbox SQLite").

package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// shareNodeFixture wires a Server with a CanonicalStore + GlobalStore +
// in-memory WAL producer, opens one tenant, and seeds one node owned by
// "user:alice". Returns (server, producer, ctx) so tests can assert
// against the WAL contents directly.
func shareNodeFixture(t *testing.T) (*api.Server, *wal.InMemory, context.Context) {
	t.Helper()
	ctx := context.Background()

	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if err := cs.OpenTenant(ctx, "acme"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, "acme", store.NodeInput{
		NodeID:     "doc-1",
		TypeID:     7,
		Payload:    map[string]any{"1": "hello"},
		OwnerActor: "user:alice",
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	producer := wal.NewInMemory(0)
	if err := producer.Connect(ctx); err != nil {
		t.Fatalf("producer.Connect: %v", err)
	}
	t.Cleanup(func() { _ = producer.Close(ctx) })

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
		api.WithWALProducer(producer),
	)
	return srv, producer, ctx
}

// shareNodeOps reads every record on the WAL "entdb-wal" topic, decodes
// the events, and returns their op slices in order. Used to assert the
// shape of what the handler appended.
func shareNodeOps(t *testing.T, p *wal.InMemory) [][]map[string]any {
	t.Helper()
	recs, err := p.PollBatch(context.Background(), "entdb-wal", "test-reader", 64, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	out := make([][]map[string]any, 0, len(recs))
	for _, r := range recs {
		ev, err := wal.DecodeEvent(r.Value)
		if err != nil {
			t.Fatalf("DecodeEvent: %v", err)
		}
		out = append(out, ev.Ops)
	}
	return out
}

// TestShareNode_OwnerHappyPath: alice (the node owner) shares doc-1
// with user:bob. Response is success=true; one record lands on the WAL
// with op="share_node" and the expected fields. Mirrors
// test_grpc_contract.py:327-338.
func TestShareNode_OwnerHappyPath(t *testing.T) {
	t.Parallel()

	srv, producer, ctx := shareNodeFixture(t)

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{
			TenantId: "acme",
			Actor:    "user:alice",
		},
		NodeId:     "doc-1",
		ActorId:    "user:bob",
		Permission: "read",
	})
	if err != nil {
		t.Fatalf("ShareNode: unexpected gRPC error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ShareNode: success=false, error=%q; want success=true", resp.GetError())
	}
	if resp.GetError() != "" {
		t.Errorf("ShareNode: unexpected error message %q", resp.GetError())
	}

	ops := shareNodeOps(t, producer)
	if len(ops) != 1 {
		t.Fatalf("WAL records: got %d, want 1", len(ops))
	}
	if len(ops[0]) != 1 {
		t.Fatalf("ops in record 0: got %d, want 1", len(ops[0]))
	}
	op := ops[0][0]
	if op["op"] != "share_node" {
		t.Errorf("op type = %q; want %q", op["op"], "share_node")
	}
	if op["node_id"] != "doc-1" {
		t.Errorf("op.node_id = %q; want %q", op["node_id"], "doc-1")
	}
	if op["actor_id"] != "user:bob" {
		t.Errorf("op.actor_id = %q; want %q", op["actor_id"], "user:bob")
	}
	if op["actor_type"] != "user" {
		t.Errorf("op.actor_type = %q; want %q", op["actor_type"], "user")
	}
	if op["permission"] != "read" {
		t.Errorf("op.permission = %q; want %q", op["permission"], "read")
	}
	if op["granted_by"] != "user:alice" {
		t.Errorf("op.granted_by = %q; want %q", op["granted_by"], "user:alice")
	}
}

// TestShareNode_BareActorIDNormalized: a bare "<id>" recipient is
// rewritten to "user:<id>" before the WAL append, mirroring
// entdb.proto:723-725.
func TestShareNode_BareActorIDNormalized(t *testing.T) {
	t.Parallel()

	srv, producer, ctx := shareNodeFixture(t)

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:  "doc-1",
		ActorId: "bob", // bare form
	})
	if err != nil {
		t.Fatalf("ShareNode: unexpected gRPC error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ShareNode: success=false, error=%q", resp.GetError())
	}

	ops := shareNodeOps(t, producer)
	if len(ops) != 1 {
		t.Fatalf("WAL records: got %d, want 1", len(ops))
	}
	if got := ops[0][0]["actor_id"]; got != "user:bob" {
		t.Errorf("normalised actor_id = %q; want %q", got, "user:bob")
	}
}

// TestShareNode_NonOwnerSoftFail: a user: actor that does NOT own the
// node receives success=false with a "permission denied" error, NOT a
// gRPC PERMISSION_DENIED status. Mirrors the soft-fail shape pinned by
// spec §Error-contract.
func TestShareNode_NonOwnerSoftFail(t *testing.T) {
	t.Parallel()

	srv, producer, ctx := shareNodeFixture(t)

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:carol"},
		NodeId:  "doc-1",
		ActorId: "user:bob",
	})
	if err != nil {
		t.Fatalf("ShareNode: unexpected gRPC error: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("ShareNode (non-owner): success=true; want false")
	}
	if !strings.Contains(resp.GetError(), "permission denied") {
		t.Errorf("error = %q; want substring %q", resp.GetError(), "permission denied")
	}

	// No record should have been appended on the deny path.
	if ops := shareNodeOps(t, producer); len(ops) != 0 {
		t.Errorf("WAL: %d records appended on deny; want 0", len(ops))
	}
}

// TestShareNode_UnknownNodeSoftFail: sharing a node that does not exist
// returns success=false (no PermissionError leaks; spec
// §Wire-contract.NodeId "auth check carries the existence signal").
// Crucially, NO record is appended to the WAL — the handler must not
// pollute the log with un-applicable events.
func TestShareNode_UnknownNodeSoftFail(t *testing.T) {
	t.Parallel()

	srv, producer, ctx := shareNodeFixture(t)

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:  "ghost",
		ActorId: "user:bob",
	})
	if err != nil {
		t.Fatalf("ShareNode: unexpected gRPC error: %v", err)
	}
	if resp.GetSuccess() {
		t.Errorf("ShareNode (unknown node): success=true; want false")
	}
	if resp.GetError() == "" {
		t.Errorf("ShareNode (unknown node): empty error message; want a permission/missing-grant note")
	}
	if ops := shareNodeOps(t, producer); len(ops) != 0 {
		t.Errorf("WAL: %d records appended on unknown-node deny; want 0", len(ops))
	}
}

// TestShareNode_AdminBypass: an admin: actor may share any node it does
// not own.
func TestShareNode_AdminBypass(t *testing.T) {
	t.Parallel()

	srv, _, ctx := shareNodeFixture(t)

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "admin:root"},
		NodeId:  "doc-1",
		ActorId: "user:bob",
	})
	if err != nil {
		t.Fatalf("ShareNode (admin): unexpected gRPC error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ShareNode (admin): success=false, error=%q", resp.GetError())
	}
}

// TestShareNode_IdempotentReshare: re-sharing the same (node, recipient)
// tuple twice each returns success=true. The applier collapses the two
// records via INSERT OR REPLACE. Spec §Side-effects: "Re-issuing the
// same ShareNode produces an INSERT OR REPLACE that updates granted_at."
func TestShareNode_IdempotentReshare(t *testing.T) {
	t.Parallel()

	srv, producer, ctx := shareNodeFixture(t)

	for i, perm := range []string{"read", "write"} {
		resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
			Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
			NodeId:     "doc-1",
			ActorId:    "user:bob",
			Permission: perm,
		})
		if err != nil {
			t.Fatalf("iter %d: ShareNode: %v", i, err)
		}
		if !resp.GetSuccess() {
			t.Fatalf("iter %d: success=false, error=%q", i, resp.GetError())
		}
	}

	// Both records should appear on the WAL — the handler appends one
	// per call. Applier-side dedupe is applier_test territory.
	ops := shareNodeOps(t, producer)
	if len(ops) != 2 {
		t.Fatalf("WAL records: got %d, want 2 (handler does not dedupe; applier does)", len(ops))
	}
	if ops[0][0]["permission"] != "read" || ops[1][0]["permission"] != "write" {
		t.Errorf("permissions in order: %q,%q; want read,write",
			ops[0][0]["permission"], ops[1][0]["permission"])
	}
}

// TestShareNode_CrossTenantRecipient: sharing with a "tenant:<X>"
// recipient surfaces user_id + source_tenant hints in the op so the
// applier can populate GlobalStore.shared_index. This is the closest
// proxy for "recipient mailbox flag" — ShareNode does NOT fan a true
// mailbox notification (spec §Side-effects "Mailbox fanout: ShareNode
// does NOT currently fan a notification into the recipient's mailbox
// SQLite"); the WAL hint is what makes ListSharedWithMe see the share.
func TestShareNode_CrossTenantRecipient(t *testing.T) {
	t.Parallel()

	srv, producer, ctx := shareNodeFixture(t)

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:  "doc-1",
		ActorId: "tenant:globex",
	})
	if err != nil {
		t.Fatalf("ShareNode: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ShareNode (cross-tenant): success=false, error=%q", resp.GetError())
	}

	ops := shareNodeOps(t, producer)
	if len(ops) != 1 {
		t.Fatalf("WAL records: got %d, want 1", len(ops))
	}
	op := ops[0][0]
	if op["actor_id"] != "tenant:globex" {
		t.Errorf("op.actor_id = %q; want %q", op["actor_id"], "tenant:globex")
	}
	if op["user_id"] != "globex" {
		t.Errorf("op.user_id (cross-tenant hint) = %q; want %q", op["user_id"], "globex")
	}
	if op["source_tenant"] != "acme" {
		t.Errorf("op.source_tenant (cross-tenant hint) = %q; want %q", op["source_tenant"], "acme")
	}
}

// TestShareNode_TypedCapsPersistedVerbatim: when CoreCaps and ExtCapIds
// are populated on the request, they must reach the WAL op verbatim.
// Numeric values arrive as JSON numbers (float64) on the consumer side —
// the test asserts the encoded form rather than the typed slice.
func TestShareNode_TypedCapsPersistedVerbatim(t *testing.T) {
	t.Parallel()

	srv, producer, ctx := shareNodeFixture(t)

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:  "doc-1",
		ActorId: "user:bob",
		CoreCaps: []pb.CoreCapability{
			pb.CoreCapability_CORE_CAP_READ,
			pb.CoreCapability_CORE_CAP_EDIT,
		},
		ExtCapIds: []int32{42, 99},
	})
	if err != nil {
		t.Fatalf("ShareNode: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ShareNode: success=false, error=%q", resp.GetError())
	}

	ops := shareNodeOps(t, producer)
	if len(ops) != 1 {
		t.Fatalf("WAL records: got %d, want 1", len(ops))
	}
	core, _ := json.Marshal(ops[0][0]["core_caps"])
	ext, _ := json.Marshal(ops[0][0]["ext_cap_ids"])
	// JSON numbers → float64, so we compare the encoded form.
	if string(core) != "[1,3]" {
		t.Errorf("op.core_caps JSON = %s; want [1,3]", core)
	}
	if string(ext) != "[42,99]" {
		t.Errorf("op.ext_cap_ids JSON = %s; want [42,99]", ext)
	}
}

// TestShareNode_NoProducerUnimplemented: when the WAL producer is not
// wired, the handler aborts with UNIMPLEMENTED (spec §Implementation —
// the handler refuses to bypass the WAL).
func TestShareNode_NoProducerUnimplemented(t *testing.T) {
	t.Parallel()

	srv := api.New() // no WithWALProducer / WithStore

	_, err := srv.ShareNode(context.Background(), &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
		NodeId:  "doc-1",
		ActorId: "user:bob",
	})
	if err == nil {
		t.Fatalf("ShareNode: expected UNIMPLEMENTED, got nil")
	}
	if got := errs.Code(err); got != codes.Unimplemented {
		t.Fatalf("ShareNode: code = %v; want Unimplemented (err=%v)", got, err)
	}
}

// shareNodeFixtureWithStore is shareNodeFixture but also returns the
// underlying *store.CanonicalStore so tests can seed node_access rows
// directly (the path normally taken by the applier). Used by the
// Phase 4A.2 grant-based ADMIN tests where we want bob to have a
// pre-existing ADMIN row before the gRPC call.
func shareNodeFixtureWithStore(t *testing.T) (*api.Server, *store.CanonicalStore, *wal.InMemory, context.Context) {
	t.Helper()
	ctx := context.Background()

	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if err := cs.OpenTenant(ctx, "acme"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, "acme", store.NodeInput{
		NodeID:     "doc-1",
		TypeID:     7,
		Payload:    map[string]any{"1": "hello"},
		OwnerActor: "user:alice",
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	producer := wal.NewInMemory(0)
	if err := producer.Connect(ctx); err != nil {
		t.Fatalf("producer.Connect: %v", err)
	}
	t.Cleanup(func() { _ = producer.Close(ctx) })

	srv := api.New(
		api.WithStore(cs),
		api.WithGlobalStore(gs),
		api.WithWALProducer(producer),
	)
	return srv, cs, producer, ctx
}

// TestShareNode_NonOwnerWithAdminGrant: a user that is NOT the node
// owner but holds an explicit ADMIN grant on the node can re-share.
// Phase 4A.2 expansion: before this change the handler enforced an
// owner-only branch that rejected this case. The acl.Checker handles
// owner short-circuit + grant-based ADMIN; wiring it in restores the
// "ADMIN means I can delegate" semantic that every adjacent system
// uses (see .claude/triage/sharenode-owner-share-analysis.md §3 #3).
func TestShareNode_NonOwnerWithAdminGrant(t *testing.T) {
	t.Parallel()

	srv, cs, producer, ctx := shareNodeFixtureWithStore(t)

	// Pre-seed: bob has an ADMIN grant on doc-1 (the fixture has
	// alice as owner). Direct store.ShareNode write since this is
	// fixture-stage setup, not a re-entry through the gRPC layer.
	if err := cs.ShareNode(ctx, "acme", store.ShareNodeInput{
		NodeID:    "doc-1",
		ActorID:   "user:bob",
		ActorType: "user",
		// CORE_CAP_ADMIN = 5. Persisted as a typed-cap row so the
		// acl.Checker's RequiredForOp(ShareNode) -> CoreCapAdmin
		// requirement is satisfied without depending on legacy
		// permission-string back-fill.
		CoreCaps:  []int32{5},
		GrantedBy: "user:alice",
	}); err != nil {
		t.Fatalf("seed ShareNode (admin grant for bob): %v", err)
	}

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context:    &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		NodeId:     "doc-1",
		ActorId:    "user:charlie",
		Permission: "read",
	})
	if err != nil {
		t.Fatalf("ShareNode: unexpected gRPC error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("ShareNode (non-owner ADMIN grantee): success=false, error=%q; want success=true",
			resp.GetError())
	}

	ops := shareNodeOps(t, producer)
	if len(ops) != 1 {
		t.Fatalf("WAL records: got %d, want 1", len(ops))
	}
	op := ops[0][0]
	if op["actor_id"] != "user:charlie" {
		t.Errorf("op.actor_id = %q; want %q", op["actor_id"], "user:charlie")
	}
	if op["granted_by"] != "user:bob" {
		t.Errorf("op.granted_by = %q; want %q (the re-sharing admin)", op["granted_by"], "user:bob")
	}
}

// TestShareNode_NonOwnerWithReadGrant: a user with ONLY a READ grant
// on the node cannot re-share. Phase 4A.2: pin the lower bound of the
// acl.Checker grant-walk — READ does not satisfy CORE_CAP_ADMIN, so
// the soft-fail "permission denied" path fires and NO WAL record is
// written.
func TestShareNode_NonOwnerWithReadGrant(t *testing.T) {
	t.Parallel()

	srv, cs, producer, ctx := shareNodeFixtureWithStore(t)

	if err := cs.ShareNode(ctx, "acme", store.ShareNodeInput{
		NodeID:    "doc-1",
		ActorID:   "user:bob",
		ActorType: "user",
		// CORE_CAP_READ = 1. No ADMIN bit set.
		CoreCaps:  []int32{1},
		GrantedBy: "user:alice",
	}); err != nil {
		t.Fatalf("seed ShareNode (read grant for bob): %v", err)
	}

	resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
		Context: &pb.RequestContext{TenantId: "acme", Actor: "user:bob"},
		NodeId:  "doc-1",
		ActorId: "user:charlie",
	})
	if err != nil {
		t.Fatalf("ShareNode: unexpected gRPC error: %v", err)
	}
	if resp.GetSuccess() {
		t.Fatalf("ShareNode (READ-only grantee): success=true; want false")
	}
	if !strings.Contains(resp.GetError(), "permission denied") {
		t.Errorf("error = %q; want substring %q", resp.GetError(), "permission denied")
	}
	// No record should have been appended on the deny path.
	if ops := shareNodeOps(t, producer); len(ops) != 0 {
		t.Errorf("WAL: %d records appended on deny; want 0", len(ops))
	}
}

// TestShareNode_EmptyNodeOrActorSoftFail: empty node_id or actor_id
// surface as soft-fail success=false rather than INVALID_ARGUMENT —
// matches Python's "no shape validation" behaviour (spec §Error-
// contract: "Python doesn't validate node_id/actor_id shape today").
func TestShareNode_EmptyNodeOrActorSoftFail(t *testing.T) {
	t.Parallel()

	srv, _, ctx := shareNodeFixture(t)

	for _, tc := range []struct {
		name    string
		nodeID  string
		actorID string
	}{
		{"empty-node", "", "user:bob"},
		{"empty-actor", "doc-1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.ShareNode(ctx, &pb.ShareNodeRequest{
				Context: &pb.RequestContext{TenantId: "acme", Actor: "user:alice"},
				NodeId:  tc.nodeID,
				ActorId: tc.actorID,
			})
			if err != nil {
				t.Fatalf("ShareNode: unexpected gRPC error: %v", err)
			}
			if resp.GetSuccess() {
				t.Fatalf("ShareNode: success=true; want false")
			}
		})
	}
}
