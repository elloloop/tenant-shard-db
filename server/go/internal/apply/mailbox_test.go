// SPDX-License-Identifier: AGPL-3.0-only

// End-to-end applier tests for the USER_MAILBOX read surface (#568):
// a create_node op carrying storage_mode=USER_MAILBOX + target_user_id
// must persist those columns AND populate the FTS index, so the
// mailbox-scoped read paths return the node.

package apply_test

import (
	"context"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const mailboxTypeID int32 = 11

// newMailboxFixture is newFixture with a schema.Registry wired into the
// store so the applier indexes the searchable field (id 1) on create.
func newMailboxFixture(t *testing.T) *fixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	dir := t.TempDir()

	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: mailboxTypeID,
		Fields: []schema.FieldDef{
			{FieldID: 1, Kind: schema.KindString, Searchable: true},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	cs, err := store.New(store.Options{RootDir: dir, WALMode: true, Registry: reg})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	a, err := apply.New(apply.Options{
		Store:       cs,
		Global:      gs,
		Consumer:    w,
		Topic:       testTopic,
		GroupID:     testGroupID,
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	if err := cs.OpenTenant(context.Background(), testTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	return &fixture{t: t, wal: w, store: cs, global: gs, applier: a}
}

func mkMailboxCreate(id, targetUser, subject string) map[string]any {
	return map[string]any{
		"op":             string(apply.OpCreateNode),
		"id":             id,
		"type_id":        mailboxTypeID,
		"data":           map[string]any{"1": subject},
		"storage_mode":   "USER_MAILBOX",
		"target_user_id": targetUser,
	}
}

// TestApplier_MailboxCreateAndRead drives a USER_MAILBOX create through
// the WAL applier and asserts (a) the storage columns persisted and
// (b) the FTS index was populated, so the mailbox-scoped read paths work.
func TestApplier_MailboxCreateAndRead(t *testing.T) {
	t.Parallel()
	f := newMailboxFixture(t)
	ctx := context.Background()

	f.appendEvent(t, apply.Event{
		TenantID:       testTenant,
		Actor:          "user:alice",
		IdempotencyKey: "mbox-1",
		TsMs:           1700000000000,
		Ops: []map[string]any{
			mkMailboxCreate("ma", "alice", "quarterly report"),
			mkMailboxCreate("mb", "bob", "quarterly report"),
		},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "mbox-1")

	// Storage columns persisted.
	n, err := f.store.GetNode(ctx, testTenant, "ma")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n.StorageMode != int32(store.StorageModeUserMailbox) || n.TargetUserID != "alice" {
		t.Fatalf("mailbox columns not persisted by applier: %+v", n)
	}

	// Mailbox-scoped get: alice sees her node, not bob's.
	if _, err := f.store.GetMailboxNode(ctx, testTenant, "alice", "ma"); err != nil {
		t.Fatalf("GetMailboxNode(alice, ma): %v", err)
	}
	if _, err := f.store.GetMailboxNode(ctx, testTenant, "alice", "mb"); err == nil {
		t.Fatal("GetMailboxNode(alice, mb): expected NotFound for another user's node")
	}

	// FTS populated BY THE APPLIER (no test-side FTSInsert): a mailbox
	// search scoped to alice matches only her node.
	hits, err := f.store.SearchMailboxNodes(ctx, testTenant, "alice", mailboxTypeID, "quarterly", []uint32{1}, 10, 0)
	if err != nil {
		t.Fatalf("SearchMailboxNodes: %v", err)
	}
	if len(hits) != 1 || hits[0].NodeID != "ma" {
		t.Fatalf("SearchMailboxNodes(alice): want [ma], got %d hits", len(hits))
	}

	// Mailbox-scoped query returns only alice's node.
	got, err := f.store.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID: testTenant, TypeID: mailboxTypeID, MailboxUser: "alice", Limit: 100,
	})
	if err != nil {
		t.Fatalf("QueryNodes(mailbox=alice): %v", err)
	}
	if len(got) != 1 || got[0].NodeID != "ma" {
		t.Fatalf("QueryNodes(mailbox=alice): want [ma], got %d", len(got))
	}
}

// TestApplier_MailboxMissingTargetUserPoisons confirms a USER_MAILBOX op
// with no target_user_id is a poison event (the applier halts rather
// than persisting an unscoped mailbox row).
func TestApplier_MailboxMissingTargetUserPoisons(t *testing.T) {
	t.Parallel()
	f := newMailboxFixture(t)

	f.appendEvent(t, apply.Event{
		TenantID:       testTenant,
		Actor:          "user:alice",
		IdempotencyKey: "mbox-bad",
		TsMs:           1700000000000,
		Ops: []map[string]any{
			{
				"op":           string(apply.OpCreateNode),
				"id":           "bad",
				"type_id":      mailboxTypeID,
				"data":         map[string]any{"1": "x"},
				"storage_mode": "USER_MAILBOX",
				// target_user_id intentionally omitted.
			},
		},
	})
	// The applier halts on poison by default; run it briefly and assert
	// the node never materialised.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = f.applier.Run(ctx)

	if _, err := f.store.GetNode(context.Background(), testTenant, "bad"); err == nil {
		t.Fatal("expected poison: USER_MAILBOX node without target_user_id must not materialise")
	}
}
