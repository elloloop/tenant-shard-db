// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
)

// fakeSQS is an in-memory FIFO-queue stand-in implementing SqsAPI.
// Dedup is enforced on MessageDeduplicationId (SQS FIFO semantics);
// delivered-but-uncommitted messages stay visible (the visibility
// timeout is not simulated — tests commit explicitly).
type fakeSQS struct {
	mu      sync.Mutex
	queueOK map[string]bool
	msgs    []*fakeSQSMsg
	dedup   map[string]bool
	idSeq   int
	sendErr error
	recvErr error
}

type fakeSQSMsg struct {
	id       string
	receipt  string
	body     string
	group    string
	attrs    map[string]string
	deleted  bool
	inflight bool // hidden by the (simulated) visibility timeout until released
	sentMs   int64
}

// releaseInflight makes all in-flight (received-but-not-committed)
// messages visible again, simulating the SQS visibility timeout
// elapsing. Tests call this to exercise redelivery.
func (f *fakeSQS) releaseInflight() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if !m.deleted {
			m.inflight = false
		}
	}
}

func newFakeSQS(queueURL string) *fakeSQS {
	return &fakeSQS{
		queueOK: map[string]bool{queueURL: true},
		dedup:   map[string]bool{},
	}
}

func (f *fakeSQS) SendMessage(ctx context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	dedupID := aws.ToString(in.MessageDeduplicationId)
	if dedupID != "" && f.dedup[dedupID] {
		// SQS FIFO silently dedupes within the dedup window.
		return &sqs.SendMessageOutput{MessageId: aws.String("dup")}, nil
	}
	f.dedup[dedupID] = true
	f.idSeq++
	id := fmt.Sprintf("msg-%d", f.idSeq)
	attrs := map[string]string{}
	for k, v := range in.MessageAttributes {
		attrs[k] = aws.ToString(v.StringValue)
	}
	f.msgs = append(f.msgs, &fakeSQSMsg{
		id:      id,
		receipt: "rh-" + id,
		body:    aws.ToString(in.MessageBody),
		group:   aws.ToString(in.MessageGroupId),
		attrs:   attrs,
		sentMs:  time.Now().UnixMilli(),
	})
	return &sqs.SendMessageOutput{MessageId: aws.String(id)}, nil
}

func (f *fakeSQS) ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	limit := int(in.MaxNumberOfMessages)
	out := []sqstypes.Message{}
	for _, m := range f.msgs {
		if m.deleted || m.inflight {
			continue
		}
		if len(out) >= limit {
			break
		}
		// SQS hides a received message until its visibility timeout or
		// until it is deleted (committed). Model that here.
		m.inflight = true
		ma := map[string]sqstypes.MessageAttributeValue{}
		for k, v := range m.attrs {
			ma[k] = sqstypes.MessageAttributeValue{StringValue: aws.String(v), DataType: aws.String("String")}
		}
		out = append(out, sqstypes.Message{
			MessageId:         aws.String(m.id),
			ReceiptHandle:     aws.String(m.receipt),
			Body:              aws.String(m.body),
			MessageAttributes: ma,
			Attributes: map[string]string{
				string(sqstypes.MessageSystemAttributeNameMessageGroupId): m.group,
				string(sqstypes.MessageSystemAttributeNameSentTimestamp):  strconv.FormatInt(m.sentMs, 10),
			},
		})
	}
	return &sqs.ReceiveMessageOutput{Messages: out}, nil
}

func (f *fakeSQS) DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if m.receipt == aws.ToString(in.ReceiptHandle) {
			m.deleted = true
			return &sqs.DeleteMessageOutput{}, nil
		}
	}
	return &sqs.DeleteMessageOutput{}, nil
}

func (f *fakeSQS) GetQueueAttributes(ctx context.Context, in *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.queueOK[aws.ToString(in.QueueUrl)] {
		return nil, &smithy.GenericAPIError{Code: "AWS.SimpleQueueService.NonExistentQueue", Message: "no queue"}
	}
	return &sqs.GetQueueAttributesOutput{Attributes: map[string]string{"QueueArn": "arn:fake"}}, nil
}

func newTestSqs(t *testing.T, fake *fakeSQS, url string) *Sqs {
	t.Helper()
	s := NewSqs(DefaultSqsConfig(url, "us-east-1"))
	s.config.WaitTimeSeconds = 0
	s.newClient = func(ctx context.Context) (SqsAPI, error) { return fake, nil }
	if err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s
}

