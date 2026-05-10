// SPDX-License-Identifier: AGPL-3.0-only

// Package wal hosts the write-ahead-log producer/consumer
// abstractions ported from server/python/entdb_server/wal/. The WAL
// is the source of truth for all mutations (CLAUDE.md invariant #1);
// SQLite is a materialized view rebuilt by replaying it.
//
// Spec: docs/go-port/shared/wal-producer.md. Phase 1 of EPIC #407
// only ships the in-memory backend (sufficient to bring up the Go
// handlers + applier behind it and run the integration suite). Real
// backends (Kafka, Kinesis, SQS, Pub/Sub, Service Bus, Event Hubs)
// land in Phase 2 as their own sub-packages.
//
// The producer contract this package enforces:
//
//   - Append returns only after the record is durably stored.
//   - Per-partition (and therefore per-tenant, since handlers always
//     key by tenant_id) total order.
//   - Idempotent retries: appending with the same Event.IdempotencyKey
//     within the same topic returns the original StreamPos and does
//     not create a duplicate record.
//   - No partial writes: Append either returns a StreamPos or a typed
//     error (ErrConnection, ErrTimeout, ErrSerialization, or a wrapped
//     ErrWal).
//
// The consumer contract:
//
//   - Subscribe yields records in offset order within a partition.
//   - PollBatch returns up to maxRecords currently available; it does
//     not block the producer (decoupled queues, not request/response).
//   - Commit advances the stored offset for (topic, group_id) so
//     subsequent PollBatch / Subscribe calls resume after the
//     committed record.
package wal

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors. Mirrors server/python/entdb_server/wal/base.py:41
// (WalError, WalConnectionError, WalTimeoutError,
// WalSerializationError). Wrap with fmt.Errorf("...: %w", ErrFoo) and
// callers check via errors.Is.
var (
	// ErrWal is the umbrella error for WAL operations. Other sentinels
	// in this package wrap this so errors.Is(err, ErrWal) is true for
	// every WAL-origin failure.
	ErrWal = errors.New("wal: error")

	// ErrConnection signals the WAL backend is not reachable. Returned
	// from Append/Subscribe/PollBatch/Commit when the producer or
	// consumer has not been Connect()ed (or Close()d in flight).
	ErrConnection = errors.New("wal: connection error")

	// ErrTimeout signals an operation exceeded its deadline. Honors
	// ctx.Err() in callers; this sentinel is returned when the failure
	// is a backend-side timeout rather than caller cancellation.
	ErrTimeout = errors.New("wal: timeout")

	// ErrSerialization signals an event payload could not be encoded
	// or decoded. Mirrors WalSerializationError in the Python tree.
	ErrSerialization = errors.New("wal: serialization error")
)

// HeaderIdempotencyKey is the reserved header key under which Append
// implementations look for the application-layer idempotency key.
// Producers wishing to use Append's idempotent-retry guarantee MUST
// set this header (mirrors the Python applier-layer dedupe in
// applier.py:765 against TransactionEvent.idempotency_key).
const HeaderIdempotencyKey = "idempotency-key"

// Producer is the producer half of the WAL stream. The Append byte
// signature mirrors docs/go-port/shared/wal-producer.md "Go interface"
// section: encoding lives in a thin wrapper (Event.Encode), the
// producer stays transport-only.
type Producer interface {
	// Connect opens the producer's underlying transport. Implementations
	// that have no setup (in-memory) may treat this as a no-op but must
	// still flip an internal "connected" flag so subsequent Append
	// calls succeed.
	Connect(ctx context.Context) error

	// Close releases producer resources. After Close, Append must
	// return ErrConnection. Calling Close twice is a no-op.
	Close(ctx context.Context) error

	// Append durably stores value under key in topic. Returns the
	// StreamPos receipt. Headers are optional; pass nil if unused.
	//
	// Idempotency: implementations MUST treat (topic, key,
	// idempotencyKey) as a dedupe identity. The idempotency key is
	// taken from the headers map under HeaderIdempotencyKey -- callers
	// should set it (typically by passing through Event.IdempotencyKey
	// at the encode boundary). If the same key has already been
	// appended for the same (topic, key) pair, Append returns the
	// previously-issued StreamPos and does not write a new record.
	Append(ctx context.Context, topic, key string, value []byte, headers map[string][]byte) (StreamPos, error)
}

// Consumer is the consumer half of the WAL stream.
type Consumer interface {
	// Subscribe yields records from topic for the given consumer group.
	// Records are delivered in offset order within a partition. The
	// returned channel is closed when ctx is cancelled or the consumer
	// is closed; the error channel surfaces a single terminal error
	// (or is closed cleanly).
	//
	// Callers must Commit each record to advance the consumer group's
	// stored offset. Uncommitted records are re-delivered on the next
	// Subscribe / PollBatch call.
	Subscribe(ctx context.Context, topic, groupID string) (<-chan Record, <-chan error, error)

	// PollBatch returns up to maxRecords currently available for
	// (topic, groupID). If no records are available it blocks up to
	// timeout (or ctx deadline, whichever fires first) waiting for new
	// records, then returns whatever showed up. Returning an empty
	// slice with a nil error is a normal "no records yet" signal and
	// is NOT an error.
	PollBatch(ctx context.Context, topic, groupID string, maxRecords int, timeout time.Duration) ([]Record, error)

	// Commit advances the stored offset for (topic, groupID) past
	// record.Position. Subsequent PollBatch / Subscribe calls for the
	// same group resume after this position.
	Commit(ctx context.Context, groupID string, record Record) error
}
