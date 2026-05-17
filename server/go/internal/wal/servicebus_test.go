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

// fakeServiceBus implements ServiceBusAPI in memory. Messages with the
// same session id are delivered in send order (per-tenant order).
// Received-but-uncompleted messages are hidden until releaseInflight()
// (models the message lock).
type fakeServiceBus struct {
	mu      sync.Mutex
	msgs    []*fakeSBMsg
	seq     int64
	sendErr error
	recvErr error
}

type fakeSBMsg struct {
	body      []byte
	props     map[string]string
	session   string
	seqNum    int64
	completed bool
	inflight  bool
	enqMs     int64
}

func (f *fakeServiceBus) releaseInflight() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if !m.completed {
			m.inflight = false
		}
	}
}

func (f *fakeServiceBus) Send(ctx context.Context, sessionID string, body []byte, props map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return f.sendErr
	}
	f.seq++
	cp := map[string]string{}
	for k, v := range props {
		cp[k] = v
	}
	f.msgs = append(f.msgs, &fakeSBMsg{
		body:    append([]byte(nil), body...),
		props:   cp,
		session: sessionID,
		seqNum:  f.seq,
		enqMs:   time.Now().UnixMilli(),
	})
	return nil
}

func (f *fakeServiceBus) Receive(ctx context.Context, maxMessages int, wait time.Duration) ([]serviceBusMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	out := []serviceBusMessage{}
	for _, m := range f.msgs {
		if m.completed || m.inflight {
			continue
		}
		if len(out) >= maxMessages {
			break
		}
		m.inflight = true
		out = append(out, serviceBusMessage{
			Body:           append([]byte(nil), m.body...),
			Properties:     m.props,
			SessionID:      m.session,
			SequenceNumber: m.seqNum,
			EnqueuedMs:     m.enqMs,
		})
	}
	return out, nil
}

func (f *fakeServiceBus) Complete(ctx context.Context, sequenceNumber int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if m.seqNum == sequenceNumber {
			m.completed = true
		}
	}
	return nil
}

func (f *fakeServiceBus) Close(ctx context.Context) error { return nil }

func newTestServiceBus(t *testing.T, fake *fakeServiceBus) *ServiceBus {
	t.Helper()
	s := NewServiceBus(DefaultServiceBusConfig("Endpoint=sb://fake/;SharedAccessKeyName=x;SharedAccessKey=y", "entdb-wal"))
	s.config.MaxWaitTime = 100 * time.Millisecond
	s.newClient = func(ctx context.Context) (ServiceBusAPI, error) { return fake, nil }
	if err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s
}

func TestServiceBusConnectMissingConfig(t *testing.T) {
	s := NewServiceBus(ServiceBusConfig{})
	if err := s.Connect(context.Background()); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestServiceBusAppendNotConnected(t *testing.T) {
	s := NewServiceBus(DefaultServiceBusConfig("c", "q"))
	if _, err := s.Append(context.Background(), "q", "k", []byte("v"), nil); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestServiceBusAppendPollCommitRoundTrip(t *testing.T) {
	fake := &fakeServiceBus{}
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	if _, err := s.Append(ctx, "entdb-wal", "tenant_a", []byte("hello"), map[string][]byte{"x-h": []byte("v")}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 200*time.Millisecond)
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
		t.Fatalf("session id = %q, want tenant_a", got[0].Key)
	}
	if string(got[0].Headers["x-h"]) != "v" {
		t.Fatalf("header x-h = %q", got[0].Headers["x-h"])
	}

	none, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("locked msg should be hidden, got %d", len(none))
	}
	if err := s.Commit(ctx, "g1", got[0]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	fake.releaseInflight()
	empty, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #3: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("completed msg redelivered: got %d", len(empty))
	}
}

func TestServiceBusRedeliveryWithoutCommit(t *testing.T) {
	fake := &fakeServiceBus{}
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	if _, err := s.Append(ctx, "entdb-wal", "tenant_a", []byte("v0"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 150*time.Millisecond)
	if err != nil || len(got) != 1 {
		t.Fatalf("PollBatch #1: %v len=%d", err, len(got))
	}
	// No commit -> lock expires -> redelivered.
	fake.releaseInflight()
	again, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(again) != 1 || string(again[0].Value) != "v0" {
		t.Fatalf("uncommitted msg not redelivered: %+v", again)
	}
}

func TestServiceBusIdempotentRetry(t *testing.T) {
	fake := &fakeServiceBus{}
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	h := map[string][]byte{HeaderIdempotencyKey: []byte("uuid-1")}
	p1, err := s.Append(ctx, "entdb-wal", "tenant_a", []byte("v1"), h)
	if err != nil {
		t.Fatalf("Append #1: %v", err)
	}
	p2, err := s.Append(ctx, "entdb-wal", "tenant_a", []byte("v2"), h)
	if err != nil {
		t.Fatalf("Append #2: %v", err)
	}
	if p1 != p2 {
		t.Fatalf("idempotent retry differs: %+v vs %+v", p1, p2)
	}
	if n := len(fake.msgs); n != 1 {
		t.Fatalf("sent %d, want 1 (dedupe)", n)
	}
}

func TestServiceBusPerTenantOrder(t *testing.T) {
	fake := &fakeServiceBus{}
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := s.Append(ctx, "entdb-wal", "tenant_a", []byte(fmt.Sprintf("v%d", i)), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d, want 5", len(got))
	}
	for i, r := range got {
		if want := fmt.Sprintf("v%d", i); string(r.Value) != want {
			t.Fatalf("record %d = %q, want %q (session order broken)", i, r.Value, want)
		}
	}
}

func TestServiceBusReceiveErrorClassified(t *testing.T) {
	fake := &fakeServiceBus{}
	s := newTestServiceBus(t, fake)
	fake.mu.Lock()
	fake.recvErr = errors.New("amqp: connection closed")
	fake.mu.Unlock()
	_, err := s.PollBatch(context.Background(), "entdb-wal", "g1", 5, 100*time.Millisecond)
	if !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}
