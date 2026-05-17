// SPDX-License-Identifier: AGPL-3.0-only

package wal

// Azure Event Hubs WAL backend for the Go server. Ported from the
// retired Python source (server/python/entdb_server/wal/eventhubs.py).
//
// Concept mapping (mirrors the Python docstring):
//
//   - Event Hub        -> WAL topic.
//   - partition_key    -> partition key (tenant_id). Event Hubs hashes
//                          the partition key to a physical partition;
//                          all events for a key land on one partition,
//                          which is ordered — that is the per-tenant
//                          total-order guarantee (same model as Kafka's
//                          key->partition hash).
//   - Consumer group   -> consumer group (groupID at the call site maps
//                          to the Event Hubs consumer group configured
//                          on the client).
//   - sequence_number  -> offset. Event Hubs assigns a per-partition
//                          monotone int64; we surface it as
//                          StreamPos.Offset and the physical partition
//                          id as StreamPos.Partition so per-partition
//                          order is observable.
//
// Checkpointing: eventhubs.py kept an in-memory checkpoint and noted
// that production should persist it. We mirror that: Commit records the
// per-partition sequence number in memory so a re-Subscribe resumes
// after it. Durable checkpoint stores (blob) are an operational add-on,
// out of scope for #518 (the WAL contract only requires at-least-once +
// in-order-per-key, which this provides).
//
// Halt-on-poison: event bodies pass through verbatim; a malformed event
// surfaces at the applier (CLAUDE.md: WAL is the source of truth).
//
// Testing: EventHubsAPI is the seam. A thin adapter wraps the SDK
// producer/consumer; unit tests inject a fake — no live Azure.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
)

// EventHubsAPI is the slice of Event Hubs operations the WAL backend
// needs. The adapter in defaultNewClient wraps the SDK; tests fake it.
type EventHubsAPI interface {
	// Send sends one event with the given partition key + properties.
	Send(ctx context.Context, partitionKey string, body []byte, props map[string]string) error
	// Partitions lists the physical partition ids.
	Partitions(ctx context.Context) ([]string, error)
	// Receive receives up to count events from partitionID starting
	// after afterSeq (-1 = from the earliest available), waiting at
	// most wait.
	Receive(ctx context.Context, partitionID string, afterSeq int64, count int, wait time.Duration) ([]eventHubEvent, error)
	// Close releases the clients.
	Close(ctx context.Context) error
}

// eventHubEvent is the transport-neutral shape Receive returns.
type eventHubEvent struct {
	Body           []byte
	Properties     map[string]string
	PartitionKey   string
	PartitionID    string
	SequenceNumber int64
	EnqueuedMs     int64
}

// EventHubsConfig captures the Event Hubs-specific knobs (parity with
// the Python EventHubsConfig).
type EventHubsConfig struct {
	// ConnectionString is the Event Hubs namespace connection string.
	ConnectionString string
	// EventHubName is the hub backing the WAL.
	EventHubName string
	// ConsumerGroup is the Event Hubs consumer group (default "$Default").
	ConsumerGroup string
	// MaxBatchSize bounds a single receive. Python default 100.
	MaxBatchSize int
	// MaxWaitTime bounds a single receive call. Python default 5s.
	MaxWaitTime time.Duration
}

// DefaultEventHubsConfig returns a config matching the Python defaults.
func DefaultEventHubsConfig(connStr, hubName, consumerGroup string) EventHubsConfig {
	if strings.TrimSpace(consumerGroup) == "" {
		consumerGroup = azeventhubs.DefaultConsumerGroup
	}
	return EventHubsConfig{
		ConnectionString: connStr,
		EventHubName:     hubName,
		ConsumerGroup:    consumerGroup,
		MaxBatchSize:     100,
		MaxWaitTime:      5 * time.Second,
	}
}

