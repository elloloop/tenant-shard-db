// SPDX-License-Identifier: AGPL-3.0-only

package wal

// AWS Kinesis Data Streams WAL backend for the Go server.
//
// Concept mapping:
//
//   - Kinesis stream  -> WAL topic (--wal-topic / config.StreamName).
//   - Shard           -> partition. The StreamPos.Partition field holds
//                         the parsed shard number ("shardId-000000000003"
//                         -> 3) so per-shard ordering survives.
//   - Sequence number -> offset. Kinesis sequence numbers are opaque
//                         decimal strings far larger than int64; we keep
//                         the numeric form in StreamPos.Offset (best
//                         effort) AND the exact string in the record
//                         headers under HeaderKinesisSeq so Commit /
//                         iterator re-creation use the lossless value.
//   - ExplicitHashKey  -> per-tenant routing. We hash the Append `key`
//                         (tenant_id) via md5 (first 4 bytes) so the
//                         same tenant always lands on the same shard,
//                         giving per-tenant total order.
//
// Durability knobs:
//
//   - PutRecord with ExplicitHashKey; the call returns only after the
//     record is durably replicated across the stream's shards.
//   - GetRecords iterator walk per shard; ExpiredIteratorException is
//     recovered from the in-memory checkpoint (AFTER_SEQUENCE_NUMBER).
//   - Idempotent retry: like memory.go / kafka.go we cache
//     (topic, key, idempotency-key) -> StreamPos so a retried Append
//     within the producer session collapses to the original receipt
//     without writing a duplicate record. Kinesis itself has no
//     server-side dedupe, so this application-level cache is the only
//     dedupe gate at append time (the applier is the apply-time gate).
//
// Halt-on-poison: undecodable record bytes are NOT dropped — they flow
// through to the applier which decides whether to halt (CLAUDE.md
// invariant: the WAL is the source of truth).
//
// Testing: KinesisAPI is the seam. The concrete *kinesis.Client from
// aws-sdk-go-v2 satisfies it; unit tests inject a fake (kinesis_test.go)
// so there are no live AWS calls.

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/smithy-go"
)

// HeaderKinesisSeq is the reserved record-header key under which the
// Kinesis backend stores the exact (lossless, string) sequence number.
// StreamPos.Offset carries a best-effort int64 of the same value but
// Kinesis sequence numbers can exceed int64; Commit and iterator
// recovery use this header so they stay exact.
const HeaderKinesisSeq = "x-entdb-kinesis-seq"

// KinesisAPI is the slice of the AWS Kinesis client the WAL backend
// uses. The concrete *kinesis.Client satisfies it; tests inject a fake.
type KinesisAPI interface {
	DescribeStream(ctx context.Context, in *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error)
	PutRecord(ctx context.Context, in *kinesis.PutRecordInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error)
	ListShards(ctx context.Context, in *kinesis.ListShardsInput, optFns ...func(*kinesis.Options)) (*kinesis.ListShardsOutput, error)
	GetShardIterator(ctx context.Context, in *kinesis.GetShardIteratorInput, optFns ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error)
	GetRecords(ctx context.Context, in *kinesis.GetRecordsInput, optFns ...func(*kinesis.Options)) (*kinesis.GetRecordsOutput, error)
}

// KinesisConfig captures the Kinesis-specific knobs (region, endpoint,
// stream name, iterator type, batch).
type KinesisConfig struct {
	// StreamName is the Kinesis data stream name. Defaults to the WAL
	// topic when empty (set at Append time from the topic arg).
	StreamName string
	// Region is the AWS region (e.g. "us-east-1").
	Region string
	// EndpointURL overrides the AWS endpoint (LocalStack / testing).
	EndpointURL string
	// IteratorType is "TRIM_HORIZON" (default, replay from the start)
	// or "LATEST".
	IteratorType string
	// MaxRecordsPerGet bounds a single GetRecords call. Kinesis caps
	// this at 10000; default is 100.
	MaxRecordsPerGet int32
}

// DefaultKinesisConfig returns a config with sensible defaults.
func DefaultKinesisConfig(streamName, region string) KinesisConfig {
	return KinesisConfig{
		StreamName:       streamName,
		Region:           region,
		IteratorType:     "TRIM_HORIZON",
		MaxRecordsPerGet: 100,
	}
}

