// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newConnected(t *testing.T) *InMemory {
	t.Helper()
	m := NewInMemory(0)
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = m.Close(context.Background()) })
	return m
}

func TestStreamPosString(t *testing.T) {
	pos := StreamPos{Topic: "wal", Partition: 2, Offset: 7}
	if got, want := pos.String(), "wal:2:7"; got != want {
		t.Fatalf("StreamPos.String() = %q, want %q", got, want)
	}
}

func TestEventEncodeDecodeRoundTrip(t *testing.T) {
	ev := Event{
		TenantID:          "t1",
		Actor:             "user:alice",
		IdempotencyKey:    "abc",
		SchemaFingerprint: "sha256:deadbeef",
		TsMs:              1700000000000,
		Ops: []map[string]any{
			{"op": "create_node", "type_id": float64(101)},
		},
	}
	b, err := ev.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := DecodeEvent(b)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if got.TenantID != ev.TenantID || got.Actor != ev.Actor || got.IdempotencyKey != ev.IdempotencyKey {
		t.Fatalf("round-trip mismatch: %+v vs %+v", got, ev)
	}
	if got.TsMs != ev.TsMs {
		t.Fatalf("ts_ms: got %d want %d", got.TsMs, ev.TsMs)
	}
	if len(got.Ops) != 1 {
		t.Fatalf("ops len = %d want 1", len(got.Ops))
	}
}

func TestEventEncodeDefaultsTsMs(t *testing.T) {
	ev := Event{
		TenantID:       "t1",
		Actor:          "user:alice",
		IdempotencyKey: "abc",
		Ops:            []map[string]any{{"op": "noop"}},
	}
	before := time.Now().UnixMilli()
	b, err := ev.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	after := time.Now().UnixMilli()
	got, err := DecodeEvent(b)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if got.TsMs < before || got.TsMs > after {
		t.Fatalf("TsMs %d not in [%d,%d]", got.TsMs, before, after)
	}
}

func TestDecodeEventRejectsMissingFields(t *testing.T) {
	_, err := DecodeEvent([]byte(`{"tenant_id":"t1"}`))
	if err == nil {
		t.Fatal("expected error on missing required fields")
	}
}

