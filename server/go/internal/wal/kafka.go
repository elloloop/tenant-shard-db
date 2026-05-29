// SPDX-License-Identifier: AGPL-3.0-only

package wal

// Kafka/Redpanda WAL backend for the Go server. The durability-critical
// knobs are:
//
//   - Producer: acks=all, enable.idempotence=true,
//                max.in.flight.requests.per.connection=1 (sarama requires
//                this when idempotent=true; see sarama.NewConfig docs),
//                linger.ms=5, request.timeout.ms=30000.
//   - Consumer: auto.offset.reset=earliest, enable.auto.commit=false
//                (manual commit via session.MarkMessage + session.Commit
//                per applied record), session.timeout.ms=30000,
//                heartbeat.interval.ms=10000.
//   - Topic: single, name comes from --wal-topic (default "entdb-wal").
//   - Partition: keyed by tenant_id (the Append `key` arg). sarama's
//                NewHashPartitioner hashes the key, giving per-tenant
//                total order while spreading load across partitions —
//                same semantic as the franz-go default and aiokafka's
//                DefaultPartitioner.
//   - Headers: pass-through, including HeaderIdempotencyKey.
//
// Halt-on-poison: a malformed record surfaces as a Subscribe error /
// PollBatch error and the applier supervisor decides whether to halt.
// We do NOT silently skip undecodable records (CLAUDE.md invariant #1
// — the WAL is the source of truth).
//
// Idempotent retry: like InMemory, we cache (topic, key, idempotency-key)
// -> StreamPos so a retried Append within the lifetime of the producer
// returns the original receipt without writing a duplicate record; the
// sarama idempotent producer config also prevents broker-side
// duplicates on retry within a single producer session.
//
// Why sarama (vs franz-go): IBM/sarama is the older, IBM-backed Kafka
// client (~11.7k stars, 350+ contributors). franz-go is faster but
// single-maintainer (~2.5k stars). For supply-chain hardening on the
// durability layer we prefer the larger contributor base.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

// KafkaConfig captures the durability-critical knobs for the Kafka /
// Redpanda WAL backend. Only the fields the Go server actually
// consumes today are present; adding SASL/SSL is a follow-up.
type KafkaConfig struct {
	// Brokers is the comma-separated bootstrap list, e.g. "redpanda:9092".
	Brokers []string
	// ClientID is the sarama client id; appears in broker logs and
	// metrics. Defaults to "entdb-server-go" when empty.
	ClientID string
	// Acks is the producer ack level. We default to "all"
	// (sarama.WaitForAll). Any non-"all" value falls back to
	// leader-only acks ("1"/"0").
	Acks string
	// EnableIdempotence enables the broker-side idempotent producer
	// (prevents duplicates on retry within a producer session).
	EnableIdempotence bool
	// MaxInFlight is the producer's max in-flight requests per
	// connection. Defaults to 5; sarama requires this to be 1 when
	// idempotence is enabled, so we cap appropriately at Connect time.
	MaxInFlight int
	// LingerMs is the batch-collection window for the producer.
	LingerMs int
	// RequestTimeoutMs bounds a single produce request.
	RequestTimeoutMs int
	// AutoOffsetReset controls where a new consumer group starts:
	// "earliest" (default) or "latest".
	AutoOffsetReset string
	// EnableAutoCommit toggles the broker-side auto-commit loop.
	// Must be false (manual commit per record) so re-delivery semantics
	// align with the applier's at-least-once contract.
	EnableAutoCommit bool
	// SessionTimeoutMs is the consumer-group session timeout.
	SessionTimeoutMs int
	// HeartbeatIntervalMs is how often the consumer heartbeats to the
	// group coordinator. Must be < SessionTimeoutMs / 3.
	HeartbeatIntervalMs int

	// ── Transport security (#569) ──────────────────────────────────
	// Applied to both the producer and consumer connections.

	// TLSEnable turns on TLS for broker connections (the "SSL" half of
	// SASL_SSL / SSL). With no CA file the host's system root pool is
	// used — the common case for managed brokers (Confluent Cloud, MSK,
	// Event Hubs Kafka).
	TLSEnable bool
	// TLSInsecureSkipVerify disables server certificate verification.
	// For testing only — never set in production.
	TLSInsecureSkipVerify bool
	// TLSCAFile is an optional PEM bundle of additional root CAs to trust
	// (private CA). Empty ⇒ system roots.
	TLSCAFile string
	// TLSClientCertFile / TLSClientKeyFile enable mutual TLS (client
	// certificate auth). Both must be set together, or neither.
	TLSClientCertFile string
	TLSClientKeyFile  string

	// SASLEnable turns on SASL authentication.
	SASLEnable bool
	// SASLMechanism is one of "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"
	// (case-insensitive). PLAIN suits Confluent Cloud / Event Hubs Kafka
	// (API key/secret or connection string); SCRAM suits self-managed
	// Kafka and MSK SCRAM.
	SASLMechanism string
	// SASLUsername / SASLPassword are the SASL credentials.
	SASLUsername string
	SASLPassword string
}

