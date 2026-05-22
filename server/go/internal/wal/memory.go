// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// DefaultNumPartitions is the default partition count for the in-memory
// WAL stream. Contract tests rely on a stable default — do not change
// without updating those tests.
const DefaultNumPartitions = 4

// InMemory is the in-memory implementation of Producer + Consumer.
//
// Guarantees:
//
//   - Per-tenant total order: records appended with the same key land
//     in the same partition (md5(key) % numPartitions) and partitions
//     are append-only with monotonically-increasing offsets.
//   - Idempotent Append: the (topic, key, HeaderIdempotencyKey) tuple
//     is the dedupe identity. A duplicate Append returns the original
//     StreamPos without writing a new record.
//   - Test helpers (GetAllRecords, ClearTopic, WaitForRecords) are
//     loud-and-public so cross-impl contract tests can introspect the
//     log without dropping into package-internal access.
//
// The zero value is NOT usable; construct via NewInMemory.
type InMemory struct {
	numPartitions int

	mu     sync.Mutex
	cond   *sync.Cond
	closed bool

	// topics[topic][partition] -> ordered list of records.
	topics map[string]map[int32][]Record

	// committed[groupID][topic][partition] -> next offset to deliver.
	committed map[string]map[string]map[int32]int64

	// idemp[topic][key][idempotencyKey] -> previously-issued StreamPos.
	// Scope is (topic, key) so two tenants reusing the same uuid for
	// distinct events (unlikely but legal) do not collide.
	idemp map[string]map[string]map[string]StreamPos
}

// NewInMemory constructs an InMemory backend with numPartitions
// partitions per topic. Pass 0 (or any value < 1) to use
// DefaultNumPartitions.
func NewInMemory(numPartitions int) *InMemory {
	if numPartitions < 1 {
		numPartitions = DefaultNumPartitions
	}
	m := &InMemory{
		numPartitions: numPartitions,
		topics:        make(map[string]map[int32][]Record),
		committed:     make(map[string]map[string]map[int32]int64),
		idemp:         make(map[string]map[string]map[string]StreamPos),
	}
	m.cond = sync.NewCond(&m.mu)
	return m
}

// Connect is a no-op for the in-memory backend; it exists to satisfy
// the Producer/Consumer interface contract.
func (m *InMemory) Connect(ctx context.Context) error {
	return nil
}

// Close marks the backend as closed and wakes any blocked PollBatch
// or Subscribe consumers so they can observe ctx.Done() / the closed
// flag and return.
func (m *InMemory) Close(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.cond.Broadcast()
	return nil
}

