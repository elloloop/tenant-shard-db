// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// TestApplier_TransferOwnership_CorruptACLHalts pins #582: a corrupt
// acl_blob must fail-stop the transfer (halt-on-poison), NOT silently
// rebuild node_visibility from an empty ACL — which would revoke every
// shared grant on the node. Pre-fix the unmarshal error was swallowed and
// the transfer applied with an empty ACL.
func TestApplier_TransferOwnership_CorruptACLHalts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	// Seed a node with a real ACL grant (non-empty acl_blob), bypassing
	// the WAL — the transfer op below only reads the stored state.
	if _, err := f.store.CreateNodeRaw(ctx, testTenant, store.NodeInput{
		NodeID: "xfer", TypeID: 1, OwnerActor: "user:alice",
		ACL: []store.ACLEntry{{Principal: "user:carol", Permission: "read"}},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	// Corrupt the acl_blob via a second connection to the tenant DB.
	path, err := f.store.TenantDBPath(testTenant)
	if err != nil {
		t.Fatalf("TenantDBPath: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := db.Exec(`UPDATE nodes SET acl_blob = '{not valid json' WHERE node_id = 'xfer'`); err != nil {
		t.Fatalf("corrupt acl_blob: %v", err)
	}
	_ = db.Close()

	// The transfer must halt the applier rather than apply with an empty ACL.
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "xfer",
		Ops: []map[string]any{{
			"op": string(apply.OpTransferOwnership), "node_id": "xfer", "new_owner": "user:bob",
		}},
	})

	ctx2, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx2) }()

	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("applier did not halt within 2s (corrupt acl_blob should halt the transfer)")
	}
	// Halt-on-poison surfaces the decode error rather than masking it; the
	// exact wrapping is cosmetic — the load-bearing guarantee is that the
	// apply STOPS instead of proceeding with an empty ACL.
	if runErr == nil {
		t.Fatal("applier returned nil; the corrupt acl_blob should have halted the transfer")
	}

	// The transfer did NOT apply: owner unchanged ⇒ grants not silently dropped.
	n, err := f.store.GetNode(ctx, testTenant, "xfer")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n.OwnerActor != "user:alice" {
		t.Fatalf("owner changed to %q despite the poison — the empty-ACL mass-revoke path ran", n.OwnerActor)
	}
}