func TestAppendRequiresConnection(t *testing.T) {
	m := NewInMemory(0)
	_ = m.Close(context.Background())
	_, err := m.Append(context.Background(), "wal", "k", []byte("v"), nil)
	if !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestAppendPollRoundTrip(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()

	pos, err := m.Append(ctx, "wal", "tenant_a", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if pos.Offset != 0 {
		t.Fatalf("first offset = %d, want 0", pos.Offset)
	}

	got, err := m.PollBatch(ctx, "wal", "g1", 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if string(got[0].Value) != "hello" {
		t.Fatalf("value = %q, want %q", got[0].Value, "hello")
	}
	if got[0].Position != pos {
		t.Fatalf("position = %+v, want %+v", got[0].Position, pos)
	}
}

func TestAppendIdempotentSameKey(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()

	headers := map[string][]byte{HeaderIdempotencyKey: []byte("uuid-1")}
	pos1, err := m.Append(ctx, "wal", "tenant_a", []byte("payload-1"), headers)
	if err != nil {
		t.Fatalf("Append #1: %v", err)
	}

	// Same idempotency key, even with different payload, returns the
	// original pos. The applier is the gate for "different payload
	// with same idempotency_key" -- producer just dedupes.
	pos2, err := m.Append(ctx, "wal", "tenant_a", []byte("payload-2"), headers)
	if err != nil {
		t.Fatalf("Append #2: %v", err)
	}
	if pos1 != pos2 {
		t.Fatalf("idempotent retry: pos1=%+v pos2=%+v", pos1, pos2)
	}
	if got := m.GetRecordCount("wal"); got != 1 {
		t.Fatalf("record count = %d, want 1 (no duplicate)", got)
	}

	// Different idempotency key under the same partition key produces
	// a fresh record.
	headers2 := map[string][]byte{HeaderIdempotencyKey: []byte("uuid-2")}
	pos3, err := m.Append(ctx, "wal", "tenant_a", []byte("payload-3"), headers2)
	if err != nil {
		t.Fatalf("Append #3: %v", err)
	}
	if pos3 == pos1 {
		t.Fatalf("distinct idempotency keys collapsed to same pos")
	}
	if got := m.GetRecordCount("wal"); got != 2 {
		t.Fatalf("record count = %d, want 2", got)
	}
}

func TestAppendNoIdempotencyKeyAlwaysWrites(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := m.Append(ctx, "wal", "tenant_a", []byte("x"), nil); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	if got := m.GetRecordCount("wal"); got != 3 {
		t.Fatalf("record count = %d, want 3", got)
	}
}

func TestPerTenantOrderUnderConcurrency(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()

	const tenants = 4
	const perTenant = 50
	var wg sync.WaitGroup
	wg.Add(tenants)
	for tt := 0; tt < tenants; tt++ {
		tenant := fmt.Sprintf("tenant_%d", tt)
		go func() {
			defer wg.Done()
			for i := 0; i < perTenant; i++ {
				val := fmt.Sprintf("%s:%d", tenant, i)
				if _, err := m.Append(ctx, "wal", tenant, []byte(val), nil); err != nil {
					t.Errorf("Append: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if got := m.GetRecordCount("wal"); got != tenants*perTenant {
		t.Fatalf("record count = %d, want %d", got, tenants*perTenant)
	}

	// Verify per-tenant order: for each tenant, the values
	// "tenant_X:0", "tenant_X:1".. appear in increasing offset
	// order within the tenant's partition.
	all := m.GetAllRecords("wal")
	perTenantSeen := make(map[string][]string)
	for _, r := range all {
		perTenantSeen[r.Key] = append(perTenantSeen[r.Key], string(r.Value))
	}
	for tenant, vals := range perTenantSeen {
		if len(vals) != perTenant {
			t.Fatalf("tenant %s: got %d records, want %d", tenant, len(vals), perTenant)
		}
		for i, v := range vals {
			want := fmt.Sprintf("%s:%d", tenant, i)
			if v != want {
				t.Fatalf("tenant %s: record %d = %q, want %q (per-tenant order broken)", tenant, i, v, want)
			}
		}
	}
}

func TestPollBatchRespectsMaxAndCommitAdvances(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()

	// Append 5 records under the same key -> same partition.
	for i := 0; i < 5; i++ {
		if _, err := m.Append(ctx, "wal", "tenant_a", []byte(fmt.Sprintf("v%d", i)), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	first, err := m.PollBatch(ctx, "wal", "g1", 2, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #1: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("got %d, want 2", len(first))
	}

	// Without commit, PollBatch returns the same prefix.
	again, err := m.PollBatch(ctx, "wal", "g1", 2, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(again) != 2 || again[0].Position != first[0].Position {
		t.Fatalf("uncommitted records did not replay: got %+v", again)
	}

	// Commit the second record -> next poll starts at offset 2.
	if err := m.Commit(ctx, "g1", first[1]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	rest, err := m.PollBatch(ctx, "wal", "g1", 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #3: %v", err)
	}
	if len(rest) != 3 {
		t.Fatalf("got %d, want 3", len(rest))
	}
	if rest[0].Position.Offset != 2 {
		t.Fatalf("first uncommitted offset = %d, want 2", rest[0].Position.Offset)
	}
}

func TestPollBatchBlocksThenReturnsAfterAppend(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()

	doneCh := make(chan []Record, 1)
	go func() {
		recs, err := m.PollBatch(ctx, "wal", "g1", 5, 2*time.Second)
		if err != nil {
			t.Errorf("PollBatch: %v", err)
		}
		doneCh <- recs
	}()

	// Give the poller a moment to park on the cond.
	time.Sleep(50 * time.Millisecond)

	if _, err := m.Append(ctx, "wal", "tenant_a", []byte("late"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}

	select {
	case recs := <-doneCh:
		if len(recs) != 1 || string(recs[0].Value) != "late" {
			t.Fatalf("got %+v, want one record 'late'", recs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PollBatch did not unblock after Append")
	}
}

func TestPollBatchTimeoutReturnsEmpty(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()
	start := time.Now()
	recs, err := m.PollBatch(ctx, "wal", "g1", 5, 100*time.Millisecond)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("got %d records, want 0", len(recs))
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("returned in %v, expected to wait at least 100ms", elapsed)
	}
}

func TestWaitForRecords(t *testing.T) {
	m := newConnected(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		for i := 0; i < 3; i++ {
			_, _ = m.Append(context.Background(), "wal", "tenant_a", []byte("x"), nil)
		}
	}()

	if !m.WaitForRecords(ctx, "wal", 3) {
		t.Fatal("WaitForRecords timed out")
	}
}

func TestWaitForRecordsTimeout(t *testing.T) {
	m := newConnected(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if m.WaitForRecords(ctx, "wal", 1) {
		t.Fatal("expected timeout, got success")
	}
}

func TestClearTopic(t *testing.T) {
	m := newConnected(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := m.Append(ctx, "wal", "tenant_a", []byte("x"), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if got := m.GetRecordCount("wal"); got != 3 {
		t.Fatalf("pre-clear count = %d", got)
	}
	m.ClearTopic("wal")
	if got := m.GetRecordCount("wal"); got != 0 {
		t.Fatalf("post-clear count = %d", got)
	}
	// And appending after clear works fresh from offset 0.
	pos, err := m.Append(ctx, "wal", "tenant_a", []byte("fresh"), nil)
	if err != nil {
		t.Fatalf("Append after clear: %v", err)
	}
	if pos.Offset != 0 {
		t.Fatalf("offset after clear = %d, want 0", pos.Offset)
	}
}

func TestSubscribeStreamsRecords(t *testing.T) {
	m := newConnected(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, errCh, err := m.Subscribe(ctx, "wal", "g1")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	const n = 5
	go func() {
		for i := 0; i < n; i++ {
			_, _ = m.Append(context.Background(), "wal", "tenant_a", []byte(fmt.Sprintf("v%d", i)), nil)
		}
	}()

	got := make([]Record, 0, n)
	timeout := time.After(2 * time.Second)
	for len(got) < n {
		select {
		case r, ok := <-out:
			if !ok {
				t.Fatalf("Subscribe channel closed early; got %d", len(got))
			}
			got = append(got, r)
		case e := <-errCh:
			if e != nil {
				t.Fatalf("Subscribe error: %v", e)
			}
		case <-timeout:
			t.Fatalf("Subscribe timeout; got %d", len(got))
		}
	}
	for i, r := range got {
		want := fmt.Sprintf("v%d", i)
		if string(r.Value) != want {
			t.Fatalf("got[%d] = %q, want %q", i, r.Value, want)
		}
	}
}

func TestPartitionForKeyDeterministic(t *testing.T) {
	m := NewInMemory(4)
	// Same key -> same partition every call.
	first := m.partitionFor("tenant_a")
	for i := 0; i < 100; i++ {
		if got := m.partitionFor("tenant_a"); got != first {
			t.Fatalf("non-deterministic partition: %d vs %d", got, first)
		}
	}
}

func TestCloseUnblocksPoll(t *testing.T) {
	m := newConnected(t)
	doneCh := make(chan error, 1)
	go func() {
		_, err := m.PollBatch(context.Background(), "wal", "g1", 5, 5*time.Second)
		doneCh <- err
	}()
	time.Sleep(50 * time.Millisecond)
	_ = m.Close(context.Background())
	select {
	case err := <-doneCh:
		if !errors.Is(err, ErrConnection) {
			t.Fatalf("err = %v, want ErrConnection", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("PollBatch did not unblock on Close")
	}
}