// Append writes value to topic under key. See Producer.Append for the
// full contract; this implementation enforces:
//
//   - Per-tenant order via partitionFor(key).
//   - Idempotency via the HeaderIdempotencyKey header.
//   - No partial writes: the lock is held for the read-or-insert
//     critical section, so a concurrent retry sees the original
//     position rather than racing past it.
func (m *InMemory) Append(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string][]byte,
) (StreamPos, error) {
	if err := ctx.Err(); err != nil {
		return StreamPos{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return StreamPos{}, fmt.Errorf("%w: closed", ErrConnection)
	}

	// Application-layer idempotent retry: we dedupe at append time so
	// callers can safely retry without polluting the log (per PLAN.md).
	idempKey := ""
	if h, ok := headers[HeaderIdempotencyKey]; ok && len(h) > 0 {
		idempKey = string(h)
	}
	if idempKey != "" {
		if byTopic, ok := m.idemp[topic]; ok {
			if byKey, ok := byTopic[key]; ok {
				if pos, ok := byKey[idempKey]; ok {
					return pos, nil
				}
			}
		}
	}

	partition := m.partitionFor(key)

	parts, ok := m.topics[topic]
	if !ok {
		parts = make(map[int32][]Record, m.numPartitions)
		m.topics[topic] = parts
	}
	offset := int64(len(parts[partition]))

	// Copy headers so the caller mutating their map after Append does
	// not corrupt the stored record.
	storedHeaders := copyHeaders(headers)
	// Copy value bytes for the same reason.
	storedValue := append([]byte(nil), value...)

	pos := StreamPos{
		Topic:       topic,
		Partition:   partition,
		Offset:      offset,
		TimestampMs: time.Now().UnixMilli(),
	}

	rec := Record{
		Key:      key,
		Value:    storedValue,
		Position: pos,
		Headers:  storedHeaders,
	}
	parts[partition] = append(parts[partition], rec)

	if idempKey != "" {
		byTopic, ok := m.idemp[topic]
		if !ok {
			byTopic = make(map[string]map[string]StreamPos)
			m.idemp[topic] = byTopic
		}
		byKey, ok := byTopic[key]
		if !ok {
			byKey = make(map[string]StreamPos)
			byTopic[key] = byKey
		}
		byKey[idempKey] = pos
	}

	m.cond.Broadcast()
	return pos, nil
}

// PollBatch returns up to maxRecords currently uncommitted records
// for (topic, groupID). Walks partitions in deterministic order so
// repeated calls with the same input produce the same output for an
// otherwise-quiet log. If nothing is available, blocks until a new
// record arrives, the timeout fires, or ctx is cancelled.
func (m *InMemory) PollBatch(
	ctx context.Context,
	topic, groupID string,
	maxRecords int,
	timeout time.Duration,
) ([]Record, error) {
	if maxRecords <= 0 {
		return nil, nil
	}

	deadline := time.Now().Add(timeout)

	m.mu.Lock()
	defer m.mu.Unlock()

	for {
		if m.closed {
			return nil, fmt.Errorf("%w: closed", ErrConnection)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		records := m.collectAvailableLocked(topic, groupID, maxRecords)
		if len(records) > 0 {
			return records, nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, nil
		}
		m.waitWithDeadline(ctx, deadline)
	}
}

// Subscribe yields records as they arrive. The returned channels are
// closed when ctx is cancelled, the backend is closed, or a fatal
// error occurs. Records are delivered in offset order within a
// partition; partitions are walked round-robin so a busy tenant
// cannot starve a quiet one.
func (m *InMemory) Subscribe(
	ctx context.Context,
	topic, groupID string,
) (<-chan Record, <-chan error, error) {
	out := make(chan Record)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)
		for {
			if err := ctx.Err(); err != nil {
				return
			}
			records, err := m.PollBatch(ctx, topic, groupID, 32, 100*time.Millisecond)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				errCh <- err
				return
			}
			for _, r := range records {
				select {
				case <-ctx.Done():
					return
				case out <- r:
					// Subscribe auto-commits per record. Callers wanting
					// at-least-once semantics with manual commits should
					// use PollBatch + Commit directly.
					_ = m.commitLocked(groupID, r)
				}
			}
		}
	}()

	return out, errCh, nil
}

// Commit advances the stored offset for (topic, groupID) to one past
// record.Position.Offset. Keyed by groupID so two consumer groups can
// independently iterate the same topic.
func (m *InMemory) Commit(ctx context.Context, groupID string, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.commitLocked(groupID, record)
}

func (m *InMemory) commitLocked(groupID string, record Record) error {
	byGroup, ok := m.committed[groupID]
	if !ok {
		byGroup = make(map[string]map[int32]int64)
		m.committed[groupID] = byGroup
	}
	byTopic, ok := byGroup[record.Position.Topic]
	if !ok {
		byTopic = make(map[int32]int64)
		byGroup[record.Position.Topic] = byTopic
	}
	next := record.Position.Offset + 1
	if cur := byTopic[record.Position.Partition]; next > cur {
		byTopic[record.Position.Partition] = next
	}
	return nil
}

