// API-level tests for the USER_MAILBOX read surface (#568): the
// target_user scope on GetNode / GetNodes / QueryNodes / SearchNodes.
// Reuses newSearchTestServer (typeID=searchTypeID with searchable fields
// 1,2) so the same FTS index backs mailbox search.

package api_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// seedMailboxNode creates a USER_MAILBOX node for targetUser and indexes
// it in the same FTS table SearchNodes uses (fields 1,2).
func seedMailboxNode(t *testing.T, cs *store.CanonicalStore, tenantID, targetUser, nodeID, title, body string) {
	t.Helper()
	ctx := context.Background()
	payload := map[string]any{"1": title, "2": body}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:       nodeID,
		TypeID:       searchTypeID,
		OwnerActor:   "user:" + targetUser,
		Payload:      payload,
		StorageMode:  int32(store.StorageModeUserMailbox),
		TargetUserID: targetUser,
	}); err != nil {
		t.Fatalf("CreateNodeRaw(%q): %v", nodeID, err)
	}
	if err := cs.FTSInsert(ctx, tenantID, searchTypeID, nodeID, payload, []uint32{1, 2}); err != nil {
		t.Fatalf("FTSInsert(%q): %v", nodeID, err)
	}
}

func TestGetNode_MailboxScope(t *testing.T) {
	srv, cs, tenantID := newSearchTestServer(t)
	ctx := context.Background()

	seedMailboxNode(t, cs, tenantID, "alice", "ma", "alpha", "alice body")
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "beta", "bob body")

	// alice scopes to her own node -> found.
	resp, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "system:test"},
		NodeId:     "ma",
		TargetUser: "alice",
	})
	if err != nil {
		t.Fatalf("GetNode(alice, ma): %v", err)
	}
	if !resp.GetFound() {
		t.Fatal("GetNode(alice, ma): expected found")
	}

	// alice scopes to bob's node -> not found (privacy boundary).
	resp, err = srv.GetNode(ctx, &pb.GetNodeRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "system:test"},
		NodeId:     "mb",
		TargetUser: "alice",
	})
	if err != nil {
		t.Fatalf("GetNode(alice, mb): %v", err)
	}
	if resp.GetFound() {
		t.Fatal("GetNode(alice, mb): expected NOT found (cross-user mailbox leak)")
	}
}

// TestGetNode_MailboxExcludedFromTenantRead pins the privacy boundary:
// a plain tenant read (no target_user) must NOT surface a USER_MAILBOX
// node even by its exact id — otherwise a leaked/guessed id leaks private
// mailbox content, while the scan path already excludes it.
func TestGetNode_MailboxExcludedFromTenantRead(t *testing.T) {
	srv, cs, tenantID := newSearchTestServer(t)
	ctx := context.Background()
	seedMailboxNode(t, cs, tenantID, "alice", "ma", "alpha", "alice body")

	// No target_user → mailbox-private node is invisible (Found=false).
	resp, err := srv.GetNode(ctx, &pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "system:test"},
		NodeId:  "ma",
	})
	if err != nil {
		t.Fatalf("GetNode(ma, no scope): %v", err)
	}
	if resp.GetFound() {
		t.Fatal("GetNode(ma) with no target_user returned a mailbox-private node (privacy leak)")
	}

	// GetNodes (batch) by id must also report it as missing, not return it.
	batch, err := srv.GetNodes(ctx, &pb.GetNodesRequest{
		Context: &pb.RequestContext{TenantId: tenantID, Actor: "system:test"},
		NodeIds: []string{"ma"},
	})
	if err != nil {
		t.Fatalf("GetNodes([ma], no scope): %v", err)
	}
	if len(batch.GetNodes()) != 0 {
		t.Fatalf("GetNodes returned %d mailbox-private nodes on a tenant read; want 0", len(batch.GetNodes()))
	}
}

func TestGetNodes_MailboxScope(t *testing.T) {
	srv, cs, tenantID := newSearchTestServer(t)
	ctx := context.Background()

	seedMailboxNode(t, cs, tenantID, "alice", "ma", "alpha", "a")
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "beta", "b")

	resp, err := srv.GetNodes(ctx, &pb.GetNodesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "system:test"},
		NodeIds:    []string{"ma", "mb"},
		TargetUser: "alice",
	})
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(resp.GetNodes()) != 1 || resp.GetNodes()[0].GetNodeId() != "ma" {
		t.Fatalf("GetNodes(alice): want [ma], got %d nodes", len(resp.GetNodes()))
	}
	// bob's node is reported missing, not leaked.
	if len(resp.GetMissingIds()) != 1 || resp.GetMissingIds()[0] != "mb" {
		t.Fatalf("GetNodes(alice): want missing [mb], got %v", resp.GetMissingIds())
	}
}

func TestQueryNodes_MailboxScope(t *testing.T) {
	srv, cs, tenantID := newSearchTestServer(t)
	ctx := context.Background()

	seedMailboxNode(t, cs, tenantID, "alice", "ma1", "a1", "x")
	seedMailboxNode(t, cs, tenantID, "alice", "ma2", "a2", "y")
	seedMailboxNode(t, cs, tenantID, "bob", "mb1", "b1", "z")

	resp, err := srv.QueryNodes(ctx, &pb.QueryNodesRequest{
		Context:    &pb.RequestContext{TenantId: tenantID, Actor: "system:test"},
		TypeId:     searchTypeID,
		TargetUser: "alice",
		Limit:      100,
	})
	if err != nil {
		t.Fatalf("QueryNodes(mailbox=alice): %v", err)
	}
	if len(resp.GetNodes()) != 2 {
		t.Fatalf("QueryNodes(mailbox=alice): want 2, got %d", len(resp.GetNodes()))
	}
	for _, n := range resp.GetNodes() {
		if n.GetNodeId() == "mb1" {
			t.Fatal("QueryNodes(mailbox=alice) leaked bob's node")
		}
	}
}

func TestSearchNodes_MailboxScope(t *testing.T) {
	srv, cs, tenantID := newSearchTestServer(t)
	ctx := context.Background()

	seedMailboxNode(t, cs, tenantID, "alice", "ma", "shared", "keyword alice")
	seedMailboxNode(t, cs, tenantID, "bob", "mb", "shared", "keyword bob")

	// Mailbox-scoped search: only alice's node, even though both match.
	resp, err := srv.SearchNodes(ctx, &pb.SearchNodesRequest{
		TenantId:   tenantID,
		Actor:      "system:test",
		TypeId:     searchTypeID,
		Query:      "keyword",
		TargetUser: "alice",
	})
	if err != nil {
		t.Fatalf("SearchNodes(target=alice): %v", err)
	}
	if len(resp.GetNodes()) != 1 || resp.GetNodes()[0].GetNodeId() != "ma" {
		t.Fatalf("SearchNodes(target=alice): want [ma], got %d", len(resp.GetNodes()))
	}

	// Unscoped (tenant) search EXCLUDES mailbox-private nodes (ADR-020).
	// Both seeded nodes are mailbox nodes, so a tenant search sees none.
	resp, err = srv.SearchNodes(ctx, &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "system:test",
		TypeId:   searchTypeID,
		Query:    "keyword",
	})
	if err != nil {
		t.Fatalf("SearchNodes(unscoped): %v", err)
	}
	if len(resp.GetNodes()) != 0 {
		t.Fatalf("SearchNodes(unscoped): want 0 (mailbox excluded), got %d", len(resp.GetNodes()))
	}
}
