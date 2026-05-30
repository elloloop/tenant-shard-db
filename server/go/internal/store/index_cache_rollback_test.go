// SPDX-License-Identifier: AGPL-3.0-only

package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// TestFTSIndexCacheInvalidatedOnRollback reproduces issue #629.
//
// The applier creates a type's FTS5 virtual table lazily on first write
// (EnsureFTSIndexConn), inside the same BEGIN IMMEDIATE batch, and records
// it in a process-local "done" cache. SQLite's DDL is transactional: if
// that batch ROLLs BACK (e.g. a later op fails a precondition), the
// CREATE VIRTUAL TABLE is reverted — but the cache still reports the table
// as present. A subsequent batch then cache-hits, skips re-creating the
// table, and FTSInsert hits "no such table: fts_tXXXX", fatally crashing
// the applier.
//
// Pre-fix this test fails on the second batch's FTSInsert. Post-fix
// BatchTxn.Rollback invalidates the tenant's index cache, so the next
// batch re-creates the reverted table (CREATE ... IF NOT EXISTS is safe).
func TestFTSIndexCacheInvalidatedOnRollback(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	const typeID int32 = 8001
	fields := []uint32{1}
	payload := map[string]any{"1": "the quick brown fox"}

	// Batch A: create the FTS table, then roll back (mirrors a multi-op
	// batch whose later op trips a precondition). SQLite reverts the DDL.
	btA, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch A: %v", err)
	}
	if err := cs.EnsureFTSIndexConn(ctx, btA.Conn(), "t1", typeID, fields); err != nil {
		t.Fatalf("EnsureFTSIndexConn A: %v", err)
	}
	if err := btA.Rollback(); err != nil {
		t.Fatalf("Rollback A: %v", err)
	}

	// Batch B: a fresh write of the SAME type must succeed. Ensure must
	// re-create the reverted table before FTSInsert runs.
	btB, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch B: %v", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = btB.Rollback()
		}
	}()

	if err := cs.EnsureFTSIndexConn(ctx, btB.Conn(), "t1", typeID, fields); err != nil {
		t.Fatalf("EnsureFTSIndexConn B: %v", err)
	}
	if err := store.FTSInsertConn(ctx, btB.Conn(), typeID, "n1", payload, fields); err != nil {
		if strings.Contains(err.Error(), "no such table") {
			t.Fatalf("issue #629: FTSInsert hit a reverted table — the index cache was not invalidated on rollback: %v", err)
		}
		t.Fatalf("FTSInsertConn B: %v", err)
	}
	if err := btB.Commit(); err != nil {
		t.Fatalf("Commit B: %v", err)
	}
	committed = true
}
