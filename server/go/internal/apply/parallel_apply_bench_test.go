// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// benchmarkApplyBatch measures the wall-clock latency of applying ONE
// poll batch of `tenants` records (one per distinct tenant) at a given
// apply concurrency, with the WAL + store fully pre-populated so the
// timed region is exactly the apply+commit work — no append, no poll
// scheduling noise. concurrency=1 reproduces the pre-#140 serial
// applier; concurrency=0 uses the default GOMAXPROCS fan-out.
//
// SQLite COMMIT latency (an fsync of the WAL frame even with
// synchronous=NORMAL) is the dominant per-record cost. Distinct tenants
// own independent DB files, so their commits' fsyncs overlap when
// applied on separate goroutines — that overlap is the #140 win.
//
// The benchmark drives processBatch indirectly via Run but slices the
// timed region tightly around a single drained batch using a fan-out
// barrier on the last record's idempotency marker, polled at a fine
// granularity that is amortised away at scale.
func benchmarkApplyBatch(b *testing.B, tenants, concurrency int) {
	b.Helper()
	type rig struct {
		w  *wal.InMemory
		cs *store.CanonicalStore
		gs *globalstore.GlobalStore
		a  *apply.Applier
	}
	newRig := func() *rig {
		w := wal.NewInMemory(32)
		if err := w.Connect(context.Background()); err != nil {
			b.Fatalf("Connect: %v", err)
		}
		dir := b.TempDir()
		cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
		if err != nil {
			b.Fatalf("store.New: %v", err)
		}
		gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
		if err != nil {
			b.Fatalf("globalstore.New: %v", err)
		}
		for ti := 0; ti < tenants; ti++ {
			tn := fmt.Sprintf("bt_%05d", ti)
			if err := cs.OpenTenant(context.Background(), tn); err != nil {
				b.Fatalf("OpenTenant: %v", err)
			}
			ev := apply.Event{
				TenantID: tn, Actor: "u",
				IdempotencyKey: fmt.Sprintf("%s-k", tn),
				Ops:            []map[string]any{mkCreateNode("doc", 1, map[string]any{"1": "x"})},
			}
			val, _ := ev.Encode()
			if _, err := w.Append(context.Background(), testTopic, tn, val,
				map[string][]byte{wal.HeaderIdempotencyKey: []byte(ev.IdempotencyKey)}); err != nil {
				b.Fatalf("Append: %v", err)
			}
		}
		a, err := apply.New(apply.Options{
			Store: cs, Global: gs, Consumer: w,
			Topic: testTopic, GroupID: testGroupID,
			BatchSize:           tenants, // whole workload in one poll batch
			PollTimeout:         50 * time.Millisecond,
			MaxApplyConcurrency: concurrency,
		})
		if err != nil {
			b.Fatalf("apply.New: %v", err)
		}
		return &rig{w: w, cs: cs, gs: gs, a: a}
	}

	rigs := make([]*rig, b.N)
	for i := range rigs {
		rigs[i] = newRig()
	}
	lastTenant := fmt.Sprintf("bt_%05d", tenants-1)
	lastKey := lastTenant + "-k"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := rigs[i]
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- r.a.Run(ctx) }()
		for {
			ok, _ := r.cs.CheckIdempotency(context.Background(), lastTenant, lastKey)
			if ok {
				break
			}
			time.Sleep(50 * time.Microsecond)
		}
		cancel()
		<-done
	}
	b.StopTimer()

	for _, r := range rigs {
		_ = r.cs.Close()
		_ = r.gs.Close()
		_ = r.w.Close(context.Background())
	}
}

func BenchmarkApply_64Tenants_Serial(b *testing.B)     { benchmarkApplyBatch(b, 64, 1) }
func BenchmarkApply_64Tenants_Parallel(b *testing.B)   { benchmarkApplyBatch(b, 64, 0) }
func BenchmarkApply_256Tenants_Serial(b *testing.B)    { benchmarkApplyBatch(b, 256, 1) }
func BenchmarkApply_256Tenants_Parallel(b *testing.B)  { benchmarkApplyBatch(b, 256, 0) }
func BenchmarkApply_1024Tenants_Serial(b *testing.B)   { benchmarkApplyBatch(b, 1024, 1) }
func BenchmarkApply_1024Tenants_Parallel(b *testing.B) { benchmarkApplyBatch(b, 1024, 0) }
