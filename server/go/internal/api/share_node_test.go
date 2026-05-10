// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the ShareNode RPC. Spec: docs/go-port/rpcs/ShareNode.md.
// Behavioural pins (mirror Python contract suite):
//
//   - Owner-as-trusted-actor happy path (test_grpc_contract.py:327-338).
//   - Non-owner / non-admin → soft-fail success=false, error contains
//     "permission denied" (Python: PermissionError → success=false at
//     grpc_server.py:1791-1793).
//   - Unknown node id → soft-fail success=false (spec §Wire-contract.NodeId
//     "auth check carries the existence signal").
//   - Idempotent re-share: two ShareNode calls with the same shape both
//     succeed; the WAL records two events but the applier collapses
//     them via INSERT OR REPLACE (canonical_store.py:2989).
//   - Cross-tenant recipient (tenant:<id>) lands a user_id /
//     source_tenant hint into the WAL op so the applier can populate
//     GlobalStore.shared_index (spec §Side-effects.4 / cross-tenant pin
//     test_cross_tenant_read.py:75-105). This is the closest analogue
//     to a "recipient mailbox flag" — ShareNode does NOT fan a
//     notification into the recipient's mailbox today (spec §Side-
//     effects "Mailbox fanout: ShareNode does NOT currently fan a
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
// entdb.proto:723-725 and grpc_server.py:1772-1778.
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
// not own. Mirrors the system/admin short-circuit at
// grpc_server.py:498-499 / 341-347.
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
// records via INSERT OR REPLACE — pinned at the handler level by simply
// not failing on the second call. Spec §Side-effects: "Re-issuing the
// same ShareNode produces an INSERT OR REPLACE that updates
// granted_at."
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
// Cross-tenant pin: test_cross_tenant_read.py:75-105.
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
// are populated on the request, they must reach the WAL op verbatim
// (spec contract pin test_acl_capabilities.py:60-82). Numeric values
// arrive as JSON numbers (float64) on the consumer side — the test
// asserts the encoded form rather than the typed slice.
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
