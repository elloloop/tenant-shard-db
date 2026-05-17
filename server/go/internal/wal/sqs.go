// SPDX-License-Identifier: AGPL-3.0-only

package wal

// AWS SQS FIFO WAL backend for the Go server. Ported from the retired
// Python source (server/python/entdb_server/wal/sqs.py).
//
// Concept mapping (mirrors the Python docstring):
//
//   - FIFO queue              -> WAL topic. The queue name MUST end in
//                                ".fifo"; ordering + dedupe only exist
//                                on FIFO queues.
//   - MessageGroupId          -> partition key (tenant_id). SQS FIFO
//                                guarantees total order within a group,
//                                so per-tenant order is preserved.
//   - MessageDeduplicationId  -> idempotency. We send the WAL
//                                HeaderIdempotencyKey as the dedup id
//                                (falling back to a content hash when
//                                absent), so SQS itself collapses a
//                                retried Append within its 5-minute
//                                dedup window — server-side dedupe on
//                                top of the in-process idempotency
//                                cache.
//   - ReceiptHandle           -> commit handle. Commit issues
//                                DeleteMessage (the SQS ack).
//
// SQS FIFO has no partitions; StreamPos.Partition is always 0 and
// StreamPos.Offset is a per-process monotone counter (parity with
// sqs.py self._counter). Ordering across a poll is preserved because
// SQS delivers a group's messages in order and we never reorder a
// batch.
//
// Halt-on-poison: message bodies are passed through verbatim; a
// malformed event surfaces at the applier, not here.
//
// Testing: SqsAPI is the seam. The concrete *sqs.Client satisfies it;
// unit tests inject a fake (sqs_test.go) — no live AWS calls.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SqsAPI is the slice of the AWS SQS client the WAL backend uses.
type SqsAPI interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	GetQueueAttributes(ctx context.Context, in *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
}

// SqsConfig captures the SQS-specific knobs (parity with the Python
// SqsConfig: queue url, region, endpoint, visibility, long-poll wait).
type SqsConfig struct {
	// QueueURL is the FIFO queue URL (must end in ".fifo"). Falls back
	// to the Append/Poll topic arg when empty.
	QueueURL string
	// Region is the AWS region.
	Region string
	// EndpointURL overrides the AWS endpoint (LocalStack / testing).
	EndpointURL string
	// MaxMessages bounds a ReceiveMessage call. SQS hard-caps at 10.
	MaxMessages int32
	// WaitTimeSeconds is the long-poll wait (0-20). Python default 1.
	WaitTimeSeconds int32
	// VisibilityTimeout seconds an in-flight message is hidden before
	// redelivery if not committed (deleted). Python default 30.
	VisibilityTimeout int32
}

// DefaultSqsConfig returns a config matching the Python defaults.
func DefaultSqsConfig(queueURL, region string) SqsConfig {
	return SqsConfig{
		QueueURL:          queueURL,
		Region:            region,
		MaxMessages:       10,
		WaitTimeSeconds:   1,
		VisibilityTimeout: 30,
	}
}

// Sqs implements Producer + Consumer against AWS SQS FIFO queues.
type Sqs struct {
	config SqsConfig

	newClient func(ctx context.Context) (SqsAPI, error)

	mu        sync.Mutex
	connected bool
	client    SqsAPI
	counter   atomic.Int64

	// pendingAcks[offsetKey] -> receipt handle for in-flight messages
	// awaiting Commit (DeleteMessage).
	pendingAcks map[string]string

	idemp map[string]map[string]map[string]StreamPos
}

// NewSqs constructs an SQS WAL backend.
func NewSqs(cfg SqsConfig) *Sqs {
	if cfg.MaxMessages <= 0 || cfg.MaxMessages > 10 {
		cfg.MaxMessages = 10
	}
	if cfg.WaitTimeSeconds < 0 || cfg.WaitTimeSeconds > 20 {
		cfg.WaitTimeSeconds = 1
	}
	if cfg.VisibilityTimeout <= 0 {
		cfg.VisibilityTimeout = 30
	}
	s := &Sqs{
		config:      cfg,
		pendingAcks: make(map[string]string),
		idemp:       make(map[string]map[string]map[string]StreamPos),
	}
	s.newClient = s.defaultNewClient
	return s
}