func TestSqsConnectMissingQueue(t *testing.T) {
	s := NewSqs(DefaultSqsConfig("", "r"))
	if err := s.Connect(context.Background()); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestSqsConnectQueueNotFound(t *testing.T) {
	fake := newFakeSQS("https://sqs/other.fifo")
	s := NewSqs(DefaultSqsConfig("https://sqs/entdb.fifo", "r"))
	s.newClient = func(ctx context.Context) (SqsAPI, error) { return fake, nil }
	if err := s.Connect(context.Background()); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestSqsAppendNotConnected(t *testing.T) {
	s := NewSqs(DefaultSqsConfig("u", "r"))
	if _, err := s.Append(context.Background(), "u", "k", []byte("v"), nil); !errors.Is(err, ErrConnection) {
		t.Fatalf("err = %v, want ErrConnection", err)
	}
}

func TestSqsAppendPollCommitRoundTrip(t *testing.T) {
	url := "https://sqs/entdb.fifo"
	fake := newFakeSQS(url)
	s := newTestSqs(t, fake, url)
	ctx := context.Background()

	if _, err := s.Append(ctx, url, "tenant_a", []byte("hello"), map[string][]byte{"x-h": []byte("v")}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.PollBatch(ctx, url, "g1", 10, 300*time.Millisecond)
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
		t.Fatalf("key = %q, want tenant_a (MessageGroupId)", got[0].Key)
	}
	if string(got[0].Headers["x-h"]) != "v" {
		t.Fatalf("header x-h = %q", got[0].Headers["x-h"])
	}

	// Uncommitted -> hidden by visibility timeout, then redelivered
	// once the timeout elapses (simulated by releaseInflight).
	empty, err := s.PollBatch(ctx, url, "g1", 10, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #2 (in-flight): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("in-flight msg should be hidden, got %d", len(empty))
	}
	fake.releaseInflight()
	again, err := s.PollBatch(ctx, url, "g1", 10, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #3: %v", err)
	}
	if len(again) != 1 {
		t.Fatalf("uncommitted msg not redelivered after visibility timeout: got %d", len(again))
	}

	if err := s.Commit(ctx, "g1", again[0]); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	none, err := s.PollBatch(ctx, url, "g1", 10, 150*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch #3: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("committed msg redelivered: got %d", len(none))
	}
}

func TestSqsIdempotentRetry(t *testing.T) {
	url := "https://sqs/entdb.fifo"
	fake := newFakeSQS(url)
	s := newTestSqs(t, fake, url)
	ctx := context.Background()

	h := map[string][]byte{HeaderIdempotencyKey: []byte("uuid-1")}
	p1, err := s.Append(ctx, url, "tenant_a", []byte("v1"), h)
	if err != nil {
		t.Fatalf("Append #1: %v", err)
	}
	p2, err := s.Append(ctx, url, "tenant_a", []byte("v2"), h)
	if err != nil {
		t.Fatalf("Append #2: %v", err)
	}
	if p1 != p2 {
		t.Fatalf("idempotent retry differs: %+v vs %+v", p1, p2)
	}
	// Only one message physically enqueued (in-process cache short-
	// circuits before the SDK call).
	got, err := s.PollBatch(ctx, url, "g1", 10, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d messages, want 1 (dedupe)", len(got))
	}
}

func TestSqsPerTenantOrder(t *testing.T) {
	url := "https://sqs/entdb.fifo"
	fake := newFakeSQS(url)
	s := newTestSqs(t, fake, url)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := s.Append(ctx, url, "tenant_a", []byte(fmt.Sprintf("v%d", i)), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := s.PollBatch(ctx, url, "g1", 10, 300*time.Millisecond)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d, want 5", len(got))
	}
	for i, r := range got {
		if want := fmt.Sprintf("v%d", i); string(r.Value) != want {
			t.Fatalf("record %d = %q, want %q (FIFO order broken)", i, r.Value, want)
		}
	}
}

func TestSqsReceiveErrorClassified(t *testing.T) {
	url := "https://sqs/entdb.fifo"
	fake := newFakeSQS(url)
	s := newTestSqs(t, fake, url)
	fake.mu.Lock()
	fake.recvErr = &smithy.GenericAPIError{Code: "InternalError", Message: "boom"}
	fake.mu.Unlock()
	_, err := s.PollBatch(context.Background(), url, "g1", 5, 150*time.Millisecond)
	if !errors.Is(err, ErrWal) {
		t.Fatalf("err = %v, want ErrWal", err)
	}
}
