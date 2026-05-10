package store

import (
	"context"
	"database/sql"
	"fmt"
)

// BatchTxn is the explicit transaction handle the applier wraps a batch
// of write ops in. Mirrors canonical_store.py:batch_transaction
// (1265-1289) — BEGIN IMMEDIATE so the writer slot is acquired up front
// (avoiding deferred-begin upgrade deadlocks).
//
// We deliberately use *sql.Conn (not *sql.Tx) because database/sql's
// BeginTx issues a plain "BEGIN" / "BEGIN DEFERRED" depending on driver
// and we need the explicit IMMEDIATE lock-mode for parity with Python.
//
// Callers MUST call either Commit or Rollback exactly once. The
// per-tenant write mutex is held for the lifetime of the BatchTxn so
// only one writer at a time touches the single pooled connection.
type BatchTxn struct {
	conn     *sql.Conn
	tenantID string
	store    *CanonicalStore
	entry    *poolEntry
	done     bool
}

// BeginBatch starts a BEGIN IMMEDIATE transaction for tenantID. Lazy-
// opens the tenant DB if needed. Callers MUST balance every BeginBatch
// with exactly one Commit or Rollback.
func (s *CanonicalStore) BeginBatch(ctx context.Context, tenantID string) (*BatchTxn, error) {
	_, e, err := s.dbAuto(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	e.writeMu.Lock()
	conn, err := e.db.Conn(ctx)
	if err != nil {
		e.writeMu.Unlock()
		return nil, fmt.Errorf("store: get conn for tenant %q: %w", tenantID, err)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		_ = conn.Close()
		e.writeMu.Unlock()
		return nil, fmt.Errorf("store: BEGIN IMMEDIATE %q: %w", tenantID, err)
	}
	return &BatchTxn{
		conn:     conn,
		tenantID: tenantID,
		store:    s,
		entry:    e,
	}, nil
}

// Conn returns the underlying *sql.Conn. Callers may execute
// parameterised statements directly against it.
func (b *BatchTxn) Conn() *sql.Conn { return b.conn }

// TenantID returns the tenant_id this batch txn is scoped to.
func (b *BatchTxn) TenantID() string { return b.tenantID }

// Commit commits the transaction, returns the conn to the pool, and
// releases the per-tenant write mutex.
func (b *BatchTxn) Commit() error {
	if b.done {
		return fmt.Errorf("store: BatchTxn already finished")
	}
	b.done = true
	defer b.cleanup()
	if _, err := b.conn.ExecContext(context.Background(), "COMMIT"); err != nil {
		return fmt.Errorf("store: commit batch: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction, returns the conn to the pool,
// and releases the per-tenant write mutex. Safe to call after a
// successful Commit (no-op).
func (b *BatchTxn) Rollback() error {
	if b.done {
		return nil
	}
	b.done = true
	defer b.cleanup()
	if _, err := b.conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		return fmt.Errorf("store: rollback batch: %w", err)
	}
	return nil
}

func (b *BatchTxn) cleanup() {
	_ = b.conn.Close() // returns the conn to the pool
	b.entry.writeMu.Unlock()
}

// withWrite runs fn inside a per-tenant write-locked BEGIN IMMEDIATE
// transaction. Used by single-op write methods (DeleteNode, ShareNode,
// etc.) so they share the same lock + transaction discipline as the
// applier batch path.
func (s *CanonicalStore) withWrite(ctx context.Context, tenantID string, fn func(conn *sql.Conn) error) (err error) {
	bt, err := s.BeginBatch(ctx, tenantID)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = bt.Rollback()
		}
	}()
	if err = fn(bt.conn); err != nil {
		return err
	}
	return bt.Commit()
}
