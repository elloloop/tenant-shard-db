// SPDX-License-Identifier: AGPL-3.0-only

package store_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// TestGetVisibleNodeIDs_ExcludesMailbox_Finding639 pins the store-layer
// chokepoint for the USER_MAILBOX privacy boundary (#639). GetVisibleNodeIDs
// backs every generic ACL read (GetConnectedNodes, ListSharedWithMe, the
// QueryNodes cross-tenant post-filter). A USER_MAILBOX node owned by a user
// would otherwise match owner_actor and leak through those paths; it must be
// excluded here, reachable only via the explicit target_user mailbox scope.
func TestGetVisibleNodeIDs_ExcludesMailbox_Finding639(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	const tenantID = "t1"
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	// A regular node owned by alice — must remain visible.
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     "reg",
		TypeID:     1,
		OwnerActor: "user:alice",
		Payload:    map[string]any{"1": "x"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw(reg): %v", err)
	}
	// A USER_MAILBOX node owned by alice (owner_actor would match) — must be
	// excluded from generic visibility.
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:       "mb",
		TypeID:       1,
		OwnerActor:   "user:alice",
		StorageMode:  int32(store.StorageModeUserMailbox),
		TargetUserID: "alice",
		Payload:      map[string]any{"1": "y"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw(mb): %v", err)
	}

	vis, err := cs.GetVisibleNodeIDs(ctx, tenantID, []string{"user:alice"}, []string{"reg", "mb"})
	if err != nil {
		t.Fatalf("GetVisibleNodeIDs: %v", err)
	}
	if _, ok := vis["reg"]; !ok {
		t.Fatal("regular owned node must be visible via GetVisibleNodeIDs")
	}
	if _, ok := vis["mb"]; ok {
		t.Fatal("USER_MAILBOX node must NOT surface through generic visibility (#639) — it would leak via GetConnectedNodes / ListSharedWithMe / the QueryNodes post-filter")
	}
}

// TestExportUserData_ExcludesOtherUsersMailbox_Finding639 pins the GDPR-export
// arm of the mailbox-privacy invariant: a USER_MAILBOX node owned by another
// user must NOT be pulled into a subject's export via the subject field, while
// the subject's OWN mailbox node (and ordinary subject-matched nodes) remain
// exportable.
func TestExportUserData_ExcludesOtherUsersMailbox_Finding639(t *testing.T) {
	subjectField := uint32(2)
	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID:       7,
		SubjectField: &subjectField,
		Fields: []schema.FieldDef{
			{FieldID: 1, Kind: schema.KindString},
			{FieldID: 2, Kind: schema.KindString},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	cs := newStoreWithRegistry(t, reg)
	ctx := context.Background()
	const tenantID = "t1"
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	// bob's PRIVATE mailbox node that names alice as its data subject (field 2).
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID: "bob-mb", TypeID: 7, OwnerActor: "user:bob",
		StorageMode: int32(store.StorageModeUserMailbox), TargetUserID: "bob",
		Payload: map[string]any{"2": "alice"},
	}); err != nil {
		t.Fatalf("create bob mailbox: %v", err)
	}
	// alice's OWN mailbox node — her data, must stay exportable to her.
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID: "alice-mb", TypeID: 7, OwnerActor: "user:alice",
		StorageMode: int32(store.StorageModeUserMailbox), TargetUserID: "alice",
		Payload: map[string]any{"1": "x", "2": "alice"},
	}); err != nil {
		t.Fatalf("create alice mailbox: %v", err)
	}
	// an ordinary (non-mailbox) node about alice, owned by bob — subject access OK.
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID: "reg-about-alice", TypeID: 7, OwnerActor: "user:bob",
		Payload: map[string]any{"2": "alice"},
	}); err != nil {
		t.Fatalf("create regular subject node: %v", err)
	}

	exp, err := cs.ExportUserData(ctx, tenantID, "user:alice", reg)
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	got := map[string]bool{}
	for _, n := range exp.Nodes {
		got[n.NodeID] = true
	}
	if got["bob-mb"] {
		t.Fatal("ExportUserData leaked bob's USER_MAILBOX node (subject=alice) into alice's export (#639)")
	}
	if !got["alice-mb"] {
		t.Fatal("alice's OWN mailbox node must remain in her export (GDPR portability of her own data)")
	}
	if !got["reg-about-alice"] {
		t.Fatal("an ordinary node naming alice as subject must still be exported (subject access)")
	}
}
