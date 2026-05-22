// SPDX-License-Identifier: AGPL-3.0-only

package wal

// Google Cloud Pub/Sub WAL backend for the Go server. We use the
// generated apiv1 PublisherClient / SubscriberClient (the gRPC Pull /
// Publish / Acknowledge RPCs) for a synchronous, bounded model —
// the high-level streaming Subscription.Receive callback can't bound a
// PollBatch the way the WAL contract requires.
//
// Concept mapping:
//
//   - Pub/Sub topic   -> WAL topic.
//   - Ordering key    -> partition key (tenant_id). With message
//                         ordering enabled, Pub/Sub delivers a given
//                         ordering key's messages in publish order —
//                         that is the per-tenant total order guarantee.
//   - Subscription    -> consumer group (groupID is the subscription
//                         id).
//   - ack_id          -> commit handle. Commit issues Acknowledge.
//
// Pub/Sub has no partitions; StreamPos.Partition is always 0 and
// StreamPos.Offset is the publish time in ms.
// Headers ride as Pub/Sub message attributes (string->string), so no
// payload envelope is needed (unlike Kinesis).
//
// Idempotency: the in-process (topic,key,idempotency-key) cache short-
// circuits a retried Append, identical to the other backends. Pub/Sub
// has no native producer dedupe.
//
// Testing: PubSubAPI is the seam — the apiv1 clients satisfy it via a
// thin adapter (newDefaultPubSub); unit tests inject a fake.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	vkit "cloud.google.com/go/pubsub/apiv1"
	pubsubpb "cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PubSubAPI is the slice of Pub/Sub operations the WAL backend needs.
// The adapter in newDefaultPubSub wraps the generated apiv1 clients;
// tests inject a fake.
type PubSubAPI interface {
	// Publish appends one message to topicPath with the given ordering
	// key + attributes and returns the server message id.
	Publish(ctx context.Context, topicPath, orderingKey string, data []byte, attrs map[string]string) (string, error)
	// Pull pulls up to maxMessages from subPath. Returns the received
	// messages (data, attrs, orderingKey, ackID, publishTimeMs).
	Pull(ctx context.Context, subPath string, maxMessages int32) ([]pubsubMessage, error)
	// Acknowledge acks ackIDs on subPath.
	Acknowledge(ctx context.Context, subPath string, ackIDs []string) error
	// Close releases the underlying clients.
	Close() error
}

// pubsubMessage is the transport-neutral shape a Pull returns.
type pubsubMessage struct {
	Data          []byte
	Attributes    map[string]string
	OrderingKey   string
	AckID         string
	PublishTimeMs int64
}

// PubSubConfig captures the Pub/Sub-specific knobs.
type PubSubConfig struct {
	// ProjectID is the GCP project.
	ProjectID string
	// TopicID is the topic short id. The full path is built as
	// projects/<project>/topics/<topic>.
	TopicID string
	// SubscriptionID is the default subscription short id. PollBatch /
	// Subscribe use the groupID arg when set, else this.
	SubscriptionID string
	// Endpoint overrides the Pub/Sub endpoint (emulator / testing).
	Endpoint string
	// OrderingEnabled turns on ordering-key delivery (per-tenant order).
	OrderingEnabled bool
	// MaxMessages bounds a single Pull (default 100).
	MaxMessages int32
}

// DefaultPubSubConfig returns a config with sensible defaults.
func DefaultPubSubConfig(projectID, topicID, subscriptionID string) PubSubConfig {
	return PubSubConfig{
		ProjectID:       projectID,
		TopicID:         topicID,
		SubscriptionID:  subscriptionID,
		OrderingEnabled: true,
		MaxMessages:     100,
	}
}

// PubSub implements Producer + Consumer against Google Cloud Pub/Sub.
type PubSub struct {
	config PubSubConfig

	newClient func(ctx context.Context) (PubSubAPI, error)

	mu        sync.Mutex
	connected bool
	api       PubSubAPI

	idemp map[string]map[string]map[string]StreamPos
}

// NewPubSub constructs a Pub/Sub WAL backend.
func NewPubSub(cfg PubSubConfig) *PubSub {
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = 100
	}
	p := &PubSub{
		config: cfg,
		idemp:  make(map[string]map[string]map[string]StreamPos),
	}
	p.newClient = p.defaultNewClient
	return p
}

func (p *PubSub) defaultNewClient(ctx context.Context) (PubSubAPI, error) {
	var pubOpts, subOpts []option.ClientOption
	if strings.TrimSpace(p.config.Endpoint) != "" {
		pubOpts = append(pubOpts, option.WithEndpoint(strings.TrimSpace(p.config.Endpoint)))
		subOpts = append(subOpts, option.WithEndpoint(strings.TrimSpace(p.config.Endpoint)))
	}
	pub, err := vkit.NewPublisherClient(ctx, pubOpts...)
	if err != nil {
		return nil, err
	}
	sub, err := vkit.NewSubscriberClient(ctx, subOpts...)
	if err != nil {
		_ = pub.Close()
		return nil, err
	}
	return &apiv1PubSub{pub: pub, sub: sub}, nil
}

