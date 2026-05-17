// ADR-026 condition-2 regression test.
//
// The #137 read/write connection split (pool_readsplit_test.go) removed
// an implicit connection-serialisation that was MASKING a pre-existing
// latent read-after-write race in the WAL->applier path:
//
//	applyEvent -> UpdateAppliedOffsetTx (writes the applied_offsets row
//	INSIDE the BatchTxn AND, pre-fix, immediately Broadcast()s the
//	offsetCond) -> tx.Commit() (the SQL COMMIT).
//
// Pre-#536 every reader shared the single write connection the
// applier's BatchTxn held until post-COMMIT, so a woken WaitForOffset
// reader physically blocked until the write committed. The early
// broadcast was harmless. Post-#536 the reader uses an independent
// read-pool connection and runs its SELECT immediately: if it lands in
// the broadcast->commit window its WAL snapshot EXCLUDES the
// uncommitted write and the client reads its own confirmed write back
// as Found=false / stale.
//
// ADR-026 condition 1 moves the offsetCond broadcast out of
// UpdateAppliedOffsetTx and into BatchTxn.Commit, *after* the SQL
// COMMIT. This test drives a real write through the WAL->applier path,
// fences a re-routed GetNode via WaitForOffset under concurrent apply,
// and asserts the write is observed. It FAILS on the pre-fix code
// (broadcast before COMMIT) and PASSES with condition 1 applied.
//
// The store.WithPreCommitHook seam (export_test.go) makes the
// broadcast->commit window deterministic instead of probabilistic: the
// hook blocks the applier inside BatchTxn.Commit *after*
// UpdateAppliedOffsetTx ran but *before* the SQL COMMIT, exactly the
// window the regression lives in.
package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	rsTopic  = "entdb-wal-readsplit"
	rsTenant = "rs_tenant"
	rsGroup  = "readsplit-regression"
)

// TestReadSplitWaitForOffsetObservesAppliedWrite is the ADR-026
// condition-2 regression. A create_node event is appended to the WAL;
// the applier materialises it through its BatchTxn +
// UpdateAppliedOffsetTx. A reader fences on WaitForOffset(target) and
// then immediately re-reads the node through the #137 read pool. With
// condition 1 applied the WaitForOffset wake happens strictly after the
// SQL COMMIT, so the read pool sees the committed row. Without it the
// wake fires inside the pre-commit window and the read pool snapshot
// excludes the write -> GetNode returns ErrNodeNotFound and the test
// fails.
func TestReadSplitWaitForOffsetObservesAppliedWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// preCommitHook fires inside BatchTxn.Commit AFTER
	// UpdateAppliedOffsetTx and BEFORE the SQL COMMIT. Holding here
	// guarantees a WaitForOffset-fenced reader executes squarely inside
	// the broadcast->commit window on the pre-fix code.
	const hold = 150 * time.Millisecond
	var hookOnce sync.Once
	inWindow := make(chan struct{})
	opts := store.WithPreCommitHook(store.Options{
		RootDir:      dir,
		WALMode:      true,
		ReadPoolSize: 8, // split ON — the #137 read pool is materialised.
	}, func() {
		hookOnce.Do(func() {
			close(inWindow) // tell the reader: applier is pre-COMMIT now.
			time.Sleep(hold)
		})
	})

	cs, err := store.New(opts)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	a, err := apply.New(apply.Options{
		Store:       cs,
		Global:      gs,
		Consumer:    w,
		Topic:       rsTopic,
		GroupID:     rsGroup,
		PollTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	if err := cs.OpenTenant(context.Background(), rsTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	// Concurrent apply: the applier runs in its own goroutine, exactly
	// as in production.
	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		<-runDone
	})

	// Append the write the client will read back.
	ev := apply.Event{
		TenantID:       rsTenant,
		Actor:          "user:alice",
		IdempotencyKey: "rs-k1",
		TsMs:           1700000000000,
		Ops: []map[string]any{{
			"op":      string(apply.OpCreateNode),
			"id":      "rs-node-1",
			"type_id": int32(1),
			"data":    map[string]any{"1": "alice@example.com"},
		}},
	}
	val, err := ev.Encode()
	if err != nil {
		t.Fatalf("event.Encode: %v", err)
	}
	pos, err := w.Append(context.Background(), rsTopic, rsTenant, val,
		map[string][]byte{wal.HeaderIdempotencyKey: []byte(ev.IdempotencyKey)})
	if err != nil {
		t.Fatalf("wal.Append: %v", err)
	}
	target := pos.Offset

	// Reader: block until the applier is INSIDE the pre-commit window
	// (so on the pre-fix code the offsetCond has already been
	// broadcast), then do the exact read-your-write the Python SDK does
	// by default — WaitForOffset(target) followed immediately by a
	// re-routed GetNode on the read pool.
	select {
	case <-inWindow:
	case <-time.After(5 * time.Second):
		t.Fatal("applier never reached the pre-commit window")
	}

	wfoCtx, wfoCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer wfoCancel()
	if err := cs.WaitForOffset(wfoCtx, rsTenant, target); err != nil {
		t.Fatalf("WaitForOffset(%d): %v", target, err)
	}

	// The contract: once WaitForOffset(target) returns, offset target's
	// data MUST be visible on ANY connection, including the read pool.
	got, err := cs.GetNode(context.Background(), rsTenant, "rs-node-1")
	if err != nil {
		if errors.Is(err, store.ErrNodeNotFound) {
			t.Fatalf("ADR-026 condition-2 regression: WaitForOffset(%d) "+
				"returned but the re-routed read-pool GetNode does NOT "+
				"see the applied write (Found=false). The offsetCond "+
				"broadcast fired before BatchTxn.Commit — read-after-"+
				"write is broken. err=%v", target, err)
		}
		t.Fatalf("GetNode: %v", err)
	}
	if got.NodeID != "rs-node-1" {
		t.Fatalf("read-your-write mismatch: got node_id %q want rs-node-1", got.NodeID)
	}
	wantPayload := `{"1":"alice@example.com"}`
	if got.PayloadJSON != wantPayload {
		t.Fatalf("stale read: payload %q want %q", got.PayloadJSON, wantPayload)
	}

	// Sanity: the offset is durably persisted too.
	off, err := cs.GetAppliedOffset(context.Background(), rsTenant, pos.Topic, pos.Partition)
	if err != nil {
		t.Fatalf("GetAppliedOffset: %v", err)
	}
	if off < target {
		t.Fatalf("persisted offset %d < target %d", off, target)
	}
}