// Kinesis implements Producer + Consumer against AWS Kinesis Data
// Streams. Construct with NewKinesis; Connect must be called before
// any Append / Subscribe / PollBatch.
type Kinesis struct {
	config KinesisConfig

	// newClient builds the API client at Connect time. Overridable in
	// tests to inject a fake without touching AWS.
	newClient func(ctx context.Context) (KinesisAPI, error)

	mu        sync.Mutex
	connected bool
	client    KinesisAPI

	// shardIters[shardID] -> next shard iterator token.
	shardIters map[string]string
	// checkpoints[shardID] -> last delivered sequence number (string,
	// lossless). Used to recover an expired iterator.
	checkpoints map[string]string

	// idemp[topic][key][idempotencyKey] -> previously-issued StreamPos.
	idemp map[string]map[string]map[string]StreamPos
}

// NewKinesis constructs a Kinesis WAL backend.
func NewKinesis(cfg KinesisConfig) *Kinesis {
	if cfg.IteratorType == "" {
		cfg.IteratorType = "TRIM_HORIZON"
	}
	if cfg.MaxRecordsPerGet <= 0 {
		cfg.MaxRecordsPerGet = 100
	}
	k := &Kinesis{
		config:      cfg,
		shardIters:  make(map[string]string),
		checkpoints: make(map[string]string),
		idemp:       make(map[string]map[string]map[string]StreamPos),
	}
	k.newClient = k.defaultNewClient
	return k
}

func (k *Kinesis) defaultNewClient(ctx context.Context) (KinesisAPI, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if strings.TrimSpace(k.config.Region) != "" {
		opts = append(opts, awsconfig.WithRegion(strings.TrimSpace(k.config.Region)))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return kinesis.NewFromConfig(awsCfg, func(o *kinesis.Options) {
		if strings.TrimSpace(k.config.EndpointURL) != "" {
			o.BaseEndpoint = aws.String(strings.TrimSpace(k.config.EndpointURL))
		}
	}), nil
}

// streamName resolves the effective stream name: explicit config wins,
// otherwise fall back to the topic the caller passed.
func (k *Kinesis) streamName(topic string) string {
	if strings.TrimSpace(k.config.StreamName) != "" {
		return k.config.StreamName
	}
	return topic
}

// Connect builds the client and verifies the stream exists via
// DescribeStream.
func (k *Kinesis) Connect(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.connected {
		return nil
	}
	if strings.TrimSpace(k.config.StreamName) == "" {
		return fmt.Errorf("%w: kinesis: stream name required", ErrConnection)
	}
	client, err := k.newClient(ctx)
	if err != nil {
		return fmt.Errorf("%w: kinesis client: %v", ErrConnection, err)
	}
	if _, err := client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
		StreamName: aws.String(k.config.StreamName),
	}); err != nil {
		return fmt.Errorf("%w: kinesis describe stream %q: %v", ErrConnection, k.config.StreamName, err)
	}
	k.client = client
	k.connected = true
	return nil
}

// Close releases the client. Safe to call multiple times.
func (k *Kinesis) Close(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.client = nil
	k.connected = false
	k.shardIters = make(map[string]string)
	k.checkpoints = make(map[string]string)
	return nil
}

// Append PutRecords value under an explicit hash key derived from the
// tenant key, so the same tenant always lands on the same shard. See
// Producer.Append for the contract.
func (k *Kinesis) Append(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string][]byte,
) (StreamPos, error) {
	if err := ctx.Err(); err != nil {
		return StreamPos{}, err
	}
	k.mu.Lock()
	if !k.connected || k.client == nil {
		k.mu.Unlock()
		return StreamPos{}, fmt.Errorf("%w: not connected", ErrConnection)
	}
	idempKey := idempotencyKey(headers)
	if idempKey != "" {
		if pos, ok := lookupIdemp(k.idemp, topic, key, idempKey); ok {
			k.mu.Unlock()
			return pos, nil
		}
	}
	client := k.client
	k.mu.Unlock()

	stream := k.streamName(topic)
	// Embed headers in the value so a pure-Kinesis record (no native
	// header support) round-trips headers through Subscribe/PollBatch.
	payload := wrapHeaders(value, headers)
	explicitHash := explicitHashKey(key)

	out, err := client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:      aws.String(stream),
		Data:            payload,
		PartitionKey:    aws.String(key),
		ExplicitHashKey: aws.String(explicitHash),
	})
	if err != nil {
		return StreamPos{}, classifyKinesisErr("put record", err)
	}

	shardID := aws.ToString(out.ShardId)
	seq := aws.ToString(out.SequenceNumber)
	pos := StreamPos{
		Topic:       stream,
		Partition:   parseShardNumber(shardID),
		Offset:      seqToInt64(seq),
		TimestampMs: time.Now().UnixMilli(),
	}

	if idempKey != "" {
		k.mu.Lock()
		storeIdemp(k.idemp, topic, key, idempKey, pos)
		k.mu.Unlock()
	}
	return pos, nil
}

