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
//                            is redelivered after the lock expires.
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
// Testing: ServiceBusAPI is the seam. A thin adapter wraps the SDK
// client/sender/receiver; unit tests inject a fake — no live Azure.

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
// needs. The adapter in defaultNewClient wraps the SDK; tests fake it.
type ServiceBusAPI interface {
	// Send sends one message to the queue with the given session id +
	// application properties.
	Send(ctx context.Context, sessionID string, body []byte, props map[string]string) error
	// Receive receives up to maxMessages, waiting at most wait.
	Receive(ctx context.Context, maxMessages int, wait time.Duration) ([]serviceBusMessage, error)
	// Complete acks (completes) a previously-received message by its
	// sequence number.
	Complete(ctx context.Context, sequenceNumber int64) error
	// Close releases the client.
	Close(ctx context.Context) error
}

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
}

// DefaultServiceBusConfig returns a config matching the Python defaults.
func DefaultServiceBusConfig(connStr, queueName string) ServiceBusConfig {
	return ServiceBusConfig{
		ConnectionString: connStr,
		QueueName:        queueName,
		MaxWaitTime:      5 * time.Second,
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
}

// NewServiceBus constructs a Service Bus WAL backend.
func NewServiceBus(cfg ServiceBusConfig) *ServiceBus {
	if cfg.MaxWaitTime <= 0 {
		cfg.MaxWaitTime = 5 * time.Second
	}
	s := &ServiceBus{
		config: cfg,
		idemp:  make(map[string]map[string]map[string]StreamPos),
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
	receiver, err := client.NewReceiverForQueue(s.config.QueueName, nil)
	if err != nil {
		_ = sender.Close(ctx)
		_ = client.Close(ctx)
		return nil, err
	}
	return &azServiceBusAdapter{
		client:   client,
		sender:   sender,
		receiver: receiver,
		// Pending received messages keyed by sequence number so
		// Complete can resolve the SDK *ReceivedMessage handle.
		pending: make(map[int64]*azservicebus.ReceivedMessage),
	}, nil
}

// Connect builds the client (sender + receiver). Service Bus has no
// cheap describe; connectivity surfaces on first Send/Receive (parity
// with servicebus.py, which only created the client here).
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

// Close releases the client. Safe to call multiple times.
func (s *ServiceBus) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// PollBatch receives up to maxRecords, blocking up to timeout.
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

	wait := timeout
	if wait > s.config.MaxWaitTime {
		wait = s.config.MaxWaitTime
	}
	msgs, err := api.Receive(ctx, maxRecords, wait)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, classifyServiceBusErr("servicebus receive", err)
	}
	out := make([]Record, 0, len(msgs))
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
	return out, nil
}

// Subscribe streams records by repeatedly receiving. Auto-completes
// each delivered record, matching the InMemory/Kafka Subscribe
// contract.
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

// Commit completes (acks) the message by sequence number. Mirrors
// servicebus.py commit -> complete_message.
func (s *ServiceBus) Commit(ctx context.Context, groupID string, record Record) error {
	s.mu.Lock()
	if !s.connected || s.api == nil {
		s.mu.Unlock()
		return fmt.Errorf("%w: not connected", ErrConnection)
	}
	api := s.api
	s.mu.Unlock()
	if err := api.Complete(ctx, record.Position.Offset); err != nil {
		return classifyServiceBusErr("servicebus complete", err)
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

// azServiceBusAdapter adapts the SDK client/sender/receiver to
// ServiceBusAPI. It keeps a sequence-number -> *ReceivedMessage map so
// Complete can resolve the SDK handle (the WAL Commit contract only
// gives us the StreamPos offset).
type azServiceBusAdapter struct {
	client   *azservicebus.Client
	sender   *azservicebus.Sender
	receiver *azservicebus.Receiver

	mu      sync.Mutex
	pending map[int64]*azservicebus.ReceivedMessage
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

func (a *azServiceBusAdapter) Receive(ctx context.Context, maxMessages int, wait time.Duration) ([]serviceBusMessage, error) {
	rctx := ctx
	if wait > 0 {
		var cancel context.CancelFunc
		rctx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	msgs, err := a.receiver.ReceiveMessages(rctx, maxMessages, nil)
	if err != nil {
		// A receive that times out with no messages is a normal empty
		// poll, not an error.
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
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

func (a *azServiceBusAdapter) Complete(ctx context.Context, sequenceNumber int64) error {
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

func (a *azServiceBusAdapter) Close(ctx context.Context) error {
	if a.receiver != nil {
		_ = a.receiver.Close(ctx)
	}
	if a.sender != nil {
		_ = a.sender.Close(ctx)
	}
	if a.client != nil {
		return a.client.Close(ctx)
	}
	return nil
}
