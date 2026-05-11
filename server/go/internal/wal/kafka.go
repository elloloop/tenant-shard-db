// SPDX-License-Identifier: AGPL-3.0-only

package wal

// Kafka/Redpanda WAL backend for the Go server.
//
// Ported from server/python/entdb_server/wal/kafka.py with 1:1 parity on
// the durability-critical knobs:
//
//   - Producer:  acks=all, enable.idempotence=true,
//                max.in.flight.requests.per.connection=5,
//                linger.ms=5, request.timeout.ms=30000.
//   - Consumer:  auto.offset.reset=earliest, enable.auto.commit=false
//                (manual commit via OffsetCommit per applied record),
//                session.timeout.ms=30000, heartbeat.interval.ms=10000.
//   - Topic:     single, name comes from --wal-topic (default "entdb-wal").
//   - Partition: keyed by tenant_id (the Append `key` arg). franz-go's
//                default partitioner hashes the key, giving per-tenant
//                total order while spreading load across partitions.
//   - Headers:   pass-through, including HeaderIdempotencyKey.
//
// Halt-on-poison: a malformed record surfaces as a Subscribe error /
// PollBatch error and the applier supervisor decides whether to halt.
// We do NOT silently skip undecodable records (CLAUDE.md invariant #1
// — the WAL is the source of truth).
//
// Idempotent retry: like InMemory, we cache (topic, key, idempotency-key)
// -> StreamPos so a retried Append within the lifetime of the producer
// returns the original receipt without writing a duplicate record. This
// mirrors memory.go and the Python applier's apply-time dedupe; the
// Kafka transactional/idempotent producer config also prevents broker-
// side duplicates on retry within a single producer session.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// KafkaConfig mirrors server/python/entdb_server/config.py KafkaConfig.
// Only the fields the Go server actually consumes today are present;
// adding SASL/SSL is a follow-up that should match Python's env-var
// names (KAFKA_SASL_MECHANISM etc.) when wired up.
type KafkaConfig struct {
	// Brokers is the comma-separated bootstrap list, e.g. "redpanda:9092".
	Brokers []string
	// ClientID is the franz-go client id; appears in broker logs and
	// metrics. Defaults to "entdb-server-go" when empty.
	ClientID string
	// Acks is the producer ack level. We default to "all" (RequireAllISRAcks)
	// to match Python's default. Any non-"all" value falls back to leader-
	// only acks (matches the Python `int(acks)` path for "1"/"0").
	Acks string
	// EnableIdempotence enables the broker-side idempotent producer
	// (prevents duplicates on retry within a producer session).
	EnableIdempotence bool
	// MaxInFlight is the producer's max in-flight requests per
	// connection. Python defaults to 5; franz-go also caps at 5 when
	// idempotence is on.
	MaxInFlight int
	// LingerMs is the batch-collection window for the producer.
	LingerMs int
	// RequestTimeoutMs bounds a single produce request.
	RequestTimeoutMs int
	// AutoOffsetReset controls where a new consumer group starts.
	// "earliest" (default, matches Python) or "latest".
	AutoOffsetReset string
	// EnableAutoCommit toggles the broker-side auto-commit loop.
	// Python defaults this to FALSE (manual commit per record); the Go
	// backend MUST match so re-delivery semantics align.
	EnableAutoCommit bool
	// SessionTimeoutMs is the consumer-group session timeout.
	SessionTimeoutMs int
	// HeartbeatIntervalMs is how often the consumer heartbeats to the
	// group coordinator. Must be < SessionTimeoutMs / 3.
	HeartbeatIntervalMs int
}

// DefaultKafkaConfig returns a config that matches Python's defaults
// (config.py:142-204).
func DefaultKafkaConfig(brokers []string) KafkaConfig {
	return KafkaConfig{
		Brokers:             brokers,
		ClientID:            "entdb-server-go",
		Acks:                "all",
		EnableIdempotence:   true,
		MaxInFlight:         5,
		LingerMs:            5,
		RequestTimeoutMs:    30000,
		AutoOffsetReset:     "earliest",
		EnableAutoCommit:    false,
		SessionTimeoutMs:    30000,
		HeartbeatIntervalMs: 10000,
	}
}

