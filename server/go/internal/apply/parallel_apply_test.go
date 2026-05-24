// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// multiTenantFixture wires an in-memory WAL + store + applier with a
// configurable apply concurrency. Unlike newFixture it does not pre-open
// a single tenant — callers open whatever tenants they need.
type multiTenantFixture struct {
	t       *testing.T
	wal     *wal.InMemory
	store   *store.CanonicalStore
	global  *globalstore.GlobalStore
	applier *apply.Applier
}

func newMultiTenantFixture(t *testing.T, maxConc int) *multiTenantFixture {
	t.Helper()
	w := wal.NewInMemory(8)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	dir := t.TempDir()
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
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
		Store:               cs,
		Global:              gs,
		Consumer:            w,
		Topic:               testTopic,
		GroupID:             testGroupID,
		PollTimeout:         25 * time.Millisecond,
		MaxApplyConcurrency: maxConc,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	return &multiTenantFixture{t: t, wal: w, store: cs, global: gs, applier: a}
}

func (f *multiTenantFixture) append(t *testing.T, ev apply.Event) {
	t.Helper()
	val, err := ev.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	headers := map[string][]byte{wal.HeaderIdempotencyKey: []byte(ev.IdempotencyKey)}
	if _, err := f.wal.Append(context.Background(), testTopic, ev.TenantID, val, headers); err != nil {
		t.Fatalf("Append: %v", err)
	}
}