// DefaultKafkaConfig returns a config populated with sensible defaults.
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
//   - We keep one sarama SyncProducer for production and a separate
//     ConsumerGroup per (topic, groupID) consumer pair. sarama splits
//     producer and consumer-group lifecycles cleanly, so this mirrors
//     the franz-go layout.
//   - The producer client is lazily created in Connect; consumer
//     groups are lazily created on the first PollBatch / Subscribe.
//   - We hold mu for the duration of Append's idempotency-cache lookup
//     and SendMessage call setup, then release it before blocking on
//     SendMessage (so concurrent appends can pipeline).
type Kafka struct {
	config KafkaConfig

	mu        sync.Mutex
	connected bool

	// producer is the sarama SyncProducer used for Append. nil until
	// Connect is called.
	producer sarama.SyncProducer

	// consumerByGroup caches one consumer-group session driver per
	// (topic, groupID). We instantiate on demand because sarama binds
	// the group to its config at construction time.
	consumerByGroup map[consumerKey]*saramaConsumer

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
		consumerByGroup: make(map[consumerKey]*saramaConsumer),
		idemp:           make(map[string]map[string]map[string]StreamPos),
	}
}

// producerConfig builds a sarama.Config for the producer half. Kept
// out of Connect so tests can inspect / override it without spinning
// up a broker.
func (k *Kafka) producerConfig() (*sarama.Config, error) {
	config := sarama.NewConfig()
	config.ClientID = k.config.ClientID
	// Match aiokafka 1:1: V3_5_0_0 is what current Redpanda LTS and
	// Confluent Cloud advertise; sarama negotiates down if the broker
	// speaks an older protocol.
	config.Version = sarama.V3_5_0_0

	switch k.config.Acks {
	case "all", "-1":
		config.Producer.RequiredAcks = sarama.WaitForAll
	case "0":
		config.Producer.RequiredAcks = sarama.NoResponse
	default:
		// "1" or anything else: leader-only ack.
		config.Producer.RequiredAcks = sarama.WaitForLocal
	}

	config.Producer.Idempotent = k.config.EnableIdempotence
	if k.config.EnableIdempotence {
		// sarama requires MaxOpenRequests == 1 when Idempotent is on
		// (otherwise NewSyncProducer returns an error); matching to 1
		// is the safest interpretation.
		config.Net.MaxOpenRequests = 1
	} else {
		config.Net.MaxOpenRequests = k.config.MaxInFlight
	}

	config.Producer.Retry.Max = 10
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Return.Successes = true
	// Hash partitioner over the message key gives per-tenant total
	// order (the Append `key` is tenant_id) while spreading load
	// across partitions. Matches aiokafka's DefaultPartitioner and
	// the franz-go default.
	config.Producer.Partitioner = sarama.NewHashPartitioner
	// 10 MiB — matching the franz-go side's implicit ceiling and our
	// applied-event payload budget.
	config.Producer.MaxMessageBytes = 10 << 20
	config.Producer.Timeout = time.Duration(k.config.RequestTimeoutMs) * time.Millisecond
	// linger.ms equivalent: sarama batches via Flush.Frequency on the
	// SyncProducer (5ms default).
	config.Producer.Flush.Frequency = time.Duration(k.config.LingerMs) * time.Millisecond

	if err := k.config.applySecurity(config); err != nil {
		return nil, err
	}
	return config, nil
}

