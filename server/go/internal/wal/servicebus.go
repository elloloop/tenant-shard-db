// SPDX-License-Identifier: AGPL-3.0-only

package wal

// Azure Service Bus WAL backend for the Go server. Ported from the
// retired Python source (server/python/entdb_server/wal/servicebus.py).
//
// Concept mapping (mirrors the Python docstring):
//
//   - Service Bus queue   -> WAL topic. The queue MUST be
//                            session-enabled for ordering to hold.
//   - SessionId           -> partition key (tenant_id). Service Bus
//                            guarantees ordered delivery within a
//                            session, so per-tenant total order is
//                            preserved exactly like Kafka's per-key
//                            order.
//   - SequenceNumber      -> offset. Service Bus assigns a broker-side
//                            monotone int64 per message; we surface it
//                            as StreamPos.Offset.
//   - Complete            -> commit. Commit calls CompleteMessage (the
//                            Service Bus ack); an uncompleted message
//                            is redelivered after the session lock
//                            expires.
//
// Session receivers (issue #543): the producer stamps every message
// with SessionId = tenant key, and a session-enabled Service Bus queue
// REQUIRES a *session* receiver — a plain `Client.NewReceiverForQueue`
// receiver fails at runtime ("entity requires sessions") and, even if
// it did not, could not preserve per-session FIFO. We therefore drive
// the consumer through `Client.AcceptNextSessionForQueue`, servicing
// one tenant session at a time and rotating fairly across sessions so
// no tenant starves. Per-session FIFO == per-tenant ordering; ordering
// across sessions is not promised by Service Bus and is not required by
// the WAL contract (per-key order only). The ServiceBusAPI seam below
// is deliberately session-shaped: there is no queue-wide Receive, so a
// non-session receiver is structurally unrepresentable.
//
// Service Bus has no partitions; StreamPos.Partition is always 0.
//
// Idempotency: the in-process (topic,key,idempotency-key) cache short-
// circuits a retried Append, identical to the other backends. Service
// Bus duplicate detection (if enabled on the queue) is an additional
// server-side gate but we do not depend on it.
//
// Halt-on-poison: message bodies pass through verbatim; a malformed
// event surfaces at the applier (CLAUDE.md: WAL is the source of
// truth).
//
// Testing: ServiceBusAPI / serviceBusSession are the seams. A thin
// adapter wraps the SDK client/sender/session-receiver; unit tests
// inject a fake that models sessions — no live Azure.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// ServiceBusAPI is the slice of Service Bus operations the WAL backend
// needs. It is deliberately session-shaped on the consume side: the
// only way to read is to AcceptNextSession (a *session* receiver), so a
// non-session receiver — the #543 bug — cannot be expressed through
// this interface. The adapter in defaultNewClient wraps the SDK; tests
// fake it.
type ServiceBusAPI interface {
	// Send sends one message to the queue with the given session id +
	// application properties.
	Send(ctx context.Context, sessionID string, body []byte, props map[string]string) error
	// AcceptNextSession accepts the next available session on the queue
	// and returns a receiver scoped to exactly that session. It blocks
	// up to wait for a session to become available; when none is
	// available within wait it returns errNoSession (a normal "nothing
	// to do right now" signal, not a failure). Per Azure semantics each
	// accepted session is held (locked) until the returned
	// serviceBusSession is Closed, at which point another consumer —
	// or this one, on the next accept — may take it.
	AcceptNextSession(ctx context.Context, wait time.Duration) (serviceBusSession, error)
	// Close releases the client.
	Close(ctx context.Context) error
}

// serviceBusSession is a receiver bound to a single Service Bus session
// (one tenant). Messages it returns are in per-session FIFO order;
// Complete acks a message by sequence number on this session's link.
// There is no cross-session receive here by construction.
type serviceBusSession interface {
	// SessionID is the session this receiver is locked to.
	SessionID() string
	// Receive receives up to maxMessages from this session, waiting at
	// most wait. An empty slice with a nil error means "no more right
	// now" (the session is drained or the wait elapsed).
	Receive(ctx context.Context, maxMessages int, wait time.Duration) ([]serviceBusMessage, error)
	// Complete acks (completes) a previously-received message by its
	// sequence number on this session's link.
	Complete(ctx context.Context, sequenceNumber int64) error
	// Pending reports how many received-but-not-yet-completed messages
	// this session receiver still holds. Zero means every delivered
	// message has been settled, so the consumer may release the session
	// lock (close it) and let the accept-next loop rotate to the next
	// tenant — keeping the queue fully drainable without starving any
	// tenant.
	Pending() int
	// Close releases the session lock so another accept can take it.
	Close(ctx context.Context) error
}

