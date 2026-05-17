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

// fakeEventHubs implements EventHubsAPI in memory with two partitions.
// Events route to a partition by hashing the partition key (so a given
// tenant always lands on one partition -> per-tenant order). Receive
// honors the afterSeq cursor so the checkpoint/commit path is testable.
type fakeEventHubs struct {
	mu      sync.Mutex
	parts   []string
	byPart  map[string][]fakeEHEvent
	seq     int64
	sendErr error
	recvErr error
}

type fakeEHEvent struct {
	body   []byte
	props  map[string]string
	pk     string
	pid    string
	seqNum int64
	enqMs  int64
}

func newFakeEventHubs() *fakeEventHubs {
	return &fakeEventHubs{
		parts:  []string{"0", "1"},
		byPart: map[string][]fakeEHEvent{},
	}
}

func (f *fakeEventHubs) partitionFor(pk string) string {
	// Simple deterministic hash into 2 partitions.
	h := 0
	for _, c := range pk {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return f.parts[h%len(f.parts)]
}

func (f *fakeEventHubs) Send(ctx context.Context, partitionKey string, body []byte, props map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return f.sendErr
	}
	f.seq++
	pid := f.partitionFor(partitionKey)
	cp := map[string]string{}
	for k, v := range props {
		cp[k] = v
	}
	f.byPart[pid] = append(f.byPart[pid], fakeEHEvent{
		body:   append([]byte(nil), body...),
		props:  cp,
		pk:     partitionKey,
		pid:    pid,
		seqNum: f.seq,
		enqMs:  time.Now().UnixMilli(),
	})
	return nil
}

func (f *fakeEventHubs) Partitions(ctx context.Context) ([]string, error) {
	return append([]string(nil), f.parts...), nil
}

func (f *fakeEventHubs) Receive(ctx context.Context, partitionID string, afterSeq int64, count int, wait time.Duration) ([]eventHubEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	out := []eventHubEvent{}
	for _, ev := range f.byPart[partitionID] {
		if afterSeq >= 0 && ev.seqNum <= afterSeq {
			continue
		}
		if len(out) >= count {
			break
		}
		out = append(out, eventHubEvent{
			Body:           append([]byte(nil), ev.body...),
			Properties:     ev.props,
			PartitionKey:   ev.pk,
			PartitionID:    ev.pid,
			SequenceNumber: ev.seqNum,
			EnqueuedMs:     ev.enqMs,
		})
	}
	return out, nil
}

func (f *fakeEventHubs) Close(ctx context.Context) error { return nil }

func newTestEventHubs(t *testing.T, fake *fakeEventHubs) *EventHubs {
	t.Helper()
	e := NewEventHubs(DefaultEventHubsConfig("Endpoint=sb://fake/;SharedAccessKeyName=x;SharedAccessKey=y", "entdb-wal", ""))
	e.config.MaxWaitTime = 100 * time.Millisecond
	e.newClient = func(ctx context.Context) (EventHubsAPI, error) { return fake, nil }
	if err := e.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = e.Close(context.Background()) })
	return e
}

func TestEventHubsConnectMissingConfig(t *testing.T) {
	e := NewEventHubs(EventHubsConfig{})
	if err := e.Connect(context.Background()); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestEventHubsAppendNotConnected(t *testing.T) {
	e := NewEventHubs(DefaultEventHubsConfig("c", "h", ""))
	if _, err := e.Append(context.Background(), "h", "k", []byte("v"), nil); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestEventHubsAppendPollRoundTrip(t *testing.T) {
	fake := newFakeEventHubs()
	e := newTestEventHubs(t, fake)
	ctx := context.Background()

	if _, err := e.Append(ctx, "entdb-wal", "tenant_a", []byte("hello"), map[string][]byte{"x-h": []byte("v")}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := e.PollBatch(ctx, "entdb-wal", "$Default", 10, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if string(got[0].Value) != "hello" {
		t.Fatalf("value = %q", got[0].Value)
	}
	if got[0].Key != "tenant_a" {
		t.Fatalf("partition key = %q, want tenant_a", got[0].Key)
	}
	if string(got[0].Headers["x-h"]) != "v" {
		t.Fatalf("header x-h = %q", got[0].Headers["x-h"])
	}
}

func TestEventHubsCommitAdvancesCheckpoint(t *testing.T) {
	fake := newFakeEventHubs()
	e := newTestEventHubs(t, fake)
	ctx := context.Background()

	// All same tenant -> same partition -> ordered.
	for i := 0; i < 4; i++ {
		if _, err := e.Append(ctx, "entdb-wal", "tenant_a", []byte(fmt.Sprintf("v%d", i)), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := e.PollBatch(ctx, "entdb-wal", "$Default", 10, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d, want 4", len(got))
	}
	for i, r := range got {
		if want := fmt.Sprintf("v%d", i); string(r.Value) != want {
			t.Fatalf("record %d = %q, want %q (per-tenant order broken)", i, r.Value, want)
		}
	}
	// Commit the 2nd record; re-poll resumes after it.
	if err := e.Commit(ctx, "$Default", got[1]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	rest, err := e.PollBatch(ctx, "entdb-wal", "$Default", 10, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(rest) != 2 {
		t.Fatalf("got %d after commit, want 2", len(rest))
	}
	if string(rest[0].Value) != "v2" {
		t.Fatalf("resumed at %q, want v2", rest[0].Value)
	}
}

func TestEventHubsIdempotentRetry(t *testing.T) {
	fake := newFakeEventHubs()
	e := newTestEventHubs(t, fake)
	ctx := context.Background()

	h := map[string][]byte{HeaderIdempotencyKey: []byte("uuid-1")}
	p1, err := e.Append(ctx, "entdb-wal", "tenant_a", []byte("v1"), h)
	if err != nil {
		t.Fatalf("Append #1: %v", err)
	}
	p2, err := e.Append(ctx, "entdb-wal", "tenant_a", []byte("v2"), h)
	if err != nil {
		t.Fatalf("Append #2: %v", err)
	}
	if p1 != p2 {
		t.Fatalf("idempotent retry differs: %+v vs %+v", p1, p2)
	}
	total := 0
	for _, evs := range fake.byPart {
		total += len(evs)
	}
	if total != 1 {
		t.Fatalf("sent %d events, want 1 (dedupe)", total)
	}
}

func TestEventHubsReceiveErrorClassified(t *testing.T) {
	fake := newFakeEventHubs()
	e := newTestEventHubs(t, fake)
	if _, err := e.Append(context.Background(), "entdb-wal", "tenant_a", []byte("v"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	fake.mu.Lock()
	fake.recvErr = errors.New("amqp: link detached, reason: unauthorized")
	fake.mu.Unlock()
	_, err := e.PollBatch(context.Background(), "entdb-wal", "$Default", 5, 150*time.Millisecond)
	if !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestPartitionIDToInt32(t *testing.T) {
	cases := map[string]int32{"0": 0, "1": 1, "12": 12, "": 0, "abc": 0}
	for in, want := range cases {
		if got := partitionIDToInt32(in); got != want {
			t.Fatalf("partitionIDToInt32(%q) = %d, want %d", in, got, want)
		}
	}
}
