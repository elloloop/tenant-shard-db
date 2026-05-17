// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakePubSub implements PubSubAPI in memory. Ordering keys are honored:
// messages with the same ordering key are delivered in publish order
// (the per-tenant total-order guarantee). Pulled-but-unacked messages
// are hidden until releaseInflight() (models the ack deadline).
type fakePubSub struct {
	mu         sync.Mutex
	msgs       []*fakePSMsg
	ackSeq     int
	publishErr error
	pullErr    error
}

type fakePSMsg struct {
	data        []byte
	attrs       map[string]string
	orderingKey string
	ackID       string
	pubMs       int64
	acked       bool
	inflight    bool
}

func (f *fakePubSub) releaseInflight() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if !m.acked {
			m.inflight = false
		}
	}
}

func (f *fakePubSub) Publish(ctx context.Context, topicPath, orderingKey string, data []byte, attrs map[string]string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishErr != nil {
		return "", f.publishErr
	}
	f.ackSeq++
	id := fmt.Sprintf("ack-%d", f.ackSeq)
	cp := map[string]string{}
	for k, v := range attrs {
		cp[k] = v
	}
	f.msgs = append(f.msgs, &fakePSMsg{
		data:        append([]byte(nil), data...),
		attrs:       cp,
		orderingKey: orderingKey,
		ackID:       id,
		pubMs:       time.Now().UnixMilli(),
	})
	return fmt.Sprintf("msg-%d", f.ackSeq), nil
}

func (f *fakePubSub) Pull(ctx context.Context, subPath string, maxMessages int32) ([]pubsubMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pullErr != nil {
		return nil, f.pullErr
	}
	out := []pubsubMessage{}
	for _, m := range f.msgs {
		if m.acked || m.inflight {
			continue
		}
		if int32(len(out)) >= maxMessages {
			break
		}
		m.inflight = true
		out = append(out, pubsubMessage{
			Data:          append([]byte(nil), m.data...),
			Attributes:    m.attrs,
			OrderingKey:   m.orderingKey,
			AckID:         m.ackID,
			PublishTimeMs: m.pubMs,
		})
	}
	if len(out) == 0 {
		return nil, status.Error(codes.DeadlineExceeded, "no messages")
	}
	return out, nil
}

func (f *fakePubSub) Acknowledge(ctx context.Context, subPath string, ackIDs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ackIDs {
		for _, m := range f.msgs {
			if m.ackID == id {
				m.acked = true
			}
		}
	}
	return nil
}

func (f *fakePubSub) Close() error { return nil }

func newTestPubSub(t *testing.T, fake *fakePubSub) *PubSub {
	t.Helper()
	p := NewPubSub(DefaultPubSubConfig("proj", "entdb-wal", "applier-sub"))
	p.newClient = func(ctx context.Context) (PubSubAPI, error) { return fake, nil }
	if err := p.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}

func TestPubSubConnectMissingConfig(t *testing.T) {
	p := NewPubSub(PubSubConfig{})
	if err := p.Connect(context.Background()); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestPubSubAppendNotConnected(t *testing.T) {
	p := NewPubSub(DefaultPubSubConfig("proj", "t", "s"))
	if _, err := p.Append(context.Background(), "t", "k", []byte("v"), nil); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestPubSubAppendPollCommitRoundTrip(t *testing.T) {
	fake := &fakePubSub{}
	p := newTestPubSub(t, fake)
	ctx := context.Background()

	if _, err := p.Append(ctx, "entdb-wal", "tenant_a", []byte("hello"), map[string][]byte{"x-h": []byte("v")}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := p.PollBatch(ctx, "entdb-wal", "applier-sub", 10, 400*time.Millisecond)
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
		t.Fatalf("ordering key = %q, want tenant_a", got[0].Key)
	}
	if string(got[0].Headers["x-h"]) != "v" {
		t.Fatalf("header x-h = %q", got[0].Headers["x-h"])
	}

	// Hidden until ack deadline; then redelivered.
	none, err := p.PollBatch(ctx, "entdb-wal", "applier-sub", 10, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("in-flight msg should be hidden, got %d", len(none))
	}
	fake.releaseInflight()
	again, err := p.PollBatch(ctx, "entdb-wal", "applier-sub", 10, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #3: %v", err)
	}
	if len(again) != 1 {
		t.Fatalf("uncommitted msg not redelivered: got %d", len(again))
	}
	if err := p.Commit(ctx, "applier-sub", again[0]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	fake.releaseInflight()
	empty, err := p.PollBatch(ctx, "entdb-wal", "applier-sub", 10, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #4: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("acked msg redelivered: got %d", len(empty))
	}
}

func TestPubSubIdempotentRetry(t *testing.T) {
	fake := &fakePubSub{}
	p := newTestPubSub(t, fake)
	ctx := context.Background()

	h := map[string][]byte{HeaderIdempotencyKey: []byte("uuid-1")}
	p1, err := p.Append(ctx, "entdb-wal", "tenant_a", []byte("v1"), h)
	if err != nil {
		t.Fatalf("Append #1: %v", err)
	}
	p2, err := p.Append(ctx, "entdb-wal", "tenant_a", []byte("v2"), h)
	if err != nil {
		t.Fatalf("Append #2: %v", err)
	}
	if p1 != p2 {
		t.Fatalf("idempotent retry differs: %+v vs %+v", p1, p2)
	}
	if n := len(fake.msgs); n != 1 {
		t.Fatalf("published %d, want 1 (dedupe)", n)
	}
}

func TestPubSubPerTenantOrder(t *testing.T) {
	fake := &fakePubSub{}
	p := newTestPubSub(t, fake)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := p.Append(ctx, "entdb-wal", "tenant_a", []byte(fmt.Sprintf("v%d", i)), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := p.PollBatch(ctx, "entdb-wal", "applier-sub", 10, 400*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d, want 5", len(got))
	}
	for i, r := range got {
		if want := fmt.Sprintf("v%d", i); string(r.Value) != want {
			t.Fatalf("record %d = %q, want %q (ordering broken)", i, r.Value, want)
		}
	}
}

func TestPubSubPullErrorClassified(t *testing.T) {
	fake := &fakePubSub{}
	p := newTestPubSub(t, fake)
	fake.mu.Lock()
	fake.pullErr = status.Error(codes.NotFound, "no subscription")
	fake.mu.Unlock()
	_, err := p.PollBatch(context.Background(), "entdb-wal", "applier-sub", 5, 150*time.Millisecond)
	if !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestPubSubPollTimeoutEmpty(t *testing.T) {
	fake := &fakePubSub{}
	p := newTestPubSub(t, fake)
	start := time.Now()
	got, err := p.PollBatch(context.Background(), "entdb-wal", "applier-sub", 5, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d, want 0", len(got))
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("returned too fast: %v", time.Since(start))
	}
}