// errNoSession is returned by AcceptNextSession when no session became
// available within the wait window. It is an internal control signal
// (an empty poll), never surfaced to WAL callers as an error.
var errNoSession = errors.New("servicebus: no session available")

// serviceBusMessage is the transport-neutral shape Receive returns.
type serviceBusMessage struct {
	Body           []byte
	Properties     map[string]string
	SessionID      string
	SequenceNumber int64
	EnqueuedMs     int64
}

// ServiceBusConfig captures the Service Bus-specific knobs (parity with
// the Python ServiceBusConfig).
type ServiceBusConfig struct {
	// ConnectionString is the Service Bus namespace connection string
	// (Endpoint=sb://...;SharedAccessKeyName=...;SharedAccessKey=...).
	ConnectionString string
	// QueueName is the session-enabled queue backing the WAL.
	QueueName string
	// MaxWaitTime bounds a single Receive call. Python default 5s.
	MaxWaitTime time.Duration
	// SessionAcceptTimeout bounds a single AcceptNextSession call. If a
	// session does not become available within this window the accept
	// is treated as an empty poll and the consumer rotates. Defaults to
	// 5s.
	SessionAcceptTimeout time.Duration
}

// DefaultServiceBusConfig returns a config matching the Python defaults.
func DefaultServiceBusConfig(connStr, queueName string) ServiceBusConfig {
	return ServiceBusConfig{
		ConnectionString:     connStr,
		QueueName:            queueName,
		MaxWaitTime:          5 * time.Second,
		SessionAcceptTimeout: 5 * time.Second,
	}
}

// ServiceBus implements Producer + Consumer against Azure Service Bus.
type ServiceBus struct {
	config ServiceBusConfig

	newClient func(ctx context.Context) (ServiceBusAPI, error)

	mu        sync.Mutex
	connected bool
	api       ServiceBusAPI

	idemp map[string]map[string]map[string]StreamPos

	// open[sessionID] -> the session receiver currently held for that
	// tenant. Commit resolves the owning session by the record's Key
	// (SessionID); the consume loop closes a session once it drains so
	// the lock can rotate to whoever needs it next.
	open map[string]serviceBusSession
}

// NewServiceBus constructs a Service Bus WAL backend.
func NewServiceBus(cfg ServiceBusConfig) *ServiceBus {
	if cfg.MaxWaitTime <= 0 {
		cfg.MaxWaitTime = 5 * time.Second
	}
	if cfg.SessionAcceptTimeout <= 0 {
		cfg.SessionAcceptTimeout = 5 * time.Second
	}
	s := &ServiceBus{
		config: cfg,
		idemp:  make(map[string]map[string]map[string]StreamPos),
		open:   make(map[string]serviceBusSession),
	}
	s.newClient = s.defaultNewClient
	return s
}

func (s *ServiceBus) defaultNewClient(ctx context.Context) (ServiceBusAPI, error) {
	client, err := azservicebus.NewClientFromConnectionString(s.config.ConnectionString, nil)
	if err != nil {
		return nil, err
	}
	sender, err := client.NewSender(s.config.QueueName, nil)
	if err != nil {
		_ = client.Close(ctx)
		return nil, err
	}
	return &azServiceBusAdapter{
		client:    client,
		sender:    sender,
		queueName: s.config.QueueName,
	}, nil
}

// Connect builds the client (sender). Service Bus has no cheap
// describe; connectivity surfaces on first Send/AcceptNextSession
// (parity with servicebus.py, which only created the client here).
func (s *ServiceBus) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected {
		return nil
	}
	if strings.TrimSpace(s.config.ConnectionString) == "" || strings.TrimSpace(s.config.QueueName) == "" {
		return fmt.Errorf("%w: servicebus: connection string and queue name required", ErrConnection)
	}
	api, err := s.newClient(ctx)
	if err != nil {
		return fmt.Errorf("%w: servicebus client: %v", ErrConnection, err)
	}
	s.api = api
	s.connected = true
	return nil
}

// Close releases the client and any held session receivers. Safe to
// call multiple times.
func (s *ServiceBus) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.open {
		_ = sess.Close(ctx)
		delete(s.open, id)
	}
	if s.api != nil {
		_ = s.api.Close(ctx)
		s.api = nil
	}
	s.connected = false
	return nil
}

