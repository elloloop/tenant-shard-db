// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/smithy-go"
)

// fakeKinesis is an in-memory stand-in for the AWS Kinesis client. It
// implements KinesisAPI so the WAL backend can be exercised with no
// live AWS calls. Records are appended to a single shard
// ("shardId-000000000000"); sequence numbers are monotone decimal
// strings so ordering is observable.
type fakeKinesis struct {
	mu        sync.Mutex
	streams   map[string]bool
	records   []fakeKRecord // single-shard model is sufficient for ordering tests
	seq       int64
	iters     map[string]int // iterator token -> next record index
	iterSeq   int
	expireOne bool // when set, the next GetRecords returns ExpiredIteratorException once
}

type fakeKRecord struct {
	partitionKey string
	data         []byte
	seq          string
}

const fakeShardID = "shardId-000000000000"

func newFakeKinesis(stream string) *fakeKinesis {
	return &fakeKinesis{
		streams: map[string]bool{stream: true},
		iters:   map[string]int{},
	}
}

func (f *fakeKinesis) DescribeStream(ctx context.Context, in *kinesis.DescribeStreamInput, _ ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.streams[aws.ToString(in.StreamName)] {
		return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "no such stream"}
	}
	return &kinesis.DescribeStreamOutput{}, nil
}

func (f *fakeKinesis) PutRecord(ctx context.Context, in *kinesis.PutRecordInput, _ ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	seq := fmt.Sprintf("%020d", f.seq)
	f.records = append(f.records, fakeKRecord{
		partitionKey: aws.ToString(in.PartitionKey),
		data:         append([]byte(nil), in.Data...),
		seq:          seq,
	})
	return &kinesis.PutRecordOutput{
		ShardId:        aws.String(fakeShardID),
		SequenceNumber: aws.String(seq),
	}, nil
}

func (f *fakeKinesis) ListShards(ctx context.Context, in *kinesis.ListShardsInput, _ ...func(*kinesis.Options)) (*kinesis.ListShardsOutput, error) {
	return &kinesis.ListShardsOutput{
		Shards: []types.Shard{{ShardId: aws.String(fakeShardID)}},
	}, nil
}

func (f *fakeKinesis) GetShardIterator(ctx context.Context, in *kinesis.GetShardIteratorInput, _ ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idx := 0
	switch in.ShardIteratorType {
	case types.ShardIteratorTypeLatest:
		idx = len(f.records)
	case types.ShardIteratorTypeAfterSequenceNumber:
		// Resume just past the checkpointed sequence.
		want := aws.ToString(in.StartingSequenceNumber)
		for i, r := range f.records {
			if r.seq == want {
				idx = i + 1
				break
			}
		}
	default: // TRIM_HORIZON
		idx = 0
	}
	f.iterSeq++
	tok := fmt.Sprintf("iter-%d", f.iterSeq)
	f.iters[tok] = idx
	return &kinesis.GetShardIteratorOutput{ShardIterator: aws.String(tok)}, nil
}

func (f *fakeKinesis) GetRecords(ctx context.Context, in *kinesis.GetRecordsInput, _ ...func(*kinesis.Options)) (*kinesis.GetRecordsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.expireOne {
		f.expireOne = false
		return nil, &smithy.GenericAPIError{Code: "ExpiredIteratorException", Message: "iterator expired"}
	}
	tok := aws.ToString(in.ShardIterator)
	idx, ok := f.iters[tok]
	if !ok {
		idx = 0
	}
	limit := 100
	if in.Limit != nil {
		limit = int(*in.Limit)
	}
	out := []types.Record{}
	for idx < len(f.records) && len(out) < limit {
		r := f.records[idx]
		ts := time.Now()
		out = append(out, types.Record{
			PartitionKey:                aws.String(r.partitionKey),
			Data:                        append([]byte(nil), r.data...),
			SequenceNumber:              aws.String(r.seq),
			ApproximateArrivalTimestamp: &ts,
		})
		idx++
	}
	f.iterSeq++
	next := fmt.Sprintf("iter-%d", f.iterSeq)
	f.iters[next] = idx
	return &kinesis.GetRecordsOutput{
		Records:           out,
		NextShardIterator: aws.String(next),
	}, nil
}

func newTestKinesis(t *testing.T, fake *fakeKinesis) *Kinesis {
	t.Helper()
	k := NewKinesis(KinesisConfig{StreamName: "entdb-wal", Region: "us-east-1"})
	k.newClient = func(ctx context.Context) (KinesisAPI, error) { return fake, nil }
	if err := k.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return k
}