// Kafka implements Producer + Consumer against any Kafka API-compatible
// broker (Apache Kafka, Redpanda, MSK, …).
//
// Implementation notes:
//
//   - We keep one franz-go client for production and a separate one
//     per (topic, groupID) consumer pair. franz-go can do both with one
//     client but the lifecycle gets noisy when Connect/Close need to
//     drop only the consumer half on a topic switch.
//   - The producer client is lazily created in Connect; consumer
//     clients are lazily created on the first PollBatch / Subscribe.
//   - We hold mu for the duration of Append's idempotency-cache lookup
//     and franz-go produce-future setup, then release it before
//     blocking on the future (so concurrent appends can pipeline).
type Kafka struct {
	config KafkaConfig

	mu        sync.Mutex
	connected bool

	// producer holds the franz-go client used for Append. nil until
	// Connect is called.
	producer *kgo.Client

	// consumerByGroup caches one client per (topic, groupID). We
	// instantiate on demand because franz-go binds the group + topic
	// to the client at construction time.
	consumerByGroup map[consumerKey]*kgo.Client

	// idemp[topic][key][idempotencyKey] -> previously-issued StreamPos.
	// Mirrors InMemory's idempotency cache; lets retried Appends within
	// the same producer session collapse to the original receipt
	// without writing a duplicate record.
	idemp map[string]map[string]map[string]StreamPos
}

type consumerKey struct {
	topic   string
	groupID string
}

// NewKafka constructs a Kafka WAL backend. Connect must be called
// before any Append / Subscribe / PollBatch.
func NewKafka(cfg KafkaConfig) *Kafka {
	if cfg.ClientID == "" {
		cfg.ClientID = "entdb-server-go"
	}
	if cfg.Acks == "" {
		cfg.Acks = "all"
	}
	if cfg.MaxInFlight == 0 {
		cfg.MaxInFlight = 5
	}
	if cfg.LingerMs == 0 {
		cfg.LingerMs = 5
	}
	if cfg.RequestTimeoutMs == 0 {
		cfg.RequestTimeoutMs = 30000
	}
	if cfg.AutoOffsetReset == "" {
		cfg.AutoOffsetReset = "earliest"
	}
	if cfg.SessionTimeoutMs == 0 {
		cfg.SessionTimeoutMs = 30000
	}
	if cfg.HeartbeatIntervalMs == 0 {
		cfg.HeartbeatIntervalMs = 10000
	}
	return &Kafka{
		config:          cfg,
		consumerByGroup: make(map[consumerKey]*kgo.Client),
		idemp:           make(map[string]map[string]map[string]StreamPos),
	}
}

// Connect opens the producer connection.
func (k *Kafka) Connect(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.connected {
		return nil
	}
	if len(k.config.Brokers) == 0 {
		return fmt.Errorf("%w: no brokers configured", ErrConnection)
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.config.Brokers...),
		kgo.ClientID(k.config.ClientID),
		kgo.ProducerLinger(time.Duration(k.config.LingerMs) * time.Millisecond),
		kgo.ProduceRequestTimeout(time.Duration(k.config.RequestTimeoutMs) * time.Millisecond),
		// Mirror aiokafka's default behaviour: the broker auto-creates
		// topics on first produce (Redpanda + Kafka both have this on
		// by default). franz-go disables auto-create on the client
		// side unless we ask for it.
		kgo.AllowAutoTopicCreation(),
	}
	switch k.config.Acks {
	case "all", "-1":
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
	case "0":
		opts = append(opts, kgo.RequiredAcks(kgo.NoAck()))
	default:
		// "1" or anything else: leader-only ack. Mirrors Python's
		// int(acks) path for non-"all" values.
		opts = append(opts, kgo.RequiredAcks(kgo.LeaderAck()))
	}
	// franz-go pins max-in-flight to 1 internally when idempotence is
	// enabled (to keep retries from reordering); explicitly setting
	// MaxProduceRequestsInflightPerBroker conflicts with that, so we
	// only apply it when idempotence is off.
	if !k.config.EnableIdempotence {
		opts = append(opts, kgo.DisableIdempotentWrite())
		opts = append(opts, kgo.MaxProduceRequestsInflightPerBroker(k.config.MaxInFlight))
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("%w: kafka new client: %v", ErrConnection, err)
	}
	// Ping with a short timeout so unreachable brokers fail-fast
	// instead of silently buffering the first Append.
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := cl.Ping(pingCtx); err != nil {
		cl.Close()
		return fmt.Errorf("%w: kafka ping: %v", ErrConnection, err)
	}

	k.producer = cl
	k.connected = true
	return nil
}

// Close shuts down all clients (producer + cached consumers). Safe to
// call multiple times.
func (k *Kafka) Close(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.producer != nil {
		k.producer.Close()
		k.producer = nil
	}
	for ck, cl := range k.consumerByGroup {
		cl.Close()
		delete(k.consumerByGroup, ck)
	}
	k.connected = false
	return nil
}

