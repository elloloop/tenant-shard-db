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
	// withWrite has already COMMITted the offset row above, so the
	// data backing this offset is durable and visible on every
	// connection (including the read pool) before we wake any
	// WaitForOffset waiter. This is the non-txn / global apply path
	// (see internal/apply/global.go); it is correct as-is because the
	// notify happens strictly post-commit. The in-txn applier path uses
	// UpdateAppliedOffsetTx + BatchTxn.Commit instead (ADR-026).
	s.notifyOffset(tenantID, offset)
	return nil
}

// notifyOffset advances the in-memory applied-offset tracker for
// tenantID and wakes every WaitForOffset waiter. It MUST only be
// called once the data backing `offset` is committed and visible on
// any SQLite connection — pre-commit notification is the read-after-
// write regression ADR-026 condition 1 closes.
//
// We MUST set the entry even when offset == 0 (the very first record
// on an in-memory WAL starts at offset 0) so that WaitForOffset's
// "ok && cur >= target" predicate becomes true. Bumping only when
// offset > cur leaves the map unset on first apply and any
// wait_applied=true call for the first event hangs until timeout.
// Pinned by the Go E2E create_single_node test.
func (s *CanonicalStore) notifyOffset(tenantID string, offset int64) {
	s.offsetMu.Lock()
	if cur, ok := s.appliedOffsets[tenantID]; !ok || offset > cur {
		s.appliedOffsets[tenantID] = offset
	}
	s.offsetCond.Broadcast()
	s.offsetMu.Unlock()
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
	// ADR-026 condition 1 (root-cause fix; also fixes the pre-existing
	// latent read-after-write bug): the offset row is written INSIDE
	// the applier's BatchTxn (above) so the advance is atomic with the
	// data it covers, but the in-memory tracker + cond Broadcast that
	// wakes WaitForOffset(N) waiters MUST NOT fire until tx.Commit()
	// succeeds. Notifying before COMMIT lets a woken reader on an
	// independent connection (the #137 read pool) take a WAL snapshot
	// that excludes the still-uncommitted write and read the client's
	// own confirmed write back as stale / Found=false.
	//
	// We stash the pending notification on the BatchTxn; BatchTxn.Commit
	// publishes it via notifyOffset only after the SQL COMMIT returns
	// (a no-op idempotency commit / Rollback intentionally publishes
	// nothing — no data became visible).
	b.pendingOffset = &pendingOffsetNotify{tenantID: b.tenantID, offset: offset}
	return nil
}

// GetAppliedOffset returns the persisted offset for (tenant, topic,
// partition), or 0 + nil if no row exists. Mirrors
// canonical_store.py:get_applied_offset (514) extended with topic /
// partition.
func (s *CanonicalStore) GetAppliedOffset(ctx context.Context, tenantID, topic string, partition int32) (int64, error) {
	db, err := s.readDB(tenantID)
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