// collectAvailableLocked walks partitions in id order and returns up
// to maxRecords records that have offset >= committed offset for
// (topic, groupID). Caller must hold m.mu.
func (m *InMemory) collectAvailableLocked(topic, groupID string, maxRecords int) []Record {
	parts, ok := m.topics[topic]
	if !ok {
		return nil
	}
	var byTopic map[int32]int64
	if g, ok := m.committed[groupID]; ok {
		byTopic = g[topic]
	}

	out := make([]Record, 0, maxRecords)
	for p := int32(0); p < int32(m.numPartitions); p++ {
		recs := parts[p]
		if len(recs) == 0 {
			continue
		}
		start := byTopic[p]
		for i := start; i < int64(len(recs)) && len(out) < maxRecords; i++ {
			out = append(out, recs[i])
		}
		if len(out) >= maxRecords {
			break
		}
	}
	return out
}

// waitWithDeadline parks the caller on m.cond until either a
// Broadcast wakes it, ctx fires, or deadline elapses. m.mu must be
// held; on return it is still held. The implementation spawns a
// helper goroutine to broadcast on ctx/deadline so we do not deadlock
// on a quiet log.
func (m *InMemory) waitWithDeadline(ctx context.Context, deadline time.Time) {
	done := make(chan struct{})
	defer close(done)

	go func() {
		t := time.NewTimer(time.Until(deadline))
		defer t.Stop()
		select {
		case <-done:
		case <-ctx.Done():
			m.cond.Broadcast()
		case <-t.C:
			m.cond.Broadcast()
		}
	}()

	m.cond.Wait()
}

func (m *InMemory) partitionFor(key string) int32 {
	sum := md5.Sum([]byte(key))
	// md5(key)[:4] big-endian uint32 mod numPartitions.
	hashInt := binary.BigEndian.Uint32(sum[:4])
	return int32(hashInt % uint32(m.numPartitions))
}

// PartitionFor exposes the partition a WAL key routes to. It is the
// exact mapping Append uses to place records, so external callers
// (notably the ADR-027 same-partition poison test in
// internal/apply) can confirm two tenant ids collide on one partition
// without reaching into unexported internals.
func (m *InMemory) PartitionFor(key string) int32 {
	return m.partitionFor(key)
}

func copyHeaders(in map[string][]byte) map[string][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		cp := append([]byte(nil), v...)
		out[k] = cp
	}
	return out
}

// --- Test helpers ---

// GetAllRecords returns every record in topic, in (partition, offset)
// order. Cross-impl contract tests rely on this for log introspection.
func (m *InMemory) GetAllRecords(topic string) []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	parts, ok := m.topics[topic]
	if !ok {
		return nil
	}
	var out []Record
	for p := int32(0); p < int32(m.numPartitions); p++ {
		out = append(out, parts[p]...)
	}
	return out
}

// GetRecordCount returns the total number of records across all
// partitions of topic.
func (m *InMemory) GetRecordCount(topic string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	parts, ok := m.topics[topic]
	if !ok {
		return 0
	}
	total := 0
	for _, recs := range parts {
		total += len(recs)
	}
	return total
}

// ClearTopic drops every record in topic and resets per-group
// committed offsets for it. Useful between test cases that share a
// backend instance.
func (m *InMemory) ClearTopic(topic string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.topics, topic)
	delete(m.idemp, topic)
	for _, byGroup := range m.committed {
		delete(byGroup, topic)
	}
}

// WaitForRecords blocks until topic has at least count records or
// ctx is cancelled. Returns true on success, false on ctx timeout /
// cancel.
func (m *InMemory) WaitForRecords(ctx context.Context, topic string, count int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		if m.closed {
			return m.recordCountLocked(topic) >= count
		}
		if m.recordCountLocked(topic) >= count {
			return true
		}
		if err := ctx.Err(); err != nil {
			return false
		}
		// Re-check every 50ms in case Append is happening on another
		// goroutine that didn't broadcast yet (defensive — Append does
		// broadcast, but we don't want a slow test to hang).
		deadline := time.Now().Add(50 * time.Millisecond)
		m.waitWithDeadline(ctx, deadline)
	}
}

func (m *InMemory) recordCountLocked(topic string) int {
	parts, ok := m.topics[topic]
	if !ok {
		return 0
	}
	total := 0
	for _, recs := range parts {
		total += len(recs)
	}
	return total
}