// Append produces value to topic, partitioned by key. Returns the
// broker-assigned StreamPos. See Producer.Append for the contract.
func (k *Kafka) Append(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string][]byte,
) (StreamPos, error) {
	k.mu.Lock()
	if !k.connected || k.producer == nil {
		k.mu.Unlock()
		return StreamPos{}, fmt.Errorf("%w: not connected", ErrConnection)
	}
	// Idempotency cache. Mirrors memory.go's behaviour: a retry with
	// the same (topic, key, idempotency-key) tuple within this
	// producer session returns the original StreamPos without
	// re-producing. The franz-go idempotent producer covers
	// broker-side dedupe on transport-level retries; this is
	// application-level dedupe for caller-driven retries.
	idempKey := ""
	if h, ok := headers[HeaderIdempotencyKey]; ok && len(h) > 0 {
		idempKey = string(h)
	}
	if idempKey != "" {
		if byTopic, ok := k.idemp[topic]; ok {
			if byKey, ok := byTopic[key]; ok {
				if pos, ok := byKey[idempKey]; ok {
					k.mu.Unlock()
					return pos, nil
				}
			}
		}
	}
	producer := k.producer
	k.mu.Unlock()

	rec := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}
	if len(headers) > 0 {
		rec.Headers = make([]kgo.RecordHeader, 0, len(headers))
		for hk, hv := range headers {
			cp := append([]byte(nil), hv...)
			rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: hk, Value: cp})
		}
	}

	// ProduceSync waits for broker ack (acks=all when configured).
	res := producer.ProduceSync(ctx, rec)
	produced, err := res.First()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return StreamPos{}, err
		}
		// Heuristic: timeout errors -> ErrTimeout, everything else
		// rolls up under ErrWal. Connection-loss errors from franz-go
		// surface as kerr.* but we treat them as ErrWal too — the
		// next Append will try to reconnect transparently because the
		// underlying client is long-lived.
		if isTimeout(err) {
			return StreamPos{}, fmt.Errorf("%w: kafka append: %v", ErrTimeout, err)
		}
		return StreamPos{}, fmt.Errorf("%w: kafka append: %v", ErrWal, err)
	}

	pos := StreamPos{
		Topic:       produced.Topic,
		Partition:   produced.Partition,
		Offset:      produced.Offset,
		TimestampMs: produced.Timestamp.UnixMilli(),
	}
	if pos.TimestampMs == 0 {
		pos.TimestampMs = time.Now().UnixMilli()
	}

	if idempKey != "" {
		k.mu.Lock()
		byTopic, ok := k.idemp[topic]
		if !ok {
			byTopic = make(map[string]map[string]StreamPos)
			k.idemp[topic] = byTopic
		}
		byKey, ok := byTopic[key]
		if !ok {
			byKey = make(map[string]StreamPos)
			byTopic[key] = byKey
		}
		byKey[idempKey] = pos
		k.mu.Unlock()
	}

	return pos, nil
}

// consumerFor returns a franz-go client bound to (topic, groupID),
// lazily creating it on first access. Caller must hold k.mu.
func (k *Kafka) consumerForLocked(topic, groupID string) (*kgo.Client, error) {
	ck := consumerKey{topic: topic, groupID: groupID}
	if cl, ok := k.consumerByGroup[ck]; ok {
		return cl, nil
	}

	autoReset := kgo.NewOffset().AtStart()
	if k.config.AutoOffsetReset == "latest" {
		autoReset = kgo.NewOffset().AtEnd()
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(k.config.Brokers...),
		kgo.ClientID(k.config.ClientID + "-consumer"),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(autoReset),
		kgo.SessionTimeout(time.Duration(k.config.SessionTimeoutMs) * time.Millisecond),
		kgo.HeartbeatInterval(time.Duration(k.config.HeartbeatIntervalMs) * time.Millisecond),
		// Manual commit. Python sets enable_auto_commit=False; we
		// match by leaving auto-commit off entirely (the AutoCommit-
		// related opts are simply not set).
		kgo.DisableAutoCommit(),
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: kafka new consumer: %v", ErrConnection, err)
	}
	k.consumerByGroup[ck] = cl
	return cl, nil
}