func (p *PubSub) topicPath() string {
	return fmt.Sprintf("projects/%s/topics/%s", p.config.ProjectID, p.config.TopicID)
}

func (p *PubSub) subPath(groupID string) string {
	sub := groupID
	if strings.TrimSpace(sub) == "" {
		sub = p.config.SubscriptionID
	}
	return fmt.Sprintf("projects/%s/subscriptions/%s", p.config.ProjectID, sub)
}

// Connect builds the clients. Pub/Sub has no cheap "describe" that
// works without IAM, so connectivity is validated lazily on first
// Publish/Pull.
func (p *PubSub) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.connected {
		return nil
	}
	if strings.TrimSpace(p.config.ProjectID) == "" || strings.TrimSpace(p.config.TopicID) == "" {
		return fmt.Errorf("%w: pubsub: project id and topic id required", ErrConnection)
	}
	api, err := p.newClient(ctx)
	if err != nil {
		return fmt.Errorf("%w: pubsub client: %v", ErrConnection, err)
	}
	p.api = api
	p.connected = true
	return nil
}

// Close releases the clients. Safe to call multiple times.
func (p *PubSub) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.api != nil {
		_ = p.api.Close()
		p.api = nil
	}
	p.connected = false
	return nil
}

// Append publishes value with ordering key = tenant key for per-tenant
// order. See Producer.Append.
func (p *PubSub) Append(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string][]byte,
) (StreamPos, error) {
	if err := ctx.Err(); err != nil {
		return StreamPos{}, err
	}
	p.mu.Lock()
	if !p.connected || p.api == nil {
		p.mu.Unlock()
		return StreamPos{}, fmt.Errorf("%w: not connected", ErrConnection)
	}
	idempKey := idempotencyKey(headers)
	if idempKey != "" {
		if pos, ok := lookupIdemp(p.idemp, topic, key, idempKey); ok {
			p.mu.Unlock()
			return pos, nil
		}
	}
	api := p.api
	p.mu.Unlock()

	orderingKey := ""
	if p.config.OrderingEnabled {
		orderingKey = key
	}
	if _, err := api.Publish(ctx, p.topicPath(), orderingKey, value, stringHeaders(headers)); err != nil {
		return StreamPos{}, classifyGRPCErr("pubsub publish", err)
	}

	nowMs := time.Now().UnixMilli()
	pos := StreamPos{
		Topic:       topic,
		Partition:   0,
		Offset:      nowMs,
		TimestampMs: nowMs,
	}
	if idempKey != "" {
		p.mu.Lock()
		storeIdemp(p.idemp, topic, key, idempKey, pos)
		p.mu.Unlock()
	}
	return pos, nil
}

// PollBatch pulls up to maxRecords from the subscription (groupID).
func (p *PubSub) PollBatch(
	ctx context.Context,
	topic, groupID string,
	maxRecords int,
	timeout time.Duration,
) ([]Record, error) {
	if maxRecords <= 0 {
		return nil, nil
	}
	p.mu.Lock()
	if !p.connected || p.api == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	api := p.api
	p.mu.Unlock()

	subPath := p.subPath(groupID)
	deadline := time.Now().Add(timeout)
	out := make([]Record, 0, maxRecords)

	for len(out) < maxRecords {
		if err := ctx.Err(); err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		want := int32(maxRecords - len(out))
		if want > p.config.MaxMessages {
			want = p.config.MaxMessages
		}
		msgs, err := api.Pull(ctx, subPath, want)
		if err != nil {
			if status.Code(err) == codes.DeadlineExceeded {
				// No messages within the pull window — a normal "nothing
				// yet" signal, not an error.
				msgs = nil
			} else {
				return nil, classifyGRPCErr("pubsub pull", err)
			}
		}
		for _, m := range msgs {
			out = append(out, Record{
				Key:   m.OrderingKey,
				Value: m.Data,
				Position: StreamPos{
					Topic:       topic,
					Partition:   0,
					Offset:      m.PublishTimeMs,
					TimestampMs: m.PublishTimeMs,
				},
				Headers: pubsubHeaders(m),
			})
		}
		if len(msgs) == 0 {
			if !time.Now().Before(deadline) {
				return out, nil
			}
			if !sleepCtx(ctx, 200*time.Millisecond, deadline) {
				return out, nil
			}
		}
		if !time.Now().Before(deadline) {
			return out, nil
		}
	}
	return out, nil
}

// Subscribe streams records by repeatedly pulling. Auto-acks each
// delivered record, matching the InMemory/Kafka Subscribe contract.
func (p *PubSub) Subscribe(
	ctx context.Context,
	topic, groupID string,
) (<-chan Record, <-chan error, error) {
	p.mu.Lock()
	if !p.connected || p.api == nil {
		p.mu.Unlock()
		return nil, nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	p.mu.Unlock()

	out := make(chan Record)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		for {
			if ctx.Err() != nil {
				return
			}
			recs, err := p.PollBatch(ctx, topic, groupID, int(p.config.MaxMessages), 500*time.Millisecond)
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
					_ = p.Commit(ctx, groupID, r)
				}
			}
		}
	}()
	return out, errCh, nil
}