// PollBatch fetches up to maxRecords across all shards, blocking up to
// timeout for records. See Consumer.PollBatch.
func (k *Kinesis) PollBatch(
	ctx context.Context,
	topic, groupID string,
	maxRecords int,
	timeout time.Duration,
) ([]Record, error) {
	if maxRecords <= 0 {
		return nil, nil
	}
	k.mu.Lock()
	if !k.connected || k.client == nil {
		k.mu.Unlock()
		return nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	k.mu.Unlock()

	deadline := time.Now().Add(timeout)
	out := make([]Record, 0, maxRecords)
	for {
		if err := ctx.Err(); err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		recs, err := k.drainAllShards(ctx, topic, maxRecords-len(out))
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
		if len(out) >= maxRecords {
			return out[:maxRecords], nil
		}
		if !time.Now().Before(deadline) {
			return out, nil
		}
		if len(recs) == 0 {
			// Avoid a tight poll loop with a 200ms back-off.
			if !sleepCtx(ctx, 200*time.Millisecond, deadline) {
				return out, nil
			}
		}
	}
}

// Subscribe streams records continuously by polling all shards.
func (k *Kinesis) Subscribe(
	ctx context.Context,
	topic, groupID string,
) (<-chan Record, <-chan error, error) {
	k.mu.Lock()
	if !k.connected || k.client == nil {
		k.mu.Unlock()
		return nil, nil, fmt.Errorf("%w: not connected", ErrConnection)
	}
	k.mu.Unlock()

	out := make(chan Record)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errCh)
		for {
			if ctx.Err() != nil {
				return
			}
			recs, err := k.drainAllShards(ctx, topic, int(k.config.MaxRecordsPerGet))
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
					_ = k.Commit(ctx, groupID, r)
				}
			}
			if len(recs) == 0 {
				if !sleepCtx(ctx, 200*time.Millisecond, time.Now().Add(time.Second)) {
					if ctx.Err() != nil {
						return
					}
				}
			}
		}
	}()
	return out, errCh, nil
}

// Commit records the per-shard checkpoint so an expired iterator can be
// rebuilt AFTER_SEQUENCE_NUMBER. Kinesis has no server-side consumer
// group; this in-memory checkpoint covers the WAL contract (production
// deployments may persist it to DynamoDB).
func (k *Kinesis) Commit(ctx context.Context, groupID string, record Record) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	seq := ""
	if h, ok := record.Headers[HeaderKinesisSeq]; ok {
		seq = string(h)
	}
	if seq == "" {
		seq = strconv.FormatInt(record.Position.Offset, 10)
	}
	shardID := fmt.Sprintf("shardId-%012d", record.Position.Partition)
	k.checkpoints[shardID] = seq
	return nil
}

// drainAllShards lists shards, walks each shard's iterator once, and
// returns up to limit records (in shard-id then arrival order). Caller
// is responsible for the outer poll/deadline loop.
func (k *Kinesis) drainAllShards(ctx context.Context, topic string, limit int) ([]Record, error) {
	if limit <= 0 {
		return nil, nil
	}
	stream := k.streamName(topic)

	shards, err := k.listShards(ctx, stream)
	if err != nil {
		return nil, err
	}
	sort.Strings(shards)

	out := make([]Record, 0, limit)
	for _, shardID := range shards {
		if len(out) >= limit {
			break
		}
		iter, err := k.iteratorForShard(ctx, stream, shardID)
		if err != nil {
			return nil, err
		}
		if iter == "" {
			continue
		}
		recs, nextIter, err := k.getRecords(ctx, stream, shardID, iter, int32(limit-len(out)))
		if err != nil {
			return nil, err
		}
		k.mu.Lock()
		if nextIter != "" {
			k.shardIters[shardID] = nextIter
		} else {
			delete(k.shardIters, shardID)
		}
		k.mu.Unlock()
		out = append(out, recs...)
	}
	return out, nil
}