// consumerConfig builds a sarama.Config for the consumer half.
func (k *Kafka) consumerConfig() (*sarama.Config, error) {
	config := sarama.NewConfig()
	config.ClientID = k.config.ClientID + "-consumer"
	config.Version = sarama.V3_5_0_0

	if k.config.AutoOffsetReset == "latest" {
		config.Consumer.Offsets.Initial = sarama.OffsetNewest
	} else {
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}
	// Manual commit: disable sarama's auto-commit loop.
	config.Consumer.Offsets.AutoCommit.Enable = false
	config.Consumer.Group.Session.Timeout = time.Duration(k.config.SessionTimeoutMs) * time.Millisecond
	config.Consumer.Group.Heartbeat.Interval = time.Duration(k.config.HeartbeatIntervalMs) * time.Millisecond
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	// Surface fatal consumer errors instead of swallowing them. We
	// drain the Errors() channel inside the consumer driver below and
	// propagate to PollBatch / Subscribe.
	config.Consumer.Return.Errors = true

	if err := k.config.applySecurity(config); err != nil {
		return nil, err
	}
	return config, nil
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

	config, err := k.producerConfig()
	if err != nil {
		return fmt.Errorf("%w: kafka producer config: %v", ErrConnection, err)
	}

	prod, err := sarama.NewSyncProducer(k.config.Brokers, config)
	if err != nil {
		return fmt.Errorf("%w: kafka new producer: %v", ErrConnection, err)
	}

	k.producer = prod
	k.connected = true
	return nil
}

