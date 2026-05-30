// SPDX-License-Identifier: AGPL-3.0-only

package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// catalogNodeDefJSON is the wire shape of a User type (type_id 1) with a
// unique email field — the same snake_case contract the register_schema
// op carries.
const catalogNodeDefJSON = `{"type_id":1,"fields":[{"field_id":1,"name":"email","kind":"str","unique":true}]}`

const catalogEdgeDefJSON = `{"edge_id":10,"from_type_id":1,"to_type_id":1,"on_subject_exit":"both"}`

func catalogDefBytes(t *testing.T) []byte {
	t.Helper()
	var nt schema.NodeTypeDef
	if err := json.Unmarshal([]byte(catalogNodeDefJSON), &nt); err != nil {
		t.Fatalf("decode node def: %v", err)
	}
	b, err := store.MarshalCatalogDef(&nt)
	if err != nil {
		t.Fatalf("MarshalCatalogDef: %v", err)
	}
	return b
}

func openCatalogStore(t *testing.T, dir string, reg *schema.Registry) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true, Registry: reg})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestCatalog_PersistsAndReloadsAcrossRestart is the core #626/ADR-035
// proof: a type written to schema_catalog in a committed batch is reloaded
// into a FRESH registry when a new store opens the SAME data dir, with NO
// WAL replay. Before ADR-035 the fresh registry stayed empty (#624).
func TestCatalog_PersistsAndReloadsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	defJSON := catalogDefBytes(t)

	// Store 1: persist the catalog row inside a committed BatchTxn.
	cs1 := openCatalogStore(t, dir, schema.NewRegistry())
	if err := cs1.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	tx, err := cs1.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs1.UpsertCatalogTx(ctx, tx, store.CatalogKindNode, 1, defJSON); err != nil {
		t.Fatalf("UpsertCatalogTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	_ = cs1.Close()

	// Store 2: a FRESH registry, the SAME dir, no applier / WAL replay.
	reg2 := schema.NewRegistry()
	cs2 := openCatalogStore(t, dir, reg2)
	if err := cs2.OpenTenant(ctx, "t1"); err != nil { // triggers the catalog reload
		t.Fatalf("OpenTenant 2: %v", err)
	}
	if reg2.NodeTypeByID(1) == nil {
		t.Fatal("issue #624: type 1 not reloaded from schema_catalog into a fresh registry after restart")
	}
	if got := reg2.UniqueFieldIDs(1); len(got) != 1 || got[0] != 1 {
		t.Errorf("UniqueFieldIDs(1) = %v, want [1]", got)
	}
}

// TestCatalog_ReloadIsIdempotentNotConflict pins the def_json canonical
// round-trip: a type reloaded from the catalog must be establish-or-reject
// IDENTICAL to a fresh register of the same definition (idempotent no-op,
// not ErrSchemaConflict). A non-canonical serialization would make a
// post-restart self-describing write of an already-known type conflict.
func TestCatalog_ReloadIsIdempotentNotConflict(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	defJSON := catalogDefBytes(t)

	cs := openCatalogStore(t, dir, schema.NewRegistry())
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	tx, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs.UpsertCatalogTx(ctx, tx, store.CatalogKindNode, 1, defJSON); err != nil {
		t.Fatalf("UpsertCatalogTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reg2 := schema.NewRegistry()
	if err := cs.LoadCatalogInto(ctx, "t1", reg2); err != nil {
		t.Fatalf("LoadCatalogInto: %v", err)
	}
	var nt schema.NodeTypeDef
	if err := json.Unmarshal(defJSON, &nt); err != nil {
		t.Fatalf("decode: %v", err)
	}
	registered, err := reg2.RegisterOrVerifyNode(&nt)
	if err != nil {
		t.Fatalf("identical re-register after catalog reload conflicted (def_json is not a canonical round-trip): %v", err)
	}
	if registered {
		t.Error("re-register of an already-loaded type should be a no-op (registered=false), got true")
	}
}

// TestCatalog_RollbackLeavesNoRow proves the catalog UPSERT is atomic with
// the batch: a rolled-back BatchTxn persists nothing.
func TestCatalog_RollbackLeavesNoRow(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cs := openCatalogStore(t, dir, schema.NewRegistry())
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	tx, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs.UpsertCatalogTx(ctx, tx, store.CatalogKindNode, 1, catalogDefBytes(t)); err != nil {
		t.Fatalf("UpsertCatalogTx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if n := countCatalogRows(t, cs, ctx); n != 0 {
		t.Errorf("rolled-back UpsertCatalogTx left %d catalog rows, want 0", n)
	}
}

// TestCatalog_UpsertIdempotent: a second identical upsert leaves one row.
func TestCatalog_UpsertIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cs := openCatalogStore(t, dir, schema.NewRegistry())
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	defJSON := catalogDefBytes(t)
	for i := 0; i < 2; i++ {
		tx, err := cs.BeginBatch(ctx, "t1")
		if err != nil {
			t.Fatalf("BeginBatch #%d: %v", i, err)
		}
		if err := cs.UpsertCatalogTx(ctx, tx, store.CatalogKindNode, 1, defJSON); err != nil {
			t.Fatalf("UpsertCatalogTx #%d: %v", i, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit #%d: %v", i, err)
		}
	}
	if n := countCatalogRows(t, cs, ctx); n != 1 {
		t.Errorf("idempotent upsert produced %d rows, want 1", n)
	}
}

// TestCatalog_EdgeRoundTrip exercises the edge branch of the catalog
// (UpsertCatalogTx kind="edge" + LoadCatalogInto + RegisterOrVerifyEdge),
// including the canonical round-trip idempotency.
func TestCatalog_EdgeRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	var et schema.EdgeTypeDef
	if err := json.Unmarshal([]byte(catalogEdgeDefJSON), &et); err != nil {
		t.Fatalf("decode edge def: %v", err)
	}
	defJSON, err := store.MarshalCatalogDef(&et)
	if err != nil {
		t.Fatalf("MarshalCatalogDef edge: %v", err)
	}

	cs := openCatalogStore(t, dir, schema.NewRegistry())
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	tx, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs.UpsertCatalogTx(ctx, tx, store.CatalogKindEdge, et.EdgeID, defJSON); err != nil {
		t.Fatalf("UpsertCatalogTx edge: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reg2 := schema.NewRegistry()
	if err := cs.LoadCatalogInto(ctx, "t1", reg2); err != nil {
		t.Fatalf("LoadCatalogInto: %v", err)
	}
	if reg2.EdgeTypeByID(10) == nil {
		t.Fatal("edge_id 10 not reloaded from schema_catalog")
	}
	registered, err := reg2.RegisterOrVerifyEdge(&et)
	if err != nil {
		t.Fatalf("identical edge re-register conflicted after reload: %v", err)
	}
	if registered {
		t.Error("edge re-register after reload should be a no-op (registered=false)")
	}
}

func countCatalogRows(t *testing.T, cs *store.CanonicalStore, ctx context.Context) int {
	t.Helper()
	db, err := cs.AdminDB("t1")
	if err != nil {
		t.Fatalf("AdminDB: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_catalog").Scan(&n); err != nil {
		t.Fatalf("count schema_catalog: %v", err)
	}
	return n
}