func (k *Kinesis) listShards(ctx context.Context, stream string) ([]string, error) {
	var shards []string
	var nextToken *string
	for {
		in := &kinesis.ListShardsInput{}
		if nextToken != nil {
			in.NextToken = nextToken
		} else {
			in.StreamName = aws.String(stream)
		}
		out, err := k.client.ListShards(ctx, in)
		if err != nil {
			return nil, classifyKinesisErr("list shards", err)
		}
		for _, s := range out.Shards {
			shards = append(shards, aws.ToString(s.ShardId))
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	return shards, nil
}

// iteratorForShard returns the cached iterator for shardID or creates a
// fresh one. A checkpointed shard resumes AFTER_SEQUENCE_NUMBER; a new
// shard uses the configured iterator type (TRIM_HORIZON | LATEST).
func (k *Kinesis) iteratorForShard(ctx context.Context, stream, shardID string) (string, error) {
	k.mu.Lock()
	if it, ok := k.shardIters[shardID]; ok {
		k.mu.Unlock()
		return it, nil
	}
	checkpoint := k.checkpoints[shardID]
	k.mu.Unlock()

	in := &kinesis.GetShardIteratorInput{
		StreamName: aws.String(stream),
		ShardId:    aws.String(shardID),
	}
	if checkpoint != "" {
		in.ShardIteratorType = types.ShardIteratorTypeAfterSequenceNumber
		in.StartingSequenceNumber = aws.String(checkpoint)
	} else if strings.EqualFold(k.config.IteratorType, "LATEST") {
		in.ShardIteratorType = types.ShardIteratorTypeLatest
	} else {
		in.ShardIteratorType = types.ShardIteratorTypeTrimHorizon
	}
	out, err := k.client.GetShardIterator(ctx, in)
	if err != nil {
		return "", classifyKinesisErr("get shard iterator", err)
	}
	it := aws.ToString(out.ShardIterator)
	k.mu.Lock()
	k.shardIters[shardID] = it
	k.mu.Unlock()
	return it, nil
}

func (k *Kinesis) getRecords(ctx context.Context, stream, shardID, iter string, limit int32) ([]Record, string, error) {
	if limit <= 0 {
		limit = 1
	}
	if limit > k.config.MaxRecordsPerGet {
		limit = k.config.MaxRecordsPerGet
	}
	out, err := k.client.GetRecords(ctx, &kinesis.GetRecordsInput{
		ShardIterator: aws.String(iter),
		Limit:         aws.Int32(limit),
	})
	if err != nil {
		// ExpiredIteratorException: rebuild from checkpoint and retry once.
		if isExpiredIterator(err) {
			k.mu.Lock()
			delete(k.shardIters, shardID)
			k.mu.Unlock()
			newIter, ierr := k.iteratorForShard(ctx, stream, shardID)
			if ierr != nil {
				return nil, "", ierr
			}
			out, err = k.client.GetRecords(ctx, &kinesis.GetRecordsInput{
				ShardIterator: aws.String(newIter),
				Limit:         aws.Int32(limit),
			})
			if err != nil {
				return nil, "", classifyKinesisErr("get records", err)
			}
		} else {
			return nil, "", classifyKinesisErr("get records", err)
		}
	}

	recs := make([]Record, 0, len(out.Records))
	for _, r := range out.Records {
		seq := aws.ToString(r.SequenceNumber)
		data, hdrs := unwrapHeaders(r.Data)
		if hdrs == nil {
			hdrs = map[string][]byte{}
		}
		// Stamp the lossless sequence number so Commit / iterator
		// recovery use the exact value (StreamPos.Offset is lossy).
		hdrs[HeaderKinesisSeq] = []byte(seq)
		ts := time.Now().UnixMilli()
		if r.ApproximateArrivalTimestamp != nil {
			ts = r.ApproximateArrivalTimestamp.UnixMilli()
		}
		recs = append(recs, Record{
			Key:   aws.ToString(r.PartitionKey),
			Value: data,
			Position: StreamPos{
				Topic:       stream,
				Partition:   parseShardNumber(shardID),
				Offset:      seqToInt64(seq),
				TimestampMs: ts,
			},
			Headers: hdrs,
		})
		k.mu.Lock()
		k.checkpoints[shardID] = seq
		k.mu.Unlock()
	}
	return recs, aws.ToString(out.NextShardIterator), nil
}

// --- helpers shared by the cloud backends ---

func idempotencyKey(headers map[string][]byte) string {
	if h, ok := headers[HeaderIdempotencyKey]; ok && len(h) > 0 {
		return string(h)
	}
	return ""
}

func lookupIdemp(m map[string]map[string]map[string]StreamPos, topic, key, idemp string) (StreamPos, bool) {
	if byTopic, ok := m[topic]; ok {
		if byKey, ok := byTopic[key]; ok {
			if pos, ok := byKey[idemp]; ok {
				return pos, true
			}
		}
	}
	return StreamPos{}, false
}

func storeIdemp(m map[string]map[string]map[string]StreamPos, topic, key, idemp string, pos StreamPos) {
	byTopic, ok := m[topic]
	if !ok {
		byTopic = make(map[string]map[string]StreamPos)
		m[topic] = byTopic
	}
	byKey, ok := byTopic[key]
	if !ok {
		byKey = make(map[string]StreamPos)
		byTopic[key] = byKey
	}
	byKey[idemp] = pos
}

// explicitHashKey: md5(key)[:8] as unsigned 32-bit int, decimal string.
// Same tenant -> same hash -> same shard -> per-tenant total order.
func explicitHashKey(key string) string {
	sum := md5.Sum([]byte(key))
	v := binary.BigEndian.Uint32(sum[:4])
	return strconv.FormatUint(uint64(v), 10)
}

// parseShardNumber extracts the trailing integer from a shard id like
// "shardId-000000000003" -> 3.
func parseShardNumber(shardID string) int32 {
	if shardID == "" {
		return 0
	}
	parts := strings.Split(shardID, "-")
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0
	}
	return int32(n)
}

// seqToInt64 best-effort parses a Kinesis sequence number (a very large
// decimal string) into int64. Kinesis sequence numbers exceed int64, so
// callers needing the exact value read HeaderKinesisSeq instead; this
// is only for StreamPos.Offset ordering hints within a shard.
func seqToInt64(seq string) int64 {
	if seq == "" {
		return 0
	}
	if n, err := strconv.ParseInt(seq, 10, 64); err == nil {
		return n
	}
	// Truncate the high digits so monotonicity within a shard is still
	// (mostly) preserved for the int64 hint; the lossless value lives
	// in the record header.
	bi, ok := new(big.Int).SetString(seq, 10)
	if !ok {
		return 0
	}
	return bi.Mod(bi, big.NewInt(1<<62)).Int64()
}

func isExpiredIterator(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return ae.ErrorCode() == "ExpiredIteratorException"
	}
	return strings.Contains(strings.ToLower(err.Error()), "expirediterator")
}

func classifyKinesisErr(op string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "ProvisionedThroughputExceededException":
			return fmt.Errorf("%w: kinesis %s: %v", ErrTimeout, op, err)
		case "ResourceNotFoundException":
			return fmt.Errorf("%w: kinesis %s: %v", ErrConnection, op, err)
		}
	}
	if isTimeout(err) {
		return fmt.Errorf("%w: kinesis %s: %v", ErrTimeout, op, err)
	}
	return fmt.Errorf("%w: kinesis %s: %v", ErrWal, op, err)
}

// sleepCtx sleeps for d, but returns false early if ctx is cancelled or
// the deadline elapses (so a poll loop doesn't overshoot its timeout).
func sleepCtx(ctx context.Context, d time.Duration, deadline time.Time) bool {
	if rem := time.Until(deadline); rem < d {
		d = rem
	}
	if d <= 0 {
		return false
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
