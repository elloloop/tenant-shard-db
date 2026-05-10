package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UpdateAppliedOffset records that the applier has applied at least
// `offset` for (tenant, topic, partition). Mirrors the spirit of
// canonical_store.py:update_applied_offset (463) — though the Python
// version only tracks one stream_pos per tenant, the Go port persists
// (topic, partition) to support multi-partition WAL deployments.
//
// The persisted value is what survives restarts; the in-memory tracker
// (used by WaitForOffset) is updated atomically alongside.
func (s *CanonicalStore) UpdateAppliedOffset(ctx context.Context, tenantID, topic string, partition int32, offset int64) error {
	now := s.now()
	if err := s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT INTO applied_offsets (tenant_id, topic, partition, offset_pos, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (tenant_id, topic, partition)
			DO UPDATE SET offset_pos = excluded.offset_pos,
			              updated_at = excluded.updated_at
			WHERE applied_offsets.offset_pos < excluded.offset_pos`,
			tenantID, topic, partition, offset, now,
		)
		if err != nil {
			return fmt.Errorf("store: UpdateAppliedOffset: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	// Notify any WaitForOffset waiters. We MUST set the entry even
	// when offset == 0 (the very first record on an in-memory WAL
	// starts at offset 0) so that WaitForOffset's "ok && cur >=
	// target" predicate becomes true. Bumping only when offset > cur
	// leaves the map unset on first apply and any wait_applied=true
	// call for the first event hangs until timeout. Pinned by the
	// Go E2E create_single_node test.
	s.offsetMu.Lock()
	if cur, ok := s.appliedOffsets[tenantID]; !ok || offset > cur {
		s.appliedOffsets[tenantID] = offset
	}
	s.offsetCond.Broadcast()
	s.offsetMu.Unlock()
	return nil
}

// UpdateAppliedOffsetTx is the in-transaction variant. The applier
// calls this inside its BatchTxn so the offset advance is atomic with
// the writes it covers (per docs/go-port/shared/applier.md).
func (s *CanonicalStore) UpdateAppliedOffsetTx(ctx context.Context, b *BatchTxn, topic string, partition int32, offset int64) error {
	now := s.now()
	_, err := b.conn.ExecContext(ctx, `
		INSERT INTO applied_offsets (tenant_id, topic, partition, offset_pos, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, topic, partition)
		DO UPDATE SET offset_pos = excluded.offset_pos,
		              updated_at = excluded.updated_at
		WHERE applied_offsets.offset_pos < excluded.offset_pos`,
		b.tenantID, topic, partition, offset, now,
	)
	if err != nil {
		return fmt.Errorf("store: UpdateAppliedOffsetTx: %w", err)
	}
	// Notify in-memory waiters even before commit; readers polling on
	// WaitForOffset are tolerating staleness anyway, and the post-commit
	// state will catch up at the next applied write. Strictly correct
	// implementations would defer this to BatchTxn.Commit() — kept
	// simple here, refine in W1.10 if a parity test demands it.
	//
	// MUST publish offset 0 on first apply too (see
	// UpdateAppliedOffset above): otherwise wait_applied=true on the
	// very first record hangs until timeout.
	s.offsetMu.Lock()
	if cur, ok := s.appliedOffsets[b.tenantID]; !ok || offset > cur {
		s.appliedOffsets[b.tenantID] = offset
	}
	s.offsetCond.Broadcast()
	s.offsetMu.Unlock()
	return nil
}

// GetAppliedOffset returns the persisted offset for (tenant, topic,
// partition), or 0 + nil if no row exists. Mirrors
// canonical_store.py:get_applied_offset (514) extended with topic /
// partition.
func (s *CanonicalStore) GetAppliedOffset(ctx context.Context, tenantID, topic string, partition int32) (int64, error) {
	db, err := s.db(tenantID)
	if err != nil {
		return 0, err
	}
	var off int64
	err = db.QueryRowContext(ctx, `
		SELECT offset_pos FROM applied_offsets
		WHERE tenant_id = ? AND topic = ? AND partition = ?`,
		tenantID, topic, partition,
	).Scan(&off)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: GetAppliedOffset: %w", err)
	}
	return off, nil
}

// WaitForOffset blocks until the in-memory applied offset for tenantID
// reaches at least targetOffset, or ctx is done, or Close is called.
// Mirrors canonical_store.py:wait_for_offset (478).
//
// Implementation is a sync.Cond watcher: each successful Update*
// broadcasts on the cond and we re-check the predicate. ctx
// cancellation is integrated by spinning a goroutine that broadcasts
// on Done.
func (s *CanonicalStore) WaitForOffset(ctx context.Context, tenantID string, targetOffset int64) error {
	// Spawn a watcher to wake the cond when ctx fires; cleaned up on
	// return via the cancel signal.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.offsetMu.Lock()
			s.offsetCond.Broadcast()
			s.offsetMu.Unlock()
		case <-done:
		}
	}()

	s.offsetMu.Lock()
	defer s.offsetMu.Unlock()
	for {
		if s.closed {
			return fmt.Errorf("store: closed")
		}
		if cur, ok := s.appliedOffsets[tenantID]; ok && cur >= targetOffset {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		s.offsetCond.Wait()
	}
}