func (f *multiTenantFixture) run(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func (f *multiTenantFixture) waitIdem(t *testing.T, tenant, key string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := f.store.CheckIdempotency(context.Background(), tenant, key)
		if err == nil && ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waitIdem: %s/%s never applied", tenant, key)
}

// TestParallelApply_PerTenantOrderingUnderConcurrency drives many tenants,
// each with a strictly ordered chain of updates that ONLY produces the
// expected final payload if every op for that tenant applied in offset
// order. A single shared field is overwritten N times per tenant; the
// terminal value encodes the last sequence number. Any reordering within
// a tenant yields a wrong terminal value.
func TestParallelApply_PerTenantOrderingUnderConcurrency(t *testing.T) {
	t.Parallel()
	const tenants = 12
	const opsPerTenant = 25

	f := newMultiTenantFixture(t, 8)
	for ti := 0; ti < tenants; ti++ {
		tenant := fmt.Sprintf("tenant_%02d", ti)
		if err := f.store.OpenTenant(context.Background(), tenant); err != nil {
			t.Fatalf("OpenTenant: %v", err)
		}
		// Create the node first.
		f.append(t, apply.Event{
			TenantID: tenant, Actor: "user:a",
			IdempotencyKey: fmt.Sprintf("%s-create", tenant),
			Ops:            []map[string]any{mkCreateNode("doc", 1, map[string]any{"1": "seq=0"})},
		})
		// Then a chain of overwrites; each depends on applying after the
		// previous (last-writer-wins on field id 1).
		for s := 1; s <= opsPerTenant; s++ {
			f.append(t, apply.Event{
				TenantID: tenant, Actor: "user:a",
				IdempotencyKey: fmt.Sprintf("%s-u%03d", tenant, s),
				Ops: []map[string]any{{
					"op": string(apply.OpUpdateNode), "id": "doc",
					"patch": map[string]any{"1": fmt.Sprintf("seq=%d", s)},
				}},
			})
		}
	}

	f.run(t)

	for ti := 0; ti < tenants; ti++ {
		tenant := fmt.Sprintf("tenant_%02d", ti)
		f.waitIdem(t, tenant, fmt.Sprintf("%s-u%03d", tenant, opsPerTenant))
		n, err := f.store.GetNode(context.Background(), tenant, "doc")
		if err != nil {
			t.Fatalf("%s GetNode: %v", tenant, err)
		}
		want := fmt.Sprintf(`"seq=%d"`, opsPerTenant)
		if !containsJSONValue(n.PayloadJSON, want) {
			t.Fatalf("%s terminal payload = %s; want field 1 == %s "+
				"(per-tenant ordering violated under concurrency)",
				tenant, n.PayloadJSON, want)
		}
	}
}

func containsJSONValue(payload, want string) bool {
	// Payload is field-id-keyed JSON like {"1":"seq=25"}; a substring
	// check on the quoted terminal value is sufficient and avoids
	// coupling the test to the payload codec internals.
	return len(payload) > 0 && (indexOf(payload, want) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestParallelApply_SingleWriterPerTenant proves no two goroutines apply
// records for the same tenant concurrently. The proof has two prongs:
//
//  1. Each tenant's records form a strict update chain (same node, same
//     field, monotonically increasing value). The store's per-tenant
//     BatchTxn write mutex is the ADR-016 backstop: if the applier ever
//     scheduled two records for one tenant in parallel, the read-modify-
//     write chain would interleave and the terminal value would be
//     wrong. We assert the terminal value is exactly the last sequence
//     for every tenant.
//  2. Run under `go test -race` (the project CI does): any unsynchronised
//     shared access in the parallel path trips the race detector.
//
// We additionally instrument the apply path via the FanoutHook (invoked
// once per record, serially in batch order, AFTER apply) to record the
// applied sequence per tenant and assert it is strictly increasing —
// catching any same-tenant reorder the value check could mask.
func TestParallelApply_SingleWriterPerTenant(t *testing.T) {
	t.Parallel()
	const tenants = 10
	const opsPerTenant = 12

	w := wal.NewInMemory(8)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	dir := t.TempDir()
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	var seqMu sync.Mutex
	lastSeq := map[string]int{}
	seqViolation := ""
	hook := func(ctx context.Context, ev *apply.Event, res *apply.Result) {
		if ev == nil || ev.TenantID == "" || len(ev.Ops) == 0 {
			return
		}
		s, ok := ev.Ops[0]["seq"]
		if !ok {
			return
		}
		// jsonnum canonicalizes integer op values to int64 (ADR-028);
		// tolerate float64 for events not routed through DecodeEvent.
		var seq int
		switch v := s.(type) {
		case int64:
			seq = int(v)
		case float64:
			seq = int(v)
		default:
			return
		}
		seqMu.Lock()
		if seq != lastSeq[ev.TenantID]+1 && seqViolation == "" {
			seqViolation = fmt.Sprintf("tenant %s: applied seq %d after %d (not strictly +1)",
				ev.TenantID, seq, lastSeq[ev.TenantID])
		}
		lastSeq[ev.TenantID] = seq
		seqMu.Unlock()
	}

	a, err := apply.New(apply.Options{
		Store:               cs,
		Global:              gs,
		Consumer:            w,
		Topic:               testTopic,
		GroupID:             testGroupID,
		PollTimeout:         25 * time.Millisecond,
		MaxApplyConcurrency: 16,
		FanoutHook:          hook,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}

	for ti := 0; ti < tenants; ti++ {
		tenant := fmt.Sprintf("tnt_%02d", ti)
		if err := cs.OpenTenant(context.Background(), tenant); err != nil {
			t.Fatalf("OpenTenant: %v", err)
		}
		for s := 1; s <= opsPerTenant; s++ {
			var ev apply.Event
			if s == 1 {
				ev = apply.Event{
					TenantID: tenant, Actor: "user:a",
					IdempotencyKey: fmt.Sprintf("%s-%d", tenant, s),
					Ops: []map[string]any{{
						"op": string(apply.OpCreateNode), "id": "doc",
						"type_id": int32(1), "seq": s,
						"data": map[string]any{"1": fmt.Sprintf("s%d", s)},
					}},
				}
			} else {
				ev = apply.Event{
					TenantID: tenant, Actor: "user:a",
					IdempotencyKey: fmt.Sprintf("%s-%d", tenant, s),
					Ops: []map[string]any{{
						"op": string(apply.OpUpdateNode), "id": "doc", "seq": s,
						"patch": map[string]any{"1": fmt.Sprintf("s%d", s)},
					}},
				}
			}
			val, _ := ev.Encode()
			if _, err := w.Append(context.Background(), testTopic, tenant, val,
				map[string][]byte{wal.HeaderIdempotencyKey: []byte(ev.IdempotencyKey)}); err != nil {
				t.Fatalf("Append: %v", err)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	for ti := 0; ti < tenants; ti++ {
		tenant := fmt.Sprintf("tnt_%02d", ti)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			ok, _ := cs.CheckIdempotency(context.Background(), tenant,
				fmt.Sprintf("%s-%d", tenant, opsPerTenant))
			if ok {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		n, err := cs.GetNode(context.Background(), tenant, "doc")
		if err != nil {
			t.Fatalf("%s GetNode: %v", tenant, err)
		}
		want := fmt.Sprintf(`"s%d"`, opsPerTenant)
		if indexOf(n.PayloadJSON, want) < 0 {
			t.Fatalf("%s terminal payload=%s want field 1==%s "+
				"(same-tenant applies interleaved — single-writer violated)",
				tenant, n.PayloadJSON, want)
		}
	}
	seqMu.Lock()
	v := seqViolation
	seqMu.Unlock()
	if v != "" {
		t.Fatalf("per-tenant apply order violated: %s", v)
	}
}

// TestParallelApply_HaltOnPoisonContiguousOffset pins the gap-free
// offset-commit invariant (#140 requirement (c)). When a record poisons
// mid-batch:
//
//   - The contiguous fully-applied prefix BEFORE the poison commits its
//     offsets (alpha/a1 here).
//   - The consumer-group offset NEVER advances past the poison, even
//     though a parallel worker for a different tenant may have
//     speculatively applied a LATER record (gamma/g1, alpha/a2). Their
//     SQLite writes are durable but uncommitted — exactly the ADR-016
//     "re-delivered after restart, idempotency probe SKIPs" contract,
//     now generalised across tenants.
//
// We prove offset non-advancement DIRECTLY: a fresh applier resumes on
// the SAME consumer group and MUST re-deliver the poison record (and the
// whole tail). If any tail offset had been wrongly committed, the
// resumed group would skip it and the assertion fails.
func TestParallelApply_HaltOnPoisonContiguousOffset(t *testing.T) {
	t.Parallel()
	f := newMultiTenantFixture(t, 8)

	for _, tn := range []string{"alpha", "beta", "gamma"} {
		if err := f.store.OpenTenant(context.Background(), tn); err != nil {
			t.Fatalf("OpenTenant %s: %v", tn, err)
		}
	}

	// Batch shape (distinct tenants -> distinct parallel workers):
	//  1. alpha  create ok       (the only contiguous-prefix record)
	//  2. beta   POISON (no id)
	//  3. gamma  create ok       (later tenant; worker may apply early)
	//  4. alpha  create ok       (after poison in batch order)
	f.append(t, apply.Event{
		TenantID: "alpha", Actor: "u", IdempotencyKey: "a1",
		Ops: []map[string]any{mkCreateNode("a-node", 1, nil)},
	})
	f.append(t, apply.Event{
		TenantID: "beta", Actor: "u", IdempotencyKey: "b1",
		Ops: []map[string]any{{"op": "create_node", "type_id": 1}}, // no id => poison
	})
	f.append(t, apply.Event{
		TenantID: "gamma", Actor: "u", IdempotencyKey: "g1",
		Ops: []map[string]any{mkCreateNode("g-node", 1, nil)},
	})
	f.append(t, apply.Event{
		TenantID: "alpha", Actor: "u", IdempotencyKey: "a2",
		Ops: []map[string]any{mkCreateNode("a-node-2", 1, nil)},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()

	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatalf("applier did not halt on poison within 3s")
	}
	if runErr == nil {
		t.Fatalf("expected halt-on-poison error, got nil")
	}

	// The contiguous applied prefix (alpha/a1) must be applied.
	if ok, _ := f.store.CheckIdempotency(context.Background(), "alpha", "a1"); !ok {
		t.Fatalf("alpha/a1 (the contiguous prefix) should be applied")
	}

	// DIRECT offset-non-advancement proof: resume a fresh applier on the
	// SAME consumer group + SAME store. The poison record's offset was
	// never committed, so it (and any speculatively-applied tail) is
	// re-polled. Skip-and-continue mode lets the resumed applier drain
	// the whole tail. After draining, every non-poison tail record must
	// be present — which can only happen if their offsets had NOT been
	// committed by the halted run (otherwise the resumed group skips
	// them and they never re-apply).
	//
	// Note we reuse f.store: the redelivered records that were already
	// applied take the idempotency-SKIP path (ADR-016), and the ones
	// that were NOT applied (e.g. alpha/a2 if its worker hadn't reached
	// it) now apply for the first time. Either way the terminal state is
	// the full deterministic set.
	skip := false
	a2, err := apply.New(apply.Options{
		Store: f.store, Global: f.global, Consumer: f.wal,
		Topic: testTopic, GroupID: testGroupID, // SAME group => resumes after last commit
		PollTimeout:  25 * time.Millisecond,
		HaltOnPoison: &skip,
	})
	if err != nil {
		t.Fatalf("apply.New resume: %v", err)
	}
	rctx, rcancel := context.WithCancel(context.Background())
	defer rcancel()
	rdone := make(chan error, 1)
	go func() { rdone <- a2.Run(rctx) }()
	defer func() { rcancel(); <-rdone }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ba, _ := f.store.CheckIdempotency(context.Background(), "beta", "b1")
		ga, _ := f.store.CheckIdempotency(context.Background(), "gamma", "g1")
		aa, _ := f.store.CheckIdempotency(context.Background(), "alpha", "a2")
		// beta/b1 is the poison and stays FAILED on skip-and-continue,
		// so CheckIdempotency stays false for it; we only require the
		// non-poison tail to converge.
		_ = ba
		if ga && aa {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ok, _ := f.store.CheckIdempotency(context.Background(), "gamma", "g1"); !ok {
		t.Fatalf("resume: gamma/g1 not applied — its offset must NOT have " +
			"been committed by the halted run (gap-free commit violated)")
	}
	if ok, _ := f.store.CheckIdempotency(context.Background(), "alpha", "a2"); !ok {
		t.Fatalf("resume: alpha/a2 not applied — tail after poison must be " +
			"re-deliverable on the same consumer group")
	}
}

// TestParallelApply_SamePartitionPoisonContiguousOffset is the ADR-027
// condition-3 test. It is deliberately stronger than
// TestParallelApply_HaltOnPoisonContiguousOffset: that test uses three
// tenants whose WAL keys land on (almost certainly) different
// partitions, so the poison and the speculative tail live in separate
// partition offset streams. THIS test forces tenant A and tenant B onto
// the *same* WAL partition and sandwiches a tenant-B poison BETWEEN two
// tenant-A records. With one shared partition, a single committed
// offset governs all three records, so the gap-free contiguous-prefix
// rule (ADR-027 invariant 3) is exercised at its weakest point: a
// faster tenant-A worker speculatively applies A's post-poison record
// while the serial finalizeBatch must still refuse to advance the one
// shared partition offset past tenant B's poison.
//
// Asserts:
//
//	(a) Gap-free contiguous-prefix commit: only the pre-poison prefix
//	    (A's first record) commits its offset; the shared partition's
//	    committed offset never advances past tenant B's poison even
//	    though A's worker speculatively materialised A's post-poison
//	    record.
//	(b) Idempotent resume: a fresh applier on the SAME consumer group +
//	    SAME store re-delivers the poison and the speculative tail; the
//	    already-applied A records take the in-txn idempotency-SKIP path
//	    (ADR-016) and the tail converges to the full deterministic set.
//
// Partition collision is asserted at runtime via wal.InMemory.PartitionFor
// (ADR-027: "Confirm via partitionFor that the chosen tenant ids
// actually collide on one partition") so the test fails loudly rather
// than silently degrading to the multi-partition case if the hash
// mapping ever changes.
func TestParallelApply_SamePartitionPoisonContiguousOffset(t *testing.T) {
	t.Parallel()

	// newMultiTenantFixture builds an 8-partition in-memory WAL.
	// tenant_00 and tenant_04 both MD5-hash to partition 1 of 8
	// (binary.BigEndian.Uint32(md5(key)[:4]) % 8). tenant_09 is a
	// same-partition sibling kept as documentation; the runtime
	// assertion below is the source of truth.
	const (
		tenantA = "tenant_00" // two ok records, sandwiching the poison
		tenantB = "tenant_04" // the poison; distinct route key => own worker
	)

	f := newMultiTenantFixture(t, 8)

	// ADR-027: confirm the chosen tenant ids actually collide on ONE
	// WAL partition. wal.Append keys records by tenant_id, so
	// PartitionFor(tenantID) is exactly the partition the record lands
	// in. If these ever diverge the test is no longer testing the
	// same-partition case and must fail.
	pa := f.wal.PartitionFor(tenantA)
	pb := f.wal.PartitionFor(tenantB)
	if pa != pb {
		t.Fatalf("test precondition broken: %s -> partition %d but %s -> partition %d; "+
			"the two tenant ids must collide on ONE partition for this test to "+
			"exercise the same-partition contiguous-offset invariant (ADR-027 cond 3)",
			tenantA, pa, tenantB, pb)
	}
	t.Logf("confirmed same-partition collision: %s and %s both -> partition %d", tenantA, tenantB, pa)

	for _, tn := range []string{tenantA, tenantB} {
		if err := f.store.OpenTenant(context.Background(), tn); err != nil {
			t.Fatalf("OpenTenant %s: %v", tn, err)
		}
	}

	// One shared partition, offsets 0,1,2 in this order:
	//   off 0  tenantA  create a1   (ok)  -> the ONLY contiguous prefix
	//   off 1  tenantB  POISON (no id)    -> halt point
	//   off 2  tenantA  create a2   (ok)  -> AFTER the poison in batch
	//                                         order; A's worker (distinct
	//                                         route key from B) has no
	//                                         shared stop and may apply
	//                                         it speculatively.
	f.append(t, apply.Event{
		TenantID: tenantA, Actor: "u", IdempotencyKey: "a1",
		Ops: []map[string]any{mkCreateNode("a-node-1", 1, nil)},
	})
	f.append(t, apply.Event{
		TenantID: tenantB, Actor: "u", IdempotencyKey: "b1",
		Ops: []map[string]any{{"op": "create_node", "type_id": 1}}, // no id => poison
	})
	f.append(t, apply.Event{
		TenantID: tenantA, Actor: "u", IdempotencyKey: "a2",
		Ops: []map[string]any{mkCreateNode("a-node-2", 1, nil)},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()

	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatalf("applier did not halt on poison within 3s")
	}
	if runErr == nil {
		t.Fatalf("expected halt-on-poison error, got nil")
	}

	// (a) The contiguous prefix (tenantA/a1, offset 0) must be applied.
	if ok, _ := f.store.CheckIdempotency(context.Background(), tenantA, "a1"); !ok {
		t.Fatalf("%s/a1 (the contiguous prefix at offset 0) should be applied", tenantA)
	}

	// (b) Gap-free + idempotent resume, proven DIRECTLY: bring up a
	// fresh applier on the SAME consumer group + SAME store. Because
	// the halted run never committed the shared partition's offset past
	// the poison at offset 1, a resumed group MUST re-poll from offset
	// 1 and re-deliver the poison + everything after it on that
	// partition. If any tail offset had been wrongly committed, the
	// resumed group would skip it and tenantA/a2 would never converge.
	//
	// tenantA/a2 may already be materialised (speculative apply by A's
	// worker before the halt) — on redelivery it takes the in-txn
	// idempotency-SKIP path (ADR-016) and stays present. Either way the
	// terminal state is the full deterministic set.
	skip := false
	a2, err := apply.New(apply.Options{
		Store: f.store, Global: f.global, Consumer: f.wal,
		Topic: testTopic, GroupID: testGroupID, // SAME group => resumes after last commit
		PollTimeout:  25 * time.Millisecond,
		HaltOnPoison: &skip,
	})
	if err != nil {
		t.Fatalf("apply.New resume: %v", err)
	}
	rctx, rcancel := context.WithCancel(context.Background())
	defer rcancel()
	rdone := make(chan error, 1)
	go func() { rdone <- a2.Run(rctx) }()
	defer func() { rcancel(); <-rdone }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := f.store.CheckIdempotency(context.Background(), tenantA, "a2"); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ok, _ := f.store.CheckIdempotency(context.Background(), tenantA, "a2"); !ok {
		t.Fatalf("resume: %s/a2 not applied — the shared partition's offset "+
			"must NOT have been committed past tenantB's poison at offset 1 "+
			"(gap-free contiguous-prefix commit violated, ADR-027 invariant 3)", tenantA)
	}
	// tenantB/b1 is the poison; under skip-and-continue it stays FAILED
	// and CheckIdempotency stays false for it. We do not require it to
	// converge — only that the non-poison tail did, which can only
	// happen via redelivery on the uncommitted shared offset.
	if ok, _ := f.store.CheckIdempotency(context.Background(), tenantB, "b1"); ok {
		t.Fatalf("resume: %s/b1 is the poison and must NOT be marked applied", tenantB)
	}
}

// TestParallelApply_EquivalentToSerial drives an identical mixed workload
// through a serial applier (MaxApplyConcurrency=1) and a parallel one and
// asserts byte-identical materialised state. This pins that parallelism
// is an optimisation with zero observable behaviour change.
func TestParallelApply_EquivalentToSerial(t *testing.T) {
	t.Parallel()

	build := func(maxConc int) map[string]string {
		w := wal.NewInMemory(8)
		_ = w.Connect(context.Background())
		dir := t.TempDir()
		cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
		if err != nil {
			t.Fatalf("store.New: %v", err)
		}
		defer cs.Close()
		gs, _ := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
		defer gs.Close()
		a, err := apply.New(apply.Options{
			Store: cs, Global: gs, Consumer: w,
			Topic: testTopic, GroupID: testGroupID,
			PollTimeout:         25 * time.Millisecond,
			MaxApplyConcurrency: maxConc,
		})
		if err != nil {
			t.Fatalf("apply.New: %v", err)
		}
		const tenants = 8
		for ti := 0; ti < tenants; ti++ {
			tn := fmt.Sprintf("t%d", ti)
			_ = cs.OpenTenant(context.Background(), tn)
		}
		// Round-robin interleave so a single poll batch mixes tenants.
		for s := 0; s < 20; s++ {
			for ti := 0; ti < tenants; ti++ {
				tn := fmt.Sprintf("t%d", ti)
				var ev apply.Event
				if s == 0 {
					ev = apply.Event{TenantID: tn, Actor: "u",
						IdempotencyKey: fmt.Sprintf("%s-c", tn),
						Ops:            []map[string]any{mkCreateNode("doc", 1, map[string]any{"1": "v0"})}}
				} else {
					ev = apply.Event{TenantID: tn, Actor: "u",
						IdempotencyKey: fmt.Sprintf("%s-u%d", tn, s),
						Ops: []map[string]any{{
							"op": string(apply.OpUpdateNode), "id": "doc",
							"patch": map[string]any{"1": fmt.Sprintf("v%d", s)},
						}}}
				}
				val, _ := ev.Encode()
				_, _ = w.Append(context.Background(), testTopic, tn, val,
					map[string][]byte{wal.HeaderIdempotencyKey: []byte(ev.IdempotencyKey)})
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- a.Run(ctx) }()
		defer func() { cancel(); <-done }()

		out := map[string]string{}
		for ti := 0; ti < tenants; ti++ {
			tn := fmt.Sprintf("t%d", ti)
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				ok, _ := cs.CheckIdempotency(context.Background(), tn, fmt.Sprintf("%s-u%d", tn, 19))
				if ok {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			n, err := cs.GetNode(context.Background(), tn, "doc")
			if err != nil {
				t.Fatalf("%s GetNode: %v", tn, err)
			}
			out[tn] = n.PayloadJSON
		}
		return out
	}

	serial := build(1)
	parallel := build(8)
	for tn, sv := range serial {
		if parallel[tn] != sv {
			t.Fatalf("tenant %s diverged: serial=%s parallel=%s", tn, sv, parallel[tn])
		}
	}
}