func TestKinesisConnectMissingStream(t *testing.T) {
	k := NewKinesis(KinesisConfig{Region: "us-east-1"})
	if err := k.Connect(context.Background()); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestKinesisConnectStreamNotFound(t *testing.T) {
	fake := newFakeKinesis("other-stream")
	k := NewKinesis(KinesisConfig{StreamName: "entdb-wal", Region: "us-east-1"})
	k.newClient = func(ctx context.Context) (KinesisAPI, error) { return fake, nil }
	if err := k.Connect(context.Background()); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestKinesisAppendNotConnected(t *testing.T) {
	k := NewKinesis(KinesisConfig{StreamName: "s", Region: "r"})
	if _, err := k.Append(context.Background(), "s", "k", []byte("v"), nil); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestKinesisAppendPollRoundTrip(t *testing.T) {
	fake := newFakeKinesis("entdb-wal")
	k := newTestKinesis(t, fake)
	ctx := context.Background()

	pos, err := k.Append(ctx, "entdb-wal", "tenant_a", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if pos.Topic != "entdb-wal" {
		t.Fatalf("pos.Topic = %q", pos.Topic)
	}

	got, err := k.PollBatch(ctx, "entdb-wal", "g1", 10, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if string(got[0].Value) != "hello" {
		t.Fatalf("value = %q, want hello", got[0].Value)
	}
	if got[0].Key != "tenant_a" {
		t.Fatalf("key = %q, want tenant_a", got[0].Key)
	}
}

func TestKinesisHeaderRoundTrip(t *testing.T) {
	fake := newFakeKinesis("entdb-wal")
	k := newTestKinesis(t, fake)
	ctx := context.Background()

	hdrs := map[string][]byte{
		HeaderIdempotencyKey: []byte("uuid-1"),
		"x-trace":            []byte("abc"),
	}
	if _, err := k.Append(ctx, "entdb-wal", "tenant_a", []byte(`{"op":"create"}`), hdrs); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := k.PollBatch(ctx, "entdb-wal", "g1", 10, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if string(got[0].Value) != `{"op":"create"}` {
		t.Fatalf("value = %q", got[0].Value)
	}
	if string(got[0].Headers[HeaderIdempotencyKey]) != "uuid-1" {
		t.Fatalf("idempotency header = %q", got[0].Headers[HeaderIdempotencyKey])
	}
	if string(got[0].Headers["x-trace"]) != "abc" {
		t.Fatalf("x-trace header = %q", got[0].Headers["x-trace"])
	}
}

func TestKinesisIdempotentRetry(t *testing.T) {
	fake := newFakeKinesis("entdb-wal")
	k := newTestKinesis(t, fake)
	ctx := context.Background()

	h := map[string][]byte{HeaderIdempotencyKey: []byte("dedupe-1")}
	pos1, err := k.Append(ctx, "entdb-wal", "tenant_a", []byte("v1"), h)
	if err != nil {
		t.Fatalf("Append #1: %v", err)
	}
	pos2, err := k.Append(ctx, "entdb-wal", "tenant_a", []byte("v2"), h)
	if err != nil {
		t.Fatalf("Append #2: %v", err)
	}
	if pos1 != pos2 {
		t.Fatalf("idempotent retry produced different pos: %+v vs %+v", pos1, pos2)
	}
	if n := len(fake.records); n != 1 {
		t.Fatalf("fake recorded %d puts, want 1 (dedupe)", n)
	}
}

func TestKinesisCommitAdvancesCheckpoint(t *testing.T) {
	fake := newFakeKinesis("entdb-wal")
	k := newTestKinesis(t, fake)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := k.Append(ctx, "entdb-wal", "tenant_a", []byte(fmt.Sprintf("v%d", i)), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	batch, err := k.PollBatch(ctx, "entdb-wal", "g1", 3, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(batch) != 3 {
		t.Fatalf("got %d, want 3", len(batch))
	}
	// Commit the 2nd record, drop cached iterators, and re-poll: the
	// iterator should be rebuilt AFTER_SEQUENCE_NUMBER of record #2,
	// yielding only record #3.
	if err := k.Commit(ctx, "g1", batch[1]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	k.mu.Lock()
	k.shardIters = map[string]string{}
	k.mu.Unlock()

	rest, err := k.PollBatch(ctx, "entdb-wal", "g1", 10, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2: %v", err)
	}
	if len(rest) != 1 {
		t.Fatalf("got %d after commit, want 1", len(rest))
	}
	if string(rest[0].Value) != "v2" {
		t.Fatalf("resumed at %q, want v2", rest[0].Value)
	}
}

func TestKinesisExpiredIteratorRecovers(t *testing.T) {
	fake := newFakeKinesis("entdb-wal")
	k := newTestKinesis(t, fake)
	ctx := context.Background()

	if _, err := k.Append(ctx, "entdb-wal", "tenant_a", []byte("v0"), nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	fake.mu.Lock()
	fake.expireOne = true
	fake.mu.Unlock()

	got, err := k.PollBatch(ctx, "entdb-wal", "g1", 10, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 1 || string(got[0].Value) != "v0" {
		t.Fatalf("expired-iterator recovery failed: %+v", got)
	}
}

func TestKinesisPollTimeoutEmpty(t *testing.T) {
	fake := newFakeKinesis("entdb-wal")
	k := newTestKinesis(t, fake)
	start := time.Now()
	got, err := k.PollBatch(context.Background(), "entdb-wal", "g1", 5, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d, want 0", len(got))
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("returned too fast: %v", time.Since(start))
	}
}

func TestKinesisExplicitHashKeyDeterministic(t *testing.T) {
	a := explicitHashKey("tenant_a")
	for i := 0; i < 50; i++ {
		if explicitHashKey("tenant_a") != a {
			t.Fatal("explicitHashKey not deterministic")
		}
	}
	if explicitHashKey("tenant_b") == a {
		t.Fatal("distinct tenants hashed to same key (unexpected collision in test)")
	}
}

func TestParseShardNumber(t *testing.T) {
	cases := map[string]int32{
		"shardId-000000000003": 3,
		"shardId-000000000000": 0,
		"shardId-42":           42,
		"":                     0,
		"garbage":              0,
	}
	for in, want := range cases {
		if got := parseShardNumber(in); got != want {
			t.Fatalf("parseShardNumber(%q) = %d, want %d", in, got, want)
		}
	}
}