func (s *Sqs) defaultNewClient(ctx context.Context) (SqsAPI, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if strings.TrimSpace(s.config.Region) != "" {
		opts = append(opts, awsconfig.WithRegion(strings.TrimSpace(s.config.Region)))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		if strings.TrimSpace(s.config.EndpointURL) != "" {
			o.BaseEndpoint = aws.String(strings.TrimSpace(s.config.EndpointURL))
		}
	}), nil
}

func (s *Sqs) queueURL(topic string) string {
	if strings.TrimSpace(s.config.QueueURL) != "" {
		return s.config.QueueURL
	}
	return topic
}

// Connect builds the client and verifies the queue is reachable
// (GetQueueAttributes — parity with sqs.py health_check on connect).
func (s *Sqs) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected {
		return nil
	}
	if strings.TrimSpace(s.config.QueueURL) == "" {
		return fmt.Errorf("%w: sqs: queue url required", ErrConnection)
	}
	client, err := s.newClient(ctx)
	if err != nil {
		return fmt.Errorf("%w: sqs client: %v", ErrConnection, err)
	}
	if _, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(s.config.QueueURL),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	}); err != nil {
		return fmt.Errorf("%w: sqs queue %q: %v", ErrConnection, s.config.QueueURL, err)
	}
	s.client = client
	s.connected = true
	return nil
}

// Close releases the client and forgets in-flight ack handles.
func (s *Sqs) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = nil
	s.connected = false
	s.pendingAcks = make(map[string]string)
	return nil
}

// Append SendMessage to the FIFO queue with MessageGroupId=key for
// per-tenant order and MessageDeduplicationId from the idempotency key
// for SQS-side dedupe. See Producer.Append.
func (s *Sqs) Append(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string][]byte,
) (StreamPos, error) {
	if err := ctx.Err(); err != nil {
		return StreamPos{}, err
	}
	s.mu.Lock()
	if !s.connected || s.client == nil {
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
	client := s.client
	s.mu.Unlock()

	queue := s.queueURL(topic)

	// MessageDeduplicationId: prefer the WAL idempotency key so a
	// caller retry collapses server-side; else fall back to a content
	// hash (parity with sqs.py sha256(value)).
	dedupID := idempKey
	if dedupID == "" {
		sum := sha256.Sum256(value)
		dedupID = hex.EncodeToString(sum[:])
	}

	in := &sqs.SendMessageInput{
		QueueUrl:               aws.String(queue),
		MessageBody:            aws.String(decodeUTF8(value)),
		MessageGroupId:         aws.String(key),
		MessageDeduplicationId: aws.String(dedupID),
	}
	if attrs := sqsMessageAttributes(headers); len(attrs) > 0 {
		in.MessageAttributes = attrs
	}

	if _, err := client.SendMessage(ctx, in); err != nil {
		return StreamPos{}, classifyAWSErr("sqs send message", err)
	}

	pos := StreamPos{
		Topic:       queue,
		Partition:   0,
		Offset:      s.counter.Add(1),
		TimestampMs: time.Now().UnixMilli(),
	}
	if idempKey != "" {
		s.mu.Lock()
		storeIdemp(s.idemp, topic, key, idempKey, pos)
		s.mu.Unlock()
	}
	return pos, nil
}

// PollBatch ReceiveMessage up to maxRecords (SQS caps the wire call at
// 10; we loop to satisfy larger maxRecords within the deadline).
func (s *Sqs) PollBatch(
	ctx context.Context,
	topic, groupID string,
	maxRecords int,
	timeout time.Duration,
) ([]Record, error) {
	if maxRecords <= 0 {
		return nil, nil
	}
	s.mu.Lock()
	if !s.connected || s.client == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	client := s.client
	s.mu.Unlock()

	queue := s.queueURL(topic)
	deadline := time.Now().Add(timeout)
	out := make([]Record, 0, maxRecords)

	for len(out) < maxRecords {
		if err := ctx.Err(); err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		remaining := maxRecords - len(out)
		batchSize := int32(remaining)
		if batchSize > 10 {
			batchSize = 10
		}
		waitSecs := s.config.WaitTimeSeconds
		if rem := time.Until(deadline); rem < time.Duration(waitSecs)*time.Second {
			waitSecs = int32(rem / time.Second)
			if waitSecs < 0 {
				waitSecs = 0
			}
		}
		resp, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(queue),
			MaxNumberOfMessages:   batchSize,
			WaitTimeSeconds:       waitSecs,
			VisibilityTimeout:     s.config.VisibilityTimeout,
			MessageAttributeNames: []string{"All"},
			MessageSystemAttributeNames: []sqstypes.MessageSystemAttributeName{
				sqstypes.MessageSystemAttributeNameSentTimestamp,
				sqstypes.MessageSystemAttributeNameMessageGroupId,
			},
		})
		if err != nil {
			if ctx.Err() != nil && len(out) > 0 {
				return out, nil
			}
			return nil, classifyAWSErr("sqs receive message", err)
		}
		for _, m := range resp.Messages {
			out = append(out, s.toRecord(queue, m))
		}
		if len(resp.Messages) == 0 {
			if !time.Now().Before(deadline) {
				return out, nil
			}
			// Long-poll already waited; small back-off avoids a hot loop
			// when WaitTimeSeconds is 0.
			if !sleepCtx(ctx, 100*time.Millisecond, deadline) {
				return out, nil
			}
		}
		if !time.Now().Before(deadline) {
			return out, nil
		}
	}
	return out, nil
}