// EventHubs implements Producer + Consumer against Azure Event Hubs.
type EventHubs struct {
	config EventHubsConfig

	newClient func(ctx context.Context) (EventHubsAPI, error)

	mu        sync.Mutex
	connected bool
	api       EventHubsAPI

	// checkpoints[partitionID] -> last committed sequence number.
	checkpoints map[string]int64

	idemp map[string]map[string]map[string]StreamPos
}

// NewEventHubs constructs an Event Hubs WAL backend.
func NewEventHubs(cfg EventHubsConfig) *EventHubs {
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = 100
	}
	if cfg.MaxWaitTime <= 0 {
		cfg.MaxWaitTime = 5 * time.Second
	}
	if strings.TrimSpace(cfg.ConsumerGroup) == "" {
		cfg.ConsumerGroup = azeventhubs.DefaultConsumerGroup
	}
	e := &EventHubs{
		config:      cfg,
		checkpoints: make(map[string]int64),
		idemp:       make(map[string]map[string]map[string]StreamPos),
	}
	e.newClient = e.defaultNewClient
	return e
}

func (e *EventHubs) defaultNewClient(ctx context.Context) (EventHubsAPI, error) {
	producer, err := azeventhubs.NewProducerClientFromConnectionString(
		e.config.ConnectionString, e.config.EventHubName, nil)
	if err != nil {
		return nil, err
	}
	consumer, err := azeventhubs.NewConsumerClientFromConnectionString(
		e.config.ConnectionString, e.config.EventHubName, e.config.ConsumerGroup, nil)
	if err != nil {
		_ = producer.Close(ctx)
		return nil, err
	}
	return &azEventHubsAdapter{
		producer:   producer,
		consumer:   consumer,
		partitions: make(map[string]*azeventhubs.PartitionClient),
	}, nil
}

// Connect builds the producer + consumer clients. Connectivity surfaces
// on first Send/Receive (parity with eventhubs.py).
func (e *EventHubs) Connect(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.connected {
		return nil
	}
	if strings.TrimSpace(e.config.ConnectionString) == "" || strings.TrimSpace(e.config.EventHubName) == "" {
		return fmt.Errorf("%w: eventhubs: connection string and hub name required", ErrConnection)
	}
	api, err := e.newClient(ctx)
	if err != nil {
		return fmt.Errorf("%w: eventhubs client: %v", ErrConnection, err)
	}
	e.api = api
	e.connected = true
	return nil
}

// Close releases the clients. Safe to call multiple times.
func (e *EventHubs) Close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.api != nil {
		_ = e.api.Close(ctx)
		e.api = nil
	}
	e.connected = false
	return nil
}

// Append sends value with partition key = tenant key for per-tenant
// order. See Producer.Append.
func (e *EventHubs) Append(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string][]byte,
) (StreamPos, error) {
	if err := ctx.Err(); err != nil {
		return StreamPos{}, err
	}
	e.mu.Lock()
	if !e.connected || e.api == nil {
		e.mu.Unlock()
		return StreamPos{}, fmt.Errorf("%w: not connected", ErrConnection)
	}
	idempKey := idempotencyKey(headers)
	if idempKey != "" {
		if pos, ok := lookupIdemp(e.idemp, topic, key, idempKey); ok {
			e.mu.Unlock()
			return pos, nil
		}
	}
	api := e.api
	e.mu.Unlock()

	if err := api.Send(ctx, key, value, stringHeaders(headers)); err != nil {
		return StreamPos{}, classifyEventHubsErr("eventhubs send", err)
	}

	nowMs := time.Now().UnixMilli()
	pos := StreamPos{
		Topic:       topic,
		Partition:   0,
		Offset:      nowMs,
		TimestampMs: nowMs,
	}
	if idempKey != "" {
		e.mu.Lock()
		storeIdemp(e.idemp, topic, key, idempKey, pos)
		e.mu.Unlock()
	}
	return pos, nil
}