// PollBatch fetches up to maxRecords from (topic, groupID), blocking
// up to timeout for records to arrive. See Consumer.PollBatch.
func (k *Kafka) PollBatch(
	ctx context.Context,
	topic, groupID string,
	maxRecords int,
	timeout time.Duration,
) ([]Record, error) {
	if maxRecords <= 0 {
		return nil, nil
	}
	k.mu.Lock()
	if !k.connected {
		k.mu.Unlock()
		return nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	cl, err := k.consumerForLocked(topic, groupID)
	if err != nil {
		k.mu.Unlock()
		return nil, err
	}
	k.mu.Unlock()

	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fetches := cl.PollRecords(pollCtx, maxRecords)
	if errs := fetches.Errors(); len(errs) > 0 {
		// Surface only fatal errors. EOF / context.DeadlineExceeded
		// on the consumer is "no records yet"; do not turn that into
		// an error (Python's getmany() returns empty for the same
		// case).
		for _, fe := range errs {
			if errors.Is(fe.Err, context.DeadlineExceeded) || errors.Is(fe.Err, context.Canceled) {
				continue
			}
			return nil, fmt.Errorf("%w: kafka poll: %v", ErrWal, fe.Err)
		}
	}

	out := make([]Record, 0, fetches.NumRecords())
	fetches.EachRecord(func(r *kgo.Record) {
		headers := map[string][]byte{}
		for _, h := range r.Headers {
			cp := append([]byte(nil), h.Value...)
			headers[h.Key] = cp
		}
		out = append(out, Record{
			Key:   string(r.Key),
			Value: append([]byte(nil), r.Value...),
			Position: StreamPos{
				Topic:       r.Topic,
				Partition:   r.Partition,
				Offset:      r.Offset,
				TimestampMs: r.Timestamp.UnixMilli(),
			},
			Headers: headers,
		})
	})
	return out, nil
}

// Subscribe streams records continuously via background polling.
// Equivalent to InMemory.Subscribe but uses the Kafka consumer group
// underneath; offsets are committed after each delivered record so
// crash + restart re-delivers only what wasn't ack'd.
func (k *Kafka) Subscribe(
	ctx context.Context,
	topic, groupID string,
) (<-chan Record, <-chan error, error) {
	k.mu.Lock()
	if !k.connected {
		k.mu.Unlock()
		return nil, nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	cl, err := k.consumerForLocked(topic, groupID)
	if err != nil {
		k.mu.Unlock()
		return nil, nil, err
	}
	k.mu.Unlock()

	out := make(chan Record)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		for {
			if err := ctx.Err(); err != nil {
				return
			}
			pollCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			fetches := cl.PollRecords(pollCtx, 32)
			cancel()
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, fe := range errs {
					if errors.Is(fe.Err, context.DeadlineExceeded) || errors.Is(fe.Err, context.Canceled) {
						continue
					}
					if ctx.Err() == nil {
						errCh <- fmt.Errorf("%w: kafka subscribe: %v", ErrWal, fe.Err)
					}
					return
				}
			}
			var sendErr error
			fetches.EachRecord(func(r *kgo.Record) {
				if sendErr != nil {
					return
				}
				headers := map[string][]byte{}
				for _, h := range r.Headers {
					cp := append([]byte(nil), h.Value...)
					headers[h.Key] = cp
				}
				rec := Record{
					Key:   string(r.Key),
					Value: append([]byte(nil), r.Value...),
					Position: StreamPos{
						Topic:       r.Topic,
						Partition:   r.Partition,
						Offset:      r.Offset,
						TimestampMs: r.Timestamp.UnixMilli(),
					},
					Headers: headers,
				}
				select {
				case <-ctx.Done():
					sendErr = ctx.Err()
					return
				case out <- rec:
					// Auto-commit per record on the Subscribe path
					// (parallel with InMemory.Subscribe). Callers
					// wanting manual commits should use PollBatch +
					// Commit directly.
					cl.MarkCommitRecords(r)
					_ = cl.CommitMarkedOffsets(ctx)
				}
			})
			if sendErr != nil {
				return
			}
		}
	}()
	return out, errCh, nil
}

// Commit advances the stored offset for (record.Position.Topic, groupID)
// past record.Position.Offset. Mirrors aiokafka's commit() with
// OffsetAndMetadata(offset+1).
func (k *Kafka) Commit(ctx context.Context, groupID string, record Record) error {
	k.mu.Lock()
	if !k.connected {
		k.mu.Unlock()
		return fmt.Errorf("%w: not connected", ErrConnection)
	}
	cl, err := k.consumerForLocked(record.Position.Topic, groupID)
	if err != nil {
		k.mu.Unlock()
		return err
	}
	k.mu.Unlock()

	// franz-go's CommitOffsets takes a map of topic -> partition ->
	// EpochOffset. The committed offset is "next record to consume",
	// i.e. record.Offset + 1 (matches aiokafka semantics).
	rec := &kgo.Record{
		Topic:     record.Position.Topic,
		Partition: record.Position.Partition,
		Offset:    record.Position.Offset,
	}
	cl.MarkCommitRecords(rec)
	if err := cl.CommitMarkedOffsets(ctx); err != nil {
		return fmt.Errorf("%w: kafka commit: %v", ErrWal, err)
	}
	return nil
}

// isTimeout returns true if err looks like a transport/produce timeout
// (heuristic by error string — franz-go surfaces these as kerr.*).
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Don't pull in kerr just for two strings; the heuristic is
	// loose-but-safe (we only map this to a typed sentinel for
	// observability, not for control flow).
	return contains(s, "timeout") || contains(s, "timed out")
}

func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			c := haystack[i+j]
			n := needle[j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != n {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
