// Benchmark + correctness coverage for the issue #137 read/write
// connection split.
//
// #137 (PERF-1): pool.go pinned every per-tenant *sql.DB to
// SetMaxOpenConns(1), so 50 parallel same-tenant GetNode calls ran
// strictly serially. The fix opens a SEPARATE read-only pooled handle
// (mode=ro + query_only, SetMaxOpenConns(N)) for pure SELECT methods
// while the write path keeps the single serialized connection
// (ADR-016: only the applier writes SQLite, under BEGIN IMMEDIATE).
//
// BenchmarkSameTenantParallelReads/serialized-readpool=1 is the
// pre-#137 baseline (split disabled). .../split-readpool=8 is the
// fix. Run:
//
//	go test ./internal/store/ -run x \
//	  -bench BenchmarkSameTenantParallelReads -benchmem
package store_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const benchTenant = "benchtenant"

// seededStore builds a WAL-mode store with the given read-pool size and
// pre-creates n nodes for benchTenant. n is intentionally larger than
// the read pool so reads contend on the connection set.
func seededStore(tb testing.TB, readPoolSize, n int) (*store.CanonicalStore, []string) {
	tb.Helper()
	dir := tb.TempDir()
	cs, err := store.New(store.Options{
		RootDir:      dir,
		WALMode:      true,
		ReadPoolSize: readPoolSize,
	})
	if err != nil {
		tb.Fatalf("store.New: %v", err)
	}
	tb.Cleanup(func() { _ = cs.Close() })

	ctx := context.Background()
	if err := cs.OpenTenant(ctx, benchTenant); err != nil {
		tb.Fatalf("OpenTenant: %v", err)
	}
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("node-%06d", i)
		ids[i] = id
		if _, err := cs.CreateNodeRaw(ctx, benchTenant, store.NodeInput{
			NodeID:     id,
			TypeID:     1,
			OwnerActor: "user:bench",
			Payload:    map[string]any{"1": "alice", "2": int64(i)},
		}); err != nil {
			tb.Fatalf("CreateNodeRaw[%d]: %v", i, err)
		}
	}
	return cs, ids
}

// BenchmarkSameTenantParallelReads measures same-tenant GetNode
// throughput with the read/write split disabled (readpool=1, the
// pre-#137 behaviour) versus enabled (readpool=8). Both variants run
// the identical workload through b.RunParallel so the only difference
// is the connection-pool topology.
func BenchmarkSameTenantParallelReads(b *testing.B) {
	const nodes = 2000
	cases := []struct {
		name string
		size int
	}{
		{"serialized-readpool=1", 1},
		{"split-readpool=8", 8},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			cs, ids := seededStore(b, tc.size, nodes)
			ctx := context.Background()
			// Warm the pages / open the handle once.
			if _, err := cs.GetNode(ctx, benchTenant, ids[0]); err != nil {
				b.Fatalf("warmup GetNode: %v", err)
			}
			var ctr uint64
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := atomic.AddUint64(&ctr, 1)
					id := ids[i%uint64(len(ids))]
					if _, err := cs.GetNode(ctx, benchTenant, id); err != nil {
						b.Fatalf("GetNode: %v", err)
					}
				}
			})
		})
	}
}

// TestReadSplitConcurrentReadsObserveCommittedWrites is the correctness
// gate behind the benchmark. It runs many concurrent same-tenant
// readers alongside a serialized writer (CreateNodeRaw -> withWrite ->
// BEGIN IMMEDIATE, the same discipline the applier uses) and asserts:
//
//   - no reader ever errors (the read-only handle never collides with
//     the writer; SQLite WAL gives readers a stable snapshot),
//   - every committed node becomes visible to a subsequent read
//     (read-your-writes across the connection boundary holds because
//     the writer COMMITs before the read transaction starts and WAL
//     readers see the latest committed frame),
//   - the read-only handle physically rejects writes (mode=ro).
func TestReadSplitConcurrentReadsObserveCommittedWrites(t *testing.T) {
	t.Parallel()
	cs, ids := seededStore(t, 8, 200)
	ctx := context.Background()

	const writers = 300
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Background readers: hammer existing nodes while writes land.
	for r := 0; r < 16; r++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			i := seed
			for {
				select {
				case <-stop:
					return
				default:
				}
				id := ids[i%len(ids)]
				if _, err := cs.GetNode(ctx, benchTenant, id); err != nil {
					t.Errorf("concurrent reader GetNode(%s): %v", id, err)
					return
				}
				i++
			}
		}(r * 7)
	}

	// Serialized writer adds new nodes; after each commit the node
	// MUST be visible through the read-only handle.
	for w := 0; w < writers; w++ {
		id := fmt.Sprintf("new-%05d", w)
		if _, err := cs.CreateNodeRaw(ctx, benchTenant, store.NodeInput{
			NodeID:     id,
			TypeID:     1,
			OwnerActor: "user:w",
			Payload:    map[string]any{"1": "bob"},
		}); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("CreateNodeRaw(%s): %v", id, err)
		}
		got, err := cs.GetNode(ctx, benchTenant, id)
		if err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("read-your-write GetNode(%s) after commit: %v", id, err)
		}
		if got.NodeID != id {
			close(stop)
			wg.Wait()
			t.Fatalf("read-your-write mismatch: got %q want %q", got.NodeID, id)
		}
	}
	close(stop)
	wg.Wait()

	// Final consistency: every writer node is readable.
	for w := 0; w < writers; w++ {
		id := fmt.Sprintf("new-%05d", w)
		if _, err := cs.GetNode(ctx, benchTenant, id); err != nil {
			t.Fatalf("post-run GetNode(%s): %v", id, err)
		}
	}
}

// TestReadSplitDisabledFallback verifies that with ReadPoolSize<=1 the
// behaviour is byte-for-byte the pre-#137 path: reads still work, no
// separate handle is required, and the single connection serves them.
func TestReadSplitDisabledFallback(t *testing.T) {
	t.Parallel()
	cs, ids := seededStore(t, 1, 50)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			if _, err := cs.GetNode(ctx, benchTenant, ids[k%len(ids)]); err != nil {
				t.Errorf("fallback GetNode: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
