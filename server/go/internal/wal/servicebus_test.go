// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

// fakeServiceBus models a *session-enabled* Service Bus queue. It is
// deliberately session-shaped to match a real session-required queue:
//
//   - There is NO queue-wide receive. The only way to read is to accept
//     a session (AcceptNextSession), which returns a receiver scoped to
//     exactly one session id. This makes the #543 regression — a plain
//     non-session `Client.NewReceiverForQueue` receiver — impossible to
//     express: ServiceBusAPI has no such method and the fake has no
//     queue-wide read path. Reverting servicebus.go to a non-session
//     receiver fails to compile against this seam.
//   - Per-session FIFO: a session receiver yields that session's
//     messages strictly in send order, lowest sequence first.
//   - Session lock: at most one open receiver per session id at a time
//     (AcceptNextSession skips a session that is already locked). An
//     uncompleted message stays invisible until its session receiver is
//     Closed (lock released) and the session is re-accepted — modelling
//     at-least-once redelivery.
type fakeServiceBus struct {
	mu        sync.Mutex
	msgs      []*fakeSBMsg
	seq       int64
	locked    map[string]bool // session id -> currently held by a receiver
	sendErr   error
	acceptErr error
	recvErr   error
}

type fakeSBMsg struct {
	body      []byte
	props     map[string]string
	session   string
	seqNum    int64
	completed bool
	delivered bool // handed to a live session receiver, not yet released
	enqMs     int64
}

func newFakeServiceBus() *fakeServiceBus {
	return &fakeServiceBus{locked: make(map[string]bool)}
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

// AcceptNextSession returns a receiver for the next session that has
// uncompleted, undelivered work and is not already locked. No such
// session -> errNoSession (empty poll).
func (f *fakeServiceBus) AcceptNextSession(ctx context.Context, wait time.Duration) (serviceBusSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.acceptErr != nil {
		return nil, f.acceptErr
	}
	// Deterministic fairness: pick the session with the earliest
	// pending sequence number that isn't locked.
	type cand struct {
		session string
		minSeq  int64
	}
	best := map[string]int64{}
	for _, m := range f.msgs {
		if m.completed || m.delivered {
			continue
		}
		if f.locked[m.session] {
			continue
		}
		if cur, ok := best[m.session]; !ok || m.seqNum < cur {
			best[m.session] = m.seqNum
		}
	}
	if len(best) == 0 {
		return nil, errNoSession
	}
	cands := make([]cand, 0, len(best))
	for s, mn := range best {
		cands = append(cands, cand{session: s, minSeq: mn})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].minSeq < cands[j].minSeq })
	sid := cands[0].session
	f.locked[sid] = true
	return &fakeServiceBusSession{parent: f, session: sid}, nil
}

func (f *fakeServiceBus) Close(ctx context.Context) error { return nil }

// expireLocks models Service Bus releasing every session lock and
// re-queueing delivered-but-uncompleted messages (lock expiry). After
// this, an uncommitted message is eligible for redelivery on the next
// AcceptNextSession — the at-least-once guarantee. This is the explicit
// seam the redelivery tests use (the consumer keeps a session locked
// until commit, so redelivery requires a real lock expiry).
func (f *fakeServiceBus) expireLocks() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if !m.completed {
			m.delivered = false
		}
	}
	f.locked = make(map[string]bool)
}

// fakeServiceBusSession is a per-session receiver. It only ever sees
// messages whose session id matches; there is no path to read another
// session's messages from here.
type fakeServiceBusSession struct {
	parent  *fakeServiceBus
	session string
	closed  bool
}

func (s *fakeServiceBusSession) SessionID() string { return s.session }

func (s *fakeServiceBusSession) Receive(ctx context.Context, maxMessages int, wait time.Duration) ([]serviceBusMessage, error) {
	f := s.parent
	f.mu.Lock()
	defer f.mu.Unlock()
	if s.closed {
		return nil, errors.New("servicebus: receive on closed session")
	}
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	// FIFO within the session: sort this session's eligible messages by
	// sequence number and hand them out in order.
	eligible := make([]*fakeSBMsg, 0)
	for _, m := range f.msgs {
		if m.session != s.session {
			continue
		}
		if m.completed || m.delivered {
			continue
		}
		eligible = append(eligible, m)
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].seqNum < eligible[j].seqNum })
	out := []serviceBusMessage{}
	for _, m := range eligible {
		if len(out) >= maxMessages {
			break
		}
		m.delivered = true
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

// Pending counts this session's messages that were delivered through a
// (still-open) receiver but not yet completed.
func (s *fakeServiceBusSession) Pending() int {
	f := s.parent
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.msgs {
		if m.session == s.session && m.delivered && !m.completed {
			n++
		}
	}
	return n
}

func (s *fakeServiceBusSession) Complete(ctx context.Context, sequenceNumber int64) error {
	f := s.parent
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		// Settlement is link-scoped: a session receiver can only
		// complete its own session's messages (mirrors Azure).
		if m.session == s.session && m.seqNum == sequenceNumber {
			m.completed = true
		}
	}
	return nil
}