// Close shuts down all clients (producer + cached consumers). Safe to
// call multiple times.
func (k *Kafka) Close(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.producer != nil {
		_ = k.producer.Close()
		k.producer = nil
	}
	for ck, c := range k.consumerByGroup {
		c.close()
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
	// re-producing. The sarama idempotent producer covers broker-side
	// dedupe on transport-level retries; this is application-level
	// dedupe for caller-driven retries.
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

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	if len(headers) > 0 {
		msg.Headers = make([]sarama.RecordHeader, 0, len(headers))
		for hk, hv := range headers {
			cp := append([]byte(nil), hv...)
			msg.Headers = append(msg.Headers, sarama.RecordHeader{
				Key:   []byte(hk),
				Value: cp,
			})
		}
	}

	// Honor ctx cancellation: sarama's SendMessage is synchronous and
	// doesn't accept a context, so we race it against ctx.Done() in a
	// goroutine. If ctx fires first we return the ctx error; the
	// underlying produce may still complete in the background, which
	// is consistent with the franz-go path (broker-side idempotent
	// producer prevents duplicates on the caller's retry).
	type result struct {
		partition int32
		offset    int64
		err       error
	}
	resCh := make(chan result, 1)
	go func() {
		p, o, err := producer.SendMessage(msg)
		resCh <- result{partition: p, offset: o, err: err}
	}()

	var partition int32
	var offset int64
	select {
	case <-ctx.Done():
		return StreamPos{}, ctx.Err()
	case r := <-resCh:
		if r.err != nil {
			if errors.Is(r.err, context.DeadlineExceeded) || errors.Is(r.err, context.Canceled) {
				return StreamPos{}, r.err
			}
			// Heuristic: timeout errors -> ErrTimeout, everything
			// else rolls up under ErrWal. Connection-loss errors
			// from sarama surface as sarama.ErrOutOfBrokers etc.; we
			// treat them as ErrWal — the next Append will try to
			// reconnect transparently because the underlying client
			// is long-lived.
			if isTimeout(r.err) {
				return StreamPos{}, fmt.Errorf("%w: kafka append: %v", ErrTimeout, r.err)
			}
			return StreamPos{}, fmt.Errorf("%w: kafka append: %v", ErrWal, r.err)
		}
		partition = r.partition
		offset = r.offset
	}

	pos := StreamPos{
		Topic:       topic,
		Partition:   partition,
		Offset:      offset,
		TimestampMs: time.Now().UnixMilli(),
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

// consumerForLocked returns a saramaConsumer bound to (topic, groupID),
// lazily creating it on first access. Caller must hold k.mu.
func (k *Kafka) consumerForLocked(topic, groupID string) (*saramaConsumer, error) {
	ck := consumerKey{topic: topic, groupID: groupID}
	if c, ok := k.consumerByGroup[ck]; ok {
		return c, nil
	}

	config, err := k.consumerConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: kafka consumer config: %v", ErrConnection, err)
	}
	group, err := sarama.NewConsumerGroup(k.config.Brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("%w: kafka new consumer: %v", ErrConnection, err)
	}

	c := newSaramaConsumer(group, topic)
	k.consumerByGroup[ck] = c
	return c, nil
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
	c, err := k.consumerForLocked(topic, groupID)
	if err != nil {
		k.mu.Unlock()
		return nil, err
	}
	k.mu.Unlock()

	return c.poll(ctx, maxRecords, timeout)
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
	c, err := k.consumerForLocked(topic, groupID)
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
			recs, err := c.poll(ctx, 32, 100*time.Millisecond)
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
					// Auto-commit per record on the Subscribe path
					// (parallel with InMemory.Subscribe). Callers
					// wanting manual commits should use PollBatch +
					// Commit directly.
					_ = c.commit(ctx, r)
				}
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
	c, err := k.consumerForLocked(record.Position.Topic, groupID)
	if err != nil {
		k.mu.Unlock()
		return err
	}
	k.mu.Unlock()

	return c.commit(ctx, record)
}

// saramaConsumer wraps sarama.ConsumerGroup and a
// sarama.ConsumerGroupHandler that forwards messages through a
// buffered channel. The handler also exposes the active session so
// MarkMessage / Commit can be plumbed into wal.Consumer.Commit.
type saramaConsumer struct {
	group  sarama.ConsumerGroup
	topic  string
	cancel context.CancelFunc

	mu      sync.Mutex
	session sarama.ConsumerGroupSession
	msgs    chan *sarama.ConsumerMessage
	errs    chan error
	done    chan struct{}
}

func newSaramaConsumer(group sarama.ConsumerGroup, topic string) *saramaConsumer {
	c := &saramaConsumer{
		group: group,
		topic: topic,
		// Buffered channel so ConsumeClaim can fill ahead of pollers.
		// 256 is roughly two poll batches; tune later if needed.
		msgs: make(chan *sarama.ConsumerMessage, 256),
		errs: make(chan error, 8),
		done: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		defer close(c.done)
		for {
			if err := ctx.Err(); err != nil {
				return
			}
			// Consume blocks until rebalance/error; we re-call it in
			// a loop so the consumer keeps running across rebalances.
			if err := c.group.Consume(ctx, []string{topic}, c); err != nil {
				// Surface fatal errors; non-fatal rebalance errors
				// also flow through here, so select on done to drop
				// them once shutdown has started.
				select {
				case c.errs <- err:
				default:
				}
				if errors.Is(err, sarama.ErrClosedConsumerGroup) || ctx.Err() != nil {
					return
				}
			}
		}
	}()

	// Pump the consumer-group-wide error channel into c.errs too. We
	// don't want any unmarshal/transport error to be silently dropped
	// (CLAUDE.md invariant: WAL is the source of truth).
	go func() {
		for err := range c.group.Errors() {
			if err == nil {
				continue
			}
			select {
			case c.errs <- err:
			default:
			}
		}
	}()

	return c
}

// Setup is part of sarama.ConsumerGroupHandler. Called when a new
// session begins; we stash the session so commit() can mark offsets.
func (c *saramaConsumer) Setup(session sarama.ConsumerGroupSession) error {
	c.mu.Lock()
	c.session = session
	c.mu.Unlock()
	return nil
}

// Cleanup is part of sarama.ConsumerGroupHandler. Called when the
// session ends (rebalance or shutdown). Clearing the session means a
// subsequent commit() will no-op until the next Setup, which matches
// aiokafka behaviour during a rebalance.
func (c *saramaConsumer) Cleanup(session sarama.ConsumerGroupSession) error {
	c.mu.Lock()
	if c.session == session {
		c.session = nil
	}
	c.mu.Unlock()
	return nil
}

// ConsumeClaim is part of sarama.ConsumerGroupHandler. Forwards
// claimed messages into c.msgs in offset order within the partition.
// We deliberately do NOT decode/unmarshal here — that's the applier's
// job, and any decode error there propagates back via Commit failure
// or supervisor halt. Transport-level errors (e.g. claim cancelled
// mid-iteration) surface via session.Context().
func (c *saramaConsumer) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			select {
			case c.msgs <- msg:
			case <-session.Context().Done():
				return nil
			}
		case <-session.Context().Done():
			return nil
		}
	}
}