func (s *Sqs) toRecord(queue string, m sqstypes.Message) Record {
	offset := s.counter.Add(1)
	offsetKey := strconv.FormatInt(offset, 10)

	s.mu.Lock()
	s.pendingAcks[offsetKey] = aws.ToString(m.ReceiptHandle)
	s.mu.Unlock()

	headers := map[string][]byte{}
	for name, attr := range m.MessageAttributes {
		headers[name] = []byte(aws.ToString(attr.StringValue))
	}
	tsMs := time.Now().UnixMilli()
	if v, ok := m.Attributes[string(sqstypes.MessageSystemAttributeNameSentTimestamp)]; ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			tsMs = n
		}
	}
	key := m.Attributes[string(sqstypes.MessageSystemAttributeNameMessageGroupId)]
	return Record{
		Key:   key,
		Value: []byte(aws.ToString(m.Body)),
		Position: StreamPos{
			Topic:       queue,
			Partition:   0,
			Offset:      offset,
			TimestampMs: tsMs,
		},
		Headers: headers,
	}
}

// Subscribe streams records by repeatedly polling. Auto-commits each
// delivered record (DeleteMessage), matching the InMemory/Kafka
// Subscribe contract.
func (s *Sqs) Subscribe(
	ctx context.Context,
	topic, groupID string,
) (<-chan Record, <-chan error, error) {
	s.mu.Lock()
	if !s.connected || s.client == nil {
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
			recs, err := s.PollBatch(ctx, topic, groupID, int(s.config.MaxMessages), time.Duration(s.config.WaitTimeSeconds)*time.Second+200*time.Millisecond)
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

// Commit deletes the message from the queue (the SQS ack). Mirrors
// sqs.py commit -> DeleteMessage by receipt handle.
func (s *Sqs) Commit(ctx context.Context, groupID string, record Record) error {
	s.mu.Lock()
	if !s.connected || s.client == nil {
		s.mu.Unlock()
		return fmt.Errorf("%w: not connected", ErrConnection)
	}
	offsetKey := strconv.FormatInt(record.Position.Offset, 10)
	receipt, ok := s.pendingAcks[offsetKey]
	if ok {
		delete(s.pendingAcks, offsetKey)
	}
	client := s.client
	s.mu.Unlock()

	if !ok || receipt == "" {
		// Nothing to ack (already committed or unknown offset). Mirrors
		// sqs.py's "no pending receipt handle" warning -> no-op.
		return nil
	}
	queue := record.Position.Topic
	if queue == "" {
		queue = s.queueURL("")
	}
	if _, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queue),
		ReceiptHandle: aws.String(receipt),
	}); err != nil {
		return classifyAWSErr("sqs delete message", err)
	}
	return nil
}

func sqsMessageAttributes(headers map[string][]byte) map[string]sqstypes.MessageAttributeValue {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]sqstypes.MessageAttributeValue, len(headers))
	for k, v := range headers {
		out[k] = sqstypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(decodeUTF8(v)),
		}
	}
	return out
}