// Commit acknowledges the message. The ack id is carried in the record
// headers under HeaderPubSubAckID (Pub/Sub ack ids are opaque strings,
// not offsets) so Commit can resolve it without an offset->ackid map.
func (p *PubSub) Commit(ctx context.Context, groupID string, record Record) error {
	p.mu.Lock()
	if !p.connected || p.api == nil {
		p.mu.Unlock()
		return fmt.Errorf("%w: not connected", ErrConnection)
	}
	api := p.api
	p.mu.Unlock()

	ackID := ""
	if h, ok := record.Headers[HeaderPubSubAckID]; ok {
		ackID = string(h)
	}
	if ackID == "" {
		// Nothing to ack (already committed or synthetic record).
		return nil
	}
	if err := api.Acknowledge(ctx, p.subPath(groupID), []string{ackID}); err != nil {
		return classifyGRPCErr("pubsub acknowledge", err)
	}
	return nil
}

// HeaderPubSubAckID is the reserved record-header key under which the
// Pub/Sub backend stashes the ack id so Commit can resolve it. Pub/Sub
// ack ids are opaque server tokens, not offsets, so they cannot be
// derived from StreamPos; carrying the ack id on the record itself
// keeps Commit stateless (no offset->ackid map to lose on crash). The
// applier ignores headers it does not recognize, so surfacing it is
// harmless.
const HeaderPubSubAckID = "x-entdb-pubsub-ackid"

func pubsubHeaders(m pubsubMessage) map[string][]byte {
	h := bytesHeaders(m.Attributes)
	if h == nil {
		h = map[string][]byte{}
	}
	h[HeaderPubSubAckID] = []byte(m.AckID)
	return h
}

// classifyGRPCErr maps a gRPC status to a WAL sentinel. Shared by the
// gRPC-transport backends (Pub/Sub).
func classifyGRPCErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	switch status.Code(err) {
	case codes.DeadlineExceeded, codes.ResourceExhausted:
		return fmt.Errorf("%w: %s: %v", ErrTimeout, op, err)
	case codes.NotFound, codes.Unavailable, codes.Unauthenticated, codes.PermissionDenied:
		return fmt.Errorf("%w: %s: %v", ErrConnection, op, err)
	}
	return fmt.Errorf("%w: %s: %v", ErrWal, op, err)
}

// apiv1PubSub adapts the generated apiv1 clients to PubSubAPI.
type apiv1PubSub struct {
	pub *vkit.PublisherClient
	sub *vkit.SubscriberClient
}

func (a *apiv1PubSub) Publish(ctx context.Context, topicPath, orderingKey string, data []byte, attrs map[string]string) (string, error) {
	resp, err := a.pub.Publish(ctx, &pubsubpb.PublishRequest{
		Topic: topicPath,
		Messages: []*pubsubpb.PubsubMessage{{
			Data:        data,
			Attributes:  attrs,
			OrderingKey: orderingKey,
		}},
	})
	if err != nil {
		return "", err
	}
	if len(resp.MessageIds) == 0 {
		return "", nil
	}
	return resp.MessageIds[0], nil
}

func (a *apiv1PubSub) Pull(ctx context.Context, subPath string, maxMessages int32) ([]pubsubMessage, error) {
	resp, err := a.sub.Pull(ctx, &pubsubpb.PullRequest{
		Subscription: subPath,
		MaxMessages:  maxMessages,
	})
	if err != nil {
		return nil, err
	}
	out := make([]pubsubMessage, 0, len(resp.ReceivedMessages))
	for _, rm := range resp.ReceivedMessages {
		msg := rm.Message
		var pubMs int64
		if msg != nil && msg.PublishTime != nil {
			pubMs = msg.PublishTime.AsTime().UnixMilli()
		}
		var data []byte
		var attrs map[string]string
		var ordKey string
		if msg != nil {
			data = msg.Data
			attrs = msg.Attributes
			ordKey = msg.OrderingKey
		}
		out = append(out, pubsubMessage{
			Data:          data,
			Attributes:    attrs,
			OrderingKey:   ordKey,
			AckID:         rm.AckId,
			PublishTimeMs: pubMs,
		})
	}
	return out, nil
}

func (a *apiv1PubSub) Acknowledge(ctx context.Context, subPath string, ackIDs []string) error {
	return a.sub.Acknowledge(ctx, &pubsubpb.AcknowledgeRequest{
		Subscription: subPath,
		AckIds:       ackIDs,
	})
}

func (a *apiv1PubSub) Close() error {
	var firstErr error
	if a.pub != nil {
		if err := a.pub.Close(); err != nil {
			firstErr = err
		}
	}
	if a.sub != nil {
		if err := a.sub.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