// PollBatch receives up to maxRecords across all partitions, blocking
// up to timeout. Each partition resumes after its committed
// checkpoint.
func (e *EventHubs) PollBatch(
	ctx context.Context,
	topic, groupID string,
	maxRecords int,
	timeout time.Duration,
) ([]Record, error) {
	if maxRecords <= 0 {
		return nil, nil
	}
	e.mu.Lock()
	if !e.connected || e.api == nil {
		e.mu.Unlock()
		return nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	api := e.api
	e.mu.Unlock()

	parts, err := api.Partitions(ctx)
	if err != nil {
		return nil, classifyEventHubsErr("eventhubs partitions", err)
	}
	sort.Strings(parts)

	deadline := time.Now().Add(timeout)
	out := make([]Record, 0, maxRecords)
	for _, pid := range parts {
		if len(out) >= maxRecords {
			break
		}
		e.mu.Lock()
		after, seen := e.checkpoints[pid]
		e.mu.Unlock()
		if !seen {
			after = -1 // earliest available
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := remaining
		if wait > e.config.MaxWaitTime {
			wait = e.config.MaxWaitTime
		}
		events, err := api.Receive(ctx, pid, after, maxRecords-len(out), wait)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if len(out) > 0 {
					return out, nil
				}
				return nil, err
			}
			return nil, classifyEventHubsErr("eventhubs receive", err)
		}
		for _, ev := range events {
			out = append(out, Record{
				Key:   ev.PartitionKey,
				Value: ev.Body,
				Position: StreamPos{
					Topic:       topic,
					Partition:   partitionIDToInt32(ev.PartitionID),
					Offset:      ev.SequenceNumber,
					TimestampMs: ev.EnqueuedMs,
				},
				Headers: bytesHeaders(ev.Properties),
			})
		}
	}
	return out, nil
}

// Subscribe streams records by repeatedly polling all partitions.
// Auto-commits each delivered record, matching the InMemory/Kafka
// Subscribe contract.
func (e *EventHubs) Subscribe(
	ctx context.Context,
	topic, groupID string,
) (<-chan Record, <-chan error, error) {
	e.mu.Lock()
	if !e.connected || e.api == nil {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	e.mu.Unlock()

	out := make(chan Record)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		for {
			if ctx.Err() != nil {
				return
			}
			recs, err := e.PollBatch(ctx, topic, groupID, e.config.MaxBatchSize, e.config.MaxWaitTime)
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
					_ = e.Commit(ctx, groupID, r)
				}
			}
		}
	}()
	return out, errCh, nil
}

// Commit advances the in-memory per-partition checkpoint so a
// subsequent PollBatch / Subscribe resumes after this sequence number.
// Mirrors eventhubs.py commit (in-memory checkpoint).
func (e *EventHubs) Commit(ctx context.Context, groupID string, record Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	pid := int32ToPartitionID(record.Position.Partition)
	if cur, ok := e.checkpoints[pid]; !ok || record.Position.Offset > cur {
		e.checkpoints[pid] = record.Position.Offset
	}
	return nil
}