// Append sends value with SessionId = tenant key for per-tenant order.
// See Producer.Append.
func (s *ServiceBus) Append(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string][]byte,
) (StreamPos, error) {
	if err := ctx.Err(); err != nil {
		return StreamPos{}, err
	}
	s.mu.Lock()
	if !s.connected || s.api == nil {
		s.mu.Unlock()
		return StreamPos{}, fmt.Errorf("%w: not connected", ErrConnection)
	}
	idempKey := idempotencyKey(headers)
	if idempKey != "" {
		if pos, ok := lookupIdemp(s.idemp, topic, key, idempKey); ok {
			s.mu.Unlock()
			return pos, nil
		}
	}
	api := s.api
	s.mu.Unlock()

	if err := api.Send(ctx, key, value, stringHeaders(headers)); err != nil {
		return StreamPos{}, classifyServiceBusErr("servicebus send", err)
	}

	nowMs := time.Now().UnixMilli()
	pos := StreamPos{
		Topic:       topic,
		Partition:   0,
		Offset:      nowMs,
		TimestampMs: nowMs,
	}
	if idempKey != "" {
		s.mu.Lock()
		storeIdemp(s.idemp, topic, key, idempKey, pos)
		s.mu.Unlock()
	}
	return pos, nil
}

// PollBatch services tenant sessions and returns up to maxRecords,
// blocking up to timeout. It accepts the next available session,
// receives what it can from that session in per-tenant FIFO order, then
// rotates to the next session for fairness so no tenant starves. A
// session from which records were delivered is kept open (tracked in
// s.open) so Commit can ack on its link and so its still-locked
// messages are not handed out twice within a poll cycle; a session that
// yielded nothing is closed immediately so its lock rotates. Sessions
// stay tracked across PollBatch calls until either every delivered
// record is Committed (then the session is closed and its lock
// released) or the lock is lost server-side (then those records
// redeliver — at-least-once; the applier dedupes).
func (s *ServiceBus) PollBatch(
	ctx context.Context,
	topic, groupID string,
	maxRecords int,
	timeout time.Duration,
) ([]Record, error) {
	if maxRecords <= 0 {
		return nil, nil
	}
	s.mu.Lock()
	if !s.connected || s.api == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	api := s.api
	s.mu.Unlock()

	deadline := time.Now().Add(timeout)
	out := make([]Record, 0, maxRecords)
	// Sessions we accepted this call but already drained, so the
	// accept-next loop does not immediately re-pick them and spin.
	visited := make(map[string]bool)

	for len(out) < maxRecords {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		acceptWait := remaining
		if acceptWait > s.config.SessionAcceptTimeout {
			acceptWait = s.config.SessionAcceptTimeout
		}
		sess, err := api.AcceptNextSession(ctx, acceptWait)
		if err != nil {
			if errors.Is(err, errNoSession) {
				// No session ready within the window. Return whatever we
				// gathered; an empty slice + nil error is a normal "no
				// records yet" signal per the Consumer contract.
				break
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if len(out) > 0 {
					return out, nil
				}
				return nil, err
			}
			return nil, classifyServiceBusErr("servicebus accept session", err)
		}
		if sess == nil {
			break
		}

		sid := sess.SessionID()
		if visited[sid] {
			// We already serviced this tenant this poll cycle. Releasing
			// its lock lets the accept-next loop move on to other
			// tenants (fair rotation) instead of spinning on this one.
			s.closeSession(ctx, sess)
			break
		}
		visited[sid] = true

		rem := time.Until(deadline)
		if rem <= 0 {
			s.closeSession(ctx, sess)
			break
		}
		recvWait := rem
		if recvWait > s.config.MaxWaitTime {
			recvWait = s.config.MaxWaitTime
		}
		msgs, rerr := sess.Receive(ctx, maxRecords-len(out), recvWait)
		if rerr != nil {
			if errors.Is(rerr, context.Canceled) || errors.Is(rerr, context.DeadlineExceeded) {
				s.closeSession(ctx, sess)
				if len(out) > 0 {
					return out, nil
				}
				return nil, rerr
			}
			s.closeSession(ctx, sess)
			return nil, classifyServiceBusErr("servicebus receive", rerr)
		}
		if len(msgs) == 0 {
			// Empty session: release the lock so it rotates and another
			// tenant (or this one later) can be accepted.
			s.closeSession(ctx, sess)
			continue
		}
		for _, m := range msgs {
			out = append(out, Record{
				Key:   m.SessionID,
				Value: m.Body,
				Position: StreamPos{
					Topic:       topic,
					Partition:   0,
					Offset:      m.SequenceNumber,
					TimestampMs: m.EnqueuedMs,
				},
				Headers: bytesHeaders(m.Properties),
			})
		}
		// Keep this session open so Commit can ack on its link and so
		// its in-flight (received, uncommitted) messages stay locked
		// (not re-handed-out) until committed or the lock is lost.
		s.trackSession(ctx, sess)
	}
	return out, nil
}