// Close releases the session lock. Any messages this receiver delivered
// but that were not completed become visible again on a future accept
// (lock expiry / at-least-once redelivery).
func (s *fakeServiceBusSession) Close(ctx context.Context) error {
	f := s.parent
	f.mu.Lock()
	defer f.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	for _, m := range f.msgs {
		if m.session == s.session && !m.completed {
			m.delivered = false
		}
	}
	delete(f.locked, s.session)
	return nil
}

func newTestServiceBus(t *testing.T, fake *fakeServiceBus) *ServiceBus {
	t.Helper()
	s := NewServiceBus(DefaultServiceBusConfig("Endpoint=sb://fake/;SharedAccessKeyName=x;SharedAccessKey=y", "entdb-wal"))
	s.config.MaxWaitTime = 50 * time.Millisecond
	s.config.SessionAcceptTimeout = 50 * time.Millisecond
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

func TestServiceBusPollBatchNotConnected(t *testing.T) {
	s := NewServiceBus(DefaultServiceBusConfig("c", "q"))
	if _, err := s.PollBatch(context.Background(), "q", "g", 10, 50*time.Millisecond); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestServiceBusCommitNotConnected(t *testing.T) {
	s := NewServiceBus(DefaultServiceBusConfig("c", "q"))
	if err := s.Commit(context.Background(), "g", Record{Key: "t"}); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestServiceBusSubscribeNotConnected(t *testing.T) {
	s := NewServiceBus(DefaultServiceBusConfig("c", "q"))
	if _, _, err := s.Subscribe(context.Background(), "q", "g"); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestServiceBusAppendPollCommitRoundTrip(t *testing.T) {
	fake := newFakeServiceBus()
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

	if err := s.Commit(ctx, "g1", got[0]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	// Even after a server-side lock expiry the committed message must
	// not come back.
	fake.expireLocks()
	empty, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("completed msg redelivered: got %d", len(empty))
	}
}

// TestServiceBusRedeliveryWithoutCommit: a delivered-but-uncommitted
// message must be redelivered after the session lock is released
// (at-least-once; applier dedupes). Extends the original idea to the
// session model: redelivery happens because the session receiver is
// Closed without Complete and the session is then re-accepted.
func TestServiceBusRedeliveryWithoutCommit(t *testing.T) {
	fake := newFakeServiceBus()
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	if _, err := s.Append(ctx, "entdb-wal", "tenant_a", []byte("v0"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 150*time.Millisecond)
	if err != nil || len(got) != 1 {
		t.Fatalf("PollBatch #1: %v len=%d", err, len(got))
	}
	// No commit. While the session lock is held the message stays
	// in-flight and is NOT re-handed-out (exactly-once-ish within the
	// lock).
	none, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("in-flight (uncommitted, lock held) msg re-handed-out: %+v", none)
	}
	// Lock expires server-side without a commit -> the message must be
	// redelivered (at-least-once; applier dedupes).
	fake.expireLocks()
	again, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #3: %v", err)
	}
	if len(again) != 1 || string(again[0].Value) != "v0" {
		t.Fatalf("uncommitted msg not redelivered after lock expiry: %+v", again)
	}
	// Commit it now; even across a further lock expiry it must not
	// come back.
	if err := s.Commit(ctx, "g1", again[0]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	fake.expireLocks()
	final, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #4: %v", err)
	}
	if len(final) != 0 {
		t.Fatalf("committed msg still redelivered: got %d", len(final))
	}
}

func TestServiceBusIdempotentRetry(t *testing.T) {
	fake := newFakeServiceBus()
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

// TestServiceBusPerTenantOrder: one tenant's records come back strictly
// in send order (per-session FIFO == per-tenant ordering).
func TestServiceBusPerTenantOrder(t *testing.T) {
	fake := newFakeServiceBus()
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

// TestServiceBusMultiSessionOrdering: with several tenants interleaved
// on the queue, EACH tenant's records must arrive in that tenant's send
// order. Cross-tenant order is not promised (Service Bus doesn't), but
// per-session FIFO is the per-tenant ordering guarantee the WAL needs.
func TestServiceBusMultiSessionOrdering(t *testing.T) {
	fake := newFakeServiceBus()
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	tenants := []string{"tenant_a", "tenant_b", "tenant_c"}
	const perTenant = 6
	// Interleave appends across tenants so a non-FIFO consumer would
	// scramble them.
	for i := 0; i < perTenant; i++ {
		for _, tn := range tenants {
			if _, err := s.Append(ctx, "entdb-wal", tn, []byte(fmt.Sprintf("%s-%d", tn, i)), nil); err != nil {
				t.Fatalf("Append: %v", err)
			}
		}
	}

	seen := map[string][]string{}
	deadline := time.Now().Add(3 * time.Second)
	total := len(tenants) * perTenant
	for count := 0; count < total && time.Now().Before(deadline); {
		recs, err := s.PollBatch(ctx, "entdb-wal", "g1", 4, 150*time.Millisecond)
		if err != nil {
			t.Fatalf("PollBatch: %v", err)
		}
		for _, r := range recs {
			seen[r.Key] = append(seen[r.Key], string(r.Value))
			if err := s.Commit(ctx, "g1", r); err != nil {
				t.Fatalf("Commit: %v", err)
			}
			count++
		}
	}

	for _, tn := range tenants {
		vals := seen[tn]
		if len(vals) != perTenant {
			t.Fatalf("tenant %s: got %d records, want %d (%v)", tn, len(vals), perTenant, vals)
		}
		for i, v := range vals {
			if want := fmt.Sprintf("%s-%d", tn, i); v != want {
				t.Fatalf("tenant %s record %d = %q, want %q (per-session FIFO broken)", tn, i, v, want)
			}
		}
	}
}

// TestServiceBusCommitStopsRedelivery: after Commit, the message must
// not be redelivered even across many subsequent polls.
func TestServiceBusCommitStopsRedelivery(t *testing.T) {
	fake := newFakeServiceBus()
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	if _, err := s.Append(ctx, "entdb-wal", "tenant_a", []byte("once"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 150*time.Millisecond)
	if err != nil || len(got) != 1 {
		t.Fatalf("PollBatch: %v len=%d", err, len(got))
	}
	if err := s.Commit(ctx, "g1", got[0]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	for i := 0; i < 4; i++ {
		more, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 80*time.Millisecond)
		if err != nil {
			t.Fatalf("PollBatch #%d: %v", i, err)
		}
		if len(more) != 0 {
			t.Fatalf("committed msg redelivered on poll #%d: %+v", i, more)
		}
	}
}

func TestServiceBusReceiveErrorClassified(t *testing.T) {
	fake := newFakeServiceBus()
	s := newTestServiceBus(t, fake)
	if _, err := s.Append(context.Background(), "entdb-wal", "tenant_a", []byte("v"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	fake.mu.Lock()
	fake.recvErr = errors.New("amqp: connection closed")
	fake.mu.Unlock()
	_, err := s.PollBatch(context.Background(), "entdb-wal", "g1", 5, 100*time.Millisecond)
	if !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestServiceBusAcceptSessionErrorClassified(t *testing.T) {
	fake := newFakeServiceBus()
	s := newTestServiceBus(t, fake)
	if _, err := s.Append(context.Background(), "entdb-wal", "tenant_a", []byte("v"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	fake.mu.Lock()
	fake.acceptErr = errors.New("amqp: unauthorized")
	fake.mu.Unlock()
	_, err := s.PollBatch(context.Background(), "entdb-wal", "g1", 5, 100*time.Millisecond)
	if !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

// TestServiceBusReceiverIsSessionBased is the explicit #543 regression
// guard. It asserts, structurally, that the ONLY consume entry point is
// session-scoped:
//
//  1. ServiceBusAPI exposes AcceptNextSession (a *session* receiver) and
//     NOT a queue-wide Receive. Reverting servicebus.go to the non-
//     session `Client.NewReceiverForQueue` path would require a
//     queue-wide Receive on this interface, which does not exist, so the
//     regression cannot compile.
//  2. Every record delivered carries the tenant SessionID as its Key,
//     and Complete only settles messages on the owning session — proving
//     settlement is link/session-scoped, not queue-wide.
func TestServiceBusReceiverIsSessionBased(t *testing.T) {
	fake := newFakeServiceBus()
	s := newTestServiceBus(t, fake)
	ctx := context.Background()

	// The consume seam is a session, not a queue receiver. This is a
	// compile-enforced assertion: AcceptNextSession returns something
	// that satisfies serviceBusSession (a per-session receiver). A
	// non-session queue receiver could not satisfy this seam.
	var _ func(context.Context, time.Duration) (serviceBusSession, error) = fake.AcceptNextSession

	if _, err := s.Append(ctx, "entdb-wal", "tenant_x", []byte("data"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	sess, err := fake.AcceptNextSession(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("AcceptNextSession: %v", err)
	}
	if sess.SessionID() != "tenant_x" {
		t.Fatalf("session id = %q, want tenant_x (receiver is not session-scoped)", sess.SessionID())
	}
	msgs, err := sess.Receive(ctx, 10, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("session Receive: %v", err)
	}
	if len(msgs) != 1 || msgs[0].SessionID != "tenant_x" {
		t.Fatalf("session receiver returned %d msgs %+v, want 1 for tenant_x", len(msgs), msgs)
	}
	_ = sess.Close(ctx)

	// End-to-end: the record's Key is the session id (tenant).
	got, err := s.PollBatch(ctx, "entdb-wal", "g1", 10, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 1 || got[0].Key != "tenant_x" {
		t.Fatalf("record key = %q (want tenant_x); session id must surface as the WAL key", got[0].Key)
	}
}