func classifyEventHubsErr(op string, err error) error {
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

// partitionIDToInt32 parses an Event Hubs partition id ("0", "1", ...)
// into the StreamPos.Partition int32. Non-numeric ids hash to 0.
func partitionIDToInt32(pid string) int32 {
	if pid == "" {
		return 0
	}
	n := 0
	for _, r := range pid {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return int32(n)
}

func int32ToPartitionID(p int32) string {
	return fmt.Sprintf("%d", p)
}

// azEventHubsAdapter adapts the SDK producer/consumer to EventHubsAPI.
// PartitionClients are created lazily per partition and recreated when
// the resume sequence number changes (the SDK binds the start position
// at client construction).
type azEventHubsAdapter struct {
	producer *azeventhubs.ProducerClient
	consumer *azeventhubs.ConsumerClient

	mu         sync.Mutex
	partitions map[string]*azeventhubs.PartitionClient
	startSeq   map[string]int64
}

func (a *azEventHubsAdapter) Send(ctx context.Context, partitionKey string, body []byte, props map[string]string) error {
	opts := &azeventhubs.EventDataBatchOptions{}
	if partitionKey != "" {
		pk := partitionKey
		opts.PartitionKey = &pk
	}
	batch, err := a.producer.NewEventDataBatch(ctx, opts)
	if err != nil {
		return err
	}
	ed := &azeventhubs.EventData{Body: body}
	if len(props) > 0 {
		p := make(map[string]any, len(props))
		for k, v := range props {
			p[k] = v
		}
		ed.Properties = p
	}
	if err := batch.AddEventData(ed, nil); err != nil {
		return err
	}
	return a.producer.SendEventDataBatch(ctx, batch, nil)
}

func (a *azEventHubsAdapter) Partitions(ctx context.Context) ([]string, error) {
	props, err := a.producer.GetEventHubProperties(ctx, nil)
	if err != nil {
		return nil, err
	}
	return props.PartitionIDs, nil
}

func (a *azEventHubsAdapter) partitionClient(partitionID string, afterSeq int64) (*azeventhubs.PartitionClient, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.startSeq == nil {
		a.startSeq = make(map[string]int64)
	}
	if pc, ok := a.partitions[partitionID]; ok && a.startSeq[partitionID] == afterSeq {
		return pc, nil
	}
	if pc, ok := a.partitions[partitionID]; ok {
		_ = pc.Close(context.Background())
		delete(a.partitions, partitionID)
	}
	pos := azeventhubs.StartPosition{}
	if afterSeq < 0 {
		pos.Earliest = to(true)
	} else {
		pos.SequenceNumber = &afterSeq
		pos.Inclusive = false
	}
	pc, err := a.consumer.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
		StartPosition: pos,
	})
	if err != nil {
		return nil, err
	}
	a.partitions[partitionID] = pc
	a.startSeq[partitionID] = afterSeq
	return pc, nil
}

func (a *azEventHubsAdapter) Receive(ctx context.Context, partitionID string, afterSeq int64, count int, wait time.Duration) ([]eventHubEvent, error) {
	pc, err := a.partitionClient(partitionID, afterSeq)
	if err != nil {
		return nil, err
	}
	rctx := ctx
	if wait > 0 {
		var cancel context.CancelFunc
		rctx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	events, err := pc.ReceiveEvents(rctx, count, nil)
	if err != nil {
		// A receive that times out with no events is a normal empty
		// poll, not an error.
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, nil
		}
		return nil, err
	}
	out := make([]eventHubEvent, 0, len(events))
	for _, ev := range events {
		var enq int64
		if ev.EnqueuedTime != nil {
			enq = ev.EnqueuedTime.UnixMilli()
		} else {
			enq = time.Now().UnixMilli()
		}
		props := map[string]string{}
		for k, v := range ev.Properties {
			props[k] = fmt.Sprintf("%v", v)
		}
		pk := ""
		if ev.PartitionKey != nil {
			pk = *ev.PartitionKey
		}
		out = append(out, eventHubEvent{
			Body:           ev.Body,
			Properties:     props,
			PartitionKey:   pk,
			PartitionID:    partitionID,
			SequenceNumber: ev.SequenceNumber,
			EnqueuedMs:     enq,
		})
	}
	return out, nil
}

func (a *azEventHubsAdapter) Close(ctx context.Context) error {
	a.mu.Lock()
	for id, pc := range a.partitions {
		_ = pc.Close(ctx)
		delete(a.partitions, id)
	}
	a.mu.Unlock()
	if a.consumer != nil {
		_ = a.consumer.Close(ctx)
	}
	if a.producer != nil {
		return a.producer.Close(ctx)
	}
	return nil
}

func to[T any](v T) *T { return &v }
