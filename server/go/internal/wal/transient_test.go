// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/IBM/sarama"
)

// fakeTimeoutErr implements net.Error with Timeout()==true, mirroring a
// "read tcp ...: i/o timeout" surfaced by sarama on idle-connection
// reaping (issue #627).
type fakeTimeoutErr struct{}

func (fakeTimeoutErr) Error() string   { return "read tcp 10.0.8.4:52494->broker:9093: i/o timeout" }
func (fakeTimeoutErr) Timeout() bool   { return true }
func (fakeTimeoutErr) Temporary() bool { return true }

func TestIsTransientConsumerErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"io.EOF (idle-connection reap)", io.EOF, true},
		{"wrapped io.EOF", fmt.Errorf("consume: %w", io.EOF), true},
		{"net timeout", fakeTimeoutErr{}, true},
		{"out of brokers (leader election)", sarama.ErrOutOfBrokers, true},
		{"unknown topic/partition (rebalance)", sarama.ErrUnknownTopicOrPartition, true},
		{"leader not available", sarama.ErrLeaderNotAvailable, true},
		{"ConsumerError wrapping EOF", &sarama.ConsumerError{Topic: "entdb-wal", Partition: 2, Err: io.EOF}, true},
		{"generic error", errors.New("boom"), false},
		{"sasl/auth-like error", errors.New("kafka: SASL authentication failed"), false},
		{"closed consumer group (shutdown, not transient)", sarama.ErrClosedConsumerGroup, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := isTransientConsumerErr(tc.err); got != tc.want {
			t.Errorf("%s: isTransientConsumerErr(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestWrapPollErrTagsTransient(t *testing.T) {
	// A transient cause is tagged ErrTransient AND ErrWal.
	transient := wrapPollErr(io.EOF)
	if !IsTransient(transient) {
		t.Errorf("wrapPollErr(io.EOF) should be transient: %v", transient)
	}
	if !errors.Is(transient, ErrWal) {
		t.Errorf("wrapPollErr(io.EOF) should also be ErrWal: %v", transient)
	}

	// A fatal cause is ErrWal but NOT ErrTransient.
	fatal := wrapPollErr(errors.New("kafka: SASL authentication failed"))
	if IsTransient(fatal) {
		t.Errorf("wrapPollErr(auth error) must NOT be transient: %v", fatal)
	}
	if !errors.Is(fatal, ErrWal) {
		t.Errorf("wrapPollErr(auth error) should be ErrWal: %v", fatal)
	}
}
