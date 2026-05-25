// Tests for the USER_MAILBOX read surface (#568): storage of
// storage_mode/target_user_id and the mailbox-scoped read paths
// (GetMailboxNode, QueryNodes with MailboxUser, SearchMailboxNodes).

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const mailboxTypeID int32 = 7

// seedMailboxNode creates a USER_MAILBOX node for targetUser and indexes
// its searchable field (id 1) so SearchMailboxNodes can match it.
func seedMailboxNode(t *testing.T, cs *store.CanonicalStore, tenantID, targetUser, nodeID, body string) {
	t.Helper()
	ctx := context.Background()
	if err := cs.EnsureFTSIndex(ctx, tenantID, mailboxTypeID, []uint32{1}); err != nil {
		t.Fatalf("EnsureFTSIndex: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:       nodeID,
		TypeID:       mailboxTypeID,
		OwnerActor:   "user:" + targetUser,
		Payload:      map[string]any{"1": body},
		StorageMode:  int32(store.StorageModeUserMailbox),
		TargetUserID: targetUser,
	}); err != nil {
		t.Fatalf("CreateNodeRaw(%q): %v", nodeID, err)
	}
	if err := cs.FTSInsert(ctx, tenantID, mailboxTypeID, nodeID,
		map[string]any{"1": body}, []uint32{1}); err != nil {
		t.Fatalf("FTSInsert(%q): %v", nodeID, err)
	}
}

func TestMailbox_CreateRequiresTargetUser(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	// USER_MAILBOX without target_user_id is rejected.
	if _, err := cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "m1", TypeID: mailboxTypeID, OwnerActor: "user:alice",
		StorageMode: int32(store.StorageModeUserMailbox),
	}); err == nil {
		t.Fatal("expected error: USER_MAILBOX without target_user_id")
	}

	// A tenant node carrying a target_user_id is rejected.
	if _, err := cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "m2", TypeID: mailboxTypeID, OwnerActor: "user:alice",
		TargetUserID: "alice",
	}); err == nil {
		t.Fatal("expected error: target_user_id without USER_MAILBOX")
	}
}

func TestMailbox_StorageRoundTrip(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	n, err := cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "m1", TypeID: mailboxTypeID, OwnerActor: "user:alice",
		Payload:      map[string]any{"1": "hi"},
		StorageMode:  int32(store.StorageModeUserMailbox),
		TargetUserID: "alice",
	})
	if err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}
	if n.StorageMode != int32(store.StorageModeUserMailbox) || n.TargetUserID != "alice" {
		t.Fatalf("returned node lost mailbox fields: %+v", n)
	}
	// Round-trips through GetNode (the column is persisted).
	got, err := cs.GetNode(ctx, "t1", "m1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.StorageMode != int32(store.StorageModeUserMailbox) || got.TargetUserID != "alice" {
		t.Fatalf("GetNode lost mailbox fields: %+v", got)
	}
}

func TestMailbox_GetMailboxNodeScoping(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	seedMailboxNode(t, cs, "t1", "alice", "ma", "alice mail body")
	seedMailboxNode(t, cs, "t1", "bob", "mb", "bob mail body")
	// A plain tenant node with the same type.
	if _, err := cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "tenant1", TypeID: mailboxTypeID, OwnerActor: "user:alice",
		Payload: map[string]any{"1": "tenant body"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw tenant: %v", err)
	}

	// alice sees her own mailbox node.
	if _, err := cs.GetMailboxNode(ctx, "t1", "alice", "ma"); err != nil {
		t.Fatalf("GetMailboxNode(alice, ma): %v", err)
	}
	// alice cannot read bob's mailbox node -> NotFound.
	if _, err := cs.GetMailboxNode(ctx, "t1", "alice", "mb"); !errors.Is(err, store.ErrNodeNotFound) {
		t.Fatalf("GetMailboxNode(alice, mb): want ErrNodeNotFound, got %v", err)
	}
	// A mailbox-scoped read of a plain tenant node -> NotFound.
	if _, err := cs.GetMailboxNode(ctx, "t1", "alice", "tenant1"); !errors.Is(err, store.ErrNodeNotFound) {
		t.Fatalf("GetMailboxNode(alice, tenant1): want ErrNodeNotFound, got %v", err)
	}
}

func TestMailbox_QueryNodesScoping(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	seedMailboxNode(t, cs, "t1", "alice", "ma1", "a one")
	seedMailboxNode(t, cs, "t1", "alice", "ma2", "a two")
	seedMailboxNode(t, cs, "t1", "bob", "mb1", "b one")
	if _, err := cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "tenant1", TypeID: mailboxTypeID, OwnerActor: "user:alice",
		Payload: map[string]any{"1": "tenant"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw tenant: %v", err)
	}

	// Mailbox-scoped query: only alice's two mailbox nodes.
	got, err := cs.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID: "t1", TypeID: mailboxTypeID, MailboxUser: "alice", Limit: 100,
	})
	if err != nil {
		t.Fatalf("QueryNodes(mailbox=alice): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("mailbox query: want 2 nodes, got %d (%+v)", len(got), nodeIDsOf(got))
	}
	for _, n := range got {
		if n.TargetUserID != "alice" {
			t.Fatalf("mailbox query leaked node %q for user %q", n.NodeID, n.TargetUserID)
		}
	}

	// An unscoped (tenant) query EXCLUDES every mailbox-private row
	// (ADR-020): only the plain tenant node is visible. Mailbox rows are
	// reachable solely through the explicit mailbox scope.
	all, err := cs.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID: "t1", TypeID: mailboxTypeID, Limit: 100,
	})
	if err != nil {
		t.Fatalf("QueryNodes(unscoped): %v", err)
	}
	if len(all) != 1 || all[0].NodeID != "tenant1" {
		t.Fatalf("unscoped query: want only [tenant1], got %v", nodeIDsOf(all))
	}
}

func TestMailbox_SearchScoping(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	seedMailboxNode(t, cs, "t1", "alice", "ma", "shared keyword alice")
	seedMailboxNode(t, cs, "t1", "bob", "mb", "shared keyword bob")

	// Search scoped to alice matches only her node, even though bob's row
	// also contains "keyword".
	hits, err := cs.SearchMailboxNodes(ctx, "t1", "alice", mailboxTypeID, "keyword", []uint32{1}, 10, 0)
	if err != nil {
		t.Fatalf("SearchMailboxNodes(alice): %v", err)
	}
	if len(hits) != 1 || hits[0].NodeID != "ma" {
		t.Fatalf("SearchMailboxNodes(alice): want [ma], got %+v", nodeIDsOf(hits))
	}

	// An unscoped (tenant) search EXCLUDES mailbox-private rows
	// (ADR-020). Both seeded nodes are mailbox nodes, so the tenant
	// search returns nothing — they are reachable only via the mailbox
	// scope above.
	all, err := cs.SearchNodes(ctx, "t1", mailboxTypeID, "keyword", []uint32{1}, 10, 0)
	if err != nil {
		t.Fatalf("SearchNodes(unscoped): %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("SearchNodes(unscoped): want 0 (mailbox excluded), got %d", len(all))
	}
}

func nodeIDsOf(nodes []*store.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.NodeID)
	}
	return out
}
