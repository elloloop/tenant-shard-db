// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// TestApplier_RegistryRebuiltFromCatalogAfterRestart is the end-to-end
// #624 proof. A register_schema applied through the full WAL→applier path
// persists its node type into the per-tenant schema_catalog (atomic with
// the type's indexes — ADR-035). A brand-new store opened on the SAME data
// dir with a FRESH empty registry and NO WAL replay reloads the type from
// the catalog on tenant-open — so the registry is rebuilt from durable
// local state, independent of the WAL/Kafka committed offset.
//
// Before ADR-035 the fresh registry stayed empty (it was only ever rebuilt
// by replaying register_schema WAL ops, which a committed-offset restart
// skips), so reads of the type failed with "unknown type_id" (#624).
func TestApplier_RegistryRebuiltFromCatalogAfterRestart(t *testing.T) {
	ctx := context.Background()
	reg1 := schema.NewRegistry()
	f := newFixtureWithRegistry(t, reg1)

	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "s1", TsMs: 1700000000000,
		Ops: []map[string]any{
			mkRegisterSchema(true),
			mkCreateNode("n1", 1, map[string]any{"1": "alice@example.com"}),
		},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "s1")
	if reg1.NodeTypeByID(1) == nil {
		t.Fatal("type 1 not registered in the live registry")
	}

	// Simulate a restart that does NOT replay the WAL: a brand-new store on
	// the SAME data dir with a FRESH registry and no applier.
	reg2 := schema.NewRegistry()
	cs2, err := store.New(store.Options{RootDir: f.dir, WALMode: true, Registry: reg2})
	if err != nil {
		t.Fatalf("store.New (restart): %v", err)
	}
	t.Cleanup(func() { _ = cs2.Close() })
	if err := cs2.OpenTenant(ctx, testTenant); err != nil {
		t.Fatalf("OpenTenant (restart): %v", err)
	}

	if reg2.NodeTypeByID(1) == nil {
		t.Fatal("issue #624: type 1 not reloaded from schema_catalog after a no-replay restart")
	}
	if got := reg2.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Errorf("UniqueFieldIDs(1) = %v, want [1]", got)
	}
	// The node written before the restart is still readable.
	if _, err := cs2.GetNode(ctx, testTenant, "n1"); err != nil {
		t.Fatalf("n1 not readable after restart: %v", err)
	}
}