// trackSession records a still-open session so Commit can resolve its
// link by session id. If a previous receiver for the same session is
// already tracked it is closed first (a fresh accept supersedes it).
func (s *ServiceBus) trackSession(ctx context.Context, sess serviceBusSession) {
	id := sess.SessionID()
	s.mu.Lock()
	prev, ok := s.open[id]
	s.open[id] = sess
	s.mu.Unlock()
	if ok && prev != nil && prev != sess {
		_ = prev.Close(ctx)
	}
}

// closeSession closes a session receiver and drops it from the tracked
// set if it was the tracked one for its session id.
func (s *ServiceBus) closeSession(ctx context.Context, sess serviceBusSession) {
	id := sess.SessionID()
	s.mu.Lock()
	if cur, ok := s.open[id]; ok && cur == sess {
		delete(s.open, id)
	}
	s.mu.Unlock()
	_ = sess.Close(ctx)
}

// Subscribe streams records by repeatedly polling sessions.
// Auto-completes each delivered record, matching the InMemory/Kafka
// Subscribe contract.
func (s *ServiceBus) Subscribe(
	ctx context.Context,
	topic, groupID string,
) (<-chan Record, <-chan error, error) {
	s.mu.Lock()
	if !s.connected || s.api == nil {
		s.mu.Unlock()
		return nil, nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	s.mu.Unlock()

	out := make(chan Record)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		for {
			if ctx.Err() != nil {
				return
			}
			recs, err := s.PollBatch(ctx, topic, groupID, 32, s.config.MaxWaitTime)
			if err != nil {
				if ctx.Err() == nil {
					errCh <- err
				}
				return
			}
			for _, r := range recs {
				select {
				case <-ctx.Done():
					return
				case out <- r:
					_ = s.Commit(ctx, groupID, r)
				}
			}
		}
	}()
	return out, errCh, nil
}

// Commit completes (acks) the message on the receiver that owns its
// session. Mirrors servicebus.py commit -> complete_message. Service
// Bus settlement is link-scoped: the message must be acked on the very
// session receiver that delivered it, which is why we resolve the
// session by record.Key (the SessionId / tenant) rather than holding a
// single queue-wide receiver.
func (s *ServiceBus) Commit(ctx context.Context, groupID string, record Record) error {
	s.mu.Lock()
	if !s.connected || s.api == nil {
		s.mu.Unlock()
		return fmt.Errorf("%w: not connected", ErrConnection)
	}
	sess, ok := s.open[record.Key]
	s.mu.Unlock()
	if !ok || sess == nil {
		// The session lock was already released (drained + closed, or a
		// rebalance). The message will be redelivered on a future accept
		// of that session; the applier dedupes (at-least-once). This
		// mirrors the "no pending message" no-op path in servicebus.py.
		return nil
	}
	if err := sess.Complete(ctx, record.Position.Offset); err != nil {
		return classifyServiceBusErr("servicebus complete", err)
	}
	// Once every in-flight message on this session is settled, release
	// the session lock so the accept-next loop can re-take it (to drain
	// any further messages for the same tenant) and so other tenants
	// are not starved by a held-but-idle lock.
	if sess.Pending() == 0 {
		s.closeSession(ctx, sess)
	}
	return nil
}

func classifyServiceBusErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	// Azure surfaces actionable failures as *azservicebus.Error with a
	// programmatic Code. Map those before falling back to string sniff.
	var sbErr *azservicebus.Error
	if errors.As(err, &sbErr) {
		switch sbErr.Code {
		case azservicebus.CodeTimeout:
			return fmt.Errorf("%w: %s: %v", ErrTimeout, op, err)
		case azservicebus.CodeConnectionLost, azservicebus.CodeUnauthorizedAccess,
			azservicebus.CodeNotFound, azservicebus.CodeClosed:
			return fmt.Errorf("%w: %s: %v", ErrConnection, op, err)
		}
	}
	if isTimeout(err) {
		return fmt.Errorf("%w: %s: %v", ErrTimeout, op, err)
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "not found") || strings.Contains(low, "unauthorized") ||
		strings.Contains(low, "connection") || strings.Contains(low, "no such host") {
		return fmt.Errorf("%w: %s: %v", ErrConnection, op, err)
	}
	return fmt.Errorf("%w: %s: %v", ErrWal, op, err)
}

// azServiceBusAdapter adapts the SDK client/sender to ServiceBusAPI.
// The consume side hands back azServiceBusSession values, one per
// accepted Service Bus session, so settlement is always link-correct.
type azServiceBusAdapter struct {
	client    *azservicebus.Client
	sender    *azservicebus.Sender
	queueName string
}