// isTransientConsumerErr reports whether a sarama consumer error is
// operationally transient (retryable) rather than deployment-fatal.
// Idle-connection reaping (io.EOF), network read/write timeouts, broker
// rebalances, and leader elections are transient and recur against
// managed Kafka surfaces (notably Azure Event Hubs, which reaps idle
// consumer sockets); auth failures and a genuinely-missing topic are
// not. See issue #627.
func isTransientConsumerErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, sarama.ErrOutOfBrokers) ||
		errors.Is(err, sarama.ErrUnknownTopicOrPartition) ||
		errors.Is(err, sarama.ErrNotLeaderForPartition) ||
		errors.Is(err, sarama.ErrLeaderNotAvailable) ||
		errors.Is(err, sarama.ErrBrokerNotAvailable) ||
		errors.Is(err, sarama.ErrNetworkException) ||
		errors.Is(err, sarama.ErrRequestTimedOut) {
		return true
	}
	// sarama surfaces broker/transport errors wrapped in a
	// *ConsumerError that does not Unwrap to its cause on older versions;
	// recurse into the cause so io.EOF / net timeouts nested inside one
	// are still classified.
	var ce *sarama.ConsumerError
	if errors.As(err, &ce) && ce != nil && ce.Err != nil && ce.Err != err {
		return isTransientConsumerErr(ce.Err)
	}
	return false
}

// wrapPollErr wraps a consumer poll error under ErrWal, additionally
// tagging it ErrTransient when it is operationally transient so the
// applier retries with backoff instead of exiting the process (#627).
func wrapPollErr(err error) error {
	if isTransientConsumerErr(err) {
		return fmt.Errorf("%w: %w: kafka poll: %v", ErrWal, ErrTransient, err)
	}
	return fmt.Errorf("%w: kafka poll: %v", ErrWal, err)
}

// poll drains up to maxRecords from c.msgs, blocking up to timeout.
func (c *saramaConsumer) poll(ctx context.Context, maxRecords int, timeout time.Duration) ([]Record, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	out := make([]Record, 0, maxRecords)
	for len(out) < maxRecords {
		select {
		case <-ctx.Done():
			if len(out) > 0 {
				return out, nil
			}
			return nil, ctx.Err()
		case err := <-c.errs:
			if err == nil {
				continue
			}
			return nil, wrapPollErr(err)
		case msg, ok := <-c.msgs:
			if !ok {
				return out, nil
			}
			out = append(out, recordFromMessage(msg))
		case <-deadline.C:
			return out, nil
		}
	}
	return out, nil
}

// commit marks the record and synchronously commits offsets via the
// active session. If no session is active (rebalance in progress),
// returns nil — sarama will re-deliver after the next Setup.
func (c *saramaConsumer) commit(ctx context.Context, record Record) error {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()
	if session == nil {
		// No active session; the offset will be re-delivered after
		// the next rebalance. Matches aiokafka behaviour during
		// rebalance.
		return nil
	}
	// Build a synthetic ConsumerMessage so we can use the session's
	// MarkMessage API. sarama only inspects Topic/Partition/Offset.
	msg := &sarama.ConsumerMessage{
		Topic:     record.Position.Topic,
		Partition: record.Position.Partition,
		Offset:    record.Position.Offset,
	}
	session.MarkMessage(msg, "")
	session.Commit()
	return nil
}

// close shuts down the consumer goroutine and the underlying sarama
// consumer group.
func (c *saramaConsumer) close() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.group != nil {
		_ = c.group.Close()
	}
	<-c.done
}

func recordFromMessage(msg *sarama.ConsumerMessage) Record {
	headers := map[string][]byte{}
	for _, h := range msg.Headers {
		cp := append([]byte(nil), h.Value...)
		headers[string(h.Key)] = cp
	}
	ts := msg.Timestamp.UnixMilli()
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return Record{
		Key:   string(msg.Key),
		Value: append([]byte(nil), msg.Value...),
		Position: StreamPos{
			Topic:       msg.Topic,
			Partition:   msg.Partition,
			Offset:      msg.Offset,
			TimestampMs: ts,
		},
		Headers: headers,
	}
}

// isTimeout returns true if err looks like a transport/produce timeout
// (heuristic by error string — sarama surfaces these as sarama.ErrXxx).
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") || strings.Contains(s, "timed out")
}