func (a *azServiceBusAdapter) Send(ctx context.Context, sessionID string, body []byte, props map[string]string) error {
	msg := &azservicebus.Message{Body: body}
	if sessionID != "" {
		msg.SessionID = &sessionID
	}
	if len(props) > 0 {
		ap := make(map[string]any, len(props))
		for k, v := range props {
			ap[k] = v
		}
		msg.ApplicationProperties = ap
	}
	return a.sender.SendMessage(ctx, msg, nil)
}

func (a *azServiceBusAdapter) AcceptNextSession(ctx context.Context, wait time.Duration) (serviceBusSession, error) {
	actx := ctx
	if wait > 0 {
		var cancel context.CancelFunc
		actx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	// AcceptNextSessionForQueue blocks until a session is available or
	// the (bounded) context elapses. When no session is available it
	// returns an *azservicebus.Error with Code == CodeTimeout — we map
	// that to errNoSession so the consume loop treats it as an empty
	// poll, not a failure.
	r, err := a.client.AcceptNextSessionForQueue(actx, a.queueName, nil)
	if err != nil {
		var sbErr *azservicebus.Error
		if errors.As(err, &sbErr) && sbErr.Code == azservicebus.CodeTimeout {
			return nil, errNoSession
		}
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, errNoSession
		}
		return nil, err
	}
	return &azServiceBusSession{receiver: r}, nil
}

func (a *azServiceBusAdapter) Close(ctx context.Context) error {
	if a.sender != nil {
		_ = a.sender.Close(ctx)
	}
	if a.client != nil {
		return a.client.Close(ctx)
	}
	return nil
}

// azServiceBusSession wraps one SDK *SessionReceiver. It keeps a
// sequence-number -> *ReceivedMessage map so Complete can resolve the
// SDK handle (the WAL Commit contract only gives us the StreamPos
// offset). All settlement happens on this session's link, which is the
// only correctness-safe place to ack a session message.
type azServiceBusSession struct {
	receiver *azservicebus.SessionReceiver

	mu      sync.Mutex
	pending map[int64]*azservicebus.ReceivedMessage
}

func (a *azServiceBusSession) SessionID() string {
	return a.receiver.SessionID()
}

func (a *azServiceBusSession) Receive(ctx context.Context, maxMessages int, wait time.Duration) ([]serviceBusMessage, error) {
	rctx := ctx
	if wait > 0 {
		var cancel context.CancelFunc
		rctx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	msgs, err := a.receiver.ReceiveMessages(rctx, maxMessages, nil)
	if err != nil {
		// A receive that times out with no messages is a normal empty
		// poll (session drained for now), not an error.
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, nil
		}
		var sbErr *azservicebus.Error
		if errors.As(err, &sbErr) && sbErr.Code == azservicebus.CodeTimeout {
			return nil, nil
		}
		return nil, err
	}
	out := make([]serviceBusMessage, 0, len(msgs))
	for _, m := range msgs {
		var seq int64
		if m.SequenceNumber != nil {
			seq = *m.SequenceNumber
		}
		var enq int64
		if m.EnqueuedTime != nil {
			enq = m.EnqueuedTime.UnixMilli()
		} else {
			enq = time.Now().UnixMilli()
		}
		props := map[string]string{}
		for k, v := range m.ApplicationProperties {
			props[k] = fmt.Sprintf("%v", v)
		}
		sess := ""
		if m.SessionID != nil {
			sess = *m.SessionID
		}
		a.mu.Lock()
		if a.pending == nil {
			a.pending = make(map[int64]*azservicebus.ReceivedMessage)
		}
		a.pending[seq] = m
		a.mu.Unlock()
		out = append(out, serviceBusMessage{
			Body:           m.Body,
			Properties:     props,
			SessionID:      sess,
			SequenceNumber: seq,
			EnqueuedMs:     enq,
		})
	}
	return out, nil
}

func (a *azServiceBusSession) Complete(ctx context.Context, sequenceNumber int64) error {
	a.mu.Lock()
	msg, ok := a.pending[sequenceNumber]
	if ok {
		delete(a.pending, sequenceNumber)
	}
	a.mu.Unlock()
	if !ok {
		// Already completed or unknown — no-op (parity with the
		// servicebus.py "no pending message" warning path).
		return nil
	}
	return a.receiver.CompleteMessage(ctx, msg, nil)
}

func (a *azServiceBusSession) Pending() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

func (a *azServiceBusSession) Close(ctx context.Context) error {
	if a.receiver != nil {
		return a.receiver.Close(ctx)
	}
	return nil
}
